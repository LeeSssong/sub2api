package repository

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

const monitorV4QualityTrafficSQL = `SELECT u.group_id,
 date_bin($4::interval,u.created_at,$2::timestamptz) AS bucket_start,
 COUNT(*) AS usage_request_count,
 COUNT(*) FILTER (WHERE u.quality_status='degraded') AS degraded_request_count,
 COUNT(u.quality_status) AS quality_snapshot_request_count
 FROM usage_logs u
 WHERE u.group_id=ANY($1::bigint[]) AND u.created_at >= $2 AND u.created_at < $3
 AND COALESCE(u.usage_completeness,'complete') <> 'unknown'
 GROUP BY u.group_id,bucket_start`

func (r *accountMonitorRepository) readQualityTrafficCounts(ctx context.Context, ids []int64, start, end time.Time, step time.Duration, points []service.MonitorV4TimelinePoint) error {
	rows, err := r.db.QueryContext(ctx, monitorV4QualityTrafficSQL, pq.Array(ids), start.UTC(), end.UTC(), step.String())
	if err != nil {
		return err
	}
	defer rows.Close()
	type key struct {
		group int64
		at    int64
	}
	index := make(map[key]*service.MonitorV4TimelinePoint, len(points))
	for i := range points {
		index[key{points[i].GroupID, points[i].Start.UnixNano()}] = &points[i]
	}
	for rows.Next() {
		var group int64
		var at time.Time
		var total, degraded, snapshots int
		if err := rows.Scan(&group, &at, &total, &degraded, &snapshots); err != nil {
			return err
		}
		if point := index[key{group, at.UnixNano()}]; point != nil {
			point.UsageRequestCount = total
			point.DegradedRequestCount = degraded
			point.QualitySnapshotRequestCount = snapshots
		}
	}
	return rows.Err()
}
