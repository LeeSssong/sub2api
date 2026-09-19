package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type openAITurnStateRuntimeStoreStub struct {
	ticket OpenAITurnStateTicket
	found  bool
	err    error
	key    OpenAITurnStateKey
}

type openAITurnStateHarvestUpstreamStub struct {
	HTTPUpstream
	responses []*http.Response
	requests  []*http.Request
}

func (s *openAITurnStateHarvestUpstreamStub) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	s.requests = append(s.requests, req)
	if len(s.responses) == 0 {
		return nil, errors.New("unexpected harvest request")
	}
	resp := s.responses[0]
	s.responses = s.responses[1:]
	return resp, nil
}

func TestOpenAITurnStateRuntimeAppliesWebsocketMetadata(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	ticket, err := ParseOpenAITurnStateReuseTicket(makeOpenAITurnStateTicket(217, now.Add(-time.Minute)), now)
	require.NoError(t, err)
	store := &openAITurnStateRuntimeStoreStub{ticket: ticket, found: true}
	settings := NewSettingService(&openAITurnStateSettingRepoStub{value: `{"enabled":true,"harvest_model":"gpt-6-astra","harvest_use_proxy_pool":true,"miss_action":"none","recovered_action":"none"}`}, nil)
	svc := &OpenAIGatewayService{settingService: settings}
	svc.SetOpenAITurnStateStore(store)
	account := &Account{ID: 74, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "access-token", "chatgpt_account_id": "workspace-1"}, Groups: []*Group{{ID: 9, Platform: PlatformOpenAI, TurnStateInjectEnabled: true}}}

	body := []byte(`{"type":"response.create","model":"gpt-6-astra","input":"hello","client_metadata":{"x-codex-turn-state":"client-state"}}`)
	got := svc.applyOpenAITurnStateReuseWebsocket(context.Background(), account, body)
	require.Equal(t, ticket.Raw, gjson.GetBytes(got, "client_metadata.x-codex-turn-state").String())

	nonAstra := []byte(`{"type":"response.create","model":"gpt-5.6-sol","client_metadata":{"x-codex-turn-state":"client-state"}}`)
	require.Equal(t, "client-state", gjson.GetBytes(svc.applyOpenAITurnStateReuseWebsocket(context.Background(), account, nonAstra), "client_metadata.x-codex-turn-state").String())
}

func (s *openAITurnStateRuntimeStoreStub) Get(_ context.Context, key OpenAITurnStateKey, _ time.Time) (OpenAITurnStateTicket, bool, error) {
	s.key = key
	return s.ticket, s.found, s.err
}
func (*openAITurnStateRuntimeStoreStub) Put(context.Context, OpenAITurnStateKey, OpenAITurnStateTicket) (bool, error) {
	return false, nil
}
func (*openAITurnStateRuntimeStoreStub) DeleteIfMatch(context.Context, OpenAITurnStateKey, string) (bool, error) {
	return false, nil
}
func (*openAITurnStateRuntimeStoreStub) AcquireLease(context.Context, OpenAITurnStateKey, string, time.Duration) (bool, error) {
	return false, nil
}
func (*openAITurnStateRuntimeStoreStub) ReleaseLease(context.Context, OpenAITurnStateKey, string) error {
	return nil
}

type openAITurnStateSettingRepoStub struct{ value string }

type openAITurnStateActionRepoStub struct {
	AccountRepository
	account     *Account
	boundGroups []int64
	setReason   string
	clearCalls  int
}

func (r *openAITurnStateActionRepoStub) GetByID(context.Context, int64) (*Account, error) {
	return r.account, nil
}
func (r *openAITurnStateActionRepoStub) BindGroups(_ context.Context, _ int64, groupIDs []int64) error {
	r.boundGroups = append([]int64(nil), groupIDs...)
	return nil
}
func (r *openAITurnStateActionRepoStub) SetTempUnschedulable(_ context.Context, _ int64, _ time.Time, reason string) error {
	r.setReason = reason
	return nil
}
func (r *openAITurnStateActionRepoStub) ClearTempUnschedulable(context.Context, int64) error {
	r.clearCalls++
	return nil
}

func (*openAITurnStateSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}
func (s *openAITurnStateSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if key != SettingKeyOpenAITurnStateReuseSettings || s.value == "" {
		return "", ErrSettingNotFound
	}
	return s.value, nil
}
func (*openAITurnStateSettingRepoStub) Set(context.Context, string, string) error { return nil }
func (*openAITurnStateSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	return nil, nil
}
func (*openAITurnStateSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	return nil
}
func (*openAITurnStateSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return nil, nil
}
func (*openAITurnStateSettingRepoStub) Delete(context.Context, string) error { return nil }

