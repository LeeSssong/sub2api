package oauthobs

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type memoryStore struct {
	mu     sync.Mutex
	events map[string]Event
	fail   int
	health Health
}

func (s *memoryStore) WriteOAuthObservations(_ context.Context, events []Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail > 0 {
		s.fail--
		return errors.New("offline")
	}
	if s.events == nil {
		s.events = map[string]Event{}
	}
	for _, e := range events {
		s.events[e.Key] = e
	}
	return nil
}
func (s *memoryStore) WriteOAuthObservationHealth(_ context.Context, h Health) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.health = h
	return nil
}
func (s *memoryStore) PruneOAuthObservations(context.Context) error { return nil }

// Lost events after an exhausted retry budget must be visible, not counted as persisted.
func TestRecorderRetryAndLoss(t *testing.T) {
	for _, tc := range []struct {
		failures           int
		persisted, dropped int64
	}{{2, 1, 0}, {3, 0, 1}} {
		s := &memoryStore{fail: tc.failures}
		r := New(s, "test", 2)
		if !r.Emit(Event{AccountID: 42, Type: "slot_denied", Key: "same-event"}) {
			t.Fatal("enqueue")
		}
		r.Start()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := r.Stop(ctx); err != nil {
			t.Fatal(err)
		}
		h := r.Health()
		if h.Persisted != tc.persisted || h.Dropped != tc.dropped || h.WriteErrors != int64(tc.failures) {
			t.Fatalf("unexpected health: %+v", h)
		}
		if len(s.events) != int(tc.persisted) {
			t.Fatalf("events: %v", s.events)
		}
	}
}

func TestRecorderSaturationAndConcurrentClose(t *testing.T) {
	s := &memoryStore{}
	r := New(s, "test", 1)
	if !r.Emit(Event{AccountID: 1, Type: "slot_denied"}) || r.Emit(Event{AccountID: 1, Type: "slot_denied"}) {
		t.Fatal("queue must be bounded")
	}
	r.Start()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				r.Emit(Event{AccountID: 1, Type: "slot_denied"})
			}
		}()
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := r.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if r.Emit(Event{AccountID: 1, Type: "slot_denied"}) {
		t.Fatal("accepted after shutdown")
	}
	h := r.Health()
	if h.Persisted+h.Dropped != 203 {
		t.Fatalf("unaccounted events %+v", h)
	}
}

func TestCriticalProbeRetriesWithStableIdentity(t *testing.T) {
	s := &memoryStore{fail: 1}
	r := New(s, "test", 4)
	r.Start()
	r.Critical(context.Background(), Event{AccountID: 1, Type: "probe_result", Key: "probe-1", Payload: Payload{Verdict: "healthy"}})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := r.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if len(s.events) != 1 || s.events["probe-1"].Payload.Verdict != "healthy" {
		t.Fatalf("missing probe: %v", s.events)
	}
}

func TestRecorderCopiesCallerOwnedPayload(t *testing.T) {
	s := &memoryStore{}
	r := New(s, "test", 4)
	group := int64(8)
	success := true
	firstToken := 250
	r.Emit(Event{AccountID: 1, Key: "immutable", Type: "selected", Payload: Payload{GroupID: &group, Success: &success, FirstTokenMS: &firstToken}})
	group = 99
	success = false
	firstToken = 999
	r.Start()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := r.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	p := s.events["immutable"].Payload
	if *p.GroupID != 8 || !*p.Success || *p.FirstTokenMS != 250 {
		t.Fatalf("caller modified queued event: %+v", p)
	}
}

func TestRecorderResumesAfterBatchFailure(t *testing.T) {
	s := &memoryStore{fail: 3}
	r := New(s, "recovery", 512)
	r.Start()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = r.Stop(ctx)
	}()
	for i := 0; i < 128; i++ {
		r.Emit(Event{AccountID: 1, Type: "slot_denied"})
	}
	deadline := time.Now().Add(2 * time.Second)
	for r.Health().Dropped < 128 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if r.Health().Dropped != 128 {
		t.Fatal("failed batch was not accounted")
	}
	for i := 0; i < 128; i++ {
		r.Emit(Event{AccountID: 1, Type: "slot_denied"})
	}
	deadline = time.Now().Add(2 * time.Second)
	for r.Health().Persisted < 128 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if r.Health().Persisted != 128 {
		t.Fatalf("recorder failed to recover: %+v", r.Health())
	}
}

func TestRecorderStopDeadlineCancelsWriterAndAccountsForQueuedEvents(t *testing.T) {
	store := &blockingStore{entered: make(chan struct{})}
	r := New(store, "test", 4)
	for i := 0; i < 3; i++ {
		requireEmit(t, r, Event{AccountID: 1, Type: "selected"})
	}
	r.Start()
	<-store.entered
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if err := r.Stop(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Stop error = %v, want deadline exceeded", err)
	}
	select {
	case <-r.done:
	case <-time.After(time.Second):
		t.Fatal("recorder writer continued after shutdown deadline")
	}
	h := r.Health()
	if h.Dropped != 3 {
		t.Fatalf("queued events must be visible as dropped: %+v", h)
	}
}

type blockingStore struct{ entered chan struct{} }

func (s *blockingStore) WriteOAuthObservations(ctx context.Context, _ []Event) error {
	select {
	case s.entered <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return ctx.Err()
}
func (*blockingStore) WriteOAuthObservationHealth(context.Context, Health) error { return nil }
func (*blockingStore) PruneOAuthObservations(context.Context) error              { return nil }

func requireEmit(t *testing.T, r *Recorder, event Event) {
	t.Helper()
	if !r.Emit(event) {
		t.Fatal("enqueue")
	}
}
