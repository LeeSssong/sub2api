"""Render public Pelican snapshots without any detector or authenticated calls.

The report is a trusted, static document. Model output is rasterized separately
in a fresh, offline browser context with JavaScript disabled; only the resulting
PNG crosses into the report. Script-only canvas output is intentionally unsupported.
Requires Pillow and Playwright with a locally installed Chromium executable.
"""
from __future__ import annotations

import asyncio
import base64
import html
import io
import math
import re
from datetime import datetime, timedelta, timezone
from html.parser import HTMLParser
from pathlib import Path
from string import Template
from typing import Any, Mapping

ASSETS = Path(__file__).with_name('report_assets')
REPORT_WIDTH, REPORT_HEIGHT, DEVICE_SCALE = 448, 610, 2
MAX_ARTWORK_BYTES = 300_000
MAX_ARTWORK_DIMENSION = 2048
BEIJING = timezone(timedelta(hours=8))
_CSP = ("default-src 'none'; script-src 'none'; connect-src 'none'; "
        "img-src data:; style-src 'unsafe-inline'; font-src data:; "
        "object-src 'none'; frame-src 'none'; base-uri 'none'; form-action 'none'")


def _text(value: Any, fallback: str = '暂无数据', limit: int = 160) -> str:
    if not isinstance(value, str) or not value.strip():
        return fallback
    return html.escape(value.strip()[:limit], quote=True)


def _time(value: Any, seconds: bool = True) -> str:
    try:
        parsed = datetime.fromisoformat(value.replace('Z', '+00:00'))
        if parsed.tzinfo is None:
            return '时间未知'
        return parsed.astimezone(BEIJING).strftime('%m/%d %H:%M:%S' if seconds else '%m/%d %H:%M')
    except (AttributeError, TypeError, ValueError, OverflowError):
        return '时间未知'


def _png_uri(data: bytes) -> str:
    from PIL import Image
    if not isinstance(data, bytes) or len(data) > 8_000_000 or not data.startswith(b'\x89PNG\r\n\x1a\n'):
        raise ValueError('thumbnail must be a bounded PNG')
    try:
        with Image.open(io.BytesIO(data)) as image:
            if image.format != 'PNG' or max(image.size) > 4096:
                raise ValueError('thumbnail dimensions exceed limit')
            image.verify()
    except (OSError, SyntaxError) as exc:
        raise ValueError('thumbnail is not a valid PNG') from exc
    return 'data:image/png;base64,' + base64.b64encode(data).decode('ascii')


