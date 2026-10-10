package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type slaCountsReaderStub struct {
	MonitorV4ProjectionReader
	points     []MonitorV4TimelinePoint
	ids        []int64
	start, end time.Time
}

func (s *slaCountsReaderStub) ReadMonitorV4Timeline(_ context.Context, ids []int64, start, end time.Time, _ time.Duration) ([]MonitorV4TimelinePoint, error) {
	s.ids = ids
	s.start = start
	s.end = end
	return s.points, nil
}
func TestMonitorV4SLACountsUsesExactSnapshotBoundsAndVisibleGroups(t *testing.T) {
	start := time.Date(2026, 10, 10, 10, 44, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	native := &slaCountsReaderStub{points: []MonitorV4TimelinePoint{{GroupID: 6, RequestCount: 600, SuccessCount: 600}, {GroupID: 6, RequestCount: 67, SuccessCount: 60}}}
	svc := &MonitorV4Service{native: native}
	snapshot := &MonitorV4Snapshot{WindowStart: start, WindowEnd: end, Groups: []MonitorV4Group{{ID: 6}, {ID: 7}}}
	counts, err := svc.SLACounts(context.Background(), snapshot)
	require.NoError(t, err)
	require.Equal(t, MonitorV4SLACounts{RequestCount: 667, SuccessCount: 660}, counts[6])
	require.Equal(t, MonitorV4SLACounts{}, counts[7])
	require.Equal(t, []int64{6, 7}, native.ids)
	require.Equal(t, start, native.start)
	require.Equal(t, end, native.end)
	native.points = []MonitorV4TimelinePoint{{GroupID: 99, RequestCount: 1, SuccessCount: 1}}
	_, err = svc.SLACounts(context.Background(), snapshot)
	require.Error(t, err)
	native.points = []MonitorV4TimelinePoint{{GroupID: 6, RequestCount: 1, SuccessCount: 2}}
	_, err = svc.SLACounts(context.Background(), snapshot)
	require.Error(t, err)
}
