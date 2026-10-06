import asyncio
import base64
import importlib.util
import io
import os
import struct
import sys
import unittest
from copy import deepcopy
from html.parser import HTMLParser
from pathlib import Path

PLUGIN = Path(__file__).resolve().parents[1] / 'plugins/astrbot_plugin_xingqiao_ops'
sys.path.insert(0, str(PLUGIN))


def snapshot(**overrides):
    result = {
        'schema_version': 1, 'fetched_at': '2026-09-26T01:05:00+08:00',
        'group_id': 6, 'group_name': '【GPT】Plus 分组', 'platform': 'openai',
        'stats': {'success_count': 59, 'total_count': 60, 'success_rate': 98.333333},
        'stats_window': {'from': '2026-09-25T01:05:00+08:00',
                         'to': '2026-09-26T01:05:00+08:00',
                         'coverage_started_at': None, 'complete': True},
        'stats_status': 'available',
        'latest': {'id': 7, 'group_id': 6, 'model': 'gpt-6-astra',
                   'reasoning_effort': 'high', 'latency_ms': 91800,
                   'generated_at': '2026-09-25T17:01:47Z',
                   'response_text': '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 800 400"><rect width="800" height="400" fill="#fcfcfc"/><circle cx="400" cy="200" r="100" fill="#e8bd6b"/></svg>'},
        'artwork_status': 'available',
    }
    result.update(overrides)
    return result


class Document(HTMLParser):
    def __init__(self, html):
        super().__init__()
        self.tags = []
        self.parts = []
        self.feed(html)

    def handle_starttag(self, tag, attrs):
        self.tags.append((tag, dict(attrs)))

    def handle_data(self, data):
        self.parts.append(data)

    @property
    def text(self):
        return ' '.join(self.parts)


