import importlib.util
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import time
import unittest

ROOT = Path(__file__).resolve().parents[2]
SUPERVISOR = ROOT / 'infra/reauth-worker/supervisor.py'

class SupervisorTest(unittest.TestCase):
    def test_term_finishes_current_child_without_claiming_again(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            started, finished = root/'started', root/'finished'
            child = root/'child.py'
            child.write_text(f'import time\nfrom pathlib import Path\nPath({str(started)!r}).write_text("started")\ntime.sleep(1)\nPath({str(finished)!r}).write_text("finished")\n')
            harness = root/'harness.py'
            harness.write_text(f'import importlib.util,sys\ns=importlib.util.spec_from_file_location("supervisor",{str(SUPERVISOR)!r})\nm=importlib.util.module_from_spec(s)\ns.loader.exec_module(m)\nraise SystemExit(m.main([sys.executable,{str(child)!r}]))\n')
            env = dict(os.environ, REAUTH_STATE_FILE=str(root/'state.json'), OPENAI_REAUTH_POLL_SECONDS='1')
            process = subprocess.Popen([sys.executable,str(harness)],env=env)
            try:
                deadline=time.monotonic()+5
                while not started.exists() and time.monotonic()<deadline:
                    time.sleep(.02)
                self.assertTrue(started.exists())
                process.send_signal(signal.SIGTERM)
                self.assertEqual(process.wait(timeout=5),0)
                self.assertTrue(finished.exists(), 'TERM must not kill the protocol child')
                self.assertFalse((root/'state.json').exists())
            finally:
                if process.poll() is None:
                    process.kill()
                    process.wait()

    def test_health_does_not_invoke_worker(self):
        with tempfile.TemporaryDirectory() as directory:
            state=Path(directory)/'state.json'
            env=dict(os.environ,REAUTH_STATE_FILE=str(state))
            for phase, expected in [('running',0),('idle',0),('draining',1),('failed',1)]:
                state.write_text(json.dumps({'pid':os.getpid(),'phase':phase}))
                result=subprocess.run([sys.executable,str(SUPERVISOR),'--health'],env=env)
                self.assertEqual(result.returncode,expected)

if __name__=='__main__':
    unittest.main()
