package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type guardNativeUpstream struct {
	status  int
	body    string
	request *http.Request
	proxy   string
}

func (u *guardNativeUpstream) Do(req *http.Request, p string, _ int64, _ int) (*http.Response, error) {
	u.request = req
	u.proxy = p
	return &http.Response{StatusCode: u.status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(u.body))}, nil
}
func (u *guardNativeUpstream) DoWithTLS(req *http.Request, p string, id int64, c int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, p, id, c)
}

func guardNativeAccount() Account {
	return Account{ID: 1, Name: "renamed-account", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"email": "user@example.com", "access_token": "old-token", "chatgpt_account_id": "account-one"}}
}
func guardNativeCredential(email, id string, expiry time.Time) map[string]any {
	claims, _ := json.Marshal(map[string]any{"email": email, "exp": expiry.Unix(), "https://api.openai.com/auth": map[string]any{"chatgpt_account_id": id}})
	return map[string]any{"access_token": "new-token", "refresh_token": "new-refresh", "id_token": "e30." + base64.RawURLEncoding.EncodeToString(claims) + ".signature", "expires_at": expiry.Format(time.RFC3339)}
}
func TestTokenGuardNativeProbeUsesAccountIdentityAndClassifiesWithoutWrites(t *testing.T) {
	for _, tc := range []struct {
		status      int
		body, state string
	}{
		{200, "data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n", AccountTokenGuardProbeOK},
		{401, "private-token", AccountTokenGuardProbeAuth},
		{403, "private-token", AccountTokenGuardProbeTransient},
		{429, "private-token", AccountTokenGuardProbeTransient},
		{200, "data: {\"type\":\"response.failed\",\"response\":{\"error\":{\"code\":\"invalid_token\"}}}\n", AccountTokenGuardProbeAuth},
		{200, "data: [DONE]\n", AccountTokenGuardProbeTransient},
	} {
		upstream := &guardNativeUpstream{status: tc.status, body: tc.body}
		svc := &AccountTestService{httpUpstream: upstream}
		account := guardNativeAccount()
		result := svc.ProbeTokenGuardAccount(context.Background(), &account, "gpt-6-astra")
		require.Equal(t, tc.state, result.State)
		require.NotContains(t, result.Detail, "private-token")
		require.Equal(t, "Bearer old-token", upstream.request.Header.Get("Authorization"))
		require.Equal(t, "account-one", upstream.request.Header.Get("Chatgpt-Account-Id"))
		require.Equal(t, chatgptCodexAPIURL, upstream.request.URL.String())
		require.NotEmpty(t, upstream.request.Header.Get("Originator"))
		require.NotEmpty(t, upstream.request.Header.Get("User-Agent"))
	}
}
func TestTokenGuardNewNativeAndLegacyExternalDefaults(t *testing.T) {
	settings := &accountOpsSettingsStub{}
	svc := NewAccountTokenGuardService(settings, nil, nil, nil, nil)
	cfg, err := svc.GetConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, "native", cfg.Mode)
	cfg.Enabled = true
	require.NoError(t, ValidateAccountTokenGuardConfig(cfg))
	settings.raw = `{"interval_seconds":300,"probe_model":"gpt-6-astra"}`
	cfg, err = svc.GetConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, "external", cfg.Mode)
}
func TestTokenGuardBindingIsStableAndRejectsAmbiguity(t *testing.T) {
	account := guardNativeAccount()
	accounts := &tokenGuardTestAccounts{items: []Account{account}}
	svc := NewAccountTokenGuardService(&accountOpsSettingsStub{}, nil, accounts, nil, nil)
	cfg := defaultAccountTokenGuardConfig()
	cfg.ReloginAccounts = []AccountTokenGuardReloginAccount{{Email: "user@example.com", Password: " padded ", MFASecret: "JBSWY3DPEHPK3PXP"}}
	saved, err := svc.SaveConfig(context.Background(), cfg)
	require.NoError(t, err)
	require.Equal(t, int64(1), saved.ReloginAccounts[0].AccountID)
	require.Equal(t, " padded ", saved.ReloginAccounts[0].Password)
	_, found := findBoundGuardReloginAccount(saved, 1)
	require.True(t, found)
	duplicate := account
	duplicate.ID = 2
	accounts.items = append(accounts.items, duplicate)
	_, err = svc.SaveConfig(context.Background(), cfg)
	require.ErrorContains(t, err, "待绑定")
	cfg.ReloginAccounts[0].AccountID = 1
	cfg.ReloginAccounts[0].MFASecret = "bad-2fa"
	_, err = svc.SaveConfig(context.Background(), cfg)
	require.ErrorContains(t, err, "Base32")
	cfg.ReloginAccounts[0].MFASecret = "JBSWY3DPEHPK3PXP"
	cfg.ReloginAccounts[0].Email = "other@example.com"
	_, err = svc.SaveConfig(context.Background(), cfg)
	require.ErrorContains(t, err, "不一致")
}
func TestTokenGuardCredentialIdentityAndExpiry(t *testing.T) {
	a := guardNativeAccount()
	entry := AccountTokenGuardReloginAccount{AccountID: 1, Email: "user@example.com"}
	for _, tc := range []struct {
		email, id string
		expiry    time.Time
		valid     bool
	}{
		{"user@example.com", "account-one", time.Now().Add(time.Hour), true},
		{"other@example.com", "account-one", time.Now().Add(time.Hour), false},
		{"user@example.com", "account-two", time.Now().Add(time.Hour), false},
		{"user@example.com", "account-one", time.Now().Add(-time.Hour), false},
	} {
		err := validateGuardCredentialIdentity(guardNativeCredential(tc.email, tc.id, tc.expiry), entry, &a)
		if tc.valid {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}
}

type guardNativeWriter struct {
	*tokenGuardTestRepo
	accounts           *tokenGuardTestAccounts
	writes, recoveries int
	conflict           bool
}

func (r *guardNativeWriter) UpdateCredentialsIfUnchanged(_ context.Context, _ *Account, patch map[string]any) (bool, error) {
	r.writes++
	if r.conflict {
		return false, nil
	}
	for k, v := range patch {
		r.accounts.items[0].Credentials[k] = v
	}
	return true, nil
}
func (r *guardNativeWriter) RecoverTokenGuardOwnedState(context.Context, *Account) (bool, error) {
	r.recoveries++
	return true, nil
}
func TestTokenGuardNativeReloginPostProbeAndCooldown(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		require.Equal(t, "Bearer executor-key", r.Header.Get("Authorization"))
		var input map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&input))
		require.Equal(t, " padded ", input["password"])
		require.Equal(t, "account-one", input["expected_account_id"])
		_ = json.NewEncoder(w).Encode(map[string]any{"type": "result", "payload": map[string]any{"credential": guardNativeCredential("user@example.com", "account-one", time.Now().Add(time.Hour))}})
	}))
	defer server.Close()
	t.Setenv("TOKEN_GUARD_RELOGIN_URL", server.URL)
	t.Setenv("TOKEN_GUARD_RELOGIN_KEY", "executor-key")
	a := guardNativeAccount()
	accounts := &tokenGuardTestAccounts{items: []Account{a}}
	repo := &guardNativeWriter{tokenGuardTestRepo: &tokenGuardTestRepo{states: map[int64]AccountTokenGuardState{}, acquired: true}, accounts: accounts}
	cfg := defaultAccountTokenGuardConfig()
	cfg.Enabled = true
	cfg.AutoRelogin = true
	cfg.RestoreSchedulable = true
	cfg.ReloginAccounts = []AccountTokenGuardReloginAccount{{AccountID: 1, Email: "user@example.com", Password: " padded ", MFASecret: "JBSWY3DPEHPK3PXP"}}
	raw, _ := json.Marshal(cfg)
	svc := NewAccountTokenGuardService(&accountOpsSettingsStub{raw: string(raw)}, repo, accounts, nil, nil)
	svc.nativeProbe = func(context.Context, *Account, string) AccountTokenGuardProbeResult {
		return AccountTokenGuardProbeResult{State: AccountTokenGuardProbeAuth}
	}
	_, err := svc.ReloginAccount(context.Background(), 1)
	require.ErrorContains(t, err, "复测未通过")
	require.Equal(t, 1, repo.writes)
	require.Zero(t, repo.recoveries)
	_, err = svc.RunCycle(context.Background(), false)
	require.NoError(t, err)
	require.Equal(t, 1, calls, "automatic retry must respect cooldown")
	svc.nativeProbe = func(context.Context, *Account, string) AccountTokenGuardProbeResult {
		return AccountTokenGuardProbeResult{State: AccountTokenGuardProbeOK}
	}
	_, err = svc.ReloginAccount(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, 2, calls, "manual retry bypasses cooldown")
	require.Equal(t, 1, repo.recoveries)
	repo.conflict = true
	_, err = svc.ReloginAccount(context.Background(), 1)
	require.ErrorContains(t, err, "已变更")
	require.Equal(t, 1, repo.recoveries)
	repo.acquired = false
	_, err = svc.ReloginAccount(context.Background(), 1)
	require.Error(t, err)
	require.Equal(t, 3, calls, "lease rejection must not invoke executor")
}

