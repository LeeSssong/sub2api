"""Consumer-visible report contract checks, using only local fixture data."""
import asyncio
import base64
import importlib.util
import io
import json
import os
import struct
import sys
import unittest
from copy import deepcopy
from html.parser import HTMLParser
from pathlib import Path

PLUGIN = Path(__file__).resolve().parents[1] / 'plugins/astrbot_plugin_xingqiao_ops'
sys.path.insert(0, str(PLUGIN))

import pelican_renderer


def snapshot():
    fixture_path = Path(__file__).with_name('fixtures') / 'pelican-report-v2.json'
    raw = json.loads(fixture_path.read_text(encoding='utf-8'))['report']
    artwork = dict(raw['artwork'], model=raw['artwork']['model_id'])
    return {
        'schema_version': 2, 'snapshot_id': raw['snapshot_id'],
        'as_of': raw['as_of'], 'fetched_at': '2026-09-26T07:09:00Z',
        'group_id': raw['group']['id'], 'group_name': raw['group']['name'],
        'platform': raw['group']['platform'], 'model_id': raw['model_id'],
        'rate_multiplier': raw['group']['rate_multiplier'],
        'stats_window': raw['window'], 'stats_status': 'available',
        'statistics': raw['statistics'], 'history': raw['history'],
        'executions': raw['latest'], 'current_round': raw['current_round'],
        'latest': artwork, 'artwork_status': 'available',
    }


def thumbnail():
    from PIL import Image
    output = io.BytesIO()
    Image.new('RGB', (8, 4), '#ff8800').save(output, format='PNG')
    return output.getvalue()


class Document(HTMLParser):
    def __init__(self, source):
        super().__init__()
        self.tags = []
        self.parts = []
        self.hidden_depth = 0
        self.feed(source)

    def handle_starttag(self, tag, attrs):
        self.tags.append((tag, dict(attrs)))
        if tag in ('style', 'script'):
            self.hidden_depth += 1

    def handle_endtag(self, tag):
        if tag in ('style', 'script'):
            self.hidden_depth = max(0, self.hidden_depth - 1)

    def handle_data(self, data):
        if not self.hidden_depth and data.strip():
            self.parts.append(data.strip())

    @property
    def text(self):
        return ' '.join(self.parts)


