#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
migration="$repo_root/upstream/sub2api/backend/migrations/263_oauth_observations.sql"
image="postgres:18-alpine"
container=""

cleanup() {
  if [[ -n "$container" ]]; then
    docker rm -f "$container" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

[[ -f "$migration" ]] || { echo "missing migration: $migration" >&2; exit 1; }
container="$(docker run -d --tmpfs /var/lib/postgresql:rw,size=256m -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=observations "$image")"
for _ in $(seq 1 60); do
  if docker exec "$container" psql -h 127.0.0.1 -U postgres -d observations -Atc 'SELECT 1' >/dev/null 2>&1; then break; fi
  sleep 1
done
docker exec "$container" pg_isready -U postgres -d observations >/dev/null

psql() { docker exec -i -e 'PGOPTIONS=-c lock_timeout=100ms -c statement_timeout=2s' "$container" psql -X -q -v ON_ERROR_STOP=1 -U postgres -d observations "$@"; }

psql <<'SQL'
CREATE TABLE accounts (
  id BIGINT PRIMARY KEY,
  platform TEXT NOT NULL,
  type TEXT NOT NULL,
  credentials JSONB NOT NULL DEFAULT '{}',
  priority INTEGER, concurrency INTEGER, load_factor INTEGER, status TEXT,
  schedulable BOOLEAN, expires_at TIMESTAMPTZ, proxy_id BIGINT
);
CREATE TABLE api_keys (id BIGINT PRIMARY KEY);
INSERT INTO api_keys VALUES (1);
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
ALTER TABLE accounts ADD extra JSONB NOT NULL DEFAULT '{}';
ALTER TABLE accounts ADD updated_at TIMESTAMPTZ DEFAULT now();
ALTER TABLE accounts ADD last_used_at TIMESTAMPTZ;
ALTER TABLE accounts ADD deleted_at TIMESTAMPTZ;
ALTER TABLE accounts ADD name TEXT;
ALTER TABLE account_groups ADD allowed_models JSONB;
ALTER TABLE usage_logs ADD total_cost NUMERIC NOT NULL DEFAULT 0;
ALTER TABLE usage_logs ADD upstream_model TEXT;
ALTER TABLE usage_logs ADD upstream_endpoint TEXT;
ALTER TABLE usage_logs ADD reasoning_effort TEXT;
ALTER TABLE usage_logs ADD service_tier TEXT;
ALTER TABLE usage_logs ADD long_context_billing_applied BOOLEAN DEFAULT FALSE;
ALTER TABLE usage_logs ADD usage_completeness TEXT DEFAULT 'complete';
SQL
docker cp "$migration" "$container:/tmp/263_oauth_observations.sql"
docker cp "$repo_root/upstream/sub2api/backend/migrations/237_add_api_key_concurrency_limit.sql" "$container:/tmp/237_add_api_key_concurrency_limit.sql"
psql -1 -f /tmp/237_add_api_key_concurrency_limit.sql
psql -1 -f /tmp/263_oauth_observations.sql
psql <<'SQL'
INSERT INTO api_keys(id) VALUES (2);
DO $$ BEGIN
 IF (SELECT count(*) FROM api_keys WHERE concurrency_limit=0)<>2 THEN RAISE EXCEPTION 'legacy key writes incompatible'; END IF;
END $$;
SQL

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
-- A degraded event can arrive first while an earlier healthy timestamp arrives later.
SELECT oauth_observation_append(1, 'arrival-first-degraded', '2025-12-31 23:00:00+00', 'probe_result', '{"model":"gpt-6","protocol":"native","probe_version":"turn_state_v1","verdict":"degraded"}');
SELECT oauth_observation_append(1, 'arrival-late-earlier-healthy', '2025-12-31 22:00:00+00', 'probe_result', '{"model":"gpt-6","protocol":"native","probe_version":"turn_state_v1","verdict":"healthy"}');
DO $$ BEGIN IF (SELECT first_degraded_at FROM oauth_observation_probe_lifetimes WHERE model='gpt-6') <> '2025-12-31 23:00:00+00'::timestamptz THEN RAISE EXCEPTION 'out-of-order probe result not reconciled'; END IF; END $$;
-- Out-of-order terminal result is evaluated by observed time, not arrival order.
SELECT oauth_observation_append(1, 'late-arrival', '2026-01-01 00:03:00+00', 'probe_result',
  '{"model":"gpt-4","protocol":"native","probe_version":"turn_state_v1","verdict":"degraded","failure":"upstream_5xx"}');
DO $$ BEGIN
  IF (SELECT first_degraded_at FROM oauth_observation_episode_lifetimes) <> '2025-12-31 23:00:00+00'::timestamptz THEN RAISE EXCEPTION 'account-level terminal time missing'; END IF;
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
INSERT INTO account_groups(account_id,group_id,priority) VALUES (1, 99, 3);
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
INSERT INTO accounts(id,platform,type,credentials,priority,concurrency,load_factor,status,schedulable,proxy_id) VALUES (3,'openai','oauth','{}',50,2,1,'active',true,7);
INSERT INTO usage_logs(id,account_id,model,input_tokens,output_tokens,actual_cost,duration_ms,first_token_ms,created_at) VALUES (10,3,'gpt-5',2,3,1.25,40,12,now());
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
-- Pruning raw rows cannot reset the compact lifetime extrema.
UPDATE oauth_observation_events SET recorded_at=clock_timestamp()-INTERVAL '91 days';
SELECT oauth_observation_prune();
SELECT oauth_observation_append(1, 'after-prune-healthy', '2026-02-01 00:00:00+00', 'probe_result', '{"model":"gpt-5","protocol":"native","probe_version":"turn_state_v1","verdict":"healthy"}');
DO $$ BEGIN IF (SELECT first_healthy_at FROM oauth_observation_episode_lifetimes WHERE episode_account_id=1) <> '2025-12-31 22:00:00+00'::timestamptz OR (SELECT first_degraded_at FROM oauth_observation_episode_lifetimes WHERE episode_account_id=1) <> '2025-12-31 23:00:00+00'::timestamptz THEN RAISE EXCEPTION 'retention reset lifetime extrema'; END IF; END $$;
SELECT 'oauth observation migration behavioral checks passed' AS result;
SQL

psql <<'SQL'
-- Cost corrections must replace both billing measures and protocol dimensions.
UPDATE usage_logs SET total_cost=10,upstream_endpoint='/basispoints/api/responses',upstream_model='gpt-6-astra' WHERE id=10;
UPDATE usage_logs SET total_cost=12,usage_completeness='partial' WHERE id=10;
DO $$ BEGIN
 IF (SELECT sum(total_cost) FROM oauth_observation_usage_minutes WHERE account_id=3)<>12 THEN RAISE EXCEPTION 'standard cost correction double counted'; END IF;
 IF NOT EXISTS(SELECT 1 FROM oauth_observation_usage_minutes WHERE account_id=3 AND protocol='bps' AND model='gpt-6-astra' AND requests=1 AND partial_requests=1 AND complete_requests=0) THEN RAISE EXCEPTION 'BPS protocol or completeness correction lost'; END IF;
END $$;
-- Full metadata snapshots must use actual account extra keys and avoid timestamp-only noise.
UPDATE accounts SET extra='{"openai_excel_bps":true,"openai_excel_bps_models":["gpt-6-astra"],"codex_5h_used_percent":75,"access_token":"DO_NOT_CAPTURE"}' WHERE id=3;
DO $$ DECLARE n BIGINT; BEGIN
 IF NOT EXISTS(SELECT 1 FROM oauth_observation_archives WHERE subject_type='account' AND subject_key='3' AND snapshot->'after'->>'bps_enabled'='true') THEN RAISE EXCEPTION 'extra-only BPS change missing'; END IF;
 SELECT count(*) INTO n FROM oauth_observation_archives WHERE subject_type='account' AND subject_key='3';
 UPDATE accounts SET last_used_at=now(),updated_at=now() WHERE id=3;
 IF n<>(SELECT count(*) FROM oauth_observation_archives WHERE subject_type='account' AND subject_key='3') THEN RAISE EXCEPTION 'timestamp-only write created config event'; END IF;
 IF EXISTS(SELECT 1 FROM oauth_observation_archives WHERE snapshot::text LIKE '%DO_NOT_CAPTURE%') THEN RAISE EXCEPTION 'extra secret leaked'; END IF;
END $$;
INSERT INTO account_groups(account_id,group_id,priority,allowed_models) VALUES(3,90,1,'["gpt-6-astra"]');
INSERT INTO scheduled_test_plans VALUES (18,3,'gpt-6-astra','*/2 * * * *',true,100,'{"question_kind":"state_probe","parallel_count":1,"quality":{"action":"enable_bps","bps":{"failure_threshold":2,"usage_percent":90,"require_all":false},"expected_answer":"DO_NOT_CAPTURE"}}');
ALTER TABLE scheduled_test_results ADD quality_round_id TEXT;
ALTER TABLE scheduled_test_results ADD pelican_config JSONB;
INSERT INTO scheduled_test_results(id,plan_id,status,quality_action,quality_round_id,pelican_config)
 VALUES(19,18,'failed','bps_enabled','round-18','{"question_kind":"state_probe","quality":{"action":"enable_bps","expected_answer":"DO_NOT_CAPTURE","bps":{"failure_threshold":2}}}');
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM oauth_observation_archives WHERE subject_type='scheduled_test_plan' AND subject_key='18' AND snapshot->'after'->'quality'->'bps'->>'failure_threshold'='2') THEN RAISE EXCEPTION 'BPS threshold missing'; END IF;
 IF NOT EXISTS(SELECT 1 FROM oauth_observation_archives WHERE subject_type='account_group' AND snapshot->'after'->'allowed_models'='["gpt-6-astra"]'::jsonb) THEN RAISE EXCEPTION 'model allowlist missing'; END IF;
 IF NOT EXISTS(SELECT 1 FROM oauth_observation_archives WHERE subject_type='scheduled_test_result' AND snapshot->>'round_id'='round-18' AND snapshot->'rule_snapshot'->'quality'->'bps'->>'failure_threshold'='2') THEN RAISE EXCEPTION 'quality round correlation missing'; END IF;
 IF EXISTS(SELECT 1 FROM oauth_observation_archives WHERE snapshot::text LIKE '%DO_NOT_CAPTURE%') THEN RAISE EXCEPTION 'quality rule content leaked'; END IF;
