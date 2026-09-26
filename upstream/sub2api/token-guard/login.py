# SPDX-License-Identifier: AGPL-3.0-only
"""Password/TOTP-only OpenAI login. Derived from the pinned upstream in UPSTREAM.md."""
from __future__ import annotations
import base64, hashlib, hmac, json, logging, re, struct, time, uuid, random
from datetime import datetime, timezone
from types import SimpleNamespace
from urllib.parse import urlparse, urljoin, parse_qs, urlencode
from protocol_helpers import ProtocolHelpers
from transport import RestrictedSession

class LoginError(Exception):pass

def validate_input(value):
    if not isinstance(value,dict):raise LoginError('invalid_credentials')
    email=value.get('email','')
    password=value.get('password','')
    secret=value.get('mfa_secret','')
    if not isinstance(email,str) or not re.fullmatch(r'[^\s@]+@[^\s@]+\.[^\s@]+',email) or not isinstance(password,str) or not password or not isinstance(secret,str):raise LoginError('invalid_credentials')
    secret=secret.strip().upper().replace(' ','').rstrip('=')
    try:
        if len(base64.b32decode(secret+'='*(-len(secret)%8)))<10:raise ValueError()
    except Exception:raise LoginError('invalid_credentials') from None
    proxy=value.get('proxy_url','')
    if not isinstance(proxy,str) or (proxy and urlparse(proxy).scheme not in ('http','https','socks5','socks5h')):raise LoginError('invalid_credentials')
    return {**value,'email':email.strip().lower(),'mfa_secret':secret}

