package service

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestIntelligenceRuleValidation(t *testing.T) {
	svc, _ := newGroupTestService(newGroupTestRepoFake(), &groupTestRouterFake{}, nil)
	input := IntelligenceRuleInput{Name: "Daily", GroupIDs: []int64{4}, ModelID: "gpt-6-astra", ReasoningEffort: "medium", CronExpression: "*/30 * * * *", Enabled: true, CandyPrompt: CandyPrompt, ExpectedAnswer: "21", DrawingPrompt: IntelligenceDrawingPrompt, Actions: []string{"坐着雪橇"}, Scenes: []string{"竹林里"}}
	plans, err := svc.intelligencePlans(context.Background(), "rule-1", input)
	require.NoError(t, err)
	require.Len(t, plans, 1)
	require.Equal(t, QualityActionObserveOnly, plans[0].PelicanConfig.Intelligence.Candy.Quality.Action)
	input.CandyPrompt = "custom"
	_, err = svc.intelligencePlans(context.Background(), "rule-1", input)
	require.Error(t, err)
	input.CandyPrompt = CandyPrompt
	input.GroupIDs = []int64{4, 4}
	_, err = svc.intelligencePlans(context.Background(), "rule-1", input)
	require.Error(t, err)
	input.GroupIDs = []int64{4}
	input.DrawingPrompt = "missing placeholders"
	_, err = svc.intelligencePlans(context.Background(), "rule-1", input)
	require.Error(t, err)
}

func TestIntelligenceSamplesSeparateKindsAndSnapshots(t *testing.T) {
	router := &groupTestRouterFake{steps: []routeStep{{account: account(1, "a")}, {account: account(2, "b")}}}
	svc, _ := newGroupTestService(newGroupTestRepoFake(), router, func(_ context.Context, _ int64, _ string, cfg *PelicanTestConfig) (*ScheduledTestResult, error) {
		if cfg.QuestionKind == "candy" {
			return &ScheduledTestResult{Status: "success", ResponseText: "29"}, nil
		}
		require.Contains(t, cfg.Prompt, "坐着雪橇在竹林里")
		require.NotContains(t, cfg.Prompt, "{动作}")
		return &ScheduledTestResult{Status: "success", ResponseText: `<html><svg viewBox="0 0 10 10"></svg></html>`}, nil
	})
	plan := groupTestPlan(1)
	plan.PelicanConfig.Intelligence = &IntelligenceRuleConfig{ID: "r", Name: "rule", Candy: &PelicanTestConfig{QuestionKind: "candy", Prompt: CandyPrompt, Quality: &QualityPolicy{ExpectedAnswer: "21", Action: QualityActionObserveOnly}}, Actions: []string{"坐着雪橇"}, Scenes: []string{"竹林里"}}
	plan.PelicanConfig.Prompt = IntelligenceDrawingPrompt
	results := svc.runSamples(context.Background(), plan)
	require.Len(t, results, 1)
	require.Equal(t, "passed", intelligenceVerdict(results[0]))
	require.Equal(t, "pelican", results[0].PelicanConfig.QuestionKind)
	require.Contains(t, plan.PelicanConfig.Prompt, "{动作}")
	require.Equal(t, "坐着雪橇", results[0].PelicanConfig.IntelligenceResult.Action)
}

func TestIntelligencePublicResultDoesNotLeakAccountsOrErrors(t *testing.T) {
	r := &PelicanGroupTestResult{ID: 1, AccountID: 42, AccountName: "secret-account", ErrorMessage: "https://secret-upstream?token=secret", Status: "failed", Attempts: []PelicanGroupTestAttempt{{AccountID: 3, Error: "secret"}}, PelicanConfig: &PelicanTestConfig{QuestionKind: "candy", Prompt: "question", Quality: &QualityPolicy{ExpectedAnswer: "21", Judge: &QualityJudgeConfig{GroupID: 99, ModelID: "private"}}}}
	dto := intelligencePublicResult(r, false)
	raw, err := json.Marshal(dto)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "secret")
	require.NotContains(t, string(raw), "private")
	require.NotContains(t, string(raw), "account_id")
	require.Equal(t, "abnormal", dto.Verdict)
	r.ErrorMessage = "answer_mismatch"
	require.Equal(t, "incorrect", intelligenceVerdict(r))
	r.ErrorMessage = "judge_unknown"
	require.Equal(t, "abnormal", intelligenceVerdict(r))
}

