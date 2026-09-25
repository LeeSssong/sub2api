-- Runner bounds lock waits, prechecks duplicates, and repairs an invalid prior attempt.
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS scheduled_test_quality_account_unique
 ON scheduled_test_plans(account_id) WHERE pelican_config->'quality' IS NOT NULL;
