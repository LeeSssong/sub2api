"""Deterministic report delivery. No API calls, model tools or platform imports.

The production adapter injects a GET-only source, renderer and sender. Missing
configuration disables the feature; dry-run can never enter the sender path.
"""
from __future__ import annotations

import asyncio
import hashlib
import json
import os
import re
import sqlite3
import stat
import tempfile
import time
from contextlib import contextmanager
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from zoneinfo import ZoneInfo

TEMPLATE_VERSION = 'xingqiao-pelican-v2-result-status'
BEIJING = ZoneInfo('Asia/Shanghai')
SESSION_RE = re.compile(r'[A-Za-z0-9_.-]{1,100}:GroupMessage:[0-9]{5,20}')
CACHE_TTL_SECONDS = 24 * 3600
CACHE_MAX_BYTES = 64 * 1024 * 1024
CACHE_MAX_FILES = 256
HISTORY_TTL_SECONDS = 30 * 86400
HISTORY_MAX_INACTIVE_ROWS = 10000
_CACHE_NAME = re.compile(r'([0-9a-f]{64})[.]png')


@dataclass(frozen=True)
class ReportTarget:
    session: str
    group_ids: tuple[int, ...]


@dataclass(frozen=True)
class ReportConfig:
    enabled: bool = False
    dry_run: bool = True
    repeat_unchanged: bool = False
    timezone: str = 'Asia/Shanghai'
    times: tuple[str, ...] = ('09:00', '15:00', '21:00')
    targets: tuple[ReportTarget, ...] = ()
    grace_seconds: int = 600
    expiry_seconds: int = 1800
    max_per_target: int = 3
    stats_max_age_seconds: int = 900
    artwork_max_age_seconds: int = 86400

    @classmethod
    def from_dict(cls, raw):
        if not isinstance(raw, dict):
            raise ValueError('report config must be an object')
        permitted = set(cls.__dataclass_fields__)
        if set(raw) - permitted:
            raise ValueError('unknown report config field')
        for key in ('enabled', 'dry_run', 'repeat_unchanged'):
            if key in raw and type(raw[key]) is not bool:
                raise ValueError('report switches must be booleans')
        if raw.get('timezone', 'Asia/Shanghai') != 'Asia/Shanghai':
            raise ValueError('report timezone must be Asia/Shanghai')
        times = raw.get('times', ['09:00', '15:00', '21:00'])
        if not isinstance(times, list) or not 1 <= len(times) <= 48:
            raise ValueError('invalid report times')
        for item in times:
            if not isinstance(item, str) or not re.fullmatch(r'(?:[01][0-9]|2[0-3]):[0-5][0-9]', item):
                raise ValueError('invalid report time')
        target_rows = raw.get('targets', [])
        if not isinstance(target_rows, list) or len(target_rows) > 20:
            raise ValueError('invalid report targets')
        targets = []
        seen = set()
        for item in target_rows:
            if not isinstance(item, dict) or set(item) != {'session', 'group_ids'}:
                raise ValueError('invalid report target')
            session, ids = item['session'], item['group_ids']
            if not isinstance(session, str) or not SESSION_RE.fullmatch(session) or session in seen:
                raise ValueError('invalid or duplicate report group session')
            if not isinstance(ids, list) or not 1 <= len(ids) <= 20:
                raise ValueError('invalid public report groups')
            if any(type(g) is not int or g < 1 for g in ids) or len(set(ids)) != len(ids):
                raise ValueError('invalid or duplicate public report group')
            targets.append(ReportTarget(session, tuple(ids)))
            seen.add(session)
        options = {}
        for key, default, low, high in (
            ('grace_seconds', 600, 30, 1800), ('expiry_seconds', 1800, 60, 3600),
            ('max_per_target', 3, 1, 10), ('stats_max_age_seconds', 900, 60, 3600),
            ('artwork_max_age_seconds', 86400, 300, 604800),
        ):
            value = raw.get(key, default)
            if type(value) is not int or not low <= value <= high:
                raise ValueError('invalid report numeric setting')
            options[key] = value
        return cls(enabled=raw.get('enabled', False), dry_run=raw.get('dry_run', True),
                   repeat_unchanged=raw.get('repeat_unchanged', False),
                   times=tuple(sorted(set(times))), targets=tuple(targets), **options)