class RendererTests(unittest.TestCase):
    def renderer(self):
        self.assertTrue((PLUGIN / 'pelican_renderer.py').exists(),
                        'public snapshot renderer has not been implemented')
        import pelican_renderer
        return pelican_renderer

    def test_aggregate_counts_do_not_claim_per_attempt_history(self):
        doc = Document(self.renderer().build_report_html(snapshot()))
        self.assertIn('成功 59/60', doc.text)
        self.assertIn('98.3%', doc.text)
        self.assertIn('汇总比例', doc.text)
        self.assertIn('非逐次时间线', doc.text)
        for fake_claim in ('糖果逻辑', '倍率', '本轮完成', '平均耗时'):
            self.assertNotIn(fake_claim, doc.text)
        bars = [attrs for tag, attrs in doc.tags if attrs.get('role') == 'img'
                and '汇总' in attrs.get('aria-label', '')]
        self.assertEqual(len(bars), 1)

    def test_missing_data_is_not_zero_or_success(self):
        doc = Document(self.renderer().build_report_html(snapshot(
            stats=None, stats_window=None, stats_status='unavailable',
            latest=None, artwork_status='none')))
        self.assertIn('统计暂不可用', doc.text)
        self.assertIn('暂无作品', doc.text)
        self.assertNotIn('0/0', doc.text)
        self.assertNotIn('100.0%', doc.text)
        self.assertNotIn('0.0%', doc.text)

    def test_incomplete_and_stale_data_are_visible(self):
        for status, label in [('incomplete', '统计不完整'), ('stale', '统计已过期')]:
            with self.subTest(status=status):
                s = snapshot(stats_status=status, artwork_status='stale')
                doc = Document(self.renderer().build_report_html(s))
                self.assertIn(label, doc.text)
                self.assertIn('作品已过期', doc.text)
                self.assertIn('成功 59/60', doc.text)

    def test_false_complete_window_overrides_success_badge(self):
        s = snapshot()
        s['stats_window']['complete'] = False
        s['stats_window']['coverage_started_at'] = '2026-09-25T12:00:00+08:00'
        doc = Document(self.renderer().build_report_html(s))
        self.assertIn('统计不完整', doc.text)
        self.assertIn('09/25 12:00', doc.text)

    def test_generated_at_is_a_record_time_in_beijing_timezone(self):
        doc = Document(self.renderer().build_report_html(snapshot()))
        self.assertIn('作品记录时间', doc.text)
        self.assertIn('09/26 01:01:47', doc.text)
        self.assertIn('获取 09/26 01:05:00', doc.text)
        self.assertIn('91.8 秒', doc.text)

    def test_empty_complete_statistics_mean_no_records_not_unavailable(self):
        s = snapshot(stats=dict(success_count=0, total_count=0, success_rate=None))
        doc = Document(self.renderer().build_report_html(s))
        self.assertIn('暂无检测记录', doc.text)
        self.assertIn('统计可用', doc.text)
        self.assertIn('所选区间覆盖完整', doc.text)
        self.assertIn('—', doc.text)
        self.assertNotIn('统计暂不可用', doc.text)
        self.assertNotIn('0.0%', doc.text)
        self.assertNotIn('100.0%', doc.text)

    def test_empty_incomplete_window_keeps_coverage_warning_without_percentage(self):
        s = snapshot(stats=dict(success_count=0, total_count=0, success_rate=None))
        s['stats_window']['complete'] = False
        doc = Document(self.renderer().build_report_html(s))
        self.assertIn('暂无检测记录', doc.text)
        self.assertIn('统计不完整', doc.text)
        self.assertIn('覆盖范围未确认', doc.text)
        self.assertIn('—', doc.text)
        self.assertNotIn('统计暂不可用', doc.text)
        self.assertNotIn('0.0%', doc.text)

    def test_all_failed_nonempty_window_has_real_zero_percent(self):
        s = snapshot(stats=dict(success_count=0, total_count=3, success_rate=0))
        doc = Document(self.renderer().build_report_html(s))
        self.assertIn('成功 0/3', doc.text)
        self.assertIn('0.0%', doc.text)
        self.assertIn('未成功 3 次', doc.text)
        self.assertNotIn('暂无检测记录', doc.text)
        self.assertNotIn('统计暂不可用', doc.text)

    def test_invalid_counts_do_not_show_success_rate(self):
        for stats in [dict(success_count=9, total_count=2, success_rate=450),
                      dict(success_count=True, total_count=1, success_rate=100)]:
            with self.subTest(stats=stats):
                doc = Document(self.renderer().build_report_html(snapshot(stats=stats)))
                self.assertNotIn('100.0%', doc.text)
                self.assertNotIn('0.0%', doc.text)
                self.assertIn('暂无有效样本', doc.text)

    def test_untrusted_public_text_cannot_become_report_markup(self):
        s = snapshot(group_name='<img src=x onerror=alert(1)>')
        s['latest']['model'] = '<script>window.pwned=1</script>'
        s['latest']['response_text'] = '<script>window.artwork_pwned=1</script>'
        html = self.renderer().build_report_html(s)
        doc = Document(html)
        self.assertNotIn('script', [tag for tag, attrs in doc.tags])
        self.assertNotIn('window.artwork_pwned', html)
        self.assertIn('&lt;img src=x', html)
        self.assertTrue(all(attrs.get('src', '').startswith('data:image/png;base64,')
                            for tag, attrs in doc.tags if tag == 'img'))

    def test_animated_leg_path_is_frozen_before_animation_removal(self):
        source = '<svg><g><path stroke="#e6af57"><animate attributeName="d" values="M487 329L450 386L477 449;M487 329L509 383L541 449"/></path></g></svg>'
        doc = Document(self.renderer().build_artwork_document(source))
        path = next(attrs for tag, attrs in doc.tags if tag == 'path')
        self.assertEqual(path.get('d'), 'M487 329L450 386L477 449')
        self.assertNotIn('animate', [tag for tag, _ in doc.tags])

    def test_animation_cannot_reintroduce_active_attributes(self):
        source = '<svg><path><animate attributeName="onclick" values="alert(1)"/><animate attributeName="d" values="url(https://example.com)"/></path><image><animate attributeName="href" values="https://example.com"/></image></svg>'
        doc = Document(self.renderer().build_artwork_document(source))
        for tag, attrs in doc.tags:
            self.assertNotIn('onclick', attrs)
            if tag == 'path':
                self.assertNotIn('d', attrs)
            if tag == 'image':
                self.assertNotIn('href', attrs)

    def test_artwork_code_is_extracted_and_sandboxed(self):
        renderer = self.renderer()
        source = 'Here is the picture:\n```svg\n<svg viewBox="0 0 800 400"><rect width="800" height="400"/></svg>\n```'
        html = renderer.build_artwork_document(source)
        doc = Document(html)
        self.assertIn('svg', [tag for tag, attrs in doc.tags])
        self.assertNotIn('Here is the picture', doc.text)
        csp = next(attrs['content'] for tag, attrs in doc.tags
                   if tag == 'meta' and attrs.get('http-equiv') == 'Content-Security-Policy')
        self.assertIn("default-src 'none'", csp)
        self.assertIn("script-src 'none'", csp)

    def test_artwork_rejects_non_visual_and_oversized_payloads(self):
        renderer = self.renderer()
        for text in ['no image available', '<script>while(true){}</script>', 'x' * 300_001]:
            with self.subTest(size=len(text)):
                with self.assertRaises(ValueError):
                    renderer.build_artwork_document(text)

    def test_artwork_cannot_navigate_or_load_active_embedded_content(self):
        renderer = self.renderer()
        source = '<html><head><meta http-equiv="refresh" content="0; url=file:///etc/passwd"></head><body><svg viewBox="0 0 20 20"><rect width="20" height="20"/></svg><script>alert(1)</script><iframe src="file:///etc/passwd"></iframe><object data="http://secret"></object></body></html>'
        doc = Document(renderer.build_artwork_document(source))
        self.assertTrue({'script','iframe','object'}.isdisjoint({tag for tag, attrs in doc.tags}))
        self.assertFalse(any(tag == 'meta' and attrs.get('http-equiv', '').lower() == 'refresh'
                             for tag, attrs in doc.tags))

    def test_thumbnail_must_be_png_and_use_contain(self):
        renderer = self.renderer()
        with self.assertRaises(ValueError):
            renderer.build_report_html(snapshot(), b'<svg onload="alert(1)">')
        from PIL import Image
        buf = io.BytesIO()
        Image.new('RGB', (8, 4), '#ff8800').save(buf, format='PNG')
        html = renderer.build_report_html(snapshot(), buf.getvalue())
        self.assertIn(base64.b64encode(buf.getvalue()).decode('ascii'), html)
        self.assertIn('object-fit:contain', html.replace(' ', ''))

    def test_building_report_does_not_mutate_snapshot(self):
        s = snapshot()
        original = deepcopy(s)
        self.renderer().build_report_html(s)
        self.assertEqual(s, original)


