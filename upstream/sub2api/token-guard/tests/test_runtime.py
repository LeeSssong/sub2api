"""Executor integration: HTTP authentication, cancellation and process supervision."""
import io,json,os,signal,socket,subprocess,sys,tempfile,time,unittest,urllib.request
from pathlib import Path
from unittest.mock import patch
sys.path.insert(0,str(Path(__file__).resolve().parents[1]))
import server
from http.server import ThreadingHTTPServer
import threading

class RuntimeTest(unittest.TestCase):
    def test_http_auth_and_disconnect_terminate_job(self):
        with tempfile.TemporaryDirectory() as temp:
            root=Path(temp)
            (root/'login.py').write_text('import time\ntime.sleep(30)\n')
            with patch.object(server,'ROOT',root),patch.dict(os.environ,{'TOKEN_GUARD_RELOGIN_KEY':'test-key'}):
                http=ThreadingHTTPServer(('127.0.0.1',0),server.Handler)
                thread=threading.Thread(target=http.serve_forever,daemon=True);thread.start()
                try:
                    port=http.server_port
                    try:urllib.request.urlopen(f'http://127.0.0.1:{port}/source.tar.gz')
                    except urllib.error.HTTPError as e:self.assertEqual(e.code,401)
                    else:self.fail('unauthenticated source accepted')
                    conn=socket.create_connection(('127.0.0.1',port))
                    data=json.dumps({'email':'test@example.com','password':'pw','mfa_secret':'GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ'}).encode()
                    conn.sendall(f'POST /relogin HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer test-key\r\nContent-Length: {len(data)}\r\n\r\n'.encode()+data)
                    deadline=time.monotonic()+3
                    while not server.CHILDREN and time.monotonic()<deadline:time.sleep(.02)
                    children=list(server.CHILDREN);self.assertEqual(len(children),1)
                    conn.close()
                    while children[0].poll() is None and time.monotonic()<deadline:time.sleep(.02)
                    self.assertIsNotNone(children[0].poll(),'disconnected login process survived')
                finally:http.shutdown();http.server_close();thread.join()
    def test_supervisor_propagates_app_exit_and_stops_executor(self):
        root=Path(__file__).resolve().parents[1]
        proc=subprocess.run([sys.executable,str(root/'supervise.py'),sys.executable,'-c','raise SystemExit(7)'],capture_output=True,timeout=15)
        self.assertEqual(proc.returncode,7,proc.stderr.decode())
        with socket.socket() as sock:self.assertNotEqual(sock.connect_ex(('127.0.0.1',8791)),0)
