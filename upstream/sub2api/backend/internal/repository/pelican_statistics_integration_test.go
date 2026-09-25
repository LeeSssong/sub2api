//go:build integration

package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func statisticsFixture(t *testing.T) (context.Context, *service.ScheduledTestPlan, []int64) {
	t.Helper()
	ctx := context.Background()
	client := testEntClient(t)
	a := mustCreateAccount(t, client, &service.Account{Name: "statistics-account"})
	ga := mustCreateGroup(t, client, &service.Group{Name: "statistics-a"})
	gb := mustCreateGroup(t, client, &service.Group{Name: "statistics-b"})
	ids := []int64{ga.ID, gb.ID}
	for _, id := range ids {
		mustBindAccountToGroup(t, client, a.ID, id, 1)
	}
	next := time.Now().Add(-time.Minute)
	plan, err := NewScheduledTestPlanRepository(integrationDB).Create(ctx, &service.ScheduledTestPlan{
		AccountID: a.ID, ModelID: "test-model", CronExpression: "*/3 * * * *", Enabled: true, MaxResults: 1, NextRunAt: &next,
		PelicanConfig: &service.PelicanTestConfig{Prompt: "draw a bird", ParallelCount: 2},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM pelican_drawing_outcomes WHERE group_ids && $1::bigint[]", pq.Array(ids))
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM accounts WHERE id = $1", a.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM groups WHERE id = ANY($1)", pq.Array(ids))
	})
	return ctx, plan, ids
}

func TestPelicanStatisticsAtomicOutcomeSurvivesRawAndPlanDeletion(t *testing.T) {
	ctx, plan, groups := statisticsFixture(t)
	resultRepo := NewScheduledTestResultRepository(integrationDB)
	finished := time.Now().Truncate(time.Microsecond)
	result, err := resultRepo.Create(ctx, &service.ScheduledTestResult{
		PlanID: plan.ID, Status: "success", StartedAt: finished.Add(-time.Minute), FinishedAt: finished,
		ResponseText: "<svg></svg>", PelicanConfig: plan.PelicanConfig,
	})
	require.NoError(t, err)
	var ids pq.Int64Array
	var succeeded bool
	var completed time.Time
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT group_ids, success, completed_at FROM pelican_drawing_outcomes WHERE source_result_id = $1", result.ID).Scan(&ids, &succeeded, &completed))
	require.ElementsMatch(t, groups, []int64(ids))
	require.True(t, succeeded)
	require.True(t, finished.Equal(completed))
	_, err = integrationDB.ExecContext(ctx, "UPDATE scheduled_test_results SET created_at = $2 WHERE id = $1", result.ID, finished.Add(-8*24*time.Hour))
	require.NoError(t, err)
	require.NoError(t, resultRepo.PruneExpiredPelican(ctx, finished.Add(-7*24*time.Hour)))
	var remaining int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM scheduled_test_results WHERE id = $1", result.ID).Scan(&remaining))
	require.Zero(t, remaining)
	require.NoError(t, resultRepo.PruneOldResults(ctx, plan.ID, 0))
	require.NoError(t, NewScheduledTestPlanRepository(integrationDB).Delete(ctx, plan.ID))
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM pelican_drawing_outcomes WHERE source_result_id = $1", result.ID).Scan(&count))
	require.Equal(t, 1, count)
	_, err = resultRepo.Create(ctx, &service.ScheduledTestResult{PlanID: plan.ID, Status: "failed",
		StartedAt: finished, FinishedAt: finished.Add(time.Second), PelicanConfig: plan.PelicanConfig, PelicanGroupIDs: groups})
	require.Error(t, err, "a deleted native plan cannot create a result or orphan outcome")
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM pelican_drawing_outcomes WHERE group_ids && $1::bigint[]", pq.Array(groups)).Scan(&count))
	require.Equal(t, 1, count)
}

