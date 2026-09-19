package service

import (
	"context"
	"errors"
	"net/http"
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
