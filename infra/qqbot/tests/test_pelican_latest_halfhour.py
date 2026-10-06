import asyncio
import base64
import copy
import json
import sys
import tempfile
import unittest
from datetime import datetime, timedelta, timezone
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'plugins/astrbot_plugin_xingqiao_ops'))
import pelican_delivery as delivery
import pelican_report_source as source
from test_pelican_report_template import Document, snapshot, thumbnail
from pelican_renderer import build_report_html


class LatestResultTests(unittest.TestCase):
    def test_mixed_batch_uses_newest_result_and_its_time(self):
        s = snapshot()
        p = s['executions']['pelican']
        latest = p['results'][0]
        p.update(status='failed', expected_count=2, completed_count=2)
        p['results'] = [dict(latest, result_id=99, status='failed'), latest]
        s['current'] = dict(pelican=latest, candy=s['executions']['candy']['results'][0])
        rendered = build_report_html(s, thumbnail())
        self.assertIn('鹈鹕测试 通过', Document(rendered).text)
        self.assertIn('鹈鹕检测完成 09/26 15:00:21', Document(rendered).text)
        self.assertIn('本次耗时 20 秒', Document(rendered).text)
        self.assertIn(base64.b64encode(thumbnail()).decode(), rendered)

    def test_latest_failure_always_shows_card_even_with_old_image(self):
        s = snapshot()
        latest = dict(s['executions']['pelican']['results'][0], status='failed')
        s['current'] = dict(pelican=latest)
        rendered = build_report_html(s, thumbnail())
        self.assertIn('鹈鹕测试 无数据', Document(rendered).text)
        self.assertNotIn(base64.b64encode(thumbnail()).decode(), rendered)
        self.assertNotIn('待更新', Document(rendered).text)

    def test_pending_batch_keeps_completed_individual_result(self):
        s = snapshot()
        latest = s['executions']['pelican']['results'][0]
        s['executions']['pelican'].update(status='pending', expected_count=2)
        s['current'] = dict(pelican=latest)
        rendered = build_report_html(s, thumbnail())
        self.assertIn('鹈鹕测试 通过', Document(rendered).text)
        self.assertIn(base64.b64encode(thumbnail()).decode(), rendered)

    def test_source_reads_current_artwork_despite_pending_batch(self):
        raw = json.loads((Path(__file__).with_name('fixtures') / 'pelican-report-v2.json').read_text())['report']
        latest = raw['latest']['pelican']
        raw['current'] = dict(candy=copy.deepcopy(raw['latest']['candy']['results'][0]),
                              pelican=copy.deepcopy(latest['results'][0]), artwork=raw['artwork'])
        raw['artwork'] = None
        latest.update(status='pending', expected_count=2, completed_at=None, latency_ms=None)
        now = datetime(2026, 9, 26, 7, 5, tzinfo=timezone.utc)
        value = source.load_detection_report_snapshot(lambda _: raw, 6, now=now)
        self.assertEqual(value['latest']['source_result_id'], 102)
        self.assertEqual(value['current']['pelican']['result_id'], 102)
        raw['current']['pelican']['result_id'] = 999
        with self.assertRaises(source.PelicanSourceUnavailable):
            source.load_detection_report_snapshot(lambda _: raw, 6, now=now)

    def test_current_can_be_newer_generation_from_a_different_completed_execution(self):
        raw = json.loads((Path(__file__).with_name('fixtures') / 'pelican-report-v2.json').read_text())['report']
        drawing = copy.deepcopy(raw['latest']['pelican']['results'][0])
        raw['current'] = dict(candy=copy.deepcopy(raw['latest']['candy']['results'][0]),
                              pelican=drawing, artwork=raw['artwork'])
        # Legacy latest is completion-ordered; current is generation-ordered.
        raw['latest']['pelican']['execution_id'] = 'slower-old-generation'
        older = dict(drawing, result_id=103, execution_id='slower-old-generation',
                     started_at='2026-09-26T06:59:00Z', completed_at='2026-09-26T07:00:30Z', latency_ms=90000)
        raw['latest']['pelican'].update(started_at=older['started_at'], completed_at=older['completed_at'],
                                        latency_ms=older['latency_ms'], results=[older])
        raw['history']['pelican'].append(older)
        raw['history_meta']['pelican'].update(total_count=2, returned_count=2)
        raw['statistics']['pelican'].update(success_count=2, total_count=2, observed_count=2,
                                           timed_count=2, avg_latency_ms=55000, execution_count=2)
        raw['artwork'] = None
        value = source.load_detection_report_snapshot(lambda _: raw, 6, now=datetime(2026,9,26,7,5,tzinfo=timezone.utc))
        self.assertEqual(value['current']['pelican']['result_id'],102)
        self.assertEqual(value['latest']['source_result_id'],102)


class HalfHourTests(unittest.TestCase):
    def test_latest_completed_result_remains_usable_outside_statistics_window(self):
        s = snapshot()
        s['current'] = dict(pelican=s['executions']['pelican']['results'][0])
        for stats in s['statistics'].values():
            stats['observed_count'] = 0
        self.assertTrue(delivery.report_is_usable(s))

    def test_48_slots_repeat_unchanged_next_slot_but_not_after_restart(self):
        cfg = delivery.ReportConfig.from_dict(dict(enabled=True, dry_run=True,
            repeat_unchanged=True, times=[f'{h:02d}:{m:02d}' for h in range(24) for m in (0, 30)],
            targets=[dict(session='fixture:GroupMessage:10001', group_ids=[6])]))
        start = datetime(2026, 9, 26, 16, 0, tzinfo=timezone.utc)
        self.assertEqual(delivery.due_slot(cfg, start), '2026-09-27T00:00+08:00')
        self.assertEqual(delivery.due_slot(cfg, start + timedelta(hours=23, minutes=30)), '2026-09-27T23:30+08:00')
        self.assertIsNone(delivery.due_slot(cfg, start + timedelta(minutes=11)))
        renders, sends = [], []
        async def load(group): return snapshot()
        async def render(data): renders.append(1); return b'fixture-image'
        async def send(*args): sends.append(1); raise AssertionError('dry run must not send')
        with tempfile.TemporaryDirectory() as directory:
            bot = delivery.ReportDelivery(cfg, Path(directory), load, render, send)
            asyncio.run(bot.tick(start))
            bot = delivery.ReportDelivery(cfg, Path(directory), load, render, send)
            asyncio.run(bot.tick(start + timedelta(minutes=1)))
            asyncio.run(bot.tick(start + timedelta(minutes=30)))
            self.assertEqual(len(bot.store.rows()), 2)
            self.assertTrue(all(r['state'] == 'dry_run' for r in bot.store.rows()))
        self.assertEqual(len(renders), 2)
        self.assertEqual(sends, [])
