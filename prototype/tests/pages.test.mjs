import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import * as M from '../model.mjs';
import {page,filteredKeys} from '../pages.mjs';

test('六个业务页面只显示页面主标题，不显示标题说明',()=>{
 const state=M.createSeed();
 const cases=[
  ['ai','请选择你的 AI 工具','比较可用线路、稳定性和价格倍率'],
  ['usage','使用记录','查看消费、Token、线路分布与请求明细'],
  ['keys','我的密钥','创建、绑定和管理用于接入 AI 线路的 API 密钥'],
  ['recharge','充值与兑换','为账户充值或兑换余额、并发及订阅权益'],
  ['orders','我的订单','查看充值与订阅订单，并处理待支付或可退款订单'],
  ['profile','个人资料','管理账户资料、登录安全与余额提醒'],
 ];
 for(const [route,title,description] of cases){
  const html=page({state,page:route,checks:{},checking:false,keyFilter:{search:'',route:'',status:''},keySort:['name',1],keyPage:1,keyPageSize:20,keyRouteOpen:'',keyRouteSearch:'',keyRouteAnchor:null,usageFilter:{...M.usageDateRange(),key:'',model:'',route:'',type:'',billingType:'',mode:''},usageSort:['time',-1],usagePage:1,usagePageSize:20,usageTab:'usage',usageErrorFilter:{},granularity:'hour',chartMetric:{},requestErrors:{},updatedAt:'12:00:00',rechargeTab:'recharge',amount:30,customAmount:false,orderFilter:''});
  assert.equal((html.match(/<h1\b/g)||[]).length,1,route);
  assert.match(html,new RegExp(`<header class="page-head"><h1>${title}<\\/h1><\\/header>`),route);
  assert.doesNotMatch(html,new RegExp(description),route);
 }
});

test('页面文字按职责共享字阶，默认配置不使用成功色',()=>{
 const css=readFileSync(new URL('../styles.css',import.meta.url),'utf8');
 assert.match(css,/--type-page-title:20px/);
 assert.match(css,/--type-section-title:17px/);
 assert.match(css,/--type-panel-title:14px/);
 assert.match(css,/--type-card-title:15px/);
 assert.match(css,/--type-compact-copy:12px/);
 assert.match(css,/--type-compact:11px/);
 assert.match(css,/--type-status:12px/);
 assert.match(css,/\.profile-row>span\.success,\.profile-row>span\.muted\s*\{[^}]*font-size:var\(--type-status\)/);
 assert.match(css,/\.endpoint \.badge\.success\s*\{[^}]*color:var\(--secondary\)/);
});

test('AI 工具卡片展示关联密钥数并始终使用关联密钥入口',()=>{
 const html=page({state:M.createSeed(),page:'ai',checks:{},checking:false});
 assert.match(html,/已关联 2 把密钥/);
 assert.match(html,/已关联 1 把密钥/);
 assert.equal((html.match(/>关联密钥<\/button>/g)||[]).length,4);
 assert.doesNotMatch(html,/已配置 \d+ 条|已配满|新增线路|去配置/);
});

test('线路列表和换线选项按成功率阈值标色，不改动可用状态',()=>{
 const state=M.createSeed();
 const ai=page({state,page:'ai',checks:{},checking:false});
 assert.match(ai,/class="rate rate-high"[^>]*>97%<\/button>/);
 assert.match(ai,/class="rate rate-medium"[^>]*>75%<\/button>/);
 assert.match(ai,/class="rate rate-medium"[^>]*>68%<\/button>/);
 const keys=page({state,page:'keys',keyFilter:{search:'',route:'',status:''},keySort:['name',1],keyPage:1,keyPageSize:20,keyRouteOpen:'key-prod',keyRouteSearch:'',keyRouteAnchor:null});
 assert.match(keys,/近 1 小时稳定性 <span class="rate-high">97%<\/span>/);
 assert.match(keys,/近 1 小时稳定性 <span class="rate-medium">68%<\/span>/);
 assert.match(keys,/class="key-route-option-state"><span class="dot danger"><\/span>不可用/);
});

