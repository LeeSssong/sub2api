package handler

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

type modelPopularityItem struct {
	Model    string `json:"model"`
	Requests int64  `json:"requests"`
}
type modelPopularityResponse struct {
	Models    []modelPopularityItem `json:"models"`
	StartTime time.Time             `json:"start_time"`
	EndTime   time.Time             `json:"end_time"`
}

// ModelPopularity projects native requested-model statistics for price-table ordering.
// GET /api/v1/usage/models/popularity; authenticated, fixed site-wide rolling 24h.
// Never expose the native aggregate's cost, token, account or user fields here.
func (h *UsageHandler) ModelPopularity(c *gin.Context) {
	if _, ok := middleware2.GetAuthSubjectFromContext(c); !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	h.modelPopularityMu.Lock()
	now := time.Now().UTC()
	if h.modelPopularity != nil && now.Sub(h.modelPopularity.EndTime) < time.Minute {
		snapshot := h.modelPopularity
		h.modelPopularityMu.Unlock()
		response.Success(c, snapshot)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	start := now.Add(-24 * time.Hour)
	stats, err := h.usageService.GetModelStatsWithFiltersBySource(ctx, start, now, usagestats.UsageLogFilters{}, usagestats.ModelSourceRequested)
	if err != nil {
		h.modelPopularityMu.Unlock()
		response.InternalError(c, "Model popularity temporarily unavailable")
		return
	}
	snapshot := &modelPopularityResponse{Models: make([]modelPopularityItem, 0, len(stats)), StartTime: start, EndTime: now}
	for _, stat := range stats {
		snapshot.Models = append(snapshot.Models, modelPopularityItem{Model: stat.Model, Requests: stat.Requests})
	}
	h.modelPopularity = snapshot
	h.modelPopularityMu.Unlock()
	response.Success(c, snapshot)
}
