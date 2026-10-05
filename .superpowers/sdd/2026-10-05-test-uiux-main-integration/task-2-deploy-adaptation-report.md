# Test station deployment adaptation

Commit: `a66f368405678edd33b60b5710e1a216338bd1f2`

- `ops/release-sub2api-test-station.sh` accepts the explicitly approved `root@43.133.75.82` target only with `/tmp/sub2api-uiux-verified-known-hosts` mode 0600, port 22, strict host key checking, and `ssh -G` hostname/port verification. Default `sub2api-test-station` behavior is unchanged; all other targets fail.
- `ops/deploy-sub2api-test-station-host.sh` performs the authorized maintenance stop before the consistent backup when migration 241 requires downtime. Failure recovery restores the complete stopped-writes PostgreSQL dump, Redis RDB and app-data archive before starting the previous application services. It no longer edits `schema_migrations` or pretends an old image reverses a migration.

Validation: `bash -n` passes; `tests/operations/release_sub2api_test_station_contract_test.sh` passes. The existing deploy host fixture still expects the retired migration-column hack and lacks complete backup artifacts, so its route rollback assertion fails against the safer full-restore contract; no live deployment, SSH, build, push or database operation was performed.
