package service

import (
	"context"
	"strconv"
	"strings"
	"time"
)

const (
	OpenAIEventStreamUpstreamFailure            = "openai.stream_upstream_failure"
	OpenAIEventAccountModelSoftFailure          = "openai.account_model_soft_failure"
	OpenAIEventAccountModelCooldownStarted      = "openai.account_model_cooldown_started"
	OpenAIEventAccountModelCooldownSkippedCache = "openai.account_model_cooldown_skipped_for_cache"
	OpenAIEventAccountModelPostFailureSelected  = "openai.account_model_post_failure_selected"
	OpenAIEventFailoverAfterStreamFailure       = "openai.failover_after_stream_failure"
	OpenAIEventAccountModelHalfOpenProbe        = "openai.account_model_half_open_probe"
	OpenAIEventRetryBillingReconciled           = "openai.retry_billing_reconciled"
	OpenAIEventSchedulerSelection               = "openai.scheduler_selection"
	OpenAIEventSchedulerRequestOutcome          = "openai.scheduler_request_outcome"
	OpenAIEventFirstOutputSlow                  = "openai.first_output_slow"
	OpenAIEventClientAbandonedAfterUpstreamWait = "openai.client_abandoned_after_upstream_wait"
	OpenAIEventResponsesFailoverDecision        = "openai.responses_failover_decision"
)

// OpenAIResilienceEvent describes legacy scheduler records. Emission is retired;
// historical log decoding and safety call sites retain the compatible shape.
type OpenAIResilienceEvent struct {
	At                    time.Time
	Platform              string
	GroupID               *int64
	CorrelationID         string
	Name                  string
	AccountID             int64
	CanonicalModel        string
	AttemptID             string
	AttemptNumber         int
	StatusCode            int
	FailureClass          string
	CapacitySubtype       string
	CurrentRequestAction  string
	FutureRequestAction   string
	SharedFeedbackWritten bool
	OutputStarted         bool
	UsageProduced         bool
	ResponseFailedOnly    bool
	UnsafeToReplay        bool
	SwitchAllowed         bool
	SwitchReason          string
	SwitchBlockReason     string
	FailureStreak         int
	CacheMode             string
	CooldownSeconds       int
	RetryAfterSeconds     int
	CooldownUntil         time.Time
	HealthState           string
	CandidateAccountIDs   []int64
	ExcludedAccountIDs    []int64
	ExcludeReasons        map[string]int
	Outcome               string // success, failure, selected, cache_hit, started
	SelectionLayer        string
	CandidateCount        int
	EligibleCount         int
	EffectiveTopK         int
	MinimumScoreThreshold float64
	StickyKept            bool
	StickyEscapeReason    string
	TTFTReportEligible    bool
	RetryBudgetExhausted  bool
	FinalOutcome          string
	// Unified-quality scheduling details. These fields are intentionally
	// non-sensitive and remain readable in historical records.
	SelectedAccountID       int64
	SelectedRank            int
	SelectedPriority        int
	SelectedPrioritySignal  float64
	ColdStartPrioritySignal float64
	DailyPrioritySignal     float64
	UnifiedQuality          bool
	ImageIntent             bool
	QualityWindowEnd        time.Time
	QualityScore            float64
	SuccessScore            float64
	FirstOutputScore        float64
	OutputRateScore         float64
	LiveLoadScore           float64
	FirstOutputSlowCount    int
	SlowEvidenceReplaced    bool
	QualityScoreGap         float64
	QualitySnapshotStale    bool
	ProfitMode              string
	ProfitBypass            bool
	ProfitBypassReason      string
	RuntimeRetryBudget      int
	ExtraRetryCount         int
	ExtraUsed               int
	SwitchCount             int
	SafeToReplay            bool
	StopReason              string
	NativeSlotWaitMs        int64
	RoutingMs               int64
	UpstreamTTFTMs          int64
	TotalMs                 int64
}

type openAIResilienceCacheModeContextKey struct{}
type openAIResilienceCorrelationIDContextKey struct{}

func WithOpenAIResilienceCacheMode(ctx context.Context, mode string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, openAIResilienceCacheModeContextKey{}, strings.TrimSpace(mode))
}

func openAIResilienceCacheModeFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	mode, _ := ctx.Value(openAIResilienceCacheModeContextKey{}).(string)
	return strings.TrimSpace(mode)
}

func WithOpenAIResilienceCorrelationID(ctx context.Context, correlationID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, openAIResilienceCorrelationIDContextKey{}, strings.TrimSpace(correlationID))
}

func openAIResilienceCorrelationIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	correlationID, _ := ctx.Value(openAIResilienceCorrelationIDContextKey{}).(string)
	return strings.TrimSpace(correlationID)
}

// RecordOpenAIResilienceOutcome is retained for compatibility with safety-event
// call sites. Full request event collection is retired; health state changes
// and native error/usage persistence are owned by their respective services.
func RecordOpenAIResilienceOutcome(event OpenAIResilienceEvent) {}

func RecordOpenAISchedulerSelection(ctx context.Context, platform string, groupID *int64, decision OpenAIAccountScheduleDecision) {
}

func RecordOpenAISchedulerRequestOutcome(ctx context.Context, platform string, groupID *int64, outcome string, retryBudgetExhausted bool) {
}

// RecordOpenAIResilienceOutcomeWithContext is a retired compatibility emitter.
// It deliberately does not assemble correlation metadata or copy event data.
func RecordOpenAIResilienceOutcomeWithContext(ctx context.Context, event OpenAIResilienceEvent) {}

// RecordOpenAIFailoverAfterStreamFailure records a completed post-output
// account switch. Callers invoke it only after the replacement attempt has
// succeeded, never when returning a terminal recovery envelope.
func RecordOpenAIFailoverAfterStreamFailure(ctx context.Context, platform string, groupID *int64, statusCode int, usageProduced bool, retryAfterSeconds int) {
}

// RecordOpenAIRetryBillingReconciled records a successful, durable billing
// completion for a retry whose earlier attempt exposed partial usage. It must
// be called after RecordUsage returns nil, not when reconciliation is queued.
func RecordOpenAIRetryBillingReconciled(ctx context.Context, platform string, groupID *int64, statusCode int, outputStarted, usageProduced bool, retryAfterSeconds int) {
}

func RecordOpenAIResilienceEvent(name string, failureStreak int, cacheMode string) {}

func openAIResilienceCorrelationKey(event OpenAIResilienceEvent) string {
	correlationID := strings.TrimSpace(event.CorrelationID)
	if correlationID == "" {
		return ""
	}
	groupID := "none"
	if event.GroupID != nil {
		groupID = strconv.FormatInt(*event.GroupID, 10)
	}
	return strings.TrimSpace(event.Platform) + "\x00" + groupID + "\x00" + correlationID
}
