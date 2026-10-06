# SPDX-License-Identifier: AGPL-3.0-only
# Adapted from Regert888/gpt-outlook-register, commit 0c28b4011daf352053149c1b28efdb037975bca2.
# Only stateless headers, PKCE and response helpers; registration and mail code removed.
from __future__ import annotations
import base64, hashlib, secrets, os, random, time, re, json, logging
from typing import Optional
from urllib.parse import urlparse, parse_qs, urlencode
logger = logging.getLogger(__name__)
class ProtocolHelpers:

    @staticmethod
    def _extract_page_type(resp_json: dict | None) -> str:
        if not isinstance(resp_json, dict):
            return ''
        page = resp_json.get('page', {})
        if not isinstance(page, dict):
            return ''
        return (page.get('type', '') or '').strip()

    @staticmethod
    def _extract_continue_url_from_step(resp_json: dict | None) -> str:
        if not isinstance(resp_json, dict):
            return ''
        continue_url = (resp_json.get('continue_url', '') or '').strip()
        if continue_url:
            return continue_url
        page = resp_json.get('page', {})
        if not isinstance(page, dict):
            return ''
        if (page.get('type', '') or '').strip() != 'external_url':
            return ''
        payload = page.get('payload', {})
        if not isinstance(payload, dict):
            return ''
        return (payload.get('url', '') or '').strip()

    @staticmethod
    def _b64url_no_pad(raw: bytes) -> str:
        return base64.urlsafe_b64encode(raw).decode('utf-8').rstrip('=')

    def _build_pkce_pair(self, raw_bytes: int=64) -> tuple[str, str]:
        verifier = self._b64url_no_pad(secrets.token_bytes(max(32, int(raw_bytes))))
        if len(verifier) < 43:
            verifier = (verifier + 'A' * 43)[:43]
        if len(verifier) > 128:
            verifier = verifier[:128]
        challenge = self._b64url_no_pad(hashlib.sha256(verifier.encode('utf-8')).digest())
        return (verifier, challenge)

    def _build_codex_authorize(self, prompt_override: Optional[str]=None) -> tuple[str, str, str, str, str]:
        client_id = (os.getenv('OAUTH_CODEX_CLIENT_ID', '') or '').strip() or 'app_EMoamEEZ73f0CkXaXp7hrann'
        redirect_uri = (os.getenv('OAUTH_CODEX_REDIRECT_URI', '') or '').strip() or 'http://localhost:1455/auth/callback'
        scope = (os.getenv('OAUTH_CODEX_SCOPE', '') or '').strip() or 'openid email profile offline_access'
        state = self._b64url_no_pad(secrets.token_bytes(24))
        (verifier, challenge) = self._build_pkce_pair()
        prompt = (os.getenv('OAUTH_CODEX_PROMPT', 'login') or '').strip() if prompt_override is None else (prompt_override or '').strip()
        params = {'client_id': client_id, 'response_type': 'code', 'redirect_uri': redirect_uri, 'scope': scope, 'state': state, 'code_challenge': challenge, 'code_challenge_method': 'S256', 'id_token_add_organizations': 'true', 'codex_cli_simplified_flow': 'true'}
        if prompt:
            params['prompt'] = prompt
        auth_url = f'https://auth.openai.com/oauth/authorize?{urlencode(params)}'
        return (auth_url, state, verifier, redirect_uri, client_id)

    @staticmethod
    def _callback_has_code(url: str, redirect_uri: str) -> bool:
        if not url:
            return False
        try:
            cb_base = (redirect_uri or '').split('?', 1)[0].rstrip('/')
            target = url.split('?', 1)[0].rstrip('/')
            if cb_base and target == cb_base:
                qs = parse_qs(urlparse(url).query)
                return bool((qs.get('code', [''])[0] or '').strip())
        except Exception:
            return False
        return False

    @staticmethod
    def _datadog_trace_headers() -> dict:
        tid = f'{random.getrandbits(64):016x}'
        sid = str(random.getrandbits(63))
        pid = str(random.getrandbits(63))
        ts_hex = f'{int(time.time()):08x}'
        return {'traceparent': f'00-0000000000000000{tid}-{random.getrandbits(64):016x}-01', 'x-datadog-trace-id': sid, 'x-datadog-parent-id': pid, 'x-datadog-sampling-priority': '1', 'x-datadog-origin': 'rum', 'x-datadog-tags': f'_dd.p.id={tid},_dd.p.tid={ts_hex}00000000,_dd.b.sr=1'}

    def _common_headers(self, referer: str='https://chatgpt.com/') -> dict:
        origin = 'https://chatgpt.com'
        try:
            parsed = urlparse(referer or '')
            if parsed.scheme and parsed.netloc:
                origin = f'{parsed.scheme}://{parsed.netloc}'
        except Exception:
            pass
        fp = self._fingerprint
        headers = {'Accept': 'application/json', 'Referer': referer, 'Origin': origin, 'User-Agent': self._ua, 'Accept-Language': fp['lang_full'], 'Accept-Encoding': 'gzip, deflate, br, zstd', 'Sec-Fetch-Dest': 'empty', 'Sec-Fetch-Mode': 'cors', 'Sec-Fetch-Site': 'same-origin', 'priority': 'u=1, i'}
        if fp.get('sec_ch_ua'):
            headers['sec-ch-ua'] = fp['sec_ch_ua']
            headers['sec-ch-ua-mobile'] = fp.get('sec_ch_ua_mobile') or '?0'
            headers['sec-ch-ua-platform'] = fp['sec_ch_ua_platform']
            if fp.get('sec_ch_ua_full_version_list'):
                headers['sec-ch-ua-full-version-list'] = fp['sec_ch_ua_full_version_list']
            if fp.get('sec_ch_ua_arch'):
                headers['sec-ch-ua-arch'] = fp['sec_ch_ua_arch']
            if fp.get('sec_ch_ua_bitness'):
                headers['sec-ch-ua-bitness'] = fp['sec_ch_ua_bitness']
            if fp.get('sec_ch_ua_model'):
                headers['sec-ch-ua-model'] = fp['sec_ch_ua_model']
            if fp.get('sec_ch_ua_platform_version'):
                headers['sec-ch-ua-platform-version'] = fp['sec_ch_ua_platform_version']
        try:
            host = (urlparse(origin).netloc or '').lower()
        except Exception:
            host = ''
        if 'auth.openai.com' in host:
            device_id = (self.result.device_id or '').strip() or (self.session.cookies.get('oai-did', '') or '').strip()
            if device_id:
                headers['oai-device-id'] = device_id
        headers.update(self._datadog_trace_headers())
        return headers

    def _navigation_headers(self) -> dict:
        fp = self._fingerprint
        headers = {'accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8', 'Accept-Language': fp['lang_full'], 'Accept-Encoding': 'gzip, deflate, br, zstd', 'sec-fetch-dest': 'document', 'sec-fetch-mode': 'navigate', 'sec-fetch-site': 'none', 'sec-fetch-user': '?1', 'upgrade-insecure-requests': '1', 'priority': 'u=0, i', 'User-Agent': self._ua}
        if fp.get('sec_ch_ua'):
            headers['sec-ch-ua'] = fp['sec_ch_ua']
            headers['sec-ch-ua-mobile'] = fp.get('sec_ch_ua_mobile') or '?0'
            headers['sec-ch-ua-platform'] = fp['sec_ch_ua_platform']
            for (key, name) in (('sec_ch_ua_full_version_list', 'sec-ch-ua-full-version-list'), ('sec_ch_ua_arch', 'sec-ch-ua-arch'), ('sec_ch_ua_bitness', 'sec-ch-ua-bitness'), ('sec_ch_ua_model', 'sec-ch-ua-model'), ('sec_ch_ua_platform_version', 'sec-ch-ua-platform-version')):
                if fp.get(key):
                    headers[name] = fp[key]
        return headers

    def _sentinel_fp_kwargs(self) -> dict:
        fp = self._fingerprint or {}
        return {'user_agent': self._ua, 'sec_ch_ua': fp.get('sec_ch_ua', ''), 'sec_ch_ua_platform': fp.get('sec_ch_ua_platform', ''), 'sec_ch_ua_mobile': fp.get('sec_ch_ua_mobile', ''), 'sec_ch_ua_full_version_list': fp.get('sec_ch_ua_full_version_list', ''), 'sec_ch_ua_arch': fp.get('sec_ch_ua_arch', ''), 'sec_ch_ua_bitness': fp.get('sec_ch_ua_bitness', ''), 'sec_ch_ua_model': fp.get('sec_ch_ua_model', ''), 'sec_ch_ua_platform_version': fp.get('sec_ch_ua_platform_version', ''), 'screen': fp.get('screen', ''), 'lang': fp.get('lang', ''), 'lang_full': fp.get('lang_full', ''), 'browser_type': fp.get('browser_type', ''), 'navigator_platform': fp.get('navigator_platform', ''), 'navigator_vendor': fp.get('navigator_vendor'), 'hardware_concurrency': fp.get('hardware_concurrency', 0), 'device_memory': fp.get('device_memory'), 'max_touch_points': fp.get('max_touch_points', 0), 'device_pixel_ratio': fp.get('device_pixel_ratio', 0.0), 'timezone': fp.get('timezone', '')}

    def get_sentinel_token(self, device_id: str) -> str:
        logger.info('[4/10] 获取 Sentinel Token (PoW)...')
        from sentinel import get_sentinel_token
        result = get_sentinel_token(self.session, device_id=device_id, flow='authorize_continue', **self._sentinel_fp_kwargs())
        (token, so_token) = result
        self._last_sentinel_token = token or ''
        self._last_sentinel_so_token = so_token or ''
        logger.debug('Sentinel Token 获取成功')
        return token

    @staticmethod
    def _extract_workspace_id_from_html(html_text: str) -> str:
        if not html_text:
            return ''
        try:
            text = html_text.replace('\\"', '"')
            patterns = ['workspaces".{0,1600}?"id","([0-9a-fA-F-]{36})"', '"workspace_id"\\s*:\\s*"([0-9a-fA-F-]{36})"', '"workspaceId"\\s*:\\s*"([0-9a-fA-F-]{36})"']
            for p in patterns:
                m = re.search(p, text, flags=re.DOTALL | re.IGNORECASE)
                if m:
                    return (m.group(1) or '').strip()
        except Exception:
            return ''
        return ''