type intelligenceReadRepo struct {
	*groupTestRepoFake
	history []*PelicanGroupTestResult
	limit   int
	since   time.Time
}

func (r *intelligenceReadRepo) ListIntelligenceResults(_ context.Context, _ []int64, limit int, since time.Time) ([]*PelicanGroupTestResult, error) {
	r.limit = limit
	r.since = since
	return r.history, nil
}
func TestIntelligenceDashboardVisibility(t *testing.T) {
	plan := groupTestPlan(1)
	plan.GroupStatus = StatusActive
	plan.PelicanConfig.Intelligence = &IntelligenceRuleConfig{ID: "r", Name: "rule"}
	result := &PelicanGroupTestResult{ID: 90, PlanID: plan.ID, GroupID: 4, Status: "failed", ErrorMessage: "upstream secret", StartedAt: groupTestNow, PelicanConfig: plan.PelicanConfig}
	base := newGroupTestRepoFake(plan)
	base.results = []*PelicanGroupTestResult{result}
	svc, _ := newGroupTestService(base, &groupTestRouterFake{}, nil)
	svc.repo = &intelligenceReadRepo{groupTestRepoFake: base, history: []*PelicanGroupTestResult{result}}
	view, err := svc.IntelligenceDashboard(context.Background())
	require.NoError(t, err)
	require.Len(t, view.Groups, 1)
	require.Len(t, view.Groups[0].Results, 1)
	dto, err := svc.IntelligenceResult(context.Background(), 90)
	require.NoError(t, err)
	require.Equal(t, "request_failed", dto.ErrorKind)
	_, err = svc.IntelligenceResult(context.Background(), 999)
	require.Error(t, err)
	plan.PelicanConfig.Intelligence = nil
	view, err = svc.IntelligenceDashboard(context.Background())
	require.NoError(t, err)
	require.Empty(t, view.Groups, "legacy plan details are not implicitly published")
	_, err = svc.IntelligenceResult(context.Background(), 90)
	require.Error(t, err)
	plan.PelicanConfig.Intelligence = &IntelligenceRuleConfig{ID: "r", Name: "rule"}
	svc.showcase.settings = &showcaseSettingsStub{runtime: PelicanShowcaseRuntime{Enabled: false}}
	view, err = svc.IntelligenceDashboard(context.Background())
	require.NoError(t, err)
	require.Len(t, view.Groups, 1, "legacy gallery switch does not gate intelligence dashboard")
	_, err = svc.IntelligenceResult(context.Background(), 90)
	require.NoError(t, err)
}

func TestIntelligenceDashboardThreeDayWindowAndSchedule(t *testing.T) {
	plan := groupTestPlan(1)
	plan.PelicanConfig.Intelligence = &IntelligenceRuleConfig{ID: "r"}
	next := groupTestNow.Add(30 * time.Minute)
	lease := groupTestNow.Add(10 * time.Minute)
	plan.NextRunAt = &next
	plan.RunningUntil = &lease
	base := newGroupTestRepoFake(plan)
	svc, _ := newGroupTestService(base, &groupTestRouterFake{}, nil)
	repo := &intelligenceReadRepo{groupTestRepoFake: base}
	svc.repo = repo
	view, err := svc.IntelligenceDashboard(context.Background())
	require.NoError(t, err)
	require.Equal(t, 512, repo.limit)
	require.Equal(t, groupTestNow.Add(-72*time.Hour), repo.since)
	require.Equal(t, next, *view.Groups[0].NextRunAt)
	require.Equal(t, lease.Add(-pelicanGroupTestLease), *view.Groups[0].RunningStartedAt)
	require.GreaterOrEqual(t, intelligenceHistoryKeep(plan), 576)
}

func (r *groupTestRepoFake) ResolveIntelligenceQualitySource(_ context.Context, groupID, templateID int64, models []string) (*QualityRuleTemplate, error) {
	return &QualityRuleTemplate{ID: 10, CronExpression: "*/5 * * * *"}, nil
}
func (r *groupTestRepoFake) SnapshotIntelligenceQuality(context.Context, *PelicanGroupTestPlan, time.Time) error {
	return nil
}
