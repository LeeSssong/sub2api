"""Exercise official SQL and full archive recovery on disposable PostgreSQL."""
import pathlib
import subprocess
import time
import uuid

ROOT = pathlib.Path(__file__).resolve().parents[2]
DOCKER = ['docker', '--context', 'colima']
NAME = 'sub2api-v297-migration-' + uuid.uuid4().hex[:8]


def query(sql, check=True):
    return subprocess.run(DOCKER + ['exec', '-i', NAME, 'psql', '-X', '-U', 'postgres', '-v', 'ON_ERROR_STOP=1', '-At'], input=sql, text=True, capture_output=True, check=check)


subprocess.run(DOCKER + ['run', '--rm', '-d', '--name', NAME, '--network', 'none', '--tmpfs', '/var/lib/postgresql:size=256m', '-e', 'POSTGRES_HOST_AUTH_METHOD=trust', 'postgres:18-alpine'], check=True, stdout=subprocess.DEVNULL)
try:
    deadline = time.monotonic() + 60
    while b'PostgreSQL init process complete' not in subprocess.check_output(DOCKER + ['logs', NAME], stderr=subprocess.DEVNULL) or query('SELECT 1', False).returncode:
        if time.monotonic() > deadline:
            raise RuntimeError('PostgreSQL not ready')
        time.sleep(.2)
    query("CREATE TABLE payment_orders(id int PRIMARY KEY, amount numeric(20,2)); INSERT INTO payment_orders VALUES(1,100); CREATE TABLE user_platform_quotas(platform text); CREATE TABLE composite_model_routes(target_platform text); INSERT INTO user_platform_quotas VALUES('openai'); INSERT INTO composite_model_routes VALUES('anthropic');")
    archive = subprocess.check_output(DOCKER + ['exec', NAME, 'pg_dump', '-U', 'postgres', '-d', 'postgres', '-Fc'])
    sql = '\n'.join((ROOT / 'upstream/sub2api/backend/migrations' / p).read_text() for p in ['241_add_payment_order_bonus_amount.sql', '241_add_typesafe_platform.sql'])
    locker = subprocess.Popen(DOCKER + ['exec', '-i', NAME, 'psql', '-X', '-U', 'postgres', '-At'], stdin=subprocess.PIPE, stdout=subprocess.PIPE, text=True)
    locker.stdin.write('BEGIN; LOCK payment_orders IN ACCESS SHARE MODE; SELECT 123;\n')
    locker.stdin.flush()
    while locker.stdout.readline().strip() != '123':
        if locker.poll() is not None:
            raise RuntimeError('lock holder failed')
    bounded = "BEGIN; SET LOCAL lock_timeout='100ms'; SET LOCAL statement_timeout='2s';\n" + sql + '\nCOMMIT;'
    blocked = query(bounded, False)
    assert blocked.returncode != 0 and 'lock timeout' in blocked.stderr, blocked.stderr
    locker.communicate('ROLLBACK;\n', timeout=5)
    assert query("SELECT count(*) FROM information_schema.columns WHERE table_name='payment_orders' AND column_name='bonus_amount'").stdout.strip() == '0'
    query(bounded)
    assert query('SELECT bonus_amount FROM payment_orders WHERE id=1').stdout.strip() == '0.00'
    query("INSERT INTO user_platform_quotas VALUES('typesafe'); INSERT INTO composite_model_routes VALUES('typesafe');")
    query("INSERT INTO payment_orders(id,amount) VALUES(2,50); INSERT INTO user_platform_quotas VALUES('openai');")
    assert query('SELECT bonus_amount FROM payment_orders WHERE id=2').stdout.strip() == '0.00'
    assert query("INSERT INTO user_platform_quotas VALUES('invalid')", False).returncode != 0
    query('BEGIN;\n' + sql + '\nCOMMIT;')
    subprocess.run(DOCKER + ['exec', '-i', NAME, 'pg_restore', '--clean', '--if-exists', '--exit-on-error', '-U', 'postgres', '-d', 'postgres'], input=archive, check=True, capture_output=True)
    assert query('SELECT amount FROM payment_orders WHERE id=1').stdout.strip() == '100.00'
    assert query("SELECT count(*) FROM information_schema.columns WHERE table_name='payment_orders' AND column_name='bonus_amount'").stdout.strip() == '0'
    assert query('SELECT count(*) FROM user_platform_quotas').stdout.strip() == '1'
    print('PASS: bounded lock failure, legacy writes, defaults, TypeSafe constraints, reruns and archive recovery')
finally:
    subprocess.run(DOCKER + ['rm', '-f', NAME], check=True, stdout=subprocess.DEVNULL)
