package service

import (
	"context"
	"errors"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/tidwall/gjson"
)

const OpenAIFastSupportedKey = "openai_fast_supported"
const OpenAIFastModelsKey = "openai_fast_models"

var ErrOpenAIFastUnavailable = errors.New("Fast is temporarily unavailable for this model; no eligible Fast account")
var ErrOpenAIFastContinuation = errors.New("this continuation is bound to an account that cannot serve Fast; start a new request or disable Fast")

// SupportsOpenAIFastUpstreamModel is an operator declaration, not an inference
// from model names or provider responses. Omitted model scope means all models;
// an explicitly empty or malformed scope enables none.
func (a *Account) SupportsOpenAIFastUpstreamModel(model string) bool {
	if a == nil || !a.IsOpenAI() {
		return false
	}
	enabled, _ := a.Extra[OpenAIFastSupportedKey].(bool)
	if !enabled {
		return false
	}
	raw, scoped := a.Extra[OpenAIFastModelsKey]
	if !scoped {
		return true
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return false
	}
	switch models := raw.(type) {
	case []string:
		for _, selected := range models {
			if strings.TrimSpace(selected) == model {
				return true
			}
		}
	case []any:
		for _, selected := range models {
			if name, ok := selected.(string); ok && strings.TrimSpace(name) == model {
				return true
			}
		}
	}
	return false
}

type openAIFastRoutingContextKey struct{}
type openAIFastCompactContextKey struct{}
type openAIFastRoutingRequest struct {
	tier              string
	policyUnavailable bool
}

// WithOpenAIFastRoutingContext freezes policy once per request. Forwarding reuses
// this snapshot, so candidate filtering never performs per-account DB reads.
func (s *OpenAIGatewayService) WithOpenAIFastRoutingContext(ctx context.Context, body []byte) context.Context {
	request := openAIFastRoutingRequest{tier: normalizedOpenAIServiceTierValue(gjson.GetBytes(body, "service_tier").String())}
	if openAIFastPolicySettingsFromContext(ctx) == nil {
		settings := DefaultOpenAIFastPolicySettings()
		if s != nil && s.settingService != nil {
			fetched, err := s.settingService.GetOpenAIFastPolicySettings(ctx)
			if err != nil {
				request.policyUnavailable = true
			} else if fetched != nil {
				settings = fetched
			}
		}
		ctx = withOpenAIFastPolicyContext(ctx, settings)
	}
	return context.WithValue(ctx, openAIFastRoutingContextKey{}, request)
}

func openAIFastRequested(ctx context.Context) bool {
	request, _ := ctx.Value(openAIFastRoutingContextKey{}).(openAIFastRoutingRequest)
	if request.tier == OpenAIFastTierPriority {
		return true
	}
	group, _ := ctx.Value(ctxkey.Group).(*Group)
	return IsGroupContextValid(group) && groupSupportsOpenAIFast(group.Platform) && group.ForceOpenAIFast
}

func openAIFastRoutingFailureReason(ctx context.Context, account *Account, requestedModel string, compact bool) string {
	if account == nil || !account.IsOpenAI() {
		if openAIFastRequested(ctx) {
			return "fast_not_supported"
		}
		return ""
	}
	if actual, ok := ctx.Value(openAIFastCompactContextKey{}).(bool); ok {
		compact = actual
	}
	_, upstreamModel := resolveOpenAIForwardMappedModels(account, requestedModel, compact)
	return openAIFastUpstreamRoutingFailureReason(ctx, account, upstreamModel)
}

func openAIFastUpstreamRoutingFailureReason(ctx context.Context, account *Account, upstreamModel string) string {
	request, _ := ctx.Value(openAIFastRoutingContextKey{}).(openAIFastRoutingRequest)
	tier := request.tier
	if openAIGroupForcesFast(ctx, account) {
		tier = OpenAIFastTierPriority
	}
	matcher := tier
	if matcher == "" {
		matcher = OpenAIFastTierMissing
	}
	action, _ := evaluateOpenAIFastPolicyWithSettings(openAIFastPolicySettingsFromContext(ctx), openAIFastPolicyUserID(ctx), account, upstreamModel, matcher)
	required := tier == OpenAIFastTierPriority || action == OpenAIFastPolicyActionForcePriority
	if !required {
		return ""
	}
	if request.policyUnavailable {
		return "fast_policy_unavailable"
	}
	if action == BetaPolicyActionFilter || action == BetaPolicyActionBlock {
		return "fast_policy_incompatible"
	}
	if !account.SupportsOpenAIFastUpstreamModel(upstreamModel) {
		return "fast_not_supported"
	}
	return ""
}

// Guard the final outbound tier as well as scheduling: settings or capabilities
// may change during slot waiting, and a WS session may enable Fast on later turns.
func checkOpenAIFastOutbound(ctx context.Context, account *Account, upstreamModel, outboundTier string) *OpenAIFastBlockedError {
	if _, managed := ctx.Value(openAIFastRoutingContextKey{}).(openAIFastRoutingRequest); !managed {
		return nil
	}
	if reason := openAIFastUpstreamRoutingFailureReason(ctx, account, upstreamModel); reason != "" {
		return &OpenAIFastBlockedError{Message: ErrOpenAIFastUnavailable.Error()}
	}
	if openAIFastRequested(ctx) && outboundTier != OpenAIFastTierPriority {
		return &OpenAIFastBlockedError{Message: "Fast cannot be silently downgraded by the upstream policy"}
	}
	if outboundTier == OpenAIFastTierPriority && !account.SupportsOpenAIFastUpstreamModel(upstreamModel) {
		return &OpenAIFastBlockedError{Message: ErrOpenAIFastUnavailable.Error()}
	}
	return nil
}
