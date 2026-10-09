SET LOCAL lock_timeout = '2s';
SET LOCAL statement_timeout = '30s';

-- Detection evidence is independent of account quarantine and BPS ownership.
-- No foreign keys: deleting rules/accounts must not rewrite request history.
CREATE TABLE IF NOT EXISTS account_quality_traffic_events (
 id BIGSERIAL PRIMARY KEY,
 account_id BIGINT NOT NULL,
 group_id BIGINT NOT NULL,
 plan_id BIGINT NOT NULL,
 observed_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 degraded BOOLEAN NOT NULL
);
CREATE INDEX IF NOT EXISTS account_quality_traffic_events_lookup
 ON account_quality_traffic_events(account_id,group_id,observed_at DESC,id DESC);

-- Seed only explicit, attributable evidence from retained results. Never
-- fabricate request snapshots for old usage or infer degradation from 5xx/BPS.
WITH rounds AS (
 SELECT p.account_id,link.tested_group_id AS group_id,r.plan_id,
 COALESCE(NULLIF(r.quality_round_id,''),r.id::text) AS round_id,
 MAX(r.finished_at) AS finished_at,
 BOOL_OR(r.quality_judgment->>'verdict'='incorrect' AND r.status='failed'
         AND r.error_message IN ('answer_mismatch','state_degraded')) AS wrong,
 BOOL_AND(COALESCE(r.quality_judgment->>'verdict'='correct' AND r.status='success'
          AND COALESCE(r.error_message,'')='',FALSE)) AS all_passed,
 COUNT(*) AS samples,
 MAX(GREATEST(COALESCE(jsonb_array_length(r.pelican_config->'model_ids'),0),1)
     *COALESCE((r.pelican_config->>'parallel_count')::int,0)) AS expected
 FROM scheduled_test_results r
 JOIN scheduled_test_plans p ON p.id=r.plan_id
 JOIN quality_rule_template_accounts link ON link.plan_id=p.id
 WHERE link.tested_group_id IS NOT NULL AND r.pelican_config->'quality' IS NOT NULL
 AND COALESCE(r.quality_action,'') NOT IN ('stale_run','action_error','action_conflict','restore_conflict')
 GROUP BY p.account_id,link.tested_group_id,r.plan_id,COALESCE(NULLIF(r.quality_round_id,''),r.id::text)
), latest AS (
 SELECT DISTINCT ON (account_id,group_id) account_id,group_id,plan_id,wrong
 FROM rounds WHERE wrong OR (all_passed AND samples=expected AND expected>0)
 ORDER BY account_id,group_id,finished_at DESC,plan_id DESC,round_id DESC
)
INSERT INTO account_quality_traffic_events(account_id,group_id,plan_id,degraded)
 SELECT account_id,group_id,plan_id,COALESCE(wrong,FALSE) FROM latest l
 WHERE NOT EXISTS (SELECT 1 FROM account_quality_traffic_events e
                   WHERE e.account_id=l.account_id AND e.group_id=l.group_id);

ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS quality_request_started_at TIMESTAMPTZ;
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS quality_status TEXT
 CHECK (quality_status IN ('healthy','degraded','unmarked'));

CREATE OR REPLACE FUNCTION snapshot_usage_quality_status() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE has_degraded BOOLEAN;
BEGIN
 -- NULL distinguishes historical/uninstrumented rows from an unmarked account.
 IF NEW.quality_request_started_at IS NULL THEN
  NEW.quality_status := NULL;
  RETURN NEW;
 END IF;
 SELECT degraded INTO has_degraded
  FROM account_quality_traffic_events
  WHERE account_id=NEW.account_id AND group_id=NEW.group_id
    AND observed_at<=NEW.quality_request_started_at
  ORDER BY observed_at DESC,id DESC LIMIT 1;
 NEW.quality_status := CASE WHEN has_degraded THEN 'degraded'
                           WHEN has_degraded IS FALSE THEN 'healthy'
                           ELSE 'unmarked' END;
 RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS usage_quality_snapshot ON usage_logs;
CREATE TRIGGER usage_quality_snapshot BEFORE INSERT ON usage_logs
 FOR EACH ROW EXECUTE FUNCTION snapshot_usage_quality_status();
