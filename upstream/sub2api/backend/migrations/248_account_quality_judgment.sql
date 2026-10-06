-- Online fusion: fail promptly on live-table lock contention, including worker startup.
SET LOCAL lock_timeout = '100ms';
SET LOCAL statement_timeout = '2s';

ALTER TABLE scheduled_test_results ADD COLUMN IF NOT EXISTS quality_judgment JSONB;
ALTER TABLE scheduled_test_results ADD COLUMN IF NOT EXISTS quality_round_id TEXT NOT NULL DEFAULT '';
