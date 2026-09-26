# SPDX-License-Identifier: AGPL-3.0-only
from urllib.parse import urlparse

POST_PATHS = {'/api/accounts/authorize/continue','/api/accounts/password/verify','/api/accounts/mfa/verify','/api/accounts/workspace/select','/api/accounts/session/select','/oauth/token'}

def _is_tls_handshake_error(exc):
    return any(x in str(exc).lower() for x in ('curl: (35)', 'tls connect error', 'sslerror'))

class RestrictedSession:
    """Only allow the login protocol. Never follow arbitrary credential redirects."""
    def __init__(self, session):self.inner=session;self.cookies=session.cookies
    def get(self,url,**kwargs):
        p=urlparse(url)
        allowed=(p.hostname in ('auth.openai.com','chatgpt.com','sentinel.openai.com') and p.scheme=='https' and not p.username and not p.password and p.port in (None,443))
        if not allowed:raise ValueError('unsupported_destination')
        kwargs['allow_redirects']=False
        return self.inner.get(url,**kwargs)
    def post(self,url,**kwargs):
        p=urlparse(url)
        allowed=(p.scheme=='https' and not p.username and not p.password and p.port in (None,443) and ((p.hostname=='auth.openai.com' and p.path in POST_PATHS) or (p.hostname=='chatgpt.com' and p.path=='/api/auth/signin/openai') or (p.hostname=='sentinel.openai.com' and p.path=='/backend-api/sentinel/req')))
        if not allowed:raise ValueError('unsupported_operation')
        kwargs['allow_redirects']=False
        return self.inner.post(url,**kwargs)
