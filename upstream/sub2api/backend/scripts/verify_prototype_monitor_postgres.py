#!/usr/bin/env python3
"""Run real monitor SQL against a disposable, network-isolated PostgreSQL 16.
No published ports, host volumes, site containers, credentials, or upstream calls.
Extracts production query text so this validates the candidate SQL, not a copy.
"""
import os
import json
import pathlib
import re
import subprocess
import time
import uuid

ROOT = pathlib.Path(__file__).resolve().parents[1]
REPORT = pathlib.Path(os.environ.get('MONITOR_SQL_REPORT', ROOT / 'scripts' / 'prototype-monitor-postgres-results.json'))
NAME = 'codex-parity-pg-' + uuid.uuid4().hex[:10]
results = {'database': os.environ.get('MONITOR_SQL_IMAGE', 'postgres:16-alpine'), 'container': NAME, 'network': 'none', 'published_ports': False, 'checks': []}


def command(args, text=None):
    return subprocess.run(args, input=text, text=True, capture_output=True, check=True).stdout.strip()


def sql(text):
    return command(['docker', 'exec', '-i', NAME, 'psql', '-U', 'postgres', '-d', 'postgres', '-qAt', '-v', 'ON_ERROR_STOP=1'], text)


def query(path, marker):
    source = (ROOT / path).read_text()
    if path.endswith('account_monitor_repo.go'):
        shared = (ROOT / 'internal/repository/monitor_v4_timeline.go').read_text().split('const monitorV4RealEventsSQL = `', 1)[1].split('`', 1)[0]
        source = source.replace('`+monitorV4RealEventsSQL+`', shared).replace('` + monitorV4RealEventsSQL + `', shared)
    pos = source.index(marker)
    return source[pos:source.index('`', pos)]


def bind(text, args):
    def literal(value):
        if value is None:
            return 'NULL'
        if isinstance(value, bool):
            return 'TRUE' if value else 'FALSE'
        if isinstance(value, (int, float)):
            return str(value)
        if isinstance(value, list):
            return 'ARRAY[' + ','.join(literal(v) for v in value) + ']'
        return "'" + value.replace("'", "''") + "'"
    return re.sub(r'\$(\d+)\b', lambda match: literal(args[int(match[1])-1]), text)


def rows(text):
    raw = sql('SELECT row_to_json(result) FROM (' + text + ') result;')
    return [json.loads(line) for line in raw.splitlines() if line]


def check(name, condition, details=None):
    if not condition:
        raise AssertionError(name + ': ' + repr(details))
    results['checks'].append({'name': name, 'passed': True, 'details': details})


