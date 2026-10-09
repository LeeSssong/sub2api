package service

import (
	"context"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestQualityTrafficVerdict(t *testing.T) {
	cfg := &PelicanTestConfig{ModelIDs: []string{"a", "b"}, ParallelCount: 1}
	good := &ScheduledTestResult{Status: "success", QualityJudgment: &QualityJudgment{Verdict: "correct"}}
	wrong := &ScheduledTestResult{Status: "failed", ErrorMessage: "answer_mismatch", QualityJudgment: &QualityJudgment{Verdict: "incorrect"}}
	timeout := &ScheduledTestResult{Status: "failed", ErrorMessage: "timeout"}
	for _, tt := range []struct {
		name    string
		results []*ScheduledTestResult
		want    string
	}{
		{"all pass restores", []*ScheduledTestResult{good, good}, "healthy"},
		{"one wrong marks account", []*ScheduledTestResult{good, wrong}, "degraded"},
		{"transport error never marks", []*ScheduledTestResult{good, timeout}, ""},
		{"wrong remains evidence beside timeout", []*ScheduledTestResult{wrong, timeout}, "degraded"},
		{"partial success cannot restore", []*ScheduledTestResult{good}, ""},
		{"no evidence", nil, ""},
	} {
		t.Run(tt.name, func(t *testing.T) { require.Equal(t, tt.want, qualityTrafficVerdict(cfg, tt.results)) })
	}
}

func TestQualityTrafficStartIsImmutableAcrossNestedForwarding(t *testing.T) {
	first := time.Now().Add(-time.Minute)
	later := time.Now()
	result := &OpenAIForwardResult{}
	stampQualityTrafficStart(result, first)
	stampQualityTrafficStart(result, later)
	require.Equal(t, first, result.QualityRequestStartedAt)
	stampQualityTrafficStart((*OpenAIForwardResult)(nil), first)
}

func TestQualityTrafficAttemptStartTracksRetries(t *testing.T) {
	require.True(t, QualityTrafficAttemptStartedAt(nil).IsZero())
	c := &gin.Context{}
	require.True(t, QualityTrafficAttemptStartedAt(c).IsZero())
	c.Set(qualityTrafficAttemptStartKey, "invalid")
	require.True(t, QualityTrafficAttemptStartedAt(c).IsZero())
	previous := time.Now().Add(-time.Minute)
	c.Set(qualityTrafficAttemptStartKey, previous)
	started := beginQualityTrafficAttempt(c)
	require.True(t, started.After(previous))
	require.Equal(t, started, QualityTrafficAttemptStartedAt(c), "retry must use the new account attempt time")
	require.False(t, beginQualityTrafficAttempt(nil).IsZero())
}

func TestQualityTrafficUsagePreservesDispatchTime(t *testing.T) {
	started := time.Now().Add(-time.Minute)
	for _, platform := range []string{"openai", "anthropic"} {
		t.Run(platform, func(t *testing.T) {
			repo := &openAIRecordUsageLogRepoStub{inserted: true}
			billing := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
			key := &APIKey{ID: 1, Group: &Group{RateMultiplier: 1}}
			user := &User{ID: 2}
			account := &Account{ID: 3, Type: AccountTypeAPIKey}
			var err error
			if platform == "openai" {
				svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(repo, billing, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
				err = svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{Result: &OpenAIForwardResult{QualityRequestStartedAt: started, RequestID: "quality-openai", Model: "gpt-5.1", Usage: OpenAIUsage{InputTokens: 8, OutputTokens: 4}}, APIKey: key, User: user, Account: account})
			} else {
				svc := newGatewayRecordUsageServiceWithBillingRepoForTest(repo, billing, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
				err = svc.RecordUsage(context.Background(), &RecordUsageInput{Result: &ForwardResult{QualityRequestStartedAt: started, RequestID: "quality-anthropic", Model: "claude-sonnet-4", Usage: ClaudeUsage{InputTokens: 8, OutputTokens: 4}}, APIKey: key, User: user, Account: account})
			}
			require.NoError(t, err)
			require.NotNil(t, repo.lastLog)
			require.Equal(t, &started, repo.lastLog.QualityRequestStartedAt, "async billing must not replace dispatch time with completion time")
		})
	}
}
