//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// b5d4590fff deliberately removed the T105 persisted-observation/group-refresh
// strategy. Keep coverage on its replacement: a bounded same-account retry
// window, followed by an ordinary runtime block that is never cleared early.
func TestOpenAIOAuth429CooldownCurrentPolicy(t *testing.T) {
	for _, tc := range []struct {
		name          string
		expired       bool
		headers       http.Header
		wantBlocked   bool
		wantRemaining time.Duration
	}{
		{name: "transient_within_retry_window"},
		{name: "expired_window_uses_short_fallback", expired: true, wantBlocked: true, wantRemaining: openAIOAuth429FallbackCooldown},
		{name: "retry_after_does_not_override_runtime_fallback", expired: true, headers: http.Header{"Retry-After": {"90"}}, wantBlocked: true, wantRemaining: openAIOAuth429FallbackCooldown},
		{name: "reliable_five_seconds_is_still_a_block", expired: true, headers: http.Header{"Retry-After": {"5"}}, wantBlocked: true, wantRemaining: 5 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &OpenAIGatewayService{}
			account := &Account{ID: 701, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
			if tc.expired {
				svc.openaiOAuth429RetryStartedAt.Store(account.ID, time.Now().Add(-openAIOAuth429RetryWindow-time.Second))
			}
			before := time.Now()
			svc.markOpenAIOAuth429RateLimited(context.Background(), account, tc.headers, []byte(`{"error":{"type":"rate_limit_error","message":"try again"}}`))
			require.Equal(t, tc.wantBlocked, svc.isOpenAIAccountRuntimeBlocked(account))
			require.Equal(t, !tc.wantBlocked, svc.ShouldRetryOpenAIOAuth429(account, tc.headers, nil))
			if tc.wantBlocked {
				value, ok := svc.openaiAccountRuntimeBlockUntil.Load(account.ID)
				require.True(t, ok)
				require.WithinDuration(t, before.Add(tc.wantRemaining), value.(time.Time), time.Second)
				// A second failure must not reopen the retry window or clear the block.
				require.False(t, svc.shouldRetryOpenAIOAuth429OnSameAccount(account, http.StatusTooManyRequests, false))
				require.True(t, svc.isOpenAIAccountRuntimeBlocked(account))
			}
		})
	}
}

func TestOpenAIOAuth429CooldownDoesNotReclassifyOtherAccounts(t *testing.T) {
	svc := &OpenAIGatewayService{}
	for _, account := range []*Account{
		{ID: 702, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
		{ID: 703, Platform: PlatformGrok, Type: AccountTypeOAuth},
	} {
		svc.markOpenAIOAuth429RateLimited(context.Background(), account, http.Header{}, nil)
		require.False(t, svc.isOpenAIAccountRuntimeBlocked(account))
		require.False(t, svc.ShouldRetryOpenAIOAuth429(account, http.Header{}, nil))
	}
}
