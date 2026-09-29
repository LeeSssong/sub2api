"""Exercise the official index migration on disposable PostgreSQL, never production."""
import os
import pathlib
import subprocess
import time
import uuid

ROOT = pathlib.Path(__file__).resolve().parents[2]
DOCKER = ['docker', '--context', os.environ.get('TEST_DOCKER_CONTEXT', 'colima')]
NAME = 'sub2api-sep30-migration-' + uuid.uuid4().hex[:8]
SQL = '\n'.join((ROOT / 'upstream/sub2api/backend/migrations' / f).read_text() for f in ['259_account_auto_config_events.sql','260_quality_rule_templates.sql','261_pelican_group_test_costs.sql'])

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
CREATE TABLE accounts(id bigint PRIMARY KEY);
CREATE TABLE groups(id bigint PRIMARY KEY);
CREATE TABLE scheduled_test_plans(id bigint PRIMARY KEY);
CREATE TABLE pelican_group_test_results(id bigint PRIMARY KEY, group_id bigint, result jsonb);
INSERT INTO accounts VALUES(1); INSERT INTO groups VALUES(1); INSERT INTO scheduled_test_plans VALUES(1);
INSERT INTO pelican_group_test_results VALUES(1,1,'{"ok":true}');
""")
    before = query('SELECT id,group_id,result FROM pelican_group_test_results ORDER BY id').stdout
    locker = subprocess.Popen(DOCKER + ['exec', '-i', NAME, 'psql', '-X', '-U', 'postgres', '-At'], stdin=subprocess.PIPE, stdout=subprocess.PIPE, text=True)
    locker.stdin.write('BEGIN; LOCK pelican_group_test_results IN ACCESS SHARE MODE; SELECT 123;\n'); locker.stdin.flush()
    while locker.stdout.readline().strip() != '123':
        if locker.poll() is not None: raise RuntimeError('lock holder failed')
    blocked=query("BEGIN; SET LOCAL lock_timeout='100ms'; SET LOCAL statement_timeout='2s';\n"+SQL+'\nCOMMIT;',check=False)
    assert blocked.returncode != 0 and 'lock timeout' in blocked.stderr,blocked.stderr
    locker.stdin.write('ROLLBACK;\n');locker.stdin.close();assert locker.wait(timeout=5)==0
    assert query("SELECT to_regclass('account_auto_config_events') IS NULL").stdout.strip()=='t'
    started=time.monotonic()
    query("BEGIN; SET LOCAL lock_timeout='100ms'; SET LOCAL statement_timeout='2s';\n"+SQL+'\nCOMMIT;')
    assert query('SELECT id,group_id,result FROM pelican_group_test_results ORDER BY id').stdout==before
    assert query('SELECT cost_usd IS NULL AND NOT cost_incomplete FROM pelican_group_test_results WHERE id=1').stdout.strip()=='t'
    query("INSERT INTO pelican_group_test_results(id,group_id,result) VALUES(2,1,'{}'); UPDATE pelican_group_test_results SET result='{}' WHERE id=1;")
    query("INSERT INTO quality_rule_templates(id,cron_expression,pelican_config) VALUES(1,'* * * * *','{}'); INSERT INTO quality_rule_template_accounts(template_id,account_id,plan_id) VALUES(1,1,1); DELETE FROM scheduled_test_plans WHERE id=1;")
    assert query('SELECT plan_id IS NULL FROM quality_rule_template_accounts').stdout.strip()=='t'
    query("INSERT INTO pelican_group_test_daily_costs VALUES(1,CURRENT_DATE,0.25,1); DELETE FROM pelican_group_test_results;")
    assert query('SELECT cost_usd=0.25 FROM pelican_group_test_daily_costs').stdout.strip()=='t'
    print(f'PASS: bounded lock failure rolls back; additive migration preserves legacy data and writes; template FK and durable costs work ({time.monotonic()-started:.2f}s).')

finally:
    subprocess.run(DOCKER + ['rm', '-f', NAME], stdout=subprocess.DEVNULL, check=False)
