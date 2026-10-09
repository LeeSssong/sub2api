package service

import (
	"context"
	"errors"
	"github.com/tidwall/gjson"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

// An unmarked high-priority account must never win a Fast request, in either scheduler.
func TestOpenAIFastRouting_GroupForcedFastFiltersEveryScheduler(t *testing.T) {
	for _, advanced := range []bool{false, true} {
		for _, batch := range []bool{false, true} {
			t.Run(fmtSchedulerMode(advanced, batch), func(t *testing.T) {
				resetOpenAIAdvancedSchedulerSettingCacheForTest()
				groupID := int64(71)
				ctx := context.WithValue(context.Background(), ctxkey.Group, &Group{ID: groupID, Platform: PlatformOpenAI, ForceOpenAIFast: true, Hydrated: true, Status: StatusActive})
				accounts := []Account{
					{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Priority: 0, Concurrency: 1},
					{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Priority: 10, Concurrency: 1, Extra: map[string]any{"openai_fast_supported": true, "openai_fast_models": []any{"gpt-5.5"}}},
				}
				cfg := &config.Config{}
				cfg.Gateway.Scheduling.LoadBatchEnabled = batch
				cache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{"ordinary-session": 1}}
				svc := &OpenAIGatewayService{cfg: cfg, accountRepo: schedulerTestOpenAIAccountRepo{accounts: accounts}, cache: cache, concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{})}
				if advanced {
					svc.rateLimitService = newOpenAIAdvancedSchedulerRateLimitService("true")
				}
				selection, _, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "ordinary-session", "gpt-5.5", nil, OpenAIUpstreamTransportAny, false)
				require.NoError(t, err)
				require.NotNil(t, selection)
				require.Equal(t, int64(2), selection.Account.ID)
				if selection.ReleaseFunc != nil {
					selection.ReleaseFunc()
				}
			})
		}
	}
}

func fmtSchedulerMode(advanced, batch bool) string {
	if advanced {
		if batch {
			return "advanced/batch"
		}
		return "advanced/single"
	}
	if batch {
		return "legacy/batch"
	}
	return "legacy/single"
}

func TestOpenAIFastRouting_ExplicitTierAndOrdinaryRequests(t *testing.T) {
	for _, advanced := range []bool{false, true} {
		for _, batch := range []bool{false, true} {
			t.Run(fmtSchedulerMode(advanced, batch), func(t *testing.T) {
				resetOpenAIAdvancedSchedulerSettingCacheForTest()
				accounts := []Account{
					{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0},
					{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 10, Credentials: map[string]any{"model_mapping": map[string]any{"public": "gpt-5.5", "other": "gpt-6-astra"}}, Extra: map[string]any{"openai_fast_supported": true, "openai_fast_models": []any{"gpt-5.5"}}},
				}
				cfg := &config.Config{}
				cfg.Gateway.Scheduling.LoadBatchEnabled = batch
				svc := &OpenAIGatewayService{cfg: cfg, accountRepo: schedulerTestOpenAIAccountRepo{accounts: accounts}, cache: &schedulerTestGatewayCache{}, concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{})}
				if advanced {
					svc.rateLimitService = newOpenAIAdvancedSchedulerRateLimitService("true")
				}
				for _, tier := range []string{"fast", "priority"} {
					ctx := svc.WithOpenAIFastRoutingContext(context.Background(), []byte(`{"service_tier":"`+tier+`"}`))
					selected, _, err := svc.SelectAccountWithScheduler(ctx, nil, "", "", "public", nil, OpenAIUpstreamTransportAny, false)
					require.NoError(t, err)
					require.Equal(t, int64(2), selected.Account.ID)
					if selected.ReleaseFunc != nil {
						selected.ReleaseFunc()
					}
					// Failover must not escape into the unmarked pool.
					_, _, err = svc.SelectAccountWithScheduler(ctx, nil, "", "", "public", map[int64]struct{}{2: {}}, OpenAIUpstreamTransportAny, false)
					require.ErrorIs(t, err, ErrOpenAIFastUnavailable)
					_, _, err = svc.SelectAccountWithScheduler(ctx, nil, "", "", "other", nil, OpenAIUpstreamTransportAny, false)
					require.ErrorIs(t, err, ErrOpenAIFastUnavailable)
				}
				ctx := svc.WithOpenAIFastRoutingContext(context.Background(), []byte(`{}`))
				selected, _, err := svc.SelectAccountWithScheduler(ctx, nil, "", "", "public", map[int64]struct{}{2: {}}, OpenAIUpstreamTransportAny, false)
				require.NoError(t, err)
				require.Equal(t, int64(1), selected.Account.ID)
				if selected.ReleaseFunc != nil {
					selected.ReleaseFunc()
				}
				selected, _, err = svc.SelectAccountWithScheduler(ctx, nil, "", "", "public", map[int64]struct{}{1: {}}, OpenAIUpstreamTransportAny, false)
				require.NoError(t, err)
				require.Equal(t, int64(2), selected.Account.ID)
				if selected.ReleaseFunc != nil {
					selected.ReleaseFunc()
				}
			})
		}
	}
}