func TestPelicanStatisticsOutcomeFailureRollsBackNativeResult(t *testing.T) {
	ctx, plan, _ := statisticsFixture(t)
	_, err := integrationDB.ExecContext(ctx, "ALTER TABLE pelican_drawing_outcomes ADD CONSTRAINT statistics_test_reject CHECK (false) NOT VALID")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "ALTER TABLE pelican_drawing_outcomes DROP CONSTRAINT statistics_test_reject")
	})
	finished := time.Now()
	_, err = NewScheduledTestResultRepository(integrationDB).Create(ctx, &service.ScheduledTestResult{
		PlanID: plan.ID, Status: "failed", StartedAt: finished.Add(-time.Second), FinishedAt: finished, PelicanConfig: plan.PelicanConfig,
	})
	require.Error(t, err)
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM scheduled_test_results WHERE plan_id = $1", plan.ID).Scan(&count))
	require.Zero(t, count, "statistics and raw result share the same transaction")
}

func TestPelicanStatisticsEligibilityUsesResultSnapshot(t *testing.T) {
	ctx, plan, _ := statisticsFixture(t)
	results := NewScheduledTestResultRepository(integrationDB)
	now := time.Now()
	cases := []struct {
		name     string
		cfg      *service.PelicanTestConfig
		status   string
		finished time.Time
		want     int
	}{
		{"drawing success", &service.PelicanTestConfig{QuestionKind: "pelican"}, "success", now, 1},
		{"legacy drawing failure", &service.PelicanTestConfig{}, "failed", now, 1},
		{"connection result", nil, "success", now, 0},
		{"candy", &service.PelicanTestConfig{QuestionKind: "candy"}, "success", now, 0},
		{"legacy candy", &service.PelicanTestConfig{Prompt: service.CandyPrompt}, "failed", now, 0},
		{"quality", &service.PelicanTestConfig{Quality: &service.QualityPolicy{}}, "success", now, 0},
		{"pending", &service.PelicanTestConfig{}, "pending", now, 0},
		{"unknown status", &service.PelicanTestConfig{}, "error", now, 0},
		{"unknown kind", &service.PelicanTestConfig{QuestionKind: "unknown"}, "failed", now, 0},
		{"unfinished", &service.PelicanTestConfig{}, "failed", time.Time{}, 0},
	}
	// The current plan can be edited into candy; the saved result snapshot is authoritative.
	_, err := integrationDB.ExecContext(ctx, "UPDATE scheduled_test_plans SET pelican_config = $2 WHERE id = $1", plan.ID, marshalPelicanConfig(&service.PelicanTestConfig{QuestionKind: "candy"}))
	require.NoError(t, err)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			saved, err := results.Create(ctx, &service.ScheduledTestResult{PlanID: plan.ID, Status: tc.status, StartedAt: now.Add(-time.Second), FinishedAt: tc.finished, PelicanConfig: tc.cfg})
			require.NoError(t, err)
			var count int
			require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM pelican_drawing_outcomes WHERE source_result_id = $1", saved.ID).Scan(&count))
			require.Equal(t, tc.want, count)
		})
	}
}

func resetStatisticsCoverage(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	var old time.Time
	err := integrationDB.QueryRowContext(ctx, "SELECT coverage_started_at FROM pelican_drawing_statistics_state WHERE id = 1").Scan(&old)
	require.True(t, err == nil || err == sql.ErrNoRows)
	_, err = integrationDB.ExecContext(ctx, "DELETE FROM pelican_drawing_statistics_state")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM pelican_drawing_statistics_state")
		if !old.IsZero() {
			_, _ = integrationDB.ExecContext(ctx, "INSERT INTO pelican_drawing_statistics_state VALUES (1, $1)", old)
		}
	})
}

