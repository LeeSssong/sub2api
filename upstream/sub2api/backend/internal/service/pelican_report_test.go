package service

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPelicanReportCandyRequiresActualGrading(t *testing.T) {
	cases := []struct {
		cfg                                   *PelicanTestConfig
		status, text, kind, judgment, outcome string
	}{
		{&PelicanTestConfig{Prompt: CandyPrompt}, "success", "21", "candy", "builtin_candy", "success"},
		{&PelicanTestConfig{Prompt: CandyPrompt}, "success", "29", "candy", "builtin_candy", "failed"},
		{&PelicanTestConfig{QuestionKind: "candy", Prompt: "custom"}, "success", "29", "candy", "ungraded_candy", "ungraded"},
		{&PelicanTestConfig{QuestionKind: "candy", Prompt: "custom"}, "failed", "", "candy", "ungraded_candy", "failed"},
		{&PelicanTestConfig{Prompt: "draw"}, "success", "<svg></svg>", "pelican", "drawing", "success"},
	}
	for _, tc := range cases {
		kind, judgment, outcome := ClassifyPelicanReportResult(&ScheduledTestResult{PelicanConfig: tc.cfg, Status: tc.status, ResponseText: tc.text})
		require.Equal(t, tc.kind, kind)
		require.Equal(t, tc.judgment, judgment)
		require.Equal(t, tc.outcome, outcome)
	}
	kind, _, _ := ClassifyPelicanReportResult(&ScheduledTestResult{PelicanConfig: &PelicanTestConfig{Quality: &QualityPolicy{}}, Status: "success"})
	require.Empty(t, kind)
}
func reportFact(id int64, kind, execution, status string, at time.Time, duration int64) PelicanReportFact {
	started := at.Add(-time.Duration(duration) * time.Millisecond)
	return PelicanReportFact{HasOutput: status == "success", ResultID: id, Kind: kind, ModelID: "model-a", ExecutionID: execution, ExpectedCount: 1, Status: status, Judgment: "drawing", ObservedAt: at, StartedAt: &started, CompletedAt: &at}
}

func TestPelicanReportCurrentSelectsNewestCompletedIndividual(t *testing.T) {
	now := time.Now().UTC()
	older := reportFact(301, "pelican", "batch", "failed", now.Add(-2*time.Minute), 1000)
	newer := reportFact(302, "pelican", "batch", "success", now.Add(-time.Minute), 2000)
	older.ExpectedCount, newer.ExpectedCount = 3, 3
	data := &PelicanReportData{Group: PelicanReportGroup{ID: 6}, ModelID: "model-a",
		Facts: []PelicanReportFact{newer, older}, Artwork: &PelicanReportArtwork{ID: 9, SourceResultID: 302, GroupID: 6, ModelID: "model-a"}}
	for _, failed := range []bool{false, true} {
		if failed {
			data.Facts[1].Status = "failed"
		}
		view := buildPelicanReport(data, now)
		raw, err := json.Marshal(view)
		require.NoError(t, err)
		var decoded struct {
			Current *struct {
				Pelican *PelicanReportResult  `json:"pelican"`
				Artwork *PelicanReportArtwork `json:"artwork"`
			} `json:"current"`
		}
		require.NoError(t, json.Unmarshal(raw, &decoded))
		require.NotNil(t, decoded.Current)
		require.Equal(t, int64(302), decoded.Current.Pelican.ResultID)
		require.Equal(t, int64(2000), *decoded.Current.Pelican.LatencyMs)
		if failed {
			require.Nil(t, decoded.Current.Artwork)
		} else {
			require.Equal(t, int64(302), decoded.Current.Artwork.SourceResultID)
		}
		require.Nil(t, view.Artwork, "legacy incomplete batch contract remains unchanged")
	}
}

