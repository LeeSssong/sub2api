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

func TestQualityModelRemovalAndRestoration(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name, mapping, extra string
		models               []string
		blocked              bool
	}{
		{"multiple", `{"model-a":"upstream-a","model-b":"upstream-b","model-c":"model-c"}`, `{}`, []string{"model-a", "model-b"}, false},
		{"last model", `{"model-a":"model-a"}`, `{}`, []string{"model-a"}, true},
		{"wildcard", `{"model-a":"model-a","*":"other"}`, `{}`, []string{"model-a"}, true},
		{"unrestricted", `{}`, `{}`, []string{"model-a"}, true},
		{"passthrough", `{"model-a":"model-a","model-c":"model-c"}`, `{"openai_passthrough":true}`, []string{"model-a"}, true},
		{"missing model", `{"model-c":"model-c"}`, `{}`, []string{"model-a"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var id int64
			require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO accounts(name,platform,type,status,schedulable,credentials,extra) VALUES('quality-model','openai','apikey','active',true,jsonb_build_object('model_mapping',$1::jsonb,'api_key','test-only'),$2::jsonb) RETURNING id`, tc.mapping, tc.extra).Scan(&id))
			t.Cleanup(func() {
				_, _ = integrationDB.ExecContext(ctx, `DELETE FROM scheduler_outbox WHERE account_id=$1`, id)
				_, _ = integrationDB.ExecContext(ctx, `DELETE FROM accounts WHERE id=$1`, id)
			})
			qraw, err := json.Marshal(map[string]any{"expected_answer": "21", "action": "remove_models", "remove_model_ids": tc.models, "auto_restore": true})
			require.NoError(t, err)
			var policy service.QualityPolicy
			require.NoError(t, json.Unmarshal(qraw, &policy))
			repo := NewScheduledTestPlanRepository(integrationDB)
			svc := service.NewScheduledTestService(repo, NewScheduledTestResultRepository(integrationDB))
			p, err := svc.CreatePlan(ctx, &service.ScheduledTestPlan{AccountID: id, ModelID: "model-a", CronExpression: "*/30 * * * *", Enabled: true, MaxResults: 100, PelicanConfig: &service.PelicanTestConfig{QuestionKind: "candy", Prompt: "question", ReasoningEffort: "high", ParallelCount: 1, Quality: &policy}})
			require.NoError(t, err)
			require.NoError(t, repo.TriggerQuality(ctx, p.ID))
			p, err = repo.GetByID(ctx, p.ID)
			require.NoError(t, err)
			var now time.Time
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&now))
			until := now.Add(15 * time.Minute)
			claimed, err := repo.ClaimPelican(ctx, p, now, until, now.Add(30*time.Minute))
			require.NoError(t, err)
			require.True(t, claimed)
			apply := func(outcome string) string {
				t.Helper()
				got, err := repo.ApplyQualityOutcome(ctx, p, until, outcome)
				require.NoError(t, err)
				return got
			}
			readAccount := func() *service.Account {
				t.Helper()
				var raw []byte
				var sched bool
				require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT credentials,schedulable FROM accounts WHERE id=$1`, id).Scan(&raw, &sched))
				var credentials map[string]any
				require.NoError(t, json.Unmarshal(raw, &credentials))
				require.True(t, sched)
				require.Equal(t, "test-only", credentials["api_key"])
				return &service.Account{Platform: "openai", Type: "apikey", Credentials: credentials}
			}
			require.Equal(t, "inconclusive", apply("inconclusive"))
			if tc.blocked {
				require.Equal(t, "model_removal_blocked", apply("failed"))
				var want map[string]any
				require.NoError(t, json.Unmarshal([]byte(tc.mapping), &want))
				require.Equal(t, want, readAccount().Credentials["model_mapping"])
				var states int
				require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM account_quality_states WHERE plan_id=$1`, p.ID).Scan(&states))
				require.Zero(t, states)
				return
			}
			require.Equal(t, "models_removed", apply("failed"))
			a := readAccount()
			require.False(t, a.IsModelSupported("model-a"))
			require.False(t, a.IsModelSupported("model-b"))
			require.True(t, a.IsModelSupported("model-c"))
			require.Equal(t, "already_quarantined", apply("failed"))
			// The next lease hydrates removed alias mappings for direct recovery probes.
			require.NoError(t, repo.FinishPelican(ctx, p.ID, until, time.Now()))
			require.NoError(t, repo.TriggerQuality(ctx, p.ID))
			p, err = repo.GetByID(ctx, p.ID)
			require.NoError(t, err)
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&now))
			until = now.Add(15 * time.Minute)
			claimed, err = repo.ClaimPelican(ctx, p, now, until, now.Add(30*time.Minute))
			require.NoError(t, err)
			require.True(t, claimed)
			require.Equal(t, map[string]string{"model-a": "upstream-a", "model-b": "upstream-b"}, p.PelicanConfig.Quality.ProbeModelMapping)

			// Unrelated native edits must survive a delta restore.
			_, err = integrationDB.ExecContext(ctx, `UPDATE accounts SET credentials=jsonb_set(credentials,'{model_mapping,model-d}','"upstream-d"'),updated_at=clock_timestamp() WHERE id=$1`, id)
			require.NoError(t, err)
			require.Equal(t, "restored", apply("passed"))
			a = readAccount()
			require.Equal(t, "upstream-a", a.GetMappedModel("model-a"))
			require.Equal(t, "upstream-b", a.GetMappedModel("model-b"))
			require.Equal(t, "upstream-d", a.GetMappedModel("model-d"))
			require.True(t, a.IsModelSupported("model-c"))
			require.Equal(t, "passed", apply("passed"))
			// A manual replacement of a removed entry is never overwritten.
			require.Equal(t, "models_removed", apply("failed"))
			_, err = integrationDB.ExecContext(ctx, `UPDATE accounts SET credentials=jsonb_set(credentials,'{model_mapping,model-a}','"manual-target"'),updated_at=clock_timestamp() WHERE id=$1`, id)
			require.NoError(t, err)
			require.Equal(t, "restore_conflict", apply("passed"))
			require.Equal(t, "manual-target", readAccount().GetMappedModel("model-a"))
			var events int
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM scheduler_outbox WHERE account_id=$1`, id).Scan(&events))
			require.Equal(t, 3, events)
		})
	}
}
