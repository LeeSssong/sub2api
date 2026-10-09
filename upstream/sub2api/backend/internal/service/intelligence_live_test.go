package service

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestIntelligenceLiveScheduleIndependentKinds(t *testing.T) {
	for _, tc := range []struct {
		name, now, candyNext, drawingNext string
		wantCandy, wantDrawing            int
		wantNext                          string
	}{
		{"candy only", "2026-10-09T10:05:00Z", "2026-10-09T10:05:00Z", "2026-10-09T10:30:00Z", 2, 0, "2026-10-09T10:10:00Z"},
		{"drawing only", "2026-10-09T10:30:00Z", "2026-10-09T11:05:00Z", "2026-10-09T10:30:00Z", 0, 1, "2026-10-09T11:00:00Z"},
		{"both overdue", "2026-10-09T10:32:00Z", "2026-10-09T10:25:00Z", "2026-10-09T10:30:00Z", 2, 1, "2026-10-09T10:35:00Z"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now, err := time.Parse(time.RFC3339, tc.now)
			require.NoError(t, err)
			p := groupTestPlan(1)
			raw := `{"id":"live","candy_cron_expression":"*/5 * * * *","candy_next_run_at":"` + tc.candyNext + `","drawing_next_run_at":"` + tc.drawingNext + `","actions":["划船"],"scenes":["海边"]}`
			require.NoError(t, json.Unmarshal([]byte(raw), &p.PelicanConfig.Intelligence))
			p.PelicanConfig.Intelligence.Candy = &PelicanTestConfig{QuestionKind: "candy", Prompt: CandyPrompt, ReasoningEffort: "high", Quality: &QualityPolicy{ExpectedAnswer: "21", Action: QualityActionObserveOnly}}
			repo := newGroupTestRepoFake(p)
			router := &groupTestRouterFake{steps: []routeStep{{account: account(1, "a")}, {account: account(2, "b")}, {account: account(3, "c")}}}
			svc, _ := newGroupTestService(repo, router, func(_ context.Context, _ int64, _ string, cfg *PelicanTestConfig) (*ScheduledTestResult, error) {
				if cfg.QuestionKind == "candy" {
					return &ScheduledTestResult{Status: "success", ResponseText: "21个。"}, nil
				}
				return &ScheduledTestResult{Status: "success", ResponseText: "<svg></svg>"}, nil
			})
			svc.now = func() time.Time { return now }
			svc.runScheduled(context.Background(), p, now)
			candy, drawing := 0, 0
			for _, r := range repo.results {
				if r.PelicanConfig.QuestionKind == "candy" {
					candy++
				} else {
					drawing++
				}
			}
			require.Equal(t, tc.wantCandy, candy)
			require.Equal(t, tc.wantDrawing, drawing)
			require.Len(t, repo.claims, 1)
			require.Equal(t, tc.wantNext, repo.claims[0].next.UTC().Format(time.RFC3339))
			require.Empty(t, p.PelicanConfig.Intelligence.Candy.ModelID, "execution must not mutate the saved candy config")
		})
	}
}

func TestIntelligenceLiveRuleSchedules(t *testing.T) {
	svc, _ := newGroupTestService(newGroupTestRepoFake(), &groupTestRouterFake{}, nil)
	svc.groups = groupTestGroupsFake{4: {ID: 4, Status: StatusActive}, 5: {ID: 5, Status: StatusActive}}
	var input IntelligenceRuleInput
	require.NoError(t, json.Unmarshal([]byte(`{"name":"Live","group_ids":[4,5],"model_id":"draw","reasoning_effort":"medium","enabled":true,"drawing_prompt":"draw {动作} {场景}","actions":["a"],"scenes":["s"],"candy_schedules":[{"group_id":4,"cron_expression":"*/5 * * * *"},{"group_id":5,"cron_expression":"15 * * * *"}]}`), &input))
	plans, err := svc.intelligencePlans(context.Background(), "live", input)
	require.NoError(t, err)
	require.Len(t, plans, 2)
	require.Equal(t, groupTestNow.Add(5*time.Minute), *plans[0].NextRunAt)
	require.Equal(t, groupTestNow.Add(15*time.Minute), *plans[1].NextRunAt)
	require.NotNil(t, plans[0].PelicanConfig.Intelligence.Candy)
	require.Equal(t, "high", plans[0].PelicanConfig.Intelligence.Candy.ReasoningEffort)
	for _, raw := range []string{
		`[{"group_id":4,"cron_expression":"not cron"}]`,
		`[{"group_id":4,"cron_expression":"0 0 31 2 *"}]`,
		`[{"group_id":99,"cron_expression":"*/5 * * * *"}]`,
		`[{"group_id":4,"cron_expression":"*/5 * * * *"},{"group_id":4,"cron_expression":"*/10 * * * *"}]`,
	} {
		require.NoError(t, json.Unmarshal([]byte(`{"candy_schedules":`+raw+`}`), &input))
		_, err = svc.intelligencePlans(context.Background(), "live", input)
		require.Error(t, err, raw)
	}
}

