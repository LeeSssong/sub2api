"""Validate one authenticated public report snapshot; no execution or fallback."""
from __future__ import annotations

import math
import re
from datetime import datetime, timedelta, timezone
from urllib.parse import parse_qsl, urlencode, urlsplit

if __package__:
    from .pelican_source import PelicanSourceUnavailable, _integer, _text, _timestamp
else:
    from pelican_source import PelicanSourceUnavailable, _integer, _text, _timestamp

REPORT_BASE = '/api/v1/admin/pelican-reports/groups/'
_PATH = re.compile(re.escape(REPORT_BASE) + r'([1-9][0-9]*)')
_KINDS = ('candy', 'pelican')
_ROW_FIELDS = ('result_id', 'execution_id', 'shared_round_id', 'scheduled_for',
               'status', 'judgment', 'started_at', 'completed_at', 'latency_ms')
_COUNT_FIELDS = ('success_count', 'total_count', 'failure_count', 'ungraded_count',
                 'observed_count', 'timed_count')

def _model(value):
    return (_text(value, 100) and len(value.encode('utf-8')) <= 100
            and not any(ord(c) < 32 for c in value))

def is_allowed_report_path(path):
    if not isinstance(path, str) or len(path) > 1600:
        return False
    try:
        parsed = urlsplit(path)
        match = _PATH.fullmatch(parsed.path)
        if parsed.scheme or parsed.netloc or parsed.fragment or not match:
            return False
        if not _integer(int(match.group(1)), 1):
            return False
        pairs = parse_qsl(parsed.query, keep_blank_values=True, strict_parsing=True)
        values = dict(pairs)
        if len(values) != len(pairs) or set(values) - {'model_id', 'window'}:
            return False
        if 'window' in values and values['window'] != '24h':
            return False
        model = values.get('model_id')
        return model is None or _model(model)
    except (ValueError, UnicodeError):
        return False

def report_path(group_id, model_id=None):
    if not _integer(group_id, 1):
        raise ValueError('invalid report group')
    query = {'window': '24h'}
    if model_id is not None:
        if not _model(model_id):
            raise ValueError('invalid report model')
        query['model_id'] = model_id
    return REPORT_BASE + str(group_id) + '?' + urlencode(query)

def _require(condition):
    if not condition:
        raise ValueError('invalid public report contract')

def _number(value):
    return type(value) in (float, int) and math.isfinite(value) and value >= 0

def _nullable_id(value):
    _require(value is None or _text(value, 200))
    return value

def _stamp(value, as_of):
    if value is None:
        return None
    parsed = _timestamp(value)
    _require(parsed is not None and parsed <= as_of + timedelta(seconds=5))
    return parsed.isoformat()

def _duration(start, end, value):
    if start is None or end is None:
        _require(value is None)
        return None
    milliseconds = (_timestamp(end) - _timestamp(start)).total_seconds() * 1000
    if milliseconds < 0:
        _require(value is None)
        return None
    _require(_number(value) and abs(value - int(milliseconds)) <= 1)
    return value

def _record(raw, kind, as_of):
    _require(isinstance(raw, dict) and all(key in raw for key in _ROW_FIELDS))
    row = {key: raw[key] for key in _ROW_FIELDS}
    _require(_integer(row['result_id'], 1))
    for key in ('execution_id', 'shared_round_id'):
        _nullable_id(row[key])
    _require(row['status'] in ('success', 'failed', 'ungraded'))
    if kind == 'pelican':
        _require(row['judgment'] == 'drawing' and row['status'] != 'ungraded')
    else:
        _require(row['judgment'] in ('builtin_candy', 'ungraded_candy'))
        _require(not (row['judgment'] == 'ungraded_candy' and row['status'] == 'success'))
        _require(not (row['judgment'] == 'builtin_candy' and row['status'] == 'ungraded'))
    for key in ('scheduled_for', 'started_at', 'completed_at'):
        row[key] = _stamp(row[key], as_of)
    _duration(row['started_at'], row['completed_at'], row['latency_ms'])
    if row['shared_round_id'] is not None:
        _require(row['execution_id'] is not None and row['scheduled_for'] is not None)
    return row