created = False
try:
    results['image_id'] = command(['docker', 'image', 'inspect', results['database'], '--format', '{{.Id}}'])
    command(['docker', 'run', '--detach', '--name', NAME, '--network', 'none', '--env', 'POSTGRES_HOST_AUTH_METHOD=trust', results['database']])
    created = True
    for attempt in range(80):
        ready = subprocess.run(['docker', 'exec', NAME, 'pg_isready', '-h', '127.0.0.1', '-U', 'postgres'], capture_output=True)
        if ready.returncode == 0:
            break
        time.sleep(0.25)
    else:
        raise RuntimeError('isolated PostgreSQL did not become ready')
    isolation = json.loads(command(['docker', 'inspect', NAME]))[0]
    check('container has no network or published ports', isolation['HostConfig']['NetworkMode'] == 'none' and not isolation['HostConfig']['PortBindings'])
    results['server_version'] = sql('SELECT version();')
    sql('''
CREATE TABLE groups(id bigint PRIMARY KEY, deleted_at timestamptz);
INSERT INTO groups(id) VALUES(7),(8);
CREATE TABLE usage_logs(id bigserial PRIMARY KEY,group_id bigint,account_id bigint,created_at timestamptz,
 first_token_ms double precision,duration_ms double precision,input_tokens bigint,cache_creation_tokens bigint,
 cache_read_tokens bigint,logical_request_id text,request_id text,usage_completeness text);
CREATE TABLE ops_error_logs(id bigserial PRIMARY KEY,group_id bigint,account_id bigint,created_at timestamptz,
 status_code int,is_count_tokens boolean,is_business_limited boolean DEFAULT FALSE,request_id text,client_request_id text,error_owner text,error_phase text,
 error_source text,error_type text,error_message text,error_body text,upstream_error_message text,upstream_error_detail text);
''')
    for migration in ['187_account_monitor.sql', '230_account_monitor_bucket_terminals.sql', '232_monitor_v4_snapshots.sql', '233_monitor_v4_windows_1h.sql', '235_account_monitor_usage.sql']:
        sql((ROOT/'migrations'/migration).read_text())
    sql('''INSERT INTO account_monitor_v4_snapshots("window",group_id,snapshot_id,generated_at,window_start,window_end,contract_version,current_operational)
 VALUES('1h',7,'7d4b56d2-8223-4f77-8d22-f6a93d818980','2026-09-20T12:00:00Z','2026-09-20T11:00:00Z','2026-09-20T12:00:00Z','2',TRUE);''')
    for migration in ['238_monitor_v4_p50.sql', '239_group_tool_mappings.sql', '240_manual_probe_group_scope.sql', '265_monitor_v4_legacy_default.sql']:
        sql('BEGIN; ' + (ROOT/'migrations'/migration).read_text() + ' COMMIT;')
        check('migration executes: '+migration, True)
    check('legacy operational flag is invalidated and P50 starts unknown', sql('SELECT (NOT current_operational AND ttft_p50_ms IS NULL AND latency_p50_ms IS NULL) FROM account_monitor_v4_snapshots;') == 't')
    check('destructive monitor retirement remains deferred', sql("SELECT COUNT(*) FROM information_schema.columns WHERE table_name='account_monitor_v4_snapshots' AND column_name='current_operational';") == '1')
    check('new snapshot defaults preserve the legacy reader contract', sql("SELECT column_default FROM information_schema.columns WHERE table_name='account_monitor_v4_snapshots' AND column_name='current_operational';") == 'false')
    sql('''INSERT INTO usage_logs(group_id,account_id,created_at,first_token_ms,duration_ms,input_tokens,cache_creation_tokens,cache_read_tokens,request_id,usage_completeness)
 VALUES(7,12,'2026-09-20T11:55:00Z',100,1000,10,0,0,'real-1','complete'),
 (7,12,'2026-09-20T11:56:00Z',200,3000,10,0,0,'real-2','complete'),
 (7,12,'2026-09-20T11:57:00Z',1000,9000,10,0,0,'real-3','complete');''')
    manual_insert = query('internal/repository/account_monitor_repo.go', 'INSERT INTO account_monitor_results (\n run_id, account_id')
    sql(bind(manual_insert, ['7d4b56d2-8223-4f77-8d22-f6a93d818981',12,'test-model','success','',None,400,5000,10,0,0,'complete','2026-09-20T11:59:00Z',7]))
    projection = query('internal/repository/account_monitor_repo.go', 'WITH scopes AS (\n  SELECT group_id, account_id')
    projection_args = ['2026-09-20T11:00:00Z','2026-09-20T12:00:00Z','5 minutes',[7,8],[12,12],[7,8]]
    projected = {row['group_id']: row for row in rows(bind(projection, projection_args))}
    check('real P50 interpolates four native selected samples', projected[7]['ttft_p50_ms'] == 300 and projected[7]['latency_p50_ms'] == 4000, {'ttft_p50_ms':projected[7]['ttft_p50_ms'],'latency_p50_ms':projected[7]['latency_p50_ms']})
    for window, start in [('24h','2026-09-19T12:00:00Z'),('7d','2026-09-13T12:00:00Z')]:
        window_args = [start] + projection_args[1:]
        window_row = rows(bind(projection, window_args))[0]
        check('real P50 window '+window, window_row['ttft_p50_ms']==300 and window_row['latency_p50_ms']==4000)
    check('manual group probe does not contaminate another group sharing account', projected[7]['request_count'] == 4 and projected[8]['request_count'] == 0 and projected[8]['ttft_p50_ms'] is None)
    check('projection excludes the removed flag', all('current_operational' not in row for row in projected.values()))
    check('real request counts exclude probes', projected[7]['real_request_count'] == 3 and projected[7]['real_success_count'] == 3 and projected[8]['real_request_count'] == 0)
    sql("UPDATE account_monitor_results SET ttft_ms=400,checked_at='2026-09-20T11:50:00Z' WHERE group_id=7; UPDATE usage_logs SET created_at='2026-09-20T11:50:00Z';")
    check('hourly counts retain requests older than five minutes', rows(bind(projection,projection_args))[0]['real_request_count'] == 3)
    sql("UPDATE account_monitor_results SET checked_at='2026-09-20T11:59:00Z' WHERE group_id=7;")
    source = (ROOT/'internal/repository/account_monitor_repo.go').read_text()
    v2_start = source.index('WITH scopes AS (', source.index('func (r *accountMonitorRepository) ProjectMonitorV2Groups'))
    v2 = source[v2_start:source.index('`',v2_start)]
    v2_rows = rows(bind(v2,['2026-09-20T11:00:00Z','2026-09-20T12:00:00Z','2026-09-20T11:55:00Z',[7,8],[12,12],'5 minutes']))
    v2_by_group = {row['group_id']:row for row in v2_rows}
    check('V2 latest status also respects manual group scope', v2_by_group[7]['current_status']=='operational' and v2_by_group[8]['current_status']=='unavailable')
    shared_source=(ROOT/'internal/repository/monitor_v4_timeline.go').read_text()
    sla_predicate=(ROOT/'internal/repository/ops_sla_sql.go').read_text().split('const opsSLAErrorPredicate = `',1)[1].split('`',1)[0]
    shared_source=re.sub(r'`\s*\+\s*opsSLAErrorPredicate\s*\+\s*`',lambda _:sla_predicate,shared_source)
    cte=shared_source.split('const monitorV4RealEventsSQL = `',1)[1].split('`',1)[0]
    tail=shared_source.split('monitorV4RealEventsSQL+`',1)[-1] if 'monitorV4RealEventsSQL+`' in shared_source else shared_source.split('monitorV4RealEventsSQL + `',1)[1]
    timeline_sql=cte+tail.split('`',1)[0]
    timeline=rows(bind(timeline_sql,['2026-09-20T11:00:00Z','2026-09-20T12:00:00Z','5 minutes',[],[],[7,8]]))
    check('timeline returns every group and empty time bucket',len(timeline)==24)
    sla=rows(bind('''SELECT g.id AS group_id,
 (SELECT COUNT(*) FROM usage_logs u WHERE u.group_id=g.id AND u.created_at >= $1::timestamptz AND u.created_at < $2::timestamptz) AS success_count,
 (SELECT COUNT(*) FROM ops_error_logs o WHERE o.group_id=g.id AND o.created_at >= $1::timestamptz AND o.created_at < $2::timestamptz AND o.is_count_tokens=FALSE AND '''+sla_predicate+''') AS failure_count
 FROM groups g''',['2026-09-20T11:00:00Z','2026-09-20T12:00:00Z']))
    check('timeline counts equal raw admin SLA counts',all(sum(p['request_count'] for p in timeline if p['group_id']==g['group_id'])==g['success_count']+g['failure_count'] and sum(p['success_count'] for p in timeline if p['group_id']==g['group_id'])==g['success_count'] for g in sla))
    populated = next(p for p in timeline if p['group_id']==7 and p['request_count']==3)
    check('timeline P50 is median of successful real requests', populated['ttft_p50_ms']==200)
    check('timeline zero cache hits is zero, empty bucket is null', populated['cache_hit_rate']==0 and all(p['cache_hit_rate'] is None and p['ttft_p50_ms'] is None for p in timeline if p['request_count']==0))
    sql("UPDATE usage_logs SET cache_read_tokens=30,cache_creation_tokens=10 WHERE request_id='real-1';")
    weighted=rows(bind(timeline_sql,['2026-09-20T11:00:00Z','2026-09-20T12:00:00Z','5 minutes',[],[],[7]]))
    weighted_bucket=next(p for p in weighted if p['request_count']==3)
    check('timeline cache rate uses weighted tokens including cache creation', abs(weighted_bucket['cache_hit_rate']-30/70)<1e-9)
    sql("UPDATE usage_logs SET cache_read_tokens=0,cache_creation_tokens=0 WHERE request_id='real-1';")
    for interval, hours, count in [('1 hour',24,24),('1 hour',168,168),('24 hours',168,7),('24 hours',24,1)]:
        end=__import__("datetime").datetime(2026,9,20,12,tzinfo=__import__("datetime").timezone.utc)
        start=end-__import__("datetime").timedelta(hours=hours)
        granulated=rows(bind(timeline_sql,[start.isoformat(),end.isoformat(),interval,[],[],[7]]))
        check('timeline granularity '+str(hours)+'h/'+interval,len(granulated)==count and sum(p['request_count'] for p in granulated)==3)
    mapping = query('internal/repository/group_tool_mapping.go','WITH changed AS (')
    sql(bind(mapping,[7,json.dumps(['codex']),42]))
    sql(bind(mapping,[7,json.dumps(['claude','codex']),42]))
    saved_mapping = rows('SELECT * FROM group_tool_mappings WHERE group_id=7')[0]
    check('mapping update increments version and preserves explicit tool IDs',saved_mapping['version']==2 and saved_mapping['tool_ids']==['claude','codex'])
    check('mapping audit records both versions with actor',sql('SELECT COUNT(*)=2 AND MIN(version)=1 AND MAX(version)=2 AND BOOL_AND(updated_by=42) FROM group_tool_mapping_audit WHERE group_id=7;')=='t')
    check('unmapped group has no inferred tool mapping',sql('SELECT COUNT(*) FROM group_tool_mappings WHERE group_id=8;')=='0')
    insert_snapshot=query('internal/repository/monitor_v4_snapshot_repo.go','INSERT INTO account_monitor_v4_snapshots (')
    sql('DELETE FROM account_monitor_v4_snapshots;')
    sql(bind(insert_snapshot,['1h',7,'7d4b56d2-8223-4f77-8d22-f6a93d818980','2026-09-20T12:00:00Z','2026-09-20T11:00:00Z','2026-09-20T12:00:00Z','2',100,4,4,3,3,1,1,0,425,4,4500,4,0,'2026-09-20T11:59:00Z',300,4000]))
    load_snapshot=query('internal/repository/monitor_v4_snapshot_repo.go','SELECT "window", group_id, snapshot_id')
    restored=rows(bind(load_snapshot,['1h']))[0]
    check('production snapshot INSERT and SELECT preserve P50 separately',restored['ttft_p50_ms']==300 and restored['latency_p50_ms']==4000 and restored['ttft_p95_ms']==425)
    check('snapshot preserves real request counts without operational flag', restored['real_request_count'] == 3 and restored['real_success_count'] == 3 and 'current_operational' not in restored)
    check('rollback legacy reader keeps snapshot counts and compatibility default', sql('SELECT real_request_count=3 AND real_success_count=3 AND NOT current_operational FROM account_monitor_v4_snapshots;') == 't')
    results['passed']=True
except Exception as exc:
    results['passed']=False
    results['error']=str(exc)
    if isinstance(exc,subprocess.CalledProcessError):
        results['sql_error']=exc.stderr
    raise
finally:
    if created:
        command(['docker','rm','--force','--volumes',NAME])
        results['container_removed']=True
    REPORT.write_text(json.dumps(results,ensure_ascii=False,indent=2)+'\n')
    print(json.dumps({'passed':results.get('passed'), 'checks':len(results['checks']), 'container_removed':results.get('container_removed'), 'report':str(REPORT)},ensure_ascii=False))
