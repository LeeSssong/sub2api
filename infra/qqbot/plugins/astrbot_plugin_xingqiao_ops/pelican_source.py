"""Read existing showcase records through a deliberately narrow GET contract.

The injected getter returns an unwrapped public showcase DTO. Its owner must
use the public site and an Admin X-API-Key. These two admin read routes require
the separate upstream bridge; this module never logs in, falls back to internal
history, or starts a drawing/detection run when that bridge is unavailable.
"""

import math
import re
from datetime import datetime, timedelta, timezone


SHOWCASE_PATH = '/api/v1/admin/pelican-showcase'
_ITEM_PATH = re.compile(r'/api/v1/admin/pelican-showcase/items/[1-9][0-9]*')
_MAX_ID = 2**63 - 1
_CLOCK_SKEW = timedelta(seconds=5)


class PelicanSourceUnavailable(ValueError):
    """Safe, credential-free failure suitable for the bot's local logs."""


def is_allowed_source_path(path):
    """Allow only the list and one positive, signed-64-bit item identifier."""
    if not isinstance(path, str):
        return False
    if path.startswith('/api/v1/admin/pelican-reports/groups/'):
        if __package__:
            from .pelican_report_source import is_allowed_report_path
        else:
            from pelican_report_source import is_allowed_report_path
        return is_allowed_report_path(path)
    if path == SHOWCASE_PATH:
        return True
    if len(path) > len(SHOWCASE_PATH) + 27 or not _ITEM_PATH.fullmatch(path):
        return False
    return int(path.rsplit('/', 1)[1]) <= _MAX_ID


def _read(get, path):
    if not is_allowed_source_path(path):
        raise PelicanSourceUnavailable('pelican source path unavailable')
    try:
        payload = get(path)
    except Exception:
        raise PelicanSourceUnavailable('pelican source unavailable') from None
    if not isinstance(payload, dict):
        raise PelicanSourceUnavailable('pelican source unavailable')
    return payload


def _integer(value, minimum=0):
    return type(value) is int and minimum <= value <= _MAX_ID


def _text(value, maximum, empty=False):
    return (isinstance(value, str) and len(value) <= maximum
            and (empty or bool(value.strip())))


def _timestamp(value):
    if not isinstance(value, str) or len(value) > 64:
        return None
    try:
        stamp = datetime.fromisoformat(value.replace('Z', '+00:00'))
        if stamp.tzinfo is None or stamp.utcoffset() is None:
            return None
        return stamp.astimezone(timezone.utc)
    except (ValueError, OverflowError):
        return None


def _listing(get):
    payload = _read(get, SHOWCASE_PATH)
    if type(payload.get('enabled')) is not bool:
        raise PelicanSourceUnavailable('pelican source unavailable')
    if not payload['enabled']:
        return payload, []
    groups = payload.get('groups')
    if not isinstance(groups, list) or len(groups) > 500:
        raise PelicanSourceUnavailable('pelican source unavailable')
    seen = set()
    for group in groups:
        if (not isinstance(group, dict) or not _integer(group.get('id'), 1)
                or not _text(group.get('name'), 256)
                or not _text(group.get('platform'), 64)
                or group['id'] in seen):
            raise PelicanSourceUnavailable('pelican source unavailable')
        seen.add(group['id'])
    return payload, groups


def fetch_visible_groups(get):
    """Return only currently showcased group identifiers and public names."""
    _, groups = _listing(get)
    return [{'id': group['id'], 'name': group['name']} for group in groups]


def _window(value, now):
    if not isinstance(value, dict) or type(value.get('complete')) is not bool:
        return None
    start, end = _timestamp(value.get('from')), _timestamp(value.get('to'))
    coverage_value = value.get('coverage_started_at')
    coverage = _timestamp(coverage_value)
    if (start is None or end is None or end - start != timedelta(hours=24)
            or end > now + _CLOCK_SKEW
            or (coverage_value is not None and coverage is None)
            or (coverage is not None and coverage > end)):
        return None
    complete = value['complete']
    if complete != (coverage is not None and coverage <= start):
        return None
    return {'from': start.isoformat(), 'to': end.isoformat(),
            'coverage_started_at': coverage.isoformat() if coverage is not None else None,
            'complete': complete}