func TestPelicanReportCurrentFollowsArtworkGenerationOrder(t *testing.T) {
	now := time.Now().UTC()
	// An older, slower generation finishes last; the page still shows the newer start first.
	newerStart := reportFact(402, "pelican", "newer-start", "success", now.Add(-2*time.Minute), 1000)
	olderStart := reportFact(403, "pelican", "older-start", "success", now.Add(-time.Minute), 300000)
	data := &PelicanReportData{Group: PelicanReportGroup{ID: 6}, ModelID: "model-a", Facts: []PelicanReportFact{newerStart, olderStart},
		Artwork: &PelicanReportArtwork{ID: 12, SourceResultID: 402, GroupID: 6, ModelID: "model-a"}}
	view := buildPelicanReport(data, now)
	require.Equal(t, int64(402), view.Current.Pelican.ResultID)
	require.Equal(t, int64(402), view.Current.Artwork.SourceResultID)
	require.Equal(t, "older-start", *view.Latest.Pelican.ExecutionID, "legacy execution ordering is unchanged")
}
func TestPelicanReportCountsExactWindowAndActualDurations(t *testing.T) {
	now := time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC)
	rows := []PelicanReportFact{reportFact(1, "pelican", "e1", "success", now.Add(-time.Hour), 2000), reportFact(2, "pelican", "e1", "failed", now.Add(-time.Hour), 4000), reportFact(3, "candy", "c1", "ungraded", now.Add(-time.Minute), 3000), reportFact(4, "pelican", "old", "success", now.Add(-24*time.Hour-time.Nanosecond), 1000), reportFact(5, "pelican", "future", "success", now, 1000)}
	rows[0].ExpectedCount = 2
	rows[1].ExpectedCount = 2
	data := &PelicanReportData{Group: PelicanReportGroup{ID: 6}, ModelID: "model-a", Facts: rows}
	view := buildPelicanReport(data, now)
	require.Equal(t, int64(2), view.Statistics.Pelican.TotalCount)
	require.Equal(t, int64(1), *view.Statistics.Pelican.ExecutionCount)
	require.Equal(t, 50.0, *view.Statistics.Pelican.SuccessRate)
	require.Equal(t, 3000.0, *view.Statistics.Pelican.AvgLatencyMs)
	require.Equal(t, int64(1), view.Statistics.Candy.UngradedCount)
	require.Nil(t, view.Statistics.Candy.SuccessRate)
	require.Nil(t, view.Statistics.Candy.AvgLatencyMs)
	require.Len(t, view.History.Pelican, 2)
	require.Equal(t, "failed", view.Latest.Pelican.Status)
	require.Nil(t, view.CurrentRound)
}
func TestPelicanReportMissingTimesAndLatestFailureSuppressArtwork(t *testing.T) {
	now := time.Now().UTC()
	f := reportFact(10, "pelican", "e", "failed", now.Add(-time.Minute), 1000)
	f.StartedAt = nil
	view := buildPelicanReport(&PelicanReportData{ModelID: "model-a", Facts: []PelicanReportFact{f}, Artwork: &PelicanReportArtwork{SourceResultID: 9, ExecutionID: reportString("old")}}, now)
	require.Equal(t, int64(0), view.Statistics.Pelican.TimedCount)
	require.Nil(t, view.Statistics.Pelican.AvgLatencyMs)
	require.Nil(t, view.Latest.Pelican.LatencyMs)
	require.Nil(t, view.Artwork)
}

