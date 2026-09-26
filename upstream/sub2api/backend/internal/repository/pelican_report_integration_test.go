//go:build integration

package repository

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestPelicanReportAtomicFactsSurvivePruningAndFilterExactModel(t *testing.T) {
	ctx, plan, groups := statisticsFixture(t)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM pelican_report_facts WHERE group_ids && $1::bigint[]", pq.Array(groups))
	})
	repo := NewScheduledTestResultRepository(integrationDB)
	finished := time.Now().UTC().Truncate(time.Microsecond).Add(-time.Minute)
	model := "report-model"
	cfg := &service.PelicanTestConfig{Prompt: "draw a bird", ModelID: model, ParallelCount: 2}
	meta := &service.PelicanReportExecutionMeta{ID: "real-execution", ExpectedCount: 2, ScheduledFor: &finished}
	ids := []int64{}
	for _, status := range []string{"success", "failed"} {
		result, err := repo.Create(ctx, &service.ScheduledTestResult{PlanID: plan.ID, Status: status, ResponseText: "<svg></svg>", PelicanConfig: cfg, PelicanGroupIDs: groups, ReportExecution: meta, StartedAt: finished.Add(-3 * time.Second), FinishedAt: finished})
		require.NoError(t, err)
		ids = append(ids, result.ID)
	}
	gallery := NewPelicanShowcaseRepository(integrationDB).(*pelicanShowcaseRepository)
	got, err := gallery.ReadReport(ctx, groups[0], model, groups, 20, time.Time{}, finished.Add(-24*time.Hour), finished.Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, got.Facts, 2)
	require.NotNil(t, got.Group.RateMultiplier)
	require.Equal(t, meta.ID, got.Facts[0].ExecutionID)
	require.Equal(t, meta.ID, got.Facts[1].ExecutionID)
	require.NoError(t, repo.PruneOldResults(ctx, plan.ID, 0))
	got, err = gallery.ReadReport(ctx, groups[0], model, groups, 20, time.Time{}, finished.Add(-24*time.Hour), finished.Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, got.Facts, 2)
	got, err = gallery.ReadReport(ctx, groups[0], "different-model", groups, 20, time.Time{}, finished.Add(-24*time.Hour), finished.Add(time.Minute))
	require.NoError(t, err)
	require.Empty(t, got.Facts)
	got, err = gallery.ReadReport(ctx, groups[0], model, []int64{groups[1]}, 20, time.Time{}, finished.Add(-24*time.Hour), finished.Add(time.Minute))
	require.NoError(t, err)
	require.Nil(t, got)
	_, err = integrationDB.ExecContext(ctx, "UPDATE groups SET status='inactive' WHERE id=$1", groups[0])
	require.NoError(t, err)
	got, err = gallery.ReadReport(ctx, groups[0], model, groups, 20, time.Time{}, finished.Add(-24*time.Hour), finished.Add(time.Minute))
	require.NoError(t, err)
	require.Nil(t, got)
	// Independent retention does not disappear until a sample ages beyond 48 hours.
	require.NoError(t, repo.MaintainPelicanStatistics(ctx, finished.Add(47*time.Hour)))
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM pelican_report_facts WHERE source_result_id=ANY($1)", pq.Array(ids)).Scan(&count))
	require.Equal(t, 2, count)
	require.NoError(t, repo.MaintainPelicanStatistics(ctx, finished.Add(49*time.Hour)))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM pelican_report_facts WHERE source_result_id=ANY($1)", pq.Array(ids)).Scan(&count))
	require.Zero(t, count)
}

