#!/usr/bin/env python3
"""Disposable local Caddy reload proof. Requires cached node:24-alpine/caddy:2.10.2-alpine; never pulls."""
import base64, http.client, json, os, socket, subprocess, tempfile, time, uuid
from pathlib import Path

def run(*args):
    try:
        return subprocess.check_output(['docker', *args], text=True, stderr=subprocess.STDOUT).strip()
    except subprocess.CalledProcessError as exc:
        print(exc.output)
        raise

# Parse the actual deployment Caddyfile too: both public and internal hosts must
# contain both exact slot paths, and no path rewrite may strip the slot prefix.
root=Path(__file__).resolve().parents[2]
adapted=subprocess.run(['docker','run','--rm','-i','--network','none','--pull=never','-e','SITE_ADDRESS=https://fusion.invalid','--entrypoint','caddy','caddy:2.10.2-alpine','adapt','--config','-','--adapter','caddyfile'],input=(root/'infra/Caddyfile').read_text(),text=True,capture_output=True,check=True)
config_json=json.loads(adapted.stdout)
def objects(value):
    if isinstance(value,dict):
        yield value
        for child in value.values():yield from objects(child)
    elif isinstance(value,list):
        for child in value:yield from objects(child)
for slot in ['blue','green']:
    routes=[item for item in objects(config_json) if any('/api/bps-images/'+slot+'/*' in match.get('path',[]) for match in item.get('match',[]) if isinstance(match,dict))]
    assert len(routes)==2, 'both hosts require callback affinity'
    for route in routes:
        assert [item['dial'] for item in objects(route) if 'dial' in item]==['sub2api-'+slot+':8080']
        assert not any(item.get('handler')=='rewrite' for item in objects(route))

prefix='fusion-stream-'+uuid.uuid4().hex[:10]
network=prefix; backend=prefix+'-backend'; proxy=prefix+'-caddy'
with tempfile.TemporaryDirectory(prefix=prefix, dir=str(Path.home())) as tmp:
    directory=Path(tmp)
    (directory/'backend.js').write_text('''const http=require('http'),crypto=require('crypto');
for(const [port,label] of [[9001,'old'],[9002,'new']]){
 const s=http.createServer((req,res)=>{if(req.url==='/sse'){res.writeHead(200,{'Content-Type':'text/event-stream'});const tick=setInterval(()=>res.write('data: '+label+'\\n\\n'),100);res.on('close',()=>clearInterval(tick));}else res.end(label);});
 s.on('upgrade',(req,socket)=>{const key=crypto.createHash('sha1').update(req.headers['sec-websocket-key']+'258EAFA5-E914-47DA-95CA-C5AB0DC85B11').digest('base64');socket.write('HTTP/1.1 101 Switching Protocols\\r\\nUpgrade: websocket\\r\\nConnection: Upgrade\\r\\nSec-WebSocket-Accept: '+key+'\\r\\n\\r\\n');const tick=setInterval(()=>socket.write(Buffer.from([0x81,label.length,...Buffer.from(label)])),100);socket.on('close',()=>clearInterval(tick));socket.on('error',()=>clearInterval(tick));});s.listen(port,'0.0.0.0');}
''')
    def config(port):
        return ':8080 {\n @blue path /api/bps-images/blue/*\n reverse_proxy @blue '+backend+':9001\n @green path /api/bps-images/green/*\n reverse_proxy @green '+backend+':9002\n reverse_proxy '+backend+':'+str(port)+' {\n  stream_close_delay 24h\n  flush_interval -1\n }\n}\n'
    (directory/'Caddyfile').write_text(config(9001))
    try:
        run('network','create',network)
        run('run','--pull=never','-d','--name',backend,'--network',network,'-v',tmp+':/fixture:ro','node:24-alpine','node','/fixture/backend.js')
        run('run','--pull=never','-d','--name',proxy,'--network',network,'-p','127.0.0.1::8080','-v',tmp+':/fixture:ro','caddy:2.10.2-alpine','caddy','run','--config','/fixture/Caddyfile','--adapter','caddyfile')
        port=int(run('port',proxy,'8080').rsplit(':',1)[1])
        deadline=time.monotonic()+15
        while True:
            try:
                conn=http.client.HTTPConnection('127.0.0.1',port,timeout=2);conn.request('GET','/');assert conn.getresponse().read()==b'old';conn.close();break
            except (OSError,AssertionError):
                if time.monotonic()>deadline:raise
                time.sleep(.1)
        sse=http.client.HTTPConnection('127.0.0.1',port,timeout=3);sse.request('GET','/sse');response=sse.getresponse();assert response.readline()==b'data: old\n'
        ws=socket.create_connection(('127.0.0.1',port),timeout=3)
        ws.sendall(b'GET /ws HTTP/1.1\r\nHost: local\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: '+base64.b64encode(os.urandom(16))+b'\r\n\r\n')
        stream=ws.makefile('rb');assert b'101' in stream.readline()
        while stream.readline()!=b'\r\n':pass
        def frame():
            header=stream.read(2);assert len(header)==2 and header[0]==0x81, 'upgraded connection closed';return stream.read(header[1]&127)
        assert frame()==b'old'
        (directory/'Caddyfile').write_text(config(9002));run('exec',proxy,'caddy','reload','--config','/fixture/Caddyfile','--adapter','caddyfile')
        conn=http.client.HTTPConnection('127.0.0.1',port,timeout=3);conn.request('GET','/');assert conn.getresponse().read()==b'new';conn.close()
        for slot,expected in [('blue',b'old'),('green',b'new')]:
            conn=http.client.HTTPConnection('127.0.0.1',port,timeout=3);conn.request('GET','/api/bps-images/'+slot+'/test-token');assert conn.getresponse().read()==expected;conn.close()
        until=time.monotonic()+3
        while time.monotonic()<until:
            assert response.readline() in (b'\n',b'data: old\n');assert frame()==b'old'
        sse.close();stream.close();ws.close()
        print('PASS: real SSE and WebSocket stayed on old backend after reload; new HTTP reached new backend; image callbacks retained slot affinity')
    except Exception:
        for name in [proxy,backend]:
            subprocess.run(["docker","logs",name])
        raise
    finally:
        for name in [proxy,backend]:
            subprocess.run(['docker','rm','-f',name],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
        subprocess.run(['docker','network','rm',network],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
