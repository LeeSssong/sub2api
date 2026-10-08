# Deferred Destructive Migration

`241_remove_monitor_v4_operational_flag.sql` is preserved unchanged for provenance.
It is intentionally outside the embedded `*.sql` migration set. The production
blue/green predecessor still reads and writes this column, so this release keeps
it and adds a conditional default through `265_monitor_v4_legacy_default.sql`.

Do not move the destructive migration back into the active set until every reader,
writer and rollback artifact has been checked for compatibility. Stations that
already applied it retain their original migration receipt and work without the
column. No receipt is fabricated on production.
