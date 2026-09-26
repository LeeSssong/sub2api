package service

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// Keep the scheduler and token invalidator real; only replace their external
// persistence boundaries so a stale scheduling snapshot can be tested directly.
type permanentAuthRepo struct {
	AccountRepository
	mu             sync.Mutex
	account        Account
	setErrorErr    error
	beforeSetError func(context.Context)
	tempCalls      int
}

func (r *permanentAuthRepo) SetError(ctx context.Context, id int64, message string) error {
	if r.beforeSetError != nil {
		r.beforeSetError(ctx)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.setErrorErr != nil {
		return r.setErrorErr
	}
	r.account.Status, r.account.Schedulable, r.account.ErrorMessage = StatusError, false, message
	return nil
}

func (r *permanentAuthRepo) GetByID(context.Context, int64) (*Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a := r.account
	return &a, nil
}

func (r *permanentAuthRepo) ClearError(context.Context, int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.account.Status, r.account.Schedulable, r.account.ErrorMessage = StatusActive, true, ""
	return nil
}

func (r *permanentAuthRepo) SetTempUnschedulable(context.Context, int64, time.Time, string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tempCalls++
	return nil
}

type permanentAuthTokenCache struct {
	GeminiTokenCache
	mu     sync.Mutex
	tokens map[string]string
}

func (c *permanentAuthTokenCache) DeleteAccessToken(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.tokens, key)
	return nil
}

func (c *permanentAuthTokenCache) GetAccessToken(_ context.Context, key string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tokens[key], nil
}

func newPermanentAuthTestService() (*RateLimitService, *OpenAIGatewayService, *permanentAuthRepo, *permanentAuthTokenCache, *Account) {
	account := &Account{ID: 398, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true,
		Credentials: map[string]any{"access_token": "revoked-test-token", "refresh_token": "test-refresh-token"}}
	repo := &permanentAuthRepo{account: *account}
	tokens := &permanentAuthTokenCache{tokens: map[string]string{"openai:account:398": "revoked-test-token"}}
	svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	gateway := &OpenAIGatewayService{accountRepo: repo, rateLimitService: svc}
	svc.SetAccountRuntimeBlocker(gateway)
	svc.SetTokenCacheInvalidator(NewCompositeTokenCacheInvalidator(tokens))
	return svc, gateway, repo, tokens, account
}

func TestPermanentAuthFailure_PreemptsPoliciesAndInvalidatesToken(t *testing.T) {
	for _, code := range []string{"token_revoked", "token_invalidated"} {
		for _, policy := range []string{"default", "pool", "custom_skip"} {
			t.Run(code+"/"+policy, func(t *testing.T) {
				svc, gateway, repo, tokens, account := newPermanentAuthTestService()
				switch policy {
				case "pool":
					account.Type = AccountTypeAPIKey
					account.Credentials["pool_mode"] = true
					require.True(t, account.IsPoolMode())
				case "custom_skip":
					account.Type = AccountTypeAPIKey
					account.Credentials["custom_error_codes_enabled"] = true
					account.Credentials["custom_error_codes"] = []any{float64(http.StatusTooManyRequests)}
					require.False(t, account.ShouldHandleErrorCode(http.StatusUnauthorized))
				}
				blockedBeforeWrite := false
				repo.beforeSetError = func(context.Context) {
					blockedBeforeWrite = gateway.isOpenAIAccountRequestRuntimeBlocked(account, "gpt-5.5", false)
				}
				handled := svc.HandleUpstreamError(context.Background(), account, http.StatusUnauthorized, nil, []byte(`{"error":{"code":"`+code+`","message":"credential revoked"}}`))
				require.True(t, handled)
				require.True(t, blockedBeforeWrite, "a stale active snapshot must stop scheduling before persistence")
				require.True(t, gateway.isOpenAIAccountRequestRuntimeBlocked(account, "gpt-5.5", false))
				stored, err := repo.GetByID(context.Background(), account.ID)
				require.NoError(t, err)
				require.False(t, stored.IsSchedulable())
				require.Equal(t, StatusError, stored.Status)
				cached, err := tokens.GetAccessToken(context.Background(), "openai:account:398")
				require.NoError(t, err)
				if account.IsOAuth() {
					require.Empty(t, cached)
				}
				require.True(t, account.IsSchedulable(), "do not mutate an in-flight request's account snapshot")
			})
		}
	}
}