func TestPelicanReportCandyFactIsUngradedAndAtomic(t *testing.T) {
	ctx, plan, groups := statisticsFixture(t)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM pelican_report_facts WHERE group_ids && $1::bigint[]", pq.Array(groups))
	})
	repo := NewScheduledTestResultRepository(integrationDB)
	now := time.Now().UTC().Truncate(time.Microsecond)
	result := &service.ScheduledTestResult{PlanID: plan.ID, Status: "success", ResponseText: "29", PelicanConfig: &service.PelicanTestConfig{QuestionKind: "candy", Prompt: "custom", ModelID: "model"}, PelicanGroupIDs: groups, ReportExecution: &service.PelicanReportExecutionMeta{ID: "candy-execution", ExpectedCount: 1}, StartedAt: now.Add(-time.Second), FinishedAt: now}
	saved, err := repo.Create(ctx, result)
	require.NoError(t, err)
	var status string
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT status FROM pelican_report_facts WHERE source_result_id=$1", saved.ID).Scan(&status))
	require.Equal(t, "ungraded", status)
	// Invalid fact metadata rolls native history back as well.
	result.ReportExecution.ExpectedCount = 9
	_, err = repo.Create(ctx, result)
	require.Error(t, err)
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM scheduled_test_results WHERE plan_id=$1", plan.ID).Scan(&count))
	require.Equal(t, 1, count)
	// Replaying fact persistence is idempotent by the saved native source identity.
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	saved.ReportExecution.ExpectedCount = 1
	require.NoError(t, recordPelicanReportFact(ctx, tx, saved, groups))
	require.NoError(t, tx.Commit())
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM pelican_report_facts WHERE source_result_id=$1", saved.ID).Scan(&count))
	require.Equal(t, 1, count)
}

func TestPelicanReportArtworkUsesExactResultAndExistingGalleryVisibility(t *testing.T) {
	ctx, plan, groups := statisticsFixture(t)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM pelican_report_facts WHERE group_ids && $1::bigint[]", pq.Array(groups))
	})
	now := time.Now().UTC().Truncate(time.Microsecond).Add(-time.Minute)
	repo := NewScheduledTestResultRepository(integrationDB)
	gallery := NewPelicanShowcaseRepository(integrationDB).(*pelicanShowcaseRepository)
	result, err := repo.Create(ctx, &service.ScheduledTestResult{PlanID: plan.ID, Status: "success", ResponseText: "<svg></svg>", PelicanConfig: &service.PelicanTestConfig{Prompt: "draw", ModelID: "model"}, PelicanGroupIDs: groups, ReportExecution: &service.PelicanReportExecutionMeta{ID: "draw-exec", ExpectedCount: 1}, StartedAt: now.Add(-time.Second), FinishedAt: now})
	require.NoError(t, err)
	require.NoError(t, gallery.Publish(ctx, result, groups, 20))
	got, err := gallery.ReadReport(ctx, groups[0], "model", groups, 20, time.Time{}, now.Add(-24*time.Hour), now.Add(time.Minute))
	require.NoError(t, err)
	require.NotNil(t, got.Artwork)
	require.Equal(t, result.ID, got.Artwork.SourceResultID)
	_, err = gallery.Delete(ctx, got.Artwork.ID)
	require.NoError(t, err)
	got, err = gallery.ReadReport(ctx, groups[0], "model", groups, 20, time.Time{}, now.Add(-24*time.Hour), now.Add(time.Minute))
	require.NoError(t, err)
	require.Nil(t, got.Artwork)
}

func TestPelicanReportPairClaimPreservesOriginalDueTime(t *testing.T) {
	ctx, plan, groups := statisticsFixture(t)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM pelican_report_facts WHERE group_ids && $1::bigint[]", pq.Array(groups))
	})
	repo := NewScheduledTestPlanRepository(integrationDB)
	plan.PelicanConfig.ReportPairKey = "test-explicit-pair"
	plan.PelicanConfig.ModelID = plan.ModelID
	plan, err := repo.Update(ctx, plan)
	require.NoError(t, err)
	cfg := *plan.PelicanConfig
	cfg.Prompt = service.CandyPrompt
	cfg.QuestionKind = "candy"
	other, err := repo.Create(ctx, &service.ScheduledTestPlan{AccountID: plan.AccountID, ModelID: plan.ModelID, CronExpression: plan.CronExpression, Enabled: true, MaxResults: 5, NextRunAt: plan.NextRunAt, PelicanConfig: &cfg})
	require.NoError(t, err)
	due := *plan.NextRunAt
	now := time.Now()
	until := now.Add(time.Minute).Truncate(time.Microsecond)
	plan.ReportExecution = &service.PelicanReportExecutionMeta{ID: "draw-claim", ExpectedCount: 2, ScheduledFor: &due, Timezone: "UTC"}
	claimed, err := repo.ClaimPelican(ctx, plan, now, until, now.Add(time.Hour))
	require.NoError(t, err)
	require.True(t, claimed)
	require.NotEmpty(t, plan.ReportExecution.SharedRoundID)
	require.NoError(t, repo.FinishPelican(ctx, plan.ID, until, now))
	other.ReportExecution = &service.PelicanReportExecutionMeta{ID: "candy-claim", ExpectedCount: 2, ScheduledFor: &due, Timezone: "UTC"}
	claimed, err = repo.ClaimPelican(ctx, other, now, until, now.Add(time.Hour))
	require.NoError(t, err)
	require.True(t, claimed)
	require.Equal(t, plan.ReportExecution.SharedRoundID, other.ReportExecution.SharedRoundID)
	require.Equal(t, due, *other.ReportExecution.ScheduledFor)
}

