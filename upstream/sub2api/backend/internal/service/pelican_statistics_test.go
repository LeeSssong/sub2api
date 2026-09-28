package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type statisticsShowcaseRepo struct {
	showcaseRepoStub
	statistics *PelicanShowcaseStatistics
	err        error
	ids        []int64
	from, to   time.Time
}

func (r *statisticsShowcaseRepo) ReadStatistics(_ context.Context, ids []int64, from, to time.Time) (*PelicanShowcaseStatistics, error) {
	r.ids, r.from, r.to = ids, from, to
	return r.statistics, r.err
}

func TestPelicanShowcaseStatisticsWindowAndVisibleGroups(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	repo := &statisticsShowcaseRepo{
		showcaseRepoStub: showcaseRepoStub{groups: []*PelicanShowcaseGroup{{ID: 3}, {ID: 5}, {ID: 7}}},
		statistics: &PelicanShowcaseStatistics{
			CoverageStartedAt: now.Add(-2 * time.Hour), Total: PelicanShowcaseCounts{SuccessCount: 2, TotalCount: 3},
			Groups: map[int64]PelicanShowcaseCounts{3: {SuccessCount: 1, TotalCount: 2}, 5: {SuccessCount: 2, TotalCount: 2}},
		},
	}
	svc := &PelicanShowcaseService{repo: repo, settings: enabledShowcase()}
	view, err := svc.View(context.Background(), now)
	require.NoError(t, err)
	require.Equal(t, []int64{3, 5, 7}, repo.ids, "only plan-backed existing active groups reach statistics")
	require.Equal(t, now.Add(-24*time.Hour), repo.from)
	require.Equal(t, now, repo.to)
	require.NotNil(t, view.Stats)
	require.EqualValues(t, 2, view.Stats.SuccessCount)
	require.EqualValues(t, 3, view.Stats.TotalCount, "total is not the sum of group denominators")
	require.InDelta(t, 200.0/3.0, *view.Stats.SuccessRate, 0.000001)
	require.Equal(t, 50.0, *view.Groups[0].Stats.SuccessRate)
	require.Equal(t, 100.0, *view.Groups[1].Stats.SuccessRate)
	require.Zero(t, view.Groups[2].Stats.TotalCount)
	require.Nil(t, view.Groups[2].Stats.SuccessRate)
	require.Equal(t, repo.from, view.StatsWindow.From)
	require.Equal(t, repo.to, view.StatsWindow.To)
	require.Equal(t, repo.statistics.CoverageStartedAt, view.StatsWindow.CoverageStartedAt)
	require.False(t, view.StatsWindow.Complete)
	repo.statistics.CoverageStartedAt = now.Add(-24 * time.Hour)
	view, err = svc.View(context.Background(), now)
	require.NoError(t, err)
	require.True(t, view.StatsWindow.Complete, "exactly 24h of collector coverage is complete")
}

func TestPelicanShowcaseStatisticsUnavailableKeepsArtwork(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		name string
		data *PelicanShowcaseStatistics
		err  error
	}{
		{"read error", nil, errors.New("statistics unavailable")},
		{"missing metadata", nil, nil},
		{"empty coverage", &PelicanShowcaseStatistics{}, nil},
		{"future coverage", &PelicanShowcaseStatistics{CoverageStartedAt: now.Add(time.Hour)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &statisticsShowcaseRepo{showcaseRepoStub: showcaseRepoStub{
				groups: []*PelicanShowcaseGroup{{ID: 3}}, items: []*PelicanShowcaseItem{{ID: 10, GroupID: 3}},
			}, statistics: tc.data, err: tc.err}
			view, err := (&PelicanShowcaseService{repo: repo, settings: enabledShowcase()}).View(context.Background(), now)
			require.NoError(t, err)
			require.Nil(t, view.Stats)
			require.Nil(t, view.StatsWindow)
			require.Nil(t, view.Groups[0].Stats)
			require.Len(t, view.Groups[0].Items, 1)
		})
	}
}

func TestPelicanDrawingStatisticsExcludesManualAndKeepsZeroPercent(t *testing.T) {
	require.False(t, IsPelicanDrawingResult(nil))
	require.False(t, IsPelicanDrawingResult(&ScheduledTestResult{
		PlanID: 0, Status: "success", FinishedAt: time.Now(), PelicanConfig: &PelicanTestConfig{},
	}), "manual output has no scheduled plan")
	counts := (PelicanShowcaseCounts{TotalCount: 3}).statistics()
	require.NotNil(t, counts.SuccessRate)
	require.Zero(t, *counts.SuccessRate, "three failures are 0 percent, not unknown")
}
