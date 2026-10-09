package handler

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type popularityRepo struct {
	service.UsageLogRepository
	calls      int
	start, end time.Time
	filters    usagestats.UsageLogFilters
	source     string
	rows       []usagestats.ModelStat
	err        error
}

func (r *popularityRepo) GetModelStatsWithUsageFiltersBySource(_ context.Context, start, end time.Time, filters usagestats.UsageLogFilters, source string) ([]usagestats.ModelStat, error) {
	r.calls++
	r.start = start
	r.end = end
	r.filters = filters
	r.source = source
	return r.rows, r.err
}
func popularityRouter(repo *popularityRepo, authenticated bool) (*gin.Engine, *UsageHandler) {
	gin.SetMode(gin.TestMode)
	h := NewUsageHandler(service.NewUsageService(repo, nil, nil, nil), nil, nil, nil)
	r := gin.New()
	if authenticated {
		r.Use(func(c *gin.Context) {
			c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
			c.Next()
		})
	}
	r.GET("/usage/models/popularity", h.ModelPopularity)
	return r, h
}
func requestPopularity(r *gin.Engine) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/usage/models/popularity?user_id=42&start_date=2000-01-01&model_source=upstream&group_id=9", nil))
	return rec
}
func TestModelPopularityRequiresLogin(t *testing.T) {
	repo := &popularityRepo{}
	r, _ := popularityRouter(repo, false)
	require.Equal(t, http.StatusUnauthorized, requestPopularity(r).Code)
	require.Zero(t, repo.calls)
}
func TestModelPopularityUsesNativeSiteRequestsForRolling24Hours(t *testing.T) {
	repo := &popularityRepo{rows: []usagestats.ModelStat{{Model: "gpt-5.4", Requests: 19, ActualCost: 45, AccountCost: 36, TotalTokens: 120}}}
	r, _ := popularityRouter(repo, true)
	before := time.Now().UTC()
	res := requestPopularity(r)
	require.Equal(t, http.StatusOK, res.Code, res.Body.String())
	require.Equal(t, 24*time.Hour, repo.end.Sub(repo.start))
	require.False(t, repo.end.Before(before))
	require.False(t, repo.end.After(time.Now().UTC()))
	require.Equal(t, usagestats.UsageLogFilters{}, repo.filters)
	require.Equal(t, usagestats.ModelSourceRequested, repo.source)
	var envelope struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data, 3)
	var models []map[string]any
	require.NoError(t, json.Unmarshal(envelope.Data["models"], &models))
	require.Len(t, models, 1)
	require.Equal(t, map[string]any{"model": "gpt-5.4", "requests": float64(19)}, models[0])
}
func TestModelPopularityCachesConcurrentRequestsAndRefreshesExpiredWindow(t *testing.T) {
	repo := &popularityRepo{}
	r, h := popularityRouter(repo, true)
	var wg sync.WaitGroup
	codes := make(chan int, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); codes <- requestPopularity(r).Code }()
	}
	wg.Wait()
	close(codes)
	for code := range codes {
		require.Equal(t, http.StatusOK, code)
	}
	require.Equal(t, 1, repo.calls)
	h.modelPopularityMu.Lock()
	h.modelPopularity.EndTime = time.Now().UTC().Add(-2 * time.Minute)
	h.modelPopularityMu.Unlock()
	require.Equal(t, http.StatusOK, requestPopularity(r).Code)
	require.Equal(t, 2, repo.calls)
}
func TestModelPopularityDoesNotCacheFailures(t *testing.T) {
	repo := &popularityRepo{err: errors.New("database unavailable")}
	r, _ := popularityRouter(repo, true)
	require.Equal(t, http.StatusInternalServerError, requestPopularity(r).Code)
	repo.err = nil
	require.Equal(t, http.StatusOK, requestPopularity(r).Code)
	require.Equal(t, 2, repo.calls)
}
func TestModelPopularityEmptyResultIsAnArray(t *testing.T) {
	r, _ := popularityRouter(&popularityRepo{}, true)
	res := requestPopularity(r)
	require.Contains(t, res.Body.String(), `"models":[]`)
}
