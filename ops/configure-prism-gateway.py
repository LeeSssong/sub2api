#!/usr/bin/env python3
"""Atomically add the Prism bridge config to the protected host Compose file.

Does not recreate or restart any service. Apply with the normal blue/green
executor, preserving the worker. No credentials are printed.
"""
import json
import os
from pathlib import Path
import shutil
import stat


def configured(compose, key):
    if len(key) < 32:
        raise ValueError('bridge key too short')
    for name in ('sub2api-blue', 'sub2api-green'):
        env = compose['services'][name].setdefault('environment', {})
        env.update(GATEWAY_PRISM_BROWSER_ENABLED='true',
                   GATEWAY_PRISM_BROWSER_BASE_URL='http://127.0.0.1:8319/v1',
                   GATEWAY_PRISM_BROWSER_API_KEY=key)
    return compose


if __name__ == '__main__':
    config = Path('/etc/sub2api-prism.env')
    compose = Path('/opt/sub2api/production/compose.yaml')
    for path in (config, compose):
        metadata = path.lstat()
        assert stat.S_ISREG(metadata.st_mode) and metadata.st_uid == 0
        assert stat.S_IMODE(metadata.st_mode) == 0o600
    values = dict(line.strip().split('=', 1) for line in config.read_text().splitlines() if line and not line.startswith('#'))
    original = json.loads(compose.read_text())
    backup = compose.with_name('compose.yaml.before-prism')
    assert not backup.exists(), 'existing Prism backup; inspect before retrying'
    shutil.copy2(compose, backup)
    candidate = configured(original, values['PRISM_ADAPTER_API_KEY'])
    temp = compose.with_name('.compose-prism.tmp')
    fd = os.open(temp, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, 'w') as stream:
        json.dump(candidate, stream, indent=2)
        stream.flush()
        os.fsync(stream.fileno())
    os.replace(temp, compose)
    print('Prism API configuration staged; application containers unchanged')