def load_report_config(path: Path) -> ReportConfig:
    try:
        data = path.read_bytes()
    except FileNotFoundError:
        return ReportConfig()
    if len(data) > 32768:
        raise ValueError('report config exceeds size limit')
    return ReportConfig.from_dict(json.loads(data))


def due_slot(config: ReportConfig, now: datetime) -> str | None:
    if now.tzinfo is None:
        raise ValueError('report scheduling requires timezone-aware time')
    local = now.astimezone(BEIJING)
    for value in reversed(config.times):
        hour, minute = map(int, value.split(':'))
        slot = local.replace(hour=hour, minute=minute, second=0, microsecond=0)
        elapsed = (local-slot).total_seconds()
        if 0 <= elapsed < config.grace_seconds:
            return slot.isoformat(timespec='minutes')
    return None


def content_digest(snapshot: dict) -> str:
    # Wall-clock refresh and moving window endpoints are not new results.
    if snapshot.get('schema_version') == 2:
        identity = {key: snapshot.get(key) for key in (
            'schema_version', 'group_id', 'group_name', 'platform', 'model_id',
            'rate_multiplier', 'statistics', 'history', 'history_meta', 'executions', 'current_round', 'current',
            'stats_status', 'artwork_status',
        )}
        artifact = snapshot.get('latest') or {}
        identity['artwork'] = {key:value for key,value in artifact.items() if key != 'response_text'}
        identity['artwork_hash'] = hashlib.sha256(str(artifact.get('response_text') or '').encode()).hexdigest()
        identity['template'] = TEMPLATE_VERSION
        return hashlib.sha256(json.dumps(identity, sort_keys=True, ensure_ascii=False,
                                         separators=(',', ':'), allow_nan=False).encode()).hexdigest()
    latest = snapshot.get('latest') or {}
    window = snapshot.get('stats_window') or {}
    identity = {
        'template': TEMPLATE_VERSION, 'group_id': snapshot.get('group_id'),
        'group_name': snapshot.get('group_name'), 'platform': snapshot.get('platform'),
        'stats': snapshot.get('stats'), 'stats_status': snapshot.get('stats_status'),
        'coverage_started_at': window.get('coverage_started_at'),
        'complete': window.get('complete'), 'artwork_status': snapshot.get('artwork_status'),
        'latest': {key: latest.get(key) for key in (
            'id', 'group_id', 'model', 'reasoning_effort', 'generated_at', 'latency_ms',
        )},
        'artwork': hashlib.sha256(str(latest.get('response_text') or '').encode()).hexdigest(),
    }
    return hashlib.sha256(json.dumps(identity, sort_keys=True, ensure_ascii=False,
                                     separators=(',', ':'), allow_nan=False).encode()).hexdigest()


def report_is_usable(snapshot: dict) -> bool:
    if snapshot.get('stats_status') == 'stale':
        return False
    if snapshot.get('schema_version') == 2:
        current = snapshot.get('current') or {}
        if any(isinstance(row, dict) and row.get('status') in ('success', 'failed', 'ungraded')
               for row in current.values()):
            return True
        return any(type(stats.get('observed_count')) is int and stats['observed_count'] > 0
                   for stats in (snapshot.get('statistics') or {}).values() if isinstance(stats, dict))
    stats = snapshot.get('stats')
    if isinstance(stats, dict) and type(stats.get('total_count')) is int and stats['total_count'] > 0:
        return True
    return bool(snapshot.get('latest') and snapshot.get('artwork_status') == 'available')


class DefiniteSendFailure(Exception):
    """Use only when the platform definitely did not accept the message."""


