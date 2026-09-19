package service

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestNativeConvergenceKeepsHealthyStickyWithoutHistoricalQuality(t *testing.T) {
	groupID := int64(11)
	accounts := []Account{unifiedQualityTestAccount(10, groupID), unifiedQualityTestAccount(20, groupID)}
	svc := &OpenAIGatewayService{accountRepo: &schedulerTestOpenAIAccountRepo{accounts: accounts}, cache: &schedulerTestGatewayCache{sessionBindings: map[string]int64{"session": 20}}}
	scheduler := newDefaultOpenAIAccountScheduler(svc, newOpenAIAccountRuntimeStats())
	selection, decision, err := scheduler.Select(context.Background(), OpenAIAccountScheduleRequest{GroupID: &groupID, Platform: PlatformOpenAI, SessionHash: "session", StickyAccountID: 20, RequestedModel: "gpt-5.4", RequiredTransport: OpenAIUpstreamTransportAny})
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, int64(20), selection.Account.ID)
	require.True(t, decision.StickySessionHit)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestNativeConvergenceUsageDoesNotRefreshQuality(t *testing.T) {
	repo := &convergenceUsageQualityRepo{openAIRecordUsageLogRepoStub: openAIRecordUsageLogRepoStub{inserted: true}}
	billing := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(repo, billing, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{RequestID: "convergence-usage", Usage: OpenAIUsage{InputTokens: 8, OutputTokens: 4}, Model: "gpt-5.1"},
		APIKey: &APIKey{ID: 1001, Group: &Group{RateMultiplier: 1}}, User: &User{ID: 2001}, Account: &Account{ID: 3001, Type: AccountTypeAPIKey},
	})
	require.NoError(t, err)
	require.Equal(t, 1, repo.calls)
	require.Equal(t, 1, billing.calls)
	require.Zero(t, repo.queries, "successful usage must not trigger historical aggregation")
}

func TestNativeConvergenceTextIgnoresAdaptiveSelection(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIScheduler.AdaptiveTopKEnabled = true
	cfg.Gateway.OpenAIScheduler.AdaptiveTopKMax = 20
	cfg.Gateway.OpenAIWS.LBTopK = 2
	svc := &OpenAIGatewayService{cfg: cfg}
	scheduler := newDefaultOpenAIAccountScheduler(svc, newOpenAIAccountRuntimeStats())
	accounts := []*Account{}
	for i := int64(1); i <= 4; i++ {
		a := unifiedQualityTestAccount(i, 11)
		accounts = append(accounts, &a)
	}
	plan := scheduler.(*defaultOpenAIAccountScheduler).buildOpenAIAccountLoadPlan(context.Background(), OpenAIAccountScheduleRequest{Platform: PlatformOpenAI, RequiredTransport: OpenAIUpstreamTransportHTTPSSE}, accounts, map[int64]*AccountLoadInfo{})
	require.Nil(t, plan.adaptivePolicy, "ordinary HTTP must not re-enable the custom adaptive selector")
	require.Equal(t, 2, plan.topK)
}

func TestNativeConvergenceRetiredAlertDoesNotEvaluateAsHealthyZero(t *testing.T) {
	evaluator := &OpsAlertEvaluatorService{}
	for _, metric := range []string{"openai_account_model_repeated_failure_count", "openai_account_model_cooldown_saturation_count", "openai_stream_failover_degradation_count", "openai_post_failure_selection_count", "openai_cache_hit_failover_decline_count"} {
		_, ok := evaluator.computeRuleMetric(context.Background(), &OpsAlertRule{MetricType: metric}, nil, time.Now().Add(-time.Minute), time.Now(), PlatformOpenAI, nil)
		require.False(t, ok, "retired metric %s must not evaluate as healthy", metric)
	}
}

