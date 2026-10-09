//go:build route_quality_integration

package repository

import (
	"context"
	"database/sql"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestQualityTrafficSnapshotsPostgres(t *testing.T) {
	dsn := os.Getenv("ROUTE_QUALITY_TEST_DSN")
	if dsn == "" {
		t.Skip("set ROUTE_QUALITY_TEST_DSN for isolated PostgreSQL verification")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	_, err = db.ExecContext(ctx, `CREATE TEMP TABLE usage_logs(id bigserial,account_id bigint,group_id bigint,created_at timestamptz,usage_completeness text);
 CREATE TEMP TABLE account_quality_traffic_events(id bigserial,account_id bigint,group_id bigint,plan_id bigint,observed_at timestamptz DEFAULT clock_timestamp(),degraded boolean);
 CREATE TEMP TABLE quality_rule_template_accounts(plan_id bigint,tested_group_id bigint);
 CREATE TEMP TABLE scheduled_test_plans(id bigint,account_id bigint);
 CREATE TEMP TABLE scheduled_test_results(id bigserial,plan_id bigint,quality_round_id text,finished_at timestamptz,status text,error_message text,pelican_config jsonb,quality_judgment jsonb,quality_action text);
 INSERT INTO quality_rule_template_accounts VALUES(11,7),(22,8),(33,NULL),(44,7),(55,7),(66,7);
 INSERT INTO scheduled_test_plans VALUES(44,44),(55,55),(66,66);
 INSERT INTO scheduled_test_results(plan_id,quality_round_id,finished_at,status,error_message,pelican_config,quality_judgment) VALUES
 (44,'wrong',NOW()-interval '2 minutes','failed','answer_mismatch','{"parallel_count":1,"quality":{}}','{"verdict":"incorrect"}'),
 (44,'unknown',NOW()-interval '1 minute','failed','judge_inconclusive','{"parallel_count":1,"quality":{}}','{"verdict":"unknown"}'),
 (55,'transport',NOW()-interval '1 minute','failed','timeout','{"parallel_count":1,"quality":{}}',NULL),
 (66,'wrong',NOW()-interval '2 minutes','failed','answer_mismatch','{"parallel_count":2,"quality":{}}','{"verdict":"incorrect"}'),
 (66,'partial-judgment',NOW()-interval '1 minute','success',NULL,'{"parallel_count":2,"quality":{}}','{"verdict":"correct"}'),
 (66,'partial-judgment',NOW()-interval '1 minute','success',NULL,'{"parallel_count":2,"quality":{}}',NULL);`)
	require.NoError(t, err)
	migration, err := migrations.FS.ReadFile("266_quality_traffic_snapshots.sql")
	require.NoError(t, err)
	apply := func() {
		t.Helper()
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer tx.Rollback()
		_, err = tx.ExecContext(ctx, string(migration))
		require.NoError(t, err)
		require.NoError(t, tx.Commit())
	}
	apply()
	var seeded int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM account_quality_traffic_events WHERE account_id=44 AND degraded`).Scan(&seeded))
	require.Equal(t, 1, seeded, "latest inconclusive result cannot erase the prior explicit degraded mark")
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM account_quality_traffic_events WHERE account_id=55`).Scan(&seeded))
	require.Zero(t, seeded, "transport failure cannot seed a degraded mark")
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM account_quality_traffic_events WHERE account_id=66 AND degraded`).Scan(&seeded))
	require.Equal(t, 1, seeded, "a missing judgment cannot count as an explicit pass and clear the mark")
	start := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	_, err = db.ExecContext(ctx, `INSERT INTO account_quality_traffic_events(account_id,group_id,plan_id,observed_at,degraded) VALUES(1,7,11,$1,true),(1,7,11,$2,false),(2,7,11,$1,true)`, start, start.Add(10*time.Minute))
	require.NoError(t, err)
	insert := func(account, group int64, dispatch any, completeness string) int64 {
		t.Helper()
		var id int64
		require.NoError(t, db.QueryRowContext(ctx, `INSERT INTO usage_logs(account_id,group_id,created_at,quality_request_started_at,usage_completeness) VALUES($1,$2,$3,$4,$5) RETURNING id`, account, group, start.Add(20*time.Minute), dispatch, completeness).Scan(&id))
		return id
	}
	status := func(id int64) sql.NullString {
		t.Helper()
		var status sql.NullString
		require.NoError(t, db.QueryRowContext(ctx, `SELECT quality_status FROM usage_logs WHERE id=$1`, id).Scan(&status))
		return status
	}
	before := insert(1, 7, start.Add(-time.Minute), "complete")
	during := insert(1, 7, start, "complete")
	after := insert(1, 7, start.Add(10*time.Minute), "complete")
	otherGroup := insert(1, 8, start.Add(time.Minute), "complete")
	legacy := insert(1, 7, nil, "complete")
	insert(1, 7, start.Add(time.Minute), "unknown")
	require.Equal(t, "unmarked", status(before).String, "later detection cannot relabel an earlier dispatch")
	require.Equal(t, "degraded", status(during).String, "SSE completed after recovery keeps dispatch-time state")
	require.Equal(t, "healthy", status(after).String)
	require.Equal(t, "unmarked", status(otherGroup).String, "group isolation")
	require.False(t, status(legacy).Valid, "legacy data is not fabricated as healthy")
	apply()
	require.Equal(t, "degraded", status(during).String, "migration rerun does not rewrite stored snapshots")
	points := []service.MonitorV4TimelinePoint{{GroupID: 7, Start: start, End: start.Add(time.Hour)}, {GroupID: 8, Start: start, End: start.Add(time.Hour)}}
	repo := &accountMonitorRepository{db: db}
	require.NoError(t, repo.readQualityTrafficCounts(ctx, []int64{7, 8}, start, start.Add(time.Hour), time.Hour, points))
	require.Equal(t, 4, points[0].UsageRequestCount)
	require.Equal(t, 1, points[0].DegradedRequestCount)
	require.Equal(t, 3, points[0].QualitySnapshotRequestCount)
	require.Equal(t, 1, points[1].UsageRequestCount)
	require.Zero(t, points[1].DegradedRequestCount)
	// Observation mode writes evidence without changing account scheduling/BPS.
	plan := &service.ScheduledTestPlan{ID: 11, AccountID: 9, QualityTrafficVerdict: "degraded"}
	write := func() {
		t.Helper()
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer tx.Rollback()
		require.NoError(t, recordQualityTrafficVerdict(ctx, tx, plan))
		require.NoError(t, tx.Commit())
	}
	write()
	write()
	var events int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM account_quality_traffic_events WHERE account_id=9`).Scan(&events))
	require.Equal(t, 1, events, "unchanged state does not append redundant events")
	plan.QualityTrafficVerdict = ""
	write()
	plan.QualityTrafficVerdict = "healthy"
	write()
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM account_quality_traffic_events WHERE account_id=9`).Scan(&events))
	require.Equal(t, 2, events, "inconclusive preserves the mark; healthy creates a recovery event")
	plan.ID = 33
	plan.QualityTrafficVerdict = "degraded"
	write()
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM account_quality_traffic_events WHERE account_id=9`).Scan(&events))
	require.Equal(t, 2, events, "ambiguous legacy group is not assigned to a guessed route")
}