def build_report_html(snapshot: Mapping[str, Any], thumbnail_png: bytes | None = None) -> str:
    """Return a self-contained trusted report. Never insert response_text here."""
    if snapshot.get('schema_version') == 2:
        if __package__:
            from .pelican_report_template import build_report_html as build_v2
        else:
            from pelican_report_template import build_report_html as build_v2
        return build_v2(snapshot, thumbnail_png)
    if snapshot.get('schema_version') != 1:
        raise ValueError('unsupported public snapshot schema')
    latest = snapshot.get('latest') or {}
    stats = snapshot.get('stats')
    window = snapshot.get('stats_window') or {}
    status = snapshot.get('stats_status', 'unavailable')
    if status == 'available' and window.get('complete') is False:
        status = 'incomplete'
    labels = {'available': '统计可用', 'incomplete': '统计不完整',
              'unavailable': '统计暂不可用', 'stale': '统计已过期'}
    stats_label = labels.get(status, labels['unavailable'])
    counts_ok = isinstance(stats, Mapping) and all(
        isinstance(stats.get(key), int) and not isinstance(stats.get(key), bool)
        for key in ('success_count', 'total_count'))
    if counts_ok:
        counts_ok = 0 <= stats['success_count'] <= stats['total_count']
    if counts_ok and stats['total_count'] == 0 and status in ('available', 'incomplete', 'stale'):
        count, rate = '暂无检测记录', '—'
        bar = '<div class="aggregate-bar empty" aria-hidden="true"></div>'
        note = '无样本 · 不计算成功率'
    elif counts_ok and status in ('available', 'incomplete', 'stale'):
        success, total = stats['success_count'], stats['total_count']
        percentage = success / total * 100
        count = f'成功 {success}/{total}'
        rate = f'{percentage:.1f}%'
        bar = (f'<div class="aggregate-bar" role="img" aria-label="成功汇总比例 {rate}，非逐次时间线">'
               f'<span style="width:{percentage:.6f}%"></span></div>')
        note = f'未成功 {total - success} 次 · 汇总比例，非逐次时间线'
    else:
        count, rate = '暂无有效样本', '—'
        bar = '<div class="aggregate-bar empty" aria-hidden="true"></div>'
        note = '公开统计暂不可用' if stats is None else '暂无有效样本 · 不计算成功率'
        if status == 'available':
            stats_label, status = '统计暂不可用', 'unavailable'
    artwork_status = snapshot.get('artwork_status', 'unavailable')
    artwork_labels = {'available': '作品可查看', 'unavailable': '作品暂不可用',
                      'stale': '作品已过期', 'none': '暂无作品'}
    artwork_label = artwork_labels.get(artwork_status, artwork_labels['unavailable'])
    if thumbnail_png is not None and latest and artwork_status in ('available', 'stale'):
        image = f'<img src="{_png_uri(thumbnail_png)}" alt="最近公开鹈鹕作品，完整画面缩放">'
    else:
        placeholder = '暂无作品' if artwork_status == 'none' else '预览暂不可用'
        image = f'<div class="artwork-empty"><svg class="icon"><use href="#i-image"/></svg><span>{placeholder}</span></div>'
    latency = latest.get('latency_ms')
    duration = (f'{latency / 1000:,.1f} 秒' if isinstance(latency, (int, float))
                and not isinstance(latency, bool) and math.isfinite(latency) and latency >= 0 else '暂无数据')
    period = (f"{_time(window.get('from'), False)} — {_time(window.get('to'), False)}"
              if window else '统计区间暂不可用')
    coverage = ('覆盖起点 ' + _time(window['coverage_started_at'], False)
                if window.get('coverage_started_at') else
                ('所选区间覆盖完整' if window.get('complete') is True else '覆盖范围未确认'))
    if status == 'stale':
        coverage = '统计已过期 · 仅供参考'
    elif status == 'unavailable':
        coverage = '请以公开页面后续更新为准'
    values = dict(
        csp=html.escape(_CSP, quote=True), css=(ASSETS / 'report.css').read_text(encoding='utf-8'),
        logo=_png_uri((ASSETS / 'xingqiao-mark.png').read_bytes()),
        group_name=_text(snapshot.get('group_name')), model=_text(latest.get('model')),
        fetched_at=_time(snapshot.get('fetched_at')), artwork_label=artwork_label,
        artwork_class='ok' if artwork_status == 'available' else 'warn',
        stats_label=stats_label, stats_class='ok' if status == 'available' else 'warn',
        thumbnail=image, count=count, rate=rate, bar=bar, note=note,
        period=period, coverage=coverage, generated_at=_time(latest.get('generated_at')),
        duration=duration, effort=_text(latest.get('reasoning_effort'), '未提供', 24),
    )
    return Template((ASSETS / 'report.html').read_text(encoding='utf-8')).substitute(values)


