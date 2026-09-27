"""GET-only transport for explicitly allowlisted public report projections."""
from __future__ import annotations

import json
import urllib.request

if __package__:
    from .pelican_source import PelicanSourceUnavailable, is_allowed_source_path
else:
    from pelican_source import PelicanSourceUnavailable, is_allowed_source_path

PUBLIC_ORIGIN = 'https://api.xingqiaolab.top'
# Bound the complete JSON, including escaped artwork and recent result rows.
MAX_RESPONSE_BYTES = 8*1024*1024


class NoReportRedirects(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        # Never forward an administrator key to a redirected address.
        return None


class ReadOnlyReportClient:
    def __init__(self, admin_key: str, *, open_request=None, timeout=15):
        self._key = (admin_key or '').strip()
        self._open = open_request or urllib.request.build_opener(NoReportRedirects()).open
        self.timeout = timeout

    def __call__(self, path: str):
        if not is_allowed_source_path(path):
            raise ValueError('report source path is not allowed')
        if not self._key:
            raise ValueError('report source credential is unavailable')
        request = urllib.request.Request(PUBLIC_ORIGIN+path, method='GET', headers={
            'X-API-Key': self._key, 'Accept': 'application/json',
            'User-Agent': 'XingqiaoReportBot/1.0',
        })
        try:
            with self._open(request, timeout=self.timeout) as response:
                raw = response.read(MAX_RESPONSE_BYTES+1)
            if len(raw) > MAX_RESPONSE_BYTES:
                raise ValueError('oversize response')
            value = json.loads(raw.decode('utf-8'))
            if not isinstance(value, dict):
                raise ValueError('invalid source response')
            if 'code' in value and value['code'] not in (0, 200):
                raise ValueError('source rejected report request')
            result = value.get('data') if 'data' in value else value
            if not isinstance(result, dict):
                raise ValueError('invalid public report data')
            return result
        except Exception:
            # Do not expose response bodies, raw exceptions or authentication data.
            raise PelicanSourceUnavailable('public_report_source_unavailable') from None
