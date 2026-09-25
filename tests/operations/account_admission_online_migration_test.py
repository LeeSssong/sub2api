#!/usr/bin/env python3
"""Exercise admission SQL on disposable PostgreSQL without compiling the app.

Run: python3 tests/operations/account_admission_online_migration_test.py
An existing disposable container may be reused with --container NAME.
"""
import argparse
import json
from pathlib import Path
import re
import subprocess
import time
import uuid


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--container')
    args = parser.parse_args()
    owned = args.container is None
    name = args.container or 'sub2api-admission-timeout-' + uuid.uuid4().hex[:10]
    if not name.startswith('sub2api-admission-timeout-'):
        parser.error('reuse requires an explicitly disposable admission test container')
    database = 'admission_' + uuid.uuid4().hex[:12]
    root = Path(__file__).resolve().parents[2]
    migrations = root / 'upstream/sub2api/backend/migrations'
    admission = (migrations / '255_account_admission.sql').read_text()
    statistics = (migrations / '255_pelican_drawing_statistics.sql').read_text()

    def command(db=database, app=None):
        cmd = ['docker', 'exec', '-i']
        if app:
            cmd += ['-e', 'PGAPPNAME=' + app]
        return cmd + [name, 'psql', '-XAt', '-U', 'postgres', '-d', db,
                      '-v', 'ON_ERROR_STOP=1', '-v', 'VERBOSITY=verbose']

    def sql(text, check=True, db=database):
        result = subprocess.run(command(db), input=text, text=True, capture_output=True, timeout=30)
        if check and result.returncode:
            raise AssertionError(result.stderr)
        return result

    created = False
    try:
        if owned:
            subprocess.run(['docker', 'run', '--rm', '-d', '--name', name,
                            '--label', 'codex.test=account-admission-migration',
                            '-e', 'POSTGRES_PASSWORD=local-migration-test',
                            'postgres:18.1-alpine3.23'], check=True, capture_output=True, timeout=120)
        deadline = time.monotonic() + 60
        while subprocess.run(['docker', 'exec', name, 'pg_isready', '-U', 'postgres'],
                             capture_output=True, timeout=15).returncode:
            if time.monotonic() >= deadline:
                raise AssertionError('disposable PostgreSQL did not become ready')
            time.sleep(.1)
        sql('CREATE DATABASE ' + database, db='postgres')
        created = True
        sql("""CREATE TABLE groups (id BIGINT PRIMARY KEY);
        CREATE TABLE accounts (id BIGINT PRIMARY KEY, credentials JSONB NOT NULL DEFAULT '{}',
          platform TEXT NOT NULL, type TEXT NOT NULL, proxy_id BIGINT,
          schedulable BOOLEAN NOT NULL DEFAULT true, extra JSONB NOT NULL DEFAULT '{}');
        CREATE TABLE account_groups (account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
          group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE, priority INT NOT NULL DEFAULT 50,
          allowed_models JSONB, PRIMARY KEY(account_id,group_id));
        INSERT INTO groups VALUES (1),(2);
        INSERT INTO accounts(id,platform,type,credentials,extra)
          VALUES (1,'openai','oauth','{"access_token":"old","api_key":"stable"}','{"legacy":true}');
        INSERT INTO account_groups VALUES(1,1,50,NULL);
        CREATE TABLE old_column_contract AS SELECT table_name,column_name,data_type
          FROM information_schema.columns WHERE table_schema='public';
        """)
        for table, update in [('accounts', 'UPDATE accounts SET extra=extra WHERE id=1'),
                              ('account_groups', 'UPDATE account_groups SET priority=priority WHERE account_id=1')]:
            app = database + '_' + table
            locker = subprocess.Popen(command(app=app), stdin=subprocess.PIPE,
                                      stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, text=True)
            try:
                locker.stdin.write('BEGIN; ' + update + '; SELECT pg_sleep(60); COMMIT;')
                locker.stdin.close()
                deadline = time.monotonic() + 20
                while sql("SELECT count(*) FROM pg_stat_activity WHERE application_name='" + app +
                           "' AND wait_event='PgSleep'").stdout.strip() != '1':
                    if time.monotonic() >= deadline:
                        raise AssertionError('legacy writer did not acquire its locks')
                    time.sleep(.05)
                result = sql(chr(92) + 'timing on' + chr(10) +
                             "BEGIN; SET LOCAL statement_timeout='1500ms'; SET LOCAL lock_timeout='0';" +
                             admission + 'COMMIT;', check=False)
                assert result.returncode != 0 and '55P03' in result.stderr, result.stderr
                timings = re.findall(r'Time: ([0-9.]+) ms', result.stdout)
                assert timings and float(timings[-1]) < 1000, result.stdout
                residue = sql("""SELECT
                  (SELECT count(*) FROM pg_class WHERE relname IN ('account_admission_jobs','account_admission_due_idx')) +
                  (SELECT count(*) FROM pg_proc WHERE proname IN ('account_admission_credential_signature','guard_account_admission','pause_admission_on_membership_change')) +
                  (SELECT count(*) FROM pg_trigger WHERE tgname IN ('account_admission_gate','account_admission_membership'))""").stdout.strip()
                assert residue == '0', residue
                print(json.dumps({'table': table, 'sqlstate': '55P03', 'blocked_statement_ms': float(timings[-1]),
                                  'schema_residue': 0}), flush=True)
            finally:
                sql("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE application_name='" + app + "'")
                locker.wait(timeout=15)

        # Successful retries execute each exact file in its own migration transaction.
        sql('BEGIN; ' + admission + 'COMMIT; BEGIN; ' + statistics + 'COMMIT;')
        assert sql('SHOW lock_timeout; SHOW statement_timeout').stdout.strip() == '0' + chr(10) + '0'
        sql("""UPDATE accounts SET credentials=credentials || '{"access_token":"refreshed"}', schedulable=true WHERE id=1;
        UPDATE account_groups SET priority=30 WHERE account_id=1;
        INSERT INTO account_groups VALUES(1,2,40,NULL);
        DELETE FROM account_groups WHERE account_id=1 AND group_id=2;
        """)
        assert sql("SELECT schedulable AND extra->'legacy'='true'::jsonb AND NOT extra ? 'account_admission_blocked' FROM accounts WHERE id=1").stdout.strip() == 't'
        sql("""UPDATE accounts SET schedulable=false WHERE id=1;
        INSERT INTO account_admission_jobs(account_id,target_group_ids) VALUES(1,'{2}');
        UPDATE accounts SET schedulable=true,extra='{}' WHERE id=1;
        UPDATE accounts SET credentials=credentials || '{"access_token":"rotated"}' WHERE id=1;
        """)
        assert sql("SELECT NOT a.schedulable AND a.extra->'account_admission_blocked'='true'::jsonb AND j.generation=1 FROM accounts a JOIN account_admission_jobs j ON j.account_id=a.id WHERE a.id=1").stdout.strip() == 't'
        sql("UPDATE accounts SET credentials=credentials || '{\"api_key\":\"changed\"}' WHERE id=1; UPDATE account_groups SET priority=31 WHERE account_id=1")
        assert sql("SELECT NOT active AND state='paused' AND blocked AND generation=3 FROM account_admission_jobs WHERE account_id=1").stdout.strip() == 't'
        assert sql("SELECT count(*) FROM old_column_contract o WHERE NOT EXISTS (SELECT 1 FROM information_schema.columns c WHERE c.table_schema='public' AND c.table_name=o.table_name AND c.column_name=o.column_name AND c.data_type=o.data_type)").stdout.strip() == '0'
        assert sql('SELECT (SELECT count(*) FROM pelican_drawing_outcomes)+(SELECT count(*) FROM pelican_drawing_statistics_state)').stdout.strip() == '0'
        print('PASS: normal migration, local timeout reset, legacy writes, admission gate, refresh, membership pause, old columns, empty statistics', flush=True)
    finally:
        if created:
            sql('DROP DATABASE ' + database + ' WITH (FORCE)', db='postgres')
        if owned:
            subprocess.run(['docker', 'rm', '-f', name], check=True, capture_output=True, timeout=30)


if __name__ == '__main__':
    main()
