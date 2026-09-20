package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tidwall/gjson"
)

type openAITurnStateHarvestResult struct {
	Ticket     OpenAITurnStateTicket
	StatusCode int
	RetryAfter time.Duration
	AuthFailed bool
}

const (
	openAITurnStateMissingCadence = 20 * time.Second
	openAITurnStateRenewCadence   = 5 * time.Minute
	openAITurnStateLeaseTTL       = 90 * time.Second
	openAITurnStateWorkerTick     = time.Second
	openAITurnStateMaxConcurrency = 3
)

type openAITurnStateWorkerAccountState struct {
	CredentialHash string
	LastAttempt    time.Time
	BackoffUntil   time.Time
	AuthPaused     bool
	MissApplied    bool
	Recovered      bool
	Status         string
	LastHTTPStatus int
	LastError      string
	LastRoute      string
}

type OpenAITurnStateAccountStatus struct {
	AccountID              int64      `json:"account_id"`
	AccountName            string     `json:"account_name"`
	Status                 string     `json:"status"`
	EncodedLength          int        `json:"encoded_length,omitempty"`
	DecodedLength          int        `json:"decoded_length,omitempty"`
	IssuedAt               *time.Time `json:"issued_at,omitempty"`
	ExpiresAt              *time.Time `json:"expires_at,omitempty"`
	RemainingSeconds       int64      `json:"remaining_seconds,omitempty"`
	LastHTTPStatus         int        `json:"last_http_status,omitempty"`
	LastError              string     `json:"last_error,omitempty"`
	LastRoute              string     `json:"last_route,omitempty"`
	TurnStateMissSuspended bool       `json:"turn_state_miss_suspended"`
}

func (s *OpenAIGatewayService) OpenAITurnStateStatus(ctx context.Context) ([]OpenAITurnStateAccountStatus, error) {
	if s == nil || s.accountRepo == nil {
		return []OpenAITurnStateAccountStatus{}, nil
	}
	accounts, err := s.accountRepo.ListByPlatform(ctx, PlatformOpenAI)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	result := make([]OpenAITurnStateAccountStatus, 0, len(accounts))
	for i := range accounts {
		account := &accounts[i]
		if !openAITurnStateAccountInScope(account) {
			continue
		}
		accessToken := strings.TrimSpace(account.GetOpenAIAccessToken())
		key := OpenAITurnStateKey{AccountID: account.ID, Model: OpenAITurnStateHarvestModel, CredentialHash: OpenAITurnStateCredentialHash(accessToken, strings.TrimSpace(account.GetChatGPTAccountID()))}
		item := OpenAITurnStateAccountStatus{AccountID: account.ID, AccountName: account.Name, Status: "missing", TurnStateMissSuspended: account.TempUnschedulableReason == openAITurnStateMissReason}
		if s.openAITurnStateStore != nil && accessToken != "" {
			if ticket, found, getErr := s.openAITurnStateStore.Get(ctx, key, now); getErr == nil && found {
				item.Status = "fresh"
				if ticket.RenewalDue(now) {
					item.Status = "renew_due"
				}
				item.EncodedLength, item.DecodedLength = len(ticket.Raw), ticket.DecodedLength
				issued, expires := ticket.IssuedAt, ticket.ExpiresAt
				item.IssuedAt, item.ExpiresAt = &issued, &expires
				item.RemainingSeconds = int64(ticket.ExpiresAt.Sub(now).Seconds())
				if item.RemainingSeconds < 0 {
					item.RemainingSeconds = 0
				}
			}
		}
		stateKey := fmt.Sprintf("%d:%s", key.AccountID, key.CredentialHash)
		s.openAITurnStateMu.RLock()
		state := s.openAITurnStateWorkerState[stateKey]
		if state != nil {
			item.LastHTTPStatus, item.LastError, item.LastRoute = state.LastHTTPStatus, state.LastError, state.LastRoute
			if state.Status == "paused_auth" || state.Status == "paused_429" {
				item.Status = state.Status
			}
		}
		s.openAITurnStateMu.RUnlock()
		result = append(result, item)
	}
	return result, nil
}

func (s *OpenAIGatewayService) SetOpenAITurnStateHarvesterProxyRepository(repo ProxyRepository) {
	if s != nil {
		s.openAITurnStateProxyRepo = repo
	}
}

