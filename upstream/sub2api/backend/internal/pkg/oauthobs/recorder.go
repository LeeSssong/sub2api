// Package oauthobs records bounded, content-free account observations. It does
// not make routing, billing or quality decisions.
package oauthobs

import (
	"context"
	"log/slog"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// Payload is deliberately typed: never add credentials, prompts, response text,
// ticket values, raw errors, emails or client identifiers here.
type Payload struct {
	ProgramVersion    string     `json:"program_version,omitempty"`
	Model             string     `json:"model,omitempty"`
	Protocol          string     `json:"protocol,omitempty"`
	ProbeVersion      string     `json:"probe_version,omitempty"`
	Verdict           string     `json:"verdict,omitempty"`
	Failure           string     `json:"failure,omitempty"`
	LatencyMS         int64      `json:"latency_ms,omitempty"`
	StartedAt         *time.Time `json:"started_at,omitempty"`
	FinishedAt        *time.Time `json:"finished_at,omitempty"`
	Source            string     `json:"source,omitempty"`
	RuleID            int64      `json:"rule_id,omitempty"`
	RoundID           string     `json:"round_id,omitempty"`
	TriggerSource     string     `json:"trigger_source,omitempty"`
	MintStatus        int        `json:"mint_status,omitempty"`
	ContinueStatus    int        `json:"continue_status,omitempty"`
	SlotID            string     `json:"slot_id,omitempty"`
	SlotRole          string     `json:"slot_role,omitempty"`
	AttemptID         string     `json:"attempt_id,omitempty"`
	Limit             int        `json:"limit,omitempty"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
	Layer             string     `json:"layer,omitempty"`
	GroupID           *int64     `json:"group_id,omitempty"`
	Success           *bool      `json:"success,omitempty"`
	FirstTokenMS      *int       `json:"first_token_ms,omitempty"`
	HTTPStatus        int        `json:"http_status,omitempty"`
	ErrorClass        string     `json:"error_class,omitempty"`
	ExcludedCount     int        `json:"excluded_count,omitempty"`
	SelectedAccountID int64      `json:"selected_account_id,omitempty"`
	QueueDelta        int        `json:"queue_delta,omitempty"`
}

type Event struct {
	AccountID  int64     `json:"account_id"`
	Key        string    `json:"event_key"`
	OccurredAt time.Time `json:"occurred_at"`
	Type       string    `json:"event_type"`
	Payload    Payload   `json:"payload"`
}

type Health struct {
	InstanceID                                string
	ObservedAt                                time.Time
	Enqueued, Persisted, Dropped, WriteErrors int64
	QueueDepth                                int
}

type Store interface {
	WriteOAuthObservations(context.Context, []Event) error
	WriteOAuthObservationHealth(context.Context, Health) error
	PruneOAuthObservations(context.Context) error
}

type Recorder struct {
	store       Store
	instanceID  string
	queue       chan Event
	done        chan struct{}
	writerCtx   context.Context
	stopWriter  context.CancelFunc
	mu          sync.RWMutex
	closed      bool
	startOnce   sync.Once
	enqueued    atomic.Int64
	persisted   atomic.Int64
	dropped     atomic.Int64
	writeErrors atomic.Int64
}

func New(store Store, instanceID string, capacity int) *Recorder {
	if capacity < 1 {
		capacity = 4096
	}
	if instanceID == "" {
		instanceID = uuid.NewString()
	}
	writerCtx, stopWriter := context.WithCancel(context.Background())
	return &Recorder{store: store, instanceID: instanceID, queue: make(chan Event, capacity), done: make(chan struct{}), writerCtx: writerCtx, stopWriter: stopWriter}
}

func prepare(e Event) Event {
	if e.Key == "" {
		e.Key = uuid.NewString()
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now().UTC()
	}
	e.Payload.ProgramVersion = programVersion
	e.Payload.GroupID = copyPointer(e.Payload.GroupID)
	e.Payload.Success = copyPointer(e.Payload.Success)
	e.Payload.FirstTokenMS = copyPointer(e.Payload.FirstTokenMS)
	e.Payload.StartedAt = copyPointer(e.Payload.StartedAt)
	e.Payload.FinishedAt = copyPointer(e.Payload.FinishedAt)
	e.Payload.ExpiresAt = copyPointer(e.Payload.ExpiresAt)
	return e
}

func copyPointer[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

var programVersion = func() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" {
				return s.Value
			}
		}
	}
	return "unknown"
}()

// Emit never waits for I/O, including when storage is unavailable. The read lock
// prevents send/close races; lifecycle transitions are not sent through this queue.
func (r *Recorder) Emit(e Event) bool {
	if r == nil || r.store == nil || e.AccountID <= 0 {
		return false
	}
	e = prepare(e)
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed {
		r.dropped.Add(1)
		return false
	}
	select {
	case r.queue <- e:
		r.enqueued.Add(1)
		return true
	default:
		r.dropped.Add(1)
		return false
	}
}

// Critical persists probe evidence before returning the probe result, with a
// bounded timeout and an idempotent queue fallback. Telemetry errors never
// replace the probe verdict or affect account scheduling.
func (r *Recorder) Critical(ctx context.Context, e Event) {
	if r == nil || r.store == nil {
		return
	}
	e = prepare(e)
	if ctx == nil {
		ctx = context.Background()
	}
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	err := r.store.WriteOAuthObservations(writeCtx, []Event{e})
	cancel()
	if err == nil {
		r.enqueued.Add(1)
		r.persisted.Add(1)
		return
	}
	r.writeErrors.Add(1)
	if !r.Emit(e) {
		slog.Warn("oauth_observation_probe_lost", "account_id", e.AccountID, "event_id", e.Key)
	}
}

func (r *Recorder) Start() {
	if r != nil {
		r.startOnce.Do(func() { go r.run() })
	}
}

func (r *Recorder) Health() Health {
	if r == nil {
		return Health{}
	}
	return Health{InstanceID: r.instanceID, ObservedAt: time.Now().UTC(), Enqueued: r.enqueued.Load(), Persisted: r.persisted.Load(), Dropped: r.dropped.Load(), WriteErrors: r.writeErrors.Load(), QueueDepth: len(r.queue)}
}

func (r *Recorder) Stop(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.Start()
	r.mu.Lock()
	if !r.closed {
		r.closed = true
		close(r.queue)
	}
	r.mu.Unlock()
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		r.stopWriter()
		<-r.done
		return ctx.Err()
	}
}

func (r *Recorder) flush(events []Event) bool {
	if len(events) == 0 {
		return true
	}
	for attempt := 0; attempt < 3; attempt++ {
		ctx, cancel := context.WithTimeout(r.writerCtx, 2*time.Second)
		err := r.store.WriteOAuthObservations(ctx, events)
		cancel()
		if err == nil {
			r.persisted.Add(int64(len(events)))
			return true
		}
		r.writeErrors.Add(1)
		if r.writerCtx.Err() != nil {
			break
		}
	}
	r.dropped.Add(int64(len(events)))
	slog.Warn("oauth_observation_batch_lost", "instance_id", r.instanceID, "events", len(events))
	return false
}

func (r *Recorder) health() bool {
	if r.writerCtx.Err() != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(r.writerCtx, 2*time.Second)
	defer cancel()
	h := r.Health()
	if err := r.store.WriteOAuthObservationHealth(ctx, h); err != nil {
		r.writeErrors.Add(1)
		slog.Warn("oauth_observation_health_unavailable", "instance_id", r.instanceID, "dropped", h.Dropped, "write_errors", h.WriteErrors, "queue_depth", h.QueueDepth)
	}
	return r.writerCtx.Err() == nil
}

func (r *Recorder) dropQueued(events []Event) {
	r.dropped.Add(int64(len(events)))
	for range r.queue {
		r.dropped.Add(1)
	}
}

func (r *Recorder) run() {
	defer close(r.done)
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	healthTick := time.NewTicker(10 * time.Second)
	defer healthTick.Stop()
	pruneTick := time.NewTicker(time.Minute)
	defer pruneTick.Stop()
	batch := make([]Event, 0, 128)
	r.health()
	for {
		select {
		case e, ok := <-r.queue:
			if !ok {
				if r.flush(batch) {
					r.health()
				}
				return
			}
			batch = append(batch, e)
			if len(batch) == 128 {
				if !r.flush(batch) {
					r.dropQueued(nil)
					return
				}
				batch = batch[:0]
			}
		case <-tick.C:
			if !r.flush(batch) {
				r.dropQueued(nil)
				return
			}
			batch = batch[:0]
		case <-healthTick.C:
			r.health()
		case <-pruneTick.C:
			ctx, cancel := context.WithTimeout(r.writerCtx, 5*time.Second)
			if err := r.store.PruneOAuthObservations(ctx); err != nil {
				r.writeErrors.Add(1)
				slog.Warn("oauth_observation_retention_failed", "instance_id", r.instanceID)
			}
			cancel()
		}
	}
}
