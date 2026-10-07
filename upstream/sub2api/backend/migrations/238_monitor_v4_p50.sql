ALTER TABLE account_monitor_v4_snapshots
    ADD COLUMN IF NOT EXISTS ttft_p50_ms double precision,
    ADD COLUMN IF NOT EXISTS latency_p50_ms double precision;

-- Old operational flags did not enforce freshness/first-token thresholds.
-- Older databases still have this column; stations that already retired it
-- must be able to apply the additive migration set as well.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'account_monitor_v4_snapshots'
          AND column_name = 'current_operational'
    ) THEN
        UPDATE account_monitor_v4_snapshots SET current_operational = FALSE;
    END IF;
END $$;
