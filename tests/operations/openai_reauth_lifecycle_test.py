import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

ROOT=Path(__file__).resolve().parents[2]
SCRIPT=ROOT/'ops/openai-reauth-worker.sh'

class LifecycleTest(unittest.TestCase):
    def fixture(self, directory):
        root=Path(directory)
        secret=root/'worker.env'
        secret.write_text('OPENAI_REAUTH_WORKER_TOKEN='+'x'*40+'\nSUB2API_BASE_URL=http://127.0.0.1:8080\n')
        secret.chmod(0o600)
        (root/'runtime.env').write_text(f'REAUTH_IMAGE=sha256:offline-test\nREAUTH_SOURCE_ROOT={ROOT}\nREAUTH_GO_CONTAINER=go-worker\n')
        install=root/'install'
        install.write_text('#!/bin/sh\nmkdir -p \"${@: -1}\"\n')
        install.write_text('#!/usr/bin/env bash\nmkdir -p \"${@: -1}\"\nchmod 700 \"${@: -1}\"\n')
        install.chmod(0o700)
        docker=root/'docker'
        docker.write_text('#!'+sys.executable+'\n'+r'''import json,os,sys
from pathlib import Path
a=sys.argv[1:]
with open(os.environ['DOCKER_CALLS'],'a') as f: f.write(json.dumps(a)+'\n')
if a[0]=='inspect':
    if a[-1]=='go-worker': print('true|healthy|go-container-id')
    elif os.environ.get('MOCK_MATCH')=='1':
        print('true|sha256:offline-test|container:go-container-id|'+os.environ['MOCK_SOURCE']+'/infra/reauth-worker/supervisor.py|unless-stopped')
    elif os.environ.get('MOCK_OLD_NAMESPACE')=='1':
        if '-f' in a and 'NetworkMode' in a[2]: print('true|sha256:offline-test|container:old-go-id|'+os.environ['MOCK_SOURCE']+'/infra/reauth-worker/supervisor.py|unless-stopped')
        elif '-f' in a: print('false')
    elif os.environ.get('MOCK_RUNNING')=='1':
        if '-f' in a: print('true')
    else: sys.exit(1)
elif a[0]=='run': print('mock-container-id')
''')
        docker.chmod(0o700)
        return dict(os.environ,REAUTH_CONFIG_DIR=str(root),PATH=str(root)+os.pathsep+os.environ['PATH'],DOCKER_CALLS=str(root/'calls.jsonl'),MOCK_SOURCE=str(ROOT))

    def test_missing_configuration_is_noop(self):
        with tempfile.TemporaryDirectory() as directory:
            env=dict(os.environ,REAUTH_CONFIG_DIR=directory)
            for action in ['pause','resume']:
                result=subprocess.run(['bash',str(SCRIPT),action],env=env,capture_output=True,text=True)
                self.assertEqual(result.returncode,0,result.stderr)

    def test_drain_timeout_never_forces_child_or_removes_container(self):
        with tempfile.TemporaryDirectory() as directory:
            env=self.fixture(directory)
            env.update(MOCK_RUNNING='1',REAUTH_DRAIN_TIMEOUT_SECONDS='0')
            result=subprocess.run(['bash',str(SCRIPT),'pause'],env=env,capture_output=True,text=True)
            self.assertEqual(result.returncode,1)
            calls=[json.loads(x) for x in Path(env['DOCKER_CALLS']).read_text().splitlines()]
            self.assertIn(['update','--restart=no','sub2api-openai-reauth-worker'],calls)
            self.assertIn(['kill','--signal','TERM','sub2api-openai-reauth-worker'],calls)
            self.assertFalse(any(x[0] in ['stop','rm','run'] for x in calls))
            self.assertIn('login remains running',result.stderr)

    def test_resume_uses_shared_namespace_and_nonclaiming_health(self):
        with tempfile.TemporaryDirectory() as directory:
            env=self.fixture(directory)
            result=subprocess.run(['bash',str(SCRIPT),'resume'],env=env,capture_output=True,text=True)
            self.assertEqual(result.returncode,0,result.stderr)
            calls=[json.loads(x) for x in Path(env['DOCKER_CALLS']).read_text().splitlines()]
            run=next(x for x in calls if x[0]=='run')
            self.assertEqual(run[run.index('--network')+1],'container:go-container-id')
            self.assertIn('--read-only',run)
            self.assertIn('python /app/reauth-supervisor.py --health',run)
            self.assertNotIn('--once',run)

    def test_resume_is_idempotent_for_current_namespace(self):
        with tempfile.TemporaryDirectory() as directory:
            env=self.fixture(directory)
            env['MOCK_MATCH']='1'
            result=subprocess.run(['bash',str(SCRIPT),'resume'],env=env,capture_output=True,text=True)
            self.assertEqual(result.returncode,0,result.stderr)
            calls=[json.loads(x) for x in Path(env['DOCKER_CALLS']).read_text().splitlines()]
            self.assertTrue(all(x[0]=='inspect' for x in calls),calls)

    def test_resume_replaces_old_namespace_after_pause(self):
        with tempfile.TemporaryDirectory() as directory:
            env=self.fixture(directory)
            env['MOCK_OLD_NAMESPACE']='1'
            result=subprocess.run(['bash',str(SCRIPT),'resume'],env=env,capture_output=True,text=True)
            self.assertEqual(result.returncode,0,result.stderr)
            calls=[json.loads(x) for x in Path(env['DOCKER_CALLS']).read_text().splitlines()]
            operations=[x[0] for x in calls]
            self.assertLess(operations.index('update'),operations.index('rm'))
            self.assertLess(operations.index('rm'),operations.index('run'))

if __name__=='__main__': unittest.main()
