"""Verify migration 262 against disposable PostgreSQL; no production data."""
import os
import pathlib
import subprocess
import time
import uuid

ROOT = pathlib.Path(__file__).resolve().parents[2]
DOCKER = ['docker', '--context', os.environ.get('TEST_DOCKER_CONTEXT', 'colima')]
NAME = 'sub2api-oct01-migration-' + uuid.uuid4().hex[:8]
SQL = (ROOT / 'upstream/sub2api/backend/migrations/262_openai_oauth_reauth_engine.sql').read_text()


def query(sql, check=True):
    return subprocess.run(DOCKER + ['exec', '-i', NAME, 'psql', '-X', '-U', 'postgres', '-v', 'ON_ERROR_STOP=1', '-At'], input=sql, text=True, capture_output=True, check=check)


subprocess.run(DOCKER + ['run', '--rm', '-d', '--name', NAME, '--network', 'none', '--tmpfs', '/var/lib/postgresql:size=256m', '-e', 'POSTGRES_HOST_AUTH_METHOD=trust', 'postgres:18-alpine'], check=True, stdout=subprocess.DEVNULL)
locker = None
try:
    deadline = time.monotonic() + 60
    while query('SELECT 1', check=False).returncode:
        if time.monotonic() > deadline:
            raise RuntimeError('disposable PostgreSQL not ready')
        time.sleep(.2)
    query("CREATE TABLE openai_oauth_reauth_configs(account_id bigint PRIMARY KEY, login_email text NOT NULL); INSERT INTO openai_oauth_reauth_configs VALUES(1,'fixture@example.invalid');")
    locker = subprocess.Popen(DOCKER + ['exec', '-i', NAME, 'psql', '-X', '-U', 'postgres', '-At'], stdin=subprocess.PIPE, stdout=subprocess.PIPE, text=True)
    locker.stdin.write('BEGIN; LOCK openai_oauth_reauth_configs IN ACCESS SHARE MODE; SELECT 123;\n')
    locker.stdin.flush()
    while locker.stdout.readline().strip() != '123':
        if locker.poll() is not None:
            raise RuntimeError('lock holder failed')
    bounded = "BEGIN; SET LOCAL lock_timeout='100ms'; SET LOCAL statement_timeout='2s';\n" + SQL + '\nCOMMIT;'
    blocked = query(bounded, check=False)
    assert blocked.returncode != 0 and 'lock timeout' in blocked.stderr, blocked.stderr
    locker.communicate('ROLLBACK;\n', timeout=5)
    assert query("SELECT count(*) FROM information_schema.columns WHERE table_name='openai_oauth_reauth_configs' AND column_name='engine'").stdout.strip() == '0'
    query(bounded)
    assert query("SELECT engine FROM openai_oauth_reauth_configs WHERE account_id=1").stdout.strip() == 'local_worker'
    # Old readers and writers remain compatible after promotion or application rollback.
    query("INSERT INTO openai_oauth_reauth_configs(account_id,login_email) VALUES(2,'old@example.invalid'); UPDATE openai_oauth_reauth_configs SET login_email='retained@example.invalid' WHERE account_id=1;")
    query("INSERT INTO openai_oauth_reauth_configs VALUES(3,'new@example.invalid','session_studio');")
    assert query("UPDATE openai_oauth_reauth_configs SET engine='invalid' WHERE account_id=1", check=False).returncode != 0
    assert query("UPDATE openai_oauth_reauth_configs SET engine=NULL WHERE account_id=1", check=False).returncode != 0
    query(bounded)
    assert query("SELECT count(*) FROM openai_oauth_reauth_configs").stdout.strip() == '3'
    assert query("SELECT engine FROM openai_oauth_reauth_configs WHERE account_id=3").stdout.strip() == 'session_studio'
    print('PASS: bounded lock failure is atomic; existing data, legacy writes, new engine validation and reruns verified.')
finally:
    if locker is not None and locker.poll() is None:
        locker.communicate('ROLLBACK;\n', timeout=5)
    subprocess.run(DOCKER + ['rm', '-f', NAME], check=False, stdout=subprocess.DEVNULL)