// Ordinary scheduler queries may hide failed accounts. The guard must exclusively
// use its all-status query for both credential binding and future probe cycles.
type guardFullStatusAccounts struct{ *tokenGuardTestAccounts }

func (a *guardFullStatusAccounts) ListByPlatform(context.Context, string) ([]Account, error) {
	panic("scheduler-only listing used")
}
func (a *guardFullStatusAccounts) ListByGroup(context.Context, int64) ([]Account, error) {
	panic("scheduler-only group listing used")
}
func TestTokenGuardErrorAccountsRemainBindableAndInScope(t *testing.T) {
	account := guardNativeAccount()
	account.Status = StatusError
	account.GroupIDs = []int64{7}
	other := guardNativeAccount()
	other.ID = 2
	other.GroupIDs = []int64{9}
	accounts := &guardFullStatusAccounts{&tokenGuardTestAccounts{items: []Account{account, other}}}
	repo := &tokenGuardTestRepo{states: map[int64]AccountTokenGuardState{}, acquired: true}
	svc := NewAccountTokenGuardService(&accountOpsSettingsStub{}, repo, accounts, nil, nil)
	cfg := defaultAccountTokenGuardConfig()
	cfg.GroupIDs = []int64{7}
	cfg.MaxProbePerCycle = 1
	cfg.ReloginAccounts = []AccountTokenGuardReloginAccount{{AccountID: 1, Email: "user@example.com", Password: " password ", MFASecret: "JBSWY3DP EHPK3PXP"}}
	saved, err := svc.SaveConfig(context.Background(), cfg)
	require.NoError(t, err)
	require.Equal(t, "JBSWY3DPEHPK3PXP", saved.ReloginAccounts[0].MFASecret)
	available, err := svc.availableAccounts(context.Background(), saved)
	require.NoError(t, err)
	require.Len(t, available, 1)
	require.Equal(t, int64(1), available[0].AccountID)
	probed := 0
	svc.nativeProbe = func(ctx context.Context, a *Account, _ string) AccountTokenGuardProbeResult {
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		require.LessOrEqual(t, time.Until(deadline), accountTokenGuardCycleTimeout)
		require.Equal(t, StatusError, a.Status)
		probed++
		return AccountTokenGuardProbeResult{State: AccountTokenGuardProbeAuth}
	}
	_, err = svc.RunCycle(context.Background(), false)
	require.NoError(t, err)
	require.Equal(t, 1, probed)
}
func TestTokenGuardRejectsExecutorInvalidCredentialsOnSave(t *testing.T) {
	a := guardNativeAccount()
	a.Credentials = map[string]any{}
	svc := NewAccountTokenGuardService(&accountOpsSettingsStub{}, nil, &tokenGuardTestAccounts{items: []Account{a}}, nil, nil)
	for _, tc := range []struct{ email, secret string }{
		{"user@example.com", "MY"}, {"user@localhost", "JBSWY3DPEHPK3PXP"}, {"user @example.com", "JBSWY3DPEHPK3PXP"},
	} {
		cfg := defaultAccountTokenGuardConfig()
		cfg.ReloginAccounts = []AccountTokenGuardReloginAccount{{AccountID: 1, Email: tc.email, Password: "password", MFASecret: tc.secret}}
		_, err := svc.SaveConfig(context.Background(), cfg)
		require.Error(t, err)
	}
}

