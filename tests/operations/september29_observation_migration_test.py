"""Exercise the official index migration on disposable PostgreSQL, never production."""
import os
import pathlib
import subprocess
import time
import uuid

ROOT = pathlib.Path(__file__).resolve().parents[2]
DOCKER = ['docker', '--context', os.environ.get('TEST_DOCKER_CONTEXT', 'colima')]
NAME = 'sub2api-sep29-migration-' + uuid.uuid4().hex[:8]
SQL = (ROOT / 'upstream/sub2api/backend/migrations/258_quality_observation_scope.sql').read_text()

def query(sql, check=True):
    return subprocess.run(DOCKER + ['exec', '-i', NAME, 'psql', '-X', '-U', 'postgres', '-v', 'ON_ERROR_STOP=1', '-At'], input=sql, text=True, capture_output=True, check=check)

subprocess.run(DOCKER + ['run', '--rm', '-d', '--name', NAME, '--network', 'none', '--tmpfs', '/var/lib/postgresql:size=256m', '-e', 'POSTGRES_HOST_AUTH_METHOD=trust', 'postgres:18-alpine'], check=True, stdout=subprocess.DEVNULL)
try:
    deadline = time.monotonic() + 60
    while query('SELECT 1', check=False).returncode:
        if time.monotonic() > deadline:
            raise RuntimeError('disposable PostgreSQL not ready')
        time.sleep(.2)
    query("""
CREATE TABLE scheduled_test_plans(id bigint PRIMARY KEY, account_id bigint, pelican_config jsonb);
CREATE UNIQUE INDEX scheduled_test_quality_account_scope_unique ON scheduled_test_plans
(account_id,(CASE WHEN pelican_config->'quality'->>'action'='enable_bps' THEN 'bps' ELSE 'quarantine' END))
WHERE pelican_config->'quality' IS NOT NULL;
INSERT INTO scheduled_test_plans VALUES
(1,427,'{"quality":{"action":"enable_bps"}}'),
(2,427,'{"quality":{"action":"disable_scheduling"}}');
""")
    before = query('SELECT id,account_id,pelican_config FROM scheduled_test_plans ORDER BY id').stdout
    index_before = query("SELECT indexdef FROM pg_indexes WHERE indexname='scheduled_test_quality_account_scope_unique'").stdout
    locker = subprocess.Popen(DOCKER + ['exec', '-i', NAME, 'psql', '-X', '-U', 'postgres', '-At'], stdin=subprocess.PIPE, stdout=subprocess.PIPE, text=True)
    locker.stdin.write('BEGIN; LOCK scheduled_test_plans IN ACCESS EXCLUSIVE MODE; SELECT 123;\n')
    locker.stdin.flush()
    while locker.stdout.readline().strip() != '123':
        if locker.poll() is not None: raise RuntimeError('lock holder failed')
    blocked = query("BEGIN; SET LOCAL lock_timeout='100ms'; SET LOCAL statement_timeout='2s';\n" + SQL + '\nCOMMIT;', check=False)
    assert blocked.returncode != 0 and 'lock timeout' in blocked.stderr, blocked.stderr
    locker.stdin.write('ROLLBACK;\n'); locker.stdin.close(); assert locker.wait(timeout=5) == 0
    assert query("SELECT indexdef FROM pg_indexes WHERE indexname='scheduled_test_quality_account_scope_unique'").stdout == index_before
    started = time.monotonic()
    query("BEGIN; SET LOCAL lock_timeout='100ms'; SET LOCAL statement_timeout='2s';\n" + SQL + '\nCOMMIT;')
    assert query('SELECT id,account_id,pelican_config FROM scheduled_test_plans ORDER BY id').stdout == before
    query("INSERT INTO scheduled_test_plans VALUES (3,427,'{\"quality\":{\"action\":\"observe_only\"}}');")
    duplicate = query("INSERT INTO scheduled_test_plans VALUES (4,427,'{\"quality\":{\"action\":\"observe_only\"}}');", check=False)
    assert duplicate.returncode != 0 and 'duplicate key' in duplicate.stderr
    query("UPDATE scheduled_test_plans SET pelican_config='{\"quality\":{\"action\":\"remove_groups\"}}' WHERE id=2;")
    assert query('SELECT count(*) FROM scheduled_test_plans').stdout.strip() == '3'
    print(f'PASS: lock timeout preserves old index; migration preserves data; scopes coexist; duplicates rejected; old writes compatible ({time.monotonic()-started:.2f}s).')
finally:
    subprocess.run(DOCKER + ['rm', '-f', NAME], stdout=subprocess.DEVNULL, check=False)
