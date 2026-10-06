"""Latest-card verdicts depend on actual output and the matching rendered artwork."""
import copy
import json
import unittest
from datetime import datetime, timezone
from pathlib import Path
from test_pelican_report_template import snapshot, thumbnail, Document
from pelican_renderer import build_report_html
import pelican_report_source as source

class ResultStatusTests(unittest.TestCase):
    def test_pelican_verdict_requires_output_and_matching_picture(self):
        for has_output, png, expected in [(True, thumbnail(), '通过'), (True, None, '不通过'), (False, None, '无数据'), (False, thumbnail(), '无数据')]:
            with self.subTest(has_output=has_output, picture=png is not None):
                s = snapshot()
                s['current'] = dict(candy=s['executions']['candy']['results'][0],
                    pelican=dict(s['executions']['pelican']['results'][0], has_output=has_output))
                doc = Document(build_report_html(s, png))
                self.assertIn('鹈鹕测试 ' + expected, doc.text)
                self.assertNotIn('以本次检测结果为准', doc.text)
                self.assertNotIn('上游维护', doc.text)

    def test_no_output_failure_does_not_reuse_previous_image(self):
        s = snapshot()
        old = s['executions']['pelican']['results'][0]
        s['current'] = dict(pelican=dict(old, result_id=999, status='failed', has_output=False))
        doc = Document(build_report_html(s, thumbnail()))
        self.assertIn('鹈鹕测试 无数据', doc.text)
        self.assertNotIn('本次检测异常', doc.text)
        self.assertNotIn('本次鹈鹕作品', str(doc.tags))

    def test_output_without_matching_picture_fails_even_if_request_succeeded(self):
        s = snapshot()
        s['current'] = dict(pelican=dict(s['executions']['pelican']['results'][0], result_id=999, has_output=True))
        doc = Document(build_report_html(s, thumbnail()))
        self.assertIn('鹈鹕测试 不通过', doc.text)

    def test_source_accepts_evaluated_candy_before_latest_transport_failure(self):
        raw = json.loads((Path(__file__).with_name('fixtures')/'pelican-report-v2.json').read_text())['report']
        candy = copy.deepcopy(raw['latest']['candy']['results'][0])
        raw['current'] = dict(candy=candy, pelican=dict(raw['latest']['pelican']['results'][0], has_output=True), artwork=raw['artwork'])
        failed = dict(candy, result_id=103, status='failed', execution_id='maintenance')
        raw['latest']['candy'].update(status='failed', execution_id='maintenance', results=[failed])
        raw['history']['candy'].append(failed)
        raw['history_meta']['candy'].update(total_count=2,returned_count=2)
        raw['statistics']['candy'].update(total_count=2, failure_count=1, observed_count=2, timed_count=2, success_rate=50, execution_count=2)
        value = source.load_detection_report_snapshot(lambda _: raw, 6, now=datetime(2026,9,26,7,5,tzinfo=timezone.utc))
        self.assertEqual(value['current']['candy']['result_id'],101)
        self.assertIs(value['current']['pelican']['has_output'], True)
        raw['current']['candy'] = None
        self.assertIsNone(source.load_detection_report_snapshot(lambda _: raw, 6, now=datetime(2026,9,26,7,5,tzinfo=timezone.utc))['current']['candy'])

    def test_source_keeps_empty_output_distinct_from_failed_output(self):
        for flag in (False, True):
            raw = json.loads((Path(__file__).with_name('fixtures')/'pelican-report-v2.json').read_text())['report']
            raw['current'] = dict(candy=raw['latest']['candy']['results'][0], pelican=dict(raw['latest']['pelican']['results'][0], has_output=flag), artwork=None)
            value = source.load_detection_report_snapshot(lambda _: raw, 6, now=datetime(2026,9,26,7,5,tzinfo=timezone.utc))
            self.assertIs(value['current']['pelican']['has_output'], flag)


class ArtworkPresenceTests(unittest.TestCase):
    def test_blank_or_text_only_document_is_not_a_picture(self):
        from pelican_renderer import build_artwork_document
        for content in ('<html><body><div>Here is your drawing</div></body></html>',
                        '<html><body><svg width="200" height="100"></svg></body></html>',
                        '<html><body><div></div></body></html>'):
            with self.subTest(content=content):
                with self.assertRaises(ValueError):
                    build_artwork_document(content)

import asyncio
import os
import importlib.util

@unittest.skipUnless(importlib.util.find_spec('playwright') and os.environ.get('PELICAN_TEST_CHROMIUM'), 'requires local Chromium')
class RenderedArtworkPresenceTests(unittest.TestCase):
    def test_invisible_svg_is_not_a_picture(self):
        from playwright.async_api import async_playwright
        from pelican_renderer import _render_artwork
        async def run():
            async with async_playwright() as p:
                browser = await p.chromium.launch(executable_path=os.environ['PELICAN_TEST_CHROMIUM'])
                try:
                    for doc in ('<html><body style="background:white"><p>There is no drawing</p></body></html>',
                                '<svg width="200" height="100"><circle cx="40" cy="40" r="30" opacity="0"/></svg>',
                                '<svg width="200" height="100"><rect width="200" height="100" fill="white"/></svg>'):
                        with self.subTest(doc=doc):
                            with self.assertRaises(ValueError):
                                await _render_artwork(browser, doc)
                finally:
                    await browser.close()
        asyncio.run(run())


if __name__ == "__main__":
    unittest.main()
