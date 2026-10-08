-- Keep old production readers compatible while new snapshots omit this retired flag.
-- Stations that already removed the column need no compatibility default.
SET LOCAL lock_timeout = '100ms';
SET LOCAL statement_timeout = '2s';

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'account_monitor_v4_snapshots'
          AND column_name = 'current_operational'
    ) THEN
        ALTER TABLE account_monitor_v4_snapshots
            ALTER COLUMN current_operational SET DEFAULT FALSE;
    END IF;
END $$;
