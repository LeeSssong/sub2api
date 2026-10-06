#!/usr/bin/env python3
"""Manage only Prism services; never start, stop or recreate Sub2API containers."""
import fcntl
import json
import os
from pathlib import Path
import subprocess


def desired_namespace(container):
    if not container.get('State', {}).get('Running'):
        return None
    labels = container['Config'].get('Labels') or {}
    if labels.get('com.docker.compose.project') != 'sub2api':
        raise ValueError('unexpected Compose project')
    pid = container['State']['Pid']
    if not isinstance(pid, int) or pid <= 1:
        raise ValueError('invalid container PID')
    return container['Id'], f'/proc/{pid}/ns/net'


def run(*args, check=True):
    return subprocess.run(args, check=check, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)


def main():
    if os.geteuid() != 0:
        raise SystemExit('root required for namespace bindings')
    with open('/run/sub2api-prism-reconcile.lock', 'w') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        for slot in ('blue', 'green', 'worker'):
            name = f'sub2api-sub2api-{slot}-1'
            unit = f'sub2api-prism@{slot}.service'
            inspected = run('docker', 'inspect', name, check=False)
            if inspected.returncode:
                # Docker outage or deleted container: preserve services and retry.
                continue
            target = desired_namespace(json.loads(inspected.stdout)[0])
            dropin = Path('/run/systemd/system') / (unit + '.d') / 'namespace.conf'
            if target is None:
                run('systemctl', 'stop', unit)
                continue
            identity, namespace = target
            if not Path(namespace).exists():
                continue
            text = f'[Service]\nNetworkNamespacePath={namespace}\nEnvironment=PRISM_CONTAINER_ID={identity}\n'
            changed = not dropin.exists() or dropin.read_text() != text
            if changed:
                run('systemctl', 'stop', unit)
                dropin.parent.mkdir(parents=True, exist_ok=True)
                temporary = dropin.with_suffix('.tmp')
                temporary.write_text(text)
                temporary.chmod(0o644)
                temporary.replace(dropin)
                run('systemctl', 'daemon-reload')
            run('systemctl', 'start', unit)


if __name__ == '__main__':
    main()
