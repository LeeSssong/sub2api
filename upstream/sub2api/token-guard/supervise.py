# SPDX-License-Identifier: AGPL-3.0-only
"""Run the application and its private login executor with shared ephemeral auth."""
import os,secrets,signal,subprocess,sys,time,urllib.request
from pathlib import Path

def main():
    os.environ.setdefault('TOKEN_GUARD_RELOGIN_KEY',secrets.token_urlsafe(32))
    os.environ.setdefault('TOKEN_GUARD_RELOGIN_URL','http://127.0.0.1:8791/relogin')
    executor=subprocess.Popen([sys.executable,str(Path(__file__).with_name('server.py'))])
    app=None
    stopping=False
    def forward(sig,_frame):
        nonlocal stopping
        stopping=True
        if app and app.poll() is None:app.send_signal(sig)
        else:executor.terminate()
    signal.signal(signal.SIGTERM,forward);signal.signal(signal.SIGINT,forward)
    try:
        deadline=time.monotonic()+10
        while time.monotonic()<deadline:
            if executor.poll() is not None:raise RuntimeError('executor startup failed')
            try:
                with urllib.request.urlopen('http://127.0.0.1:8791/health',timeout=1) as response:
                    if response.status==200:break
            except Exception:time.sleep(.1)
        else:raise RuntimeError('executor readiness timeout')
        app=subprocess.Popen(sys.argv[1:])
        while app.poll() is None:
            if executor.poll() is not None:
                app.terminate();app.wait(timeout=30);return 1
            time.sleep(.25)
        return app.returncode
    finally:
        if app and app.poll() is None:app.terminate()
        if executor.poll() is None:
            executor.terminate()
            try:executor.wait(timeout=5)
            except subprocess.TimeoutExpired:executor.kill();executor.wait()

if __name__=='__main__':raise SystemExit(main())
