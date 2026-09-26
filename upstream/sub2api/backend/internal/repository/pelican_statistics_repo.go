package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// This runs on collector startup and each scheduled tick, independently of gallery settings.
// ON CONFLICT does not update the stable coverage timestamp or rewrite the metadata row.
func (r *scheduledTestResultRepository) MaintainPelicanStatistics(ctx context.Context, now time.Time) error {
	if _, err := r.db.ExecContext(ctx, `INSERT INTO pelican_drawing_statistics_state (id, coverage_started_at)
 VALUES (1, $1) ON CONFLICT (id) DO NOTHING`, now); err != nil {
		return err
	}
	// Keep two days so an in-flight 24h read never races the retention boundary.
	// Batches bound each transaction; drain the backlog instead of capping cleanup throughput.
	cutoff := now.Add(-48 * time.Hour)
	for {
		result, err := r.db.ExecContext(ctx, `DELETE FROM pelican_drawing_outcomes WHERE source_result_id IN (
   SELECT source_result_id FROM pelican_drawing_outcomes WHERE completed_at < $1
   ORDER BY completed_at, source_result_id LIMIT 1000)`, cutoff)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n < 1000 {
			return r.prunePelicanReportFacts(ctx, cutoff)
		}
	}
}

func recordPelicanDrawingOutcome(ctx context.Context, tx *sql.Tx, result *service.ScheduledTestResult, groupIDs []int64) error {
	if groupIDs == nil {
		// Results saved without a scheduler claim still snapshot membership in their write
		// transaction. Claimed executions pass their earlier execution-start snapshot.
		var groups pq.Int64Array
		err := tx.QueryRowContext(ctx, `SELECT COALESCE(array_agg(DISTINCT ag.group_id ORDER BY ag.group_id), '{}'::bigint[])
   FROM scheduled_test_plans p
   JOIN accounts a ON a.id = p.account_id AND a.deleted_at IS NULL
   JOIN account_groups ag ON ag.account_id = p.account_id WHERE p.id = $1`, result.PlanID).Scan(&groups)
		if err != nil {
			return err
		}
		groupIDs = []int64(groups)
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO pelican_drawing_outcomes (source_result_id, completed_at, success, group_ids)
 VALUES ($1, $2, $3, ARRAY(SELECT DISTINCT id FROM unnest($4::bigint[]) AS id WHERE id > 0 ORDER BY id))
 ON CONFLICT (source_result_id) DO NOTHING`, result.ID, result.FinishedAt, result.Status == "success", pq.Array(groupIDs))
	return err
}

// ReadStatistics uses one MVCC snapshot for metadata, per-group counts and distinct total.
// Group visibility is enforced here as well as by the service so a concurrent disable
// or deletion cannot make historical membership expose a hidden group.
func (r *pelicanShowcaseRepository) ReadStatistics(ctx context.Context, groupIDs []int64, from, to time.Time) (*service.PelicanShowcaseStatistics, error) {
	rows, err := r.db.QueryContext(ctx, `WITH visible AS (
  SELECT id FROM groups WHERE id = ANY($1) AND deleted_at IS NULL AND status = $4
 ), matched AS (
  SELECT o.source_result_id, o.success, g.id AS group_id
  FROM pelican_drawing_outcomes o JOIN visible g ON g.id = ANY(o.group_ids)
  WHERE o.completed_at >= $2 AND o.completed_at < $3
 ), counts AS (
  SELECT 0::bigint AS group_id,
   COUNT(DISTINCT source_result_id) FILTER (WHERE success) AS success_count,
   COUNT(DISTINCT source_result_id) AS total_count FROM matched
  UNION ALL
  SELECT group_id, COUNT(DISTINCT source_result_id) FILTER (WHERE success), COUNT(DISTINCT source_result_id)
  FROM matched GROUP BY group_id
 ) SELECT s.coverage_started_at, c.group_id, c.success_count, c.total_count
 FROM pelican_drawing_statistics_state s CROSS JOIN counts c WHERE s.id = 1`,
		pq.Array(groupIDs), from, to, service.StatusActive)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out *service.PelicanShowcaseStatistics
	for rows.Next() {
		var coverage time.Time
		var groupID int64
		var counts service.PelicanShowcaseCounts
		if err := rows.Scan(&coverage, &groupID, &counts.SuccessCount, &counts.TotalCount); err != nil {
			return nil, err
		}
		if out == nil {
			out = &service.PelicanShowcaseStatistics{CoverageStartedAt: coverage, Groups: map[int64]service.PelicanShowcaseCounts{}}
		}
		if groupID == 0 {
			out.Total = counts
		} else {
			out.Groups[groupID] = counts
		}
	}
	return out, rows.Err()
}
