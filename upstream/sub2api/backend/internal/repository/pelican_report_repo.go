package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"time"
)

// The native result and its independent report fact commit together.
func recordPelicanReportFact(ctx context.Context, tx *sql.Tx, result *service.ScheduledTestResult, groups []int64) error {
	kind, judgment, status := service.ClassifyPelicanReportResult(result)
	meta := result.ReportExecution
	if kind == "" || meta == nil || meta.ID == "" || groups == nil || result.PelicanConfig.ModelID == "" {
		return nil
	}
	if meta.ExpectedCount < 1 || meta.ExpectedCount > 8 {
		return fmt.Errorf("invalid report execution result count")
	}
	observed := result.FinishedAt
	if observed.IsZero() {
		observed = result.CreatedAt
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO pelican_report_facts
 (source_result_id,group_ids,model_id,kind,status,judgment,execution_id,shared_round_id,expected_count,scheduled_for,started_at,completed_at,observed_at)
 VALUES($1,ARRAY(SELECT DISTINCT id FROM unnest($2::bigint[]) AS id WHERE id>0 ORDER BY id),$3,$4,$5,$6,$7,NULLIF($8,''),$9,$10,$11,$12,$13)
 ON CONFLICT(source_result_id) DO NOTHING`, result.ID, pq.Array(groups), result.PelicanConfig.ModelID, kind, status, judgment, meta.ID, meta.SharedRoundID, meta.ExpectedCount, meta.ScheduledFor, nullableTime(result.StartedAt), nullableTime(result.FinishedAt), observed)
	return err
}
func (r *scheduledTestResultRepository) prunePelicanReportFacts(ctx context.Context, cutoff time.Time) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM pelican_report_pair_slots WHERE scheduled_for < $1`, cutoff); err != nil {
		return err
	}
	for {
		res, err := r.db.ExecContext(ctx, `DELETE FROM pelican_report_facts WHERE source_result_id IN
 (SELECT source_result_id FROM pelican_report_facts WHERE observed_at<$1 ORDER BY observed_at,source_result_id LIMIT 1000)`, cutoff)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n < 1000 {
			return nil
		}
	}
}

