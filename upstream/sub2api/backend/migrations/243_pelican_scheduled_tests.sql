-- Online fusion: fail promptly on live-table lock contention, including worker startup.
SET LOCAL lock_timeout = '100ms';
SET LOCAL statement_timeout = '2s';

-- NULL configuration preserves existing connectivity tests.
ALTER TABLE scheduled_test_plans ADD COLUMN IF NOT EXISTS pelican_config JSONB;
ALTER TABLE scheduled_test_plans ADD COLUMN IF NOT EXISTS running_until TIMESTAMPTZ;
ALTER TABLE scheduled_test_results ADD COLUMN IF NOT EXISTS pelican_config JSONB;