func TestPelicanReportIncompleteExecutionIsPending(t *testing.T) {
	now := time.Now().UTC()
	f := reportFact(12, "pelican", "partial", "success", now.Add(-time.Minute), 1000)
	f.ExpectedCount = 2
	view := buildPelicanReport(&PelicanReportData{ModelID: "model-a", Facts: []PelicanReportFact{f}}, now)
	require.Equal(t, "pending", view.Latest.Pelican.Status)
	require.Nil(t, view.Latest.Pelican.CompletedAt)
	require.Nil(t, view.Latest.Pelican.LatencyMs)
}
func TestPelicanReportRoundNeedsExplicitSameTriggerAndCompleteResults(t *testing.T) {
	now := time.Now().UTC()
	scheduled := now.Add(-time.Hour)
	a := reportFact(1, "candy", "c", "success", now.Add(-time.Minute), 1000)
	b := reportFact(2, "pelican", "p", "success", now.Add(-time.Second), 2000)
	a.SharedRoundID = "same"
	b.SharedRoundID = "same"
	a.ScheduledFor = &scheduled
	b.ScheduledFor = &scheduled
	data := &PelicanReportData{ModelID: "model-a", Facts: []PelicanReportFact{a, b}}
	require.NotNil(t, buildPelicanReport(data, now).CurrentRound)
	data.Facts[1].SharedRoundID = "different"
	require.Nil(t, buildPelicanReport(data, now).CurrentRound)
	data.Facts[1].SharedRoundID = "same"
	data.Facts[1].ExpectedCount = 2
	require.Nil(t, buildPelicanReport(data, now).CurrentRound)
}
func TestPelicanReportPairNeedsExactlyTwoExplicitMatchingPlans(t *testing.T) {
	scheduled := time.Date(2026, 9, 26, 7, 0, 0, 0, time.UTC)
	a := pelicanPlan()
	a.ID = 1
	a.NextRunAt = &scheduled
	a.PelicanConfig.ReportPairKey = "daily"
	b := pelicanPlan()
	b.ID = 2
	b.NextRunAt = &scheduled
	b.PelicanConfig.ReportPairKey = "daily"
	b.PelicanConfig.Prompt = CandyPrompt
	id := PelicanReportPairIdentity(a, []*ScheduledTestPlan{a, b}, "UTC")
	require.NotEmpty(t, id)
	require.Equal(t, id, PelicanReportPairIdentity(b, []*ScheduledTestPlan{b, a}, "UTC"))
	b.CronExpression = "*/15 * * * *"
	require.Empty(t, PelicanReportPairIdentity(a, []*ScheduledTestPlan{a, b}, "UTC"))
	b.CronExpression = a.CronExpression
	require.Empty(t, PelicanReportPairIdentity(a, []*ScheduledTestPlan{a, b, a}, "UTC"))
}

func TestPelicanReportPairIdentityChangesWhenMemberEdited(t *testing.T) {
	scheduled := time.Date(2026, 9, 26, 7, 0, 0, 0, time.UTC)
	a, b := pelicanPlan(), pelicanPlan()
	a.ID, b.ID = 1, 2
	a.NextRunAt, b.NextRunAt = &scheduled, &scheduled
	a.PelicanConfig.ReportPairKey, b.PelicanConfig.ReportPairKey = "daily", "daily"
	b.PelicanConfig.Prompt = CandyPrompt
	id := PelicanReportPairIdentity(a, []*ScheduledTestPlan{a, b}, "UTC")
	require.NotEmpty(t, id)
	b.UpdatedAt = scheduled.Add(time.Second)
	require.NotEqual(t, id, PelicanReportPairIdentity(a, []*ScheduledTestPlan{a, b}, "UTC"), "edited or re-enabled members must not reuse an earlier round")
	b.UpdatedAt = time.Time{}
	a.AccountID++
	b.AccountID++
	require.NotEqual(t, id, PelicanReportPairIdentity(a, []*ScheduledTestPlan{a, b}, "UTC"), "account scope must be part of the identity")
}

func TestPelicanReportInvalidResultTimesAreUnknown(t *testing.T) {
	now := time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC)
	f := reportFact(31, "pelican", "execution", "success", now.Add(-time.Minute), -1000)
	view := buildPelicanReport(&PelicanReportData{ModelID: "model-a", Facts: []PelicanReportFact{f}}, now)
	require.Nil(t, view.History.Pelican[0].StartedAt)
	require.Nil(t, view.History.Pelican[0].CompletedAt)
	require.Nil(t, view.History.Pelican[0].LatencyMs)
	require.Nil(t, view.Latest.Pelican.StartedAt)
	require.Nil(t, view.Latest.Pelican.CompletedAt)
	require.Zero(t, view.Statistics.Pelican.TimedCount)
}