class _StaticArtwork(HTMLParser):
    # Disallow active documents, navigation, script execution, and animation.
    _blocked = {'script', 'iframe', 'object', 'embed', 'applet', 'canvas',
                'animate', 'animatetransform', 'animatemotion', 'set', 'noscript'}
    _discard = {'meta', 'link', 'base'}
    _visual = {'path', 'rect', 'circle', 'ellipse', 'line', 'polyline', 'polygon', 'image', 'img'}
    _paint = re.compile(r'(?:background(?:-color|-image)?|border(?:-[a-z]+)?|box-shadow)\s*:', re.I)

    def __init__(self):
        super().__init__(convert_charrefs=False)
        self.parts: list[str] = []
        self.skip: list[str] = []
        self.visual = False
        self.nodes = 0
        self.parents: list[tuple[str, int]] = []

    def _freeze_geometry(self, attrs):
        # Some SVG paths have no base geometry: their first SMIL frame supplies
        # the entire shape. Preserve only numeric geometry before dropping SMIL.
        if not self.parents:
            return
        parent, index = self.parents[-1]
        values = dict(attrs)
        attribute = values.get('attributename')
        if (parent, attribute) not in {('path', 'd'), ('polygon', 'points'), ('polyline', 'points')}:
            return
        if values.get('href') or values.get('xlink:href'):
            return
        value = (values.get('values') or values.get('from') or '').split(';', 1)[0].strip()
        allowed = r'[MmZzLlHhVvCcSsQqTtAaEe0-9+.,\s-]+' if attribute == 'd' else r'[Ee0-9+.,\s-]+'
        if not value or len(value) > 100_000 or not re.fullmatch(allowed, value):
            return
        start = self.parts[index]
        start = re.sub(r' ' + attribute + r'="[^"]*"', '', start)
        self.parts[index] = start[:-1] + f' {attribute}="{html.escape(value, quote=True)}">'

    def handle_starttag(self, tag, attrs):
        self.nodes += 1
        if self.nodes > 8000:
            raise ValueError('artwork node limit exceeded')
        if self.skip:
            if tag in self._blocked:
                self.skip.append(tag)
            return
        if tag in self._blocked:
            if tag == 'animate':
                self._freeze_geometry(attrs)
            if tag not in {'embed'}:
                self.skip.append(tag)
            return
        if tag in self._discard:
            return
        self.visual = self.visual or tag in self._visual or any(
            key == 'style' and self._paint.search(value or '') for key, value in attrs)
        safe_attrs = []
        for key, value in attrs:
            if key.startswith('on') or key in {'srcdoc', 'srcset', 'action', 'formaction', 'ping'}:
                continue
            if key in {'src', 'href', 'xlink:href'} and value and not (
                    value.startswith('#') or value.lower().startswith('data:image/')):
                continue
            safe_attrs.append(key if value is None else f'{key}="{html.escape(value, quote=True)}"')
        index = len(self.parts)
        self.parts.append('<' + tag + (' ' + ' '.join(safe_attrs) if safe_attrs else '') + '>')
        if tag not in {'area', 'br', 'col', 'hr', 'img', 'input', 'param', 'source', 'track', 'wbr'}:
            self.parents.append((tag, index))

    def handle_startendtag(self, tag, attrs):
        self.handle_starttag(tag, attrs)
        self.handle_endtag(tag)

    def handle_endtag(self, tag):
        if self.skip:
            if tag == self.skip[-1]:
                self.skip.pop()
            return
        if tag not in self._blocked | self._discard:
            self.parts.append(f'</{tag}>')
            for index in range(len(self.parents) - 1, -1, -1):
                if self.parents[index][0] == tag:
                    del self.parents[index:]
                    break

    def handle_data(self, data):
        if not self.skip:
            if self.parents and self.parents[-1][0] == 'style' and self._paint.search(data):
                self.visual = True
            self.parts.append(data)

    def handle_entityref(self, name):
        if not self.skip:
            self.parts.append(f'&{name};')

    def handle_charref(self, name):
        if not self.skip:
            self.parts.append(f'&#{name};')


