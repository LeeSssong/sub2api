//go:build integration

package repository

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestIntelligenceRuleAtomicFanoutAndTimeline(t *testing.T) {
	ctx := context.Background()
	groups := createPelicanGroupTestGroups(t, "iq-a", "iq-b")
	repo := NewPelicanGroupTestRepository(integrationDB).(*pelicanGroupTestRepository)
	plans := []*service.PelicanGroupTestPlan{}
	key := "iq-" + time.Now().Format("150405.000000000")
	for _, g := range groups {
		p := newPelicanGroupTestPlan(g.ID, true, time.Now())
		p.PelicanConfig.Intelligence = &service.IntelligenceRuleConfig{ID: key, Name: "rule"}
		plans = append(plans, p)
	}
	require.NoError(t, repo.SaveIntelligenceRule(ctx, key, plans))
	all, err := repo.ListPlans(ctx)
	require.NoError(t, err)
	ids := []int64{}
	for _, p := range all {
		if p.PelicanConfig.Intelligence != nil && p.PelicanConfig.Intelligence.ID == key {
			ids = append(ids, p.ID)
		}
	}
	require.Len(t, ids, 2)
	overlap := *plans[0]
	oc := *overlap.PelicanConfig
	oc.Intelligence = &service.IntelligenceRuleConfig{ID: key + "-other", Name: "other"}
	overlap.PelicanConfig = &oc
	require.Error(t, repo.SaveIntelligenceRule(ctx, key+"-other", []*service.PelicanGroupTestPlan{&overlap}), "same group/model cannot mix different rules")
	invalid := *plans[1]
	invalid.GroupID = 9223372036854775000
	plans[0].ModelID = "should-rollback"
	require.Error(t, repo.SaveIntelligenceRule(ctx, key, []*service.PelicanGroupTestPlan{plans[0], &invalid}))
	original, err := repo.GetPlan(ctx, ids[0])
	require.NoError(t, err)
	require.NotEqual(t, "should-rollback", original.ModelID)
	now := time.Now()
	for _, kind := range []string{"candy", "pelican"} {
		for i := 0; i < 4; i++ {
			_, err = repo.CreateResult(ctx, &service.PelicanGroupTestResult{PlanID: ids[0], Status: "failed", ErrorMessage: "answer_mismatch", StartedAt: now, FinishedAt: now, PelicanConfig: &service.PelicanTestConfig{QuestionKind: kind, ModelID: "model", IntelligenceResult: &service.IntelligenceResultMetadata{Action: "雪橇", Source: "quality_ops"}}})
			require.NoError(t, err)
		}
	}
	results, err := repo.ListIntelligenceResults(ctx, ids, 2, now.Add(-time.Minute))
	require.NoError(t, err)
	require.Len(t, results, 4)
	require.Equal(t, "雪橇", results[0].PelicanConfig.IntelligenceResult.Action)
	_, err = integrationDB.ExecContext(ctx, `UPDATE pelican_group_test_plans SET running_until=NOW()+INTERVAL '1 minute' WHERE id=$1`, ids[0])
	require.NoError(t, err)
	require.ErrorIs(t, repo.DeleteIntelligenceRule(ctx, key), service.ErrPelicanGroupTestPlanRunning)
	_, err = integrationDB.ExecContext(ctx, `UPDATE pelican_group_test_plans SET running_until=NULL WHERE id=$1`, ids[0])
	require.NoError(t, err)
	require.NoError(t, repo.DeleteIntelligenceRule(ctx, key))
	p, err := repo.GetPlan(ctx, ids[0])
	require.NoError(t, err)
	require.Nil(t, p)
}