test('我的密钥列表线路列展示统一线路身份，并提供下拉更换入口',()=>{
 const state=M.createSeed(),keyRouteOpen=state.keys[0].id;
 const html=page({state,page:'keys',keyFilter:{search:'',route:'',status:''},keySort:['name',1],keyPage:1,keyPageSize:20,keyRouteOpen,keyRouteSearch:'',keyRouteAnchor:null,columns:{keys:[]}});
 assert.match(html,/可点击线路名称或编辑按钮更换绑定线路/);
 assert.match(html,/<button[^>]+key-route-control[^>]+data-action="key-route-dropdown"/);
 assert.match(html,/route-provider-logo/);
 assert.match(html,/route-choice-multiplier data">1\.0x倍率<\/span>/);
 assert.match(html,/key-route-chevron/);
 assert.match(html,/近 1 小时稳定性 <span class="rate-high">97%<\/span> · 首字 2\.16s/);
 assert.match(html,/当前线路/);
 assert.match(html,/显示 1 至 5 共 5 条结果/);
 assert.match(html,/每页数量/);
 assert.match(html,/page-current[^>]*>1</);
});

test('我的密钥分页只渲染当前页，并显示正确的结果范围',()=>{
 const state=M.createSeed();
 state.keys=Array.from({length:23},(_,i)=>({...state.keys[0],id:`key-${i+1}`,name:`密钥 ${String(i+1).padStart(2,'0')}`,token:`DEMO-ONLY-${i+1}`}));
 const html=page({state,page:'keys',keyFilter:{search:'',route:'',status:''},keySort:['name',1],keyPage:2,keyPageSize:20,keyRouteOpen:'',columns:{keys:[]}});
 assert.match(html,/显示 21 至 23 共 23 条结果/);
 assert.match(html,/page-current[^>]*>2</);
 const tbody=html.match(/<tbody>([\s\S]*?)<\/tbody>/)?.[1]||'';
 assert.equal((tbody.match(/<tr>/g)||[]).length,3);
 assert.match(html,/data-action="key-page"[^>]+data-delta="-1"/);
 assert.match(html,/data-action="key-page"[^>]+data-delta="1"[^>]+disabled/);
});

test('密钥可选列覆盖测试站的管理信息，默认不挤占主表格',()=>{
 const state=M.createSeed(),base={state,page:'keys',keyFilter:{search:'',route:'',status:''},keySort:['name',1],keyPage:1,keyPageSize:20,keyRouteOpen:'',keyRouteSearch:'',keyRouteAnchor:null};
 const defaultView=page({...base});
 assert.doesNotMatch(defaultView,/<th[^>]*>ID<\/th>/);
 assert.doesNotMatch(defaultView,/<th[^>]*>速率限制<\/th>/);
 state.keys[0].limit=10;state.keys[0].rate5h=10;state.keys[0].usage5h=1.2;
 const expanded=page({...base,state:{...state,columns:{keys:[]}}});
 for(const heading of ['ID','额度限制','速率限制','状态','最近使用','最近使用 IP','创建时间'])assert.match(expanded,new RegExp(`<th[^>]*>(?:<button[^>]*>)?${heading}`));
 assert.match(expanded,/data-action="reset-key-quota"/);
 assert.match(expanded,/data-action="reset-key-rate"/);
 assert.match(expanded,/5h \$1\.20 \/ \$10\.00/);
});

test('最近使用排序按真实请求时间，不按密钥对象的空字段排序',()=>{
 const state=M.createSeed();state.keys.reverse();
 const rows=filteredKeys({state,keyFilter:{search:'',route:'',status:''},keySort:['lastUsed',-1]});
 assert.equal(rows[0].id,state.usage[0].keyId);
 assert.deepEqual(rows.slice(-2).map(key=>key.id).sort(),['key-backup','key-old']);
});

test('Passkey 默认沿用原生关闭态，管理员启用后才开放操作',()=>{
 const state=M.createSeed();
 const disabled=page({state,page:'profile'});
 assert.match(disabled,/Passkey<\/strong><span class="muted">管理员未启用<\/span>/);
 assert.match(disabled,/>未启用<\/button>/);
 assert.match(disabled,/data-action="passkey"[^>]+disabled/);
 assert.doesNotMatch(disabled,/>添加<\/button>/);

 state.passkeyEnabled=true;
 const enabled=page({state,page:'profile'});
 assert.match(enabled,/Passkey<\/strong><span class="muted">未添加<\/span>/);
 assert.match(enabled,/data-action="passkey"[^>]*>添加<\/button>/);
 assert.doesNotMatch(enabled,/data-action="passkey"[^>]+disabled/);
});

test('使用统计读取失败显示明确错误，不把失败伪装为零数据',()=>{
 const state=M.createSeed();
 const html=page({
  state,page:'usage',usageFilter:{...M.usageDateRange(),key:'',model:'',route:'',type:'',billingType:'',mode:''},
  usageSort:['time',-1],usagePage:1,usagePageSize:20,usageTab:'usage',usageErrorFilter:{},
  granularity:'hour',chartMetric:{},requestErrors:{usageStats:{message:'用量统计加载失败，请稍后重试。'}},updatedAt:'12:00:00'
 });
 assert.match(html,/role="alert"/);
 assert.match(html,/用量统计加载失败，请稍后重试。/);
 assert.match(html,/data-action="retry-usage-stats"/);
 assert.doesNotMatch(html,/data-stat="总请求数">0</);
 assert.match(html,/请求明细/);
});

test('使用记录合并为概览、分析和请求明细三个工作区',()=>{
 const state=M.createSeed();
 const base={state,page:'usage',usageFilter:{...M.usageDateRange(),key:'',model:'',route:'',type:'',billingType:'',mode:''},usageSort:['time',-1],usagePage:1,usagePageSize:20,usageTab:'usage',usageErrorFilter:{},granularity:'hour',chartMetric:{},updatedAt:'12:00:00'};
 const html=page({...base,requestErrors:{}});
 assert.match(html,/<section class="usage-overview"[^>]*>[\s\S]*?class="time-toolbar"[\s\S]*?class="stats"[\s\S]*?<\/section>\s*<section class="usage-analysis"/);
 assert.match(html,/<section class="usage-analysis"[^>]*>[\s\S]*?class="charts"[\s\S]*?模型分布[\s\S]*?Token 使用趋势[\s\S]*?<\/section>\s*<section class="usage-records"/);
 assert.match(html,/<section class="usage-records"[^>]*>[\s\S]*?class="filters usage-filters"[\s\S]*?data-action="export-csv"[\s\S]*?请求明细[\s\S]*?class="table-footer"/);
 assert.equal((html.match(/class="usage-(?:overview|analysis|records)"/g)||[]).length,3);
 const statsFailure=page({...base,requestErrors:{usageStats:{message:'统计失败'}}});
 assert.match(statsFailure,/class="usage-overview"[\s\S]*?统计失败/);
 assert.doesNotMatch(statsFailure,/class="usage-analysis"/);
 assert.match(statsFailure,/class="usage-records"[\s\S]*?请求明细/);
 const listFailure=page({...base,requestErrors:{usageList:{message:'明细失败'}}});
 assert.match(listFailure,/class="usage-analysis"/);
 assert.match(listFailure,/class="usage-records"[\s\S]*?明细失败/);
 state.allowUserViewErrorRequests=true;
 const errors=page({...base,usageTab:'errors',requestErrors:{usageErrors:{message:'错误请求读取失败'}}});
 assert.match(errors,/class="usage-records"[\s\S]*?role="tablist"[\s\S]*?错误请求[\s\S]*?data-change="usage-error-filter"[\s\S]*?错误请求读取失败/);
 assert.doesNotMatch(errors,/data-action="export-csv"/);
});

test('使用记录概览控件靠右成组，不展示最近更新时间',()=>{
 const html=page({state:M.createSeed(),page:'usage',usageFilter:{...M.usageDateRange(),key:'',model:'',route:'',type:'',billingType:'',mode:''},usageSort:['time',-1],usagePage:1,usagePageSize:20,usageTab:'usage',usageErrorFilter:{},granularity:'hour',chartMetric:{},requestErrors:{},updatedAt:'12:00:00'});
 const toolbar=html.match(/<div class="time-toolbar">([\s\S]*?)<\/div><div class="stats">/)?.[1];
 assert.ok(toolbar);
 assert.match(toolbar,/class="usage-time-control"[^>]*><span>时间范围<\/span>/);
 assert.match(toolbar,/class="usage-time-control"[^>]*><span>统计粒度<\/span><div class="brand-dropdown brand-dropdown-simple"/);
 assert.doesNotMatch(toolbar,/最近更新|12:00:00|class="push"/);
});

test('概览时间控件与图表切换器使用同级紧凑尺寸',()=>{
 const css=readFileSync(new URL('../styles.css',import.meta.url),'utf8');
 assert.match(css,/\.usage-overview \.usage-date-picker>button,\s*\.usage-overview \.time-toolbar select\s*\{[^}]*min-height:28px;[^}]*height:28px;[^}]*font-size:11px;/);
});

