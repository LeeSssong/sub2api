# SPDX-License-Identifier: AGPL-3.0-only
"""Loopback-only executor. Every login has an isolated, killable process group."""
from __future__ import annotations
import hmac,io,json,os,signal,subprocess,sys,tarfile,threading,time,select,socket
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
from pathlib import Path
from login import validate_input,LoginError
ROOT=Path(__file__).resolve().parent
SLOTS=threading.BoundedSemaphore(1)
CHILDREN=set()
LOCK=threading.Lock()

def source_archive():
    out=io.BytesIO()
    with tarfile.open(fileobj=out,mode='w:gz') as archive:
        for path in sorted(ROOT.iterdir()):
            if path.is_file() and (path.suffix in ('.py','.js','.md','.txt') or path.name=='LICENSE'):
                archive.add(path,arcname='token-guard/'+path.name)
        for name in ('Dockerfile','docker-entrypoint.sh'):
            path=ROOT/'build'/name
            if path.is_file():archive.add(path,arcname='token-guard/build/'+name)
    return out.getvalue()

class Handler(BaseHTTPRequestHandler):
    def log_message(self,*args):pass
    def setup(self):
        super().setup();self.connection.settimeout(15)
    def send(self,status,body,kind='application/x-ndjson'):
        self.send_response(status);self.send_header('Content-Type',kind);self.send_header('Cache-Control','no-store');self.send_header('Content-Length',str(len(body)));self.end_headers();self.wfile.write(body)
    def error(self,code,status=200):self.send(status,json.dumps({'type':'result','payload':{'error':{'code':code}}}).encode())
    def authenticated(self):
        key=os.environ.get('TOKEN_GUARD_RELOGIN_KEY','')
        return bool(key) and hmac.compare_digest(self.headers.get('Authorization',''),'Bearer '+key)
    def do_GET(self):
        if self.path=='/health':return self.send(200,b'{"status":"ok"}','application/json')
        if not self.authenticated():return self.error('unauthorized',401)
        if self.path=='/source.tar.gz':return self.send(200,source_archive(),'application/gzip')
        self.error('not_found',404)
    def do_POST(self):
        if not self.authenticated():return self.error('unauthorized',401)
        if self.path!='/relogin':return self.error('not_found',404)
        try:
            length=int(self.headers.get('Content-Length','0'))
            if not 0<length<=32768:raise ValueError()
            value=validate_input(json.loads(self.rfile.read(length)))
        except Exception:return self.error('invalid_credentials',400)
        if not SLOTS.acquire(blocking=False):return self.error('busy',429)
        proc=None
        try:
            proc=subprocess.Popen([sys.executable,str(ROOT/'login.py')],stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,start_new_session=True)
            with LOCK:CHILDREN.add(proc)
            deadline=time.monotonic()+180
            payload=json.dumps(value).encode()
            while True:
                try:
                    stdout,_=proc.communicate(payload,timeout=.25)
                    break
                except subprocess.TimeoutExpired:
                    payload=None
                    if select.select([self.connection],[],[],0)[0] and not self.connection.recv(1,socket.MSG_PEEK):
                        os.killpg(proc.pid,signal.SIGKILL);proc.communicate();return
                    if time.monotonic()>=deadline:
                        os.killpg(proc.pid,signal.SIGKILL);proc.communicate();return self.error('timeout')
            if proc.returncode!=0 or len(stdout)>65536:return self.error('executor_failed')
            # Reject malformed output rather than reflecting arbitrary subprocess messages.
            data=json.loads(stdout)
            if data.get('type')!='result' or not isinstance(data.get('payload'),dict):raise ValueError()
            self.send(200,stdout)
        except (BrokenPipeError,ConnectionResetError):pass
        except Exception:self.error('executor_failed')
        finally:
            if proc:
                with LOCK:CHILDREN.discard(proc)
                if proc.poll() is None:
                    os.killpg(proc.pid,signal.SIGKILL);proc.wait()
            SLOTS.release()

def main():
    if not os.environ.get('TOKEN_GUARD_RELOGIN_KEY'):raise SystemExit('TOKEN_GUARD_RELOGIN_KEY required')
    server=ThreadingHTTPServer(('127.0.0.1',8791),Handler)
    def stop(*_):
        with LOCK:
            for child in list(CHILDREN):
                try:os.killpg(child.pid,signal.SIGKILL)
                except ProcessLookupError:pass
        raise SystemExit(0)
    signal.signal(signal.SIGTERM,stop);signal.signal(signal.SIGINT,stop)
    server.serve_forever()
if __name__=='__main__':main()
