package repository

import (
	"context"
	"database/sql"
	"errors"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

func (r *pelicanGroupTestRepository) SaveIntelligenceRule(ctx context.Context, key string, plans []*service.PelicanGroupTestPlan) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "intelligence-rule:"+key); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id, group_id, COALESCE(running_until > NOW(),false) FROM pelican_group_test_plans WHERE pelican_config->'intelligence'->>'id'=$1 FOR UPDATE`, key)
	if err != nil {
		return err
	}
	existing := map[int64]int64{}
	runningGroups := map[int64]bool{}
	for rows.Next() {
		var id, gid int64
		var running bool
		if err = rows.Scan(&id, &gid, &running); err != nil {
			rows.Close()
			return err
		}
		existing[gid] = id
		runningGroups[gid] = running
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	// Runs own a configuration snapshot. Updates affect only future claims and
	// must keep the lease and plan row so the current run can persist and finish.
	retained := map[int64]bool{}
	for _, p := range plans {
		retained[p.GroupID] = true
	}
	for gid, running := range runningGroups {
		if running && !retained[gid] {
			if len(plans) == 0 {
				return service.ErrPelicanGroupTestPlanRunning
			}
			return infraerrors.Conflict("INTELLIGENCE_GROUP_RUNNING", "正在检测的分组暂不能移除；可先暂停规则，等待本轮结束后移除")
		}
	}
	// Lock group rows in stable order to serialize overlapping rule saves, including first creation.
	sorted := append([]*service.PelicanGroupTestPlan(nil), plans...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].GroupID < sorted[j].GroupID })
	for _, p := range sorted {
		var gid int64
		if err = tx.QueryRowContext(ctx, `SELECT id FROM groups WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, p.GroupID).Scan(&gid); err != nil {
			return err
		}
		var overlap bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pelican_group_test_plans WHERE group_id=$1 AND model_id=$2 AND pelican_config->'intelligence'->>'id' IS NOT NULL AND pelican_config->'intelligence'->>'id' <> $3)`, p.GroupID, p.ModelID, key).Scan(&overlap); err != nil {
			return err
		}
		if overlap {
			return infraerrors.Conflict("INTELLIGENCE_RULE_OVERLAP", "同一分组和模型已有检测规则，请编辑现有规则")
		}
	}
	for _, p := range plans {
		if id, ok := existing[p.GroupID]; ok {
			_, err = tx.ExecContext(ctx, `UPDATE pelican_group_test_plans SET model_id=$2,cron_expression=$3,enabled=$4,pelican_config=$5,next_run_at=$6,updated_at=NOW() WHERE id=$1`, id, p.ModelID, p.CronExpression, p.Enabled, marshalPelicanConfig(p.PelicanConfig), p.NextRunAt)
			delete(existing, p.GroupID)
		} else {
			_, err = tx.ExecContext(ctx, `INSERT INTO pelican_group_test_plans(group_id,model_id,cron_expression,enabled,pelican_config,next_run_at) VALUES($1,$2,$3,$4,$5,$6)`, p.GroupID, p.ModelID, p.CronExpression, p.Enabled, marshalPelicanConfig(p.PelicanConfig), p.NextRunAt)
		}
		if err != nil {
			return err
		}
	}
	for _, id := range existing {
		if _, err = tx.ExecContext(ctx, `DELETE FROM pelican_group_test_plans WHERE id=$1`, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (r *pelicanGroupTestRepository) DeleteIntelligenceRule(ctx context.Context, key string) error {
	return r.SaveIntelligenceRule(ctx, key, nil)
}

// Window each group/model/kind independently; failure samples are retained alongside successes.
func (r *pelicanGroupTestRepository) ListIntelligenceResults(ctx context.Context, ids []int64, limit int, since time.Time) ([]*service.PelicanGroupTestResult, error) {
	rows, err := r.db.QueryContext(ctx, `WITH ranked AS (
 SELECT r.id,row_number() OVER(PARTITION BY p.group_id, COALESCE(r.pelican_config->>'model_id',p.model_id),COALESCE(r.pelican_config->>'question_kind','pelican') ORDER BY r.started_at DESC,r.id DESC) AS pos
 FROM pelican_group_test_results r JOIN pelican_group_test_plans p ON p.id=r.plan_id
 WHERE p.id=ANY($1) AND r.started_at >= $2
 ) `+pelicanGroupTestResultSelect+` FROM ranked n JOIN pelican_group_test_results r ON r.id=n.id JOIN pelican_group_test_plans p ON p.id=r.plan_id JOIN groups g ON g.id=p.group_id WHERE n.pos <= $3 ORDER BY r.started_at DESC,r.id DESC`, pq.Array(ids), since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*service.PelicanGroupTestResult{}
	for rows.Next() {
		result, err := scanPelicanGroupTestResult(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, result)
	}
	return out, rows.Err()
}

// Authorize and fetch only one body. The rank check uses its group/model/kind,
// avoiding a complete dashboard query for every preview on initial page load.
func (r *pelicanGroupTestRepository) GetIntelligenceResult(ctx context.Context, id int64, limit int, since time.Time) (*service.PelicanGroupTestResult, error) {
	row := r.db.QueryRowContext(ctx, pelicanGroupTestResultSelect+`,r.response_text
 FROM pelican_group_test_results r JOIN pelican_group_test_plans p ON p.id=r.plan_id JOIN groups g ON g.id=p.group_id
 WHERE r.id=$1 AND r.started_at >= $2 AND g.deleted_at IS NULL AND g.status='active'
 AND p.pelican_config->'intelligence'->>'id' IS NOT NULL
 AND (SELECT COUNT(*) FROM pelican_group_test_results n JOIN pelican_group_test_plans np ON np.id=n.plan_id
 WHERE np.group_id=p.group_id AND np.pelican_config->'intelligence'->>'id' IS NOT NULL
 AND COALESCE(n.pelican_config->>'model_id',np.model_id)=COALESCE(r.pelican_config->>'model_id',p.model_id)
 AND COALESCE(n.pelican_config->>'question_kind','pelican')=COALESCE(r.pelican_config->>'question_kind','pelican')
 AND (n.started_at,n.id)>(r.started_at,r.id)) < $3`, id, since, limit)
	result, err := scanPelicanGroupTestResult(row, true)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return result, err
}