class ReportTemplateTests(unittest.TestCase):
    def test_fixture_watermark_is_opt_in_and_absent_from_live_reports(self):
        s = snapshot()
        self.assertNotIn('离线自测', Document(self.build(s)).text)
        s['offline_fixture'] = True
        self.assertIn('【示例数据】离线自测', Document(self.build(s)).text)

    def build(self, snapshot, thumbnail=None):
        try:
            return pelican_renderer.build_report_html(snapshot, thumbnail)
        except ValueError as exc:
            self.fail(f'Version 2 report must render available fields: {exc}')

    def test_empty_v2_report_preserves_required_sections_without_fabricated_values(self):
        doc = Document(self.build({'schema_version': 2}))
        for heading in ('分组', '模型', '倍率', '糖果逻辑', '鹈鹕测试',
                        '本次作品截图', '近期检测记录', '鹈鹕生成', '鹈鹕检测完成', '本次耗时'):
            self.assertIn(heading, doc.text)
        self.assertIn('通过 —/—', doc.text)
        self.assertIn('成功 —/—', doc.text)
        self.assertEqual(doc.text.count('异常 — 次 · 平均耗时 — 秒'), 2)
        for fabricated in ('0/0', '100.0%', '0.0%', '0 批次', '0 次'):
            self.assertNotIn(fabricated, doc.text)

    def test_missing_fields_do_not_restore_rejected_coverage_or_failure_copy(self):
        doc = Document(self.build({'schema_version': 2}))
        for rejected in ('统计不完整', '覆盖起点', '未成功', '覆盖范围',
                         '汇总比例', '非逐次时间线', '统计暂不可用'):
            self.assertNotIn(rejected, doc.text)

    def test_report_stays_static_and_self_contained(self):
        doc = Document(self.build({'schema_version': 2}))
        tags = {tag for tag, attrs in doc.tags}
        self.assertTrue({'script', 'iframe', 'object', 'embed'}.isdisjoint(tags))
        csp = next(attrs['content'] for tag, attrs in doc.tags
                   if tag == 'meta' and attrs.get('http-equiv') == 'Content-Security-Policy')
        self.assertIn("default-src 'none'", csp)
        self.assertIn("connect-src 'none'", csp)
        self.assertIn("script-src 'none'", csp)
        self.assertTrue(all(attrs.get('src', '').startswith('data:image/png;base64,')
                            for tag, attrs in doc.tags if tag == 'img'))

    def test_available_fields_use_report_as_of_and_real_attempt_count(self):
        doc = Document(self.build(snapshot()))
        self.assertIn('GPT-Pro5x', doc.text)
        self.assertIn('gpt-6-astra', doc.text)
        self.assertIn('5x', doc.text)
        self.assertIn('09/26 15:05:00 北京时间', doc.text)
        self.assertNotIn('15:09:00', doc.text)
        self.assertIn('24 小时 · 2 次', doc.text)
        self.assertNotIn('批次', doc.text)
        self.assertIn('通过 1/1', doc.text)
        self.assertIn('成功 1/1', doc.text)
        self.assertEqual(doc.text.count('100.0%'), 2)
        self.assertIn('异常 0 次 · 平均耗时 10.0 秒', doc.text)
        self.assertIn('异常 0 次 · 平均耗时 20.0 秒', doc.text)

    def test_ungraded_history_is_neutral_and_not_a_passing_sample(self):
        s = snapshot()
        rows = s['history']['candy']
        rows.append(dict(rows[0], result_id=103, status='ungraded', judgment='ungraded_candy'))
        rows.append(dict(rows[0], result_id=104, status='failed'))
        s['statistics']['candy'] = dict(
            success_count=1, total_count=2, failure_count=1, ungraded_count=1,
            observed_count=3, success_rate=50, timed_count=2,
            avg_latency_ms=12500, execution_count=1)
        doc = Document(self.build(s))
        self.assertIn('通过 1/2', doc.text)
        self.assertIn('50.0%', doc.text)
        self.assertIn('24 小时 · 4 次', doc.text)
        self.assertIn('异常 1 次 · 平均耗时 12.5 秒', doc.text)
        ticks = [attrs['data-status'] for tag, attrs in doc.tags if 'data-status' in attrs]
        self.assertEqual(ticks, ['success', 'ungraded', 'failed', 'success'])

    def test_latest_ungraded_check_does_not_claim_a_pass(self):
        for status in ('ungraded',):
            with self.subTest(status=status):
                s = snapshot()
                s['executions']['candy']['status'] = status
                s['executions']['candy']['results'][0]['status'] = status
                doc = Document(self.build(s))
                self.assertIn('糖果逻辑 — 鹈鹕测试 不通过', doc.text)

    def test_all_failed_statistics_show_real_zero_percent_and_actual_failures(self):
        s = snapshot()
        s['statistics']['pelican'] = dict(
            success_count=0, total_count=3, failure_count=3, ungraded_count=0,
            observed_count=3, success_rate=0, timed_count=0,
            avg_latency_ms=None, execution_count=1)
        s['executions']['pelican']['status'] = 'failed'
        s['executions']['pelican']['results'][0]['status'] = 'failed'
        doc = Document(self.build(s))
        self.assertIn('鹈鹕测试 无数据', doc.text)
        self.assertIn('成功 0/3', doc.text)
        self.assertIn('0.0%', doc.text)
        self.assertIn('异常 3 次 · 平均耗时 — 秒', doc.text)
        self.assertNotIn('未成功', doc.text)

    def test_real_empty_statistics_are_distinct_from_missing_statistics(self):
        s = snapshot()
        for kind in ('candy', 'pelican'):
            s['statistics'][kind] = dict(success_count=0, total_count=0, failure_count=0,
                ungraded_count=0, observed_count=0, success_rate=None, timed_count=0,
                avg_latency_ms=None, execution_count=0)
            s['history'][kind] = []
            s['executions'][kind] = None
        s['latest'] = None
        s['artwork_status'] = 'none'
        doc = Document(self.build(s))
        self.assertIn('24 小时 · 0 次', doc.text)
        self.assertIn('通过 0/0', doc.text)
        self.assertIn('成功 0/0', doc.text)
        self.assertEqual(doc.text.count('异常 0 次 · 平均耗时 — 秒'), 2)
        self.assertNotIn('0.0%', doc.text)
        self.assertNotIn('100.0%', doc.text)
        self.assertEqual([attrs for _, attrs in doc.tags if 'data-status' in attrs], [])

    def test_footer_uses_latest_result_even_when_shared_round_has_different_times(self):
        s = snapshot()
        doc = Document(self.build(s))
        self.assertIn('鹈鹕检测完成 09/26 15:00:21 本次耗时 20 秒', doc.text)
        s['current_round'] = {
            'round_id': 'verified-shared-round',
            'scheduled_for': '2026-09-26T07:00:00Z',
            'started_at': '2026-09-26T07:00:01Z',
            'completed_at': '2026-09-26T07:01:47Z',
            'latency_ms': 106000,
            'candy_execution_id': s['executions']['candy']['execution_id'],
            'pelican_execution_id': s['executions']['pelican']['execution_id'],
        }
        doc = Document(self.build(s))
        self.assertIn('鹈鹕检测完成 09/26 15:00:21 本次耗时 20 秒', doc.text)

    def test_failed_or_mismatched_latest_execution_cannot_show_an_old_artwork(self):
        for change in ('failed', 'mismatched_execution', 'mismatched_result'):
            with self.subTest(change=change):
                s = snapshot()
                if change == 'failed':
                    s['executions']['pelican']['status'] = 'failed'
                    s['executions']['pelican']['results'][0]['status'] = 'failed'
                elif change == 'mismatched_execution':
                    s['latest']['execution_id'] = 'different-execution'
                else:
                    s['latest']['source_result_id'] = 999
                encoded = base64.b64encode(thumbnail()).decode('ascii')
                source = self.build(s, thumbnail())
                self.assertNotIn(encoded, source)
                self.assertIn('本次作品截图', Document(source).text)

    def test_current_successful_artwork_uses_validated_png_only(self):
        data = thumbnail()
        source = self.build(snapshot(), data)
        self.assertTrue(base64.b64encode(data).decode('ascii') in source, 'matching PNG must be shown')
        self.assertNotIn('Fixture pelican', source)
        with self.assertRaisesRegex(ValueError, 'PNG'):
            pelican_renderer.build_report_html(snapshot(), b'<svg onload=alert(1)>')

    def test_historical_single_result_keeps_its_matching_artwork_without_inventing_a_batch(self):
        s = snapshot()
        execution = s['executions']['pelican']
        execution['execution_id'] = None
        execution['expected_count'] = None
        execution['results'][0]['execution_id'] = None
        s['latest']['execution_id'] = None
        s['statistics']['pelican']['execution_count'] = None
        data = thumbnail()
        source = self.build(s, data)
        self.assertTrue(base64.b64encode(data).decode('ascii') in source, 'historical matching PNG must be shown')
        doc = Document(source)
        self.assertIn('24 小时 · 2 次', doc.text)
        self.assertIn('鹈鹕检测完成 09/26 15:00:21 本次耗时 20 秒', doc.text)
        self.assertNotIn('批次', doc.text)

    def test_public_text_is_escaped_and_internal_fields_never_appear(self):
        s = snapshot()
        s['group_name'] = '<img src=x onerror=alert(1)>'
        s['model_id'] = '<script>window.pwned=1</script>'
        s['latest']['response_text'] = '<script>window.artwork_pwned=1</script>'
        s['internal_error'] = 'private-upstream-detail'
        source = self.build(s)
        doc = Document(source)
        self.assertNotIn('script', [tag for tag, _ in doc.tags])
        self.assertIn('&lt;img src=x', source)
        self.assertNotIn('window.artwork_pwned', source)
        self.assertNotIn('private-upstream-detail', source)

    def test_invalid_numeric_values_do_not_become_success_or_css(self):
        for bad in (True, float('nan'), float('inf'), -1, '<b>5</b>'):
            with self.subTest(value=bad):
                s = snapshot()
                s['rate_multiplier'] = bad
                s['statistics']['candy']['success_count'] = bad
                s['statistics']['pelican']['avg_latency_ms'] = bad
                doc = Document(self.build(s))
                self.assertIn('倍率 —', doc.text)
                self.assertIn('通过 —/—', doc.text)
                self.assertIn('异常 0 次 · 平均耗时 — 秒', doc.text)
                self.assertNotIn('nan', doc.text)
                self.assertNotIn('inf', doc.text)

    def test_building_report_does_not_mutate_snapshot(self):
        s = snapshot()
        original = deepcopy(s)
        self.build(s)
        self.assertEqual(s, original)