def _stats(raw, rows, meta):
    keys = _COUNT_FIELDS + ('success_rate', 'avg_latency_ms', 'execution_count')
    _require(isinstance(raw, dict) and all(key in raw for key in keys))
    result = {key: raw[key] for key in keys}
    _require(all(_integer(result[key]) for key in _COUNT_FIELDS))
    _require(isinstance(meta, dict) and meta.get('limit') == 240)
    _require(_integer(meta.get('total_count')) and _integer(meta.get('returned_count')))
    _require(meta['total_count'] == result['observed_count'])
    _require(meta['returned_count'] == len(rows) == min(240, result['observed_count']))
    complete = meta['total_count'] == len(rows)
    successful = sum(row['status'] == 'success' for row in rows)
    failed = sum(row['status'] == 'failed' for row in rows)
    ungraded = sum(row['status'] == 'ungraded' for row in rows)
    _require(result['success_count'] + result['failure_count'] == result['total_count'])
    _require(result['total_count'] + result['ungraded_count'] == result['observed_count'])
    for field, actual in (('success_count', successful), ('failure_count', failed), ('ungraded_count', ungraded)):
        _require(result[field] == actual if complete else result[field] >= actual)
    total, rate = result['total_count'], result['success_rate']
    _require(rate is None if total == 0 else _number(rate) and
             math.isclose(rate, result['success_count'] / total * 100, rel_tol=1e-9, abs_tol=1e-6))
    timed = [row['latency_ms'] for row in rows if row['status'] != 'ungraded' and row['latency_ms'] is not None]
    _require(result['timed_count'] <= total)
    _require(result['timed_count'] == len(timed) if complete else result['timed_count'] >= len(timed))
    average = result['avg_latency_ms']
    if result['timed_count'] == 0:
        _require(average is None)
    else:
        _require(_number(average))
        if complete:
            _require(math.isclose(average, sum(timed) / len(timed), rel_tol=1e-9, abs_tol=1e-6))
        else:
            _require(average * result['timed_count'] + 1e-6 >= sum(timed))
    executions = result['execution_count']
    if any(row['execution_id'] is None for row in rows):
        _require(executions is None)
    elif complete:
        _require(_integer(executions) and executions == len({row['execution_id'] for row in rows}))
    elif executions is not None:
        _require(_integer(executions) and len({row['execution_id'] for row in rows}) <= executions <= result['observed_count'])
    return result

def _execution(raw, kind, history, as_of):
    if raw is None:
        _require(not history)
        return None
    _require(isinstance(raw, dict))
    keys = ('execution_id', 'shared_round_id', 'scheduled_for', 'status',
            'expected_count', 'completed_count', 'started_at', 'completed_at', 'latency_ms')
    _require(all(key in raw for key in keys) and isinstance(raw.get('results'), list))
    result = {key: raw[key] for key in keys}
    rows = [_record(row, kind, as_of) for row in raw['results']]
    _require(0 < len(rows) <= 8 and len({r['result_id'] for r in rows}) == len(rows))
    result['results'] = rows
    _nullable_id(result['execution_id'])
    _nullable_id(result['shared_round_id'])
    for key in ('scheduled_for', 'started_at', 'completed_at'):
        result[key] = _stamp(result[key], as_of)
    _require(_integer(result['completed_count'], 1) and result['completed_count'] == len(rows))
    expected = result['expected_count']
    if result['execution_id'] is None:
        _require(expected is None and len(rows) == 1 and result['shared_round_id'] is None)
    else:
        _require(_integer(expected, 1) and len(rows) <= expected <= 8)
    for row in rows:
        for key in ('execution_id', 'shared_round_id', 'scheduled_for'):
            _require(row[key] == result[key])
    existing = {row['result_id']: row for row in history}
    for row in rows:
        if row['result_id'] in existing:
            _require(existing[row['result_id']] == row)
    complete = expected is None or expected == len(rows)
    status = ('pending' if not complete else 'failed' if any(r['status'] == 'failed' for r in rows)
              else 'ungraded' if any(r['status'] == 'ungraded' for r in rows) else 'success')
    _require(result['status'] == status)
    if not complete:
        _require(result['completed_at'] is None and result['latency_ms'] is None)
    else:
        _duration(result['started_at'], result['completed_at'], result['latency_ms'])
        if all(r['started_at'] is not None and r['completed_at'] is not None for r in rows):
            _require(_timestamp(result['started_at']) == min(_timestamp(r['started_at']) for r in rows))
            _require(_timestamp(result['completed_at']) == max(_timestamp(r['completed_at']) for r in rows))
        else:
            _require(result['completed_at'] is None and result['latency_ms'] is None)
    return result