func TestIntelligenceSingleResultVisibility(t *testing.T) {
	ctx := context.Background()
	g := createPelicanGroupTestGroups(t, "iq-detail")[0]
	repo := NewPelicanGroupTestRepository(integrationDB).(*pelicanGroupTestRepository)
	p := newPelicanGroupTestPlan(g.ID, true, time.Now())
	p.PelicanConfig.Intelligence = &service.IntelligenceRuleConfig{ID: "detail", Name: "detail"}
	p, err := repo.CreatePlan(ctx, p)
	require.NoError(t, err)
	var oldest int64
	now := time.Now()
	for i := 0; i < 3; i++ {
		r, e := repo.CreateResult(ctx, &service.PelicanGroupTestResult{PlanID: p.ID, Status: "success", ResponseText: "21", StartedAt: now.Add(time.Duration(i) * time.Second), FinishedAt: now, PelicanConfig: &service.PelicanTestConfig{QuestionKind: "candy", ModelID: p.ModelID, IntelligenceResult: &service.IntelligenceResultMetadata{Source: "quality_ops"}}})
		require.NoError(t, e)
		if i == 0 {
			oldest = r.ID
		}
	}
	result, err := repo.GetIntelligenceResult(ctx, oldest, 2, now.Add(-time.Hour))
	require.NoError(t, err)
	require.Nil(t, result)
	result, err = repo.GetIntelligenceResult(ctx, oldest, 3, now.Add(-time.Hour))
	require.NoError(t, err)
	require.Equal(t, "21", result.ResponseText)
	require.NotNil(t, result)
	legacy, e := repo.CreateResult(ctx, &service.PelicanGroupTestResult{PlanID: p.ID, Status: "success", StartedAt: now, FinishedAt: now, PelicanConfig: &service.PelicanTestConfig{QuestionKind: "candy", ModelID: p.ModelID}})
	require.NoError(t, e)
	hidden, e := repo.GetIntelligenceResult(ctx, legacy.ID, 512, now.Add(-time.Hour))
	require.NoError(t, e)
	require.Nil(t, hidden, "independent legacy candy is not exposed as quality ops")

	_, err = integrationDB.ExecContext(ctx, `UPDATE groups SET status='disabled' WHERE id=$1`, g.ID)
	require.NoError(t, err)
	result, err = repo.GetIntelligenceResult(ctx, oldest, 3, now.Add(-time.Hour))
	require.NoError(t, err)
	require.Nil(t, result)
}

func TestIntelligenceRuleEditAndPauseDuringRun(t *testing.T) {
	ctx := context.Background()
	repo := NewPelicanGroupTestRepository(integrationDB).(*pelicanGroupTestRepository)
	g := createPelicanGroupTestGroups(t, "iq-live-edit")[0]
	now := time.Now().Truncate(time.Microsecond)
	key := "iq-live-edit-" + now.Format("150405.000000000")
	input := newPelicanGroupTestPlan(g.ID, true, now.Add(-time.Minute))
	input.PelicanConfig.Intelligence = &service.IntelligenceRuleConfig{ID: key, Name: "original"}
	require.NoError(t, repo.SaveIntelligenceRule(ctx, key, []*service.PelicanGroupTestPlan{input}))
	listed, err := repo.ListDue(ctx, now)
	require.NoError(t, err)
	var snapshot *service.PelicanGroupTestPlan
	for _, p := range listed {
		if p.GroupID == g.ID {
			snapshot = p
		}
	}
	require.NotNil(t, snapshot)
	lease, next := now.Add(15*time.Minute), now.Add(30*time.Minute)
	claimed, err := repo.Claim(ctx, snapshot, now, lease, &next)
	require.NoError(t, err)
	require.True(t, claimed)

	input.ModelID = "gpt-6-sol"
	input.CronExpression = "0 * * * *"
	input.PelicanConfig.Prompt = "updated drawing prompt"
	input.NextRunAt = &next
	require.NoError(t, repo.SaveIntelligenceRule(ctx, key, []*service.PelicanGroupTestPlan{input}), "running rules can be edited")
	updated, err := repo.GetPlan(ctx, snapshot.ID)
	require.NoError(t, err)
	require.Equal(t, "gpt-6-sol", updated.ModelID)
	require.Equal(t, lease.UnixMicro(), updated.RunningUntil.UnixMicro(), "edit keeps the existing run lease")
	require.Equal(t, "gpt-6-astra", snapshot.ModelID)
	require.Equal(t, "draw a pelican", snapshot.PelicanConfig.Prompt, "current run keeps its original configuration")
	claimed, err = repo.Claim(ctx, updated, now, lease.Add(time.Second), nil)
	require.NoError(t, err)
	require.False(t, claimed, "editing cannot start a duplicate run")

	input.Enabled = false
	require.NoError(t, repo.SaveIntelligenceRule(ctx, key, []*service.PelicanGroupTestPlan{input}), "running rules can be paused")
	paused, err := repo.GetPlan(ctx, snapshot.ID)
	require.NoError(t, err)
	require.False(t, paused.Enabled)
	require.Equal(t, lease.UnixMicro(), paused.RunningUntil.UnixMicro())
	require.ErrorIs(t, repo.DeleteIntelligenceRule(ctx, key), service.ErrPelicanGroupTestPlanRunning)
	_, err = repo.CreateResult(ctx, &service.PelicanGroupTestResult{PlanID: snapshot.ID, Status: "success", StartedAt: now, FinishedAt: now, PelicanConfig: snapshot.PelicanConfig})
	require.NoError(t, err, "current results can still be recorded after edits and pause")
	require.NoError(t, repo.Finish(ctx, snapshot.ID, lease, now.Add(time.Minute)))
	paused, err = repo.GetPlan(ctx, snapshot.ID)
	require.NoError(t, err)
	require.Nil(t, paused.RunningUntil)
	require.Equal(t, "gpt-6-sol", paused.ModelID)
	require.Equal(t, "updated drawing prompt", paused.PelicanConfig.Prompt)
	require.Equal(t, next.UnixMicro(), paused.NextRunAt.UnixMicro(), "finish must not overwrite the new schedule")
	due, err := repo.ListDue(ctx, next.Add(time.Minute))
	require.NoError(t, err)
	for _, p := range due {
		require.NotEqual(t, snapshot.ID, p.ID, "paused rule has no future scheduled run")
	}

	input.Enabled = true
	require.NoError(t, repo.SaveIntelligenceRule(ctx, key, []*service.PelicanGroupTestPlan{input}))
	claimed, err = repo.Claim(ctx, snapshot, next, next.Add(15*time.Minute), nil)
	require.NoError(t, err)
	require.False(t, claimed, "a stale manual request cannot run old configuration")
	claimed, err = repo.Claim(ctx, snapshot, next, next.Add(15*time.Minute), &next)
	require.NoError(t, err)
	require.False(t, claimed, "a stale scheduler snapshot cannot run old configuration")
	resumed, err := repo.GetPlan(ctx, snapshot.ID)
	require.NoError(t, err)
	claimed, err = repo.Claim(ctx, resumed, next, next.Add(15*time.Minute), &next)
	require.NoError(t, err)
	require.True(t, claimed)
	require.Equal(t, "updated drawing prompt", resumed.PelicanConfig.Prompt)
}

