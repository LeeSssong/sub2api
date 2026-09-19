package service

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const openAITurnStateSettingsCacheTTL = 3 * time.Second

type openAITurnStateSettingsSnapshot struct {
	settings *OpenAITurnStateReuseSettings
	loadedAt time.Time
}

func (s *OpenAIGatewayService) SetOpenAITurnStateStore(store OpenAITurnStateStore) {
	if s == nil {
		return
	}
	s.openAITurnStateMu.Lock()
	s.openAITurnStateStore = store
	s.openAITurnStateCached = nil
	s.openAITurnStateMu.Unlock()
}

func (s *OpenAIGatewayService) openAITurnStateSettings(ctx context.Context, now time.Time) (*OpenAITurnStateReuseSettings, bool) {
	if s == nil || s.settingService == nil {
		return nil, false
	}
	s.openAITurnStateMu.RLock()
	cached := s.openAITurnStateCached
	s.openAITurnStateMu.RUnlock()
	if cached != nil && now.Sub(cached.loadedAt) < openAITurnStateSettingsCacheTTL {
		return cached.settings, true
	}
	settings, err := s.settingService.GetOpenAITurnStateReuseSettings(ctx)
	if err != nil {
		return nil, false
	}
	s.openAITurnStateMu.Lock()
	s.openAITurnStateCached = &openAITurnStateSettingsSnapshot{settings: settings, loadedAt: now}
	s.openAITurnStateMu.Unlock()
	return settings, true
}

func openAITurnStateAccountInScope(account *Account) bool {
	if account == nil || !account.IsOpenAIOAuth() {
		return false
	}
	for _, group := range account.Groups {
		if group != nil && strings.EqualFold(strings.TrimSpace(group.Platform), PlatformOpenAI) && group.TurnStateInjectEnabled {
			return true
		}
	}
	for _, membership := range account.AccountGroups {
		group := membership.Group
		if group != nil && strings.EqualFold(strings.TrimSpace(group.Platform), PlatformOpenAI) && group.TurnStateInjectEnabled {
			return true
		}
	}
	return false
}

func (s *OpenAIGatewayService) openAITurnStateAccountSchedulableForRequest(ctx context.Context, account *Account, model string) (bool, string) {
	if !openAITurnStateSchedulingEnabled(ctx) || strings.TrimSpace(model) != OpenAITurnStateHarvestModel || !openAITurnStateAccountInScope(account) {
		return true, ""
	}
	now := time.Now()
	settings, ok := s.openAITurnStateSettings(ctx, now)
	if !ok || settings == nil || !settings.Enabled || settings.MissAction == OpenAITurnStateMissNone || s.openAITurnStateStore == nil {
		return true, ""
	}
	key := OpenAITurnStateKey{
		AccountID:      account.ID,
		Model:          OpenAITurnStateHarvestModel,
		CredentialHash: OpenAITurnStateCredentialHash(strings.TrimSpace(account.GetOpenAIAccessToken()), strings.TrimSpace(account.GetChatGPTAccountID())),
	}
	_, found, err := s.openAITurnStateStore.Get(ctx, key, now)
	if err != nil {
		return true, ""
	}
	if !found {
		return false, "turn_state_missing"
	}
	return true, ""
}

func (s *OpenAIGatewayService) resolveOpenAITurnStateReuse(ctx context.Context, account *Account, body []byte, endpoint string) (OpenAITurnStateTicket, bool) {
	if s == nil || s.openAITurnStateStore == nil || !openAITurnStateAccountInScope(account) {
		return OpenAITurnStateTicket{}, false
	}
	now := time.Now()
	settings, ok := s.openAITurnStateSettings(ctx, now)
	if !ok || settings == nil || !settings.Enabled {
		return OpenAITurnStateTicket{}, false
	}
	model := strings.TrimSpace(gjson.GetBytes(body, "model").String())
	key := OpenAITurnStateKey{
		AccountID:      account.ID,
		Model:          model,
		CredentialHash: OpenAITurnStateCredentialHash(strings.TrimSpace(account.GetOpenAIAccessToken()), strings.TrimSpace(account.GetChatGPTAccountID())),
	}
	ticket, found, err := s.openAITurnStateStore.Get(ctx, key, now)
	if err != nil || !found {
		return OpenAITurnStateTicket{}, false
	}
	probe := make(http.Header)
	if !ApplyOpenAITurnStateReuseOutbound(probe, OpenAITurnStateDecisionInput{
		Enabled:              true,
		InScope:              true,
		Endpoint:             endpoint,
		Model:                model,
		HasCompactionTrigger: HasCompactionTriggerInInput(body),
		Ticket:               &ticket,
		Now:                  now,
	}) {
		return OpenAITurnStateTicket{}, false
	}
	return ticket, true
}

func (s *OpenAIGatewayService) applyOpenAITurnStateReuse(ctx context.Context, headers http.Header, account *Account, body []byte, endpoint string) bool {
	if headers == nil {
		return false
	}
	ticket, ok := s.resolveOpenAITurnStateReuse(ctx, account, body, endpoint)
	if !ok {
		return false
	}
	headers.Set(OpenAITurnStateHeader, ticket.Raw)
	return true
}

func (s *OpenAIGatewayService) applyOpenAITurnStateReuseWebsocket(ctx context.Context, account *Account, body []byte) []byte {
	ticket, ok := s.resolveOpenAITurnStateReuse(ctx, account, body, "/responses")
	if !ok {
		return body
	}
	updated, err := sjson.SetBytes(body, "client_metadata.x-codex-turn-state", ticket.Raw)
	if err != nil {
		return body
	}
	return updated
}

func (s *OpenAIGatewayService) applyOpenAITurnStateReuseWebsocketPayload(ctx context.Context, account *Account, payload map[string]any) {
	if len(payload) == 0 {
		return
	}
	body := payloadAsJSONBytes(payload)
	ticket, ok := s.resolveOpenAITurnStateReuse(ctx, account, body, "/responses")
	if !ok {
		return
	}
	metadata, _ := payload["client_metadata"].(map[string]any)
	if metadata == nil {
		metadata = make(map[string]any)
	}
	metadata[openAIWSTurnStateHeader] = ticket.Raw
	payload["client_metadata"] = metadata
}