func TestPelicanReportPairSlotRejectsChangedMembershipAndTrigger(t *testing.T) {
	for _, change := range []string{"third-member", "edited-member", "disabled-member", "timezone", "due", "model", "cron"} {
		t.Run(change, func(t *testing.T) {
			ctx, plan, _ := statisticsFixture(t)
			t.Cleanup(func() {
				_, _ = integrationDB.ExecContext(ctx, "DELETE FROM pelican_report_pair_slots WHERE account_id=$1", plan.AccountID)
			})
			repo := NewScheduledTestPlanRepository(integrationDB)
			plan.PelicanConfig.ReportPairKey = "explicit-pair"
			plan, err := repo.Update(ctx, plan)
			require.NoError(t, err)
			cfg := *plan.PelicanConfig
			cfg.Prompt, cfg.QuestionKind = service.CandyPrompt, "candy"
			other, err := repo.Create(ctx, &service.ScheduledTestPlan{AccountID: plan.AccountID, ModelID: plan.ModelID, CronExpression: plan.CronExpression, Enabled: true, MaxResults: 5, NextRunAt: plan.NextRunAt, PelicanConfig: &cfg})
			require.NoError(t, err)
			due := *plan.NextRunAt
			now := time.Now().Truncate(time.Microsecond)
			until := now.Add(time.Minute)
			plan.ReportExecution = &service.PelicanReportExecutionMeta{ID: "draw", ExpectedCount: 2, ScheduledFor: &due, Timezone: "UTC"}
			claimed, err := repo.ClaimPelican(ctx, plan, now, until, now.Add(time.Hour))
			require.NoError(t, err)
			require.True(t, claimed)
			require.NotEmpty(t, plan.ReportExecution.SharedRoundID)
			require.NoError(t, repo.FinishPelican(ctx, plan.ID, until, now))
			plan, err = repo.GetByID(ctx, plan.ID)
			require.NoError(t, err)
			zone := "UTC"
			switch change {
			case "third-member":
				_, err = repo.Create(ctx, &service.ScheduledTestPlan{AccountID: plan.AccountID, ModelID: plan.ModelID, CronExpression: plan.CronExpression, Enabled: true, MaxResults: 5, NextRunAt: &due, PelicanConfig: &cfg})
			case "edited-member":
				plan.PelicanConfig.ReasoningEffort = "high"
				_, err = repo.Update(ctx, plan)
			case "disabled-member":
				plan.Enabled = false
				_, err = repo.Update(ctx, plan)
			case "timezone":
				zone = "Asia/Shanghai"
			case "due":
				due = due.Add(-time.Minute)
				other.NextRunAt = &due
				other, err = repo.Update(ctx, other)
			case "model":
				other.ModelID = "another-model"
				other, err = repo.Update(ctx, other)
			case "cron":
				other.CronExpression = "*/7 * * * *"
				other, err = repo.Update(ctx, other)
			}
			require.NoError(t, err)
			other.ReportExecution = &service.PelicanReportExecutionMeta{ID: "candy", ExpectedCount: 2, ScheduledFor: &due, Timezone: zone}
			claimed, err = repo.ClaimPelican(ctx, other, now, until, now.Add(time.Hour))
			require.NoError(t, err)
			require.True(t, claimed)
			require.Empty(t, other.ReportExecution.SharedRoundID, "a stale slot must not pair a changed plan set or trigger")
		})
	}
}