func TestIntelligenceRuleReplacesLegacySchedule(t *testing.T) {
	ctx := context.Background()
	repo := NewPelicanGroupTestRepository(integrationDB).(*pelicanGroupTestRepository)
	g := createPelicanGroupTestGroups(t, "iq-legacy")[0]
	now := time.Now().Truncate(time.Microsecond)
	legacy, err := repo.CreatePlan(ctx, newPelicanGroupTestPlan(g.ID, true, now.Add(-time.Minute)))
	require.NoError(t, err)
	otherModel := newPelicanGroupTestPlan(g.ID, true, now)
	otherModel.ModelID = "other-model"
	otherModel, err = repo.CreatePlan(ctx, otherModel)
	require.NoError(t, err)
	history, err := repo.CreateResult(ctx, &service.PelicanGroupTestResult{PlanID: legacy.ID, Status: "success", ResponseText: "legacy drawing", StartedAt: now, FinishedAt: now})
	require.NoError(t, err)
	input := newPelicanGroupTestPlan(g.ID, false, now)
	input.PelicanConfig.Intelligence = &service.IntelligenceRuleConfig{ID: "replace-legacy", Name: "rule"}
	require.NoError(t, repo.SaveIntelligenceRule(ctx, "replace-legacy", []*service.PelicanGroupTestPlan{input}))
	unchanged, err := repo.GetPlan(ctx, legacy.ID)
	require.NoError(t, err)
	require.False(t, unchanged.Enabled, "saving even a paused rule retires the legacy schedule")
	input.Enabled = true
	require.NoError(t, repo.SaveIntelligenceRule(ctx, "replace-legacy", []*service.PelicanGroupTestPlan{input}))
	paused, err := repo.GetPlan(ctx, legacy.ID)
	require.NoError(t, err)
	require.False(t, paused.Enabled, "enabling the dual-question rule pauses the matching legacy plan")
	require.True(t, paused.UpdatedAt.After(legacy.UpdatedAt), "stale scheduled snapshots are invalidated")
	preserved, err := repo.GetResult(ctx, history.ID)
	require.NoError(t, err)
	require.Equal(t, "legacy drawing", preserved.ResponseText)
	other, err := repo.GetPlan(ctx, otherModel.ID)
	require.NoError(t, err)
	require.True(t, other.Enabled, "a different model keeps its schedule")
	legacy.Enabled = true
	_, err = repo.UpdatePlan(ctx, legacy)
	require.Error(t, err, "the legacy endpoint cannot re-enable the conflicting plan")
	_, err = repo.CreatePlan(ctx, newPelicanGroupTestPlan(g.ID, true, now))
	require.Error(t, err, "the legacy endpoint cannot create another enabled plan")
	claimed, err := repo.Claim(ctx, paused, now, now.Add(15*time.Minute), nil)
	require.Error(t, err, "manual runs cannot bypass the active intelligence rule")
	require.False(t, claimed)
}

