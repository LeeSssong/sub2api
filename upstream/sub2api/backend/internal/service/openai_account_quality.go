package service

import (
	"context"

	"time"
)

// OpenAIAccountQuality is the account-level, non-image quality projection used
// by the unified scheduler. U is deliberately absent: effective cost is read
// live from EffectiveCostForAccount at candidate-build time.
type OpenAIQualityWindow string

const (
	OpenAIQualityWindow5M  OpenAIQualityWindow = "w5"
	OpenAIQualityWindow55M OpenAIQualityWindow = "w55"
)

type OpenAIQualityWindowMetrics struct {
	AttemptCount              int64
	SuccessCount              int64
	SuccessRate               *float64
	TTFTSampleCount           int64
	TTFTP50MS                 *float64
	TTFTP90MS                 *float64
	OutputRateSampleCount     int64
	OutputRateTokensPerSecond *float64
}

type OpenAIAccountQuality struct {
	AccountID int64
	Windows   map[OpenAIQualityWindow]OpenAIQualityWindowMetrics
	// Legacy aggregate fields remain additive for callers not yet migrated.
	AttemptCount         int64
	SuccessCount         int64
	SuccessRate          *float64
	TTFTSampleCount      int64
	TTFTTrimmedMeanMS    *float64
	LatencySampleCount   int64
	LatencyTrimmedMeanMS *float64
}

// OpenAIAccountQualityRepository is the narrow read-only repository contract
// consumed by the scheduler. The broader UsageLogRepository remains unchanged
// for compatibility with existing test doubles and services.
type OpenAIAccountQualityRepository interface {
	ListOpenAIAccountQuality(ctx context.Context, start, end time.Time) ([]OpenAIAccountQuality, error)
}

type OpenAIAccountQualitySnapshot struct {
	WindowStart time.Time
	WindowEnd   time.Time
	SnapshotAt  time.Time
	Stale       bool
	Accounts    map[int64]OpenAIAccountQuality
}

type OpenAIAccountQualitySnapshotProvider interface {
	Snapshot(ctx context.Context) OpenAIAccountQualitySnapshot
}

// OpenAIAccountQualityRefreshRequester is an optional, non-blocking signal
// used after a real request has been persisted. It intentionally stays
// separate from Snapshot so dispatch reads remain cheap and side-effect free.
type OpenAIAccountQualityRefreshRequester interface {
	RequestRefresh(ctx context.Context)
}
