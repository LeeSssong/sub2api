export const STORAGE_KEY='starbridge-interactive-v1';
export const tools=['Codex','Claude Code','Grok','DeepSeek'];
export const endpoint='https://api.ai-link.example/v1';
export const uid=()=>globalThis.crypto?.randomUUID?.()||`demo-${Date.now()}-${Math.random().toString(36).slice(2)}`;
const round=n=>Math.round(n*100)/100;
export function createSeed(now=Date.now()){
 const routes=[
 ['gpt-pro','GPT-Pro','Codex','OpenAI',1,97,1248,2.16,5.84,1.92,true,true,'success'],['gpt-economy','GPT-经济','Codex','OpenAI',.8,92,864,2.84,6.72,2.43,true,true,'success'],['gpt-fast','GPT-极速','Codex','OpenAI',1.2,68,319,3.42,7.81,null,true,false,'timeout'],
 ['claude-standard','Claude-常规','Claude Code','Anthropic',1,75,920,2.16,5.26,null,true,true,'failed'],['claude-economy','Claude-经济','Claude Code','Anthropic',.8,70,620,3.24,7.18,null,true,false,'failed'],['claude-fast','Claude-极速','Claude Code','Anthropic',1.2,72,432,2.05,5.02,null,true,false,'timeout'],
 ['grok-discount','Grok-特惠','Grok','xAI',.8,68,360,3.60,8.12,null,true,false,'timeout'],['grok-standard','Grok-常规','Grok','xAI',1,65,120,3.10,7.46,null,true,false,'failed'],['deepseek','DeepSeek','DeepSeek','DeepSeek',1,null,0,null,null,null,false,false,'disabled']
 ].map(([id,name,tool,provider,multiplier,success,total,p50,durationP50,checkLatency,enabled,available,result])=>({id,name,tool,provider,multiplier,success,total,p50,durationP50,checkLatency,enabled,available,result}));
 const keys=[['key-prod','生产主密钥','gpt-pro',true],['key-backup','备用密钥','gpt-pro',true],['key-dev','Claude 测试','claude-standard',true],['key-grok','Grok 接入','grok-discount',true],['key-old','旧版接入','deepseek',false]].map(([id,name,routeId,enabled],i)=>({id,name,routeId,enabled,token:`DEMO-ONLY-${id}-NOT-VALID-8F${i}A`,limit:0,ip:'',rate:0,expires:'',concurrency:i===0?2:0,created:now-i*86400000}));
 keys[0].limit=10;keys[0].rate5h=10;keys[0].rate1d=20;keys[0].rate7d=40;keys[0].usage5h=1.2;keys[0].usage1d=2.5;keys[0].usage7d=8.4;
 const usage=Array.from({length:60},(_,i)=>{const route=routes[[0,3,6,0,1][i%5]];const keyId={'gpt-pro':'key-prod','claude-standard':'key-dev','grok-discount':'key-grok','gpt-economy':'archived-key'}[route.id];return {id:'req-'+(10000+i),keyId,keyName:keys.find(k=>k.id===keyId)?.name||'历史密钥',routeId:route.id,routeName:route.name,model:route.tool==='Codex'?'GPT-4.1':route.tool==='Claude Code'?'Claude 3.7':'Grok',reasoning:i%2?'中':'高',endpoint:route.tool==='Claude Code'?'/v1/messages':'/v1/chat/completions',ip:i%2?'172.16.*.*':'104.28.*.*',type:['stream','sync','ws_v2','live'][i%4],billingType:i%8===0?'1':'0',mode:['token','per_request','image','video'][i%4],input:4200+(i%7)*1320,output:1200+(i%5)*810,cache:i%3?300:0,cost:round(.12+(i%8)*.07),standard:round(.18+(i%8)*.09),latency:round(1.5+(i%7)*.21),time:now-i*3600000,region:''};});
 return {version:1,usageSchema:2,allowUserViewErrorRequests:false,errorRequests:[],seededAt:now,balance:30,concurrency:5,passkeyEnabled:false,profile:{name:'1098808377',email:'awen@example.com',avatar:'',passwordSet:true,twoFactor:false,passkeys:[],threshold:10},routes,keys,usage,orders:[{id:'1842',number:'AL20260914001842',amount:30,credit:30,bonus:0,status:'paid',method:'支付宝',created:now-2*86400000},{id:'1768',number:'AL20260910001768',amount:100,credit:110,bonus:10,status:'paid',method:'支付宝',created:now-6*86400000},{id:'1651',number:'AL20260902001651',amount:50,credit:0,bonus:0,status:'closed',method:'支付宝',created:now-14*86400000}],redemptions:[{id:'r-old',code:'STAR-30-••••-9A2C',reward:'$30.00 账户额度',created:now-6*86400000,status:'success'}],usedCodes:[],contactInfo:'',columns:{keys:[7,8,9,10,11,12,13],usage:[]},imports:[]};
}
export function configured(s,tool){return new Set(s.keys.filter(k=>s.routes.find(r=>r.id===k.routeId)?.tool===tool).map(k=>k.routeId)).size;}
export function linkedKeyCount(s,tool){return s.keys.filter(k=>s.routes.find(r=>r.id===k.routeId)?.tool===tool).length;}
export function configuredRoutes(s){return s.routes.filter(r=>s.keys.some(k=>k.routeId===r.id));}
export function toolButton(s,tool){return {label:'关联密钥',disabled:!s.routes.some(r=>r.tool===tool&&r.enabled)};}
function keyFields(s,f,existing){const route=s.routes.find(r=>r.id===f.routeId);if(!route||(!route.enabled&&existing?.routeId!==route.id))throw Error('请选择管理正常的线路');const name=String(f.name||'').trim();if(!name||name.length>50)throw Error('名称需为 1–50 个字符');const limit=Number(f.limit||0),rate5h=Number(f.rate5h||0),rate1d=Number(f.rate1d||0),rate7d=Number(f.rate7d||0);if(!Number.isFinite(limit)||limit<0)throw Error('额度限制必须是大于或等于 0 的数字');if([rate5h,rate1d,rate7d].some(value=>!Number.isFinite(value)||value<0))throw Error('速率额度必须是大于或等于 0 的数字');if(f.expires&&(!Number.isFinite(Date.parse(f.expires))||Date.parse(f.expires)<Date.now()))throw Error('有效期必须晚于当前时间');return {name,routeId:route.id,limit,ipWhitelist:String(f.ipWhitelist||'').trim(),ipBlacklist:String(f.ipBlacklist||'').trim(),rate5h,rate1d,rate7d,expires:f.expires||''};}
export function createKey(s,f){const fields=keyFields(s,f);let token=f.custom?String(f.custom).trim():`DEMO-ONLY-${uid()}-NOT-VALID`;if(f.custom&&(!/^[a-zA-Z0-9_-]{8,100}$/.test(token)))throw Error('自定义密钥使用 8–100 位字母、数字、下划线或横线');if(s.keys.some(k=>k.token===token))throw Error('此密钥已存在');const k={...fields,id:uid(),token,enabled:true,concurrency:0,created:Date.now()};s.keys.unshift(k);return k;}
export function editKey(s,id,f){const k=s.keys.find(k=>k.id===id);if(!k)throw Error('密钥不存在');const fields=keyFields(s,{...k,...f},k);Object.assign(k,fields);return k;}
export function deleteKey(s,id){s.keys=s.keys.filter(k=>k.id!==id);}
export function keyStatus(k){return !k.enabled?'disabled':k.expires&&Date.parse(k.expires)<Date.now()?'expired':'enabled';}
export function createOrder(s,amount){amount=Number(amount);if(!Number.isFinite(amount)||amount<1||amount>10000||round(amount)!==amount)throw Error('请输入 1–10,000 之间、最多两位小数的充值额度');const o={id:uid(),number:'DEMO'+Date.now(),amount,credit:amount,bonus:0,status:'pending',method:'支付宝',created:Date.now()};s.orders.unshift(o);return o;}
export function settleOrder(s,id,status){const o=s.orders.find(o=>o.id===id);if(!o)throw Error('订单不存在');if(o.status===status)return o;if(o.status!=='pending')throw Error('订单已经结束，不能重复处理');if(!['paid','closed'].includes(status))throw Error('无效的支付结果');o.status=status;if(status==='paid')s.balance=round(s.balance+o.credit);return o;}
export function redeem(s,raw){const code=String(raw).trim().toUpperCase();if(!['DEMO30','PLUS2'].includes(code))throw Error('兑换码无效，请检查后重试');if(s.usedCodes.includes(code))throw Error('此兑换码已使用');s.usedCodes.push(code);const reward=code==='DEMO30'?'$30.00 账户额度':'并发上限 +2（演示）';if(code==='DEMO30')s.balance=round(s.balance+30);else s.concurrency+=2;const record={id:uid(),code,reward,status:'success',created:Date.now()};s.redemptions.unshift(record);return record;}
export const usageTypeLabels={ws_v2:'WS',live:'Live',stream:'流式',sync:'同步'};
export const usageModeLabels={token:'按量',per_request:'按次',image:'按次(图片)',video:'按次(视频)'};
export const usageErrorLabels={auth:'认证失败',rate_limit:'限流',quota:'余额/订阅',invalid_request:'参数错误',service_unavailable:'服务暂时不可用',upstream:'上游错误',internal:'平台错误',cyber:'安全策略'};
export const usageDatePresets=[['today','今天'],['yesterday','昨天'],['last24Hours','近24小时'],['7days','近7天'],['14days','近14天'],['30days','近30天'],['thisMonth','本月'],['lastMonth','上月']];
const localDate=d=>`${d.getFullYear()}-${String(d.getMonth()+1).padStart(2,'0')}-${String(d.getDate()).padStart(2,'0')}`;
export function usageDateRange(preset='last24Hours',now=Date.now()){
 const end=new Date(now),start=new Date(now);
 if(preset==='yesterday'){start.setDate(start.getDate()-1);end.setDate(end.getDate()-1);}
 else if(preset==='last24Hours')start.setTime(now-86400000);
 else if(/^(7|14|30)days$/.test(preset))start.setDate(start.getDate()-parseInt(preset)+1);
 else if(preset==='thisMonth')start.setDate(1);
 else if(preset==='lastMonth'){start.setDate(1);start.setMonth(start.getMonth()-1);end.setDate(0);}
 return {startDate:localDate(start),endDate:localDate(end)};
}
export function usageDateLabel(f){for(const [preset,label] of usageDatePresets){const range=usageDateRange(preset);if(range.startDate===f.startDate&&range.endDate===f.endDate)return label;}return `${f.startDate} 至 ${f.endDate}`;}
export function filterUsage(s,f={}){
 const start=f.startDate?new Date(f.startDate+'T00:00:00').getTime():Date.now()-Number(f.hours||720)*3600000;
 const end=f.endDate?new Date(f.endDate+'T00:00:00'):null;if(end)end.setDate(end.getDate()+1);
 return s.usage.filter(r=>r.time>=start&&(!end||r.time<end.getTime())&&(!f.key||r.keyId===f.key)&&(!f.model||r.model===f.model)&&(!f.route||r.routeId===f.route)&&(!f.type||r.type===f.type)&&(!f.billingType||String(r.billingType)===f.billingType)&&(!f.mode||r.mode===f.mode));
}
export function filterUsageErrors(s,f={}){return filterUsage({usage:s.errorRequests||[]},{startDate:f.startDate,endDate:f.endDate,key:f.key}).filter(r=>(!f.model||r.model.toLowerCase().includes(f.model.toLowerCase()))&&(!f.category||r.category===f.category));}
export function summarize(rows){const sum=k=>rows.reduce((n,r)=>n+r[k],0);return {requests:rows.length,input:sum('input'),output:sum('output'),cache:sum('cache'),tokens:sum('input')+sum('output')+sum('cache'),cost:round(sum('cost')),standard:round(sum('standard')),latency:rows.length?sum('latency')/rows.length:null};}
export function distribution(rows,field,metric='tokens'){const m={};for(const r of rows)m[r[field]]=(m[r[field]]||0)+(metric==='cost'?r.cost:r.input+r.output+r.cache);return Object.entries(m).sort((a,b)=>b[1]-a[1]);}
export function checkResult(r){if(!r.enabled)return {text:'线路已停用',kind:'muted'};if(r.result==='timeout')return {text:'响应超时',kind:'danger'};if(r.result==='failed')return {text:'检查失败',kind:'danger'};return {text:Number(r.checkLatency??r.p50).toFixed(2)+'s',kind:'success'};}
export function lineStatus(r){if(!r?.enabled)return {text:'停用',kind:'muted'};return r.result==='success'?{text:'可用',kind:'success'}:{text:'不可用',kind:'danger'};}
export function successRateTone(rate){return rate==null||!Number.isFinite(rate)?'unknown':rate>=80?'high':rate>=50?'medium':'low';}
export function sortRoutesByQuality(routes){const rank=r=>lineStatus(r).kind==='success'?0:lineStatus(r).kind==='danger'?1:2;return [...routes].sort((a,b)=>rank(a)-rank(b)||(b.success??-1)-(a.success??-1)||(a.p50??Infinity)-(b.p50??Infinity)||(a.durationP50??Infinity)-(b.durationP50??Infinity)||a.multiplier-b.multiplier||a.name.localeCompare(b.name,'zh'));}
export function bestRoute(routes){return sortRoutesByQuality(routes).find(r=>lineStatus(r).kind==='success')||null;}
export function toCsv(headers,rows){const cell=x=>{let str=String(x??'');if(/^[=+@\-\t\r]/.test(str))str="'"+str;return '"'+str.replaceAll('"','""')+'"';};return '\uFEFF'+[headers,...rows].map(row=>row.map(cell).join(',')).join('\r\n');}
export function applyLineChecks(s){const results={};for(const r of configuredRoutes(s)){results[r.id]=checkResult(r);if(!r.enabled)continue;const successes=r.successCount??Math.round(r.total*r.success/100);r.successCount=successes+(r.result==='success'?1:0);r.total++;r.success=Math.round(r.successCount/r.total*100);r.available=r.result==='success';}return results;}