func TestOpenAIFastRouting_ManualScopeFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name  string
		extra map[string]any
		model string
		want  bool
	}{
		{"unmarked", nil, "gpt-5.5", false},
		{"disabled", map[string]any{"openai_fast_supported": false}, "gpt-5.5", false},
		{"all", map[string]any{"openai_fast_supported": true}, "gpt-5.5", true},
		{"empty", map[string]any{"openai_fast_supported": true, "openai_fast_models": []any{}}, "gpt-5.5", false},
		{"malformed", map[string]any{"openai_fast_supported": true, "openai_fast_models": "gpt-5.5"}, "gpt-5.5", false},
		{"typed", map[string]any{"openai_fast_supported": true, "openai_fast_models": []string{"gpt-5.5"}}, "gpt-5.5", true},
		{"json", map[string]any{"openai_fast_supported": true, "openai_fast_models": []any{"gpt-5.5", 1}}, "gpt-5.5", true},
		{"other model", map[string]any{"openai_fast_supported": true, "openai_fast_models": []any{"gpt-5.5"}}, "gpt-6-astra", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := &Account{Platform: PlatformOpenAI, Extra: tc.extra}
			require.Equal(t, tc.want, account.SupportsOpenAIFastUpstreamModel(tc.model))
		})
	}
}

func TestOpenAIFastRouting_WSFollowupAndPolicyCannotDowngrade(t *testing.T) {
	svc := &OpenAIGatewayService{}
	ctx := svc.WithOpenAIFastRoutingContext(context.Background(), []byte(`{"service_tier":"priority"}`))
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Extra: map[string]any{"openai_fast_supported": true, "openai_fast_models": []any{"gpt-5.5"}}}
	// A Standard follow-up on a Fast connection must remain Standard.
	out, blocked, err := svc.applyOpenAIFastPolicyToWSResponseCreate(ctx, account, "gpt-5.5", []byte(`{"type":"response.create","model":"gpt-5.5"}`))
	require.NoError(t, err)
	require.Nil(t, blocked)
	require.False(t, gjson.GetBytes(out, "service_tier").Exists())
	// Conversely, turning Fast on later cannot bypass capability checks.
	standardCtx := svc.WithOpenAIFastRoutingContext(context.Background(), []byte(`{}`))
	_, blocked, err = svc.applyOpenAIFastPolicyToWSResponseCreate(standardCtx, account, "gpt-6-astra", []byte(`{"type":"response.create","service_tier":"fast"}`))
	require.NoError(t, err)
	require.NotNil(t, blocked)
	out, blocked, err = svc.applyOpenAIFastPolicyToWSResponseCreate(standardCtx, account, "gpt-5.5", []byte(`{"type":"response.create","service_tier":"fast"}`))
	require.NoError(t, err)
	require.Nil(t, blocked)
	require.Equal(t, "priority", gjson.GetBytes(out, "service_tier").String())
	ctx = withOpenAIFastPolicyContext(ctx, &OpenAIFastPolicySettings{Rules: []OpenAIFastPolicyRule{{ServiceTier: OpenAIFastTierPriority, Action: BetaPolicyActionFilter}}})
	_, err = svc.applyOpenAIFastPolicyToBody(ctx, account, "gpt-5.5", []byte(`{"service_tier":"priority"}`))
	var denied *OpenAIFastBlockedError
	require.True(t, errors.As(err, &denied))
}