class ReportStore:
    def __init__(self, path: Path):
        self.path = Path(path)
        self.path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
        with self.connect() as db:
            db.execute('PRAGMA journal_mode=WAL')
            db.execute('''CREATE TABLE IF NOT EXISTS reports (
                id INTEGER PRIMARY KEY, session TEXT NOT NULL, group_id INTEGER NOT NULL,
                slot TEXT NOT NULL, digest TEXT NOT NULL, state TEXT NOT NULL,
                attempts INTEGER NOT NULL DEFAULT 0, ready_at REAL NOT NULL,
                expires_at REAL NOT NULL, receipt TEXT,
                UNIQUE(session,group_id,slot,digest))''')
            db.execute('CREATE INDEX IF NOT EXISTS reports_state_expiry ON reports(state,expires_at)')
            db.execute('CREATE INDEX IF NOT EXISTS reports_digest_id ON reports(digest,id)')
            # One plugin worker owns this database. A previous process can have
            # sent a message before losing its receipt: never automatically replay.
            db.execute("UPDATE reports SET state='uncertain' WHERE state='sending'")
        os.chmod(self.path, 0o600)

    @contextmanager
    def connect(self):
        db = sqlite3.connect(self.path, timeout=10)
        db.row_factory = sqlite3.Row
        try:
            with db:
                yield db
        finally:
            db.close()

    def rows(self):
        with self.connect() as db:
            return [dict(row) for row in db.execute('SELECT * FROM reports ORDER BY id')]

    def handled(self, session, group_id, slot, digest, dry_run=False, repeat_unchanged=False):
        states = ('sent', 'uncertain', 'dry_run') if dry_run else ('sent', 'uncertain')
        placeholders = ','.join('?' for _ in states)
        match = 'slot=?' if repeat_unchanged else '(slot=? OR digest=?)'
        keys = (session, group_id, slot) if repeat_unchanged else (session, group_id, slot, digest)
        with self.connect() as db:
            return db.execute(f'''SELECT 1 FROM reports WHERE session=? AND group_id=?
                AND {match} AND state IN ({placeholders}) LIMIT 1''',
                (*keys, *states)).fetchone() is not None

    def enqueue(self, session, group_id, slot, digest, now, ttl):
        with self.connect() as db:
            db.execute('''INSERT OR IGNORE INTO reports
                (session,group_id,slot,digest,state,ready_at,expires_at)
                VALUES (?,?,?,?,'pending',?,?)''', (session, group_id, slot, digest, now, now+ttl))
            row = db.execute('SELECT * FROM reports WHERE session=? AND group_id=? AND slot=? AND digest=?',
                             (session, group_id, slot, digest)).fetchone()
            if row['state'] == 'dry_run':
                db.execute("UPDATE reports SET state='pending',ready_at=?,expires_at=? WHERE id=?",
                           (now, now+ttl, row['id']))
                row = db.execute('SELECT * FROM reports WHERE id=?', (row['id'],)).fetchone()
            elif row['state'] == 'cancelled' and row['expires_at'] > now:
                # A later grace-window read has revalidated this same public
                # digest. Recover pre-send source/render failures, keeping the
                # original deadline and send-attempt budget. Sent, uncertain,
                # failed and expired records are never reopened here.
                db.execute("UPDATE reports SET state='pending',ready_at=? WHERE id=? AND state='cancelled' AND expires_at>?",
                           (now, row['id'], now))
                row = db.execute('SELECT * FROM reports WHERE id=?', (row['id'],)).fetchone()
            return dict(row)

    def ready(self, now):
        with self.connect() as db:
            db.execute("UPDATE reports SET state='expired' WHERE expires_at<=? AND state IN ('pending','retry')", (now,))
            return [dict(row) for row in db.execute("SELECT * FROM reports WHERE state IN ('pending','retry') AND ready_at<=? AND expires_at>? ORDER BY id", (now, now))]

    def claim(self, job_id, now):
        with self.connect() as db:
            return db.execute("UPDATE reports SET state='sending',attempts=attempts+1 WHERE id=? AND state IN ('pending','retry') AND ready_at<=? AND expires_at>?", (job_id, now, now)).rowcount == 1

    def finish(self, job_id, state, receipt=None):
        if state not in {'sent', 'uncertain', 'cancelled', 'dry_run', 'failed'}:
            raise ValueError('invalid report terminal state')
        with self.connect() as db:
            db.execute('UPDATE reports SET state=?,receipt=? WHERE id=?',
                       (state, str(receipt)[:100] if receipt is not None else None, job_id))

    def retry(self, job_id, now):
        with self.connect() as db:
            row = db.execute('SELECT * FROM reports WHERE id=?', (job_id,)).fetchone()
            delays = (30, 120, 300)
            attempt = row['attempts']
            if attempt > len(delays) or now+delays[max(0, attempt-1)] >= row['expires_at']:
                db.execute("UPDATE reports SET state='failed' WHERE id=?", (job_id,))
            else:
                db.execute("UPDATE reports SET state='retry',ready_at=? WHERE id=?", (now+delays[max(0, attempt-1)], job_id))

    def cache_references(self, now):
        """Expire dead jobs, bound non-dedupe history, and return cache owners.

        Sent/uncertain rows remain lightweight permanent dedupe evidence.
        Pruning never removes a live job or its original retry budget.
        """
        with self.connect() as db:
            db.execute("UPDATE reports SET state='expired' WHERE expires_at<=? AND state IN ('pending','retry')", (now,))
            protected = {row[0] for row in db.execute("SELECT DISTINCT digest FROM reports WHERE state IN ('pending','retry','sending') AND expires_at>?", (now,))}
            rejected = {row[0] for row in db.execute('''SELECT r.digest FROM reports r
                WHERE r.state IN ('cancelled','failed','expired') AND NOT EXISTS
                (SELECT 1 FROM reports newer WHERE newer.digest=r.digest AND newer.id>r.id)''')}
            inactive = "state IN ('cancelled','failed','expired','dry_run') AND expires_at<=?"
            db.execute(f'DELETE FROM reports WHERE {inactive} AND expires_at<=?',
                       (now, now-HISTORY_TTL_SECONDS))
            db.execute(f'''DELETE FROM reports WHERE id IN
                (SELECT id FROM reports WHERE {inactive} ORDER BY expires_at DESC,id DESC LIMIT -1 OFFSET ?)''',
                       (now, HISTORY_MAX_INACTIVE_ROWS))
            return protected, rejected


