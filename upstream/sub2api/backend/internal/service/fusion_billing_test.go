package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestFusionAccountMultiplierPreservesProfitGate(t *testing.T) {
	ctx := context.WithValue(context.Background(), openAIProfitControlGateCtxKey{}, &openAIProfitControlGate{threshold: 1})
	rate := 0.8
	a := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, RateMultiplier: &rate}
	for _, tc := range []struct {
		factor float64
		veto   bool
	}{{0, true}, {0.5, true}, {1, false}, {2, false}} {
		a.GroupRateMultiplier = &tc.factor
		veto, _ := openAIProfitControlVetoReasonReadOnly(ctx, a)
		require.Equal(t, tc.veto, veto, "factor %v", tc.factor)
	}
}
func TestFusionBPSRawCostRemainsIndependentFromCustomerBuckets(t *testing.T) {
	repo := &openAIRecordUsageLogRepoStub{inserted: true}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(repo, &openAIRecordUsageBillingRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	svc.billingService = NewBillingService(svc.cfg, &PricingService{pricingData: map[string]*LiteLLMModelPricing{"gpt-6-astra": {InputCostPerToken: 5e-6, OutputCostPerToken: 30e-6, CacheCreationInputTokenCost: 6.25e-6, CacheCreationInputTokenCostExplicit: true, CacheReadInputTokenCost: 0.5e-6}}})
	gid := int64(7)
	a := excelAccount()
	a.Extra["openai_excel_bps_cache_creation_as_input"] = true
	err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{Result: &OpenAIForwardResult{RequestID: "fusion-bps", UpstreamEndpoint: "/basispoints/api/responses", Model: "gpt-6-astra", Usage: OpenAIUsage{InputTokens: 1000, CacheCreationInputTokens: 200, CacheReadInputTokens: 100, OutputTokens: 50}, Duration: time.Second}, APIKey: &APIKey{ID: 1, GroupID: &gid, Group: &Group{ID: gid, RateMultiplier: 1}}, User: &User{ID: 2}, Account: a})
	require.NoError(t, err)
	require.NotNil(t, repo.lastLog.AccountCost)
	require.InDelta(t, 700*5e-6+200*6.25e-6+100*0.5e-6+50*30e-6, *repo.lastLog.AccountCost, 1e-12)
	require.Equal(t, 900, repo.lastLog.InputTokens)
	require.Zero(t, repo.lastLog.CacheCreationTokens)
}