func TestOpenAIFastRouting_ContinuationDoesNotMoveWithoutContext(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	groupID := int64(79)
	accounts := []Account{
		{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1},
		{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Extra: map[string]any{"openai_fast_supported": true}},
	}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, accountRepo: schedulerTestOpenAIAccountRepo{accounts: accounts}, cache: &schedulerTestGatewayCache{}, concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{})}
	ctx := svc.WithOpenAIFastRoutingContext(context.Background(), []byte(`{"service_tier":"fast"}`))
	require.NoError(t, svc.getOpenAIWSStateStore().BindResponseAccount(ctx, groupID, "resp_standard", 1, time.Hour))
	_, _, err := svc.SelectAccountWithSchedulerForCapability(ctx, &groupID, "resp_standard", "", "gpt-5.5", nil, OpenAIUpstreamTransportAny, "", false, false, true)
	require.ErrorIs(t, err, ErrOpenAIFastContinuation)
	selected, _, err := svc.SelectAccountWithSchedulerForCapability(ctx, &groupID, "resp_standard", "", "gpt-5.5", nil, OpenAIUpstreamTransportAny, "", false, true, true)
	require.NoError(t, err)
	require.Equal(t, int64(2), selected.Account.ID)
	if selected.ReleaseFunc != nil {
		selected.ReleaseFunc()
	}
}

func TestOpenAIFastRouting_RevokedCapabilityInvalidatesTurn(t *testing.T) {
	selected := &Account{ID: 5, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Extra: map[string]any{"openai_fast_supported": true}}
	latest := *selected
	latest.Extra = map[string]any{}
	svc := &OpenAIGatewayService{accountRepo: &turnAdmissionRepo{account: &latest}}
	ctx := svc.WithOpenAIFastRoutingContext(context.Background(), []byte(`{"service_tier":"fast"}`))
	_, err := svc.admitOpenAITurn(ctx, nil, selected, "gpt-5.5")
	require.Error(t, err)
}

