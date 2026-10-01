package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/oauthobs"
	"github.com/google/uuid"
)

type oauthProbeSourceKey struct{}
type oauthProbeRuleKey struct{}
type oauthObservationAttemptKey struct{}
type oauthObservationSlotMetadataKey struct{}

type oauthObservationSlotMetadata struct {
	model    string
	protocol string
	role     string
}
type oauthProbeRule struct {
	id            int64
	roundID       string
	triggerSource string
}

// WithOAuthObservationAttempt creates an opaque, server-generated correlation
// ID. It is unrelated to client, billing, or upstream request identifiers.
func WithOAuthObservationAttempt(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, oauthObservationAttemptKey{}, uuid.NewString())
}

func OAuthObservationAttemptID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(oauthObservationAttemptKey{}).(string)
	return v
}

func WithOAuthObservationSlotMetadata(ctx context.Context, model, protocol, role string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, oauthObservationSlotMetadataKey{}, oauthObservationSlotMetadata{model: observationModel(model), protocol: protocol, role: role})
}

func oauthObservationSlotMetadataFromContext(ctx context.Context) oauthObservationSlotMetadata {
	if ctx == nil {
		return oauthObservationSlotMetadata{}
	}
	v, _ := ctx.Value(oauthObservationSlotMetadataKey{}).(oauthObservationSlotMetadata)
	return v
}

func withOAuthObservationRequestSlotMetadata(ctx context.Context, model string) context.Context {
	if metadata := oauthObservationSlotMetadataFromContext(ctx); metadata.model != "" || metadata.role != "" {
		return ctx
	}
	return WithOAuthObservationSlotMetadata(ctx, model, "", "request")
}

func (s *OpenAIGatewayService) oauthRecorder() *oauthobs.Recorder {
	if s == nil || s.concurrencyService == nil {
		return nil
	}
	return s.concurrencyService.observations
}

func (s *OpenAIGatewayService) StopOAuthObservations(ctx context.Context) error {
	return s.oauthRecorder().Stop(ctx)
}

func (s *OpenAIGatewayService) observeOAuthProbe(ctx context.Context, account *Account, p *OpenAICodexStateProbeResult) {
	if account == nil || !account.IsOpenAIOAuth() || p == nil {
		return
	}
	source := "manual"
	if v, _ := ctx.Value(oauthProbeSourceKey{}).(string); v == "scheduled" {
		source = v
	}
	rule, _ := ctx.Value(oauthProbeRuleKey{}).(oauthProbeRule)
	s.oauthRecorder().Critical(ctx, oauthobs.Event{AccountID: account.ID,
		Key: fmt.Sprintf("probe:%d:%d", account.ID, p.StartedAt.UnixNano()), OccurredAt: p.FinishedAt, Type: "probe_result",
		Payload: oauthobs.Payload{Model: observationModel(p.Model), Protocol: "native", ProbeVersion: "turn_state_v1", Verdict: string(p.Verdict), Failure: p.Failure, RuleID: rule.id, RoundID: rule.roundID, TriggerSource: rule.triggerSource,
			LatencyMS: p.LatencyMs, StartedAt: &p.StartedAt, FinishedAt: &p.FinishedAt, Source: source, MintStatus: p.MintStatus, ContinueStatus: p.ContinueStatus}})
}

func observationModel(model string) string {
	model = strings.TrimSpace(model)
	if len(model) > 200 {
		return "oversized_model"
	}
	return model
}

func observationProtocol(account *Account, model string) string {
	if account.IsExcelBPSEnabledForModel(model) {
		return "bps"
	}
	return "native"
}

func (s *OpenAIGatewayService) observeOAuthSelection(ctx context.Context, selection *AccountSelectionResult, decision OpenAIAccountScheduleDecision, groupID *int64, model string, excluded map[int64]struct{}) {
	r := s.oauthRecorder()
	if r == nil || selection == nil || selection.Account == nil {
		return
	}
	a := selection.Account
	kind := "selected"
	if !selection.Acquired {
		kind = "wait_selected"
	}
	metadata := oauthObservationSlotMetadataFromContext(ctx)
	if metadata.model != "" {
		model = metadata.model
	}
	protocol := observationProtocol(a, model)
	if metadata.protocol != "" {
		protocol = metadata.protocol
	}
	p := oauthobs.Payload{AttemptID: OAuthObservationAttemptID(ctx), Model: observationModel(model), Protocol: protocol, Layer: decision.Layer, GroupID: groupID, ExcludedCount: len(excluded)}
	r.Emit(oauthobs.Event{AccountID: a.ID, Type: kind, Payload: p})
	for id := range excluded {
		r.Emit(oauthobs.Event{AccountID: id, Type: "fallback", Payload: oauthobs.Payload{AttemptID: OAuthObservationAttemptID(ctx), Model: observationModel(model), GroupID: groupID, ErrorClass: "retry_excluded", SelectedAccountID: a.ID}})
	}
}

