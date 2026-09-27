"""A bounded, read-only usage snapshot and QQ image renderer."""
import io
import math
import os
import re
import tempfile
import time
from datetime import datetime, timedelta, timezone
from pathlib import Path
from statistics import median
from urllib.parse import urlencode

BEIJING = timezone(timedelta(hours=8))
MIN_CACHE_SAMPLES = 20
REFRESH_INTERVAL = 3600
CACHE_MAX_AGE = 3900


def write_cached_card(path, image, snapshot_timestamp):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, temp = tempfile.mkstemp(prefix='.latest-', suffix='.tmp', dir=path.parent)
    try:
        with os.fdopen(fd, 'wb') as stream:
            stream.write(image)
            stream.flush()
            os.fsync(stream.fileno())
        os.utime(temp, (snapshot_timestamp, snapshot_timestamp))
        os.replace(temp, path)
    finally:
        Path(temp).unlink(missing_ok=True)


def next_refresh_delay(now=None):
    now = time.time() if now is None else now
    return REFRESH_INTERVAL - now % REFRESH_INTERVAL


def read_cached_card(path, max_age=CACHE_MAX_AGE):
    # Use the same opened inode for age and bytes while a writer replaces the path.
    with Path(path).open('rb') as stream:
        age = time.time() - os.fstat(stream.fileno()).st_mtime
        if age < -5 or age > max_age:
            raise ValueError('performance cache expired')
        image = stream.read()
        if not image:
            raise ValueError('performance cache empty')
        return image


def is_performance_query(text):
    if any(word in text for word in ('不要', '别查', '优化', '代码', '怎么做', '如何', '实现', '设计')):
        return False
    text = re.sub(r'@小星(?:[（(][^）)]*[）)])?', '', text).strip()
    return bool(re.fullmatch(r'(?:查一下|看看|查|看|告诉我|告诉|发一下|发|现在|当前|目前|的|一下|你们|我们|分组|GPT|gpt|Plus|plus|Pro|pro|特惠|性能|怎么样|如何|咋样|好不好|情况|表现|数据|卡片|图|\s|[？?，,。！!])+', text)) and '性能' in text


def number(value):
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        return None
    return value if math.isfinite(value) and value >= 0 else None


def summarize(rows, start, end):
    selected = []
    for row in rows:
        stamp = datetime.fromisoformat(row['created_at'].replace('Z', '+00:00'))
        if stamp.tzinfo is None:
            raise ValueError('timestamp missing timezone')
        if start <= stamp < end:
            selected.append(row)
    def latency(key):
        values = [number(row.get(key)) for row in selected]
        values = [v for v in values if v is not None]
        return f'{median(values)/1000:.1f} s' if values else '暂无数据'
    hits = total = samples = 0
    for row in selected:
        values = [number(row.get(k)) for k in ('input_tokens', 'cache_read_tokens', 'cache_creation_tokens')]
        if any(v is None for v in values) or sum(values) <= 0:
            continue
        # Sub2API usage input_tokens excludes cached reads (verified on live records).
        hits += values[1]
        total += sum(values)
        samples += 1
    cache = ('暂无数据' if not selected else '样本不足')
    if samples >= MIN_CACHE_SAMPLES and total:
        cache = f'{hits/total*100:.1f}%'
    return dict(first=latency('first_token_ms'), duration=latency('duration_ms'), cache=cache)


def load_snapshot(get, end=None):
    end = end or datetime.now(timezone.utc)
    start = end - timedelta(hours=1)
    deadline = time.monotonic() + 45
    def pages(path, params, cutoff=None):
        rows, seen = [], set()
        previous_stamp = None
        for page in range(1, 101):
            if time.monotonic() > deadline:
                raise ValueError('snapshot deadline exceeded')
            data = get(path + '?' + urlencode(dict(params, page=page, page_size=50)))
            if not isinstance(data, dict) or not isinstance(data.get('items'), list):
                raise ValueError('snapshot API unavailable')
            items = data['items']
            for row in items:
                identity = row.get('id')
                if identity in seen:
                    # New usage rows can arrive while descending offset pages are
                    # being read, shifting boundary rows onto the next page.
                    # Ignore those repeats and keep paging toward the fixed cutoff.
                    continue
                seen.add(identity)
                rows.append(row)
                if cutoff is not None:
                    stamp = datetime.fromisoformat(row['created_at'].replace('Z', '+00:00'))
                    if stamp.tzinfo is None or (previous_stamp is not None and stamp > previous_stamp):
                        raise ValueError('usage ordering not honored')
                    previous_stamp = stamp
            if cutoff is not None and previous_stamp is not None and previous_stamp < cutoff:
                return rows
            count = data.get('pages')
            if isinstance(count, int) and page >= count:
                return rows
            if count is None and len(items) < 50:
                return rows
        raise ValueError('snapshot exceeds page limit')
    groups = pages('/api/v1/admin/groups', {})
    groups = sorted(
        (group for group in groups
         if group.get('platform') == 'openai'
         and group.get('status') == 'active'
         and group.get('is_exclusive') is False),
        key=lambda group: (group.get('sort_order') or 0, group.get('id') or 0),
    )
    output = []
    for group in groups:
        rows = []
        # Query both local dates for midnight crossings, then enforce exact bounds locally.
        rows = pages('/api/v1/admin/usage', dict(
            group_id=group['id'], start_date=start.astimezone(BEIJING).strftime('%Y-%m-%d'),
            end_date=end.astimezone(BEIJING).strftime('%Y-%m-%d'),
            timezone='Asia/Shanghai', sort_by='created_at', sort_order='desc', exact_total='true'), cutoff=start)
        if any(r.get('group_id') != group['id'] for r in rows):
            raise ValueError('group filter not honored')
        rate = number(group.get('rate_multiplier'))
        output.append(dict(name=group['name'], rate=f'{rate:g}×' if rate is not None else '暂无数据',
                           **summarize(rows, start, end)))
    return dict(start=start, end=end, groups=output)