func TestOpenAITurnStateRuntimeAppliesOnlyScopedOAuthAstra(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	ticket, err := ParseOpenAITurnStateReuseTicket(makeOpenAITurnStateTicket(217, now.Add(-time.Minute)), now)
	require.NoError(t, err)
	store := &openAITurnStateRuntimeStoreStub{ticket: ticket, found: true}
	settings := NewSettingService(&openAITurnStateSettingRepoStub{value: `{"enabled":true,"harvest_model":"gpt-6-astra","harvest_use_proxy_pool":true,"miss_action":"none","recovered_action":"none"}`}, nil)
	svc := &OpenAIGatewayService{settingService: settings}
	svc.SetOpenAITurnStateStore(store)
	account := &Account{
		ID: 71, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "access-token", "chatgpt_account_id": "workspace-1"},
		Groups:      []*Group{{ID: 9, Platform: PlatformOpenAI, TurnStateInjectEnabled: true, Hydrated: true}},
	}

	for _, tc := range []struct {
		name    string
		account *Account
		body    []byte
		store   *openAITurnStateRuntimeStoreStub
		want    string
	}{
		{name: "astra", account: account, body: []byte(`{"model":"gpt-6-astra","input":"hello"}`), store: store, want: ticket.Raw},
		{name: "sol", account: account, body: []byte(`{"model":"gpt-5.6-sol","input":"hello"}`), store: store, want: "client-state"},
		{name: "compact", account: account, body: []byte(`{"model":"gpt-6-astra","input":[{"type":"compaction_trigger"}]}`), store: store, want: "client-state"},
		{name: "api key", account: &Account{ID: 72, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, body: []byte(`{"model":"gpt-6-astra"}`), store: store, want: "client-state"},
		{name: "out of scope", account: &Account{ID: 73, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: account.Credentials}, body: []byte(`{"model":"gpt-6-astra"}`), store: store, want: "client-state"},
		{name: "store failure", account: account, body: []byte(`{"model":"gpt-6-astra"}`), store: &openAITurnStateRuntimeStoreStub{err: errors.New("redis down")}, want: "client-state"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc.SetOpenAITurnStateStore(tc.store)
			headers := http.Header{OpenAITurnStateHeader: []string{"client-state"}}
			svc.applyOpenAITurnStateReuse(context.Background(), headers, tc.account, tc.body, "/responses")
			require.Equal(t, tc.want, headers.Get(OpenAITurnStateHeader))
		})
	}
	require.Equal(t, OpenAITurnStateKey{AccountID: account.ID, Model: OpenAITurnStateHarvestModel, CredentialHash: OpenAITurnStateCredentialHash("access-token", "workspace-1")}, store.key)
}

