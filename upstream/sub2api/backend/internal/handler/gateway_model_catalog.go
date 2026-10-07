package handler

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// nativeModelIDsForListing is the ordinary /v1/models source shared with the
// user group catalogue. The bool preserves the native default metadata path.
// User-specific denied models are applied by each response writer afterwards.
func (h *GatewayHandler) nativeModelIDsForListing(ctx context.Context, groupID *int64, platform string, allowlist service.GroupModelAllowlist) ([]string, bool) {
	var configured []string
	if platform == service.PlatformComposite {
		configured = h.compositeAvailableModels(ctx, groupID, true)
	} else {
		configured = h.gatewayService.GetAvailableModels(ctx, groupID, platform)
	}
	if allowlist.Enabled {
		source := modelListingSource(platform, configured, defaultModelIDsForPlatform(platform))
		return allowlist.FilterForListing(source), len(configured) > 0
	}
	if len(configured) > 0 {
		return configured, true
	}
	// Preserve the ordinary endpoint's existing fallback for platforms without
	// a dedicated response branch; do not substitute the Codex client catalog.
	switch platform {
	case service.PlatformComposite, service.PlatformOpenAI, service.PlatformGemini, service.PlatformGrok, service.PlatformTypeSafe:
		return defaultModelIDsForPlatform(platform), false
	default:
		return defaultModelIDsForPlatform(service.PlatformAnthropic), false
	}
}
