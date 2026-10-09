//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestIntelligenceQualitySnapshotPostgres(t *testing.T) {
	ctx := context.Background()
	// Dedicated temporary tables keep the fixture independent of unrelated schemas.
	integrationDB.SetMaxOpenConns(1)
	defer integrationDB.SetMaxOpenConns(0)
	_, err := integrationDB.ExecContext(ctx, `CREATE TEMP TABLE quality_rule_templates(id bigint,model_id text,cron_expression text,pelican_config jsonb,enabled boolean,account_filter jsonb,updated_at timestamptz);
 CREATE TEMP TABLE quality_rule_template_accounts(template_id bigint,account_id bigint,plan_id bigint,tested_group_id bigint);
 CREATE TEMP TABLE scheduled_test_results(id bigserial,plan_id bigint,quality_round_id text,status text,response_text text,error_message text,latency_ms bigint,started_at timestamptz,finished_at timestamptz,pelican_config jsonb,quality_judgment jsonb);
 CREATE TEMP TABLE pelican_group_test_results(id bigserial,plan_id bigint,account_name text,attempts jsonb,status text,response_text text,error_message text,latency_ms bigint,pelican_config jsonb,started_at timestamptz,finished_at timestamptz,cost_usd numeric,cost_incomplete boolean);
 INSERT INTO quality_rule_templates VALUES(10,'gpt-6-astra','*/5 * * * *','{"question_kind":"candy","model_ids":["gpt-6-astra","gpt-6.1-sol"],"quality":{"expected_answer":"21"}}',true,'{"group":"2"}',NOW());
 INSERT INTO quality_rule_template_accounts VALUES(10,1,11,2),(10,2,22,6);`)
	require.NoError(t, err)
	defer integrationDB.ExecContext(ctx, `DROP TABLE pg_temp.quality_rule_templates,pg_temp.quality_rule_template_accounts,pg_temp.scheduled_test_results,pg_temp.pelican_group_test_results`)
	slot := time.Date(2026, 10, 9, 10, 30, 0, 0, time.UTC)
	insert := func(planID int64, model, round, status, verdict string, started, finished time.Time) {
		t.Helper()
		cfg, _ := json.Marshal(&service.PelicanTestConfig{QuestionKind: "candy", ModelID: model, ReasoningEffort: "high", Prompt: "question", Quality: &service.QualityPolicy{ExpectedAnswer: "21"}})
		j, _ := json.Marshal(&service.QualityJudgment{Verdict: verdict})
		_, e := integrationDB.ExecContext(ctx, `INSERT INTO scheduled_test_results(plan_id,quality_round_id,status,response_text,error_message,latency_ms,started_at,finished_at,pelican_config,quality_judgment) VALUES($1,$2,$3,'answer','private?token=secret',4,$4,$5,$6,$7)`, planID, round, status, started, finished, string(cfg), string(j))
		require.NoError(t, e)
	}
	for _, model := range []string{"gpt-6-astra", "gpt-6.1-sol"} {
		insert(11, model, "old", "success", "correct", slot.Add(-15*time.Minute), slot.Add(-11*time.Minute))
		insert(22, model, "wrong-group", "success", "correct", slot.Add(-5*time.Minute), slot.Add(-time.Minute))
	}
	insert(11, "gpt-6-astra", "current", "failed", "incorrect", slot.Add(-5*time.Minute), slot.Add(-time.Minute))
	insert(11, "gpt-6.1-sol", "current", "failed", "unknown", slot.Add(-5*time.Minute), slot.Add(-time.Minute))
	repo := &pelicanGroupTestRepository{db: integrationDB}
	plan := &service.PelicanGroupTestPlan{ID: 9, GroupID: 2, PelicanConfig: &service.PelicanTestConfig{Intelligence: &service.IntelligenceRuleConfig{QualityTemplateID: 10}}}
	copies := make(chan error, 4)
	for i := 0; i < 4; i++ {
		go func() { copies <- repo.SnapshotIntelligenceQuality(ctx, plan, slot) }()
	}
	for i := 0; i < 4; i++ {
		require.NoError(t, <-copies)
	}
	backfillPlan := *plan
	backfillPlan.ID = 99
	require.NoError(t, repo.BackfillIntelligenceQuality(ctx, &backfillPlan, slot))
	var historyCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM pelican_group_test_results WHERE plan_id=99`).Scan(&historyCount))
	require.Equal(t, 2, historyCount, "first use backfills retained source slots without a drawing request")
	_, err = integrationDB.ExecContext(ctx, `DELETE FROM pelican_group_test_results WHERE plan_id=99`)
	require.NoError(t, err)
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM pelican_group_test_results`).Scan(&count))
	require.Equal(t, 2, count)
	var snapshot []byte
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT pelican_config FROM pelican_group_test_results WHERE pelican_config->>'model_id'='gpt-6-astra'`).Scan(&snapshot))
	var cfg service.PelicanTestConfig
	require.NoError(t, json.Unmarshal(snapshot, &cfg))
	require.Equal(t, "incorrect", cfg.IntelligenceResult.Verdict)
	require.Equal(t, int64(10), cfg.IntelligenceResult.SourceTemplateID)
	require.Equal(t, slot.Add(-time.Minute), *cfg.IntelligenceResult.SourceFinishedAt)
	require.NotContains(t, string(snapshot), "secret")
	var verdict string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT pelican_config->'intelligence_result'->>'quality_verdict' FROM pelican_group_test_results WHERE pelican_config->>'model_id'='gpt-6.1-sol'`).Scan(&verdict))
	require.Equal(t, "unknown", verdict)
	_, err = integrationDB.ExecContext(ctx, `DELETE FROM scheduled_test_results`)
	require.NoError(t, err)
	require.NoError(t, repo.SnapshotIntelligenceQuality(ctx, plan, slot))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM pelican_group_test_results`).Scan(&count))
	require.Equal(t, 2, count, "source pruning and refresh retain the exact original snapshots")
	require.NoError(t, repo.SnapshotIntelligenceQuality(ctx, plan, slot.Add(30*time.Minute)))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM pelican_group_test_results`).Scan(&count))
	require.Equal(t, 2, count, "missing source must not produce invented results")
	_, err = repo.ResolveIntelligenceQualitySource(ctx, 6, 10, []string{"gpt-6-astra"})
	require.Error(t, err)
}
