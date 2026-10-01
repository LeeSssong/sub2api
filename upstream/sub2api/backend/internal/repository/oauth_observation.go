package repository

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/Wei-Shaw/sub2api/internal/pkg/oauthobs"
)

var _ oauthobs.Store = (*accountRepository)(nil)

// A single PostgreSQL statement makes the batch atomic. Stable event keys make
// retries safe even when the commit succeeded but its acknowledgment was lost.
func (r *accountRepository) WriteOAuthObservations(ctx context.Context, events []oauthobs.Event) error {
	if len(events) == 0 {
		return nil
	}
	// Episode rows are locked by the append routine. Sorting a copied batch gives
	// concurrent recorders one lock order without changing the caller's order.
	ordered := append([]oauthobs.Event(nil), events...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].AccountID < ordered[j].AccountID })
	raw, err := json.Marshal(ordered)
	if err != nil {
		return err
	}
	_, err = r.sql.ExecContext(ctx, `SELECT oauth_observation_append(
 (v->>'account_id')::bigint,v->>'event_key',(v->>'occurred_at')::timestamptz,
 v->>'event_type',v->'payload') FROM jsonb_array_elements($1::jsonb) AS v`, string(raw))
	return err
}

func (r *accountRepository) WriteOAuthObservationHealth(ctx context.Context, h oauthobs.Health) error {
	_, err := r.sql.ExecContext(ctx, `WITH latest AS (
 INSERT INTO oauth_observation_recorder_health
 (instance_id,observed_at,enqueued,persisted,dropped,write_errors,queue_depth)
 VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(instance_id) DO UPDATE SET
 observed_at=EXCLUDED.observed_at,enqueued=EXCLUDED.enqueued,persisted=EXCLUDED.persisted,
 dropped=EXCLUDED.dropped,write_errors=EXCLUDED.write_errors,queue_depth=EXCLUDED.queue_depth
 RETURNING instance_id,observed_at,enqueued,persisted,dropped,write_errors,queue_depth
)
INSERT INTO oauth_observation_recorder_health_minutes
 (instance_id,minute_at,observed_at,enqueued,persisted,dropped,write_errors,queue_depth)
SELECT instance_id,date_trunc('minute',observed_at),observed_at,enqueued,persisted,dropped,write_errors,queue_depth FROM latest
ON CONFLICT(instance_id,minute_at) DO UPDATE SET
 observed_at=EXCLUDED.observed_at,enqueued=EXCLUDED.enqueued,persisted=EXCLUDED.persisted,
 dropped=EXCLUDED.dropped,write_errors=EXCLUDED.write_errors,
 queue_depth=GREATEST(oauth_observation_recorder_health_minutes.queue_depth,EXCLUDED.queue_depth)`,
		h.InstanceID, h.ObservedAt, h.Enqueued, h.Persisted, h.Dropped, h.WriteErrors, h.QueueDepth)
	return err
}

func (r *accountRepository) PruneOAuthObservations(ctx context.Context) error {
	_, err := r.sql.ExecContext(ctx, `SELECT oauth_observation_prune()`)
	return err
}
