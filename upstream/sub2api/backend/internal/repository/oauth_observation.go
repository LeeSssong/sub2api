package repository

import (
	"context"
	"encoding/json"

	"github.com/Wei-Shaw/sub2api/internal/pkg/oauthobs"
)

var _ oauthobs.Store = (*accountRepository)(nil)

// A single PostgreSQL statement makes the batch atomic. Stable event keys make
// retries safe even when the commit succeeded but its acknowledgment was lost.
func (r *accountRepository) WriteOAuthObservations(ctx context.Context, events []oauthobs.Event) error {
	if len(events) == 0 {
		return nil
	}
	raw, err := json.Marshal(events)
	if err != nil {
		return err
	}
	_, err = r.sql.ExecContext(ctx, `SELECT oauth_observation_append(
 (v->>'account_id')::bigint,v->>'event_key',(v->>'occurred_at')::timestamptz,
 v->>'event_type',v->'payload') FROM jsonb_array_elements($1::jsonb) AS v`, string(raw))
	return err
}

func (r *accountRepository) WriteOAuthObservationHealth(ctx context.Context, h oauthobs.Health) error {
	_, err := r.sql.ExecContext(ctx, `INSERT INTO oauth_observation_recorder_health
 (instance_id,observed_at,enqueued,persisted,dropped,write_errors,queue_depth)
 VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(instance_id) DO UPDATE SET
 observed_at=EXCLUDED.observed_at,enqueued=EXCLUDED.enqueued,persisted=EXCLUDED.persisted,
 dropped=EXCLUDED.dropped,write_errors=EXCLUDED.write_errors,queue_depth=EXCLUDED.queue_depth`,
		h.InstanceID, h.ObservedAt, h.Enqueued, h.Persisted, h.Dropped, h.WriteErrors, h.QueueDepth)
	return err
}

func (r *accountRepository) PruneOAuthObservations(ctx context.Context) error {
	_, err := r.sql.ExecContext(ctx, `SELECT oauth_observation_prune()`)
	return err
}
