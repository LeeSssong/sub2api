import test from 'node:test';
import assert from 'node:assert/strict';
import * as M from '../model.mjs';
import {page} from '../pages.mjs';
test('日期区间包含结束日并排除次日，组合筛选使用原生枚举',()=>{
 const s=M.createSeed(),base=s.usage[0];
 s.usage=[{...base,id:'before',time:new Date(2026,8,18,23,59).getTime()},{...base,id:'start',time:new Date(2026,8,19).getTime(),type:'stream',billingType:'0',mode:'token'},{...base,id:'end',time:new Date(2026,8,19,23,59).getTime(),type:'sync',billingType:'1',mode:'per_request'},{...base,id:'after',time:new Date(2026,8,20).getTime()}];
 const f={startDate:'2026-09-19',endDate:'2026-09-19'};
 assert.deepEqual(M.filterUsage(s,f).map(r=>r.id),['start','end']);
 assert.deepEqual(M.filterUsage(s,{...f,type:'sync',billingType:'1',mode:'per_request'}).map(r=>r.id),['end']);
});
test('日期预设跟随原生日期语义',()=>{
 assert.equal(typeof M.usageDateRange,'function');
 const now=new Date(2026,8,19,15).getTime();
 assert.deepEqual(M.usageDateRange('last24Hours',now),{startDate:'2026-09-18',endDate:'2026-09-19'});
 assert.deepEqual(M.usageDateRange('7days',now),{startDate:'2026-09-13',endDate:'2026-09-19'});
});
test('普通用量只展示四项筛选，错误开关控制独立页签',()=>{
 const state=M.createSeed(),c={state,page:'usage',usageFilter:{},usageDraft:{},usageSort:['time',-1],usagePage:1,granularity:'hour',chartMetric:{}};
 let html=page(c);
 assert.deepEqual([...html.matchAll(/data-change="usage-filter" data-field="([^"]+)"/g)].map(m=>m[1]),['key','model','route','mode']);
 assert.doesNotMatch(html,/全部类型|全部计费类型|请求类别|赠送额度|筛选已修改/);
 assert.doesNotMatch(html,/data-action="usage-tab"/);
 state.allowUserViewErrorRequests=true;c.usageTab='errors';html=page(c);
 assert.match(html,/错误请求/);
 assert.deepEqual([...html.matchAll(/data-change="usage-error-filter" data-field="([^"]+)"/g)].map(m=>m[1]),['key','category','model']);
});
