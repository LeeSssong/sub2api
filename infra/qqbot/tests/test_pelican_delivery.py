import asyncio
import importlib
import importlib.util
import io
import json
import os
import sys
import tempfile
import unittest
from datetime import datetime, timedelta, timezone
from email.message import Message
from pathlib import Path
from unittest.mock import patch
from urllib.response import addinfourl

PLUGIN = Path(__file__).resolve().parents[1] / 'plugins/astrbot_plugin_xingqiao_ops'
sys.path.insert(0, str(PLUGIN))


def snapshot(item_id=12, **extra):
    value = {
        'schema_version': 1, 'fetched_at': '2026-09-26T01:00:00+00:00',
        'group_id': 7, 'group_name': '公开示例分组', 'platform': 'openai',
        'stats': {'success_count': 3, 'total_count': 4, 'success_rate': 75.0},
        'stats_window': {'from': '2026-09-25T01:00:00+00:00',
                         'to': '2026-09-26T01:00:00+00:00',
                         'coverage_started_at': '2026-09-25T00:00:00+00:00', 'complete': True},
        'stats_status': 'available', 'artwork_status': 'available',
        'latest': {'id': item_id, 'group_id': 7, 'model': 'example-model',
                   'generated_at': '2026-09-26T00:58:00+00:00',
                   'latency_ms': 1000, 'response_text': '<svg></svg>'},
    }
    value.update(extra)
    return value


class DeliveryTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.module = importlib.import_module('pelican_delivery') if importlib.util.find_spec('pelican_delivery') else None

    def setUp(self):
        self.assertIsNotNone(self.module, 'Report delivery implementation is missing')
        self.d = self.module
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.path = Path(self.tmp.name)
        self.now = datetime(2026, 9, 26, 1, 0, tzinfo=timezone.utc)

    def config(self, **kw):
        raw = {'enabled': True, 'dry_run': False, 'timezone': 'Asia/Shanghai',
               'times': ['09:00', '15:00', '21:00'],
               'targets': [{'session': 'fixture:GroupMessage:10001', 'group_ids': [7]}]}
        raw.update(kw)
        return self.d.ReportConfig.from_dict(raw)

    def test_missing_config_cannot_fetch_render_or_send(self):
        cfg = self.d.load_report_config(self.path / 'missing.json')
        self.assertFalse(cfg.enabled)
        self.assertTrue(cfg.dry_run)
        async def forbidden(*args):
            self.fail('Disabled report service must not execute side effects')
        svc = self.d.ReportDelivery(cfg, self.path, forbidden, forbidden, forbidden)
        self.assertEqual(asyncio.run(svc.tick(self.now))['status'], 'disabled')

    def test_malformed_config_fails_closed(self):
        for raw in [{'enabled': 'false'}, {'dry_run': 'false'}, {'times': ['25:99']},
                    {'timezone': 'UTC'}, {'targets': [{'session': 'x:FriendMessage:2', 'group_ids': [7]}]},
                    {'targets': [{'session': 'x:GroupMessage:2', 'group_ids': [True]}]}]:
            with self.subTest(raw=raw), self.assertRaises(ValueError):
                self.d.ReportConfig.from_dict(raw)

    def test_time_slots_use_beijing_and_do_not_backfill_old_slots(self):
        cfg = self.config()
        self.assertEqual(self.d.due_slot(cfg, self.now), '2026-09-26T09:00+08:00')
        self.assertIsNone(self.d.due_slot(cfg, datetime(2026, 9, 26, 1, 11, tzinfo=timezone.utc)))
        self.assertIsNone(self.d.due_slot(cfg, datetime(2026, 9, 26, 19, 0, tzinfo=timezone.utc)))

    def test_dry_run_renders_without_calling_sender_or_marking_sent(self):
        sent = []
        async def load(g): return snapshot()
        async def render(s): return b'png-fixture'
        async def send(*args): sent.append(args); return '123'
        svc = self.d.ReportDelivery(self.config(dry_run=True), self.path, load, render, send)
        result = asyncio.run(svc.tick(self.now))
        self.assertEqual(result['rendered'], 1)
        self.assertEqual(sent, [])
        self.assertEqual(svc.store.rows()[0]['state'], 'dry_run')
        self.assertTrue(list((self.path / 'cache').glob('*.png')))

    def test_digest_ignores_refresh_time_but_tracks_real_result_and_counts(self):
        a = snapshot()
        b = snapshot(fetched_at='2026-09-26T01:05:00+00:00')
        b['stats_window'] = dict(a['stats_window'], to='2026-09-26T01:05:00+00:00',
                                 **{'from': '2026-09-25T01:05:00+00:00'})
        self.assertEqual(self.d.content_digest(a), self.d.content_digest(b))
        b['stats'] = {'success_count': 3, 'total_count': 5, 'success_rate': 60.0}
        self.assertNotEqual(self.d.content_digest(a), self.d.content_digest(b))
        self.assertNotEqual(self.d.content_digest(a), self.d.content_digest(snapshot(13)))

    def test_success_is_deduplicated_across_restarts_and_slots(self):
        sent = []
        async def load(g): return snapshot()
        async def render(s): return b'png-fixture'
        async def send(session, data): sent.append((session, data)); return '123'
        cfg = self.config()
        first = self.d.ReportDelivery(cfg, self.path, load, render, send)
        self.assertEqual(asyncio.run(first.tick(self.now))['sent'], 1)
        second = self.d.ReportDelivery(cfg, self.path, load, render, send)
        asyncio.run(second.tick(self.now))
        asyncio.run(second.tick(datetime(2026, 9, 26, 7, 0, tzinfo=timezone.utc)))
        self.assertEqual(len(sent), 1)
        self.assertEqual(second.store.rows()[0]['state'], 'sent')

    def test_unconfirmed_send_is_not_blindly_repeated(self):
        sent = []
        async def load(g): return snapshot()
        async def render(s): return b'png-fixture'
        async def send(*args): sent.append(1); raise TimeoutError('untrusted secret error')
        cfg = self.config()
        first = self.d.ReportDelivery(cfg, self.path, load, render, send)
        asyncio.run(first.tick(self.now))
        again = self.d.ReportDelivery(cfg, self.path, load, render, send)
        asyncio.run(again.tick(datetime(2026, 9, 26, 1, 1, tzinfo=timezone.utc)))
        self.assertEqual(sent, [1])
        self.assertEqual(again.store.rows()[0]['state'], 'uncertain')
        self.assertNotIn('secret', json.dumps(again.store.rows()))

    def test_stale_missing_or_empty_reports_never_send(self):
        rendered, sent = [], []
        async def render(s): rendered.append(s); return b'png-fixture'
        async def send(*args): sent.append(args); return 'receipt'
        for changes in [{'stats_status': 'stale'}, {'stats': None, 'latest': None},
                        {'stats': {'success_count': 0, 'total_count': 0, 'success_rate': None}, 'latest': None}]:
            async def load(g): return snapshot(**changes)
            svc = self.d.ReportDelivery(self.config(), self.path, load, render, send)
            self.assertEqual(asyncio.run(svc.tick(self.now))['sent'], 0)
        self.assertEqual(rendered, [])
        self.assertEqual(sent, [])

    def test_source_removal_before_delivery_cancels_message(self):
        calls, sent = [], []
        async def load(g):
            calls.append(g)
            if len(calls) > 1: raise ValueError('no longer publicly visible')
            return snapshot()
        async def render(s): return b'png-fixture'
        async def send(*args): sent.append(args); return 'receipt'
        svc = self.d.ReportDelivery(self.config(), self.path, load, render, send)
        self.assertEqual(asyncio.run(svc.tick(self.now))['sent'], 0)
        self.assertEqual(sent, [])
        self.assertEqual(list((self.path / 'cache').glob('*.png')), [])

    def test_one_target_failure_does_not_block_other_target(self):
        sent = []
        async def load(g): return snapshot()
        async def render(s): return b'png-fixture'
        async def send(session, data):
            sent.append(session)
            if session.endswith('10001'): raise TimeoutError()
            return '222'
        cfg = self.config(targets=[{'session': 'fixture:GroupMessage:10001', 'group_ids': [7]},
                                   {'session': 'fixture:GroupMessage:10002', 'group_ids': [7]}])
        svc = self.d.ReportDelivery(cfg, self.path, load, render, send)
        self.assertEqual(asyncio.run(svc.tick(self.now))['sent'], 1)
        self.assertEqual(len(sent), 2)

    def test_process_crash_sending_state_recovers_as_uncertain(self):
        store = self.d.ReportStore(self.path / 'outbox.sqlite3')
        job = store.enqueue('fixture:GroupMessage:10001', 7, 'slot', 'digest', self.now.timestamp(), 1800)
        self.assertTrue(store.claim(job['id'], self.now.timestamp()))
        recovered = self.d.ReportStore(self.path / 'outbox.sqlite3')
        self.assertEqual(recovered.rows()[0]['state'], 'uncertain')

    def test_definite_failure_retries_only_after_backoff(self):
        frozen_clock = patch('time.monotonic', return_value=0.0)
        frozen_clock.start()
        self.addCleanup(frozen_clock.stop)
        sent = []
        async def load(g): return snapshot()
        async def render(s): return b'png-fixture'
        async def send(*args):
            sent.append(1)
            if len(sent) == 1: raise self.d.DefiniteSendFailure()
            return 'receipt'
        svc = self.d.ReportDelivery(self.config(), self.path, load, render, send)
        asyncio.run(svc.tick(self.now))
        self.assertEqual(svc.store.rows()[0]['state'], 'retry')
        asyncio.run(svc.tick(datetime(2026, 9, 26, 1, 0, 29, tzinfo=timezone.utc)))
        self.assertEqual(len(sent), 1)
        asyncio.run(svc.tick(datetime(2026, 9, 26, 1, 0, 30, tzinfo=timezone.utc)))
        self.assertEqual(len(sent), 2)
        self.assertEqual(svc.store.rows()[0]['state'], 'sent')

    def test_wrong_group_snapshot_is_rejected(self):
        rendered, sent = [], []
        async def load(g): return snapshot(group_id=8)
        async def render(s): rendered.append(s); return b'png-fixture'
        async def send(*args): sent.append(args); return 'receipt'
        svc = self.d.ReportDelivery(self.config(), self.path, load, render, send)
        self.assertEqual(asyncio.run(svc.tick(self.now))['sent'], 0)
        self.assertEqual(rendered, [])
        self.assertEqual(sent, [])

    def test_transient_render_failure_recovers_same_job_inside_grace(self):
        rendered, sent = [], []
        async def load(g): return snapshot()
        async def render(s):
            rendered.append(1)
            if len(rendered) == 1:
                raise RuntimeError('temporary renderer failure')
            return b'png-fixture'
        async def send(*args): sent.append(args); return 'receipt'
        svc = self.d.ReportDelivery(self.config(), self.path, load, render, send)
        first = asyncio.run(svc.tick(self.now))
        self.assertEqual(first['sent'], 0)
        original = svc.store.rows()[0]
        self.assertEqual(original['state'], 'cancelled')
        recovered = asyncio.run(svc.tick(self.now + timedelta(seconds=60)))
        self.assertEqual(recovered['sent'], 1)
        jobs = svc.store.rows()
        self.assertEqual(len(jobs), 1)
        self.assertEqual(jobs[0]['expires_at'], original['expires_at'])
        self.assertEqual(jobs[0]['attempts'], 1)
        asyncio.run(svc.tick(self.now + timedelta(seconds=120)))
        self.assertEqual(len(sent), 1)

    def test_transient_visibility_read_recovers_without_sending_unverified_data(self):
        loaded, sent = [], []
        async def load(g):
            loaded.append(g)
            if len(loaded) == 2:
                raise TimeoutError('temporary read failure')
            return snapshot()
        async def render(s): return b'png-fixture'
        async def send(*args): sent.append(args); return 'receipt'
        svc = self.d.ReportDelivery(self.config(), self.path, load, render, send)
        self.assertEqual(asyncio.run(svc.tick(self.now))['sent'], 0)
        self.assertEqual(sent, [])
        self.assertEqual(asyncio.run(svc.tick(self.now + timedelta(seconds=60)))['sent'], 1)
        self.assertEqual(len(sent), 1)

    def test_cancelled_recovery_preserves_attempts_and_original_expiry(self):
        store = self.d.ReportStore(self.path / 'outbox.sqlite3')
        ts = self.now.timestamp()
        job = store.enqueue('fixture:GroupMessage:10001', 7, 'slot', 'digest', ts, 1800)
        self.assertTrue(store.claim(job['id'], ts))
        store.finish(job['id'], 'cancelled')
        revived = store.enqueue('fixture:GroupMessage:10001', 7, 'slot', 'digest', ts+60, 3600)
        self.assertEqual(revived['state'], 'pending')
        self.assertEqual(revived['attempts'], 1)
        self.assertEqual(revived['expires_at'], ts+1800)

    def test_terminal_or_expired_jobs_are_never_reopened(self):
        store = self.d.ReportStore(self.path / 'outbox.sqlite3')
        ts = self.now.timestamp()
        for index, state in enumerate(('sent', 'uncertain', 'failed', 'cancelled')):
            digest = f'digest-{index}'
            job = store.enqueue('fixture:GroupMessage:10001', 7, 'slot', digest, ts, 60)
            store.finish(job['id'], state)
            current = store.enqueue('fixture:GroupMessage:10001', 7, 'slot', digest, ts+60, 3600)
            self.assertEqual(current['state'], state)
        pending = store.enqueue('fixture:GroupMessage:10001', 7, 'slot', 'expired', ts, 60)
        store.ready(ts+60)
        current = store.enqueue('fixture:GroupMessage:10001', 7, 'slot', 'expired', ts+60, 3600)
        self.assertEqual(current['id'], pending['id'])
        self.assertEqual(current['state'], 'expired')
        self.assertEqual(store.ready(ts+61), [])

    def test_recoverable_cancelled_job_cannot_duplicate_an_uncertain_slot(self):
        sent = []
        async def load(g): return snapshot()
        async def render(s): return b'png-fixture'
        async def send(*args): sent.append(args); return 'receipt'
        svc = self.d.ReportDelivery(self.config(), self.path, load, render, send)
        ts = self.now.timestamp()
        slot = self.d.due_slot(svc.config, self.now)
        job = svc.store.enqueue('fixture:GroupMessage:10001', 7, slot,
                                self.d.content_digest(snapshot()), ts, 1800)
        svc.store.finish(job['id'], 'cancelled')
        other = svc.store.enqueue('fixture:GroupMessage:10001', 7, slot, 'other-digest', ts, 1800)
        svc.store.finish(other['id'], 'uncertain')
        self.assertEqual(asyncio.run(svc.tick(self.now+timedelta(seconds=60)))['sent'], 0)
        self.assertEqual(sent, [])

    def test_slow_preparation_cannot_send_after_original_deadline(self):
        clock, sent = [1000.0], []
        async def load(g): return snapshot()
        async def render(s):
            clock[0] += 61
            return b'png-fixture'
        async def send(*args): sent.append(args); return 'receipt'
        svc = self.d.ReportDelivery(self.config(expiry_seconds=60), self.path, load, render, send)
        with patch('time.monotonic', side_effect=lambda: clock[0]):
            result = asyncio.run(svc.tick(self.now))
        self.assertEqual(result['sent'], 0)
        self.assertEqual(sent, [])
        self.assertEqual(svc.store.rows()[0]['attempts'], 0)

    def test_definite_retry_backoff_starts_after_the_failed_attempt_finishes(self):
        clock = [1000.0]
        async def load(g): return snapshot()
        async def render(s): return b'png-fixture'
        async def send(*args):
            clock[0] += 45
            raise self.d.DefiniteSendFailure()
        svc = self.d.ReportDelivery(self.config(), self.path, load, render, send)
        with patch('time.monotonic', side_effect=lambda: clock[0]):
            asyncio.run(svc.tick(self.now))
        row = svc.store.rows()[0]
        self.assertEqual(row['state'], 'retry')
        self.assertEqual(row['ready_at'], self.now.timestamp()+75)
        self.assertEqual(row['expires_at'], self.now.timestamp()+1800)

    def test_definite_failures_stop_after_the_bounded_retry_budget(self):
        sent = []
        async def load(g): return snapshot()
        async def render(s): return b'png-fixture'
        async def send(*args):
            sent.append(1)
            raise self.d.DefiniteSendFailure()
        svc = self.d.ReportDelivery(self.config(), self.path, load, render, send)
        with patch('time.monotonic', return_value=0.0):
            for elapsed in (0, 30, 150, 450, 500):
                asyncio.run(svc.tick(self.now+timedelta(seconds=elapsed)))
        self.assertEqual(len(sent), 4)
        self.assertEqual(svc.store.rows()[0]['state'], 'failed')

    def test_missing_receipt_is_uncertain_across_restart_and_later_slot(self):
        sent = []
        async def load(g): return snapshot()
        async def render(s): return b'png-fixture'
        async def send(*args): sent.append(1); return None
        cfg = self.config()
        first = self.d.ReportDelivery(cfg, self.path, load, render, send)
        asyncio.run(first.tick(self.now))
        recovered = self.d.ReportDelivery(cfg, self.path, load, render, send)
        asyncio.run(recovered.tick(self.now+timedelta(hours=6)))
        self.assertEqual(sent, [1])
        self.assertEqual(recovered.store.rows()[0]['state'], 'uncertain')

    def test_default_http_client_refuses_redirect_without_forwarding_key(self):
        client = importlib.import_module('pelican_client')
        requests = []
        def https_open(handler, request):
            requests.append(request)
            headers = Message()
            headers['Location'] = 'https://elsewhere.invalid/redirect'
            response = addinfourl(io.BytesIO(b''), headers, request.full_url, 302)
            response.msg = 'Found'
            return response
        # Exercise the real default opener and redirect handling. Only the
        # socket boundary is replaced; no connection is opened by this test.
        with patch('urllib.request.HTTPSHandler.https_open', new=https_open):
            with self.assertRaises(client.PelicanSourceUnavailable):
                client.ReadOnlyReportClient('fixture-key')('/api/v1/admin/pelican-showcase')
        self.assertEqual(len(requests), 1)
        self.assertEqual(requests[0].full_url,
                         'https://api.xingqiaolab.top/api/v1/admin/pelican-showcase')

    def cache_file(self, digest, content=b'png-fixture', age=0):
        path = self.path / 'cache' / (digest+'.png')
        self.d.write_report_cache(path, content)
        stamp = self.now.timestamp()-age
        os.utime(path, (stamp, stamp))
        return path

    def collect_cache(self, store, reserve_bytes=0):
        collect = getattr(self.d, 'prune_report_cache', None)
        self.assertTrue(callable(collect), 'Bounded cache collection has not been implemented')
        return collect(self.path / 'cache', store, self.now.timestamp(), reserve_bytes)

    def test_cache_ttl_and_size_remove_oldest_unreferenced_images(self):
        store = self.d.ReportStore(self.path / 'outbox.sqlite3')
        expired = self.cache_file('a'*64, b'1234567890', age=86401)
        oldest = self.cache_file('b'*64, b'1234567', age=30)
        newest = self.cache_file('c'*64, b'12345678', age=10)
        with patch.object(self.d, 'CACHE_MAX_BYTES', 12, create=True):
            self.assertTrue(self.collect_cache(store))
        self.assertFalse(expired.exists())
        self.assertFalse(oldest.exists())
        self.assertEqual(newest.read_bytes(), b'12345678')
        self.assertEqual(newest.stat().st_mode & 0o777, 0o600)

    def test_cache_file_count_is_bounded_even_for_tiny_images(self):
        store = self.d.ReportStore(self.path / 'outbox.sqlite3')
        files = [self.cache_file(char*64, b'x', age=age)
                 for char, age in (('a', 30), ('b', 20), ('c', 10))]
        with patch.object(self.d, 'CACHE_MAX_FILES', 2, create=True):
            self.assertTrue(self.collect_cache(store))
        self.assertEqual([path.exists() for path in files], [False, True, True])

    def test_active_retry_reference_keeps_shared_cancelled_cache(self):
        store = self.d.ReportStore(self.path / 'outbox.sqlite3')
        ts, digest = self.now.timestamp(), 'a'*64
        old = store.enqueue('fixture:GroupMessage:10001', 7, 'slot', digest, ts, 1800)
        store.finish(old['id'], 'cancelled')
        other = store.enqueue('fixture:GroupMessage:10002', 7, 'slot', digest, ts, 1800)
        store.claim(other['id'], ts)
        store.retry(other['id'], ts)
        path = self.cache_file(digest, b'1234567890', age=86401)
        with patch.object(self.d, 'CACHE_MAX_BYTES', 10, create=True):
            self.assertFalse(self.collect_cache(store, reserve_bytes=1))
        self.assertTrue(path.exists())
        store.finish(other['id'], 'cancelled')
        self.collect_cache(store)
        self.assertFalse(path.exists())

    def test_expired_pending_reference_no_longer_keeps_cache(self):
        store = self.d.ReportStore(self.path / 'outbox.sqlite3')
        ts, digest = self.now.timestamp(), 'a'*64
        store.enqueue('fixture:GroupMessage:10001', 7, 'slot', digest, ts-60, 60)
        path = self.cache_file(digest)
        self.collect_cache(store)
        self.assertFalse(path.exists())
        self.assertEqual(store.rows()[0]['state'], 'expired')

    def test_cancellation_discards_cache_despite_later_sent_reference(self):
        async def unavailable(g): raise ValueError('public record unavailable')
        async def forbidden(*args): self.fail('Unavailable records must never render or send')
        svc = self.d.ReportDelivery(self.config(), self.path, unavailable, forbidden, forbidden)
        ts, digest = self.now.timestamp(), self.d.content_digest(snapshot())
        svc.store.enqueue('fixture:GroupMessage:10001', 7, self.d.due_slot(svc.config, self.now), digest, ts, 1800)
        sent = svc.store.enqueue('fixture:GroupMessage:10002', 7, 'another-slot', digest, ts, 1800)
        svc.store.finish(sent['id'], 'sent', 'receipt')
        path = self.cache_file(digest)
        self.assertEqual(asyncio.run(svc.tick(self.now))['sent'], 0)
        self.assertFalse(path.exists())
        self.assertTrue(svc.store.handled('fixture:GroupMessage:10002', 7, 'future-slot', digest))

    def test_cache_gc_ignores_unowned_files_and_never_follows_symlinks(self):
        store = self.d.ReportStore(self.path / 'outbox.sqlite3')
        cache = self.path / 'cache'
        cache.mkdir()
        external = self.path / 'outside.png'
        external.write_bytes(b'keep-external')
        (cache / ('a'*64+'.png')).symlink_to(external)
        unknown = cache / 'user-image.png'
        unknown.write_bytes(b'keep-user-image')
        lookalike = cache / ('c'*64+'Xpng')
        lookalike.write_bytes(b'keep-lookalike')
        (cache / ('b'*64+'.png')).mkdir()
        with patch.object(self.d, 'CACHE_MAX_BYTES', 0, create=True):
            self.collect_cache(store)
        self.assertEqual(external.read_bytes(), b'keep-external')
        self.assertTrue((cache / ('a'*64+'.png')).is_symlink())
        self.assertEqual(unknown.read_bytes(), b'keep-user-image')
        self.assertTrue(lookalike.exists())
        self.assertEqual(lookalike.read_bytes(), b'keep-lookalike')
        self.assertTrue((cache / ('b'*64+'.png')).is_dir())

    def test_history_gc_keeps_sent_and_uncertain_dedupe_and_live_jobs(self):
        store = self.d.ReportStore(self.path / 'outbox.sqlite3')
        ts = self.now.timestamp()
        for state in ('cancelled', 'failed', 'dry_run', 'sent', 'uncertain'):
            row = store.enqueue('fixture:GroupMessage:10001', 7, state, state, ts-31*86400, 60)
            store.finish(row['id'], state)
        live = store.enqueue('fixture:GroupMessage:10001', 7, 'live', 'live', ts, 1800)
        self.collect_cache(store)
        self.assertEqual({row['state'] for row in store.rows()}, {'sent', 'uncertain', 'pending'})
        self.assertTrue(store.handled('fixture:GroupMessage:10001', 7, 'later', 'sent'))
        self.assertTrue(store.handled('fixture:GroupMessage:10001', 7, 'later', 'uncertain'))
        self.assertTrue(any(row['id'] == live['id'] for row in store.rows()))

    def test_history_gc_caps_recent_non_dedupe_terminal_rows(self):
        store = self.d.ReportStore(self.path / 'outbox.sqlite3')
        ts = self.now.timestamp()
        for i in range(4):
            row = store.enqueue('fixture:GroupMessage:10001', 7, 'slot', str(i), ts-100+i, 60)
            store.finish(row['id'], 'cancelled')
        with patch.object(self.d, 'HISTORY_MAX_INACTIVE_ROWS', 2, create=True):
            self.collect_cache(store)
        self.assertEqual([row['digest'] for row in store.rows()], ['2', '3'])

    def test_full_protected_cache_uses_new_image_in_memory_without_growth(self):
        sent = []
        async def load(g):
            if g == 99: raise ValueError('temporary read failure')
            return snapshot()
        async def render(s): return b'fresh-image'
        async def send(session, data): sent.append(data); return 'receipt'
        cfg = self.config(targets=[{'session': 'fixture:GroupMessage:10001', 'group_ids': [7, 99]}])
        svc = self.d.ReportDelivery(cfg, self.path, load, render, send)
        ts, digest = self.now.timestamp(), 'a'*64
        row = svc.store.enqueue('fixture:GroupMessage:10001', 99, 'earlier-slot', digest, ts, 1800)
        svc.store.claim(row['id'], ts)
        svc.store.retry(row['id'], ts)
        protected = self.cache_file(digest, b'full!')
        with patch.object(self.d, 'CACHE_MAX_BYTES', 5, create=True):
            result = asyncio.run(svc.tick(self.now))
        self.assertEqual(result['sent'], 1)
        self.assertEqual(sent, [b'fresh-image'])
        self.assertEqual(list((self.path / 'cache').glob('*.png')), [protected])
        self.assertEqual(protected.read_bytes(), b'full!')


if __name__ == '__main__':
    unittest.main()