func TestOpenAITurnStateSchedulerEligibilityHonorsMissActionAndRequestScope(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	ticket, err := ParseOpenAITurnStateReuseTicket(makeOpenAITurnStateTicket(217, now.Add(-time.Minute)), now)
	require.NoError(t, err)
	account := &Account{
		ID: 81, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "access-token", "chatgpt_account_id": "workspace-1"},
		Groups:      []*Group{{ID: 9, Platform: PlatformOpenAI, TurnStateInjectEnabled: true}},
	}
	request := OpenAIAccountScheduleRequest{Platform: PlatformOpenAI, RequestedModel: OpenAITurnStateHarvestModel}
	marked := WithOpenAITurnStateReuseScheduling(context.Background(), true)

	newScheduler := func(settingsJSON string, store *openAITurnStateRuntimeStoreStub) *defaultOpenAIAccountScheduler {
		svc := &OpenAIGatewayService{settingService: NewSettingService(&openAITurnStateSettingRepoStub{value: settingsJSON}, nil)}
		svc.SetOpenAITurnStateStore(store)
		return &defaultOpenAIAccountScheduler{service: svc}
	}

	rebind := `{"enabled":true,"harvest_model":"gpt-6-astra","harvest_use_proxy_pool":true,"miss_action":"rebind_group","miss_target_group_id":9,"recovered_action":"none"}`
	compatible, reason := newScheduler(rebind, &openAITurnStateRuntimeStoreStub{}).isAccountRequestCompatibleReason(marked, account, request)
	require.False(t, compatible)
	require.Equal(t, "turn_state_missing", reason)

	for _, tc := range []struct {
		name     string
		ctx      context.Context
		account  *Account
		request  OpenAIAccountScheduleRequest
		settings string
		store    *openAITurnStateRuntimeStoreStub
	}{
		{name: "ticket present", ctx: marked, account: account, request: request, settings: rebind, store: &openAITurnStateRuntimeStoreStub{ticket: ticket, found: true}},
		{name: "none preserves scheduling", ctx: marked, account: account, request: request, settings: `{"enabled":true,"miss_action":"none","recovered_action":"none"}`, store: &openAITurnStateRuntimeStoreStub{}},
		{name: "unmarked messages request", ctx: context.Background(), account: account, request: request, settings: rebind, store: &openAITurnStateRuntimeStoreStub{}},
		{name: "non astra", ctx: marked, account: account, request: OpenAIAccountScheduleRequest{Platform: PlatformOpenAI, RequestedModel: "gpt-5.6-sol"}, settings: rebind, store: &openAITurnStateRuntimeStoreStub{}},
		{name: "api key", ctx: marked, account: &Account{ID: 82, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, request: request, settings: rebind, store: &openAITurnStateRuntimeStoreStub{}},
		{name: "out of scope", ctx: marked, account: &Account{ID: 83, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: account.Credentials}, request: request, settings: rebind, store: &openAITurnStateRuntimeStoreStub{}},
		{name: "redis failure fails open", ctx: marked, account: account, request: request, settings: rebind, store: &openAITurnStateRuntimeStoreStub{err: errors.New("redis down")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			compatible, reason := newScheduler(tc.settings, tc.store).isAccountRequestCompatibleReason(tc.ctx, tc.account, tc.request)
			require.True(t, compatible)
			require.Empty(t, reason)
		})
	}
}

func TestOpenAITurnStateSchedulerFiltersMissingTicketAcrossSelectionPaths(t *testing.T) {
	groupID := int64(91)
	missing := Account{
		ID: 91, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0,
		Credentials: map[string]any{"access_token": "missing-token", "chatgpt_account_id": "workspace-1"},
		GroupIDs:    []int64{groupID}, Groups: []*Group{{ID: groupID, Platform: PlatformOpenAI, TurnStateInjectEnabled: true}},
	}
	fallback := Account{ID: 92, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 5, GroupIDs: []int64{groupID}}
	settingsJSON := `{"enabled":true,"miss_action":"unschedulable","recovered_action":"none"}`

	newService := func(accounts []Account, missAction string) *OpenAIGatewayService {
		settings := settingsJSON
		if missAction == OpenAITurnStateMissNone {
			settings = `{"enabled":true,"miss_action":"none","recovered_action":"none"}`
		}
		svc := &OpenAIGatewayService{
			accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
			cache:              &schedulerTestGatewayCache{},
			cfg:                &config.Config{RunMode: config.RunModeSimple},
			concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
			settingService:     NewSettingService(&openAITurnStateSettingRepoStub{value: settings}, nil),
		}
		svc.SetOpenAITurnStateStore(&openAITurnStateRuntimeStoreStub{})
		return svc
	}
	selectAccount := func(ctx context.Context, svc *OpenAIGatewayService, sessionHash string) (*AccountSelectionResult, error) {
		selection, _, err := svc.SelectAccountWithSchedulerForCapability(
			WithOpenAITurnStateReuseScheduling(ctx, true), &groupID, "", sessionHash, OpenAITurnStateHarvestModel, nil,
			OpenAIUpstreamTransportAny, OpenAIEndpointCapabilityResponses, false, false, true,
		)
		return selection, err
	}

	t.Run("load balance skips missing ticket", func(t *testing.T) {
		selection, err := selectAccount(context.Background(), newService([]Account{missing, fallback}, OpenAITurnStateMissUnschedulable), "")
		require.NoError(t, err)
		require.Equal(t, fallback.ID, selection.Account.ID)
		selection.ReleaseFunc()
	})

	t.Run("no candidate returns existing error", func(t *testing.T) {
		selection, err := selectAccount(context.Background(), newService([]Account{missing}, OpenAITurnStateMissUnschedulable), "")
		require.Nil(t, selection)
		require.ErrorIs(t, err, ErrNoAvailableAccounts)
		require.ErrorContains(t, err, "turn_state_missing=1")
	})

	t.Run("none remains selectable", func(t *testing.T) {
		selection, err := selectAccount(context.Background(), newService([]Account{missing}, OpenAITurnStateMissNone), "")
		require.NoError(t, err)
		require.Equal(t, missing.ID, selection.Account.ID)
		selection.ReleaseFunc()
	})

	t.Run("sticky falls through", func(t *testing.T) {
		svc := newService([]Account{missing, fallback}, OpenAITurnStateMissUnschedulable)
		cache := svc.cache.(*schedulerTestGatewayCache)
		cache.sessionBindings = map[string]int64{"openai:sticky-turn-state": missing.ID}
		selection, err := selectAccount(context.Background(), svc, "sticky-turn-state")
		require.NoError(t, err)
		require.Equal(t, fallback.ID, selection.Account.ID)
		selection.ReleaseFunc()
	})

	t.Run("forced retry cannot bypass miss gate", func(t *testing.T) {
		ctx := WithOpenAIForcedAccount(context.Background(), missing.ID)
		selection, err := selectAccount(ctx, newService([]Account{missing, fallback}, OpenAITurnStateMissUnschedulable), "")
		require.NoError(t, err)
		require.Nil(t, selection)
	})
}

