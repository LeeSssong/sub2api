import pathlib,subprocess,uuid,time,json,hashlib
root=pathlib.Path(__file__).resolve().parents[3]; base='upstream/sub2api/backend/migrations/'
name='codex-uiux-upgrade-'+uuid.uuid4().hex[:8];checks=[]
def run(args,inp=None):return subprocess.run(args,input=inp,text=True,capture_output=True,check=True).stdout

def sql(s):return run(['docker','exec','-i',name,'psql','-U','postgres','-qAt','-v','ON_ERROR_STOP=1'],s)
try:
 run(['docker','run','-d','--name',name,'--network','none','-e','POSTGRES_HOST_AUTH_METHOD=trust','postgres:18-alpine'])
 for i in range(100):
  if subprocess.run(['docker','exec',name,'pg_isready','-h','127.0.0.1','-U','postgres'],capture_output=True).returncode==0:break
  time.sleep(.1)
 sql('CREATE TABLE schema_migrations(filename text PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now());')
 testfiles=run(['git','ls-tree','-r','--name-only','MERGE_HEAD',base]).splitlines();testfiles=[f for f in testfiles if f.endswith('.sql')]
 for f in sorted(testfiles):
  content=run(['git','show','MERGE_HEAD:'+f])
  sql(content if f.endswith('_notx.sql') else 'BEGIN;\n'+content+'\nCOMMIT;')
  sql("INSERT INTO schema_migrations(filename,checksum) VALUES('"+pathlib.Path(f).name+"','"+hashlib.sha256(content.strip().encode()).hexdigest()+"');")
 checks.append({'name':'all test branch migrations from empty schema','count':len(testfiles),'passed':True})
 missing=[p for p in (root/base).glob('*.sql') if base+p.name not in testfiles]
 for p in sorted(missing):
  content=p.read_text();sql(content if p.name.endswith('_notx.sql') else 'BEGIN;\n'+content+'\nCOMMIT;')
 checks.append({'name':'all candidate missing migrations upgrade test branch schema','count':len(missing),'passed':True})
 print(json.dumps({'passed':True,'checks':checks,'legacy_logs_absent':sql("select to_regclass('openai_scheduler_logs') is null;").strip(),'operational_absent':sql("select count(*)=0 from information_schema.columns where table_name='account_monitor_v4_snapshots' and column_name='current_operational';").strip()}))
except subprocess.CalledProcessError as e:
 print(json.dumps({'passed':False,'checks':checks,'file':str(locals().get('p',locals().get('f'))),'error':e.stderr}));raise
finally:run(['docker','rm','-f','-v',name])
