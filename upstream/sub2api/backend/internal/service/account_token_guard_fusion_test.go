package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTokenGuardFusionDefaultsNeedExplicitTrust(t *testing.T) {
	c := defaultAccountTokenGuardConfig()
	require.False(t, c.Enabled)
	require.False(t, c.AutoRelogin)
	require.False(t, c.RestoreSchedulable)
	require.Empty(t, c.ProbeEndpoint)
	require.Empty(t, c.ReloginEndpoint)
	require.Empty(t, c.ProbeHeaders)
	require.NoError(t, ValidateAccountTokenGuardConfig(c))
	for _, endpoint := range []string{"https://user:password@example.com/probe", "https://example.com/probe?token=secret", "https://example.com/probe#secret"} {
		c.ProbeEndpoint = endpoint
		require.Error(t, ValidateAccountTokenGuardConfig(c))
	}
}

func TestTokenGuardFusionMasksAndPreservesSecrets(t *testing.T) {
	repo := &accountOpsSettingsStub{}
	s := NewAccountTokenGuardService(repo, nil, nil, nil, nil)
	cfg := defaultAccountTokenGuardConfig()
	cfg.ProbeEndpoint = "https://trusted.example/probe"
	cfg.ReloginEndpoint = "https://trusted.example/login"
	cfg.ProbeHeaders = map[string]string{"Authorization": "Bearer private-probe"}
	cfg.ReloginHeaders = map[string]string{"X-Api-Key": "private-login"}
	cfg.BarkKey = "private-bark"
	cfg.ReloginAccounts = []AccountTokenGuardReloginAccount{{Email: "user@example.com", Password: "private-password", MFASecret: "private-mfa"}}
	saved, err := s.SaveConfig(context.Background(), cfg)
	require.NoError(t, err)
	require.Equal(t, "********", saved.BarkKey)
	require.Equal(t, "********", saved.ProbeHeaders["Authorization"])
	require.Equal(t, "********", saved.ReloginAccounts[0].Password)
	saved.IntervalSeconds = 600
	_, err = s.SaveConfig(context.Background(), saved)
	require.NoError(t, err)
	internal, err := s.GetConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, "private-bark", internal.BarkKey)
	require.Equal(t, "Bearer private-probe", internal.ProbeHeaders["Authorization"])
	require.Equal(t, "private-password", internal.ReloginAccounts[0].Password)
	require.Equal(t, "private-mfa", internal.ReloginAccounts[0].MFASecret)
}