func TestOpenAITurnStateStateActionsAreScopedAndReasonSafe(t *testing.T) {
	now := time.Now()
	account := &Account{ID: 101}

	t.Run("miss actions", func(t *testing.T) {
		target := int64(44)
		for _, tc := range []struct {
			name     string
			settings *OpenAITurnStateReuseSettings
			groups   []int64
			reason   string
		}{
			{name: "rebind", settings: &OpenAITurnStateReuseSettings{MissAction: OpenAITurnStateMissRebindGroup, MissTargetGroupID: &target}, groups: []int64{target}},
			{name: "unbind", settings: &OpenAITurnStateReuseSettings{MissAction: OpenAITurnStateMissUnbindGroups}},
			{name: "unschedulable", settings: &OpenAITurnStateReuseSettings{MissAction: OpenAITurnStateMissUnschedulable}, reason: openAITurnStateMissReason},
		} {
			t.Run(tc.name, func(t *testing.T) {
				repo := &openAITurnStateActionRepoStub{}
				svc := &OpenAIGatewayService{accountRepo: repo}
				require.NoError(t, svc.applyOpenAITurnStateMissAction(context.Background(), account, tc.settings, now))
				require.Equal(t, tc.groups, repo.boundGroups)
				require.Equal(t, tc.reason, repo.setReason)
			})
		}
	})

	t.Run("restore clears only turn state reason", func(t *testing.T) {
		settings := &OpenAITurnStateReuseSettings{RecoveredAction: OpenAITurnStateRecoveredRestore}
		for _, reason := range []string{openAITurnStateMissReason, "manual_pause"} {
			repo := &openAITurnStateActionRepoStub{account: &Account{ID: account.ID, TempUnschedulableReason: reason}}
			svc := &OpenAIGatewayService{accountRepo: repo}
			require.NoError(t, svc.applyOpenAITurnStateRecoveredAction(context.Background(), account, settings))
			if reason == openAITurnStateMissReason {
				require.Equal(t, 1, repo.clearCalls)
			} else {
				require.Zero(t, repo.clearCalls)
			}
		}
	})
}

func TestOpenAITurnStateHarvesterRequiresTwoQualifiedCalls(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	firstRaw := makeOpenAITurnStateTicket(217, now.Add(-2*time.Minute))
	secondRaw := makeOpenAITurnStateTicket(249, now.Add(-time.Minute))
	qualified := func(raw string) *http.Response {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{OpenAITurnStateHeader: []string{raw}},
			Body:       io.NopCloser(strings.NewReader("data: {\"type\":\"response.completed\",\"response\":{\"model\":\"gpt-6-astra\"}}\n\n")),
		}
	}
	upstream := &openAITurnStateHarvestUpstreamStub{responses: []*http.Response{qualified(firstRaw), qualified(secondRaw)}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{ID: 111, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 1, Credentials: map[string]any{"access_token": "token", "chatgpt_account_id": "workspace"}}

	result, err := svc.harvestOpenAITurnStateTicket(context.Background(), account, "", now)
	require.NoError(t, err)
	require.Equal(t, secondRaw, result.Ticket.Raw)
	require.Len(t, upstream.requests, 2)
	require.Empty(t, upstream.requests[0].Header.Get(OpenAITurnStateHeader))
	require.Equal(t, firstRaw, upstream.requests[1].Header.Get(OpenAITurnStateHeader))
	require.Equal(t, "Bearer token", upstream.requests[1].Header.Get("Authorization"))
	require.Equal(t, "workspace", upstream.requests[1].Header.Get("chatgpt-account-id"))
}

