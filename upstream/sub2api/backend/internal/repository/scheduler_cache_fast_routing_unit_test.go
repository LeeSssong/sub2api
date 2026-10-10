//go:build unit

package repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// Exercise Redis serialization, metadata filtering and gateway selection together:
// dropping Fast metadata must not reject eligible accounts or broaden model scope.
func TestSchedulerCacheFastRoutingRedisToGateway(t *testing.T) {
	for _, batch := range []bool{false, true} {
		for _, forced := range []bool{false, true} {
			for _, tc := range []struct {
				name  string
				extra map[string]any
				want  bool
			}{
				{"all_models", map[string]any{"openai_fast_supported": true}, true},
				{"mapped_model", map[string]any{"openai_fast_supported": true, "openai_fast_models": []string{"gpt-6.1-sol"}}, true},
				{"json_models", map[string]any{"openai_fast_supported": true, "openai_fast_models": []any{"gpt-6.1-sol"}}, true},
				{"public_model_is_not_upstream_scope", map[string]any{"openai_fast_supported": true, "openai_fast_models": []string{"public"}}, false},
				{"empty_scope", map[string]any{"openai_fast_supported": true, "openai_fast_models": []string{}}, false},
				{"null_scope", map[string]any{"openai_fast_supported": true, "openai_fast_models": nil}, false},
				{"malformed_scope", map[string]any{"openai_fast_supported": true, "openai_fast_models": "gpt-6.1-sol"}, false},
				{"disabled", map[string]any{"openai_fast_supported": false}, false},
				{"unmarked", nil, false},
			} {
				t.Run(fmt.Sprintf("batch=%t/forced=%t/%s", batch, forced, tc.name), func(t *testing.T) {
					ctx := context.Background()
					groupID := int64(71)
					cache := newSchedulerCacheUnit(t)
					account := service.Account{
						ID: 201, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
						Status: service.StatusActive, Schedulable: true, Concurrency: 10,
						GroupIDs: []int64{groupID}, Extra: tc.extra,
						Credentials: map[string]any{"model_mapping": map[string]any{"public": "gpt-6.1-sol"}},
					}
					bucket := service.SchedulerBucket{GroupID: groupID, Platform: service.PlatformOpenAI, Mode: service.SchedulerModeSingle}
					token, err := cache.CaptureBucketWriteToken(ctx, bucket)
					require.NoError(t, err)
					require.NoError(t, cache.SetSnapshot(ctx, bucket, token, []service.Account{account}))

					cfg := &config.Config{}
					cfg.Gateway.Scheduling.LoadBatchEnabled = batch
					repo := &ticketRoutingRepo{account: account}
					snapshot := service.NewSchedulerSnapshotService(cache, nil, repo, nil, cfg)
					gateway := service.NewOpenAIGatewayService(
						repo, nil, nil, nil, nil, nil, nil, nil, cfg, snapshot,
						service.NewConcurrencyService(ticketRoutingConcurrency{}),
						nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
					)
					t.Cleanup(gateway.StopOpenAICodexTicketHarvester)
					body := []byte(`{"service_tier":"priority"}`)
					if forced {
						ctx = context.WithValue(ctx, ctxkey.Group, &service.Group{
							ID: groupID, Platform: service.PlatformOpenAI,
							Status: service.StatusActive, Hydrated: true, ForceOpenAIFast: true,
						})
						body = []byte(`{}`)
					}
					ctx = gateway.WithOpenAIFastRoutingContext(ctx, body)
					for attempt := 0; attempt < 2; attempt++ {
						selection, _, err := gateway.SelectAccountWithScheduler(ctx, &groupID, "", "", "public", nil, service.OpenAIUpstreamTransportAny, false)
						if !tc.want {
							require.ErrorIs(t, err, service.ErrOpenAIFastUnavailable)
							require.Nil(t, selection)
							continue
						}
						require.NoError(t, err)
						require.NotNil(t, selection)
						require.NotNil(t, selection.Account)
						require.Equal(t, int64(201), selection.Account.ID)
						if selection.ReleaseFunc != nil {
							selection.ReleaseFunc()
						}
					}
				})
			}
		}
	}
}

func TestSchedulerCacheFastScopeUpdates(t *testing.T) {
	ctx := context.Background()
	cache := newSchedulerCacheUnit(t)
	account := service.Account{
		ID: 202, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true,
		Extra: map[string]any{"openai_fast_supported": true},
	}
	bucket := service.SchedulerBucket{GroupID: 71, Platform: service.PlatformOpenAI, Mode: service.SchedulerModeSingle}
	token, err := cache.CaptureBucketWriteToken(ctx, bucket)
	require.NoError(t, err)
	require.NoError(t, cache.SetSnapshot(ctx, bucket, token, []service.Account{account}))

	for _, tc := range []struct {
		name  string
		extra map[string]any
		want  bool
	}{
		{"enabled", map[string]any{"openai_fast_supported": true}, true},
		{"null_scope", map[string]any{"openai_fast_supported": true, "openai_fast_models": nil}, false},
		{"restricted", map[string]any{"openai_fast_supported": true, "openai_fast_models": []string{"gpt-6-astra"}}, false},
		{"restored", map[string]any{"openai_fast_supported": true, "openai_fast_models": []string{"gpt-6.1-sol"}}, true},
		{"disabled", map[string]any{"openai_fast_supported": false}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account.Extra = tc.extra
			require.NoError(t, cache.SetAccount(ctx, &account))
			candidates, hit, err := cache.GetSnapshot(ctx, bucket)
			require.NoError(t, err)
			require.True(t, hit)
			require.Len(t, candidates, 1)
			require.Equal(t, tc.want, candidates[0].SupportsOpenAIFastUpstreamModel("gpt-6.1-sol"))
		})
	}
}