def totp(secret,now=None):
    key=base64.b32decode(secret+'='*(-len(secret)%8))
    digest=hmac.new(key,struct.pack('>Q',int(time.time() if now is None else now)//30),hashlib.sha1).digest()
    offset=digest[-1]&15
    return str((struct.unpack('>I',digest[offset:offset+4])[0]&0x7fffffff)%1000000).zfill(6)

def claims(token):
    try:
        part=token.split('.')[1];data=json.loads(base64.urlsafe_b64decode(part+'='*(-len(part)%4)))
        if not isinstance(data,dict):raise ValueError()
        return data
    except Exception:raise LoginError('invalid_token') from None

def validate_credentials(data,target):
    if not isinstance(data,dict) or any(not isinstance(data.get(k),str) or not data[k] for k in ('access_token','refresh_token','id_token')):raise LoginError('invalid_token')
    access=claims(data['access_token']);identity=claims(data['id_token'])
    email=identity.get('email') or access.get('email') or (access.get('https://api.openai.com/profile') or {}).get('email')
    if not isinstance(email,str) or email.lower()!=target['email'].lower():raise LoginError('identity_mismatch')
    auth=access.get('https://api.openai.com/auth') or {}
    expected=target.get('expected_account_id')
    if expected and auth.get('chatgpt_account_id')!=expected:raise LoginError('identity_mismatch')
    exp=access.get('exp')
    if not isinstance(exp,(float,int)) or exp<time.time()+60:raise LoginError('invalid_token')
    if not isinstance(identity.get('exp'),(float,int)) or identity['exp']<time.time():raise LoginError('invalid_token')
    # Tokens came directly from the TLS-protected OAuth endpoint. Claims are consistency checks.
    return {k:data[k] for k in ('access_token','refresh_token','id_token')}|{'expires_at':datetime.fromtimestamp(exp,timezone.utc).isoformat(),'token_type':'Bearer'}

class LoginFlow(ProtocolHelpers):
    def __init__(self,value,session=None):
        self.input=validate_input(value)
        from fingerprint import generate_fingerprint
        self._fingerprint=generate_fingerprint()
        self._ua=self._fingerprint['user_agent']
        if session is None:
            from curl_cffi.requests import Session
            raw=Session(impersonate=self._fingerprint['impersonate'],proxy=self.input.get('proxy_url') or None,timeout=30)
            raw.trust_env=False
            session=RestrictedSession(raw)
        self.session=session
        self.result=SimpleNamespace(device_id='')
        self._last_sentinel_token='';self._last_sentinel_so_token=''
        self.auth_url,self.state,self.verifier,self.redirect_uri,self.client_id=self._build_codex_authorize(prompt_override="")
    @staticmethod
    def _state(data):
        page=data.get('page') or {}
        return str(page.get('type') or '').lower(),ProtocolHelpers._extract_continue_url_from_step(data)
    @staticmethod
    def _check_state(kind,url):
        text=(kind+' '+url).lower()
        if any(x in text for x in ('create-account','create_account','signup','register')):raise LoginError('registration_required')
        if any(x in text for x in ('email-verification','email_otp','add-phone','add_phone','phone-verification','captcha')):raise LoginError('extra_verification')
    def _post(self,path,payload,referer,error):
        h=self._common_headers(referer);h['Content-Type']='application/json'
        if self._last_sentinel_token:h['openai-sentinel-token']=self._last_sentinel_token
        if self._last_sentinel_so_token:h['openai-sentinel-so-token']=self._last_sentinel_so_token
        response=self.session.post('https://auth.openai.com'+path,headers=h,json=payload,timeout=30)
        if response.status_code!=200:raise LoginError(error)
        try:
            data=response.json()
            if not isinstance(data,dict):raise ValueError()
            return data
        except Exception:raise LoginError('unsupported_state') from None
    def _workspace_id(self,html):
        expected=self.input.get('expected_account_id','')
        raw=self.session.cookies.get('oai-client-auth-session','')
        for part in str(raw).split('.')[:2]:
            try:
                data=json.loads(base64.urlsafe_b64decode(part+'='*(-len(part)%4)))
                choices=[x.get('id') for x in data.get('workspaces',[]) if isinstance(x,dict)]
                if data.get('workspace_id'):choices.insert(0,data['workspace_id'])
                if expected in choices:return expected
                if choices and not expected:return choices[0]
            except Exception:continue
        # Existing account ID has priority, server validates workspace membership.
        return expected or self._extract_workspace_id_from_html(html)
    def _follow(self,url):
        for _ in range(16):
            url=urljoin('https://auth.openai.com',url)
            if self._callback_has_code(url,self.redirect_uri):return url
            self._check_state('',url)
            headers=self._navigation_headers();headers['Referer']='https://chatgpt.com/';headers['sec-fetch-site']='cross-site';headers.pop('sec-fetch-user',None)
            response=self.session.get(url,headers=headers,timeout=30,allow_redirects=False)
            if response.status_code in (301,302,303,307,308):
                location=response.headers.get('Location','')
                if not location:raise LoginError('unsupported_state')
                url=urljoin(url,location);continue
            if response.status_code!=200:raise LoginError('network_error')
            path=urlparse(url).path
            if '/log-in' in path:return url
            if '/choose-an-account' in path:
                sessions=set(re.findall(r'us_[A-Za-z0-9]{16,}',response.text))
                if len(sessions)!=1:raise LoginError('identity_mismatch')
                data=self._post('/api/accounts/session/select',{'session_id':sessions.pop()},url,'unsupported_state')
                _,next_url=self._state(data)
                if not next_url:raise LoginError('unsupported_state')
                url=next_url;continue
            if any(x in path for x in ('/workspace','/consent','/sign-in-with-chatgpt/')):
                wid=self._workspace_id(response.text)
                if not wid:raise LoginError('unsupported_state')
                data=self._post('/api/accounts/workspace/select',{'workspace_id':wid},url,'identity_mismatch')
                _,url=self._state(data)
                if not url:raise LoginError('unsupported_state')
                continue
            raise LoginError('unsupported_state')
        raise LoginError('unsupported_state')
    def _initialize_login(self):
        self.session.get('https://chatgpt.com/',headers=self._navigation_headers(),timeout=40)
        self.result.device_id=self.session.cookies.get_dict().get('oai-did') or str(uuid.uuid4())
        response=self.session.get('https://chatgpt.com/api/auth/csrf',headers=self._common_headers('https://chatgpt.com/auth/login'),timeout=30)
        if response.status_code!=200:raise LoginError('network_error')
        csrf=response.json().get('csrfToken')
        if not csrf:raise LoginError('unsupported_state')
        query=urlencode({'prompt':'login','screen_hint':'login','ext-oai-did':self.result.device_id,'auth_session_logging_id':str(uuid.uuid4()),'ext-passkey-client-capabilities':'1111','login_hint':self.input['email']})
        headers=self._common_headers('https://chatgpt.com/auth/login');headers['Content-Type']='application/x-www-form-urlencoded'
        response=self.session.post('https://chatgpt.com/api/auth/signin/openai?'+query,headers=headers,data={'csrfToken':csrf,'callbackUrl':'https://chatgpt.com/','json':'true'},timeout=30)
        if response.status_code!=200:raise LoginError('network_error')
        url=response.json().get('url')
        if not url:raise LoginError('unsupported_state')
        end=self._follow(url)
        self.result.device_id=self.session.cookies.get_dict().get('oai-did') or self.result.device_id
        return end
    def run(self):
        end=self._initialize_login()
        if not self._callback_has_code(end,self.redirect_uri):
            self.get_sentinel_token(self.result.device_id)
            step=self._post('/api/accounts/authorize/continue',{'username':{'value':self.input['email'],'kind':'email'},'screen_hint':'login'},'https://auth.openai.com/log-in','unsupported_state')
            kind,url=self._state(step);self._check_state(kind,url)
            if kind!='login_password' and '/log-in/password' not in url:raise LoginError('unsupported_state')
            step=self._post('/api/accounts/password/verify',{'password':self.input['password']},'https://auth.openai.com/log-in/password','password_rejected')
            kind,url=self._state(step);self._check_state(kind,url)
            if kind=='mfa_challenge' or '/mfa-challenge/' in url:
                challenge=urlparse(url).path.rstrip('/').split('/')[-1]
                if not challenge or challenge=='mfa-challenge':raise LoginError('unsupported_state')
                step=self._post('/api/accounts/mfa/verify',{'code':totp(self.input['mfa_secret']),'type':'totp','id':challenge},'https://auth.openai.com/mfa-challenge','mfa_rejected')
                kind,url=self._state(step);self._check_state(kind,url)
            if not url:raise LoginError('unsupported_state')
            end=self._follow(self.auth_url)
        if not self._callback_has_code(end,self.redirect_uri):raise LoginError('unsupported_state')
        query=parse_qs(urlparse(end).query)
        if query.get('state')!=[self.state]:raise LoginError('callback_state')
        response=self.session.post('https://auth.openai.com/oauth/token',headers={'Content-Type':'application/x-www-form-urlencoded','User-Agent':self._ua},data={'grant_type':'authorization_code','client_id':self.client_id,'code':query['code'][0],'redirect_uri':self.redirect_uri,'code_verifier':self.verifier},timeout=30)
        if response.status_code!=200:raise LoginError('token_exchange_failed')
        return validate_credentials(response.json(),self.input)

def main():
    import sys
    logging.disable(logging.CRITICAL)
    try:
        value=json.load(sys.stdin)
        result={'credential':LoginFlow(value).run()}
    except LoginError as e:result={'error':{'code':str(e)}}
    except Exception:result={'error':{'code':'network_error'}}
    print(json.dumps({'type':'result','payload':result},ensure_ascii=True))

if __name__=='__main__':main()
