package dto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSystemSettingsOmitsRetiredSchedulerFields(t *testing.T) {
	raw, err := json.Marshal(SystemSettings{})
	require.NoError(t, err)

	payload := string(raw)
	for _, key := range []string{
		"openai_advanced_scheduler_candidate_pool_mode",
		"openai_advanced_scheduler_exploration_ratio",
		"openai_advanced_scheduler_starvation_threshold_seconds",
		"openai_advanced_scheduler_fairness_weight",
		"openai_advanced_scheduler_group_overrides",
		"openai_advanced_scheduler_group_policies",
		"openai_advanced_scheduler_custom_presets",
	} {
		require.NotContains(t, payload, key)
	}

	for _, key := range []string{
		"openai_advanced_scheduler_enabled",
		"openai_advanced_scheduler_sticky_weighted_enabled",
		"openai_advanced_scheduler_subscription_priority_enabled",
		"openai_advanced_scheduler_lb_top_k",
		"openai_advanced_scheduler_weight_priority",
	} {
		require.Contains(t, payload, key)
	}
}
