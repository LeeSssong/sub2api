package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestQualityModelRemovalRejectsNativeFallbacks(t *testing.T) {
	for _, tc := range []struct{ name, platform, kind, selected, remaining string }{
		{"bedrock default", service.PlatformAnthropic, service.AccountTypeBedrock, "claude-sonnet-4-5", "other"},
		{"anthropic oauth alias", service.PlatformAnthropic, service.AccountTypeOAuth, "claude-sonnet-4-5", "claude-sonnet-4-5-20250929"},
		{"antigravity prefix", service.PlatformAntigravity, service.AccountTypeOAuth, "models/custom-model", "custom-model"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := map[string]any{tc.selected: tc.selected, tc.remaining: tc.remaining}
			a := &service.Account{Platform: tc.platform, Type: tc.kind, Credentials: map[string]any{"model_mapping": original}}
			_, ok := removeQualityModels(a, []string{tc.selected})
			require.False(t, ok, "native scheduler would still route the selected model")
			require.Equal(t, original, a.Credentials["model_mapping"], "reject without changes")
		})
	}
}