func TestPermanentAuthFailure_PersistenceFailureKeepsSchedulingBlocked(t *testing.T) {
	svc, gateway, repo, tokens, account := newPermanentAuthTestService()
	repo.setErrorErr = errors.New("database unavailable")
	entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	repo.beforeSetError = func(context.Context) { close(entered); <-release }
	go func() {
		defer close(finished)
		svc.HandleUpstreamError(context.Background(), account, http.StatusUnauthorized, nil, []byte(`{"error":{"code":"token_revoked"}}`))
	}()
	<-entered
	blockedDuringWrite := gateway.isOpenAIAccountRequestRuntimeBlocked(account, "gpt-5.5", false)
	close(release)
	<-finished
	require.True(t, blockedDuringWrite)
	require.True(t, gateway.isOpenAIAccountRequestRuntimeBlocked(account, "gpt-5.5", false))
	cached, err := tokens.GetAccessToken(context.Background(), "openai:account:398")
	require.NoError(t, err)
	require.Empty(t, cached, "a database failure must not skip token invalidation")
	stored, err := repo.GetByID(context.Background(), account.ID)
	require.NoError(t, err)
	require.Equal(t, StatusActive, stored.Status, "do not claim failed persistence succeeded")
}

func TestPermanentAuthFailure_ConcurrentRejectionsLeaveOneBlockedAccount(t *testing.T) {
	svc, gateway, repo, tokens, account := newPermanentAuthTestService()
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			svc.HandleUpstreamError(context.Background(), account, http.StatusUnauthorized, nil, []byte(`{"error":{"code":"token_revoked"}}`))
		}()
	}
	wg.Wait()
	require.True(t, gateway.isOpenAIAccountRequestRuntimeBlocked(account, "gpt-5.5", false))
	stored, err := repo.GetByID(context.Background(), account.ID)
	require.NoError(t, err)
	require.False(t, stored.IsSchedulable())
	cached, err := tokens.GetAccessToken(context.Background(), "openai:account:398")
	require.NoError(t, err)
	require.Empty(t, cached)
	result, err := svc.RecoverAccountState(context.Background(), account.ID, AccountRecoveryOptions{InvalidateToken: true})
	require.NoError(t, err)
	require.True(t, result.ClearedError)
	require.False(t, gateway.isOpenAIAccountRequestRuntimeBlocked(account, "gpt-5.5", false), "native explicit recovery must clear the same block")
}

func TestPermanentAuthFailure_RejectsMessageTextAndMalformedJSON(t *testing.T) {
	for _, body := range []string{
		`{"error":{"message":"token_revoked"}}`,
		`{"error":{"message":"{\"error\":{\"code\":\"token_revoked\"}}"}}`,
		`{"error":{"code":"token_revoked"}}trailing`,
		`{"error":{"code":"TOKEN_REVOKED"}}`,
		`{"error":{"code":"token_revoked "}}`,
	} {
		t.Run(body, func(t *testing.T) {
			svc, _, repo, _, account := newPermanentAuthTestService()
			svc.HandleUpstreamError(context.Background(), account, http.StatusUnauthorized, nil, []byte(body))
			stored, err := repo.GetByID(context.Background(), account.ID)
			require.NoError(t, err)
			require.Equal(t, StatusActive, stored.Status, "ordinary OAuth 401 must retain refresh recovery")
			require.Equal(t, 1, repo.tempCalls)
		})
	}
}

func TestPermanentAuthFailure_SelectorSkipsStaleRevokedAccount(t *testing.T) {
	svc, gateway, repo, _, account := newPermanentAuthTestService()
	account.Concurrency = 1
	healthy := *account
	healthy.ID, healthy.Priority = 399, 1
	// Both snapshots deliberately remain active, including when SetError fails.
	gateway.accountRepo = schedulerTestOpenAIAccountRepo{accounts: []Account{*account, healthy}}
	gateway.cache = &schedulerTestGatewayCache{}
	gateway.cfg = &config.Config{}
	gateway.concurrencyService = NewConcurrencyService(schedulerTestConcurrencyCache{})
	repo.setErrorErr = errors.New("database unavailable")
	selectAccount := func() (*AccountSelectionResult, error) {
		result, _, err := gateway.SelectAccountWithScheduler(context.Background(), nil, "", "", "gpt-5.5", nil, OpenAIUpstreamTransportAny, false)
		return result, err
	}
	before, err := selectAccount()
	require.NoError(t, err)
	require.Equal(t, int64(398), before.Account.ID)
	if before.ReleaseFunc != nil {
		before.ReleaseFunc()
	}
	svc.HandleUpstreamError(context.Background(), account, http.StatusUnauthorized, nil, []byte(`{"error":{"code":"token_revoked"}}`))
	after, err := selectAccount()
	require.NoError(t, err)
	require.Equal(t, int64(399), after.Account.ID)
	if after.ReleaseFunc != nil {
		after.ReleaseFunc()
	}
	gateway.accountRepo = schedulerTestOpenAIAccountRepo{accounts: []Account{*account}}
	_, err = selectAccount()
	require.Error(t, err, "an empty healthy pool must not select the revoked account")
}