func (s *OpenAIGatewayService) StartOpenAITurnStateHarvester() {
	if s == nil || s.accountRepo == nil || s.openAITurnStateStore == nil || s.settingService == nil {
		return
	}
	s.openAITurnStateWorkerOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		s.openAITurnStateWorkerCancel = cancel
		s.openAITurnStateWorkerState = make(map[string]*openAITurnStateWorkerAccountState)
		s.openAITurnStateWorkerOwner = fmt.Sprintf("turn-state-%d", time.Now().UnixNano())
		s.openAITurnStateWorkerWG.Add(1)
		go s.runOpenAITurnStateHarvester(ctx)
	})
}

func (s *OpenAIGatewayService) StopOpenAITurnStateHarvester() {
	if s == nil {
		return
	}
	s.openAITurnStateWorkerStopOnce.Do(func() {
		if s.openAITurnStateWorkerCancel != nil {
			s.openAITurnStateWorkerCancel()
		}
		s.openAITurnStateWorkerWG.Wait()
	})
}

func (s *OpenAIGatewayService) runOpenAITurnStateHarvester(ctx context.Context) {
	defer s.openAITurnStateWorkerWG.Done()
	ticker := time.NewTicker(openAITurnStateWorkerTick)
	defer ticker.Stop()
	sem := make(chan struct{}, openAITurnStateMaxConcurrency)
	var jobs sync.WaitGroup
	defer jobs.Wait()
	for {
		s.sweepOpenAITurnStateHarvester(ctx, sem, &jobs, time.Now())
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *OpenAIGatewayService) sweepOpenAITurnStateHarvester(ctx context.Context, sem chan struct{}, jobs *sync.WaitGroup, now time.Time) {
	settings, ok := s.openAITurnStateSettings(ctx, now)
	if !ok || settings == nil {
		return
	}
	accounts, err := s.accountRepo.ListByPlatform(ctx, PlatformOpenAI)
	if err != nil {
		return
	}
	if !settings.Enabled {
		for i := range accounts {
			if accounts[i].TempUnschedulableReason == openAITurnStateMissReason {
				_ = s.accountRepo.ClearTempUnschedulable(ctx, accounts[i].ID)
			}
		}
		return
	}
	routes := s.openAITurnStateHarvestRoutes(ctx, settings, now)
	for i := range accounts {
		account := accounts[i]
		if !openAITurnStateAccountInScope(&account) {
			continue
		}
		accessToken := strings.TrimSpace(account.GetOpenAIAccessToken())
		chatGPTAccountID := strings.TrimSpace(account.GetChatGPTAccountID())
		if accessToken == "" {
			continue
		}
		key := OpenAITurnStateKey{AccountID: account.ID, Model: OpenAITurnStateHarvestModel, CredentialHash: OpenAITurnStateCredentialHash(accessToken, chatGPTAccountID)}
		stateKey := fmt.Sprintf("%d:%s", key.AccountID, key.CredentialHash)
		s.openAITurnStateMu.Lock()
		state := s.openAITurnStateWorkerState[stateKey]
		if state == nil {
			state = &openAITurnStateWorkerAccountState{CredentialHash: key.CredentialHash}
			s.openAITurnStateWorkerState[stateKey] = state
		}
		s.openAITurnStateMu.Unlock()
		ticket, found, getErr := s.openAITurnStateStore.Get(ctx, key, now)
		if getErr != nil {
			found = false
		}
		if found && !ticket.RenewalDue(now) {
			s.updateOpenAITurnStateWorkerState(stateKey, func(st *openAITurnStateWorkerAccountState) { st.Status = "fresh" })
			continue
		}
		cadence := openAITurnStateMissingCadence
		if found {
			cadence = openAITurnStateRenewCadence
		}
		if state.AuthPaused || now.Before(state.BackoffUntil) || (!state.LastAttempt.IsZero() && now.Sub(state.LastAttempt) < cadence) {
			continue
		}
		if len(routes) == 0 {
			s.updateOpenAITurnStateWorkerState(stateKey, func(st *openAITurnStateWorkerAccountState) {
				st.LastAttempt, st.LastError, st.Status = now, "no_harvest_route", "missing"
			})
			if !found && !state.MissApplied {
				_ = s.applyOpenAITurnStateMissAction(ctx, &account, settings, now)
				s.updateOpenAITurnStateWorkerState(stateKey, func(st *openAITurnStateWorkerAccountState) { st.MissApplied = true })
			}
			continue
		}
		owner := s.openAITurnStateWorkerOwner
		leased, leaseErr := s.openAITurnStateStore.AcquireLease(ctx, key, owner, openAITurnStateLeaseTTL)
		if leaseErr != nil || !leased {
			continue
		}
		route := routes[int(s.openAITurnStateRouteIndex.Add(1)-1)%len(routes)]
		s.updateOpenAITurnStateWorkerState(stateKey, func(st *openAITurnStateWorkerAccountState) {
			st.LastAttempt, st.LastRoute = now, redactOpenAITurnStateRoute(route)
		})
		select {
		case sem <- struct{}{}:
			jobs.Add(1)
			go func(account Account, key OpenAITurnStateKey, hadTicket bool, route, stateKey string) {
				defer jobs.Done()
				defer func() { <-sem }()
				defer s.openAITurnStateStore.ReleaseLease(context.Background(), key, owner)
				s.harvestOpenAITurnStateAccount(ctx, &account, key, hadTicket, route, stateKey, settings, now)
			}(account, key, found, route, stateKey)
		case <-ctx.Done():
			_ = s.openAITurnStateStore.ReleaseLease(context.Background(), key, owner)
			return
		default:
			_ = s.openAITurnStateStore.ReleaseLease(context.Background(), key, owner)
		}
	}
}

func (s *OpenAIGatewayService) harvestOpenAITurnStateAccount(ctx context.Context, account *Account, key OpenAITurnStateKey, hadTicket bool, route, stateKey string, settings *OpenAITurnStateReuseSettings, now time.Time) {
	result, err := s.harvestOpenAITurnStateTicket(ctx, account, route, now)
	s.updateOpenAITurnStateWorkerState(stateKey, func(st *openAITurnStateWorkerAccountState) {
		st.LastHTTPStatus = result.StatusCode
		st.LastError = ""
		if err != nil {
			st.LastError = "harvest_failed"
		}
	})
	if result.StatusCode == http.StatusTooManyRequests {
		backoff := result.RetryAfter
		if backoff < openAITurnStateRenewCadence {
			backoff = openAITurnStateRenewCadence
		}
		s.updateOpenAITurnStateWorkerState(stateKey, func(st *openAITurnStateWorkerAccountState) {
			st.BackoffUntil, st.Status = now.Add(backoff), "paused_429"
		})
		return
	}
	if result.AuthFailed {
		s.updateOpenAITurnStateWorkerState(stateKey, func(st *openAITurnStateWorkerAccountState) { st.AuthPaused, st.Status = true, "paused_auth" })
		if !hadTicket && !s.openAITurnStateMissAlreadyApplied(stateKey) {
			_ = s.applyOpenAITurnStateMissAction(ctx, account, settings, now)
			s.updateOpenAITurnStateWorkerState(stateKey, func(st *openAITurnStateWorkerAccountState) { st.MissApplied = true })
		}
		return
	}
	if err != nil || result.StatusCode != http.StatusOK || !result.Ticket.Valid(now) {
		if !hadTicket && !s.openAITurnStateMissAlreadyApplied(stateKey) {
			_ = s.applyOpenAITurnStateMissAction(ctx, account, settings, now)
			s.updateOpenAITurnStateWorkerState(stateKey, func(st *openAITurnStateWorkerAccountState) { st.MissApplied, st.Status = true, "missing" })
		}
		return
	}
	published, putErr := s.openAITurnStateStore.Put(ctx, key, result.Ticket)
	if putErr != nil || !published {
		return
	}
	if s.openAITurnStateMissAlreadyApplied(stateKey) {
		_ = s.applyOpenAITurnStateRecoveredAction(ctx, account, settings)
	}
	s.updateOpenAITurnStateWorkerState(stateKey, func(st *openAITurnStateWorkerAccountState) {
		st.MissApplied, st.AuthPaused, st.Recovered, st.Status = false, false, true, "fresh"
	})
}

func (s *OpenAIGatewayService) openAITurnStateMissAlreadyApplied(key string) bool {
	s.openAITurnStateMu.RLock()
	defer s.openAITurnStateMu.RUnlock()
	return s.openAITurnStateWorkerState[key] != nil && s.openAITurnStateWorkerState[key].MissApplied
}

func (s *OpenAIGatewayService) updateOpenAITurnStateWorkerState(key string, fn func(*openAITurnStateWorkerAccountState)) {
	s.openAITurnStateMu.Lock()
	defer s.openAITurnStateMu.Unlock()
	if st := s.openAITurnStateWorkerState[key]; st != nil {
		fn(st)
	}
}

func (s *OpenAIGatewayService) openAITurnStateHarvestRoutes(ctx context.Context, settings *OpenAITurnStateReuseSettings, now time.Time) []string {
	routes := make([]string, 0, len(settings.HarvestProxyURLs))
	for _, route := range settings.HarvestProxyURLs {
		if strings.TrimSpace(route) != "" {
			routes = append(routes, strings.TrimSpace(route))
		}
	}
	if len(routes) > 0 || !settings.HarvestUseProxyPool || s.openAITurnStateProxyRepo == nil {
		return routes
	}
	proxies, err := s.openAITurnStateProxyRepo.ListActive(ctx)
	if err != nil {
		return routes
	}
	for i := range proxies {
		if !proxies[i].IsExpired(now) {
			routes = append(routes, proxies[i].URL())
		}
	}
	return routes
}

func redactOpenAITurnStateRoute(route string) string {
	if route == "" {
		return "direct"
	}
	if i := strings.Index(route, "@"); i >= 0 {
		return "***" + route[i:]
	}
	return route
}

func (s *OpenAIGatewayService) harvestOpenAITurnStateTicket(ctx context.Context, account *Account, proxyURL string, now time.Time) (openAITurnStateHarvestResult, error) {
	if s == nil || s.httpUpstream == nil || account == nil || !account.IsOpenAIOAuth() {
		return openAITurnStateHarvestResult{}, fmt.Errorf("turn-state harvester unavailable")
	}
	token := strings.TrimSpace(account.GetOpenAIAccessToken())
	if s.openAITokenProvider != nil {
		resolved, err := s.openAITokenProvider.GetAccessToken(ctx, account)
		if err != nil {
			return openAITurnStateHarvestResult{}, err
		}
		token = strings.TrimSpace(resolved)
	}
	if token == "" {
		return openAITurnStateHarvestResult{}, fmt.Errorf("turn-state access token is empty")
	}

	first, result, err := s.harvestOpenAITurnStateCall(ctx, account, proxyURL, token, "", now)
	if err != nil || result.StatusCode != http.StatusOK {
		return result, err
	}
	second, result, err := s.harvestOpenAITurnStateCall(ctx, account, proxyURL, token, first.Raw, now)
	if err != nil || result.StatusCode != http.StatusOK {
		return result, err
	}
	result.Ticket = second
	return result, nil
}

func (s *OpenAIGatewayService) harvestOpenAITurnStateCall(ctx context.Context, account *Account, proxyURL, token, sentTicket string, now time.Time) (OpenAITurnStateTicket, openAITurnStateHarvestResult, error) {
	body := []byte(`{"model":"gpt-6-astra","input":"Reply with OK.","stream":true,"store":false}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, buildOpenAIResponsesURL(account.GetOpenAIBaseURL()), bytes.NewReader(body))
	if err != nil {
		return OpenAITurnStateTicket{}, openAITurnStateHarvestResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	setOpenAIChatGPTAccountHeaders(req.Header, account)
	if strings.TrimSpace(sentTicket) != "" {
		req.Header.Set(OpenAITurnStateHeader, sentTicket)
	}
	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		return OpenAITurnStateTicket{}, openAITurnStateHarvestResult{}, err
	}
	defer resp.Body.Close()
	result := openAITurnStateHarvestResult{StatusCode: resp.StatusCode}
	if resp.StatusCode == http.StatusTooManyRequests {
		if seconds, parseErr := strconv.Atoi(strings.TrimSpace(resp.Header.Get("Retry-After"))); parseErr == nil && seconds > 0 {
			result.RetryAfter = time.Duration(seconds) * time.Second
		}
		return OpenAITurnStateTicket{}, result, nil
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		result.AuthFailed = true
		return OpenAITurnStateTicket{}, result, nil
	}
	if resp.StatusCode != http.StatusOK {
		return OpenAITurnStateTicket{}, result, nil
	}
	completed, actualModel, err := readOpenAITurnStateHarvestSSE(resp.Body)
	if err != nil || !completed || actualModel != OpenAITurnStateHarvestModel {
		return OpenAITurnStateTicket{}, result, fmt.Errorf("turn-state harvest response not qualified")
	}
	ticket, err := ParseOpenAITurnStateReuseTicket(resp.Header.Get(OpenAITurnStateHeader), now)
	if err != nil {
		return OpenAITurnStateTicket{}, result, err
	}
	return ticket, result, nil
}

func readOpenAITurnStateHarvestSSE(reader io.Reader) (bool, string, error) {
	scanner := bufio.NewScanner(io.LimitReader(reader, 2<<20))
	scanner.Buffer(make([]byte, 64*1024), 512*1024)
	completed := false
	actualModel := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" || !json.Valid([]byte(payload)) {
			continue
		}
		if gjson.Get(payload, "type").String() == "response.completed" {
			completed = true
			actualModel = strings.TrimSpace(gjson.Get(payload, "response.model").String())
			if actualModel == "" {
				actualModel = strings.TrimSpace(gjson.Get(payload, "model").String())
			}
		}
	}
	return completed, actualModel, scanner.Err()
}