END $$;
-- Release failure is not a successful release, including reordered delivery.
SELECT oauth_observation_append(3,'slot-fail','2026-10-01 01:00:30Z','slot_release_failed','{"slot_id":"lease-a"}');
SELECT oauth_observation_append(3,'slot-start','2026-10-01 01:00:00Z','slot_acquired','{"slot_id":"lease-a","limit":15,"expires_at":"2026-10-01 01:01:00Z"}');
SELECT oauth_observation_append(3,'slot-refresh','2026-10-01 01:00:45Z','slot_refreshed','{"slot_id":"lease-a","expires_at":"2026-10-01 01:01:45Z"}');
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM oauth_observation_slots WHERE slot_id='lease-a' AND released_at IS NOT NULL) THEN RAISE EXCEPTION 'failed release closed slot'; END IF;
END $$;
-- Neither arbitrary audit route ids nor long digits may abort unrelated logging.
INSERT INTO audit_logs(id,extra) VALUES(20,'{"params":{"id":"not-a-number"}}'),(21,'{"params":{"id":"999999999999999999999999999999"}}');
SELECT 'extended observation checks passed' AS result;
INSERT INTO usage_logs(id,account_id,model,total_cost,reasoning_effort,service_tier)
 VALUES(22,3,'sk-client-secret@example.invalid',1,'sk-client-secret@example.invalid','sk-client-secret@example.invalid');
