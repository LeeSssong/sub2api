#!/usr/bin/env python3
"""Publish compiled homepage alongside the live production edge, with rollback."""
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import time

ROOT = Path('/opt/sub2api/production')
STATE = Path('/var/lib/sub2api/release-state')


def run(args, data=None):
    p = subprocess.run(args, input=data, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if p.returncode:
        raise RuntimeError(args[0] + ' failed, exit=' + str(p.returncode))
    return p.stdout


def route(config, service, assets):
    if not re.fullmatch(r'sub2api-homepage-[a-f0-9]{12}', service):
        raise ValueError('invalid service')
    if not assets or any(not re.fullmatch(r'home-assets/[A-Za-z0-9_.-]+', x) for x in assets):
        raise ValueError('invalid assets')
    marker = '{$SITE_ADDRESS} {\n'
    if config.count(marker) != 1:
        raise ValueError('expected one production site')
    # Exact new hashes win; old assets, production config, docs and API keep their routes.
    block = ('\t# production-homepage ' + service + '\n'
             '\t@' + service.replace('-', '_') + ' path / /home /home/ /support /support/ ' + ' '.join('/'+x for x in assets) + '\n'
             '\thandle @' + service.replace('-', '_') + ' {\n'
             '\t\treverse_proxy ' + service + ':80\n\t}\n')
    return config.replace(marker, marker+block)


def private_write(path, text):
    tmp = path.with_name('.'+path.name+'.new')
    tmp.write_text(text)
    tmp.chmod(0o600)
    os.replace(tmp, path)


def deploy(bundle):
    manifest = json.loads((bundle/'manifest.json').read_text())
    commit = manifest['source_commit']
    if not re.fullmatch(r'[a-f0-9]{40}', commit):
        raise ValueError('invalid source')
    state = json.loads(STATE.read_text())
    if (state['source_commit'], state['source_tree']) != (commit, manifest['source_tree']):
        raise ValueError('application source differs from homepage')
    lock = Path('/var/lib/sub2api/release-records/.blue-green.lock')
    # This is the same directory lock used by the application release controller.
    lock.mkdir(mode=0o700)
    private_write(lock/'owner.pid', str(os.getpid())+'\n')
    started = time.monotonic()
    try:
        caddy = json.loads(run(['docker','inspect','sub2api-caddy-1']))[0]
        config_path = ROOT/'Caddyfile'
        if config_path.is_symlink() or not any(m['Source']==str(config_path) and m['Destination']=='/etc/caddy/Caddyfile' for m in caddy['Mounts']):
            raise ValueError('unexpected Caddy config mount')
        if caddy['Image'] != manifest['base_image_id']:
            raise ValueError('Caddy dependency image changed')
        for name, sha in manifest['checksums'].items():
            path = bundle/'dist'/name
            if path.is_symlink() or not path.is_file() or '..' in Path(name).parts or hashlib.sha256(path.read_bytes()).hexdigest()!=sha:
                raise ValueError('artifact mismatch')
        for path in (bundle/'dist').rglob('*'):
            if path.is_symlink() or (path.is_file() and str(path.relative_to(bundle/'dist')) not in manifest['checksums']):
                raise ValueError('unmanifested artifact')
        service = 'sub2api-homepage-'+commit[:12]
        image = service+':'+commit
        base = caddy['Config']['Image']
        if json.loads(run(['docker','image','inspect',base]))[0]['Id'] != caddy['Image']:
            raise ValueError('base tag changed')
        (bundle/'Dockerfile').write_text('FROM '+base+'\nCOPY dist/ /srv/home/\nCOPY Caddyfile /etc/caddy/Caddyfile\n')
        (bundle/'Caddyfile').write_text(':80 {\n root * /srv/home\n try_files {path} /index.html\n file_server\n}\n')
        run(['docker','build','--pull=false','-t',image,str(bundle)])
        image_id = json.loads(run(['docker','image','inspect',image]))[0]['Id']
        network = next(iter(caddy['NetworkSettings']['Networks']))
        run(['docker','run','-d','--name',service,'--network',network,'--restart','unless-stopped',image])
        for name, sha in manifest['checksums'].items():
            actual = run(['docker','exec',service,'sha256sum','/srv/home/'+name]).decode().split()[0]
            if actual != sha:
                raise ValueError('running artifact mismatch')
        deadline = time.monotonic()+30
        while True:
            try:
                html = run(['docker','exec',caddy['Id'],'wget','-q','-O','-','http://'+service+'/'])
                if hashlib.sha256(html).hexdigest() != manifest['checksums']['index.html']:
                    raise ValueError('internal index mismatch')
                break
            except RuntimeError:
                if time.monotonic()>=deadline: raise
                time.sleep(1)
        old = config_path.read_text()
        release = ROOT/'homepage-releases'/commit
        release.mkdir(parents=True,mode=0o700)
        private_write(release/'previous-Caddyfile',old)
        assets = [x for x in manifest['checksums'] if x.startswith('home-assets/') and x.endswith(('.js','.css'))]
        new = route(old,service,assets)
        env = 'SUB2API_ACTIVE_UPSTREAM='+state['active_upstream']
        def reload(text):
            for action in ['validate','reload']:
                run(['docker','exec','-i','-e',env,caddy['Id'],'caddy',action,'--config','-','--adapter','caddyfile'],text.encode())
        reload(old)  # Establish a known working recovery path before cutover.
        try:
            reload(new)
            private_write(config_path,new)
            for name in ['index.html',*assets]:
                url='https://api.xingqiaolab.top/'+('' if name=='index.html' else name)
                data=run(['curl','--fail','--silent','--show-error','--max-time','15','--resolve','api.xingqiaolab.top:443:127.0.0.1',url])
                if hashlib.sha256(data).hexdigest()!=manifest['checksums'][name]: raise ValueError('origin artifact mismatch')
        except Exception:
            reload(old)
            private_write(config_path,old)
            raise
        private_write(release/'deployment.json',json.dumps(dict(manifest,image_id=image_id,service=service,result='succeeded',seconds=round(time.monotonic()-started,2),rollback_config=str(release/'previous-Caddyfile')),indent=2)+'\n')
        print(json.dumps({'result':'succeeded','image_id':image_id,'service':service,'seconds':round(time.monotonic()-started,2),'rollback_config':str(release/'previous-Caddyfile')}))
    finally:
        (lock/'owner.pid').unlink()
        lock.rmdir()


if __name__ == '__main__':
    if os.geteuid()!=0 or ROOT.is_symlink(): raise SystemExit('protected root required')
    deploy(Path(sys.argv[1]).resolve())
