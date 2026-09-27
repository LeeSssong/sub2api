package service

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestExcelBPSRecoveryPreservesConfigurationAndFallbackMapping(t *testing.T) {
	a := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{"openai_excel_bps": true, "openai_excel_bps_models": []string{"astra"}, "openai_excel_bps_fallback_models": []string{"gpt-6-sol"}, "openai_excel_bps_recovery": map[string]any{"active": true}}, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-6-sol": "astra"}}}
	require.False(t, a.IsExcelBPSEnabled())
	require.True(t, a.IsModelSupported("gpt-6-sol"))
	require.False(t, a.IsModelSupported("astra"))
	require.Equal(t, "gpt-6-sol", a.GetMappedModel("gpt-6-sol"))
	delete(a.Extra, "openai_excel_bps_recovery")
	require.True(t, a.IsExcelBPSEnabled())
	require.Equal(t, "astra", a.GetMappedModel("gpt-6-sol"))
}
func TestExcelBPSRecoveryBackoff(t *testing.T) {
	for failures, want := range []time.Duration{5, 10, 15, 20, 25, 30, 30, 30} {
		require.Equal(t, want*time.Minute, ExcelBPSRecoveryDelay(failures))
	}
}

func TestExcelBPSRecoveryAdminStateAndValidation(t *testing.T) {
	state := map[string]any{"active": true, "generation": "server"}
	current := map[string]any{"openai_excel_bps": true, ExcelBPSShadowRecoveryKey: true, ExcelBPSFallbackModelsKey: []string{"normal"}, ExcelBPSRecoveryKey: state}
	extra := map[string]any{"openai_excel_bps": true, ExcelBPSShadowRecoveryKey: false, ExcelBPSRecoveryKey: map[string]any{"active": false}}
	merged := MergeExcelBPSRecoveryExtra(extra, current)
	require.Equal(t, state, merged[ExcelBPSRecoveryKey])
	require.NoError(t, ValidateExcelBPSRecoveryExtra(map[string]any{ExcelBPSShadowRecoveryKey: false}, current))
	require.NoError(t, ValidateExcelBPSRecoveryExtra(map[string]any{"openai_excel_bps": false}, current))
	require.NotContains(t, MergeExcelBPSRecoveryExtra(map[string]any{"openai_excel_bps": false}, current), ExcelBPSRecoveryKey)
	require.Error(t, ValidateExcelBPSRecoveryExtra(map[string]any{"openai_excel_bps": true, ExcelBPSShadowRecoveryKey: true}, nil))
	require.Error(t, ValidateExcelBPSRecoveryExtra(map[string]any{ExcelBPSFallbackModelsKey: []string{}}, current))
	require.NoError(t, ValidateExcelBPSRecoveryExtra(map[string]any{ExcelBPSFallbackModelsKey: []string{"new"}, ExcelBPSShadowRecoveryKey: false}, current))
}

func TestExcelBPSRecoveryModelsListIgnoresPassthrough(t *testing.T) {
	a := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"model_mapping": map[string]any{"alias": "bps-model"}}, Extra: map[string]any{"openai_excel_bps": true, "openai_passthrough": true, ExcelBPSRecoveryKey: map[string]any{"active": true}, ExcelBPSFallbackModelsKey: []string{"gpt-6-sol"}}}
	body := []byte(`{"object":"list","data":[{"id":"bps-model"},{"id":"gpt-6-sol"}]}`)
	projected, err := projectAccountModelsBody(body, a, nil, false)
	require.NoError(t, err)
	require.JSONEq(t, `{"object":"list","data":[{"id":"gpt-6-sol"}]}`, string(projected))
}
