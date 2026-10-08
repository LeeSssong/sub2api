package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type lineCheckRouteService struct{ calls atomic.Int64 }

func (*lineCheckRouteService) Snapshot(context.Context, int64, service.MonitorV4Window, time.Time) (*service.MonitorV4Snapshot, error) {
	return &service.MonitorV4Snapshot{}, nil
}

func (s *lineCheckRouteService) Check(context.Context, int64, []int64) ([]service.MonitorV4CheckResult, error) {
	s.calls.Add(1)
	return []service.MonitorV4CheckResult{}, nil
}

func lineCheckRateLimitRouter(client *redis.Client, checker interface {
	Snapshot(context.Context, int64, service.MonitorV4Window, time.Time) (*service.MonitorV4Snapshot, error)
}) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	jwt := middleware.JWTAuthMiddleware(func(c *gin.Context) {
		id, _ := strconv.ParseInt(c.GetHeader("Test-User"), 10, 64)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: id})
		c.Set(string(middleware.ContextKeyUserRole), service.RoleAdmin)
		c.Next()
	})
	RegisterUserRoutes(router.Group("/api/v1"), &handler.Handlers{MonitorV4: handler.NewMonitorV4Handler(checker)}, jwt,
		middleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }), nil, middleware.NewPanelRateLimiter(client, nil))
	return router
}

func requestLineCheck(router *gin.Engine, user int64) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/monitor-v4/check", strings.NewReader(`{"group_ids":[7,8],"user_id":999}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Test-User", strconv.FormatInt(user, 10))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestLineCheckRateLimitThirtyPerUserAcrossInstances(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	checker := &lineCheckRouteService{}
	first, second := lineCheckRateLimitRouter(client, checker), lineCheckRateLimitRouter(client, checker)
	for i := 0; i < 30; i++ {
		require.Equal(t, http.StatusOK, requestLineCheck(first, 42).Code)
	}
	limited := requestLineCheck(second, 42)
	require.Equal(t, http.StatusTooManyRequests, limited.Code)
	require.Equal(t, "60", limited.Header().Get("Retry-After"))
	require.Contains(t, limited.Body.String(), `"code":"LINE_CHECK_RATE_LIMITED"`)
	require.Contains(t, limited.Body.String(), `"retry_after_seconds":60`)
	require.EqualValues(t, 30, checker.calls.Load())
	require.Equal(t, http.StatusOK, requestLineCheck(second, 43).Code)
	// Unauthenticated requests must not consume a user's allowance.
	require.Equal(t, http.StatusUnauthorized, requestLineCheck(second, 0).Code)
}

func TestLineCheckRateLimitSlidingWindow(t *testing.T) {
	mr := miniredis.RunT(t)
	start := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	mr.SetTime(start)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	checker := &lineCheckRouteService{}
	router := lineCheckRateLimitRouter(client, checker)
	for i := 0; i < 15; i++ {
		require.Equal(t, http.StatusOK, requestLineCheck(router, 42).Code)
	}
	mr.FastForward(30 * time.Second)
	mr.SetTime(start.Add(30 * time.Second))
	for i := 0; i < 15; i++ {
		require.Equal(t, http.StatusOK, requestLineCheck(router, 42).Code)
	}
	limited := requestLineCheck(router, 42)
	require.Equal(t, http.StatusTooManyRequests, limited.Code)
	require.Equal(t, "30", limited.Header().Get("Retry-After"))
	mr.FastForward(30 * time.Second)
	mr.SetTime(start.Add(time.Minute))
	for i := 0; i < 15; i++ {
		require.Equal(t, http.StatusOK, requestLineCheck(router, 42).Code)
	}
	require.Equal(t, http.StatusTooManyRequests, requestLineCheck(router, 42).Code)
	mr.FastForward(time.Minute)
	mr.SetTime(start.Add(2 * time.Minute))
	require.Empty(t, mr.Keys(), "expired rate-limit data must not accumulate")
	require.Equal(t, http.StatusOK, requestLineCheck(router, 42).Code)
}

func TestLineCheckRateLimitConcurrentRequests(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	checker := &lineCheckRouteService{}
	router := lineCheckRateLimitRouter(client, checker)
	var wg sync.WaitGroup
	var allowed, limited atomic.Int64
	for i := 0; i < 60; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			switch requestLineCheck(router, 42).Code {
			case http.StatusOK:
				allowed.Add(1)
			case http.StatusTooManyRequests:
				limited.Add(1)
			}
		}()
	}
	wg.Wait()
	require.EqualValues(t, 30, allowed.Load())
	require.EqualValues(t, 30, limited.Load())
	require.EqualValues(t, 30, checker.calls.Load())
}

func TestLineCheckRateLimitRedisFailureStopsProbe(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1, DialTimeout: 100 * time.Millisecond})
	t.Cleanup(func() { _ = client.Close() })
	mr.Close()
	checker := &lineCheckRouteService{}
	w := requestLineCheck(lineCheckRateLimitRouter(client, checker), 42)
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.Contains(t, w.Body.String(), "LINE_CHECK_UNAVAILABLE")
	require.Zero(t, checker.calls.Load())
}

// Exercise the real service admission policy, replacing only external upstream IO.
type lineCheckNativeProbe struct{ calls atomic.Int64 }

func (*lineCheckNativeProbe) ProjectMonitorV4Groups(context.Context, []int64, time.Time, time.Time, time.Duration) (map[int64]service.MonitorV4GroupProjection, error) {
	return nil, nil
}
func (s *lineCheckNativeProbe) CheckMonitorGroup(context.Context, int64) (service.AccountMonitorProbeResult, error) {
	s.calls.Add(1)
	ttft := 80.0
	return service.AccountMonitorProbeResult{Status: "success", TTFTMS: &ttft, CheckedAt: time.Now()}, nil
}

type lineCheckAvailableGroups struct{}

func (lineCheckAvailableGroups) GetAvailableGroups(context.Context, int64) ([]service.Group, error) {
	return []service.Group{{ID: 7, Status: service.StatusActive}, {ID: 8, Status: service.StatusActive}}, nil
}
func TestLineCheckRateLimitWithRealServiceAllowsThirtySequentialChecks(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	native := &lineCheckNativeProbe{}
	svc := service.NewMonitorV4Service(nil, lineCheckAvailableGroups{}, native, nil, nil)
	router := lineCheckRateLimitRouter(client, svc)
	for i := 0; i < 30; i++ {
		w := requestLineCheck(router, 42)
		require.Equal(t, http.StatusOK, w.Code, "sequential check %d: %s", i+1, w.Body.String())
		require.Contains(t, w.Body.String(), `"status":"success"`)
	}
	w := requestLineCheck(router, 42)
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	require.Contains(t, w.Body.String(), "LINE_CHECK_RATE_LIMITED")
	require.EqualValues(t, 60, native.calls.Load(), "two requested groups per batch; denied check must not probe")
}
