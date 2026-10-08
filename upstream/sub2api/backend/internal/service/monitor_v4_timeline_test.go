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
			require.Equal(t, "ops_sla", result.SuccessRateBasis)
			require.Equal(t, start, native.start)
			counts := map[MonitorV4Window]int{MonitorV4Window1H: 12, MonitorV4Window24H: 24, MonitorV4Window7D: 168}
			require.Equal(t, counts[w], int(end.Sub(start)/native.step))
			_, err = svc.TimelineWithGranularity(context.Background(), 42, w, "day", end)
			require.NoError(t, err)
			if w == MonitorV4Window1H {
				require.Equal(t, 5*time.Minute, native.step)
			} else {
				require.Equal(t, 24*time.Hour, native.step)
			}
			_, err = svc.TimelineWithGranularity(context.Background(), 42, w, "invalid", end)
			require.Error(t, err)
		})
	}
}

func TestMonitorV4RefreshSummaryAndTimelineShareHourlyBounds(t *testing.T) {
	zone := time.FixedZone("Asia/Shanghai", 8*60*60)
	for _, asOf := range []time.Time{
		time.Date(2026, 10, 7, 2, 47, 37, 0, zone),
		time.Date(2026, 10, 7, 0, 3, 37, 0, zone),
		time.Date(2026, 10, 7, 2, 0, 0, 0, zone),
	} {
		t.Run(asOf.Format(time.RFC3339), func(t *testing.T) {
			native := &timelineReaderStub{}
			store := &monitorV4RefreshStoreStub{}
			svc := NewMonitorV4Service(&monitorV4GroupRepoStub{groups: []Group{{ID: 7, Status: StatusActive}}}, &monitorV4AvailableGroupReaderStub{}, native, nil, &monitorV4ConfiguredGroupReaderStub{config: &ChannelMonitorV2Config{GroupIDs: []int64{7}}})
			svc.SetSnapshotStore(store)
			require.NoError(t, svc.RefreshMonitorV4Snapshots(context.Background(), asOf))
			store.byWindow = map[MonitorV4Window]MonitorV4StoredWindow{}
			for i, row := range store.replaced {
				store.byWindow[row.Window] = row
				wantEnd := asOf.UTC().Truncate(time.Hour)
				duration := 24 * time.Hour
				if row.Window == MonitorV4Window1H {
					wantEnd = asOf.UTC().Truncate(time.Minute)
					duration = time.Hour
				} else if row.Window == MonitorV4Window7D {
					duration = 7 * 24 * time.Hour
				}
				require.Equal(t, wantEnd, row.WindowEnd)
				require.Equal(t, wantEnd.Add(-duration), row.WindowStart)
				require.Equal(t, row.WindowEnd, native.calls[i].end)
				require.Equal(t, row.WindowStart, native.calls[i].start)
				snapshot, err := svc.Snapshot(context.Background(), 42, row.Window, asOf)
				require.NoError(t, err)
				// An hourly statistical cutoff must not make a freshly rebuilt snapshot stale.
				require.Equal(t, asOf.UTC().Truncate(time.Minute), snapshot.GeneratedAt)
				for _, granularity := range []string{"hour", "day"} {
					timeline, err := svc.TimelineWithGranularity(context.Background(), 42, row.Window, granularity, asOf)
					require.NoError(t, err)
					require.Equal(t, row.WindowStart, native.start)
					require.Equal(t, row.WindowEnd, native.end)
					require.Equal(t, snapshot.GeneratedAt, timeline.GeneratedAt)
				}
			}
		})
	}
}
