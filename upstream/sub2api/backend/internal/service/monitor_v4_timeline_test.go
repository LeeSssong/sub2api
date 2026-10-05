package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type timelineReaderStub struct {
	monitorV4NativeReaderStub
	ids        []int64
	step       time.Duration
	start, end time.Time
}

func (r *timelineReaderStub) ReadMonitorV4Timeline(_ context.Context, ids []int64, start, end time.Time, step time.Duration) ([]MonitorV4TimelinePoint, error) {
	r.ids = ids
	r.step = step
	r.start = start
	r.end = end
	return []MonitorV4TimelinePoint{}, nil
}
func TestMonitorV4TimelineVisibilityAndWindows(t *testing.T) {
	for _, w := range []MonitorV4Window{MonitorV4Window1H, MonitorV4Window24H, MonitorV4Window7D} {
		t.Run(string(w), func(t *testing.T) {
			end := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
			start, _ := monitorV4WindowStart(w, end)
			native := &timelineReaderStub{}
			svc := NewMonitorV4Service(&monitorV4GroupRepoStub{groups: []Group{{ID: 7, Status: StatusActive}, {ID: 99, Status: StatusActive}}}, &monitorV4AvailableGroupReaderStub{}, native, nil, &monitorV4ConfiguredGroupReaderStub{config: &ChannelMonitorV2Config{GroupIDs: []int64{7}}})
			svc.SetSnapshotStore(&monitorV4SnapshotStoreStub{loaded: MonitorV4StoredWindow{Window: w, SnapshotID: "snapshot", WindowStart: start, WindowEnd: end, GeneratedAt: end, ContractVersion: MonitorV4ContractVersion, Groups: map[int64]MonitorV4GroupProjection{}}})
			_, err := svc.Timeline(context.Background(), 0, w, end)
			require.Error(t, err)
			require.Nil(t, native.ids)
			result, err := svc.Timeline(context.Background(), 42, w, end)
			require.NoError(t, err)
			require.Equal(t, []int64{7}, native.ids)
			require.Equal(t, end, result.GeneratedAt)
			require.Equal(t, start, native.start)
			counts := map[MonitorV4Window]int{MonitorV4Window1H: 12, MonitorV4Window24H: 48, MonitorV4Window7D: 42}
			require.Equal(t, counts[w], int(end.Sub(start)/native.step))
		})
	}
}