// Export actual service DTOs for the bot's cross-language contract verification.
// No detector, database or sender is used by this fixture generator.
func TestPelicanReportExportContractCases(t *testing.T) {
	dir := os.Getenv("PELICAN_REPORT_CONTRACT_DIR")
	if dir == "" {
		return
	}
	require.True(t, filepath.IsAbs(dir))
	require.NoError(t, os.MkdirAll(dir, 0700))
	now := time.Now().UTC().Truncate(time.Second)
	due := now.Add(-5 * time.Minute)
	candy := reportFact(101, "candy", "fixture-candy", "success", now.Add(-2*time.Minute), 1400)
	candy.Judgment = "builtin_candy"
	drawing := reportFact(102, "pelican", "fixture-drawing", "success", now.Add(-time.Minute), 2600)
	candy.SharedRoundID, drawing.SharedRoundID = "fixture-shared-round", "fixture-shared-round"
	candy.ScheduledFor, drawing.ScheduledFor = &due, &due
	partial := drawing
	partial.ExpectedCount = 2
	legacy := drawing
	legacy.ExecutionID, legacy.SharedRoundID, legacy.ExpectedCount, legacy.ScheduledFor = "", "", 0, nil
	missing := drawing
	missing.StartedAt = nil
	reversed := drawing
	later := now.Add(-30 * time.Second)
	reversed.StartedAt = &later
	ungraded := candy
	ungraded.Status, ungraded.Judgment = "ungraded", "ungraded_candy"
	failed := drawing
	failed.Status = "failed"
	bounded := make([]PelicanReportFact, 1001)
	for i := range bounded {
		bounded[i] = reportFact(int64(1000+i), "pelican", "", "success", now.Add(-time.Duration(len(bounded)-i)*time.Second), 1000)
	}
	for name, facts := range map[string][]PelicanReportFact{
		"full": {candy, drawing}, "partial": {partial}, "historical": {legacy},
		"missing-time": {missing}, "reversed-time": {reversed}, "empty": {},
		"ungraded": {ungraded, drawing}, "failed": {candy, failed}, "bounded": bounded,
	} {
		rate := 1.25
		data := &PelicanReportData{Group: PelicanReportGroup{ID: 6, Name: "星桥自测分组", Platform: "openai", RateMultiplier: &rate}, ModelID: "model-a", Facts: facts}
		if name == "full" || name == "historical" {
			data.Artwork = &PelicanReportArtwork{ID: 201, SourceResultID: 102, GroupID: 6, ModelID: "model-a", GeneratedAt: *drawing.StartedAt, ResponseText: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 400 200"><rect width="400" height="200" fill="#102435"/><text x="40" y="110" fill="white" font-size="28">Offline test artwork</text></svg>`}
		}
		raw, err := json.MarshalIndent(map[string]any{"report": buildPelicanReport(data, now)}, "", "  ")
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, name+".json"), raw, 0600))
	}
}

func TestPelicanReportPairRejectsDifferentDueTimeOrAccount(t *testing.T) {
	scheduled := time.Date(2026, 9, 26, 7, 0, 0, 0, time.UTC)
	a, b := pelicanPlan(), pelicanPlan()
	a.ID, b.ID = 1, 2
	a.NextRunAt, b.NextRunAt = &scheduled, &scheduled
	a.PelicanConfig.ReportPairKey, b.PelicanConfig.ReportPairKey = "daily", "daily"
	b.PelicanConfig.Prompt = CandyPrompt
	later := scheduled.Add(time.Second)
	b.NextRunAt = &later
	require.Empty(t, PelicanReportPairIdentity(a, []*ScheduledTestPlan{a, b}, "UTC"))
	b.NextRunAt = &scheduled
	b.AccountID++
	require.Empty(t, PelicanReportPairIdentity(a, []*ScheduledTestPlan{a, b}, "UTC"))
}

func TestPelicanReportBoundsHistoryWithoutLosingStatistics(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	facts := make([]PelicanReportFact, 10001)
	for i := range facts {
		facts[i] = reportFact(int64(i+1), "pelican", "", "success", now.Add(-time.Duration(len(facts)-i)*time.Second), 1000)
	}
	view := buildPelicanReport(&PelicanReportData{ModelID: "model-a", Facts: facts}, now)
	require.Equal(t, int64(10001), view.Statistics.Pelican.TotalCount)
	require.Len(t, view.History.Pelican, 240)
	require.Equal(t, int64(10001), view.HistoryMeta.Pelican.TotalCount)
	require.Equal(t, 240, view.HistoryMeta.Pelican.ReturnedCount)
	require.Equal(t, 240, view.HistoryMeta.Pelican.Limit)
	require.Equal(t, int64(9762), view.History.Pelican[0].ResultID)
	require.Equal(t, int64(10001), view.Latest.Pelican.Results[0].ResultID)
	// The repository only loads these timeline rows but supplies complete SQL totals.
	stats := view.Statistics
	limited := buildPelicanReport(&PelicanReportData{ModelID: "model-a", Facts: facts[len(facts)-240:], Statistics: &stats}, now)
	require.Equal(t, int64(10001), limited.Statistics.Pelican.TotalCount)
	require.Equal(t, int64(10001), limited.HistoryMeta.Pelican.TotalCount)
	require.Len(t, limited.History.Pelican, 240)
}