def _round(raw, latest, as_of):
    if raw is None:
        return None
    _require(isinstance(raw, dict))
    keys = ('round_id', 'scheduled_for', 'started_at', 'completed_at', 'latency_ms',
            'candy_execution_id', 'pelican_execution_id')
    _require(all(key in raw for key in keys))
    result = {key: raw[key] for key in keys}
    _require(_text(result['round_id'], 200))
    for key in ('scheduled_for', 'started_at', 'completed_at'):
        result[key] = _stamp(result[key], as_of)
        _require(result[key] is not None)
    for kind in _KINDS:
        execution = latest[kind]
        _require(execution is not None and execution['execution_id'] is not None)
        _require(execution['shared_round_id'] == result['round_id'])
        _require(execution['scheduled_for'] == result['scheduled_for'])
        _require(execution['execution_id'] == result[kind + '_execution_id'])
        _require(execution['status'] != 'pending' and execution['completed_at'] is not None)
    _require(_timestamp(result['started_at']) == min(_timestamp(latest[k]['started_at']) for k in _KINDS))
    _require(_timestamp(result['completed_at']) == max(_timestamp(latest[k]['completed_at']) for k in _KINDS))
    _duration(result['started_at'], result['completed_at'], result['latency_ms'])
    return result

def _artwork(raw, group, model, pelican, as_of, now, max_age):
    if raw is None:
        return None, 'none'
    _require(isinstance(raw, dict) and pelican is not None and pelican['status'] == 'success')
    _require(_integer(raw.get('id'), 1) and raw.get('group_id') == group['id'] and raw.get('model_id') == model)
    _require(raw.get('execution_id') == pelican['execution_id'])
    source = next((r for r in pelican['results'] if r['result_id'] == raw.get('source_result_id')), None)
    _require(source is not None and source['status'] == 'success')
    generated = _stamp(raw.get('generated_at'), as_of)
    _require(generated is not None and _text(raw.get('reasoning_effort'), 64, empty=True))
    text = raw.get('response_text')
    if not _text(text, 300000) or len(text.encode('utf-8')) > 300000:
        return None, 'unavailable'
    artwork = dict(id=raw['id'], source_result_id=source['result_id'], execution_id=raw['execution_id'],
                   group_id=group['id'], group_name=group['name'], model=model,
                   reasoning_effort=raw['reasoning_effort'], generated_at=generated,
                   response_text=text, latency_ms=source['latency_ms'])
    return artwork, ('stale' if (now - _timestamp(generated)).total_seconds() > max_age else 'available')


def _current(raw, latest, histories, as_of):
    # Legacy fallback is completion-ordered. New current.pelican uses the page's
    # generation order, which can select a different completed execution.
    selected = {kind: (latest[kind]['results'][-1] if latest[kind] else None) for kind in _KINDS}
    if raw is not None:
        _require(isinstance(raw, dict) and all(k in raw for k in (*_KINDS, 'artwork')))
        for kind in _KINDS:
            record = _record(raw[kind], kind, as_of) if raw[kind] is not None else None
            if kind == 'pelican':
                _require((record is None) == (latest[kind] is None))
            elif record is not None:
                _require(latest[kind] is not None and record['judgment'] == 'builtin_candy')
            known = histories[kind] + (latest[kind]['results'] if latest[kind] else [])
            if record is not None:
                for other in known:
                    if other['result_id'] == record['result_id']:
                        _require(other == record)
            if kind == 'pelican' and record is not None and 'has_output' in raw[kind]:
                _require(type(raw[kind]['has_output']) is bool)
                record['has_output'] = raw[kind]['has_output']
            selected[kind] = record
    return selected


