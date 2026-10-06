import unittest,sys,time,base64,json,ast
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch
sys.path.insert(0,str(Path(__file__).resolve().parents[1]))
from login import LoginFlow, LoginError, validate_input, validate_credentials, totp

SECRET='GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ'
def jwt(claims):
    return 'x.'+base64.urlsafe_b64encode(json.dumps(claims).encode()).decode().rstrip('=')+'.x'
def credential(email='test@example.com',aid='acct'):
    claims={'email':email,'exp':int(time.time())+3600,'https://api.openai.com/auth':{'chatgpt_account_id':aid}}
    return {'access_token':jwt(claims),'id_token':jwt(claims),'refresh_token':'rt','expires_in':3600}

class FakeTransport:
    def __init__(self,states):self.states=list(states);self.calls=[];self.cookies=SimpleNamespace(get_dict=lambda:{'oai-did':'device'},get=lambda *a:'device')
    def get(self,url,**kw):
        self.calls.append(('GET',url,kw));return SimpleNamespace(status_code=200,headers={},text='',json=lambda:{})
    def post(self,url,**kw):
        self.calls.append(('POST',url,kw));data=self.states.pop(0)
        return SimpleNamespace(status_code=200,headers={},text='',json=lambda:data)

class LoginTest(unittest.TestCase):
    def flow(self,states):
        transport=FakeTransport(states)
        flow=LoginFlow({'email':'test@example.com','password':' a,b ','mfa_secret':SECRET,'expected_account_id':'acct'},session=transport)
        flow.get_sentinel_token=lambda _: 'sentinel'
        flow._initialize_login=lambda: 'https://auth.openai.com/log-in'
        flow._follow=lambda url: 'http://localhost:1455/auth/callback?code=c&state='+flow.state
        return flow,transport
    def test_password_totp_oauth_and_exact_password(self):
        f,t=self.flow([{'page':{'type':'login_password'},'continue_url':'/log-in/password'},{'page':{'type':'mfa_challenge'},'continue_url':'/mfa-challenge/123'},{'continue_url':'/done'},credential()])
        result=f.run()
        self.assertEqual(result['refresh_token'],'rt')
        self.assertEqual(t.calls[1][2]['json']['password'],' a,b ')
        self.assertTrue(any('/mfa/verify' in c[1] for c in t.calls))
    def test_unwanted_states_stop_before_any_registration(self):
        for state,url in [('signup','/create-account'),('email_otp_verification','/email-verification'),('add_phone','/add-phone')]:
            with self.subTest(state=state):
                f,t=self.flow([{'page':{'type':state},'continue_url':url}])
                with self.assertRaises(LoginError):f.run()
                self.assertEqual(len([c for c in t.calls if c[0]=='POST']),1)
    def test_bad_state_never_exchanges_code(self):
        f,t=self.flow([]);f._initialize_login=lambda: 'http://localhost:1455/auth/callback?code=c&state=wrong'
        with self.assertRaisesRegex(LoginError,'callback_state'):f.run()
        self.assertFalse(any('/oauth/token' in c[1] for c in t.calls))
    def test_required_credentials(self):
        for field in ['email','password','mfa_secret']:
            v={'email':'test@example.com','password':'pw','mfa_secret':SECRET};v[field]=''
            with self.assertRaises(LoginError):validate_input(v)
    def test_totp_rfc_vector(self):self.assertEqual(totp(SECRET,59),'287082')
    def test_identity_and_expiry_fail_closed(self):
        v={'email':'test@example.com','expected_account_id':'acct'}
        for c in [credential('other@example.com'),credential(aid='other'),{'access_token':'bad','id_token':'bad','refresh_token':'rt'}]:
            with self.assertRaises(LoginError):validate_credentials(c,v)
    def test_no_registration_or_mail_implementation(self):
        root=Path(__file__).resolve().parents[1]
        for name in ['login.py','protocol_helpers.py']:
            text=(root/name).read_text()
            for endpoint in ['/user/register','/create_account','/email-otp/send','/phone-otp/','/mfa/enroll','/api/auth/session']:
                self.assertNotIn(endpoint,text)

if __name__=='__main__':unittest.main()

class TransportTest(unittest.TestCase):
    def test_forbidden_mutations_and_redirects(self):
        from transport import RestrictedSession
        raw=FakeTransport([]);s=RestrictedSession(raw)
        for url in ['https://auth.openai.com/api/accounts/user/register','https://auth.openai.com/api/accounts/create_account','https://auth.openai.com/api/accounts/email-otp/send','https://evil.example/oauth/token','http://auth.openai.com/oauth/token']:
            with self.assertRaises(ValueError):s.post(url,json={'password':'secret'})
        self.assertEqual(raw.calls,[])
        s.get('https://auth.openai.com/log-in',allow_redirects=True)
        self.assertFalse(raw.calls[0][2]['allow_redirects'])
    def test_error_output_does_not_reflect_secret(self):
        import subprocess
        result=subprocess.run([sys.executable,str(Path(__file__).resolve().parents[1]/'login.py')],input=json.dumps({'password':'SECRET-CANARY','mfa_secret':'KEY-CANARY'}),capture_output=True,text=True)
        self.assertNotIn('CANARY',result.stdout+result.stderr)
        self.assertEqual(json.loads(result.stdout)['payload']['error']['code'],'invalid_credentials')
    def test_source_archive_only_source(self):
        from server import source_archive
        import tarfile,io
        with tarfile.open(fileobj=io.BytesIO(source_archive()),mode='r:gz') as t:
            names=t.getnames()
        self.assertIn('token-guard/LICENSE',names)
        self.assertIn('token-guard/login.py',names)
        self.assertFalse(any('.json' in n or '__pycache__' in n for n in names))

class InitTest(unittest.TestCase):
    def test_latest_upstream_login_initialization(self):
        f,t=LoginTest().flow([{'url':'https://auth.openai.com/authorize?state=test'}])
        old=t.get
        def get(url,**kw):
            r=old(url,**kw)
            if '/csrf' in url:r.json=lambda:{'csrfToken':'csrf'}
            return r
        t.get=get
        LoginFlow._initialize_login(f)
        self.assertTrue(any('/api/auth/signin/openai?' in x[1] for x in t.calls))
        self.assertEqual(f.result.device_id,'device')