func TestPelicanReportCurrentCandyIgnoresUnavailableAttempts(t *testing.T) {
	now := time.Now().UTC()
	answered := reportFact(1, "candy", "answered", "success", now.Add(-time.Hour), 1000)
	answered.Judgment = "builtin_candy"
	unavailable := reportFact(2, "candy", "maintenance", "failed", now.Add(-time.Minute), 20)
	unavailable.Judgment = "builtin_candy"
	data := &PelicanReportData{ModelID: "model-a", Facts: []PelicanReportFact{answered, unavailable}}
	view := buildPelicanReport(data, now)
	require.NotNil(t, view.Current.Candy)
	require.Equal(t, int64(1), view.Current.Candy.ResultID)
	require.Equal(t, "success", view.Current.Candy.Status)
	require.Equal(t, "failed", view.Latest.Candy.Status)
	require.Equal(t, int64(1), view.Statistics.Candy.FailureCount)
	data.Facts = []PelicanReportFact{unavailable}
	require.Nil(t, buildPelicanReport(data, now).Current.Candy)
}

func TestPelicanReportCurrentCandyKeepsLatestWrongAnswer(t *testing.T) {
	now := time.Now().UTC()
	passed := reportFact(1, "candy", "pass", "success", now.Add(-time.Hour), 1000)
	wrong := reportFact(2, "candy", "wrong", "failed", now.Add(-time.Minute), 1000)
	transport := reportFact(3, "candy", "timeout", "failed", now.Add(-time.Second), 20)
	passed.Judgment, wrong.Judgment, transport.Judgment = "builtin_candy", "builtin_candy", "builtin_candy"
	wrong.CandyEvaluated = true
	view := buildPelicanReport(&PelicanReportData{ModelID: "model-a", Facts: []PelicanReportFact{passed, wrong, transport}}, now)
	require.Equal(t, int64(2), view.Current.Candy.ResultID)
	require.Equal(t, "failed", view.Current.Candy.Status)
}

func TestPelicanReportCurrentPelicanExposesOutputWithoutChangingHistory(t *testing.T) {
	now := time.Now().UTC()
	for _, hasOutput := range []bool{false, true} {
		f := reportFact(5, "pelican", "latest", "failed", now.Add(-time.Minute), 1000)
		f.HasOutput = hasOutput
		view := buildPelicanReport(&PelicanReportData{ModelID: "model-a", Facts: []PelicanReportFact{f}}, now)
		require.NotNil(t, view.Current.Pelican.HasOutput)
		require.Equal(t, hasOutput, *view.Current.Pelican.HasOutput)
		require.Nil(t, view.History.Pelican[0].HasOutput)
		require.Equal(t, "failed", view.History.Pelican[0].Status)
	}
}

func TestPelicanReportCannotResurrectPassAfterPrunedUnknownCandyFailure(t *testing.T) {
	now := time.Now().UTC()
	pass := reportFact(1, "candy", "pass", "success", now.Add(-time.Hour), 1000)
	missing := reportFact(2, "candy", "missing", "failed", now.Add(-time.Minute), 1000)
	pass.Judgment, missing.Judgment = "builtin_candy", "builtin_candy"
	missing.ResultPruned = true
	view := buildPelicanReport(&PelicanReportData{ModelID: "model-a", Facts: []PelicanReportFact{pass, missing}}, now)
	require.Nil(t, view.Current.Candy, "a deleted failure must not be guessed to be a transport error")
}
