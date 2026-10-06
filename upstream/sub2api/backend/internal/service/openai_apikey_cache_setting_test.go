package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAPIKeyIndependentCacheCreationSetting(t *testing.T) {
	for _, tt := range []struct {
		name  string
		extra map[string]any
		want  bool
	}{
		{"independent enabled", map[string]any{"openai_apikey_cache_creation_as_input": true}, true},
		{"BPS cleanup does not reset API key", map[string]any{"openai_apikey_cache_creation_as_input": true, "openai_excel_bps": false, "openai_excel_bps_cache_creation_as_input": false}, true},
		{"explicit disable overrides legacy", map[string]any{"openai_apikey_cache_creation_as_input": false, "openai_excel_bps_cache_creation_as_input": true}, false},
		{"legacy remains compatible", map[string]any{"openai_excel_bps_cache_creation_as_input": true}, true},
		{"invalid is not enabled", map[string]any{"openai_apikey_cache_creation_as_input": "true"}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Extra: tt.extra}
			require.Equal(t, tt.want, a.IsAPIKeyCacheCreationAsInputEnabled())
			require.False(t, a.IsExcelBPSEnabled())
		})
	}
}

func TestUpdateAccountAPIKeyIndependentCacheCreationSetting(t *testing.T) {
	account := &Account{ID: 51, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "test-only"}, Extra: map[string]any{"openai_excel_bps_cache_creation_as_input": true}}
	repo := &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{account.ID: account}}
	svc := &adminServiceImpl{accountRepo: repo}
	for _, enabled := range []bool{false, true, false} {
		updated, err := svc.UpdateAccount(context.Background(), account.ID, &UpdateAccountInput{Extra: map[string]any{
			"openai_apikey_cache_creation_as_input": enabled, "openai_excel_bps_cache_creation_as_input": true,
		}})
		require.NoError(t, err)
		require.Equal(t, enabled, updated.IsAPIKeyCacheCreationAsInputEnabled())
		require.Equal(t, enabled, repo.accounts[account.ID].IsAPIKeyCacheCreationAsInputEnabled())
	}
}