// This repository fails if a dashboard refresh tries to materialize quality data.
type liveReadOnlyRepo struct{ *intelligenceReadRepo }

func (r *liveReadOnlyRepo) SnapshotIntelligenceQuality(context.Context, *PelicanGroupTestPlan, time.Time) error {
	panic("dashboard wrote a quality snapshot")
}
func (r *liveReadOnlyRepo) BackfillIntelligenceQuality(context.Context, *PelicanGroupTestPlan, time.Time) error {
	panic("dashboard backfilled quality results")
}
func (r *liveReadOnlyRepo) ResolveIntelligenceQualitySource(context.Context, int64, int64, []string) (*QualityRuleTemplate, error) {
	panic("dashboard depends on quality source")
}
func TestIntelligenceLiveDashboardIsReadOnly(t *testing.T) {
	p := groupTestPlan(1)
	p.PelicanConfig.Intelligence = &IntelligenceRuleConfig{ID: "live", CandyCronExpression: "0 0 * * *"}
	base := newGroupTestRepoFake(p)
	svc, _ := newGroupTestService(base, nil, nil)
	svc.repo = &liveReadOnlyRepo{&intelligenceReadRepo{groupTestRepoFake: base, history: []*PelicanGroupTestResult{
		{ID: 1, GroupID: 4, Status: "success", StartedAt: groupTestNow, PelicanConfig: &PelicanTestConfig{QuestionKind: "candy", ModelID: "gpt-6-astra"}},
	}}}
	view, err := svc.IntelligenceDashboard(context.Background())
	require.NoError(t, err)
	require.Equal(t, "21", view.Groups[0].ExpectedAnswer)
	require.Equal(t, "0 0 * * *", view.Groups[0].CandyCronExpression)
	require.Equal(t, "2026-09-29T00:10:00Z", view.Groups[0].Results[0].ValidUntil.UTC().Format(time.RFC3339))
}

func TestIntelligenceLiveManualRunDoesNotMoveCursors(t *testing.T) {
	p := groupTestPlan(1)
	candyNext, drawingNext := groupTestNow.Add(5*time.Minute), groupTestNow.Add(30*time.Minute)
	p.PelicanConfig.Intelligence = &IntelligenceRuleConfig{ID: "live", CandyNextRunAt: &candyNext, DrawingNextRunAt: &drawingNext}
	base := newGroupTestRepoFake(p)
	router := &groupTestRouterFake{steps: []routeStep{{account: account(1, "a")}, {account: account(2, "b")}, {account: account(3, "c")}}}
	svc, _ := newGroupTestService(base, router, func(_ context.Context, _ int64, _ string, cfg *PelicanTestConfig) (*ScheduledTestResult, error) {
		text := "<svg></svg>"
		if cfg.QuestionKind == "candy" {
			text = "21"
		}
		return &ScheduledTestResult{Status: "success", ResponseText: text}, nil
	})
	require.NoError(t, svc.RunNow(context.Background(), p.ID))
	svc.runs.Wait()
	require.Len(t, base.results, 3)
	require.Nil(t, base.claims[0].next)
	require.Equal(t, candyNext, *p.PelicanConfig.Intelligence.CandyNextRunAt)
	require.Equal(t, drawingNext, *p.PelicanConfig.Intelligence.DrawingNextRunAt)
}
