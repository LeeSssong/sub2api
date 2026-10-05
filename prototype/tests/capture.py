import concurrent.futures, os, pathlib, subprocess, tempfile, time, shutil, sys
ROOT=pathlib.Path(__file__).resolve().parents[1]
CHROME='/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'
def capture(page):
    profile=tempfile.mkdtemp(prefix='starbridge-qa-')
    target=ROOT/'qa'/f'{page}-1730.png'
    if target.exists(): target.unlink()
    proc=subprocess.Popen([CHROME,'--headless','--disable-gpu','--no-first-run','--no-default-browser-check','--disable-background-networking','--disable-component-update','--user-data-dir='+profile,'--screenshot='+str(target),'--window-size=1730,1000','--hide-scrollbars','http://127.0.0.1:4173/#'+page],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
    try:
        limit=time.monotonic()+15
        while time.monotonic()<limit:
            if target.exists() and target.stat().st_size>1000: return page, str(target)
            time.sleep(.25)
        return page, '截图未生成'
    finally:
        proc.terminate()
        try: proc.wait(timeout=3)
        except subprocess.TimeoutExpired: proc.kill()
        shutil.rmtree(profile,ignore_errors=True)
if __name__ == '__main__':
    with concurrent.futures.ThreadPoolExecutor(max_workers=2) as ex:
        for result in ex.map(capture,sys.argv[1:] or ['ai','keys','usage','recharge','orders','profile']): print(result,flush=True)
