package service

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestIntelligenceQualityWindow(t *testing.T) {
	slot := time.Date(2026, 10, 9, 10, 30, 0, 0, time.UTC)
	start, end, err := IntelligenceQualityWindow("*/5 * * * *", slot)
	require.NoError(t, err)
	require.Equal(t, slot.Add(-5*time.Minute), start)
	require.Equal(t, slot, end)
	_, _, err = IntelligenceQualityWindow("0 0 1 1 *", slot)
	require.Error(t, err, "no old annual sample may impersonate current quality")
}

func TestIntelligenceQualityDefaultsAndModelGrouping(t *testing.T) {
	models, err := normalizeIntelligenceModels(nil)
	require.NoError(t, err)
	require.Equal(t, []string{"gpt-6-astra", "gpt-6.1-sol"}, models)
	_, err = normalizeIntelligenceModels([]string{"a", "a"})
	require.Error(t, err)
	for _, verdict := range []string{"passed", "incorrect", "unknown", "abnormal"} {
		dto := intelligencePublicResult(&PelicanGroupTestResult{Status: "failed", PelicanConfig: &PelicanTestConfig{ModelID: "gpt-6.1-sol", IntelligenceResult: &IntelligenceResultMetadata{Source: "quality_ops", Verdict: verdict}}}, false)
		require.Equal(t, verdict, dto.Verdict)
		require.Equal(t, "gpt-6.1-sol", dto.ModelID)
	}
}