test('使用记录筛选高于操作层级，导出与其他操作同级',()=>{
 const html=page({state:M.createSeed(),page:'usage',usageFilter:{...M.usageDateRange()},usageSort:['time',-1],usagePage:1,granularity:'hour',chartMetric:{}});
 assert.match(html,/class="toolbar"[^>]*>[\s\S]*?data-action="export-csv"/);
 assert.doesNotMatch(html,/class="primary" data-action="export-csv"/);
 const css=readFileSync(new URL('../styles.css',import.meta.url),'utf8');
 assert.match(css,/\.usage-records \.usage-filters select\s*\{[^}]*min-height:40px;[^}]*font-size:12px;[^}]*font-weight:400;/);
 assert.match(css,/\.usage-records \.usage-filters\+\.toolbar button\s*\{[^}]*min-height:28px;[^}]*height:28px;[^}]*font-size:11px;[^}]*font-weight:500;/);
});

test('使用明细读取失败替换表格空态，并保留统计工作面',()=>{
 const state=M.createSeed();
 const html=page({
  state,page:'usage',usageFilter:{...M.usageDateRange(),key:'',model:'',route:'',type:'',billingType:'',mode:''},
  usageSort:['time',-1],usagePage:1,usagePageSize:20,usageTab:'usage',usageErrorFilter:{},
  granularity:'hour',chartMetric:{},requestErrors:{usageList:{message:'使用记录加载失败，请检查网络后重试。'}},updatedAt:'12:00:00'
 });
 assert.match(html,/data-stat="总请求数"/);
 assert.match(html,/使用记录加载失败，请检查网络后重试。/);
 assert.match(html,/data-action="retry-usage-list"/);
 assert.doesNotMatch(html,/暂无数据/);
});

