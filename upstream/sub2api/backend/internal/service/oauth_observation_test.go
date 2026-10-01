//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/oauthobs"
	"github.com/stretchr/testify/require"
)

type observationStoreTest struct {
	mu     sync.Mutex
	events []oauthobs.Event
}

func (s *observationStoreTest) WriteOAuthObservations(_ context.Context, events []oauthobs.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, events...)
	return nil
}
func (s *observationStoreTest) WriteOAuthObservationHealth(context.Context, oauthobs.Health) error {
	return nil
}
func (s *observationStoreTest) PruneOAuthObservations(context.Context) error { return nil }
func finishObservationTest(t *testing.T, r *oauthobs.Recorder) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, r.Stop(ctx))
}

func TestOAuthObservationSlotsPreserveAcquireAndTrackFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		sink := &observationStoreTest{}
		cache := &stubConcurrencyCacheForTest{acquireResult: true}
		if fail {
			cache.releaseErr = errors.New("SECRET_RAW_UPSTREAM")
		}
		s := NewConcurrencyService(cache)
		s.observations = oauthobs.New(sink, "test", 10)
		s.observationSlotTTL = time.Minute
		s.observations.Start()
		result, err := s.AcquireAccountSlot(context.Background(), 42, 15)
		require.NoError(t, err)
		require.True(t, result.Acquired)
		result.ReleaseFunc()
		cache.acquireResult = false
		denied, err := s.AcquireAccountSlot(context.Background(), 42, 15)
		require.NoError(t, err)
		require.False(t, denied.Acquired)
		finishObservationTest(t, s.observations)
		require.Len(t, sink.events, 3)
		require.Equal(t, "slot_acquired", sink.events[0].Type)
		require.Equal(t, 15, sink.events[0].Payload.Limit)
		require.NotNil(t, sink.events[0].Payload.ExpiresAt)
		want := "slot_released"
		if fail {
			want = "slot_release_failed"
		}
		require.Equal(t, want, sink.events[1].Type)
		require.Equal(t, sink.events[0].Payload.SlotID, sink.events[1].Payload.SlotID)
		require.Equal(t, "slot_denied", sink.events[2].Type)
		b, err := json.Marshal(sink.events)
		require.NoError(t, err)
		require.NotContains(t, string(b), "SECRET_RAW_UPSTREAM")
	}
}

func TestOAuthObservationAttemptCorrelatesSlotsAndOutcome(t *testing.T) {
	sink := &observationStoreTest{}
	cache := &stubConcurrencyCacheForTest{acquireResult: true}
	concurrency := NewConcurrencyService(cache)
	concurrency.observations = oauthobs.New(sink, "test", 10)
	concurrency.observations.Start()

	ctx := WithOAuthObservationAttempt(context.Background())
	ctx = WithOAuthObservationSlotMetadata(ctx, "gpt-6-astra", "native", "request")
	result, err := concurrency.AcquireAccountSlot(ctx, 42, 3)
	require.NoError(t, err)
	require.NotEmpty(t, OAuthObservationAttemptID(ctx))
	result.ReleaseFunc()

	gateway := &OpenAIGatewayService{concurrencyService: concurrency}
	gateway.ReportOpenAIAccountScheduleResultWithContext(ctx, stateProbeAccount(), "gpt-6-astra", true, nil)
	finishObservationTest(t, concurrency.observations)

	require.Len(t, sink.events, 3)
	for _, event := range sink.events {
		require.Equal(t, OAuthObservationAttemptID(ctx), event.Payload.AttemptID)
	}
	require.Equal(t, "request", sink.events[0].Payload.SlotRole)
	require.Equal(t, "gpt-6-astra", sink.events[0].Payload.Model)
	require.Equal(t, "native", sink.events[0].Payload.Protocol)
	require.Equal(t, "request_outcome", sink.events[2].Type)
}

