"""Static v2 presentation for validated read-only results; no fetching or sending.

Model output never enters this document. Only its separately sandboxed PNG does.
"""
from __future__ import annotations

import html
import math
from datetime import datetime, timedelta, timezone
from pathlib import Path
from string import Template
from typing import Any, Mapping

ASSETS = Path(__file__).with_name('report_assets')
BEIJING = timezone(timedelta(hours=8))


def _mapping(value: Any) -> Mapping[str, Any]:
    return value if isinstance(value, Mapping) else {}


def _text(value: Any, limit: int = 160) -> str:
    if not isinstance(value, str) or not value.strip():
        return '—'
    return html.escape(value.strip()[:limit], quote=True)


def _number(value: Any) -> bool:
    return (isinstance(value, (int, float)) and not isinstance(value, bool)
            and math.isfinite(value) and value >= 0)


def _count(value: Any) -> bool:
    return isinstance(value, int) and not isinstance(value, bool) and value >= 0


def _time(value: Any) -> str:
    try:
        parsed = datetime.fromisoformat(value.replace('Z', '+00:00'))
        if parsed.tzinfo is None:
            return '—'
        return parsed.astimezone(BEIJING).strftime('%m/%d %H:%M:%S')
    except (AttributeError, TypeError, ValueError, OverflowError):
        return '—'


def _duration(value: Any, *, average: bool = False) -> str:
    if not _number(value):
        return '—'
    seconds = value / 1000
    return f'{seconds:,.1f}' if average else f'{seconds:,.1f}'.removesuffix('.0')


def _statistics(value: Any) -> Mapping[str, Any]:
    stats = _mapping(value)
    if not all(_count(stats.get(key)) for key in
               ('success_count', 'total_count', 'failure_count',
                'ungraded_count', 'observed_count', 'timed_count')):
        return {}
    if (stats['success_count'] + stats['failure_count'] != stats['total_count']
            or stats['total_count'] + stats['ungraded_count'] != stats['observed_count']
            or stats['timed_count'] > stats['total_count']):
        return {}
    return stats


def _badge(execution: Any, *, candy: bool) -> str:
    status = _mapping(execution).get('status')
    if status == 'success':
        label, css, symbol = '通过', 'ok', 'i-check'
    elif status == 'failed':
        label, css, symbol = ('异常' if candy else '不通过'), 'failed', 'i-alert'
    else:
        return '<span class="badge missing">无数据</span>' if not candy else '<span class="badge missing">—</span>'
    return (f'<span class="badge {css}"><svg class="icon" aria-hidden="true">'
            f'<use href="#{symbol}"/></svg>{label}</span>')


def _history(rows: Any, *, candy: bool) -> str:
    if not isinstance(rows, list) or not rows:
        return '<div class="ticks empty" aria-label="—"></div>'
    # Preserve one mark per supplied result. Never fabricate a timeline from
    # percentages, discard neutral observations, or cap the actual record list.
    marks = []
    for value in rows:
        row = _mapping(value)
        status = row.get('status')
        if status not in ('success', 'failed', 'ungraded'):
            continue
        label = ('通过' if candy else '成功') if status == 'success' else ('异常' if status == 'failed' else '—')
        title = f"{_time(row.get('completed_at'))} · {label}"
        marks.append(f'<span class="tick {status}" data-status="{status}" '
                     f'title="{title}" aria-label="{title}"></span>')
    if not marks:
        return '<div class="ticks empty" aria-label="—"></div>'
    count = len(marks)
    gap = min(1.6, 180 / count)
    return (f'<div class="ticks" role="img" aria-label="{count} 次检测记录，按时间排列" '
            f'style="--record-count:{count};--tick-gap:{gap:.3f}px">'
            + ''.join(marks) + '</div>')


def _panel(stats: Mapping[str, Any], history: Any, *, candy: bool) -> dict[str, str]:
    prefix = '通过' if candy else '成功'
    if stats:
        total, success = stats['total_count'], stats['success_count']
        count = f'{prefix} {success}/{total}'
        rate = f'{success / total * 100:.1f}%' if total else '—'
        failure = str(stats['failure_count'])
        average = _duration(stats.get('avg_latency_ms'), average=True) if stats['timed_count'] else '—'
    else:
        count, rate, failure, average = f'{prefix} —/—', '—', '—', '—'
    return dict(count=count, rate=rate, rate_class='missing-value' if rate == '—' else '',
                note=f'异常 {failure} 次 · 平均耗时 {average} 秒',
                history=_history(history, candy=candy))