def _current_artwork(raw, group, model, result, as_of, now, max_age):
    if raw is None:
        return None, 'none'
    _require(result is not None and result['status'] == 'success')
    # Reuse the strict artwork identity validation with this single sample.
    execution = dict(status=result['status'], execution_id=result['execution_id'], results=[result])
    return _artwork(raw, group, model, execution, as_of, now, max_age)

def load_detection_report_snapshot(get, group_id, now=None, max_age_seconds=900, *,
                                   artwork_max_age_seconds=86400, model_id=None):
    """One GET for a coherent report; invalid or unavailable data fails closed."""
    try:
        now = datetime.now(timezone.utc) if now is None else now
        _require(isinstance(now, datetime) and now.tzinfo is not None and now.utcoffset() is not None)
        _require(_number(max_age_seconds) and max_age_seconds > 0)
        _require(_number(artwork_max_age_seconds) and artwork_max_age_seconds > 0)
        raw = get(report_path(group_id, model_id))
        _require(isinstance(raw, dict) and raw.get('schema_version') == 2)
        _require(_text(raw.get('snapshot_id'), 200))
        as_of = _timestamp(raw.get('as_of'))
        _require(as_of is not None and as_of <= now + timedelta(seconds=5))
        group = raw.get('group')
        _require(isinstance(group, dict) and type(group.get('id')) is int and group['id'] == group_id)
        _require(_text(group.get('name'), 256) and _text(group.get('platform'), 64))
        rate = group.get('rate_multiplier')
        _require(rate is None or _number(rate))
        selected_model = raw.get('model_id')
        _require(selected_model is None or _model(selected_model))
        _require(model_id is None or selected_model == model_id)
        window = raw.get('window')
        _require(isinstance(window, dict))
        start, end = _timestamp(window.get('from')), _timestamp(window.get('to'))
        _require(start is not None and end == as_of and end - start == timedelta(hours=24))
        _require(all(isinstance(raw.get(k), dict) for k in ('statistics','history','history_meta','latest')))
        histories, statistics, latest, history_meta = {}, {}, {}, {}
        seen = set()
        for kind in _KINDS:
            rows = raw['history'].get(kind)
            _require(isinstance(rows, list) and len(rows) <= 240)
            histories[kind] = [_record(row, kind, as_of) for row in rows]
            for row in histories[kind]:
                _require(row['result_id'] not in seen)
                seen.add(row['result_id'])
            meta = raw['history_meta'].get(kind)
            statistics[kind] = _stats(raw['statistics'].get(kind), histories[kind], meta)
            history_meta[kind] = {key: meta[key] for key in ('total_count', 'returned_count', 'limit')}
            latest[kind] = _execution(raw['latest'].get(kind), kind, histories[kind], as_of)
        _require(selected_model is not None or not seen)
        current = _round(raw.get('current_round'), latest, as_of)
        selected = _current(raw.get('current'), latest, histories, as_of)
        candidate = raw['current']['artwork'] if isinstance(raw.get('current'), dict) else raw.get('artwork')
        artwork, art_status = _current_artwork(candidate, group, selected_model, selected['pelican'],
                                              as_of, now, artwork_max_age_seconds)
        return dict(schema_version=2, fetched_at=now.isoformat(), as_of=as_of.isoformat(),
                    snapshot_id=raw['snapshot_id'], group_id=group_id, group_name=group['name'],
                    platform=group['platform'], model_id=selected_model, rate_multiplier=rate,
                    stats_window={'from':start.isoformat(),'to':end.isoformat()},
                    stats_status='stale' if (now-as_of).total_seconds()>max_age_seconds else 'available',
                    statistics=statistics, history=histories, history_meta=history_meta,
                    executions=latest, current_round=current, current=selected,
                    latest=artwork, artwork_status=art_status)
    except Exception:
        raise PelicanSourceUnavailable('public_report_source_unavailable') from None