func TestOpenAIFastRouting_MixedCatalogAdvertisesManuallyRoutableFast(t *testing.T) {
	accounts := []Account{
		{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://api.openai.com", "model_mapping": map[string]any{"gpt-6-astra": "gpt-6-astra"}}},
		{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://proxy.example", "model_mapping": map[string]any{"gpt-6-astra": "gpt-6-astra"}}, Extra: map[string]any{"openai_fast_supported": true, "openai_fast_models": []any{"gpt-6-astra"}}},
	}
	svc := &OpenAIGatewayService{accountRepo: codexModelsVisibilityAccountRepo{byGroup: map[int64][]Account{1: accounts}}}
	manifest, _, err := svc.BuildGroupConfiguredCodexModelsManifest(context.Background(), &Group{ID: 1, Platform: PlatformOpenAI}, "")
	require.NoError(t, err)
	models := decodeCodexManifestModels(t, manifest.Body)
	tiers := models[0]["service_tiers"].([]any)
	require.NotEmpty(t, tiers)
	require.Equal(t, "priority", tiers[0].(map[string]any)["id"])
}

func TestOpenAIFastRouting_MissingTierForceReportsFastUnavailable(t *testing.T) {
	for _, advanced := range []bool{false, true} {
		for _, batch := range []bool{false, true} {
			t.Run(fmtSchedulerMode(advanced, batch), func(t *testing.T) {
				resetOpenAIAdvancedSchedulerSettingCacheForTest()
				svc := newOpenAIGatewayServiceWithSettings(t, &OpenAIFastPolicySettings{Rules: []OpenAIFastPolicyRule{{ServiceTier: OpenAIFastTierMissing, Action: OpenAIFastPolicyActionForcePriority}}})
				svc.accountRepo = schedulerTestOpenAIAccountRepo{accounts: []Account{{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1}}}
				svc.cfg = &config.Config{}
				svc.cfg.Gateway.Scheduling.LoadBatchEnabled = batch
				svc.concurrencyService = NewConcurrencyService(schedulerTestConcurrencyCache{})
				if advanced {
					svc.rateLimitService = newOpenAIAdvancedSchedulerRateLimitService("true")
				}
				ctx := svc.WithOpenAIFastRoutingContext(context.Background(), []byte(`{}`))
				_, _, err := svc.SelectAccountWithScheduler(ctx, nil, "", "", "gpt-5.5", nil, OpenAIUpstreamTransportAny, false)
				require.ErrorIs(t, err, ErrOpenAIFastUnavailable)
			})
		}
	}
}

func TestOpenAIFastRouting_PassthroughScopeUsesActualModel(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": map[string]any{"public": "gpt-5.5"}}, Extra: map[string]any{"openai_passthrough": true, "openai_fast_supported": true, "openai_fast_models": []any{"gpt-5.5"}}}
	svc := &OpenAIGatewayService{}
	ctx := svc.WithOpenAIFastRoutingContext(context.Background(), []byte(`{"service_tier":"fast"}`))
	require.Equal(t, "fast_not_supported", openAIFastRoutingFailureReason(ctx, account, "public", false))
	account.Extra["openai_fast_models"] = []any{"public"}
	require.Empty(t, openAIFastRoutingFailureReason(ctx, account, "public", false))
}

func TestOpenAIFastRouting_CompactScopeUsesCompactMapping(t *testing.T) {
	for _, advanced := range []bool{false, true} {
		for _, batch := range []bool{false, true} {
			t.Run(fmtSchedulerMode(advanced, batch), func(t *testing.T) {
				resetOpenAIAdvancedSchedulerSettingCacheForTest()
				account := Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Credentials: map[string]any{"model_mapping": map[string]any{"public": "gpt-5.5"}, "compact_model_mapping": map[string]any{"public": "gpt-6-astra"}}, Extra: map[string]any{"openai_compact_supported": true, "openai_fast_supported": true, "openai_fast_models": []any{"gpt-6-astra"}}}
				cfg := &config.Config{}
				cfg.Gateway.Scheduling.LoadBatchEnabled = batch
				svc := &OpenAIGatewayService{cfg: cfg, accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{account}}, cache: &schedulerTestGatewayCache{}, concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{})}
				if advanced {
					svc.rateLimitService = newOpenAIAdvancedSchedulerRateLimitService("true")
				}
				ctx := svc.WithOpenAIFastRoutingContext(context.Background(), []byte(`{"service_tier":"fast"}`))
				selected, _, err := svc.SelectAccountWithScheduler(ctx, nil, "", "", "public", nil, OpenAIUpstreamTransportAny, true)
				require.NoError(t, err)
				require.Equal(t, int64(1), selected.Account.ID)
				if selected.ReleaseFunc != nil {
					selected.ReleaseFunc()
				}
				_, _, err = svc.SelectAccountWithScheduler(ctx, nil, "", "", "public", nil, OpenAIUpstreamTransportAny, false)
				require.ErrorIs(t, err, ErrOpenAIFastUnavailable)
			})
		}
	}
}