func TestPelicanStatisticsWindowDeduplicationVisibilityAndHistory(t *testing.T) {
	ctx, plan, ids := statisticsFixture(t)
	resetStatisticsCoverage(t)
	results := NewScheduledTestResultRepository(integrationDB)
	reader := NewPelicanShowcaseRepository(integrationDB).(service.PelicanShowcaseStatisticsRepository)
	now := time.Now().Truncate(time.Microsecond)
	from := now.Add(-24 * time.Hour)
	require.NoError(t, results.MaintainPelicanStatistics(ctx, now.Add(-25*time.Hour)))
	save := func(status string, at time.Time, groups []int64) *service.ScheduledTestResult {
		t.Helper()
		result, err := results.Create(ctx, &service.ScheduledTestResult{PlanID: plan.ID, Status: status,
			StartedAt: at.Add(-time.Minute), FinishedAt: at, PelicanConfig: plan.PelicanConfig, PelicanGroupIDs: groups})
		require.NoError(t, err)
		return result
	}
	first := save("success", from, []int64{ids[0], ids[0], ids[1]})
	first.ResponseText = "<svg></svg>"
	require.NoError(t, NewPelicanShowcaseRepository(integrationDB).Publish(ctx, first, ids, 10))
	var artworkCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM pelican_showcase_items WHERE source_result_id = $1", first.ID).Scan(&artworkCount))
	require.Equal(t, 2, artworkCount)
	save("failed", now.Add(-time.Hour), []int64{ids[0]})
	save("success", now.Add(-2*time.Hour), []int64{ids[1]})
	save("failed", from.Add(-time.Microsecond), ids)
	save("failed", now, ids)
	save("success", now.Add(time.Microsecond), ids)
	save("failed", now.Add(-time.Hour), []int64{})
	// Repeating a write for one source id neither inflates counts nor relabels its groups.
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, recordPelicanDrawingOutcome(ctx, tx, first, []int64{ids[1]}))
	require.NoError(t, tx.Commit())
	stats, err := reader.ReadStatistics(ctx, ids, from, now)
	require.NoError(t, err)
	require.Equal(t, service.PelicanShowcaseCounts{SuccessCount: 2, TotalCount: 3}, stats.Total)
	require.Equal(t, service.PelicanShowcaseCounts{SuccessCount: 1, TotalCount: 2}, stats.Groups[ids[0]])
	require.Equal(t, service.PelicanShowcaseCounts{SuccessCount: 2, TotalCount: 2}, stats.Groups[ids[1]])

	// Changed membership and all three independent deletion paths preserve the same history.
	_, err = integrationDB.ExecContext(ctx, "DELETE FROM account_groups WHERE account_id = $1", plan.AccountID)
	require.NoError(t, err)
	require.NoError(t, results.PruneOldResults(ctx, plan.ID, 0))
	require.NoError(t, NewPelicanShowcaseRepository(integrationDB).Prune(ctx, []int64{}, 1, now))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM pelican_showcase_items WHERE source_result_id = $1", first.ID).Scan(&artworkCount))
	require.Zero(t, artworkCount)
	require.NoError(t, NewScheduledTestPlanRepository(integrationDB).Delete(ctx, plan.ID))
	again, err := reader.ReadStatistics(ctx, ids, from, now)
	require.NoError(t, err)
	require.Equal(t, stats, again)

	stats, err = reader.ReadStatistics(ctx, []int64{ids[0]}, from, now)
	require.NoError(t, err)
	require.Equal(t, service.PelicanShowcaseCounts{SuccessCount: 1, TotalCount: 2}, stats.Total)
	_, err = integrationDB.ExecContext(ctx, "UPDATE groups SET status = 'disabled' WHERE id = $1", ids[0])
	require.NoError(t, err)
	stats, err = reader.ReadStatistics(ctx, ids, from, now)
	require.NoError(t, err)
	require.Equal(t, service.PelicanShowcaseCounts{SuccessCount: 2, TotalCount: 2}, stats.Total)
	require.NotContains(t, stats.Groups, ids[0])
	_, err = integrationDB.ExecContext(ctx, "UPDATE groups SET deleted_at = NOW() WHERE id = $1", ids[1])
	require.NoError(t, err)
	stats, err = reader.ReadStatistics(ctx, ids, from, now)
	require.NoError(t, err)
	require.Zero(t, stats.Total.TotalCount)
	require.Empty(t, stats.Groups)
}

func TestPelicanStatisticsClaimKeepsExecutionStartMembership(t *testing.T) {
	ctx, plan, ids := statisticsFixture(t)
	now := time.Now().Truncate(time.Microsecond)
	plans := NewScheduledTestPlanRepository(integrationDB)
	claimed, err := plans.ClaimPelican(ctx, plan, now, now.Add(time.Minute), now.Add(time.Hour))
	require.NoError(t, err)
	require.True(t, claimed)
	require.ElementsMatch(t, ids, plan.PelicanGroupIDs)
	_, err = integrationDB.ExecContext(ctx, "DELETE FROM account_groups WHERE account_id = $1", plan.AccountID)
	require.NoError(t, err)
	result, err := NewScheduledTestResultRepository(integrationDB).Create(ctx, &service.ScheduledTestResult{
		PlanID: plan.ID, Status: "failed", StartedAt: now, FinishedAt: now.Add(time.Second),
		PelicanConfig: plan.PelicanConfig, PelicanGroupIDs: plan.PelicanGroupIDs,
	})
	require.NoError(t, err)
	var groups pq.Int64Array
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT group_ids FROM pelican_drawing_outcomes WHERE source_result_id = $1", result.ID).Scan(&groups))
	require.ElementsMatch(t, ids, []int64(groups))
}