func TestQualityTrafficUsageInsertPathsPostgres(t *testing.T) {
	dsn := os.Getenv("ROUTE_QUALITY_TEST_DSN")
	if dsn == "" {
		t.Skip("set ROUTE_QUALITY_TEST_DSN for isolated PostgreSQL verification")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	started := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	log := &service.UsageLog{UserID: 1, APIKeyID: 2, AccountID: 3, GroupID: func() *int64 { v := int64(7); return &v }(), RequestID: "single", Model: "gpt-5", QualityRequestStartedAt: &started, CreatedAt: started.Add(time.Minute)}
	prepared := prepareUsageLogInsert(log)
	query, _ := buildUsageLogBestEffortInsertQuery([]usageLogInsertPrepared{prepared})
	match := regexp.MustCompile(`(?s)INSERT INTO usage_logs \((.*?)\)`).FindStringSubmatch(query)
	require.Len(t, match, 2)
	columns := strings.Split(match[1], ",")
	require.Len(t, columns, len(usageLogInsertArgTypes))
	definitions := []string{"id BIGSERIAL PRIMARY KEY"}
	for i, column := range columns {
		definitions = append(definitions, strings.TrimSpace(column)+" "+usageLogInsertArgTypes[i])
	}
	_, err = db.ExecContext(ctx, "CREATE TEMP TABLE usage_logs("+strings.Join(definitions, ",")+",UNIQUE(request_id,api_key_id));"+`
 CREATE TEMP TABLE account_quality_traffic_events(id bigserial,account_id bigint,group_id bigint,plan_id bigint,observed_at timestamptz DEFAULT clock_timestamp(),degraded boolean);
 CREATE TEMP TABLE quality_rule_template_accounts(plan_id bigint,tested_group_id bigint);
 CREATE TEMP TABLE scheduled_test_plans(id bigint,account_id bigint);
 CREATE TEMP TABLE scheduled_test_results(id bigint,plan_id bigint,quality_round_id text,finished_at timestamptz,status text,error_message text,pelican_config jsonb,quality_judgment jsonb,quality_action text);`)
	require.NoError(t, err)
	migration, err := migrations.FS.ReadFile("266_quality_traffic_snapshots.sql")
	require.NoError(t, err)
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	_, err = db.ExecContext(ctx, `INSERT INTO account_quality_traffic_events(account_id,group_id,plan_id,observed_at,degraded) VALUES(3,7,1,$1,true)`, started.Add(-time.Minute))
	require.NoError(t, err)
	repo := &usageLogRepository{sql: db}
	inserted, err := repo.createSingle(ctx, db, log)
	require.NoError(t, err)
	require.True(t, inserted)
	log.RequestID = "no-result"
	require.NoError(t, execUsageLogInsertNoResult(ctx, db, prepareUsageLogInsert(log)))
	log.RequestID = "batch"
	prepared = prepareUsageLogInsert(log)
	key := usageLogBatchKey(log.RequestID, log.APIKeyID)
	query, args := buildUsageLogBatchInsertQuery([]string{key}, map[string]usageLogInsertPrepared{key: prepared})
	rows, err := db.QueryContext(ctx, query, args...)
	require.NoError(t, err)
	require.NoError(t, rows.Close())
	log.RequestID = "best-effort"
	prepared = prepareUsageLogInsert(log)
	query, args = buildUsageLogBestEffortInsertQuery([]usageLogInsertPrepared{prepared})
	rows, err = db.QueryContext(ctx, query, args...)
	require.NoError(t, err)
	require.NoError(t, rows.Close())
	var count int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_logs WHERE quality_status='degraded' AND quality_request_started_at=$1`, started).Scan(&count))
	require.Equal(t, 4, count, "all four native insert paths carry the dispatch timestamp and create a snapshot")
	// Idempotent retries must keep the original classification after recovery.
	_, err = db.ExecContext(ctx, `INSERT INTO account_quality_traffic_events(account_id,group_id,plan_id,observed_at,degraded) VALUES(3,7,1,$1,false)`, started.Add(time.Minute))
	require.NoError(t, err)
	newStarted := started.Add(2 * time.Minute)
	log.QualityRequestStartedAt = &newStarted
	log.RequestID = "single"
	inserted, err = repo.createSingle(ctx, db, log)
	require.NoError(t, err)
	require.False(t, inserted)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_logs WHERE quality_status='degraded'`).Scan(&count))
	require.Equal(t, 4, count)
}