test('充值创建订单后进入原生支付承接工作面，不向用户暴露模拟结算按钮',()=>{
 const state=M.createSeed();
 const order=M.createOrder(state,30);
 const html=page({
  state,page:'recharge',rechargeTab:'recharge',amount:30,customAmount:false,
  paymentFlow:{orderId:order.id,orderNumber:order.number,amount:order.amount,payAmount:order.amount,paymentType:'alipay',expiresAt:Date.now()+30*60*1000,status:'pending'}
 });
 assert.match(html,/等待支付/);
 assert.match(html,new RegExp(order.number));
 assert.match(html,/data-action="open-native-payment"/);
 assert.match(html,/data-action="cancel-payment-order"/);
 assert.match(html,/data-action="go-orders"/);
 assert.doesNotMatch(html,/模拟支付成功|模拟取消支付/);
 assert.doesNotMatch(html,/选择充值额度/);
});

test('我的订单沿用原生状态筛选并为待支付订单提供取消操作',()=>{
 const state=M.createSeed();
 M.createOrder(state,30);
 const html=page({state,page:'orders',orderFilter:''});
 assert.match(html,/value="pending"/);
 assert.match(html,/value="completed"/);
 assert.match(html,/value="failed"/);
 assert.match(html,/value="refunded"/);
 assert.match(html,/data-action="cancel-order"/);
 assert.match(html,/待支付/);
});