func TestPermanentAuthFailure_OnlyHandlesReal401StructuredCodes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"stream_200", http.StatusOK, `{"error":{"code":"token_revoked"}}`},
		{"forbidden", http.StatusForbidden, `{"error":{"code":"token_revoked"}}`},
		{"rate_limit", http.StatusTooManyRequests, `{"error":{"code":"token_revoked"}}`},
		{"ordinary_401", http.StatusUnauthorized, `{"error":{"code":"invalid_token"}}`},
		{"message_only", http.StatusUnauthorized, `{"error":{"message":"token_revoked"}}`},
		{"array_code", http.StatusUnauthorized, `{"error":{"code":["token_revoked"]}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, gateway, _, tokens, account := newPermanentAuthTestService()
			require.False(t, svc.HandleOpenAIPermanentAuthFailure(context.Background(), account, tc.status, []byte(tc.body)))
			require.False(t, gateway.isOpenAIAccountRuntimeBlocked(account))
			cached, err := tokens.GetAccessToken(context.Background(), "openai:account:398")
			require.NoError(t, err)
			require.Equal(t, "revoked-test-token", cached)
		})
	}
}

func TestPermanentAuthFailure_NestedCodePersistsAfterClientCancellation(t *testing.T) {
	svc, gateway, repo, _, account := newPermanentAuthTestService()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var persistContextError error
	repo.beforeSetError = func(ctx context.Context) { persistContextError = ctx.Err() }
	require.True(t, svc.HandleOpenAIPermanentAuthFailure(ctx, account, http.StatusUnauthorized, []byte(`{"response":{"error":{"code":"token_invalidated"}}}`)))
	require.NoError(t, persistContextError)
	_, err := gateway.latestOpenAITurnAccount(context.Background(), nil, account)
	require.True(t, IsOpenAITurnAdmissionError(err), "new sends must reject even the stale selected account")
	// A later transient 429 or ordinary auth bridge must never shorten revocation.
	gateway.BlockAccountScheduling(account, time.Now().Add(time.Minute), "429")
	require.True(t, gateway.isOpenAIAccountRequestRuntimeBlocked(account, "gpt-5.5", false))
}

func TestPermanentAuthFailure_ShadowUsesCredentialOwnerBlock(t *testing.T) {
	svc, gateway, repo, tokens, account := newPermanentAuthTestService()
	shadow := *account
	shadow.ID, shadow.ParentAccountID, shadow.Credentials = 400, &account.ID, nil
	repo.setErrorErr = errors.New("database unavailable")
	require.True(t, svc.HandleOpenAIPermanentAuthFailure(context.Background(), &shadow, http.StatusUnauthorized, []byte(`{"error":{"code":"token_revoked"}}`)))
	require.True(t, gateway.isOpenAIAccountRequestRuntimeBlocked(account, "gpt-5.5", false))
	require.True(t, gateway.isOpenAIAccountRequestRuntimeBlocked(&shadow, "gpt-5.3-codex-spark", false), "a stale shadow shares its owner's revoked credentials")
	_, err := gateway.latestOpenAITurnAccount(context.Background(), nil, &shadow)
	require.True(t, IsOpenAITurnAdmissionError(err))
	cached, err := tokens.GetAccessToken(context.Background(), "openai:account:398")
	require.NoError(t, err)
	require.Empty(t, cached)
	gateway.ClearAccountSchedulingBlock(account.ID)
	require.False(t, gateway.isOpenAIAccountRequestRuntimeBlocked(&shadow, "gpt-5.3-codex-spark", false), "owner recovery must not leave a separate permanent shadow block")
}