func TestIntelligenceRuleLegacyTakeoverIsAtomic(t *testing.T) {
	ctx := context.Background()
	repo := NewPelicanGroupTestRepository(integrationDB).(*pelicanGroupTestRepository)
	groups := createPelicanGroupTestGroups(t, "iq-atomic-a", "iq-atomic-b")
	now := time.Now().Truncate(time.Microsecond)
	legacy := make([]*service.PelicanGroupTestPlan, 2)
	inputs := make([]*service.PelicanGroupTestPlan, 2)
	for i, g := range groups {
		var err error
		legacy[i], err = repo.CreatePlan(ctx, newPelicanGroupTestPlan(g.ID, true, now))
		require.NoError(t, err)
		inputs[i] = newPelicanGroupTestPlan(g.ID, true, now)
		inputs[i].PelicanConfig.Intelligence = &service.IntelligenceRuleConfig{ID: "atomic-legacy", Name: "rule"}
	}
	lease := now.Add(15 * time.Minute)
	claimed, err := repo.Claim(ctx, legacy[1], now, lease, nil)
	require.NoError(t, err)
	require.True(t, claimed)
	require.Error(t, repo.SaveIntelligenceRule(ctx, "atomic-legacy", inputs), "a running legacy plan must finish before takeover")
	for _, p := range legacy {
		original, err := repo.GetPlan(ctx, p.ID)
		require.NoError(t, err)
		require.True(t, original.Enabled, "a conflict rolls back every group's pause")
	}
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM pelican_group_test_plans WHERE pelican_config->'intelligence'->>'id'='atomic-legacy'`).Scan(&count))
	require.Zero(t, count, "failed takeover does not create any part of the rule")
	require.NoError(t, repo.Finish(ctx, legacy[1].ID, lease, now))
	require.NoError(t, repo.SaveIntelligenceRule(ctx, "atomic-legacy", inputs))
	for _, p := range legacy {
		paused, err := repo.GetPlan(ctx, p.ID)
		require.NoError(t, err)
		require.False(t, paused.Enabled)
	}
}

func TestPelicanLegacyClaimRejectsExistingIntelligenceOverlap(t *testing.T) {
	ctx := context.Background()
	repo := NewPelicanGroupTestRepository(integrationDB).(*pelicanGroupTestRepository)
	g := createPelicanGroupTestGroups(t, "iq-existing-overlap")[0]
	now := time.Now().Truncate(time.Microsecond)
	legacy, err := repo.CreatePlan(ctx, newPelicanGroupTestPlan(g.ID, true, now.Add(-time.Minute)))
	require.NoError(t, err)
	// Reproduce existing data from before the fix without using the new save path.
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO pelican_group_test_plans(group_id,model_id,enabled,pelican_config) VALUES($1,$2,true,'{"intelligence":{"id":"existing-rule"}}')`, g.ID, legacy.ModelID)
	require.NoError(t, err)
	for _, scheduled := range []bool{false, true} {
		var next *time.Time
		if scheduled {
			n := now.Add(30 * time.Minute)
			next = &n
		}
		claimed, err := repo.Claim(ctx, legacy, now, now.Add(15*time.Minute), next)
		require.Error(t, err, "existing overlap cannot issue another legacy request")
		require.False(t, claimed)
	}
}

func TestIntelligenceRuleSerializesLegacyActivation(t *testing.T) {
	for _, operation := range []string{"create", "enable", "claim"} {
		t.Run(operation, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			repo := NewPelicanGroupTestRepository(integrationDB).(*pelicanGroupTestRepository)
			g := createPelicanGroupTestGroups(t, "iq-race-"+operation)[0]
			now := time.Now().Truncate(time.Microsecond)
			legacy, err := repo.CreatePlan(ctx, newPelicanGroupTestPlan(g.ID, operation == "claim", now.Add(-time.Minute)))
			require.NoError(t, err)
			input := newPelicanGroupTestPlan(g.ID, true, now)
			key := "iq-race-" + operation
			input.PelicanConfig.Intelligence = &service.IntelligenceRuleConfig{ID: key, Name: "rule"}
			start := make(chan struct{})
			ruleDone := make(chan error, 1)
			legacyDone := make(chan error, 1)
			go func() {
				<-start
				ruleDone <- repo.SaveIntelligenceRule(ctx, key, []*service.PelicanGroupTestPlan{input})
			}()
			go func() {
				<-start
				var err error
				switch operation {
				case "create":
					_, err = repo.CreatePlan(ctx, newPelicanGroupTestPlan(g.ID, true, now))
				case "enable":
					legacy.Enabled = true
					_, err = repo.UpdatePlan(ctx, legacy)
				case "claim":
					next := now.Add(30 * time.Minute)
					_, err = repo.Claim(ctx, legacy, now, now.Add(15*time.Minute), &next)
				}
				legacyDone <- err
			}()
			close(start)
			ruleErr, legacyErr := <-ruleDone, <-legacyDone
			if operation == "claim" && ruleErr != nil {
				require.NoError(t, legacyErr, "only a successful claim may block takeover")
			} else {
				require.NoError(t, ruleErr)
			}
			var enabled int
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM pelican_group_test_plans WHERE group_id=$1 AND model_id=$2 AND enabled=true`, g.ID, input.ModelID).Scan(&enabled))
			require.Equal(t, 1, enabled, "concurrent operations leave exactly one enabled schedule")
		})
	}
}