def _statistics(value):
    if not isinstance(value, dict) or 'success_rate' not in value:
        return None
    success, total, rate = (value.get(key) for key in
                            ('success_count', 'total_count', 'success_rate'))
    if not _integer(success) or not _integer(total) or success > total:
        return None
    if total == 0:
        if rate is not None:
            return None
    elif (type(rate) not in (int, float) or not 0 <= rate <= 100
          or not math.isclose(rate, success / total * 100, rel_tol=1e-9, abs_tol=1e-6)):
        return None
    return {'success_count': success, 'total_count': total, 'success_rate': rate}


def _metadata(item, group, now):
    if not isinstance(item, dict):
        return None
    stamp = _timestamp(item.get('generated_at'))
    if (not _integer(item.get('id'), 1) or item.get('group_id') != group['id']
            or not _integer(item.get('group_id'), 1)
            or not _text(item.get('model_id'), 256)
            or not _text(item.get('reasoning_effort'), 64, empty=True)
            or not _integer(item.get('latency_ms'))
            or stamp is None or stamp > now + _CLOCK_SKEW):
        return None
    return {'id': item['id'], 'group_id': group['id'], 'group_name': group['name'],
            'model': item['model_id'], 'reasoning_effort': item['reasoning_effort'],
            'latency_ms': item['latency_ms'], 'generated_at': stamp.isoformat()}


def _artwork(get, group, now, max_age_seconds):
    items = group.get('items')
    if not isinstance(items, list) or len(items) > 1000:
        return None, 'unavailable'
    if not items:
        return None, 'none'
    candidates = [record for item in items if (record := _metadata(item, group, now)) is not None]
    if not candidates:
        return None, 'unavailable'
    selected = max(candidates, key=lambda item: (item['generated_at'], item['id']))
    try:
        detail = _read(get, f"{SHOWCASE_PATH}/items/{selected['id']}")
    except PelicanSourceUnavailable:
        return None, 'unavailable'
    # A detail read cannot replace the listing's visible identity or provenance.
    if (_metadata(detail, group, now) != selected
            or not _text(detail.get('response_text'), 300000)):
        return None, 'unavailable'
    selected['response_text'] = detail['response_text']
    age = (now - _timestamp(selected['generated_at'])).total_seconds()
    return selected, 'stale' if age > max_age_seconds else 'available'


def load_report_snapshot(get, group_id, now=None, max_age_seconds=900, *,
                         artwork_max_age_seconds=86400):
    """Load one public group with independent statistics and artwork status.

    Counts come exclusively from that group's 24-hour statistics. The retained
    gallery length and top-level deduplicated aggregate never substitute for it.
    A missing bridge or hidden group fails closed; missing statistics/artwork
    remain distinct from a valid zero count or an empty gallery.
    """
    now = datetime.now(timezone.utc) if now is None else now
    if (not _integer(group_id, 1) or not isinstance(now, datetime)
            or now.tzinfo is None or now.utcoffset() is None):
        raise PelicanSourceUnavailable('pelican source request unavailable')
    for age in (max_age_seconds, artwork_max_age_seconds):
        if type(age) not in (int, float) or not 0 < age <= _MAX_ID:
            raise PelicanSourceUnavailable('pelican source request unavailable')
    now = now.astimezone(timezone.utc)
    payload, groups = _listing(get)
    group = next((group for group in groups if group['id'] == group_id), None)
    if group is None:
        raise PelicanSourceUnavailable('pelican showcase group unavailable')
    window = _window(payload.get('stats_window'), now)
    stats = _statistics(group.get('stats')) if window is not None else None
    stats_status = 'unavailable'
    if stats is not None:
        if (now - _timestamp(window['to'])).total_seconds() > max_age_seconds:
            stats_status = 'stale'
        else:
            stats_status = 'available' if window['complete'] else 'incomplete'
    latest, artwork_status = _artwork(get, group, now, artwork_max_age_seconds)
    return {'schema_version': 1, 'fetched_at': now.isoformat(),
            'group_id': group['id'], 'group_name': group['name'], 'platform': group['platform'],
            'stats': stats, 'stats_window': window, 'stats_status': stats_status,
            'latest': latest, 'artwork_status': artwork_status}
