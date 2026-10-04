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
			_, err = repo.CreateResult(ctx, &service.PelicanGroupTestResult{PlanID: ids[0], Status: "failed", ErrorMessage: "answer_mismatch", StartedAt: now, FinishedAt: now, PelicanConfig: &service.PelicanTestConfig{QuestionKind: kind, ModelID: "model", IntelligenceResult: &service.IntelligenceResultMetadata{Action: "雪橇"}}})
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
		r, e := repo.CreateResult(ctx, &service.PelicanGroupTestResult{PlanID: p.ID, Status: "success", ResponseText: "21", StartedAt: now.Add(time.Duration(i) * time.Second), FinishedAt: now, PelicanConfig: &service.PelicanTestConfig{QuestionKind: "candy", ModelID: p.ModelID}})
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
	_, err = integrationDB.ExecContext(ctx, `UPDATE groups SET status='disabled' WHERE id=$1`, g.ID)
	require.NoError(t, err)
	result, err = repo.GetIntelligenceResult(ctx, oldest, 3, now.Add(-time.Hour))
	require.NoError(t, err)
	require.Nil(t, result)
}
