#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
migration="$repo_root/upstream/sub2api/backend/migrations/263_oauth_observations.sql"
image="postgres:16-alpine"
container=""

cleanup() {
  if [[ -n "$container" ]]; then
    docker rm -f "$container" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

[[ -f "$migration" ]] || { echo "missing migration: $migration" >&2; exit 1; }
container="$(docker run -d --tmpfs /var/lib/postgresql/data:rw,size=256m -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=observations -P "$image")"
for _ in $(seq 1 60); do
  if docker exec "$container" pg_isready -U postgres -d observations >/dev/null 2>&1; then break; fi
  sleep 1
done
docker exec "$container" pg_isready -U postgres -d observations >/dev/null

psql() { docker exec -i "$container" psql -v ON_ERROR_STOP=1 -U postgres -d observations "$@"; }

psql <<'SQL'
CREATE TABLE accounts (
  id BIGINT PRIMARY KEY,
  platform TEXT NOT NULL,
  type TEXT NOT NULL,
  credentials JSONB NOT NULL DEFAULT '{}',
  priority INTEGER, concurrency INTEGER, load_factor INTEGER, status TEXT,
  schedulable BOOLEAN, expires_at TIMESTAMPTZ, proxy_id BIGINT
);
CREATE TABLE account_groups (account_id BIGINT NOT NULL, group_id BIGINT NOT NULL, priority INTEGER, PRIMARY KEY(account_id, group_id));
CREATE TABLE scheduled_test_plans (
  id BIGINT PRIMARY KEY, account_id BIGINT NOT NULL, model_id TEXT, cron_expression TEXT,
  enabled BOOLEAN, max_results INTEGER, pelican_config JSONB DEFAULT '{}'
);
CREATE TABLE scheduled_test_results (
  id BIGINT PRIMARY KEY, plan_id BIGINT NOT NULL, status TEXT, response_text TEXT,
  error_message TEXT, latency_ms BIGINT, started_at TIMESTAMPTZ, finished_at TIMESTAMPTZ,
  quality_action TEXT DEFAULT '', quality_judgment JSONB
);
CREATE TABLE usage_logs (
  id BIGINT PRIMARY KEY, account_id BIGINT NOT NULL, model TEXT NOT NULL,
  input_tokens INTEGER NOT NULL DEFAULT 0, output_tokens INTEGER NOT NULL DEFAULT 0,
  cache_creation_tokens INTEGER NOT NULL DEFAULT 0, cache_read_tokens INTEGER NOT NULL DEFAULT 0,
  actual_cost NUMERIC NOT NULL DEFAULT 0, duration_ms INTEGER, first_token_ms INTEGER,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE audit_logs (
  id BIGINT PRIMARY KEY, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), actor_user_id BIGINT,
  actor_role TEXT NOT NULL DEFAULT '', action TEXT NOT NULL DEFAULT '', method TEXT NOT NULL DEFAULT '',
  status_code INTEGER NOT NULL DEFAULT 0, request_body TEXT NOT NULL DEFAULT '', actor_email TEXT NOT NULL DEFAULT '',
  extra JSONB NOT NULL DEFAULT '{}'
);
INSERT INTO accounts VALUES
 (1,'openai','oauth','{"chatgpt_user_id":"user-a","chatgpt_account_id":"workspace-a","access_token":"secret"}',50,2,1,'active',true,NULL,7),
 (2,'anthropic','oauth','{}',50,2,1,'active',true,NULL,7);
SQL
docker cp "$migration" "$container:/tmp/263_oauth_observations.sql"
psql -f /tmp/263_oauth_observations.sql

psql <<'SQL'
-- A failed probe before the first healthy probe never closes a lifetime.
SELECT oauth_observation_append(1, 'before-fail', '2026-01-01 00:00:00+00', 'probe_result',
  '{"model":"gpt-5","protocol":"native","probe_version":"turn_state_v1","verdict":"degraded","failure":"upstream_5xx","latency_ms":10,"started_at":"2026-01-01T00:00:00Z","finished_at":"2026-01-01T00:00:01Z","source":"manual"}');
SELECT oauth_observation_append(1, 'healthy', '2026-01-01 00:01:00+00', 'probe_result',
  '{"model":"gpt-5","protocol":"native","probe_version":"turn_state_v1","verdict":"healthy","latency_ms":11,"started_at":"2026-01-01T00:01:00Z","finished_at":"2026-01-01T00:01:01Z","source":"scheduled"}');
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM oauth_observation_probe_lifetimes WHERE first_degraded_at IS NOT NULL) THEN RAISE EXCEPTION 'pre-pass failure closed lifetime'; END IF;
END $$;
-- Replaying the key is idempotent. An inconclusive probe remains non-terminal.
SELECT oauth_observation_append(1, 'healthy', '2026-01-01 00:01:00+00', 'probe_result',
  '{"model":"gpt-5","protocol":"native","probe_version":"turn_state_v1","verdict":"healthy"}');
