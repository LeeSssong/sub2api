package repository

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func qualityTimelineSample(model, verdict string) gradedCandySample {
	return gradedCandySample{answered: true, result: service.ScheduledTestResult{
		Status: "success", PelicanConfig: &service.PelicanTestConfig{QuestionKind: "candy", ModelID: model, ModelIDs: []string{"a", "b"}, ParallelCount: 1, Quality: &service.QualityPolicy{}},
		QualityJudgment: &service.QualityJudgment{Verdict: verdict},
	}}
}

func TestGradedCandyRound(t *testing.T) {
	good := qualityTimelineSample("a", "correct")
	other := qualityTimelineSample("b", "correct")
	wrong := qualityTimelineSample("b", "incorrect")
	wrong.result.Status = "failed"
	wrong.result.ErrorMessage = "answer_mismatch"
	tests := []struct {
		name            string
		samples         []gradedCandySample
		valid, degraded bool
	}{
		{"all graded pass", []gradedCandySample{good, other}, true, false},
		{"one wrong counts one round", []gradedCandySample{good, wrong}, true, true},
		{"partial round excluded", []gradedCandySample{good}, false, false},
		{"duplicate model cannot hide missing model", []gradedCandySample{good, good}, false, false},
	}
	unknown := other
	unknown.result.QualityJudgment = &service.QualityJudgment{Verdict: "unknown"}
	tests = append(tests, struct {
		name            string
		samples         []gradedCandySample
		valid, degraded bool
	}{"grading unavailable", []gradedCandySample{good, unknown}, false, false})
	timeout := other
	timeout.result.Status = "failed"
	timeout.result.ErrorMessage = "timeout"
	tests = append(tests, struct {
		name            string
		samples         []gradedCandySample
		valid, degraded bool
	}{"timeout even beside wrong answer", []gradedCandySample{wrong, timeout}, false, false})
	noAnswer := other
	noAnswer.answered = false
	tests = append(tests, struct {
		name            string
		samples         []gradedCandySample
		valid, degraded bool
	}{"missing answer", []gradedCandySample{good, noAnswer}, false, false})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, d := classifyGradedCandyRound(tt.samples)
			require.Equal(t, tt.valid, v)
			require.Equal(t, tt.degraded, d)
		})
	}
}

func TestQualityTimelineRoundAssignedByLastCompletion(t *testing.T) {
	start := time.Date(2026, 10, 6, 10, 2, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	rounds := map[qualityTimelineRoundKey]*qualityTimelineRound{
		{groupID: 7, planID: 1, roundID: "round"}: {finishedAt: start.Add(6 * time.Minute), samples: []gradedCandySample{qualityTimelineSample("a", "correct"), qualityTimelineSample("b", "correct")}},
		{groupID: 7, planID: 2, roundID: "late"}:  {finishedAt: end, samples: []gradedCandySample{qualityTimelineSample("a", "correct"), qualityTimelineSample("b", "correct")}},
	}
	points := []service.MonitorV4TimelinePoint{{GroupID: 7, Start: start, End: start.Add(5 * time.Minute)}, {GroupID: 7, Start: start.Add(5 * time.Minute), End: start.Add(10 * time.Minute)}}
	applyGradedCandyRounds(points, rounds)
	require.Equal(t, 0, points[0].GradedRoundCount)
	require.Equal(t, 1, points[1].GradedRoundCount)
}