func TestOAuthObservationManualProbePreservesVerdictAndPrivacy(t *testing.T) {
	sink := &observationStoreTest{}
	r := oauthobs.New(sink, "test", 10)
	r.Start()
	upstream := &stateProbeUpstream{replies: []stateProbeReply{stateProbeMint("SECRET_TICKET"), {status: 200, body: stateProbeCompletedStream}}}
	gateway := &OpenAIGatewayService{httpUpstream: upstream, concurrencyService: &ConcurrencyService{observations: r}}
	probe := gateway.ProbeOpenAICodexState(context.Background(), stateProbeAccount(), "gpt-6-astra")
	require.Equal(t, OpenAICodexStateHealthy, probe.Verdict)
	finishObservationTest(t, r)
	require.Len(t, sink.events, 1)
	e := sink.events[0]
	require.Equal(t, "probe_result", e.Type)
	require.Equal(t, "healthy", e.Payload.Verdict)
	require.Equal(t, "manual", e.Payload.Source)
	b, err := json.Marshal(e)
	require.NoError(t, err)
	require.NotContains(t, string(b), "SECRET_TICKET")
}

func TestOAuthObservationErrorClassification(t *testing.T) {
	for _, tc := range []struct {
		err   error
		code  int
		class string
	}{
		{context.Canceled, 0, "cancelled"}, {context.DeadlineExceeded, 0, "timeout"},
		{&UpstreamFailoverError{StatusCode: 429, ResponseBody: []byte("SECRET")}, 429, "rate_limited"},
		{&UpstreamFailoverError{StatusCode: 401}, 401, "credential_invalid"},
		{errors.New("secret error body"), 0, "unknown"},
	} {
		code, class := oauthObservationError(tc.err)
		require.Equal(t, tc.code, code)
		require.Equal(t, tc.class, class)
	}
}

func TestOAuthObservationRejectsClientTextInModel(t *testing.T) {
	for _, model := range []string{"sk-sensitive@example.invalid", "please store this private text", "gpt-6-astra-sk-secret"} {
		require.Equal(t, "other_model", observationModel(model))
	}
	require.Equal(t, "gpt-6-astra", observationModel("gpt-6-astra"))
	require.Equal(t, "gpt-5.6-sol", observationModel("gpt-5.6-sol"))
}

func TestOAuthObservationLiveLeaseRefreshAndRelease(t *testing.T) {
	sink := &observationStoreTest{}
	r := oauthobs.New(sink, "live-test", 8)
	r.Start()
	cache := &liveTestConcurrencyCache{}
	gateway := &OpenAIGatewayService{concurrencyService: NewConcurrencyService(cache)}
	gateway.concurrencyService.observations = r
	ctx := WithOAuthObservationAttempt(context.Background())
	record := &LiveCallRecord{AccountID: 42, LeaseID: "internal-lease", Model: "gpt-live", observationAttemptID: OAuthObservationAttemptID(ctx)}
	require.True(t, gateway.refreshLiveLease(record))
	gateway.releaseLiveLease(42, 0, 0, "internal-lease", record.Model, record.observationAttemptID)
	finishObservationTest(t, r)
	require.Len(t, sink.events, 2)
	require.Equal(t, "slot_refreshed", sink.events[0].Type)
	require.Equal(t, "live", sink.events[0].Payload.Protocol)
	require.Equal(t, "live_lease", sink.events[0].Payload.SlotRole)
	require.Equal(t, "gpt-live", sink.events[0].Payload.Model)
	require.Equal(t, OAuthObservationAttemptID(ctx), sink.events[0].Payload.AttemptID)
	require.Equal(t, "slot_released", sink.events[1].Type)
}

func TestOAuthObservationScheduledProbeSource(t *testing.T) {
	sink := &observationStoreTest{}
	r := oauthobs.New(sink, "scheduled-test", 8)
	r.Start()
	s := stateProbeTestService(&stateProbeUpstream{replies: []stateProbeReply{stateProbeMint("ticket"), {status: 200, body: stateProbeCompletedStream}}})
	s.openaiGatewayService.concurrencyService = &ConcurrencyService{observations: r}
	result, err := s.runOpenAICodexStateProbeScheduled(context.Background(), 7, "gpt-6-astra", stateProbePlanConfig())
	require.NoError(t, err)
	require.Equal(t, "success", result.Status)
	finishObservationTest(t, r)
	require.Len(t, sink.events, 1)
	require.Equal(t, "scheduled", sink.events[0].Payload.Source)
}