func TestNativeConvergencePreservesExplicitProfitFallback(t *testing.T) {
	groupID := int64(11)
	expensive := unifiedQualityTestAccount(1, groupID)
	rate := 0.8
	expensive.RateMultiplier = &rate
	svc := &OpenAIGatewayService{accountRepo: &schedulerTestOpenAIAccountRepo{accounts: []Account{expensive}}}
	scheduler := newDefaultOpenAIAccountScheduler(svc, newOpenAIAccountRuntimeStats())
	ctx := context.WithValue(context.Background(), openAIProfitControlGateCtxKey{}, &openAIProfitControlGate{threshold: 0.5})
	selection, _, err := scheduler.Select(ctx, OpenAIAccountScheduleRequest{GroupID: &groupID, Platform: PlatformOpenAI, RequestedModel: "gpt-5.4", RequiredTransport: OpenAIUpstreamTransportHTTPSSE})
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.True(t, selection.ProfitBypassActive())
	vetoed, _ := OpenAIProfitControlVeto(ContextWithSelectionProfitGate(ctx, selection), selection.Account)
	require.False(t, vetoed)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestNativeConvergenceProjectionRetiredWithoutReadingQuality(t *testing.T) {
	svc := &OpenAIGatewayService{}
	projection, err := svc.Project(context.Background(), OpenAIAccountSchedulerProjectionRequest{Platform: PlatformOpenAI, GroupID: 11, Accounts: []*Account{{ID: 1}}})
	require.NoError(t, err)
	require.Equal(t, "retired", projection.Availability)
	require.Empty(t, projection.Candidates)
}

type convergenceUsageQualityRepo struct {
	openAIRecordUsageLogRepoStub
	queries int
}

func (r *convergenceUsageQualityRepo) ListOpenAIAccountQuality(context.Context, time.Time, time.Time) ([]OpenAIAccountQuality, error) {
	r.queries++
	return nil, nil
}

type convergenceAlertRepo struct{ opsRepoMock }

func (r *convergenceAlertRepo) ListAlertRules(context.Context) ([]*OpsAlertRule, error) {
	return []*OpsAlertRule{{ID: 1, MetricType: "openai_stream_failover_degradation_count", Enabled: true}, {ID: 2, MetricType: "cpu_usage_percent", Enabled: true}}, nil
}
func TestNativeConvergenceListsRetiredAlertAsUnavailable(t *testing.T) {
	svc := &OpsService{opsRepo: &convergenceAlertRepo{}}
	rules, err := svc.ListAlertRules(context.Background())
	require.NoError(t, err)
	require.Equal(t, "retired", rules[0].Availability)
	require.True(t, rules[0].Enabled, "stored enabled state must not be rewritten")
	require.NotEqual(t, "retired", rules[1].Availability)
}

func TestNativeConvergenceProfitQualifiedBusyPoolDoesNotBypass(t *testing.T) {
	groupID := int64(11)
	cheap, expensive := unifiedQualityTestAccount(1, groupID), unifiedQualityTestAccount(2, groupID)
	rate := 0.8
	expensive.RateMultiplier = &rate
	var acquired []int64
	svc := &OpenAIGatewayService{accountRepo: &schedulerTestOpenAIAccountRepo{accounts: []Account{cheap, expensive}}, concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{acquireResults: map[int64]bool{1: false, 2: true}, acquiredIDs: &acquired})}
	scheduler := newDefaultOpenAIAccountScheduler(svc, newOpenAIAccountRuntimeStats())
	ctx := context.WithValue(context.Background(), openAIProfitControlGateCtxKey{}, &openAIProfitControlGate{threshold: 0.5})
	selection, _, err := scheduler.Select(ctx, OpenAIAccountScheduleRequest{GroupID: &groupID, Platform: PlatformOpenAI, RequestedModel: "gpt-5.4", RequiredTransport: OpenAIUpstreamTransportHTTPSSE})
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, int64(1), selection.Account.ID)
	require.False(t, selection.ProfitBypassActive())
	require.False(t, selection.Acquired)
	require.NotNil(t, selection.WaitPlan)
	require.NotContains(t, acquired, int64(2))
}

func TestNativeConvergenceProfitBypassDoesNotReviveIneligibleAccounts(t *testing.T) {
	for _, reason := range []string{"disabled", "unsupported_model", "excluded"} {
		t.Run(reason, func(t *testing.T) {
			groupID := int64(11)
			a := unifiedQualityTestAccount(1, groupID)
			rate := 0.8
			a.RateMultiplier = &rate
			req := OpenAIAccountScheduleRequest{GroupID: &groupID, Platform: PlatformOpenAI, RequestedModel: "gpt-5.4", RequiredTransport: OpenAIUpstreamTransportHTTPSSE}
			switch reason {
			case "disabled":
				a.Schedulable = false
			case "unsupported_model":
				a.Credentials = map[string]any{"model_mapping": map[string]any{"other-model": "other-model"}}
			case "excluded":
				req.ExcludedIDs = map[int64]struct{}{1: {}}
			}
			svc := &OpenAIGatewayService{accountRepo: &schedulerTestOpenAIAccountRepo{accounts: []Account{a}}}
			scheduler := newDefaultOpenAIAccountScheduler(svc, newOpenAIAccountRuntimeStats())
			ctx := context.WithValue(context.Background(), openAIProfitControlGateCtxKey{}, &openAIProfitControlGate{threshold: 0.5})
			selection, _, err := scheduler.Select(ctx, req)
			require.Error(t, err)
			require.Nil(t, selection)
		})
	}
}

func TestNativeConvergenceGroupedReportsFeedNativeRuntime(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	t.Cleanup(resetOpenAIAdvancedSchedulerSettingCacheForTest)
	cfg := &config.Config{}
	settings := &openAIAdvancedSchedulerSettingRepoStub{values: map[string]string{openAIAdvancedSchedulerSettingKey: "true"}}
	svc := &OpenAIGatewayService{cfg: cfg, rateLimitService: &RateLimitService{settingService: NewSettingService(settings, cfg)}}
	ttft := 900
	svc.ReportOpenAIAccountScheduleResultForGroup(11, 42, "gpt-5.4", false, &ttft)
	scheduler := svc.getOrCreateOpenAIAccountScheduler().(*defaultOpenAIAccountScheduler)
	errorRate, firstOutput, observed := scheduler.stats.snapshot(42)
	require.True(t, observed, "native scheduler must observe grouped request results")
	require.Greater(t, errorRate, 0.0)
	require.Equal(t, float64(900), firstOutput)
	groupError, groupFirst, groupObserved := scheduler.stats.snapshotForGroup(11, 42)
	require.True(t, groupObserved)
	require.Equal(t, errorRate, groupError)
	require.Equal(t, firstOutput, groupFirst)
	svc.ReportOpenAIAccountScheduleResultForGroup(0, 42, "gpt-5.4", true, &ttft)
	evidence, ok := scheduler.stats.qualityGateEvidence(0, 42)
	require.True(t, ok)
	require.EqualValues(t, 2, evidence.SampleCount, "group zero reports must not be counted twice")
}

func BenchmarkNativeConvergenceRetiredEvent(b *testing.B) {
	event := OpenAIResilienceEvent{Name: OpenAIEventSchedulerSelection, CorrelationID: "benchmark", CandidateAccountIDs: []int64{1, 2, 3}}
	for i := 0; i < 4096; i++ {
		RecordOpenAIResilienceOutcome(event)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RecordOpenAIResilienceOutcome(event)
	}
}
