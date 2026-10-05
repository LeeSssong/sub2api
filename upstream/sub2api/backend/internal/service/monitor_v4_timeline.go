package service

import (
	"context"
	"fmt"
	"time"
)

type MonitorV4TimelinePoint struct {
	CacheHitRate *float64  `json:"cache_hit_rate"`
	TTFTP50MS    *float64  `json:"ttft_p50_ms"`
	GroupID      int64     `json:"group_id"`
	Start        time.Time `json:"start"`
	End          time.Time `json:"end"`
	RequestCount int       `json:"request_count"`
	SuccessCount int       `json:"success_count"`
}
type MonitorV4Timeline struct {
	Window      MonitorV4Window          `json:"window"`
	GeneratedAt time.Time                `json:"generated_at"`
	Points      []MonitorV4TimelinePoint `json:"points"`
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
	// Reuse the exact user visibility contract; never accept arbitrary caller group IDs.
	snapshot, err := s.Snapshot(ctx, userID, window, now)
	if err != nil {
		return nil, err
	}
	step := 5 * time.Minute
	switch window {
	case MonitorV4Window24H:
		step = 30 * time.Minute
	case MonitorV4Window7D:
		step = 4 * time.Hour
	}
	end := snapshot.GeneratedAt.UTC()
	start, err := monitorV4WindowStart(window, end)
	if err != nil {
		return nil, err
	}
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
	return &MonitorV4Timeline{Window: window, GeneratedAt: end, Points: points}, nil
}
