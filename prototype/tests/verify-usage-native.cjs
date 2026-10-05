const {chromium}=require('/Users/awen/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright');
const path=require('node:path');
const assert=require('node:assert/strict');
(async()=>{
 const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
 const context=await browser.newContext({viewport:{width:1730,height:1000},deviceScaleFactor:1,locale:'zh-CN',timezoneId:'Asia/Shanghai',reducedMotion:'reduce'});
 const page=await context.newPage(),errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.clock.install({time:new Date('2026-09-19T12:00:00+08:00')});await page.clock.pauseAt(new Date('2026-09-19T12:00:00+08:00'));
 const base='http://127.0.0.1:4173/';
 const out=path.resolve('docs/refactorUIUXv0.1/evidence/prototype-baseline');
 async function fresh(){await page.goto(base+'#usage');await page.evaluate(()=>localStorage.clear());await page.reload();await page.locator('[data-stat="总请求数"]').waitFor();}
 async function shot(name){await page.evaluate(async()=>{await document.fonts.ready;await Promise.all([...document.images].map(i=>i.decode().catch(()=>{})));});await page.screenshot({path:path.join(out,name),fullPage:false});}
 await fresh();await shot('1730x1000/usage.png');await shot('interactive/usage-default-state.png');
 const initial=await page.locator('[data-stat="总请求数"]').innerText();
 await page.getByLabel('类型',{exact:true}).selectOption('sync');assert.notEqual(await page.locator('[data-stat="总请求数"]').innerText(),initial);
 assert.equal(await page.getByText('筛选已修改，点击刷新应用').count(),0);await shot('interactive/usage-native-filtered.png');
 await page.getByRole('button',{name:'重置',exact:true}).click();assert.equal(await page.locator('[data-stat="总请求数"]').innerText(),initial);
 await page.getByRole('button',{name:'时间范围',exact:true}).click();await page.getByRole('button',{name:'近7天',exact:true}).click();assert.equal(await page.locator('[data-stat="总请求数"]').innerText(),initial);await shot('interactive/usage-date-range.png');
 await page.getByRole('button',{name:'应用',exact:true}).click();assert.equal(await page.locator('[data-stat="总请求数"]').innerText(),'60');
 const countBeforePage=await page.locator('[data-stat="总请求数"]').innerText();await page.getByRole('button',{name:'下一页',exact:true}).click();assert.equal(await page.locator('[data-stat="总请求数"]').innerText(),countBeforePage);
 await page.getByLabel('每页数量',{exact:true}).selectOption('10');assert.match(await page.locator('.table-footer').innerText(),/第 1 \/ 6 页/);
 await fresh();await page.locator('[data-action="request-detail"]').first().click();await shot('interactive/usage-request-detail.png');await page.getByRole('button',{name:'关闭弹窗',exact:true}).click();
 await page.getByRole('button',{name:'列设置',exact:true}).click();await shot('interactive/usage-column-settings.png');await page.getByRole('button',{name:'关闭弹窗',exact:true}).click();
 await page.evaluate(async()=>{const M=await import('./model.mjs');const s=M.createSeed();s.allowUserViewErrorRequests=true;s.errorRequests=[{id:'error-demo-1',keyId:'key-prod',keyName:'生产主密钥',model:'GPT-4.1',endpoint:'/v1/responses',status:429,category:'rate_limit',message:'请求频率超过限制（示例）',time:Date.now()}];localStorage.setItem(M.STORAGE_KEY,JSON.stringify(s));});
 await page.reload();await page.getByRole('tab',{name:'错误请求',exact:true}).click();assert.equal(await page.getByRole('button',{name:'导出 CSV',exact:true}).count(),0);assert.match(await page.locator('.usage-table').innerText(),/请求频率超过限制/);await shot('interactive/usage-errors-enabled.png');
 await page.getByLabel('分类',{exact:true}).selectOption('auth');assert.match(await page.locator('.panel').last().innerText(),/暂无错误请求/);await shot('interactive/usage-errors-empty.png');
 await fresh();await page.setViewportSize({width:390,height:844});await shot('interactive/usage-native-390.png');
 assert.equal(errors.length,0,errors.join('\n'));
 console.log(JSON.stringify({browser:browser.version(),assertions:'即时筛选、重置、日期应用、全量统计不随分页改变、页数、详情、列设置、错误开关与筛选、无运行错误',screenshots:9}));
 await browser.close();
})().catch(e=>{console.error(e);process.exitCode=1;});
