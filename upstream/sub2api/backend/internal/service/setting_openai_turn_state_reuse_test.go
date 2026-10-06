//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpenAITurnStateReuseSettingsDefaultsAndRoundTrip(t *testing.T) {
	repo := newMockSettingRepo()
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetOpenAITurnStateReuseSettings(context.Background())
	require.NoError(t, err)
	require.False(t, settings.Enabled)
	require.Equal(t, OpenAITurnStateHarvestModel, settings.HarvestModel)
	require.True(t, settings.HarvestUseProxyPool)
	require.Equal(t, OpenAITurnStateMissNone, settings.MissAction)
	require.Equal(t, OpenAITurnStateRecoveredNone, settings.RecoveredAction)
	require.False(t, settings.InjectCompact)

	target := int64(17)
	err = svc.SetOpenAITurnStateReuseSettings(context.Background(), &OpenAITurnStateReuseSettings{
		Enabled:                true,
		HarvestModel:           "ignored",
		HarvestProxyURLs:       []string{" http://proxy.example:8080 "},
		MissAction:             OpenAITurnStateMissRebindGroup,
		MissTargetGroupID:      &target,
		RecoveredAction:        OpenAITurnStateRecoveredRestore,
		RecoveredTargetGroupID: &target,
		InjectCompact:          true,
	})
	require.NoError(t, err)

	settings, err = svc.GetOpenAITurnStateReuseSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.Enabled)
	require.Equal(t, OpenAITurnStateHarvestModel, settings.HarvestModel)
	require.Equal(t, []string{"http://proxy.example:8080"}, settings.HarvestProxyURLs)
	require.False(t, settings.InjectCompact)
	require.NotNil(t, settings.MissTargetGroupID)
	require.EqualValues(t, target, *settings.MissTargetGroupID)
	require.Nil(t, settings.RecoveredTargetGroupID)
}

func TestOpenAITurnStateReuseSettingsRequireRebindTargets(t *testing.T) {
	svc := NewSettingService(newMockSettingRepo(), &config.Config{})

	err := svc.SetOpenAITurnStateReuseSettings(context.Background(), &OpenAITurnStateReuseSettings{
		MissAction:      OpenAITurnStateMissRebindGroup,
		RecoveredAction: OpenAITurnStateRecoveredNone,
	})
	require.ErrorContains(t, err, "miss_target_group_id")

	err = svc.SetOpenAITurnStateReuseSettings(context.Background(), &OpenAITurnStateReuseSettings{
		MissAction:      OpenAITurnStateMissNone,
		RecoveredAction: OpenAITurnStateRecoveredRebind,
	})
	require.ErrorContains(t, err, "recovered_target_group_id")
}
