"""Exercise the release drain helper with a stateful Docker/psql double."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
SOURCE = (ROOT / 'ops/deploy-sub2api-blue-green-host.sh').read_text()
HELPER = SOURCE.split("<<'PY_WORKER_DRAIN'\n", 1)[1].split('\nPY_WORKER_DRAIN', 1)[0]
FAKE = '''#!/usr/bin/env python3
import os, sys
from pathlib import Path
root = Path(os.environ['DRAIN_TEST_ROOT'])
mode = os.environ['DRAIN_TEST_MODE']
if sys.argv[1] == 'exec':
    for line in sys.stdin:
        if line.startswith('FROM scheduled_test_plans;'):
            if mode == 'lost': sys.exit(1)
            (root / 'locked').touch()
            print('worker_guard|' + ('1|0' if mode == 'ordinary' else '0|1' if mode == 'busy' else '0|0'), flush=True)
        if line.strip() == 'ROLLBACK;':
            (root / 'locked').unlink(missing_ok=True)
            break
elif sys.argv[1] == 'stop':
    assert (root / 'locked').exists(), 'worker stopped without claim lock'
    assert (root / 'marker').exists(), 'worker stopped without recovery marker'
    (root / 'stopped').touch()
    if mode == 'stop_failure': sys.exit(1)
elif sys.argv[1] == 'inspect':
    print('false')
'''

class DrainTest(unittest.TestCase):
    def run_case(self, mode):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            (root / 'docker').write_text(FAKE)
            (root / 'docker').chmod(0o700)
            (root / 'helper.py').write_text(HELPER)
            env = dict(os.environ, PATH=folder + os.pathsep + os.environ['PATH'], DRAIN_TEST_ROOT=folder, DRAIN_TEST_MODE=mode)
            result = subprocess.run(['python3', str(root / 'helper.py'), 'postgres', 'worker', '0.1', str(root / 'marker')], env=env, capture_output=True, text=True, timeout=10)
            self.assertEqual(result.stdout, '', 'drain diagnostics must not corrupt release JSON')
            return result.returncode, (root / 'stopped').exists(), (root / 'marker').exists(), (root / 'locked').exists()

    def test_idle_stop_holds_lock_and_retains_recovery_marker(self):
        self.assertEqual(self.run_case('idle'), (0, True, True, False))

    def test_ordinary_plans_abort_without_worker_mutation(self):
        code, stopped, marker, locked = self.run_case('ordinary')
        self.assertNotEqual(code, 0)
        self.assertEqual((stopped, marker, locked), (False, False, False))

    def test_busy_plans_abort_without_worker_mutation(self):
        code, stopped, marker, locked = self.run_case('busy')
        self.assertNotEqual(code, 0)
        self.assertEqual((stopped, marker, locked), (False, False, False))

    def test_lost_database_session_never_stops_worker(self):
        code, stopped, marker, _ = self.run_case('lost')
        self.assertNotEqual(code, 0)
        self.assertEqual((stopped, marker), (False, False))

    def test_failed_stop_keeps_recovery_marker(self):
        code, stopped, marker, locked = self.run_case('stop_failure')
        self.assertNotEqual(code, 0)
        self.assertEqual((stopped, marker, locked), (True, True, False))

if __name__ == '__main__':
    unittest.main()