def write_report_cache(path: Path, image: bytes, created_at=None):
    if not isinstance(image, bytes) or not image or len(image) > 8*1024*1024:
        raise ValueError('invalid report image')
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    if path.parent.is_symlink() or path.is_symlink():
        raise ValueError('invalid report cache path')
    tmp = None
    try:
        with tempfile.NamedTemporaryFile(dir=path.parent, prefix='.report-', delete=False) as f:
            tmp = Path(f.name)
            f.write(image)
        os.chmod(tmp, 0o600)
        if created_at is not None:
            os.utime(tmp, (created_at, created_at))
        os.replace(tmp, path)
    finally:
        if tmp is not None and tmp.exists():
            tmp.unlink()


def prune_report_cache(directory: Path, store: ReportStore, now, reserve_bytes=0, *, discard_digests=()):
    """Collect owned PNGs and report whether a new image fits the fixed quota.

    Live pending/retry/sending references take precedence over eviction. When
    they fill the quota the caller uses its rendered bytes in memory instead
    of growing disk storage. Unknown files, directories and links are untouched.
    """
    protected, rejected = store.cache_references(now)
    rejected.update(discard_digests)
    directory = Path(directory)
    if directory.is_symlink():
        return False
    entries = []
    if directory.exists():
        for path in directory.iterdir():
            match = _CACHE_NAME.fullmatch(path.name)
            if not match:
                continue
            try:
                info = path.lstat()
            except FileNotFoundError:
                continue
            if not stat.S_ISREG(info.st_mode):
                continue
            digest = match.group(1)
            if digest not in protected and (digest in rejected or now-info.st_mtime >= CACHE_TTL_SECONDS):
                path.unlink(missing_ok=True)
                continue
            os.chmod(path, 0o600)
            entries.append((info.st_mtime, path.name, path, info.st_size, digest))
    size = sum(entry[3] for entry in entries)
    count = len(entries)
    reserve_files = int(reserve_bytes > 0)
    for _, _, path, length, digest in sorted(entries):
        if size+reserve_bytes <= CACHE_MAX_BYTES and count+reserve_files <= CACHE_MAX_FILES:
            break
        if digest in protected:
            continue
        path.unlink(missing_ok=True)
        size -= length
        count -= 1
    return size+reserve_bytes <= CACHE_MAX_BYTES and count+reserve_files <= CACHE_MAX_FILES