func (s *OpenAIGatewayService) observeOAuthOutcome(ctx context.Context, account *Account, model string, success bool, firstTokenMS *int, observedErr []error) {
	if account == nil || !account.IsOpenAIOAuth() {
		return
	}
	var err error
	if len(observedErr) > 0 {
		err = observedErr[0]
	}
	status, class := oauthObservationError(err)
	if success {
		class = "success"
	}
	s.oauthRecorder().Emit(oauthobs.Event{AccountID: account.ID, Type: "request_outcome", Payload: oauthobs.Payload{AttemptID: OAuthObservationAttemptID(ctx), Model: observationModel(model), Protocol: observationProtocol(account, model), Success: &success, FirstTokenMS: firstTokenMS, HTTPStatus: status, ErrorClass: class}})
}

func (s *OpenAIGatewayService) observeOAuthFilter(account *Account, req OpenAIAccountScheduleRequest, reason string) {
	if account == nil || !account.IsOpenAIOAuth() {
		return
	}
	s.oauthRecorder().Emit(oauthobs.Event{AccountID: account.ID, Type: "candidate_filtered", Payload: oauthobs.Payload{Model: observationModel(req.RequestedModel), Protocol: observationProtocol(account, req.RequestedModel), GroupID: req.GroupID, ErrorClass: reason, Limit: account.Concurrency}})
}

func oauthObservationError(err error) (int, string) {
	if errors.Is(err, context.Canceled) {
		return 0, "cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return 0, "timeout"
	}
	var failure *UpstreamFailoverError
	if errors.As(err, &failure) {
		switch failure.StatusCode {
		case 401:
			return 401, "credential_invalid"
		case 403:
			return 403, "forbidden"
		case 429:
			return 429, "rate_limited"
		}
		if failure.StatusCode >= 500 {
			return failure.StatusCode, "upstream_error"
		}
		return failure.StatusCode, "request_error"
	}
	var network net.Error
	if errors.As(err, &network) {
		if network.Timeout() {
			return 0, "timeout"
		}
		return 0, "network_error"
	}
	return 0, "unknown"
}

func (s *ConcurrencyService) observeSlot(ctx context.Context, accountID int64, eventType, slotID string, limit int) {
	if s.observations == nil {
		return
	}
	now := time.Now().UTC()
	metadata := oauthObservationSlotMetadataFromContext(ctx)
	p := oauthobs.Payload{AttemptID: OAuthObservationAttemptID(ctx), Model: metadata.model, Protocol: metadata.protocol, SlotID: slotID, SlotRole: metadata.role, Limit: limit}
	if eventType == "slot_acquired" {
		ttl := s.observationSlotTTL
		if ttl <= 0 {
			ttl = 30 * time.Minute
		}
		expires := now.Add(ttl)
		p.ExpiresAt = &expires
	}
	s.observations.Emit(oauthobs.Event{AccountID: accountID, Type: eventType, OccurredAt: now, Payload: p})
}

func (s *OpenAIGatewayService) observeLiveSlot(accountID int64, eventType, slotID string, limit int, model, attemptID string) {
	now := time.Now().UTC()
	p := oauthobs.Payload{AttemptID: attemptID, Model: observationModel(model), Protocol: "live", SlotID: slotID, SlotRole: "live_lease", Limit: limit}
	// Matches the distributed Live lease's fixed 60s TTL. Refreshes extend the
	// observed interval; a failed release remains incomplete until lease expiry.
	if eventType == "slot_acquired" || eventType == "slot_refreshed" {
		expires := now.Add(time.Minute)
		p.ExpiresAt = &expires
	}
	s.oauthRecorder().Emit(oauthobs.Event{AccountID: accountID, OccurredAt: now, Type: eventType, Payload: p})
}

func (s *OpenAIGatewayService) observeLiveOutcome(account *Account, model, attemptID string, success bool, observedErr error) {
	if account == nil || !account.IsOpenAIOAuth() {
		return
	}
	status, class := oauthObservationError(observedErr)
	if success {
		class = "success"
	}
	s.oauthRecorder().Emit(oauthobs.Event{AccountID: account.ID, Type: "live_call_outcome", Payload: oauthobs.Payload{AttemptID: attemptID, Model: observationModel(model), Protocol: "live", Success: &success, HTTPStatus: status, ErrorClass: class}})
}