func TestOpenAITurnStateHarvesterRejectsIncompleteOrWrongModelSSE(t *testing.T) {
	for _, body := range []string{
		"data: {\"type\":\"response.output_text.delta\"}\n\n",
		"data: {\"type\":\"response.completed\",\"response\":{\"model\":\"gpt-5.6-sol\"}}\n\n",
	} {
		completed, model, err := readOpenAITurnStateHarvestSSE(strings.NewReader(body))
		require.NoError(t, err)
		require.False(t, completed && model == OpenAITurnStateHarvestModel)
	}
}

type openAITurnStateWorkerStoreStub struct {
	mu           sync.Mutex
	ticket       OpenAITurnStateTicket
	found        bool
	leaseGranted bool
	putCalls     int
	deleteRaw    string
	leaseCalls   int
	releaseCalls int
}

func (s *openAITurnStateWorkerStoreStub) Get(context.Context, OpenAITurnStateKey, time.Time) (OpenAITurnStateTicket, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ticket, s.found, nil
}
func (s *openAITurnStateWorkerStoreStub) Put(_ context.Context, _ OpenAITurnStateKey, ticket OpenAITurnStateTicket) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ticket, s.found = ticket, true
	s.putCalls++
	return true, nil
}
func (s *openAITurnStateWorkerStoreStub) DeleteIfMatch(_ context.Context, _ OpenAITurnStateKey, raw string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleteRaw = raw
	if s.found && s.ticket.Raw == raw {
		s.found = false
		return true, nil
	}
	return false, nil
}
func (s *openAITurnStateWorkerStoreStub) AcquireLease(context.Context, OpenAITurnStateKey, string, time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.leaseCalls++
	return s.leaseGranted, nil
}
func (s *openAITurnStateWorkerStoreStub) ReleaseLease(context.Context, OpenAITurnStateKey, string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.releaseCalls++
	return nil
}

type openAITurnStateWorkerRepoStub struct {
	openAITurnStateActionRepoStub
	accounts []Account
}

func (r *openAITurnStateWorkerRepoStub) ListByPlatform(context.Context, string) ([]Account, error) {
	return append([]Account(nil), r.accounts...), nil
}

type openAITurnStateProxyRepoStub struct {
	ProxyRepository
	proxies []Proxy
}

func (r *openAITurnStateProxyRepoStub) ListActive(context.Context) ([]Proxy, error) {
	return append([]Proxy(nil), r.proxies...), nil
}

func TestOpenAITurnStateHarvesterClassifies429AndAuth(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	account := &Account{ID: 201, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 1, Credentials: map[string]any{"access_token": "token", "chatgpt_account_id": "workspace"}}
	for _, tc := range []struct {
		name       string
		status     int
		retryAfter string
		wantRetry  time.Duration
		wantAuth   bool
	}{
		{name: "429", status: http.StatusTooManyRequests, retryAfter: "420", wantRetry: 420 * time.Second},
		{name: "401", status: http.StatusUnauthorized, wantAuth: true},
		{name: "403", status: http.StatusForbidden, wantAuth: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := &openAITurnStateHarvestUpstreamStub{responses: []*http.Response{{StatusCode: tc.status, Header: http.Header{"Retry-After": []string{tc.retryAfter}}, Body: io.NopCloser(strings.NewReader(""))}}}
			result, err := (&OpenAIGatewayService{httpUpstream: upstream}).harvestOpenAITurnStateTicket(context.Background(), account, "", now)
			require.NoError(t, err)
			require.Equal(t, tc.status, result.StatusCode)
			require.Equal(t, tc.wantRetry, result.RetryAfter)
			require.Equal(t, tc.wantAuth, result.AuthFailed)
			require.Len(t, upstream.requests, 1)
		})
	}
}