def _current_result(snapshot: Mapping[str, Any], kind: str) -> Mapping[str, Any]:
    if 'current' in snapshot:
        return _mapping(_mapping(snapshot.get('current')).get(kind))
    rows = _mapping(_mapping(snapshot.get('executions')).get(kind)).get('results') or []
    return _mapping(rows[-1]) if rows else {}


def _current_artwork(snapshot: Mapping[str, Any]) -> bool:
    latest = _mapping(snapshot.get('latest'))
    result = _current_result(snapshot, 'pelican')
    if (not latest or snapshot.get('artwork_status') not in ('available', 'stale')
            or result.get('status') != 'success'
            or latest.get('execution_id') != result.get('execution_id')):
        return False
    return latest.get('source_result_id') == result.get('result_id')


def build_report_html(snapshot: Mapping[str, Any], thumbnail_png: bytes | None = None) -> str:
    """Build a v2 report using public fields and truthful missing-value markers."""
    if snapshot.get('schema_version') != 2:
        raise ValueError('unsupported result report schema')
    # Resolve at call time: the shared renderer dispatches to this module.
    if __package__:
        from .pelican_renderer import _CSP, _png_uri
    else:
        from pelican_renderer import _CSP, _png_uri
    statistics = _mapping(snapshot.get('statistics'))
    histories = _mapping(snapshot.get('history'))
    candy_stats, pelican_stats = (_statistics(statistics.get(kind)) for kind in ('candy', 'pelican'))
    candy = _panel(candy_stats, histories.get('candy'), candy=True)
    pelican = _panel(pelican_stats, histories.get('pelican'), candy=False)
    # Per-kind executions do not prove complete shared batches across a window.
    observed = (str(candy_stats['observed_count'] + pelican_stats['observed_count'])
                if candy_stats and pelican_stats else '—')
    rate = snapshot.get('rate_multiplier')
    multiplier = f'{rate:g}x' if _number(rate) else '—'
    current_pelican = _current_result(snapshot, 'pelican')
    duration = _duration(current_pelican.get('latency_ms'))
    # An HTTP success alone is not a picture. Only the selected result's
    # successfully rendered thumbnail can pass; never reuse an older artwork.
    has_output = current_pelican.get('has_output')
    if has_output is None:
        # Older servers lack the flag: a completed success proves output;
        # failed requests without output evidence remain no-data.
        has_output = current_pelican.get('status') == 'success'
    has_picture = thumbnail_png is not None and _current_artwork(snapshot)
    verdict = 'no_data' if not has_output else 'success' if has_picture else 'failed'
    if verdict == 'success':
        image = f'<img src="{_png_uri(thumbnail_png)}" alt="本次鹈鹕作品，完整画面缩放">'
    elif verdict == 'failed':
        image = ('<div class="artwork-empty artwork-failed"><svg class="icon" aria-hidden="true">'
                 '<use href="#i-alert"/></svg><strong>不通过</strong></div>')
    else:
        image = ('<div class="artwork-empty"><svg class="icon" aria-hidden="true">'
                 '<use href="#i-image"/></svg><span>无数据</span></div>')
    values = dict(
        csp=html.escape(_CSP, quote=True), css=(ASSETS / 'report-v2.css').read_text(encoding='utf-8'),
        logo=_png_uri((ASSETS / 'xingqiao-mark.png').read_bytes()),
        group_name=_text(snapshot.get('group_name')), model=_text(snapshot.get('model_id')),
        as_of=_time(snapshot.get('as_of')), multiplier=multiplier,
        fixture_notice=('<div class="fixture-notice">【示例数据】离线自测</div>'
                        if snapshot.get('offline_fixture') is True else ''),
        multiplier_class='missing-value' if multiplier == '—' else '',
        candy_badge=_badge(_current_result(snapshot, 'candy'), candy=True),
        pelican_badge=_badge({'status': verdict}, candy=False),
        thumbnail=image, observed=observed,
        artwork_caption='本次作品截图',
        completed_at=_time(current_pelican.get('completed_at')),
        duration='—' if duration == '—' else duration + ' 秒',
    )
    values.update({f'candy_{key}': value for key, value in candy.items()})
    values.update({f'pelican_{key}': value for key, value in pelican.items()})
    return Template((ASSETS / 'report-v2.html').read_text(encoding='utf-8')).substitute(values)