// All source facts, active public group metadata, and the matching public artwork are
// read from one database snapshot. No scheduled test service is reachable from this path.
func (r *pelicanShowcaseRepository) ReadReport(ctx context.Context, groupID int64, model string, allowed []int64, maxItems int, since, from, to time.Time) (*service.PelicanReportData, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	out := &service.PelicanReportData{ModelID: model, Facts: []service.PelicanReportFact{}}
	err = tx.QueryRowContext(ctx, `SELECT id,name,platform,rate_multiplier FROM groups WHERE id=$1 AND id=ANY($2) AND deleted_at IS NULL AND status=$3`, groupID, pq.Array(allowed), service.StatusActive).Scan(&out.Group.ID, &out.Group.Name, &out.Group.Platform, &out.Group.RateMultiplier)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if model == "" {
		err = tx.QueryRowContext(ctx, `SELECT model_id FROM pelican_report_facts WHERE $1=ANY(group_ids) AND observed_at >= $2 AND observed_at < $3 ORDER BY observed_at DESC,source_result_id DESC LIMIT 1`, groupID, to.Add(-48*time.Hour), to).Scan(&out.ModelID)
		if errors.Is(err, sql.ErrNoRows) {
			err = nil
		}
		if err != nil {
			return nil, err
		}
	}
	if out.ModelID != "" {
		// Statistics cover every observed result in the window. Only timeline rows
		// are bounded; a fast supported schedule must not lose counts or fail.
		stats, err := readPelicanReportStatistics(ctx, tx, groupID, out.ModelID, from, to)
		if err != nil {
			return nil, err
		}
		out.Statistics = stats
		// Fetch recent history independently from the latest retained execution.
		// A slow parallel result may have siblings outside the visible timeline.
		rows, err := tx.QueryContext(ctx, `WITH kinds(kind) AS (VALUES ('candy'), ('pelican')),
 recent AS (
  SELECT h.source_result_id FROM kinds k CROSS JOIN LATERAL (
   SELECT source_result_id FROM pelican_report_facts
   WHERE $1=ANY(group_ids) AND model_id=$2 AND kind=k.kind AND observed_at >= $3 AND observed_at < $4
   ORDER BY observed_at DESC,source_result_id DESC LIMIT $6
  ) h
 ), latest AS (
  SELECT l.source_result_id,l.execution_id FROM kinds k CROSS JOIN LATERAL (
   SELECT source_result_id,execution_id FROM pelican_report_facts
   WHERE $1=ANY(group_ids) AND model_id=$2 AND kind=k.kind AND observed_at >= $5 AND observed_at < $4
   ORDER BY observed_at DESC,source_result_id DESC LIMIT 1
  ) l
 )
 SELECT f.source_result_id,f.kind,f.model_id,COALESCE(f.execution_id,''),COALESCE(f.shared_round_id,''),COALESCE(f.expected_count,0),f.scheduled_for,f.status,f.judgment,f.started_at,f.completed_at,f.observed_at
 FROM pelican_report_facts f
 WHERE $1=ANY(f.group_ids) AND f.model_id=$2 AND f.observed_at >= $5 AND f.observed_at < $4
 AND (f.source_result_id IN (SELECT source_result_id FROM recent)
 OR f.source_result_id IN (SELECT source_result_id FROM latest)
 OR f.execution_id IN (SELECT execution_id FROM latest WHERE execution_id IS NOT NULL))
 ORDER BY f.observed_at,f.source_result_id`, groupID, out.ModelID, from, to, to.Add(-48*time.Hour), service.PelicanReportHistoryLimit)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var f service.PelicanReportFact
			if err = rows.Scan(&f.ResultID, &f.Kind, &f.ModelID, &f.ExecutionID, &f.SharedRoundID, &f.ExpectedCount, &f.ScheduledFor, &f.Status, &f.Judgment, &f.StartedAt, &f.CompletedAt, &f.ObservedAt); err != nil {
				_ = rows.Close()
				return nil, err
			}
			out.Facts = append(out.Facts, f)
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return nil, err
		}
		if len(out.Facts) > 2*service.PelicanReportHistoryLimit+16 {
			return nil, fmt.Errorf("invalid report execution result count")
		}
		var latest *service.PelicanReportFact
		for i := range out.Facts {
			if out.Facts[i].Kind == "pelican" {
				latest = &out.Facts[i]
			}
		}
		if latest != nil {
			art := &service.PelicanReportArtwork{}
			err = tx.QueryRowContext(ctx, `SELECT i.id,i.source_result_id,i.group_id,i.model_id,i.reasoning_effort,i.generated_at,i.response_text
 FROM pelican_showcase_items i JOIN pelican_report_facts f ON f.source_result_id=i.source_result_id
 WHERE i.group_id=$1 AND i.model_id=$2 AND f.kind='pelican' AND f.status='success' AND $1=ANY(f.group_ids)
 AND (f.source_result_id=$3 OR ($4<>'' AND f.execution_id=$4)) AND f.observed_at<$5
 AND ($6::timestamptz IS NULL OR i.generated_at >= $6)
 AND (SELECT COUNT(*) FROM pelican_showcase_items newer WHERE newer.group_id=i.group_id AND (newer.generated_at,newer.id)>(i.generated_at,i.id)) < $7
 ORDER BY f.observed_at DESC,i.id DESC LIMIT 1`, groupID, out.ModelID, latest.ResultID, latest.ExecutionID, to, nullableTime(since), maxItems).Scan(&art.ID, &art.SourceResultID, &art.GroupID, &art.ModelID, &art.ReasoningEffort, &art.GeneratedAt, &art.ResponseText)
			if err == nil {
				out.Artwork = art
			} else if !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func readPelicanReportStatistics(ctx context.Context, tx *sql.Tx, groupID int64, model string, from, to time.Time) (*service.PelicanReportStatistics, error) {
	rows, err := tx.QueryContext(ctx, `SELECT kind,
 COUNT(*) FILTER (WHERE status='success'),
 COUNT(*) FILTER (WHERE status IN ('success','failed')),
 COUNT(*) FILTER (WHERE status='failed'),
 COUNT(*) FILTER (WHERE status='ungraded'),
 COUNT(*),
 COUNT(*) FILTER (WHERE status<>'ungraded' AND started_at IS NOT NULL AND completed_at >= started_at),
 AVG(FLOOR(EXTRACT(EPOCH FROM (completed_at-started_at))*1000)) FILTER (WHERE status<>'ungraded' AND started_at IS NOT NULL AND completed_at >= started_at),
 CASE WHEN BOOL_OR(execution_id IS NULL) THEN NULL ELSE COUNT(DISTINCT execution_id) END
 FROM pelican_report_facts
 WHERE $1=ANY(group_ids) AND model_id=$2 AND observed_at >= $3 AND observed_at < $4
 GROUP BY kind`, groupID, model, from, to)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	zeroCandy, zeroPelican := int64(0), int64(0)
	out := &service.PelicanReportStatistics{Candy: service.PelicanReportStats{ExecutionCount: &zeroCandy}, Pelican: service.PelicanReportStats{ExecutionCount: &zeroPelican}}
	for rows.Next() {
		var kind string
		var stats service.PelicanReportStats
		if err := rows.Scan(&kind, &stats.SuccessCount, &stats.TotalCount, &stats.FailureCount, &stats.UngradedCount, &stats.ObservedCount, &stats.TimedCount, &stats.AvgLatencyMs, &stats.ExecutionCount); err != nil {
			return nil, err
		}
		if stats.TotalCount > 0 {
			rate := float64(stats.SuccessCount) * 100 / float64(stats.TotalCount)
			stats.SuccessRate = &rate
		}
		if kind == "candy" {
			out.Candy = stats
		} else if kind == "pelican" {
			out.Pelican = stats
		}
	}
	return out, rows.Err()
}
