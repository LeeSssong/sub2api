package migrations

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRemoveCustomSchedulerArtifactsMigration(t *testing.T) {
	content, err := FS.ReadFile("240_remove_custom_scheduler_artifacts.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "DROP TABLE IF EXISTS openai_scheduler_logs")

	deleteStatement := regexp.MustCompile(`(?i)DELETE FROM settings WHERE key IN \(([^;]+)\)`).FindString(sql)
	require.NotEmpty(t, deleteStatement)
	for _, key := range []string{
		"openai_advanced_scheduler_candidate_pool_mode",
		"openai_advanced_scheduler_exploration_ratio",
		"openai_advanced_scheduler_starvation_threshold_seconds",
		"openai_advanced_scheduler_fairness_weight",
		"openai_advanced_scheduler_group_overrides",
		"openai_advanced_scheduler_group_policies",
		"openai_advanced_scheduler_custom_presets",
	} {
		require.Contains(t, deleteStatement, "'"+key+"'")
	}
	for _, retainedKey := range []string{
		"openai_advanced_scheduler_enabled",
		"openai_advanced_scheduler_sticky_weighted_enabled",
		"openai_advanced_scheduler_subscription_priority_enabled",
		"openai_advanced_scheduler_lb_top_k",
		"openai_advanced_scheduler_weight_priority",
	} {
		require.NotContains(t, deleteStatement, "'"+retainedKey+"'")
	}
}
