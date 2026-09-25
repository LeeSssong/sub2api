package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// PelicanShowcaseStats counts recorded terminal drawing results, not retained artwork.
type PelicanShowcaseStats struct {
	SuccessCount int64    `json:"success_count"`
	TotalCount   int64    `json:"total_count"`
	SuccessRate  *float64 `json:"success_rate"`
}

type PelicanShowcaseStatsWindow struct {
	From              time.Time `json:"from"`
	To                time.Time `json:"to"`
	CoverageStartedAt time.Time `json:"coverage_started_at"`
	Complete          bool      `json:"complete"`
}

type PelicanShowcaseCounts struct{ SuccessCount, TotalCount int64 }

type PelicanShowcaseStatistics struct {
	CoverageStartedAt time.Time
	Total             PelicanShowcaseCounts
	Groups            map[int64]PelicanShowcaseCounts
}

// Optional capability keeps gallery reads compatible when statistics are unavailable.
type PelicanShowcaseStatisticsRepository interface {
	ReadStatistics(context.Context, []int64, time.Time, time.Time) (*PelicanShowcaseStatistics, error)
}

func (counts PelicanShowcaseCounts) statistics() *PelicanShowcaseStats {
	out := &PelicanShowcaseStats{SuccessCount: counts.SuccessCount, TotalCount: counts.TotalCount}
	if counts.TotalCount > 0 {
		rate := float64(counts.SuccessCount) / float64(counts.TotalCount) * 100
		out.SuccessRate = &rate
	}
	return out
}

func (s *PelicanShowcaseService) populateStatistics(ctx context.Context, view *PelicanShowcaseView, groupIDs []int64, now time.Time) {
	reader, ok := s.repo.(PelicanShowcaseStatisticsRepository)
	if !ok {
		return
	}
	from := now.Add(-24 * time.Hour)
	data, err := reader.ReadStatistics(ctx, groupIDs, from, now)
	if err != nil {
		logger.LegacyPrintf("service.pelican_showcase", "statistics unavailable: %v", err)
		return
	}
	if data == nil || data.CoverageStartedAt.IsZero() || data.CoverageStartedAt.After(now) {
		return
	}
	view.Stats = data.Total.statistics()
	view.StatsWindow = &PelicanShowcaseStatsWindow{From: from, To: now,
		CoverageStartedAt: data.CoverageStartedAt, Complete: !data.CoverageStartedAt.After(from)}
	for _, group := range view.Groups {
		group.Stats = data.Groups[group.ID].statistics()
	}
}

// IsPelicanDrawingResult uses the execution's config snapshot and terminal status.
// The runner already validates generation completion; statistics do not grade images.
func IsPelicanDrawingResult(result *ScheduledTestResult) bool {
	if result == nil || result.PlanID <= 0 || result.FinishedAt.IsZero() ||
		(result.Status != "success" && result.Status != "failed") {
		return false
	}
	cfg := result.PelicanConfig
	return cfg != nil && cfg.Quality == nil && (cfg.QuestionKind == "" || cfg.QuestionKind == "pelican") && !isBuiltinCandyPlan(cfg)
}