def build_artwork_document(response_text: str) -> str:
    """Extract bounded static markup and add a restrictive offline CSP."""
    if not isinstance(response_text, str) or len(response_text.encode('utf-8')) > MAX_ARTWORK_BYTES:
        raise ValueError('artwork payload limit exceeded')
    fenced = re.search(r'```(?:html|svg|xml)?\s*([\s\S]*?)```', response_text, re.IGNORECASE)
    source = fenced.group(1).strip() if fenced else response_text.strip()
    if '<html' not in source.lower() and '<!doctype' not in source.lower():
        svg = re.search(r'<svg\b[\s\S]*?</svg\s*>', source, re.IGNORECASE)
        if svg:
            source = svg.group(0)
    parser = _StaticArtwork()
    parser.feed(source)
    parser.close()
    if not parser.visual:
        raise ValueError('no static artwork found')
    return ('<!doctype html><html><head><meta charset="utf-8">'
            '<meta http-equiv="Content-Security-Policy" content="' + html.escape(_CSP, quote=True) + '">'
            '<style>html,body{margin:0;padding:0}*{animation:none!important;transition:none!important;caret-color:transparent!important}</style>'
            '</head><body>' + ''.join(parser.parts) + '</body></html>')


async def _offline_context(browser, *, width=REPORT_WIDTH, height=REPORT_HEIGHT):
    context = await browser.new_context(viewport={'width': width, 'height': height},
                                        device_scale_factor=DEVICE_SCALE,
                                        java_script_enabled=False, service_workers='block',
                                        accept_downloads=False, offline=True,
                                        locale='zh-CN', timezone_id='Asia/Shanghai')
    await context.route('**/*', lambda route: route.abort())
    return context


async def _render_artwork(browser, response_text: str) -> bytes:
    document = build_artwork_document(response_text)
    context = await _offline_context(browser, width=1200, height=900)
    try:
        page = await context.new_page()
        page.set_default_timeout(3000)
        # srcdoc lives in an opaque sandbox; it has no origin, scripts or storage.
        await page.set_content('<!doctype html><html><body style="margin:0">'
                               '<iframe sandbox="" style="display:block;border:0;width:1200px;height:900px" srcdoc="'
                               + html.escape(document, quote=True) + '"></iframe></body></html>',
                               wait_until='load', timeout=3000)
        frame = page.frames[-1]
        has_graphic = await frame.evaluate('''() => {
            const visible = el => {
                const box = el.getBoundingClientRect();
                if (box.width <= 0 || box.height <= 0) return false;
                for (let node = el; node; node = node.parentElement) {
                    const css = getComputedStyle(node);
                    if (css.display === 'none' || css.visibility === 'hidden' || Number(css.opacity) === 0) return false;
                }
                return true;
            };
            return [...document.body.querySelectorAll('*')].some(el => {
                if (!visible(el)) return false;
                const tag = el.tagName.toLowerCase();
                if (['path','rect','circle','ellipse','line','polyline','polygon','image'].includes(tag) && el.ownerSVGElement) return true;
                if (tag === 'img') return el.complete && el.naturalWidth > 0;
                if (!['div','section','main','article','figure','span','i'].includes(tag)) return false;
                const css = getComputedStyle(el);
                return css.backgroundImage !== 'none' || css.boxShadow !== 'none'
                    || !['rgba(0, 0, 0, 0)','transparent'].includes(css.backgroundColor)
                    || ['Top','Right','Bottom','Left'].some(side => parseFloat(css['border'+side+'Width']) > 0);
            });
        }''')
        if not has_graphic:
            raise ValueError('artwork contains no visible graphic')
        bounds = await frame.evaluate('''() => {
            const svg = document.querySelector('body > svg');
            if (svg) {
                const box = svg.getBoundingClientRect();
                return {x:box.x,y:box.y,width:box.width,height:box.height};
            }
            const elements = [...document.body.querySelectorAll('*')];
            let left=0,top=0,right=0,bottom=0;
            for (const el of elements) {
                if (['STYLE','SCRIPT','HEAD','META'].includes(el.tagName)) continue;
                // The outer SVG owns its viewport; clipped internal geometry
                // can have bounds far outside the visible HTML composition.
                if (el.ownerSVGElement) continue;
                const r=el.getBoundingClientRect();
                if (!r.width || !r.height) continue;
                left=Math.min(left,r.left);top=Math.min(top,r.top);
                right=Math.max(right,r.right);bottom=Math.max(bottom,r.bottom);
            }
            return {x:left,y:top,width:right-left,height:bottom-top};
        }''')
        if (not all(isinstance(bounds.get(k), (int, float)) and math.isfinite(bounds[k])
                    for k in ('x', 'y', 'width', 'height'))
                or bounds['width'] <= 0 or bounds['height'] <= 0
                or max(bounds['width'], bounds['height']) > MAX_ARTWORK_DIMENSION):
            raise ValueError('artwork bounds are invalid or exceed limit')
        scale = min(1200 / bounds['width'], 900 / bounds['height'], 1)
        width, height = math.ceil(bounds['width'] * scale), math.ceil(bounds['height'] * scale)
        # Preserve the original viewport/media queries: scale every edge into it.
        await frame.evaluate('''({x,y,scale}) => {
            document.documentElement.style.setProperty('transform',`matrix(${scale},0,0,${scale},${-x*scale},${-y*scale})`);
            document.documentElement.style.setProperty('transform-origin','top left');
        }''', dict(bounds, scale=scale))
        raw = await page.screenshot(type='png', clip={'x': 0, 'y': 0, 'width': width, 'height': height},
                                    animations='disabled', timeout=3000)
        from PIL import Image
        with Image.open(io.BytesIO(raw)) as image:
            # A blank SVG/container can produce a valid PNG; it is still no picture.
            if all(low == high for low, high in image.convert('RGB').getextrema()):
                raise ValueError('artwork rendered an empty picture')
            image.thumbnail((656, 440), Image.Resampling.LANCZOS)
            output = io.BytesIO()
            image.save(output, format='PNG')
            return output.getvalue()
    finally:
        await asyncio.wait_for(context.close(), timeout=3)