def render_card(snapshot):
    from PIL import Image, ImageDraw, ImageFont
    regular_candidates = ['/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc',
                          '/System/Library/Fonts/STHeiti Medium.ttc']
    bold_candidates = ['/usr/share/fonts/opentype/noto/NotoSansCJK-Bold.ttc',
                       '/System/Library/Fonts/STHeiti Medium.ttc']
    regular_path = next((p for p in regular_candidates if Path(p).exists()), None)
    bold_path = next((p for p in bold_candidates if Path(p).exists()), regular_path)
    if not regular_path:
        raise ValueError('Chinese font unavailable')

    row_count = len(snapshot['groups'])
    height = 720 + max(0, row_count - 3) * 112
    im = Image.new('RGB', (1200, height), '#181c23')
    draw = ImageDraw.Draw(im)
    draw.rounded_rectangle((24, 20, 1176, height - 20), 16, fill='#0c1016',
                           outline='#29313e', width=2)

    fonts = {}
    def font(size, bold=False):
        key = (size, bold)
        if key not in fonts:
            fonts[key] = ImageFont.truetype(bold_path if bold else regular_path, size)
        return fonts[key]

    def text(x, y, value, size=24, color='#edf1f7', bold=False, anchor='la'):
        draw.text((x, y), value, font=font(size, bold), fill=color, anchor=anchor)

    text(66, 72, '星桥', 34, '#edf4ff', True, 'lm')
    draw.line((158, 48, 158, 96), fill='#38414d', width=2)
    text(184, 72, '当前性能', 34, '#edf1f7', True, 'lm')

    draw.rounded_rectangle((1010, 47, 1134, 97), 7, fill='#0c1016',
                           outline='#384655', width=2)
    text(1072, 72, '近 1 小时', 20, '#accee6', anchor='mm')

    start, end = [snapshot[k].astimezone(BEIJING) for k in ('start', 'end')]
    end_label = end.strftime('%H:%M') if start.date() == end.date() else end.strftime('%m/%d %H:%M')
    text(66, 132, f'{start:%m/%d %H:%M}–{end_label} · 北京时间', 20, '#a8b2c0')

    columns = ((66, '分组', 'la'), (500, '倍率', 'ra'), (690, '首字', 'ra'),
               (900, '耗时', 'ra'), (1134, '缓存命中', 'ra'))
    for x, label, anchor in columns:
        text(x, 220, label, 19, '#909fb1', anchor=anchor)
    draw.line((66, 252, 1134, 252), fill='#2b3340', width=2)

    def metric(x, y, value, accent=False):
        color = '#94c8ed' if accent else '#edf1f7'
        if value in ('暂无数据', '样本不足'):
            text(x, y, value, 22, '#7f8c9c', anchor='rm')
            return
        match = re.fullmatch(r'([0-9.]+)\s*(.*)', value)
        if not match:
            text(x, y, value, 29, color, True, 'rm')
            return
        number_part, unit_part = match.groups()
        if unit_part:
            unit_font = font(20)
            unit_width = draw.textlength(unit_part, font=unit_font)
            text(x, y + 3, unit_part, 20, '#a6b3c3', anchor='rm')
            text(x - unit_width - 5, y, number_part, 30, color, True, 'rm')
        else:
            text(x, y, number_part, 30, color, True, 'rm')

    for i, row in enumerate(snapshot['groups']):
        y = 318 + 112*i
        text(66, y, row['name'], 28, '#edf1f7', True, 'lm')
        metric(500, y, row['rate'], True)
        metric(690, y, row['first'])
        metric(900, y, row['duration'])
        metric(1134, y, row['cache'])
        draw.line((66, y + 55, 1134, y + 55), fill='#232b35', width=2)

    text(66, height - 70, '每整点更新', 16, '#8896a8')
    text(66, height - 43, 'api.xingqiaolab.top', 16, '#77899f')

    out = io.BytesIO()
    im.save(out, format='PNG')
    return out.getvalue()
