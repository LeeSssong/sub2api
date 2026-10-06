# Test station deployment adaptation

Commit: `ffa43b516197279a34cdac8edf4d8f3a7bc45f5e`

- `ops/release-sub2api-test-station.sh` accepts the explicitly approved `ubuntu@43.133.75.82` target only with the protected key, `/tmp/sub2api-uiux-verified-known-hosts` mode 0600, port 22, strict host key checking, and `ssh -G` hostname/port verification. Default `sub2api-test-station` behavior is unchanged; all other targets fail.
- `ops/deploy-sub2api-test-station-host.sh` performs the authorized maintenance stop before the consistent backup. Failure recovery restores the complete stopped-writes PostgreSQL dump, Redis RDB and named app-data volume before starting the previous application services. It no longer edits `schema_migrations` or pretends an old image reverses a migration.

Validation: `bash -n` passes; backup and release contract tests pass. The deploy fixture needs further fake Docker alignment for the new maintenance mode; no live deployment, SSH, build, push or database operation was performed.