type guardDeadlineRepo struct {
	*tokenGuardTestRepo
	deadline time.Time
}

func (r *guardDeadlineRepo) AcquireTokenGuardLease(ctx context.Context) (func(), bool, error) {
	r.deadline, _ = ctx.Deadline()
	return func() {}, true, nil
}
func TestTokenGuardWholeCycleDeadlineIncludesLeaseAndStopsRepairs(t *testing.T) {
	account := guardNativeAccount()
	accounts := &tokenGuardTestAccounts{items: []Account{account}}
	repo := &guardDeadlineRepo{tokenGuardTestRepo: &tokenGuardTestRepo{states: map[int64]AccountTokenGuardState{}}}
	cfg := defaultAccountTokenGuardConfig()
	raw, _ := json.Marshal(cfg)
	svc := NewAccountTokenGuardService(&accountOpsSettingsStub{raw: string(raw)}, repo, accounts, nil, nil)
	svc.nativeProbe = func(ctx context.Context, _ *Account, _ string) AccountTokenGuardProbeResult {
		<-ctx.Done()
		return AccountTokenGuardProbeResult{State: AccountTokenGuardProbeTransient}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := svc.RunCycle(ctx, true)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.NotZero(t, repo.deadline)
	require.True(t, repo.deadline.Before(time.Now().Add(time.Second)))
	require.Empty(t, repo.states, "expired cycle must not start account mutations")
	accounts.items = nil
	_, err = svc.RunCycle(context.Background(), false)
	require.NoError(t, err)
	require.InDelta(t, (25 * time.Minute).Seconds(), time.Until(repo.deadline).Seconds(), 2, "default bound must include lease and all repair work")

}