SELECT oauth_observation_append(1, 'unknown', '2026-01-01 00:02:00+00', 'probe_result',
  '{"model":"gpt-5","protocol":"native","probe_version":"turn_state_v1","verdict":"inconclusive"}');
DO $$ BEGIN
  IF (SELECT count(*) FROM oauth_observation_events WHERE event_key = 'healthy') <> 1 THEN RAISE EXCEPTION 'duplicate key recorded'; END IF;
END $$;
-- Out-of-order terminal result is evaluated by observed time, not arrival order.
SELECT oauth_observation_append(1, 'late-arrival', '2026-01-01 00:03:00+00', 'probe_result',
  '{"model":"gpt-4","protocol":"native","probe_version":"turn_state_v1","verdict":"degraded","failure":"upstream_5xx"}');
DO $$ BEGIN
  IF (SELECT first_degraded_at FROM oauth_observation_episode_lifetimes) <> '2026-01-01 00:03:00+00'::timestamptz THEN RAISE EXCEPTION 'account-level terminal time missing'; END IF;
  IF EXISTS (SELECT 1 FROM oauth_observation_probe_lifetimes WHERE model = 'gpt-5' AND first_degraded_at IS NOT NULL) THEN RAISE EXCEPTION 'per-model detail was overwritten by another model'; END IF;
END $$;
-- Raw credential values must never be copied to observation data.
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM oauth_observation_events WHERE payload::text LIKE '%secret%') THEN RAISE EXCEPTION 'secret leaked'; END IF;
END $$;
-- Non OpenAI/OAuth accounts are ignored.
SELECT oauth_observation_append(2, 'wrong-account', now(), 'slot_acquired', '{"slot_id":"s2","limit":1,"expires_at":"2026-01-01T01:00:00Z"}');
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM oauth_observation_events WHERE event_key = 'wrong-account') THEN RAISE EXCEPTION 'wrong account type accepted'; END IF;
END $$;
-- Trigger archives survive deletion of an account and scheduled plan.
INSERT INTO account_groups VALUES (1, 99, 3);
INSERT INTO scheduled_test_plans VALUES (8, 1, 'gpt-5', '*/30 * * * *', true, 10, '{"quality":{"action":"enable_bps","auto_restore":true},"untrusted":"drop"}');
INSERT INTO scheduled_test_results VALUES (9, 8, 'failed', 'raw response', 'raw error', 22, now(), now(), 'enable_bps', '{"verdict":"incorrect","raw":"drop"}');
DELETE FROM accounts WHERE id = 1;
DELETE FROM scheduled_test_plans WHERE id = 8;
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM oauth_observation_archives WHERE subject_type = 'account') THEN RAISE EXCEPTION 'account archive erased'; END IF;
  IF NOT EXISTS (SELECT 1 FROM oauth_observation_archives WHERE subject_type = 'scheduled_test_result') THEN RAISE EXCEPTION 'result archive erased'; END IF;
  IF EXISTS (SELECT 1 FROM oauth_observation_archives WHERE snapshot::text LIKE '%raw response%' OR snapshot::text LIKE '%raw error%' OR snapshot::text LIKE '%untrusted%') THEN RAISE EXCEPTION 'archive privacy whitelist failed'; END IF;
END $$;
-- Usage facts aggregate native source values and are corrected on UPDATE.
INSERT INTO accounts VALUES (3,'openai','oauth','{}',50,2,1,'active',true,NULL,7);
INSERT INTO usage_logs VALUES (10,3,'gpt-5',2,3,0,0,1.25,40,12,'2026-01-02 00:00:20+00');
UPDATE usage_logs SET output_tokens = 7, actual_cost = 2.25 WHERE id = 10;
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM oauth_observation_usage_minutes WHERE requests = 1 AND output_tokens = 7 AND actual_cost = 2.25) THEN RAISE EXCEPTION 'usage minute was not updated'; END IF;
END $$;
-- Successful administrator mutations are attributed from approved audit fields only.
INSERT INTO audit_logs(id,actor_user_id,actor_role,action,method,status_code,actor_email,request_body,extra)
VALUES (12, 77, 'admin', 'admin.accounts.update', 'PUT', 200, 'never-copy@example.invalid', '{"secret":"never-copy"}', '{"params":{"id":3}}');
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM oauth_observation_archives WHERE subject_type = 'admin_account_mutation' AND snapshot->>'actor_id' = '77' AND snapshot->>'reason' = 'admin_api') THEN RAISE EXCEPTION 'admin mutation archive missing'; END IF;
  IF EXISTS (SELECT 1 FROM oauth_observation_archives WHERE subject_type = 'admin_account_mutation' AND snapshot::text LIKE '%never-copy%') THEN RAISE EXCEPTION 'audit privacy whitelist failed'; END IF;
END $$;
SELECT 'oauth observation migration behavioral checks passed' AS result;
SQL