func TestOpenAITurnStateHarvesterKeepsStoredTicketOnAuthFailure(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	existing, err := ParseOpenAITurnStateReuseTicket(makeOpenAITurnStateTicket(217, now.Add(-time.Minute)), now)
	require.NoError(t, err)
	account := &Account{ID: 201, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 1, Credentials: map[string]any{"access_token": "token", "chatgpt_account_id": "workspace"}}
	key := OpenAITurnStateKey{AccountID: account.ID, Model: OpenAITurnStateHarvestModel, CredentialHash: OpenAITurnStateCredentialHash("token", "workspace")}
	stateKey := fmt.Sprintf("%d:%s", key.AccountID, key.CredentialHash)
	store := &openAITurnStateWorkerStoreStub{ticket: existing, found: true}
	svc := &OpenAIGatewayService{
		httpUpstream:         &openAITurnStateHarvestUpstreamStub{responses: []*http.Response{{StatusCode: http.StatusUnauthorized, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(""))}}},
		openAITurnStateStore: store,
		openAITurnStateWorkerState: map[string]*openAITurnStateWorkerAccountState{
			stateKey: {},
		},
	}

	svc.harvestOpenAITurnStateAccount(context.Background(), account, key, true, "", stateKey, &OpenAITurnStateReuseSettings{}, now)

	store.mu.Lock()
	defer store.mu.Unlock()
	require.True(t, store.found)
	require.Equal(t, existing.Raw, store.ticket.Raw)
	require.Empty(t, store.deleteRaw)
}

func TestOpenAITurnStateHarvesterRoutesUseExplicitThenActivePool(t *testing.T) {
	now := time.Now()
	expired := now.Add(-time.Minute)
	svc := &OpenAIGatewayService{openAITurnStateProxyRepo: &openAITurnStateProxyRepoStub{proxies: []Proxy{
		{Protocol: "http", Host: "active", Port: 8080, Status: StatusActive},
		{Protocol: "socks5", Host: "expired", Port: 1080, Status: StatusActive, ExpiresAt: &expired},
	}}}
	require.Equal(t, []string{"http://explicit:9000"}, svc.openAITurnStateHarvestRoutes(context.Background(), &OpenAITurnStateReuseSettings{HarvestProxyURLs: []string{" http://explicit:9000 "}, HarvestUseProxyPool: true}, now))
	require.Equal(t, []string{"http://active:8080"}, svc.openAITurnStateHarvestRoutes(context.Background(), &OpenAITurnStateReuseSettings{HarvestUseProxyPool: true}, now))
}

func TestOpenAITurnStateHarvesterSweepUsesLeaseAndStopsCleanly(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	firstRaw := makeOpenAITurnStateTicket(217, now.Add(-2*time.Minute))
	secondRaw := makeOpenAITurnStateTicket(249, now.Add(-time.Minute))
	qualified := func(raw string) *http.Response {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{OpenAITurnStateHeader: []string{raw}}, Body: io.NopCloser(strings.NewReader("data: {\"type\":\"response.completed\",\"response\":{\"model\":\"gpt-6-astra\"}}\n\n"))}
	}
	account := Account{ID: 202, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 1, Credentials: map[string]any{"access_token": "token", "chatgpt_account_id": "workspace"}, Groups: []*Group{{ID: 9, Platform: PlatformOpenAI, TurnStateInjectEnabled: true}}}
	repo := &openAITurnStateWorkerRepoStub{accounts: []Account{account}}
	store := &openAITurnStateWorkerStoreStub{leaseGranted: true}
	svc := &OpenAIGatewayService{
		accountRepo:                repo,
		httpUpstream:               &openAITurnStateHarvestUpstreamStub{responses: []*http.Response{qualified(firstRaw), qualified(secondRaw)}},
		settingService:             NewSettingService(&openAITurnStateSettingRepoStub{value: `{"enabled":true,"harvest_proxy_urls":["http://route:8080"],"harvest_use_proxy_pool":true,"miss_action":"none","recovered_action":"none"}`}, nil),
		openAITurnStateStore:       store,
		openAITurnStateWorkerState: make(map[string]*openAITurnStateWorkerAccountState),
		openAITurnStateWorkerOwner: "test-owner",
	}
	sem := make(chan struct{}, openAITurnStateMaxConcurrency)
	var jobs sync.WaitGroup
	svc.sweepOpenAITurnStateHarvester(context.Background(), sem, &jobs, now)
	jobs.Wait()
	require.Equal(t, 1, store.leaseCalls)
	require.Equal(t, 1, store.releaseCalls)
	require.Equal(t, 1, store.putCalls)

	svc.StartOpenAITurnStateHarvester()
	done := make(chan struct{})
	go func() { svc.StopOpenAITurnStateHarvester(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("harvester did not stop")
	}
}
