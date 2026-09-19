package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestNativeConvergenceFirstForwardAfterLocalWait(t *testing.T) {
	now := time.Now()
	budget := newOpenAIRetryBudget(openAIRetryBudgetConfigForRequest(nil, service.PlatformOpenAI, false), func() time.Time { return now })
	now = now.Add(6 * time.Second)
	require.True(t, budget.ConsumeAttempt(1), "local queue must not consume a first-forward retry deadline")
	require.True(t, budget.CanSwitch(2, false, false), "native handler owns retry count; elapsed first response must not add a second deadline")
	require.False(t, budget.CanSwitch(2, true, false), "output-start safety veto remains")
}

func TestNativeConvergenceRetryScope(t *testing.T) {
	for _, tc := range []struct {
		name, platform string
		image, native  bool
	}{
		{"text", service.PlatformOpenAI, false, true},
		{"images", service.PlatformOpenAI, true, false},
		{"grok", service.PlatformGrok, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now()
			budget := newOpenAIRetryBudget(openAIRetryBudgetConfigForRequest(nil, tc.platform, tc.image), func() time.Time { return now })
			now = now.Add(6 * time.Second)
			require.Equal(t, tc.native, budget.ConsumeAttempt(1))
			require.False(t, budget.CanSwitch(2, true, false))
		})
	}
}
func TestNativeConvergenceConfiguredSwitchesRemainScoped(t *testing.T) {
	for _, configured := range []int{0, 2, 8} {
		cfg := &config.Config{}
		cfg.Gateway.MaxAccountSwitches = configured
		h := NewOpenAIGatewayHandler(nil, nil, nil, nil, nil, nil, nil, nil, cfg)
		want := configured
		if want == 0 {
			want = 3
		}
		require.Equal(t, want, h.requestMaxAccountSwitches(service.PlatformOpenAI, false))
		legacy := configured
		if legacy <= 0 || legacy > 4 {
			legacy = 4
		}
		require.Equal(t, legacy, h.requestMaxAccountSwitches(service.PlatformOpenAI, true))
		require.Equal(t, legacy, h.requestMaxAccountSwitches(service.PlatformGrok, false))
	}
}