@unittest.skipUnless(importlib.util.find_spec('playwright') and
                     os.environ.get('PELICAN_TEST_CHROMIUM'),
                     'set PELICAN_TEST_CHROMIUM to a local Chromium executable')
class BrowserReportTemplateTests(unittest.TestCase):
    def test_v2_png_keeps_dpr_two_size_and_both_artwork_edges(self):
        s = snapshot()
        s['latest']['response_text'] = (
            '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 800 400">'
            '<rect width="800" height="400" fill="white"/>'
            '<rect width="100" height="400" fill="red"/>'
            '<rect x="700" width="100" height="400" fill="blue"/>'
            '<image href="http://127.0.0.1:1/forbidden"/>'
            '<script>while(true){}</script></svg>')
        png = asyncio.run(pelican_renderer.render_report_png(
            s, executable_path=os.environ['PELICAN_TEST_CHROMIUM'], timeout_seconds=60))
        self.assertTrue(png.startswith(b'\x89PNG\r\n\x1a\n'))
        self.assertEqual(struct.unpack('>II', png[16:24]), (896, 1220))
        from PIL import Image
        region = Image.open(io.BytesIO(png)).convert('RGB').crop((528, 376, 850, 596))
        pixels = list(region.get_flattened_data() if hasattr(region, 'get_flattened_data') else region.getdata())
        self.assertTrue(any(r > 230 and g < 30 and b < 30 for r, g, b in pixels))
        self.assertTrue(any(b > 230 and r < 30 and g < 30 for r, g, b in pixels))


if __name__ == '__main__':
    unittest.main()
