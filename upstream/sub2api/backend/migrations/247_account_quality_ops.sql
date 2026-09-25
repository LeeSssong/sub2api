-- Online fusion: fail promptly on live-table lock contention, including worker startup.
SET LOCAL lock_timeout = '100ms';
SET LOCAL statement_timeout = '2s';

-- One quality rule owns account mutations, including while paused.
-- Uniqueness is installed by 252_account_quality_unique_notx.sql.
ALTER TABLE scheduled_test_results ADD COLUMN IF NOT EXISTS quality_action TEXT NOT NULL DEFAULT '';
CREATE TABLE IF NOT EXISTS account_quality_states (
 plan_id BIGINT PRIMARY KEY REFERENCES scheduled_test_plans(id) ON DELETE CASCADE,
 state JSONB NOT NULL DEFAULT '{}'
);
