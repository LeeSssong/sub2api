#!/usr/bin/env python3
"""Publish only the compiled homepage; keep API, data and the public Caddy running."""
import fcntl
import hashlib
from html.parser import HTMLParser
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tarfile
import time
import urllib.request

ROOT = Path('/opt/sub2api-test-station')
PROJECT = 'sub2api-test-station'
SSH = ['ssh', '-T', '-o', 'BatchMode=yes', '-o', 'StrictHostKeyChecking=yes', '-o', 'ConnectTimeout=15',
       '-o', 'ServerAliveInterval=10', 'sub2api-test-station']


def run(args, data=None):
    result = subprocess.run(args, input=data, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if result.returncode:
        raise RuntimeError('command failed: ' + args[0] + ' exit=' + str(result.returncode))
    return result.stdout.decode()


def save(path, value):
    path = Path(path)
    tmp = path.with_suffix(path.suffix + '.new')
    tmp.write_text(value if isinstance(value, str) else json.dumps(value, indent=2) + '\n')
    tmp.chmod(0o600)
    os.replace(tmp, path)


def homepage_route(config, service, files):
    if not re.fullmatch(r'homepage-[a-z0-9]+', service):
        raise ValueError('invalid homepage service')
    if not files or any(not re.fullmatch(r'home-assets/[a-zA-Z0-9_.-]+', name) for name in files):
        raise ValueError('invalid homepage asset path')
    if not config.startswith(':80 {\n'):
        raise ValueError('unexpected test-station Caddy config')
    # Preserve earlier hashed assets for already-open tabs. The new route wins
    # for shared files, while older version containers remain recovery artifacts.
    pattern = r'  # homepage-brand-release begin\n.*?  # homepage-brand-release end\n'
    old = re.search(pattern, config, re.S)
    if old:
        archive = old.group(0)
        suffix = hashlib.sha256(archive.encode()).hexdigest()[:12]
        archive = archive.replace('homepage_brand_release', 'homepage_brand_archive_' + suffix)
        archive = archive.replace(' path / /home /home/ ', ' path ')
        archive = archive.replace('# homepage-brand-release', '# homepage-brand-archive-' + suffix)
        config = config[:old.start()] + archive + config[old.end():]
    block = ('  # homepage-brand-release begin\n'
             '  @homepage_brand_release path / /home /home/ ' + ' '.join('/' + name for name in files) + '\n'
             '  handle @homepage_brand_release {\n    reverse_proxy ' + service + ':80\n  }\n'
             '  # homepage-brand-release end\n')
    return config.replace(':80 {\n', ':80 {\n' + block, 1)


def verify_favicon(html, logo):
    class Icons(HTMLParser):
        def __init__(self):
            super().__init__()
            self.icons = []

        def handle_starttag(self, tag, attributes):
            attrs = dict(attributes)
            if tag == 'link' and attrs.get('rel') == 'icon':
                self.icons.append(attrs.get('href'))
    parser = Icons()
    parser.feed(html)
    if parser.icons != [logo]:
        raise ValueError('favicon differs from public branding')


def inspect(container):
    return json.loads(run(['docker', 'inspect', container]))[0]


def reload_caddy(container, config):
    for action in ('validate', 'reload'):
        run(['docker', 'exec', '-i', container, 'caddy', action, '--config', '-', '--adapter', 'caddyfile'], config.encode())


def check_public(manifest):
    for path, expected in [('/health', {'status': 'ok'}), ('/readyz', {'status': 'ready'})]:
        with urllib.request.urlopen('http://127.0.0.1' + path, timeout=10) as response:
            if json.load(response) != expected:
                raise ValueError('public health failed')
    for name, checksum in manifest['checksums'].items():
        if name == 'index.html' or name.endswith(('.js', '.css')):
            path = '/' if name == 'index.html' else '/' + name
            with urllib.request.urlopen('http://127.0.0.1' + path, timeout=15) as response:
                if hashlib.sha256(response.read()).hexdigest() != checksum:
                    raise ValueError('public artifact checksum mismatch')


def deploy(bundle):
    started = time.monotonic()
    manifest = json.loads((bundle / 'manifest.json').read_text())
    commit, tree = manifest['source_commit'], manifest['source_tree']
    if not all(re.fullmatch('[a-f0-9]{40}', value) for value in (commit, tree)):
        raise ValueError('invalid source identity')
    image = manifest['image_tag']
    if not re.fullmatch(PROJECT + r'-homepage:[a-f0-9]{40}', image) or hashlib.sha256((bundle / 'image.tar').read_bytes()).hexdigest() != manifest['image_archive_sha256']:
        raise ValueError('image checksum mismatch')
    previous = json.loads((ROOT / 'release-state.json').read_text())
    if previous['source_commit'] != manifest['previous_commit']:
        raise ValueError('API release changed during preparation')
    cid = run(['docker', 'ps', '-q', '--filter', 'label=com.docker.compose.project=' + PROJECT,
               '--filter', 'label=com.docker.compose.service=test-station-caddy']).strip()
    if not cid or '\n' in cid:
        raise ValueError('expected one public Caddy')
    caddy = inspect(cid)
    path = Path(next(m['Source'] for m in caddy['Mounts'] if m['Destination'] == '/etc/caddy/Caddyfile'))
    if path.is_symlink() or not str(path).startswith(str(ROOT / 'releases') + '/'):
        raise ValueError('unsafe Caddy config path')
    # Earlier API releases atomically replaced this bind-mounted file. The
    # container can retain an old inode while Caddy correctly uses stdin reload.
    # Gate on the restart config matching the live admin API, not the stale inode.
    old_config = path.read_text()
    # Confirm that the file also describes the currently loaded configuration.
    adapted = json.loads(run(['docker', 'exec', '-i', cid, 'caddy', 'adapt', '--config', '-', '--adapter', 'caddyfile'], old_config.encode()))
    loaded = json.loads(run(['docker', 'exec', cid, 'wget', '-qO-', 'http://127.0.0.1:2019/config/']))
    if adapted != loaded:
        raise ValueError('persisted and live Caddy config differ')
    release = ROOT / 'homepage-releases' / commit
    release.mkdir(parents=True, mode=0o700)
    service = 'homepage-' + commit[:12]
    files = sorted(name for name in manifest['checksums'] if name.startswith('home-assets/'))
    config = homepage_route(old_config, service, files)
    save(release / 'previous-Caddyfile', old_config)
    save(release / 'Caddyfile', config)
    shutil.copyfile(bundle / 'manifest.json', release / 'manifest.json')
    record = dict(manifest, result='prepared', rolled_back=False, caddy_container=cid, caddy_host_path=str(path),
                  candidate_service=service, previous_homepage=previous.get('homepage'), stage_seconds={})
    save(release / 'deployment.json', record)
    run(['docker', 'load', '-i', str(bundle / 'image.tar')])
    if run(['docker', 'image', 'inspect', '--format', '{{.Id}}', image]).strip() != manifest['image_id']:
        raise ValueError('loaded image identity mismatch')
    compose = {'name': PROJECT, 'services': {service: {'image': image, 'restart': 'unless-stopped',
               'networks': ['test-station'], 'stop_grace_period': '300s'}},
               'networks': {'test-station': {'external': True, 'name': PROJECT + '-network'}}}
    save(release / 'compose.json', compose)
    run(['docker', 'compose', '-p', PROJECT, '-f', str(release / 'compose.json'), 'up', '-d', '--no-deps', service])
    candidate = run(['docker', 'compose', '-p', PROJECT, '-f', str(release / 'compose.json'), 'ps', '-q', service]).strip()
    try:
        deadline = time.monotonic() + 30
        while True:
            try:
                index = run(['docker', 'exec', candidate, 'wget', '-qO-', 'http://127.0.0.1/'])
                if hashlib.sha256(index.encode()).hexdigest() != manifest['checksums']['index.html']:
                    raise ValueError('candidate homepage differs')
                break
            except RuntimeError:
                if time.monotonic() >= deadline:
                    raise
                time.sleep(1)
        record['stage_seconds']['load_and_ready'] = round(time.monotonic() - started, 2)
        reload_caddy(cid, config)
        check_public(manifest)
        # Keep the inode, because the public Caddy uses a read-only file bind.
        # Runtime changes are atomic via reload; persist only after verification.
        with path.open('w') as f:
            f.write(config)
            f.flush()
            os.fsync(f.fileno())
        if path.read_text() != config:
            raise ValueError('persistent routing mismatch')
        previous['homepage'] = {'source_commit': commit, 'source_tree': tree, 'image_id': manifest['image_id'],
                                'release_dir': str(release), 'service': service}
        save(ROOT / 'release-state.json', previous)
        record.update(result='succeeded', candidate_container=candidate, duration_seconds=round(time.monotonic() - started, 2))
        save(release / 'deployment.json', record)
        print(json.dumps({'result': 'succeeded', 'release_dir': str(release), 'image_id': manifest['image_id']}))
    except Exception:
        reload_caddy(cid, old_config)
        with path.open('w') as f:
            f.write(old_config)
        record.update(result='failed', rolled_back=True)
        save(release / 'deployment.json', record)
        raise


def publish(reuse=None):
    root = Path.cwd()
    if not (root / '.git').is_dir() or run(['git', 'branch', '--show-current']).strip() != 'main' or run(['git', 'status', '--porcelain']).strip():
        raise ValueError('use the clean root main checkout')
    run(['git', 'fetch', 'origin', 'main'])
    commit, tree = [run(['git', 'rev-parse', ref]).strip() for ref in ('HEAD', 'HEAD^{tree}')]
    if [commit, tree] != [run(['git', 'rev-parse', ref]).strip() for ref in ('origin/main', 'origin/main^{tree}')]:
        raise ValueError('main differs from origin/main')
    config = run(['ssh', '-G', 'sub2api-test-station'])
    if not all(line in config.splitlines() for line in ('hostname 43.133.75.82', 'user ubuntu', 'port 22', 'stricthostkeychecking true')):
        raise ValueError('SSH route mismatch')
    previous = json.loads(run(SSH + ['sudo -n cat /opt/sub2api-test-station/release-state.json']))
    stage = root / '.release' / ('homepage-' + commit)
    stage.mkdir(parents=True)
    if reuse:
        reuse = Path(reuse).resolve()
        if reuse.parent != root / '.release' or reuse.is_symlink():
            raise ValueError('reuse requires a local release artifact')
        artifact = json.loads((reuse / 'manifest.json').read_text())
        original_commit = artifact.get('artifact_source_commit', artifact['source_commit'])
        run(['git', 'diff', '--exit-code', original_commit, 'HEAD', '--', 'homepage', 'infra/independent-test-station/Dockerfile.homepage'])
        if hashlib.sha256((reuse / 'image.tar').read_bytes()).hexdigest() != artifact['image_archive_sha256']:
            raise ValueError('reused artifact checksum mismatch')
        shutil.copyfile(reuse / 'image.tar', stage / 'image.tar')
        manifest = dict(artifact, source_commit=commit, source_tree=tree, previous_commit=previous['source_commit'],
                        artifact_source_commit=original_commit, artifact_source_tree=artifact.get('artifact_source_tree', artifact['source_tree']))
    else:
        manifest = build_artifact(root, stage, commit, tree, previous)
    if run(['git', 'status', '--porcelain']).strip() or commit != run(['git', 'rev-parse', 'HEAD']).strip():
        raise ValueError('source changed while building')
    save(stage / 'manifest.json', manifest)
    shutil.copyfile(__file__, stage / 'deploy.py')
    with tarfile.open(stage / 'bundle.tar.gz', 'w:gz') as archive:
        for name in ('image.tar', 'manifest.json', 'deploy.py'):
            archive.add(stage / name, arcname=name)
    remote = run(SSH + ['mktemp -d /var/tmp/sub2api-test-station-homepage.XXXXXX']).strip()
    if not re.fullmatch('/var/tmp/sub2api-test-station-homepage.[a-zA-Z0-9]+', remote):
        raise ValueError('unsafe remote staging path')
    run(['scp', '-q', '-o', 'BatchMode=yes', '-o', 'StrictHostKeyChecking=yes', str(stage / 'bundle.tar.gz'), 'sub2api-test-station:' + remote + '/bundle.tar.gz'])
    checksum = hashlib.sha256((stage / 'bundle.tar.gz').read_bytes()).hexdigest()
    command = "test \"$(sha256sum '" + remote + "/bundle.tar.gz' | cut -d' ' -f1)\" = '" + checksum + "' && tar -xzf '" + remote + "/bundle.tar.gz' -C '" + remote + "' && sudo -n python3 '" + remote + "/deploy.py' deploy '" + remote + "'"
    print(run(SSH + [command]), end='')


def build_artifact(root, stage, commit, tree, previous):
    subprocess.run(['npm', 'run', 'build'], cwd=root / 'homepage', check=True)
    dist = stage / 'dist'
    dist.mkdir()
    shutil.copyfile(root / 'homepage/dist/index.html', dist / 'index.html')
    shutil.copytree(root / 'homepage/dist/home-assets', dist / 'home-assets')
    (stage / 'Caddyfile').write_text(':80 {\n root * /srv/home\n @home path / /home /home/\n header @home Cache-Control "no-store, max-age=0"\n rewrite @home /index.html\n @config path /home-assets/site-config.json\n header @config Cache-Control "no-store, max-age=0"\n @assets path /home-assets/*\n header @assets Cache-Control "public, max-age=31536000, immutable"\n file_server\n}\n')
    shutil.copyfile(root / 'infra/independent-test-station/Dockerfile.homepage', stage / 'Dockerfile')
    image = PROJECT + '-homepage:' + commit
    subprocess.run(['docker', 'buildx', 'build', '--platform', 'linux/amd64', '--load', '-t', image, str(stage)], check=True)
    image_id = run(['docker', 'image', 'inspect', '--format', '{{.Id}}', image]).strip()
    run(['docker', 'save', '-o', str(stage / 'image.tar'), image])
    manifest = {'source_commit': commit, 'source_tree': tree, 'previous_commit': previous['source_commit'],
                'image_tag': image, 'image_id': image_id, 'image_archive_sha256': hashlib.sha256((stage / 'image.tar').read_bytes()).hexdigest(),
                'checksums': {str(path.relative_to(dist)): hashlib.sha256(path.read_bytes()).hexdigest() for path in dist.rglob('*') if path.is_file()}}
    return manifest


if __name__ == '__main__':
    if sys.argv[1:] and sys.argv[1] == 'deploy':
        bundle = Path(sys.argv[2]).resolve()
        if os.geteuid() != 0 or not str(bundle).startswith('/var/tmp/sub2api-test-station-homepage.'):
            raise ValueError('protected test-station staging required')
        lock = open(ROOT / '.api-release.lock', 'a')
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        deploy(bundle)
    else:
        if sys.argv[1:] and (len(sys.argv) != 3 or sys.argv[1] != '--reuse'):
            raise ValueError('usage: release-test-station-homepage.py [--reuse LOCAL_ARTIFACT]')
        publish(sys.argv[2] if len(sys.argv) == 3 else None)