func TestPelicanReportClaimRejectsChangedOriginalDueTime(t *testing.T) {
	ctx, plan, _ := statisticsFixture(t)
	now := time.Now().Truncate(time.Microsecond)
	_, err := integrationDB.ExecContext(ctx, "UPDATE scheduled_test_plans SET next_run_at=$2 WHERE id=$1", plan.ID, now.Add(-2*time.Minute))
	require.NoError(t, err)
	claimed, err := NewScheduledTestPlanRepository(integrationDB).ClaimPelican(ctx, plan, now, now.Add(time.Minute), now.Add(time.Hour))
	require.NoError(t, err)
	require.False(t, claimed, "a stale due-list entry must not claim a different logical trigger")
}

func TestPelicanReportLargeWindowKeepsAllCountsAndBoundsHistory(t *testing.T) {
	ctx, _, groups := statisticsFixture(t)
	const baseID int64 = 9000000000
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM pelican_report_facts WHERE group_ids && $1::bigint[]", pq.Array(groups))
	})
	now := time.Now().UTC().Truncate(time.Second)
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO pelican_report_facts
 (source_result_id,group_ids,model_id,kind,status,judgment,execution_id,expected_count,started_at,completed_at,observed_at)
 SELECT $1::bigint+n,$2::bigint[],'busy-model','pelican',CASE WHEN n%10=0 THEN 'failed' ELSE 'success' END,'drawing',
 CASE WHEN n=1 THEN NULL ELSE 'busy-execution-'||n END,CASE WHEN n=1 THEN NULL ELSE 1 END,
 $3::timestamptz-(11001-n)*INTERVAL '1 second'-INTERVAL '1.5009 seconds',
 $3::timestamptz-(11001-n)*INTERVAL '1 second',$3::timestamptz-(11001-n)*INTERVAL '1 second'
 FROM generate_series(1,11000) n`, baseID, pq.Array(groups), now)
	require.NoError(t, err)
	gallery := NewPelicanShowcaseRepository(integrationDB).(*pelicanShowcaseRepository)
	got, err := gallery.ReadReport(ctx, groups[0], "busy-model", groups, 20, time.Time{}, now.Add(-24*time.Hour), now)
	require.NoError(t, err)
	require.NotNil(t, got.Statistics)
	require.Equal(t, int64(11000), got.Statistics.Pelican.TotalCount)
	require.Equal(t, int64(9900), got.Statistics.Pelican.SuccessCount)
	require.Equal(t, int64(1100), got.Statistics.Pelican.FailureCount)
	require.Equal(t, int64(11000), got.Statistics.Pelican.TimedCount)
	require.Equal(t, 1500.0, *got.Statistics.Pelican.AvgLatencyMs)
	require.Nil(t, got.Statistics.Pelican.ExecutionCount, "one unknown historical execution keeps batch count unknown")
	require.Len(t, got.Facts, 240)
	require.Equal(t, baseID+10761, got.Facts[0].ResultID)
	require.Equal(t, baseID+11000, got.Facts[len(got.Facts)-1].ResultID)
	require.Equal(t, "busy-execution-11000", got.Facts[len(got.Facts)-1].ExecutionID)
	// A sibling outside the recent timeline must still complete the latest execution.
	_, err = integrationDB.ExecContext(ctx, "UPDATE pelican_report_facts SET execution_id='busy-execution-11000',expected_count=2 WHERE source_result_id=ANY($1)", pq.Array([]int64{baseID + 1, baseID + 11000}))
	require.NoError(t, err)
	got, err = gallery.ReadReport(ctx, groups[0], "busy-model", groups, 20, time.Time{}, now.Add(-24*time.Hour), now)
	require.NoError(t, err)
	require.Equal(t, int64(11000), got.Statistics.Pelican.TotalCount)
	require.Len(t, got.Facts, 241)
	require.Equal(t, baseID+1, got.Facts[0].ResultID)
	require.Equal(t, got.Facts[0].ExecutionID, got.Facts[len(got.Facts)-1].ExecutionID)
}
