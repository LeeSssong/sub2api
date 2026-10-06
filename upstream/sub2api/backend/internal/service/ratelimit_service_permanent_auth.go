package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/tidwall/gjson"
)

const openAIPermanentAuthBlockReason = "openai_permanent_auth_failure"

// HandleOpenAIPermanentAuthFailure handles only an actual HTTP 401 carrying an
// exact structured revocation code. Call before ordinary error-policy gates.
// A true result means this account must not receive another request; it does
// not authorize replaying the failed request or canceling other in-flight work.
func (s *RateLimitService) HandleOpenAIPermanentAuthFailure(ctx context.Context, account *Account, statusCode int, responseBody []byte) bool {
	if s == nil || account == nil || account.Platform != PlatformOpenAI || statusCode != http.StatusUnauthorized || !json.Valid(responseBody) {
		return false
	}
	code := gjson.GetBytes(responseBody, "error.code")
	if !code.Exists() {
		code = gjson.GetBytes(responseBody, "response.error.code")
	}
	if code.Type != gjson.String || (code.Str != "token_revoked" && code.Str != "token_invalidated") {
		return false
	}

	// The upstream rejection remains an account fact even when the client has
	// disconnected. Bound persistence work without inheriting that cancellation.
	stateCtx, cancel := openAIAccountStateContext(ctx)
	defer cancel()
	authAccount := account
	if resolved, err := resolveCredentialAccount(stateCtx, s.accountRepo, account); err == nil && resolved != nil {
		authAccount = resolved
	}

	// Use the native runtime map first, before database/scheduler propagation.
	// Unlike a temporary cooldown, revocation must survive a stale snapshot or
	// failed write. Native credential recovery clears this same runtime block.
	s.notifyAccountSchedulingBlocked(authAccount, time.Time{}, openAIPermanentAuthBlockReason)
	if s.accountRepo != nil {
		// SetError also publishes the scheduler outbox event and syncs its account
		// snapshot. Repeated rejections keep the same disabled state.
		s.handleAuthError(stateCtx, authAccount, "Token revoked (401): "+code.Str+"; reauthorization required")
	}
	// A failed SetError must not prevent stale OAuth credentials being evicted.
	if s.tokenCacheInvalidator != nil {
		if err := s.tokenCacheInvalidator.InvalidateToken(stateCtx, authAccount); err != nil {
			slog.Warn("oauth_permanent_auth_invalidate_cache_failed", "account_id", authAccount.ID, "code", code.Str, "error", err)
		}
	}
	return true
}
