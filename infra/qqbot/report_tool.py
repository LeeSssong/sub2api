#!/usr/bin/env python3
"""Read/report diagnostics. This CLI has no real QQ sender or detector client."""
from __future__ import annotations

import argparse
import asyncio
import copy
import json
import os
import shutil
import sys
from datetime import datetime, timezone
from pathlib import Path

PLUGIN = Path(__file__).resolve().parent/'plugins/astrbot_plugin_xingqiao_ops'
sys.path.insert(0, str(PLUGIN))
from pelican_client import MAX_RESPONSE_BYTES, ReadOnlyReportClient
from pelican_delivery import ReportConfig, ReportDelivery, load_report_config, write_report_cache
from pelican_renderer import build_report_html, render_report_png
from pelican_source import fetch_visible_groups, is_allowed_source_path, load_report_snapshot
from pelican_report_source import load_detection_report_snapshot, report_path


def fixture_source(path):
    raw = path.read_bytes()
    if len(raw) > MAX_RESPONSE_BYTES:
        raise ValueError('fixture too large')
    data = json.loads(raw)
    calls = []
    if isinstance(data.get('report'), dict) and data['report'].get('schema_version') == 2:
        report = data['report']
        def get_v2(route):
            if not is_allowed_source_path(route):
                raise ValueError('fixture route is not read-only')
            calls.append(('GET', route))
            if route == '/api/v1/admin/pelican-showcase':
                group = report['group']
                return {'enabled':True,'groups':[{k:group[k] for k in ('id','name','platform')}]}
            if not route.startswith('/api/v1/admin/pelican-reports/groups/'):
                raise ValueError('fixture has no report for this route')
            return copy.deepcopy(report)
        get_v2.report_schema_version = 2
        clock = datetime.fromisoformat(report['as_of'].replace('Z', '+00:00'))
        if clock.tzinfo is None:
            raise ValueError('fixture requires a timezone-aware timestamp')
        return get_v2, clock, calls
    def get(route):
        if not is_allowed_source_path(route):
            raise ValueError('fixture route is not read-only')
        calls.append(('GET', route))
        if route == '/api/v1/admin/pelican-showcase':
            return copy.deepcopy(data['listing'])
        return copy.deepcopy(data['items'][route.rsplit('/', 1)[1]])
    value = data['listing']['stats_window']['to']
    clock = datetime.fromisoformat(value.replace('Z', '+00:00'))
    if clock.tzinfo is None:
        raise ValueError('fixture requires a timezone-aware timestamp')
    get.report_schema_version = 1
    return get, clock, calls


def read_snapshot(get, group_id, now, model_id=None):
    # The legacy reader is available only for explicitly supplied old fixtures.
    # Live reads always use v2 and never fall back to gallery summaries.
    if getattr(get, 'report_schema_version', 2) == 1:
        if model_id is not None:
            raise ValueError('legacy fixture has no model-filtered report')
        return load_report_snapshot(get, group_id, now=now)
    snapshot = load_detection_report_snapshot(get, group_id, now=now, model_id=model_id)
    if getattr(get, 'report_schema_version', None) == 2:
        snapshot['offline_fixture'] = True
    return snapshot


def protected_admin_key():
    key = os.environ.get('XINGQIAO_ADMIN_API_KEY', '').strip()
    if key:
        return key
    for path in (Path('/AstrBot/data/xingqiao_login.env'), Path('/opt/qqbot/secrets.env'), Path('/opt/qqbot/.env')):
        try:
            lines = path.read_text().splitlines()
        except OSError:
            continue
        for line in lines:
            if line.strip().startswith('XINGQIAO_ADMIN_API_KEY='):
                return line.split('=', 1)[1].strip().strip(chr(34)).strip(chr(39))
    raise ValueError('protected report key unavailable')


def source_for(args):
    if args.fixture:
        return fixture_source(args.fixture)
    if not args.live_read:
        raise ValueError('explicit source required')
    return ReadOnlyReportClient(protected_admin_key()), datetime.now(timezone.utc), []