class ReportDelivery:
    def __init__(self, config: ReportConfig, directory: Path, load, render, send):
        self.config, self.directory = config, Path(directory)
        self.load, self.render, self.send = load, render, send
        self.store = ReportStore(self.directory / 'outbox.sqlite3')
        self.lock = None

    def _cancel_job(self, job, now):
        self.store.finish(job['id'], 'cancelled')
        prune_report_cache(self.directory / 'cache', self.store, now,
                           discard_digests=(job['digest'],))

    async def tick(self, now=None):
        started = time.monotonic()
        now = now or datetime.now(timezone.utc)
        result = {'status': 'idle', 'rendered': 0, 'sent': 0, 'skipped': 0, 'errors': 0}
        if not self.config.enabled:
            prune_report_cache(self.directory / 'cache', self.store, now.timestamp())
            result['status'] = 'disabled'
            return result
        if self.lock is None:
            self.lock = asyncio.Lock()
        async with self.lock:
            slot = due_slot(self.config, now)
            ts = now.timestamp()
            # The caller's wall-clock anchor may be deterministic in tests;
            # monotonic elapsed time still bounds slow source/render/send work.
            def current_timestamp():
                return ts + max(0.0, time.monotonic() - started)
            prune_report_cache(self.directory / 'cache', self.store, current_timestamp())
            current = {}
            allowed = {(t.session, group) for t in self.config.targets for group in t.group_ids[:self.config.max_per_target]}
            if slot:
                for target in self.config.targets:
                    for group_id in target.group_ids[:self.config.max_per_target]:
                        try:
                            if group_id not in current:
                                current[group_id] = await self.load(group_id)
                            item = current[group_id]
                            if item.get('group_id') != group_id or not report_is_usable(item):
                                result['skipped'] += 1
                                continue
                            digest = content_digest(item)
                            if self.store.handled(target.session, group_id, slot, digest, self.config.dry_run, self.config.repeat_unchanged):
                                continue
                            self.store.enqueue(target.session, group_id, slot, digest, ts, self.config.expiry_seconds)
                        except Exception:
                            # Raw upstream errors may contain protected data. Persist counts only.
                            result['errors'] += 1
            for job in self.store.ready(current_timestamp()):
                if (job['session'], job['group_id']) not in allowed:
                    self._cancel_job(job, current_timestamp())
                    continue
                if self.store.handled(job['session'], job['group_id'], job['slot'], job['digest'], self.config.dry_run, self.config.repeat_unchanged):
                    self._cancel_job(job, current_timestamp())
                    continue
                try:
                    item = current.get(job['group_id']) or await self.load(job['group_id'])
                    if item.get('group_id') != job['group_id'] or not report_is_usable(item) or content_digest(item) != job['digest']:
                        self._cancel_job(job, current_timestamp())
                        continue
                    cache_path = self.directory / 'cache' / (job['digest']+'.png')
                    if cache_path.parent.is_symlink() or cache_path.is_symlink():
                        raise ValueError('invalid report cache path')
                    if cache_path.exists() and not self.config.repeat_unchanged:
                        image = cache_path.read_bytes()
                    else:
                        image = await self.render(item)
                        if not isinstance(image, bytes) or not image or len(image) > 8*1024*1024:
                            raise ValueError('invalid report image')
                        if prune_report_cache(cache_path.parent, self.store, current_timestamp(), len(image)):
                            write_report_cache(cache_path, image, current_timestamp())
                        result['rendered'] += 1
                    if self.config.dry_run:
                        self.store.finish(job['id'], 'dry_run')
                        continue
                    # Recheck visibility after rendering and before every retry.
                    latest = await self.load(job['group_id'])
                    if latest.get('group_id') != job['group_id'] or not report_is_usable(latest) or content_digest(latest) != job['digest']:
                        self._cancel_job(job, current_timestamp())
                        continue
                except Exception:
                    result['errors'] += 1
                    # Do not retain an image queued against unverified public data.
                    self._cancel_job(job, current_timestamp())
                    continue
                if not self.store.claim(job['id'], current_timestamp()):
                    continue
                if current_timestamp() >= job['expires_at']:
                    self._cancel_job(job, current_timestamp())
                    continue
                try:
                    receipt = await self.send(job['session'], image)
                    if not receipt:
                        self.store.finish(job['id'], 'uncertain')
                        result['errors'] += 1
                    else:
                        self.store.finish(job['id'], 'sent', receipt)
                        result['sent'] += 1
                except DefiniteSendFailure:
                    self.store.retry(job['id'], current_timestamp())
                    result['errors'] += 1
                except asyncio.CancelledError:
                    self.store.finish(job['id'], 'uncertain')
                    raise
                except Exception:
                    self.store.finish(job['id'], 'uncertain')
                    result['errors'] += 1
            prune_report_cache(self.directory / 'cache', self.store, current_timestamp())
            result['status'] = 'dry_run' if self.config.dry_run else 'active'
        return result