@unittest.skipUnless(importlib.util.find_spec('playwright') and
                     os.environ.get('PELICAN_TEST_CHROMIUM'),
                     'set PELICAN_TEST_CHROMIUM to a local Chromium executable')
class BrowserRendererTests(unittest.TestCase):
    def test_render_is_fixed_dpr_two_png_and_blocks_remote_artwork(self):
        import pelican_renderer
        s = snapshot()
        s['latest']['response_text'] = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 800 400"><rect width="800" height="400" fill="#ffffff"/><rect width="100" height="400" fill="#ff0000"/><rect x="700" width="100" height="400" fill="#0000ff"/><image href="http://127.0.0.1:1/forbidden"/><script>while(true){}</script></svg>'
        png = asyncio.run(pelican_renderer.render_report_png(
            s, executable_path=os.environ['PELICAN_TEST_CHROMIUM'], timeout_seconds=60))
        self.assertTrue(png.startswith(b'\x89PNG\r\n\x1a\n'))
        self.assertEqual(struct.unpack('>II', png[16:24]), (896, 1220))
        from PIL import Image
        image = Image.open(io.BytesIO(png)).convert('RGB')
        # Both extreme artwork edges must survive contain fitting.
        region = image.crop((528, 376, 850, 596))
        pixels = list(region.getdata())
        self.assertTrue(any(r > 230 and g < 30 and b < 30 for r, g, b in pixels))
        self.assertTrue(any(b > 230 and r < 30 and g < 30 for r, g, b in pixels))

    def test_html_composition_keeps_far_edges_without_responsive_reflow(self):
        import pelican_renderer
        from playwright.async_api import async_playwright
        source = '<html><head><style>body{margin:0}main{position:relative;width:1600px;height:1000px;background:white}i{position:absolute;width:200px;height:200px;background:red;left:0;top:0}b{position:absolute;width:200px;height:200px;background:blue;right:0;bottom:0}@media(min-width:1300px){main{width:2048px;height:2048px}}</style></head><body><main><i></i><b></b></main></body></html>'
        async def render():
            async with async_playwright() as p:
                browser = await p.chromium.launch(headless=True, executable_path=os.environ['PELICAN_TEST_CHROMIUM'], timeout=20000)
                try:
                    return await pelican_renderer._render_artwork(browser, source)
                finally:
                    await browser.close()
        png = asyncio.run(render())
        from PIL import Image
        image = Image.open(io.BytesIO(png)).convert('RGB')
        self.assertGreater(image.width, image.height)
        pixels = list(image.getdata())
        self.assertTrue(any(r > 230 and g < 30 and b < 30 for r,g,b in pixels))
        self.assertTrue(any(b > 230 and r < 30 and g < 30 for r,g,b in pixels))

    def test_nested_svg_ignores_geometry_clipped_outside_its_viewport(self):
        import pelican_renderer
        from playwright.async_api import async_playwright
        source = ('<html><head><style>body{margin:0}main{width:800px;height:400px}'
                  'svg{display:block;width:800px;height:400px;overflow:hidden}</style></head>'
                  '<body><main><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 800 400">'
                  '<rect x="-10000" y="-10000" width="30000" height="30000" fill="white"/>'
                  '<rect width="100" height="400" fill="red"/>'
                  '<rect x="700" width="100" height="400" fill="blue"/>'
                  '</svg></main></body></html>')
        async def render():
            async with async_playwright() as p:
                browser = await p.chromium.launch(
                    headless=True, executable_path=os.environ['PELICAN_TEST_CHROMIUM'], timeout=20000,
                    args=['--disable-background-networking', '--host-resolver-rules=MAP * ~NOTFOUND'])
                try:
                    return await pelican_renderer._render_artwork(browser, source)
                finally:
                    await browser.close()
        try:
            png = asyncio.run(render())
        except ValueError as exc:
            self.fail(f'Visible nested SVG must produce a thumbnail: {exc}')
        from PIL import Image
        image = Image.open(io.BytesIO(png)).convert('RGB')
        self.assertEqual(image.size, (656, 328))
        self.assertEqual(image.getpixel((10, 164)), (255, 0, 0))
        self.assertEqual(image.getpixel((646, 164)), (0, 0, 255))

    def test_nested_svg_viewport_still_obeys_artwork_dimension_limit(self):
        import pelican_renderer
        from playwright.async_api import async_playwright
        source = ('<html><body><main><svg xmlns="http://www.w3.org/2000/svg" '
                  'viewBox="0 0 2049 400" style="display:block;width:2049px;height:400px">'
                  '<rect width="2049" height="400" fill="red"/></svg></main></body></html>')
        async def render():
            async with async_playwright() as p:
                browser = await p.chromium.launch(
                    headless=True, executable_path=os.environ['PELICAN_TEST_CHROMIUM'], timeout=20000,
                    args=['--disable-background-networking', '--host-resolver-rules=MAP * ~NOTFOUND'])
                try:
                    return await pelican_renderer._render_artwork(browser, source)
                finally:
                    await browser.close()
        with self.assertRaisesRegex(ValueError, 'artwork bounds'):
            asyncio.run(render())


if __name__ == '__main__':
    unittest.main()
