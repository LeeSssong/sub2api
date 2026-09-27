import ast
import asyncio
import importlib
import importlib.util
import json
import sys
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace

PLUGIN = Path(__file__).resolve().parents[1] / 'plugins/astrbot_plugin_xingqiao_ops'
sys.path.insert(0, str(PLUGIN))


class IntegrationTests(unittest.TestCase):
    def client(self):
        self.assertIsNotNone(importlib.util.find_spec('pelican_client'), 'Report client is missing')
        return importlib.import_module('pelican_client')

    def test_reader_uses_only_get_with_admin_key_and_unwraps_public_dto(self):
        client = self.client()
        seen = []
        class Response:
            def __enter__(self): return self
            def __exit__(self, *args): pass
            def read(self, n): return b'{"code":0,"data":{"enabled":true,"groups":[]}}'
        def open_request(request, timeout):
            seen.append(request)
            return Response()
        result = client.ReadOnlyReportClient('fixture-key', open_request=open_request)('/api/v1/admin/pelican-showcase')
        self.assertEqual(result, {'enabled': True, 'groups': []})
        self.assertEqual(seen[0].get_method(), 'GET')
        self.assertEqual(seen[0].full_url, 'https://api.xingqiaolab.top/api/v1/admin/pelican-showcase')
        self.assertEqual(dict(seen[0].header_items())['X-api-key'], 'fixture-key')
        self.assertIsNone(seen[0].data)

    def test_mutating_or_other_routes_are_rejected_before_http(self):
        client = self.client()
        calls = []
        def open_request(*args, **kw): calls.append(args)
        get = client.ReadOnlyReportClient('fixture-key', open_request=open_request)
        for path in ['/api/v1/admin/scheduled-test-plans/1/run', '/api/v1/auth/login',
                     '/api/v1/admin/pelican-test-results', 'https://elsewhere.invalid/',
                     '/api/v1/admin/pelican-showcase/items/1?token=secret']:
            with self.subTest(path=path), self.assertRaises(ValueError): get(path)
        self.assertEqual(calls, [])

    def test_v2_transport_accepts_full_history_and_remains_bounded(self):
        import io
        client = self.client()
        from test_pelican_report_source import fixture
        report = fixture()
        row = report['history']['pelican'][0]
        report['history']['pelican'] = [dict(row, result_id=i + 1000) for i in range(9999)]
        raw = json.dumps({'code': 0, 'data': report}).encode()
        self.assertGreater(len(raw), 2 * 1024 * 1024)
        get = client.ReadOnlyReportClient('fixture-key', open_request=lambda *a, **k: io.BytesIO(raw))
        result = get('/api/v1/admin/pelican-reports/groups/6?window=24h')
        self.assertEqual(len(result['history']['pelican']), 9999)
        get = client.ReadOnlyReportClient('fixture-key', open_request=lambda *a, **k:
            io.BytesIO(b' ' * (client.MAX_RESPONSE_BYTES + 1)))
        with self.assertRaises(client.PelicanSourceUnavailable):
            get('/api/v1/admin/pelican-reports/groups/6?window=24h')

    def test_missing_key_does_not_fall_back_to_login(self):
        client = self.client()
        calls = []
        def open_request(*args, **kw): calls.append(args)
        with self.assertRaises(ValueError):
            client.ReadOnlyReportClient('', open_request=open_request)('/api/v1/admin/pelican-showcase')
        self.assertEqual(calls, [])

    def test_source_error_body_is_not_exposed(self):
        client = self.client()
        def bad(*args, **kw): raise RuntimeError('secret-key-and-private-upstream')
        with self.assertRaises(Exception) as raised:
            client.ReadOnlyReportClient('fixture-key', open_request=bad)('/api/v1/admin/pelican-showcase')
        self.assertNotIn('secret-key', str(raised.exception))

    def method(self, name, extras):
        tree = ast.parse((PLUGIN / 'main.py').read_text())
        cls = next(n for n in tree.body if isinstance(n, ast.ClassDef) and n.name == 'Main')
        found = [n for n in cls.body if isinstance(n, (ast.FunctionDef, ast.AsyncFunctionDef)) and n.name == name]
        self.assertTrue(found, f'Missing plugin method {name}')
        method = found[0]
        method.decorator_list = []
        scope = dict(asyncio=asyncio, AstrMessageEvent=object, Path=Path)
        scope.update(extras)
        exec(compile(ast.Module(body=[method], type_ignores=[]), '<report-handler>', 'exec'), scope)
        return scope[name]

    def test_last_moment_dry_run_guard_prevents_platform_send(self):
        from pelican_delivery import ReportConfig, DefiniteSendFailure
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / 'config.json'
            path.write_text(json.dumps({'enabled': True, 'dry_run': True}))
            called = []
            async def send(*args): called.append(args); return True
            from pelican_delivery import load_report_config
            cfg = ReportConfig.from_dict({'enabled': True, 'dry_run': False})
            method = self.method('_send_report', {'load_report_config': load_report_config,
                'DefiniteSendFailure': DefiniteSendFailure})
            bot = SimpleNamespace(_report_config_path=path, _report_config=cfg,
                                  context=SimpleNamespace(send_message=send))
            with self.assertRaises(DefiniteSendFailure):
                asyncio.run(method(bot, 'fixture:GroupMessage:10001', b'png'))
            self.assertEqual(called, [])

    def test_manual_query_only_matches_explicit_report_requests(self):
        self.assertIsNotNone(importlib.util.find_spec('pelican_commands'), 'Report command parser is missing')
        from pelican_commands import parse_report_query
        for value in ['检测报告', '/鹈鹕报告', '@小星 检测报告']:
            self.assertEqual(parse_report_query(value), 0)
        self.assertEqual(parse_report_query('检测报告 7'), 7)
        for value in ['执行鹈鹕检测', '检测报告怎么做', '检测报告 -1', '检测报告 0']:
            self.assertIsNone(parse_report_query(value))

    def manual(self, extra):
        from pelican_delivery import load_report_config, content_digest, report_is_usable
        self.assertIsNotNone(importlib.util.find_spec('pelican_commands'), 'Report command parser is missing')
        from pelican_commands import parse_report_query
        scope = dict(load_report_config=load_report_config, parse_report_query=parse_report_query,
                     content_digest=content_digest, report_is_usable=report_is_usable,
                     MessageType=SimpleNamespace(GROUP_MESSAGE='group'),
                     message_ats_bot=lambda e: True, Image=SimpleNamespace(fromBytes=lambda b: b),
                     logger=SimpleNamespace(warning=lambda *a: None))
        scope.update(extra)
        return self.method('pelican_report_card', scope)

    def test_manual_dry_run_and_unsubscribed_group_have_no_message(self):
        with tempfile.TemporaryDirectory() as tmp:
            cfgpath = Path(tmp)/'config.json'
            calls = []
            async def render(*args): calls.append(args); return b'png'
            def load(*args, **kwargs): calls.append(args); return {'group_id': 7}
            method = self.manual({'load_report_snapshot': load})
            class Event:
                message_str = '检测报告'
                unified_msg_origin = 'fixture:GroupMessage:10001'
                is_at_or_wake_command = True
                stopped = False
                def get_message_type(self): return 'group'
                def stop_event(self): self.stopped = True
                def chain_result(self, data): return data
                def plain_result(self, data): return data
            for raw in [{'dry_run': True, 'targets': [{'session': 'fixture:GroupMessage:10001', 'group_ids': [7]}]},
                        {'dry_run': False, 'targets': [{'session': 'fixture:GroupMessage:10002', 'group_ids': [7]}]}]:
                cfgpath.write_text(json.dumps(raw))
                bot = SimpleNamespace(_report_config_path=cfgpath, _render_report=render, _report_get=lambda p: None)
                async def collect(): return [v async for v in method(bot, Event())]
                self.assertEqual(asyncio.run(collect()), [])
            self.assertEqual(calls, [])

    def test_manual_report_works_with_schedule_off_and_stops_after_yield(self):
        with tempfile.TemporaryDirectory() as tmp:
            cfgpath = Path(tmp)/'config.json'
            cfgpath.write_text(json.dumps({'enabled': False, 'dry_run': False,
                'targets': [{'session': 'fixture:GroupMessage:10001', 'group_ids': [7]}]}))
            record = {'group_id': 7, 'stats_status': 'available', 'stats': {'success_count': 1, 'total_count': 1, 'success_rate': 100.0},
                      'latest': None, 'artwork_status': 'none'}
            def load(*args, **kwargs): return record
            async def render(*args): return b'png-fixture'
            method = self.manual({'load_report_snapshot': load})
            class Event:
                message_str = '检测报告'
                unified_msg_origin = 'fixture:GroupMessage:10001'
                is_at_or_wake_command = True
                stopped = False
                def get_message_type(self): return 'group'
                def stop_event(self): self.stopped = True
                def chain_result(self, data): return data
                def plain_result(self, data): return data
            bot = SimpleNamespace(_report_config_path=cfgpath, _render_report=render, _report_get=lambda p: None)
            async def run():
                event = Event()
                result = method(bot, event)
                self.assertEqual(await result.__anext__(), [b'png-fixture'])
                self.assertFalse(event.stopped)
                with self.assertRaises(StopAsyncIteration): await result.__anext__()
                self.assertTrue(event.stopped)
            asyncio.run(run())

    def test_manual_batch_rechecks_earlier_group_after_later_render(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp)/'config.json'
            path.write_text(json.dumps({'enabled': False, 'dry_run': False,
                'targets': [{'session': 'fixture:GroupMessage:10001', 'group_ids': [7, 8]}]}))
            hidden, reads = [], []
            def load(get, group_id, **kwargs):
                reads.append(group_id)
                if group_id == 7 and hidden:
                    raise ValueError('fixture group withdrawn')
                return {'group_id': group_id, 'stats_status': 'available',
                        'stats': {'success_count': 1, 'total_count': 1, 'success_rate': 100.0},
                        'latest': None, 'artwork_status': 'none'}
            async def render(snapshot):
                if snapshot['group_id'] == 8:
                    hidden.append(7)
                return b'fixture-png'
            method = self.manual({'load_report_snapshot': load})
            event = SimpleNamespace(message_str='检测报告', unified_msg_origin='fixture:GroupMessage:10001',
                is_at_or_wake_command=True, get_message_type=lambda: 'group',
                stop_event=lambda: None, chain_result=lambda images: images)
            bot = SimpleNamespace(_report_config_path=path, _render_report=render, _report_get=lambda p: None)
            async def collect():
                return [result async for result in method(bot, event)]
            self.assertEqual(asyncio.run(collect()), [])
            self.assertEqual(reads[-1], 7)

    def test_disabled_loop_cleans_existing_cache_without_fetch_render_or_send(self):
        import time
        from pelican_delivery import (ReportDelivery, ReportStore, load_report_config,
                                      write_report_cache, CACHE_TTL_SECONDS)
        for existing in (False, True):
            with self.subTest(existing=existing), tempfile.TemporaryDirectory() as tmp:
                directory = Path(tmp)/'reports'
                image = directory/'cache'/('a'*64+'.png')
                if existing:
                    ReportStore(directory/'outbox.sqlite3')
                    write_report_cache(image, b'fixture', time.time()-CACHE_TTL_SECONDS-1)
                calls = []
                async def forbidden(*args):
                    calls.append(args)
                    raise AssertionError('disabled reports cannot access adapters')
                async def stop_after_iteration(*args):
                    raise asyncio.CancelledError()
                method = self.method('_report_loop', {
                    'asyncio': SimpleNamespace(sleep=stop_after_iteration),
                    'load_report_config': load_report_config, 'ReportDelivery': ReportDelivery,
                    'logger': SimpleNamespace(warning=lambda *args: None)})
                bot = SimpleNamespace(_report_config_path=directory/'config.json',
                    _report_directory=directory, _report_config=None, _report_service=None,
                    _load_report=forbidden, _render_report=forbidden, _send_report=forbidden)
                with self.assertRaises(asyncio.CancelledError):
                    asyncio.run(method(bot))
                self.assertEqual(calls, [])
                self.assertFalse(image.exists())
                self.assertEqual((directory/'outbox.sqlite3').exists(), existing)


if __name__ == '__main__':
    unittest.main()
