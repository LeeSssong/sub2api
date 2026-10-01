-- OAuth observation reports. Execute with psql variables: -v from='2026-01-01T00:00:00Z' -v to='2026-01-02T00:00:00Z'.
-- Account labels require an authorized join to accounts outside this file. Never join credentials.

-- Primary account-episode lifetime: only a degraded probe strictly after the first healthy probe is terminal.
SELECT date_trunc('week', first_healthy_at) AS cohort_week,
       count(*) AS started_episodes,
       count(first_degraded_at) AS ended_episodes,
       percentile_cont(0.5) WITHIN GROUP (ORDER BY first_degraded_at - first_healthy_at)
         FILTER (WHERE first_degraded_at IS NOT NULL) AS median_lifetime
FROM oauth_observation_episode_lifetimes
GROUP BY 1
ORDER BY cohort_week DESC;

-- Explanatory series detail remains separate and never resets the primary account lifetime.
SELECT model, protocol, probe_version, count(*) AS started_series, count(first_degraded_at) AS ended_series
FROM oauth_observation_probe_lifetimes
GROUP BY model, protocol, probe_version
ORDER BY model, protocol, probe_version;

-- A single episode timeline. Absence means no recorded event, never a zero-valued metric.
SELECT occurred_at, event_type, payload, recorded_at
FROM oauth_observation_events
WHERE episode_account_id = $1
ORDER BY occurred_at, id;

-- Native usage facts copied from usage_logs, by minute/model/protocol.
SELECT minute_at, account_id AS episode_account_id, model, protocol,
       requests, input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
       actual_cost,
       CASE WHEN latency_count = 0 THEN NULL ELSE latency_ms_total::numeric / latency_count END AS avg_latency_ms,
       CASE WHEN first_token_count = 0 THEN NULL ELSE first_token_ms_total::numeric / first_token_count END AS avg_first_token_ms
FROM oauth_observation_usage_minutes
WHERE minute_at >= $1 AND minute_at < $2
ORDER BY minute_at, episode_account_id, model, protocol;

-- Slot occupancy, including explicitly incomplete open intervals. A NULL occupied_seconds is not zero.
SELECT date_trunc('minute', acquired_at) AS minute_at, episode_account_id,
       count(*) FILTER (WHERE released_at IS NULL) AS open_incomplete_slots,
       count(*) FILTER (WHERE released_at IS NOT NULL) AS closed_slots,
       sum(EXTRACT(EPOCH FROM (released_at - acquired_at))) FILTER (WHERE released_at IS NOT NULL) AS occupied_seconds
FROM oauth_observation_slots
WHERE acquired_at >= $1 AND acquired_at < $2
GROUP BY 1, 2
ORDER BY minute_at, episode_account_id;

-- Snapshot history is observational. It supports timelines and coverage checks, not causal claims.
SELECT observed_at, subject_type, subject_key, operation, snapshot
FROM oauth_observation_archives
WHERE subject_type IN ('account', 'account_group', 'scheduled_test_plan', 'scheduled_test_result')
  AND observed_at >= $1 AND observed_at < $2
ORDER BY observed_at, id;

-- Recorder coverage: counters are instance-local snapshots. Missing intervals are unknown coverage.
SELECT instance_id, observed_at, enqueued, persisted, dropped, write_errors, queue_depth,
       CASE WHEN enqueued = 0 THEN NULL ELSE persisted::numeric / enqueued END AS persisted_ratio
FROM oauth_observation_recorder_health
WHERE observed_at >= $1 AND observed_at < $2
ORDER BY instance_id, observed_at;
