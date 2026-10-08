-- Route health is derived from one-hour real request counts.
-- Stop the old API/worker before applying; old binaries still read this column.
ALTER TABLE account_monitor_v4_snapshots DROP COLUMN IF EXISTS current_operational;
