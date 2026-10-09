package service

import (
	"context"
	"fmt"
	"time"
)

type MonitorV4TimelinePoint struct {
	UsageRequestCount           int       `json:"usage_request_count"`
	DegradedRequestCount        int       `json:"degraded_request_count"`
	QualitySnapshotRequestCount int       `json:"quality_snapshot_request_count"`
	GradedRoundCount            int       `json:"graded_round_count"`
	SuspectedDegradedRoundCount int       `json:"suspected_degraded_round_count"`
	CacheHitRate                *float64  `json:"cache_hit_rate"`
	TTFTP50MS                   *float64  `json:"ttft_p50_ms"`
	GroupID                     int64     `json:"group_id"`
	Start                       time.Time `json:"start"`
	End                         time.Time `json:"end"`
	RequestCount                int       `json:"request_count"`
	SuccessCount                int       `json:"success_count"`
}
type MonitorV4Timeline struct {
	SuccessRateBasis string                   `json:"success_rate_basis"`
	Granularity      string                   `json:"granularity"`
	Window           MonitorV4Window          `json:"window"`
	GeneratedAt      time.Time                `json:"generated_at"`
	Points           []MonitorV4TimelinePoint `json:"points"`
}
type MonitorV4TimelineReader interface {
	ReadMonitorV4Timeline(context.Context, []int64, time.Time, time.Time, time.Duration) ([]MonitorV4TimelinePoint, error)
}

func (s *AccountMonitorService) ReadMonitorV4Timeline(ctx context.Context, ids []int64, start, end time.Time, step time.Duration) ([]MonitorV4TimelinePoint, error) {
	r, ok := s.repo.(MonitorV4TimelineReader)
	if !ok {
		return nil, fmt.Errorf("timeline repository unavailable")
	}
	return r.ReadMonitorV4Timeline(ctx, ids, start, end, step)
}
func (s *MonitorV4Service) Timeline(ctx context.Context, userID int64, window MonitorV4Window, now time.Time) (*MonitorV4Timeline, error) {
	return s.TimelineWithGranularity(ctx, userID, window, "hour", now)
}
func (s *MonitorV4Service) TimelineWithGranularity(ctx context.Context, userID int64, window MonitorV4Window, granularity string, now time.Time) (*MonitorV4Timeline, error) {
	if granularity != "hour" && granularity != "day" {
		return nil, fmt.Errorf("unsupported timeline granularity")
	}
	// Reuse the exact user visibility contract; never accept arbitrary caller group IDs.
	snapshot, err := s.Snapshot(ctx, userID, window, now)
	if err != nil {
		return nil, err
	}
	step := time.Hour
	if granularity == "day" {
		step = 24 * time.Hour
	}
	if window == MonitorV4Window1H {
		step = 5 * time.Minute
		granularity = "5m"
	}
	// Read the same persisted statistical range as the summary. GeneratedAt
	// represents refresh freshness and may be later than the hourly cutoff.
	start, end := snapshot.WindowStart, snapshot.WindowEnd
	ids := make([]int64, 0, len(snapshot.Groups))
	for _, g := range snapshot.Groups {
		ids = append(ids, g.ID)
	}
	reader, ok := s.native.(MonitorV4TimelineReader)
	if !ok {
		return nil, fmt.Errorf("timeline reader unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	points, err := reader.ReadMonitorV4Timeline(ctx, ids, start, end, step)
	if err != nil {
		return nil, err
	}
	return &MonitorV4Timeline{SuccessRateBasis: "ops_sla", Granularity: granularity, Window: window, GeneratedAt: snapshot.GeneratedAt, Points: points}, nil
}