async def _render_report(snapshot, executable_path):
    try:
        from playwright.async_api import async_playwright
    except ImportError as exc:
        raise RuntimeError('Pelican PNG rendering requires Playwright and installed Chromium') from exc
    async with async_playwright() as playwright:
        browser = await playwright.chromium.launch(
            headless=True, executable_path=executable_path, timeout=20000,
            args=['--disable-background-networking', '--disable-extensions',
                  '--disable-sync', '--no-first-run', '--host-resolver-rules=MAP * ~NOTFOUND'])
        try:
            thumbnail = None
            latest = snapshot.get('latest') or {}
            if latest and snapshot.get('artwork_status') in ('available', 'stale'):
                try:
                    thumbnail = await asyncio.wait_for(
                        _render_artwork(browser, latest.get('response_text', '')), timeout=6)
                except (Exception, asyncio.TimeoutError):
                    # Invalid/unsupported artwork must not hide valid public statistics.
                    thumbnail = None
            context = await _offline_context(browser)
            try:
                page = await context.new_page()
                await page.set_content(build_report_html(snapshot, thumbnail), wait_until='load', timeout=3000)
                return await page.screenshot(type='png', clip={'x': 0, 'y': 0, 'width': REPORT_WIDTH,
                                                              'height': REPORT_HEIGHT},
                                             animations='disabled', timeout=3000)
            finally:
                await asyncio.wait_for(context.close(), timeout=3)
        finally:
            await asyncio.wait_for(browser.close(), timeout=3)


async def render_report_png(snapshot: Mapping[str, Any], *, timeout_seconds: float = 45,
                            executable_path: str | None = None) -> bytes:
    """Render a 896×1220 PNG within a bounded overall deadline.

    A fresh browser is used per report: there are no login sessions, credentials,
    persistent profiles, network requests, report scripts, or model/API calls.
    Chromium uses Playwright's installed executable unless explicitly supplied.
    """
    if not isinstance(timeout_seconds, (int, float)) or not math.isfinite(timeout_seconds) or timeout_seconds <= 0:
        raise ValueError('timeout_seconds must be finite and positive')
    return await asyncio.wait_for(_render_report(snapshot, executable_path), timeout=timeout_seconds)
