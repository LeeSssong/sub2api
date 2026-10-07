//go:build route_quality_integration

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

// Uses a dedicated local QA database and temporary tables. It cannot alter
// the application's tables, even if a developer supplies the wrong database.
func TestGradedCandyTimelinePostgres(t *testing.T) {
	dsn := os.Getenv("ROUTE_QUALITY_TEST_DSN")
	if dsn == "" {
		t.Skip("set ROUTE_QUALITY_TEST_DSN for isolated PostgreSQL verification")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	_, err = db.ExecContext(ctx, `CREATE TEMP TABLE quality_rule_templates(id bigint,account_filter jsonb);
 CREATE TEMP TABLE quality_rule_template_accounts(template_id bigint,plan_id bigint,tested_group_id bigint);
 CREATE TEMP TABLE scheduled_test_results(id bigserial,plan_id bigint,quality_round_id text,finished_at timestamptz,status text,error_message text,pelican_config jsonb,quality_judgment jsonb,response_text text);
 INSERT INTO quality_rule_templates VALUES(1,'{"group":"8"}'),(2,'{"group":"8"}');
 INSERT INTO quality_rule_template_accounts VALUES(1,11,7),(2,22,8),(1,33,NULL);`)
	require.NoError(t, err)
	start := time.Date(2026, 10, 6, 10, 0, 0, 0, time.UTC)
	end := start.Add(10 * time.Minute)
	insert := func(planID int64, round string, sample gradedCandySample, finished time.Time) {
		t.Helper()
		cfg, _ := json.Marshal(sample.result.PelicanConfig)
		judgment, _ := json.Marshal(sample.result.QualityJudgment)
		_, err := db.ExecContext(ctx, `INSERT INTO scheduled_test_results(plan_id,quality_round_id,finished_at,status,error_message,pelican_config,quality_judgment,response_text) VALUES($1,$2,$3,$4,$5,$6::jsonb,$7::jsonb,'answer')`, planID, round, finished, sample.result.Status, sample.result.ErrorMessage, string(cfg), string(judgment))
		require.NoError(t, err)
	}
	a := qualityTimelineSample("a", "correct")
	b := qualityTimelineSample("b", "correct")
	insert(11, "cross-start", a, start.Add(-time.Minute))
	insert(11, "cross-start", b, start.Add(time.Minute))
	wrong := qualityTimelineSample("b", "incorrect")
	wrong.result.Status = "failed"
	wrong.result.ErrorMessage = "answer_mismatch"
	insert(11, "bad", a, start.Add(2*time.Minute))
	insert(11, "bad", wrong, start.Add(3*time.Minute))
	timeout := b
	timeout.result.Status = "failed"
	timeout.result.ErrorMessage = "timeout"
	timeout.result.QualityJudgment = nil
	insert(11, "timeout", a, start.Add(2*time.Minute))
	insert(11, "timeout", timeout, start.Add(3*time.Minute))
	insert(11, "partial", a, start.Add(2*time.Minute))
	insert(11, "cross-end", a, end.Add(-time.Minute))
	insert(11, "cross-end", b, end.Add(time.Minute))
	insert(22, "other-group", a, start.Add(time.Minute))
	insert(22, "other-group", wrong, start.Add(2*time.Minute))
	insert(33, "unproven-history", a, start.Add(time.Minute))
	insert(33, "unproven-history", wrong, start.Add(2*time.Minute))
	points := []service.MonitorV4TimelinePoint{{GroupID: 7, Start: start, End: start.Add(5 * time.Minute), RequestCount: 10, SuccessCount: 9}, {GroupID: 7, Start: start.Add(5 * time.Minute), End: end}}
	repo := &accountMonitorRepository{db: db}
	require.NoError(t, repo.readGradedCandyRounds(ctx, []int64{7}, start, end, points))
	require.Equal(t, 2, points[0].GradedRoundCount)
	require.Equal(t, 1, points[0].SuspectedDegradedRoundCount)
	require.Equal(t, 0, points[1].GradedRoundCount)
	require.Equal(t, 10, points[0].RequestCount)
	require.Equal(t, 9, points[0].SuccessCount)
}

func TestQualityRouteSnapshotPostgres(t *testing.T) {
	dsn := os.Getenv("ROUTE_QUALITY_TEST_DSN")
	if dsn == "" {
		t.Skip("set ROUTE_QUALITY_TEST_DSN for isolated PostgreSQL verification")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	_, err = db.ExecContext(ctx, `CREATE TEMP TABLE quality_rule_templates(id bigint,account_filter jsonb,enabled boolean,updated_at timestamptz);
 CREATE TEMP TABLE quality_rule_template_accounts(template_id bigint,account_id bigint,plan_id bigint,created_at timestamptz DEFAULT NOW(),PRIMARY KEY(template_id,account_id));
 CREATE TEMP TABLE accounts(id bigint,deleted_at timestamptz);
 CREATE TEMP TABLE scheduled_test_plans(id bigserial,account_id bigint,model_id text,cron_expression text,enabled boolean,max_results int,auto_recover boolean,last_run_at timestamptz,next_run_at timestamptz,created_at timestamptz,updated_at timestamptz,pelican_config jsonb,running_until timestamptz);
 INSERT INTO quality_rule_templates VALUES(1,'{"group":"7"}',true,'2026-10-06 10:00Z'),(2,'{"group":"8"}',true,'2026-10-06 10:00Z'),(3,'{"group":"ungrouped"}',true,'2026-10-06 10:00Z'),(4,'{"group":"9999999999999999999"}',true,'2026-10-06 10:00Z');
 INSERT INTO quality_rule_template_accounts(template_id,account_id,created_at) VALUES(1,91,'2026-10-06 10:01Z'),(2,92,'2026-10-06 09:00Z'),(3,93,'2026-10-06 10:01Z'),(4,94,'2026-10-06 10:01Z');
 INSERT INTO accounts VALUES(1,NULL),(2,NULL),(3,NULL);`)
	require.NoError(t, err)
	migration, err := migrations.FS.ReadFile("264_quality_rule_tested_group.sql")
	require.NoError(t, err)
	applyMigration := func() {
		t.Helper()
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer tx.Rollback()
		_, err = tx.ExecContext(ctx, string(migration))
		require.NoError(t, err)
		require.NoError(t, tx.Commit())
	}
	applyMigration()
	groupFor := func(templateID, accountID int64) sql.NullInt64 {
		t.Helper()
		var group sql.NullInt64
		require.NoError(t, db.QueryRowContext(ctx, `SELECT tested_group_id FROM quality_rule_template_accounts WHERE template_id=$1 AND account_id=$2`, templateID, accountID).Scan(&group))
		return group
	}
	require.Equal(t, sql.NullInt64{Int64: 7, Valid: true}, groupFor(1, 91))
	require.False(t, groupFor(2, 92).Valid, "edited legacy link has no proven route")
	require.False(t, groupFor(3, 93).Valid)
	require.False(t, groupFor(4, 94).Valid, "invalid bigint filter must not break migration")

	repo := &qualityRuleTemplateRepository{db: db}
	template := &service.QualityRuleTemplate{ID: 1, UpdatedAt: time.Date(2026, 10, 6, 10, 0, 0, 0, time.UTC), AccountFilter: service.QualityRuleAccountFilter{Group: "8"}}
	_, err = repo.CreateLinkedPlan(ctx, template, &service.ScheduledTestPlan{AccountID: 1})
	require.NoError(t, err)
	require.Equal(t, sql.NullInt64{Int64: 7, Valid: true}, groupFor(1, 1), "snapshot uses locked DB filter, not caller input")
	require.NoError(t, db.QueryRowContext(ctx, `UPDATE quality_rule_templates SET account_filter='{"group":"8"}',updated_at=NOW() WHERE id=1 RETURNING updated_at`).Scan(&template.UpdatedAt))
	_, err = repo.CreateLinkedPlan(ctx, template, &service.ScheduledTestPlan{AccountID: 2})
	require.NoError(t, err)
	require.Equal(t, sql.NullInt64{Int64: 8, Valid: true}, groupFor(1, 2))
	require.Equal(t, sql.NullInt64{Int64: 7, Valid: true}, groupFor(1, 1), "editing does not move prior plans")
	applyMigration()
	require.Equal(t, sql.NullInt64{Int64: 7, Valid: true}, groupFor(1, 91), "rerunning migration preserves snapshot")
	require.False(t, groupFor(2, 92).Valid)
}