SELECT oauth_observation_append(3,'untrusted-model',now(),'probe_result','{"model":"sk-client-secret@example.invalid","protocol":"native","probe_version":"turn_state_v1","verdict":"inconclusive"}');
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM oauth_observation_usage_contributions WHERE model LIKE '%sk-client-secret%' OR reasoning_effort LIKE '%sk-client-secret%' OR service_tier LIKE '%sk-client-secret%') THEN RAISE EXCEPTION 'client label leaked into usage observations'; END IF;
 IF EXISTS(SELECT 1 FROM oauth_observation_events WHERE payload::text LIKE '%sk-client-secret%') THEN RAISE EXCEPTION 'client model leaked into event'; END IF;
END $$;
SQL

psql <<'SQL'
SELECT oauth_observation_append(3,'a1','2026-10-01 02:00:00Z','slot_acquired','{"slot_id":"a","limit":2,"expires_at":"2026-10-01 03:00:00Z"}');
SELECT oauth_observation_append(3,'b1','2026-10-01 02:00:30Z','slot_acquired','{"slot_id":"b","limit":2,"expires_at":"2026-10-01 03:00:00Z"}');
SELECT oauth_observation_append(3,'b2','2026-10-01 02:01:00Z','slot_released','{"slot_id":"b"}');
SELECT oauth_observation_append(3,'a2','2026-10-01 02:01:30Z','slot_released','{"slot_id":"a"}');
SELECT oauth_observation_append(3,'admission1','2026-10-01 02:00:00Z','slot_acquired','{"slot_id":"admission","limit":2,"slot_role":"live_admission","expires_at":"2026-10-01 03:00:00Z"}');
SELECT oauth_observation_append(3,'admission2','2026-10-01 02:01:30Z','slot_released','{"slot_id":"admission"}');
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM oauth_observation_occupancy('2026-10-01 02:00:00Z','2026-10-01 02:02:00Z',3) WHERE minute_at='2026-10-01 02:00:00Z' AND occupied_seconds=90 AND average_concurrency=1.5 AND peak_concurrency=2 AND full_capacity_seconds=30) THEN RAISE EXCEPTION 'overlap/peak/saturation sweep incorrect'; END IF;
 IF NOT EXISTS(SELECT 1 FROM oauth_observation_occupancy('2026-10-01 02:00:00Z','2026-10-01 02:02:00Z',3) WHERE minute_at='2026-10-01 02:01:00Z' AND occupied_seconds=30 AND average_concurrency=.5 AND peak_concurrency=1) THEN RAISE EXCEPTION 'minute boundary allocation incorrect'; END IF;
 IF NOT EXISTS(SELECT 1 FROM oauth_observation_occupancy('2026-10-01 01:00:00Z','2026-10-01 01:02:00Z',3) WHERE incomplete_slots>0 AND average_concurrency IS NULL AND peak_concurrency IS NULL) THEN RAISE EXCEPTION 'open slot incorrectly treated as complete'; END IF;
END $$;
SQL
docker cp "$repo_root/docs/reports/oauth-observation-reports.sql" "$container:/tmp/oauth-observation-reports.sql"
psql -v from='2026-10-01T00:00:00Z' -v to='2026-10-02T00:00:00Z' -v account_id=3 -f /tmp/oauth-observation-reports.sql >/dev/null
echo 'migration, correction, retention, occupancy and executable report tests passed'
