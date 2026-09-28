package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccountStatsPricingMergedMediaReasoningMultipliers(t *testing.T) {
	imagePrice, videoPrice := 0.08, 0.1
	channel := &Channel{AccountStatsPricingRules: []AccountStatsPricingRule{{
		AccountIDs: []int64{1},
		Pricing: []ChannelModelPricing{
			{Models: []string{"gpt-image-2"}, BillingMode: BillingModeImage,
				ReasoningEffortMultipliers: map[string]float64{"high": 1.5},
				Intervals:                  []PricingInterval{{TierLabel: "2K", PerRequestPrice: &imagePrice}}},
			{Models: []string{"video-model"}, BillingMode: BillingModeVideo,
				ReasoningEffortMultipliers: map[string]float64{"high": 1.5}, PerRequestPrice: &videoPrice},
		},
	}}}
	image := tryCustomRulesResolution(channel, 1, 10, PlatformOpenAI, "gpt-image-2", BillingModeImage, "2K", 2, UsageTokens{}, 2, "high")
	require.True(t, image.Matched)
	require.False(t, image.ApplyAccountRate, "fixed upstream image tier prices do not receive the account rate again")
	require.NotNil(t, image.StatsCost)
	require.InDelta(t, 0.24, *image.StatsCost, 1e-12)
	video := tryCustomRulesResolution(channel, 1, 10, PlatformOpenAI, "video-model", BillingModeVideo, "", 0, UsageTokens{InputTokens: 99}, 2, "high")
	require.True(t, video.Matched)
	require.True(t, video.ApplyAccountRate)
	require.NotNil(t, video.StatsCost)
	require.InDelta(t, 0.3, *video.StatsCost, 1e-12)
}