async def offline_self_test(args):
    get, now, requests = fixture_source(args.fixture)
    directory = args.output_dir.resolve()
    directory.mkdir(parents=True, exist_ok=True)
    # Fresh state makes rerunning the diagnostic an actual render, while the
    # second tick below proves persistent deduplication in the same diagnostic.
    import tempfile
    renders, sender_calls = [], []
    async def load(group_id):
        return read_snapshot(get, group_id, now=now)
    async def render(snapshot):
        renders.append(1)
        return await render_report_png(snapshot, timeout_seconds=args.timeout, executable_path=args.chromium)
    async def forbidden_sender(*unused):
        sender_calls.append(1)
        raise AssertionError('offline diagnostics have no sender')
    beijing_slot = now.astimezone(__import__('zoneinfo').ZoneInfo('Asia/Shanghai')).strftime('%H:%M')
    cfg = ReportConfig.from_dict({'enabled': True, 'dry_run': True, 'times': [beijing_slot],
        'targets': [{'session': 'offline_fixture:GroupMessage:10001', 'group_ids': [args.group]}]})
    with tempfile.TemporaryDirectory(prefix='pelican-selftest-', dir=directory) as temp:
        state = Path(temp)
        first = ReportDelivery(cfg, state, load, render, forbidden_sender)
        initial = await first.tick(now)
        second = ReportDelivery(cfg, state, load, render, forbidden_sender)
        repeat = await second.tick(now)
        files = list((state/'cache').glob('*.png'))
        if sender_calls or initial['sent'] or repeat['sent'] or len(files) != 1 or len(renders) != 1:
            raise AssertionError('offline delivery invariant failed')
        if not second.store.rows() or any(row['state'] != 'dry_run' for row in second.store.rows()):
            raise AssertionError('offline outbox state invariant failed')
        output = directory/'report.png'
        shutil.copyfile(files[0], output)
        summary = {'mode': 'offline-dry-run', 'source_gets': len(requests),
                   'renderer_calls': len(renders), 'real_qq_sends': len(sender_calls),
                   'detection_requests': 0, 'restart_deduplication': 'passed',
                   'output': str(output)}
        (directory/'selftest-summary.json').write_text(json.dumps(summary, ensure_ascii=False, indent=2))
        return summary


def parser():
    result = argparse.ArgumentParser(description=__doc__)
    commands = result.add_subparsers(dest='command', required=True)
    validate = commands.add_parser('validate-config')
    validate.add_argument('path', type=Path)
    for name in ('render', 'groups'):
        cmd = commands.add_parser(name)
        source = cmd.add_mutually_exclusive_group(required=True)
        source.add_argument('--fixture', type=Path)
        source.add_argument('--live-read', action='store_true')
        if name == 'render':
            cmd.add_argument('--group', type=int, required=True)
            cmd.add_argument('--model')
            cmd.add_argument('--output', type=Path, required=True)
            cmd.add_argument('--html-only', action='store_true')
            cmd.add_argument('--chromium')
            cmd.add_argument('--timeout', type=float, default=60)
    test = commands.add_parser('self-test', help='fixture-only end-to-end test; no QQ sender exists')
    test.add_argument('--fixture', type=Path, required=True)
    test.add_argument('--group', type=int, required=True)
    test.add_argument('--output-dir', type=Path, required=True)
    test.add_argument('--chromium')
    test.add_argument('--timeout', type=float, default=60)
    return result


def main(argv=None):
    args = parser().parse_args(argv)
    try:
        if args.command == 'validate-config':
            cfg = load_report_config(args.path)
            result = {'enabled': cfg.enabled, 'dry_run': cfg.dry_run,
                      'target_count': len(cfg.targets), 'times': list(cfg.times)}
        elif args.command == 'self-test':
            result = asyncio.run(offline_self_test(args))
        else:
            get, now, _ = source_for(args)
            if args.command == 'groups':
                result = {'groups': fetch_visible_groups(get)}
            else:
                snapshot = read_snapshot(get, args.group, now=now, model_id=args.model)
                args.output.parent.mkdir(parents=True, exist_ok=True)
                if args.html_only:
                    args.output.write_text(build_report_html(snapshot), encoding='utf-8')
                else:
                    image = asyncio.run(render_report_png(snapshot, timeout_seconds=args.timeout,
                                                          executable_path=args.chromium))
                    write_report_cache(args.output, image)
                result = {'output': str(args.output.resolve()), 'mode': 'render-only-no-send'}
        print(json.dumps(result, ensure_ascii=False))
        return 0
    except Exception as exc:
        print('Report diagnostic failed: '+type(exc).__name__+'. No QQ message was sent.', file=sys.stderr)
        return 2


if __name__ == '__main__':
    raise SystemExit(main())
