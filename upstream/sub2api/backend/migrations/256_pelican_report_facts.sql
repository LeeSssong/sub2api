-- Additive public-report projection. No account, credentials, prompt or raw errors.
-- Source identities intentionally have no foreign keys: native history may be pruned.
SET LOCAL lock_timeout = '2s';
SET LOCAL statement_timeout = '30s';
CREATE TABLE IF NOT EXISTS pelican_report_facts (
 source_result_id BIGINT PRIMARY KEY,
 group_ids BIGINT[] NOT NULL,
 model_id VARCHAR(100) NOT NULL,
 kind TEXT NOT NULL CHECK (kind IN ('candy','pelican')),
 status TEXT NOT NULL CHECK (status IN ('success','failed','ungraded')),
 judgment TEXT NOT NULL CHECK (judgment IN ('builtin_candy','ungraded_candy','drawing')),
 execution_id TEXT,
 shared_round_id TEXT,
 expected_count INTEGER CHECK (expected_count BETWEEN 1 AND 8),
 scheduled_for TIMESTAMPTZ,
 started_at TIMESTAMPTZ,
 completed_at TIMESTAMPTZ,
 observed_at TIMESTAMPTZ NOT NULL,
 CHECK ((execution_id IS NULL) = (expected_count IS NULL)),
 CHECK (shared_round_id IS NULL OR (execution_id IS NOT NULL AND scheduled_for IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS idx_pelican_report_facts_time ON pelican_report_facts(observed_at,source_result_id);
CREATE INDEX IF NOT EXISTS idx_pelican_report_facts_model_time ON pelican_report_facts(model_id,observed_at DESC);
CREATE INDEX IF NOT EXISTS idx_pelican_report_facts_groups ON pelican_report_facts USING GIN(group_ids);
CREATE INDEX IF NOT EXISTS idx_pelican_report_facts_execution ON pelican_report_facts(execution_id) WHERE execution_id IS NOT NULL;
COMMENT ON TABLE pelican_report_facts IS '48-hour scheduled result facts with execution-start group attribution; GET cannot trigger detection';

-- A pair slot preserves the original due time after the first plan advances its
-- next_run_at. It is written only after a claim succeeds and is never inferred
-- from completion-time proximity.
CREATE TABLE IF NOT EXISTS pelican_report_pair_slots (
 account_id BIGINT NOT NULL,
 pair_key TEXT NOT NULL,
 model_id VARCHAR(100) NOT NULL,
 cron_expression TEXT NOT NULL,
 scheduled_for TIMESTAMPTZ NOT NULL,
 shared_round_id TEXT NOT NULL,
 membership_fingerprint TEXT NOT NULL,
 timezone TEXT NOT NULL,
 PRIMARY KEY (account_id, pair_key, model_id, cron_expression, scheduled_for, membership_fingerprint, timezone)
);
CREATE INDEX IF NOT EXISTS idx_pelican_report_pair_slots_time
 ON pelican_report_pair_slots(scheduled_for);
-- Only proven historical drawing facts: execution-start membership already exists in
-- outcomes, model and times come from the saved result snapshot. Never infer batches.
INSERT INTO pelican_report_facts(source_result_id,group_ids,model_id,kind,status,judgment,started_at,completed_at,observed_at)
 SELECT o.source_result_id,o.group_ids,r.pelican_config->>'model_id','pelican',
  CASE WHEN o.success THEN 'success' ELSE 'failed' END,'drawing',
  CASE WHEN r.started_at > TIMESTAMPTZ '1970-01-01' THEN r.started_at END,
  CASE WHEN r.finished_at > TIMESTAMPTZ '1970-01-01' THEN r.finished_at END,o.completed_at
 FROM pelican_drawing_outcomes o JOIN scheduled_test_results r ON r.id=o.source_result_id
 WHERE o.completed_at >= NOW()-INTERVAL '48 hours'
 AND NULLIF(BTRIM(r.pelican_config->>'model_id'),'') IS NOT NULL
 AND LENGTH(r.pelican_config->>'model_id')<=100
 ON CONFLICT(source_result_id) DO NOTHING;