func TestPelicanStatisticsCoverageRetentionAndMissingMetadata(t *testing.T) {
	ctx, plan, ids := statisticsFixture(t)
	resetStatisticsCoverage(t)
	reader := NewPelicanShowcaseRepository(integrationDB).(service.PelicanShowcaseStatisticsRepository)
	results := NewScheduledTestResultRepository(integrationDB)
	now := time.Now().Truncate(time.Microsecond)
	stats, err := reader.ReadStatistics(ctx, ids, now.Add(-24*time.Hour), now)
	require.NoError(t, err)
	require.Nil(t, stats, "no coverage is not zero traffic")
	require.NoError(t, results.MaintainPelicanStatistics(ctx, now))
	stats, err = reader.ReadStatistics(ctx, ids, now.Add(-24*time.Hour), now)
	require.NoError(t, err)
	require.True(t, stats.CoverageStartedAt.Equal(now))
	require.Zero(t, stats.Total.TotalCount)

	// A backlog exceeding one cleanup batch is drained, while the retention edge survives.
	_, err = integrationDB.ExecContext(ctx, "INSERT INTO pelican_drawing_outcomes (source_result_id,completed_at,success,group_ids) SELECT -1000000 - n, $1, false, $2::bigint[] FROM generate_series(1,1005) n", now.Add(-49*time.Hour), pq.Array(ids))
	require.NoError(t, err)
	for _, at := range []time.Time{now.Add(-48 * time.Hour), now.Add(-24 * time.Hour)} {
		_, err = results.Create(ctx, &service.ScheduledTestResult{PlanID: plan.ID, Status: "failed", StartedAt: at.Add(-time.Second), FinishedAt: at, PelicanConfig: plan.PelicanConfig})
		require.NoError(t, err)
	}
	require.NoError(t, results.MaintainPelicanStatistics(ctx, now))
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM pelican_drawing_outcomes WHERE group_ids && $1::bigint[]", pq.Array(ids)).Scan(&count))
	require.Equal(t, 2, count)
	require.NoError(t, results.MaintainPelicanStatistics(ctx, now.Add(time.Minute)))
	stats, err = reader.ReadStatistics(ctx, ids, now.Add(-24*time.Hour), now)
	require.NoError(t, err)
	require.True(t, stats.CoverageStartedAt.Equal(now), "cleanup and restarts never reset coverage")
	require.EqualValues(t, 1, stats.Total.TotalCount)

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	stats, err = reader.ReadStatistics(cancelled, ids, now.Add(-24*time.Hour), now)
	require.Error(t, err)
	require.Nil(t, stats, "failed read must not return zero counts")
}

func TestPelicanStatisticsCollectorActivationPrecedesFirstResult(t *testing.T) {
	resetStatisticsCoverage(t)
	plans := NewScheduledTestPlanRepository(integrationDB)
	results := NewScheduledTestResultRepository(integrationDB)
	scheduled := service.NewScheduledTestService(plans, results)
	runner := service.NewScheduledTestRunnerService(plans, scheduled, nil, nil, nil)
	before := time.Now()
	runner.Start()
	defer runner.Stop()
	var started time.Time
	require.NoError(t, integrationDB.QueryRowContext(context.Background(), "SELECT coverage_started_at FROM pelican_drawing_statistics_state WHERE id = 1").Scan(&started))
	require.False(t, started.Before(before.Truncate(time.Microsecond)))
	require.False(t, started.After(time.Now()))
	another := service.NewScheduledTestRunnerService(plans, scheduled, nil, nil, nil)
	another.Start()
	defer another.Stop()
	var restarted time.Time
	require.NoError(t, integrationDB.QueryRowContext(context.Background(), "SELECT coverage_started_at FROM pelican_drawing_statistics_state WHERE id = 1").Scan(&restarted))
	require.True(t, started.Equal(restarted), "new workers do not claim fresh coverage or backfill history")
}
