package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type convergenceCapacityCache struct {
	concurrencyCacheMock
	full         bool
	backendError bool
}

func (c *convergenceCapacityCache) AcquireAccountSlot(context.Context, int64, int, string) (bool, error) {
	if c.backendError {
		return false, errors.New("redis unavailable")
	}
	return false, nil
}
func (c *convergenceCapacityCache) IncrementAccountWaitCount(context.Context, int64, int) (bool, error) {
	return !c.full, nil
}

func TestNativeConvergenceCapacityErrorsWriteTerminalResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name               string
		full, backendError bool
	}{{"queue full", true, false}, {"redis unavailable", false, true}, {"wait timeout", false, false}} {
		for _, stream := range []bool{false, true} {
			name := tc.name + "/json"
			if stream {
				name = tc.name + "/started_sse"
			}
			t.Run(name, func(t *testing.T) {
				cache := &convergenceCapacityCache{full: tc.full, backendError: tc.backendError}
				h := &OpenAIGatewayHandler{gatewayService: &service.OpenAIGatewayService{}, concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, 0)}
				selection := &service.AccountSelectionResult{Account: &service.Account{ID: 1, Concurrency: 1}, WaitPlan: &service.AccountWaitPlan{AccountID: 1, MaxConcurrency: 1, MaxWaiting: 1, Timeout: time.Millisecond}}
				router := gin.New()
				router.POST("/v1/responses", func(c *gin.Context) {
					started := stream
					if stream {
						c.Header("Content-Type", "text/event-stream")
						c.String(http.StatusOK, ": ping\n\n")
					}
					_, result := h.acquireResponsesAccountSlot(c, nil, "", selection, stream, &started, zap.NewNop())
					require.Equal(t, openAISlotAcquireFailed, result)
				})
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/responses", nil))
				if stream {
					require.Contains(t, rec.Body.String(), "response.failed")
				} else {
					require.GreaterOrEqual(t, rec.Code, 400)
				}
				require.NotEmpty(t, rec.Body.String())
				require.Contains(t, rec.Body.String(), "error")
			})
		}
	}
}

func TestNativeConvergenceWaitSuccessReleasesSlot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var attempts atomic.Int32
	cache := &concurrencyCacheMock{acquireAccountSlotFn: func(context.Context, int64, int, string) (bool, error) { return attempts.Add(1) >= 2, nil }}
	h := &OpenAIGatewayHandler{gatewayService: &service.OpenAIGatewayService{}, concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, 0)}
	selection := &service.AccountSelectionResult{Account: &service.Account{ID: 1, Concurrency: 1}, WaitPlan: &service.AccountWaitPlan{AccountID: 1, MaxConcurrency: 1, MaxWaiting: 1, Timeout: time.Second}}
	router := gin.New()
	router.POST("/v1/responses", func(c *gin.Context) {
		started := false
		release, result := h.acquireResponsesAccountSlot(c, nil, "", selection, false, &started, zap.NewNop())
		require.Equal(t, openAISlotAcquireOK, result)
		require.NotNil(t, release)
		defer release()
		c.JSON(http.StatusOK, gin.H{"forward_allowed": true})
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/responses", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"forward_allowed":true`)
	require.GreaterOrEqual(t, attempts.Load(), int32(2))
	require.EqualValues(t, 1, atomic.LoadInt32(&cache.releaseAccountCalled))
}
