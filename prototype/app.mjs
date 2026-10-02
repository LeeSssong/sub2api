import * as M from './model.mjs';
import * as U from './ui.mjs';
import * as P from './pages.mjs';
import * as D from './dialogs.mjs';
import {defaultHiddenKeyColumns} from './key-columns.mjs';
const initialFilter=()=>({...M.usageDateRange(),key:'',model:'',route:'',type:'',billingType:'',mode:''});
let saved;try{saved=JSON.parse(localStorage.getItem(M.STORAGE_KEY));if(saved?.version!==1||!Array.isArray(saved.keys)||!Array.isArray(saved.routes))saved=null;}catch{saved=null;}
if(saved&&saved.usageSchema!==2){saved.usage=M.createSeed(saved.seededAt).usage;saved.usageSchema=2;saved.allowUserViewErrorRequests=false;saved.errorRequests=[];}
if(saved&&saved.keyColumnsVersion!==1){saved.columns||={};saved.columns.keys=[...new Set([...(saved.columns.keys||[]),...defaultHiddenKeyColumns])];}
const recoveredPayment=saved?.paymentRecovery&&saved.orders?.some(o=>o.id===saved.paymentRecovery.orderId&&String(o.status).toLowerCase()==='pending')?saved.paymentRecovery:null;
const c={state:saved||M.createSeed(),page:['ai','keys','usage','recharge','orders','profile'].includes(location.hash.slice(1))?location.hash.slice(1):'ai',keyFilter:{search:'',route:'',status:''},keySort:['name',1],keyPage:1,keyPageSize:20,keyRouteOpen:'',keyRouteSearch:'',keyRouteAnchor:null,usageFilter:initialFilter(),usageDraft:initialFilter(),usageSort:['time',-1],usagePage:1,usagePageSize:20,usageTab:'usage',usageErrorFilter:{},usageDateOpen:false,granularity:'hour',chartMetric:{},detailHours:1,rechargeTab:'recharge',amount:30,customAmount:false,paymentFlow:recoveredPayment,paymentError:'',orderFilter:'',checks:{},checking:false,checkEpoch:0,updatedAt:new Date().toLocaleTimeString('en-GB'),modal:null,requestErrors:{},geoLoading:false,failCreate:false,failPayment:false,failStats:false,failGeo:false,failExport:false};
c.state.keyColumnsVersion=1;
c.save=()=>{try{localStorage.setItem(M.STORAGE_KEY,JSON.stringify(c.state));}catch{U.toast('浏览器未能保存数据，本次更改仅在当前页面保留。',true);}};
c.render=()=>{const focus=document.activeElement;const marker=focus?.dataset?.input;let selection;try{selection=[focus.selectionStart,focus.selectionEnd];}catch{}const pos=window.scrollY;document.querySelector('#app').innerHTML=P.shell(c);P.drawTrend(c);if(marker){const el=document.querySelector(`[data-input="${marker}"]`);el?.focus({preventScroll:true});if(selection&&el?.setSelectionRange){try{el.setSelectionRange(...selection);}catch{}}}window.scrollTo(0,pos);};
function closeKeyRoute(){c.keyRouteOpen='';c.keyRouteSearch='';c.keyRouteAnchor=null;}
function closeDropdowns(except){document.querySelectorAll('.brand-dropdown-simple').forEach(root=>{if(root===except)return;root.querySelector('.brand-dropdown-menu').hidden=true;root.querySelector('.brand-dropdown-trigger').setAttribute('aria-expanded','false');root.classList.remove('brand-dropdown-up');});}
const privateIp=ip=>/^(10\.|127\.|192\.168\.|172\.(1[6-9]|2\d|3[01])\.)/.test(String(ip||''));
async function lookupRegions(rows){
 if(c.geoLoading)return;
 const publicRows=rows.filter(row=>row.ip&&!privateIp(row.ip));
 rows.forEach(row=>{if(row.ip)row.region=privateIp(row.ip)?'内网地址':'查询中…';});
 c.geoLoading=true;c.render();
 await new Promise(resolve=>setTimeout(resolve,450));
 const failed=c.failGeo;c.failGeo=false;
 publicRows.forEach((row,index)=>{row.region=failed?'查询失败':['美国（示例）','新加坡（示例）'][index%2];});
 c.geoLoading=false;c.save();c.render();
 U.toast(failed?'地区查询失败，请稍后重试。':'已更新当前页地区（模拟结果）',failed);
}
function clearUsageErrors(){c.requestErrors={};}
function keyRouteAnchor(button,keyId){const rect=button.getBoundingClientRect(),minLeft=Math.max(12,(document.querySelector('.sidebar')?.getBoundingClientRect().right||0)+12),width=Math.min(360,window.innerWidth-minLeft-12),left=Math.min(Math.max(minLeft,rect.left),window.innerWidth-width-12),key=c.state.keys.find(k=>k.id===keyId),optionCount=c.state.routes.filter(r=>r.enabled||r.id===key?.routeId).length,desiredHeight=64+Math.min(310,Math.max(70,optionCount*59)),below=window.innerHeight-rect.bottom-12,above=rect.top-12,openDown=below>=Math.min(desiredHeight,260)||below>=above,available=Math.max(80,(openDown?below:above)-64),height=Math.min(desiredHeight,(openDown?below:above));return {left,width,top:openDown?rect.bottom+8:Math.max(12,rect.top-height-8),maxHeight:Math.min(310,available)};}
function navigate(page){if(!['ai','keys','usage','recharge','orders','profile'].includes(page))return;const previous=c.page;c.page=page;closeKeyRoute();U.closeModal();c.modal=null;if(previous==='ai'&&page!=='ai'){c.checkEpoch++;c.checks={};c.checking=false;}if(page==='usage')c.usagePage=1;if(location.hash!==`#${page}`)history.pushState(null,'',`#${page}`);c.render();window.scrollTo(0,0);document.querySelector('#main').focus({preventScroll:true});}
window.addEventListener('popstate',()=>navigate(location.hash.slice(1)||'ai'));
window.addEventListener('resize',()=>{if(c.keyRouteOpen){closeKeyRoute();c.render();}else P.drawTrend(c);});
const modal=document.querySelector('#modal');modal.addEventListener('cancel',e=>{e.preventDefault();D.closeDialog(c);});modal.addEventListener('click',e=>{if(e.target===modal){const b=modal.getBoundingClientRect();if(e.clientX<b.left||e.clientX>b.right||e.clientY<b.top||e.clientY>b.bottom)D.closeDialog(c);}});
document.addEventListener('click',async e=>{const b=e.target.closest('[data-action]');if(!b||b.disabled)return;const a=b.dataset.action,id=b.dataset.id;try{switch(a){
 case 'dropdown-toggle':{const root=b.closest('.brand-dropdown'),menu=root.querySelector('.brand-dropdown-menu'),opening=menu.hidden;closeDropdowns(root);menu.hidden=!opening;b.setAttribute('aria-expanded',String(opening));if(opening){const rect=b.getBoundingClientRect();root.classList.toggle('brand-dropdown-up',window.innerHeight-rect.bottom<Math.min(menu.scrollHeight+8,280)&&rect.top>window.innerHeight-rect.bottom);menu.querySelector('[aria-selected="true"]')?.focus({preventScroll:true});}break;}
 case 'dropdown-choose':{const root=b.closest('.brand-dropdown'),native=root.querySelector('select'),trigger=root.querySelector('.brand-dropdown-trigger');native.value=b.dataset.value;closeDropdowns();native.dispatchEvent(new Event('change',{bubbles:true}));const selected=document.querySelector(`.brand-dropdown-simple select[name="${CSS.escape(native.name)}"]`)?.closest('.brand-dropdown')?.querySelector('.brand-dropdown-trigger');(selected||trigger.isConnected&&trigger)?.focus({preventScroll:true});break;}
 case 'navigate':navigate(b.dataset.page);break;
 case 'close':D.closeDialog(c);break;
 case 'user-menu':{const menu=document.querySelector('#user-menu');menu.hidden=!menu.hidden;b.setAttribute('aria-expanded',String(!menu.hidden));break;}
 case 'logout':{const menu=document.querySelector('#user-menu');if(menu)menu.hidden=true;const user=document.querySelector('.user-bar .user');user?.setAttribute('aria-expanded','false');U.toast('已退出登录（演示）');break;}
 case 'create-key':D.openKey(c,{tool:b.dataset.tool||''});break;
 case 'edit-key':D.openKey(c,{id});break;
 case 'route-details':D.details(c,b.dataset.tool);break;
 case 'detail-create':D.openKey(c,{tool:b.dataset.tool,routeId:id,returnTo:{type:'details',tool:b.dataset.tool}});break;
 case 'detail-hours':c.detailHours=Number(b.dataset.hours);D.details(c,c.modal?.tool);break;
 case 'route-dropdown':{const opts=document.querySelector('#route-options');opts.hidden=!opts.hidden;b.setAttribute('aria-expanded',String(!opts.hidden));if(!opts.hidden)opts.querySelector('input').focus();break;}
 case 'choose-route':{const r=c.state.routes.find(r=>r.id===id);modal.querySelector('[name=routeId]').value=id;modal.querySelector('#route-value').innerHTML=D.routeChoice(r);modal.querySelector('#route-options').hidden=true;const t=modal.querySelector('.select-trigger');t.setAttribute('aria-expanded','false');t.setAttribute('aria-invalid','false');const error=modal.querySelector('#route-error');if(error)error.textContent='';t.focus();break;}
 case 'key-expires-preset':{const date=new Date();date.setDate(date.getDate()+Number(b.dataset.days));const input=modal.querySelector('[name=expires]');input.value=[date.getFullYear(),String(date.getMonth()+1).padStart(2,'0'),String(date.getDate()).padStart(2,'0')].join('-');break;}
 case 'key-route-dropdown':{if(c.keyRouteOpen===id)closeKeyRoute();else{c.keyRouteOpen=id;c.keyRouteSearch='';c.keyRouteAnchor=keyRouteAnchor(b,id);}c.render();document.querySelector('[data-input="key-route-search"]')?.focus({preventScroll:true});break;}
 case 'choose-key-route':{const keyId=b.dataset.keyId,key=c.state.keys.find(k=>k.id===keyId);if(!key)break;M.editKey(c.state,keyId,{routeId:id});closeKeyRoute();c.save();c.render();U.toast('绑定线路已更新，历史使用记录保持不变');break;}
 case 'related-keys':c.keyFilter={search:'',route:id,status:''};c.keyPage=1;navigate('keys');break;
 case 'check-lines':{if(c.checking)return;const epoch=++c.checkEpoch;c.failStats=false;c.checking=true;c.checks={};c.render();await new Promise(r=>setTimeout(r,850));if(epoch!==c.checkEpoch||c.page!=='ai')break;c.checking=false;c.checks=M.applyLineChecks(c.state);c.save();c.render();break;}
 case 'rate-detail':{const r=c.state.routes.find(r=>r.id===id);U.showModal('最近 1 小时',`<div class="modal-body"><h3>${r.name}</h3><dl class="kv"><dt>成功请求</dt><dd>${Math.round(r.total*r.success/100)} 次</dd><dt>总请求</dt><dd>${r.total} 次</dd></dl></div><footer class="modal-actions">${U.btn('关闭','close')}</footer>`);break;}
 case 'copy-key':{const k=c.state.keys.find(k=>k.id===id);if(k)await U.copy(k.token);break;}
 case 'copy-endpoint':await U.copy(M.endpoint);break;
 case 'copy-support-group':await U.copy('1080152144');break;
 case 'use-key':D.useKey(c,id);break;
 case 'import-key':D.importKey(c,id);break;
 case 'reset-key-quota':D.resetKeyUsage(c,id,'quota');break;
 case 'reset-key-rate':D.resetKeyUsage(c,id,'rate');break;
 case 'toggle-key':D.confirmKey(c,id,'toggle');break;
 case 'delete-key':D.confirmKey(c,id,'delete');break;
 case 'key-sort':c.keySort=[b.dataset.field,c.keySort[0]===b.dataset.field?-c.keySort[1]:1];c.keyPage=1;closeKeyRoute();c.render();break;
 case 'key-page':c.keyPage+=Number(b.dataset.delta);closeKeyRoute();c.render();break;
 case 'clear-keys':c.keyFilter={search:'',route:'',status:''};c.keyPage=1;closeKeyRoute();c.render();break;
 case 'refresh':c.failStats=false;c.updatedAt=new Date().toLocaleTimeString('en-GB');c.render();U.toast('已刷新当前数据');break;
 case 'columns':D.columns(c,b.dataset.table);break;
 case 'columns-reset':modal.querySelectorAll('[name=column]').forEach(n=>n.checked=c.modal.kind==='keys'?!defaultHiddenKeyColumns.includes(Number(n.value)):true);break;
 case 'apply-usage':clearUsageErrors();c.usagePage=1;c.updatedAt=new Date().toLocaleTimeString('en-GB');c.render();break;
 case 'reset-usage':clearUsageErrors();c.usageFilter=initialFilter();c.usageDraft=initialFilter();c.usagePage=1;c.usageErrorFilter={};c.usageDateOpen=false;c.granularity='hour';c.render();break;
 case 'retry-usage-stats':delete c.requestErrors.usageStats;c.render();U.toast('用量统计已恢复');break;
 case 'retry-usage-list':delete c.requestErrors.usageList;c.render();U.toast('使用记录已恢复');break;
 case 'retry-usage-errors':delete c.requestErrors.usageErrors;c.render();U.toast('错误请求已恢复');break;
 case 'chart-metric':c.chartMetric[b.dataset.chart]=b.dataset.metric;c.render();break;
 case 'usage-sort':c.usagePage=1;c.usageSort=[b.dataset.field,c.usageSort[0]===b.dataset.field?-c.usageSort[1]:1];c.render();break;
 case 'usage-error-sort':c.usageErrorSort=[b.dataset.field,c.usageErrorSort?.[0]===b.dataset.field?-c.usageErrorSort[1]:1];c.usagePage=1;c.render();break;
 case 'error-regions':{const rows=M.filterUsageErrors(c.state,{...c.usageErrorFilter,startDate:c.usageFilter.startDate,endDate:c.usageFilter.endDate}).slice((c.usagePage-1)*c.usagePageSize,c.usagePage*c.usagePageSize);await lookupRegions(rows);break;}
 case 'usage-tab':c.usageTab=b.dataset.tab;c.usagePage=1;c.render();break;
 case 'usage-date-toggle':c.usageDateOpen=!c.usageDateOpen;c.usageDateDraft={startDate:c.usageFilter.startDate,endDate:c.usageFilter.endDate};c.render();break;
 case 'usage-date-preset':c.usageDateDraft=M.usageDateRange(b.dataset.preset);c.render();break;
 case 'usage-date-apply':{const d=c.usageDateDraft;if(!d?.startDate||!d.endDate||d.startDate>d.endDate)throw Error('请选择有效的日期范围');clearUsageErrors();Object.assign(c.usageFilter,d);c.usageDraft={...c.usageFilter};c.usageDateOpen=false;c.usagePage=1;c.granularity=(new Date(d.endDate)-new Date(d.startDate))/86400000<=1?'hour':'day';c.render();break;}
 case 'usage-error-detail':{const r=c.state.errorRequests.find(r=>r.id===id);if(!r)break;U.showModal('错误请求详情',`<div class="modal-body"><dl class="kv">${[['请求 ID',r.id],['模型',r.model],['端点',r.endpoint],['状态码',r.status],['分类',M.usageErrorLabels[r.category]],['错误信息',r.message],['时间',U.date(r.time)]].map(([k,v])=>`<dt>${k}</dt><dd>${U.esc(v)}</dd>`).join('')}</dl></div><footer class="modal-actions">${U.btn('关闭','close')}</footer>`);break;}
 case 'usage-page':c.usagePage+=Number(b.dataset.delta);c.render();break;
 case 'request-detail':D.requestDetail(c,id);break;
 case 'regions':{const rows=P.usageRows(c).slice((c.usagePage-1)*c.usagePageSize,c.usagePage*c.usagePageSize);await lookupRegions(rows);break;}
 case 'export-csv':{if(c.failExport){c.failExport=false;throw Error('导出失败，请稍后重试。');}const rows=P.usageRows(c);U.download('星桥-使用记录.csv',M.toCsv(P.usageHeaders.slice(0,-1),rows.map(P.usageValues)),'text/csv;charset=utf-8');U.toast(`已导出 ${rows.length} 条筛选记录`);break;}
 case 'cache-info':U.showModal('缓存 Token',`<div class="modal-body"><p class="secondary">本演示将输入、输出和缓存 Token 分别统计，总 Token 为三者之和。所有概览、图表和明细使用相同筛选范围。</p></div><footer class="modal-actions">${U.btn('关闭','close')}</footer>`);break;
 case 'amount':c.amount=Number(b.dataset.amount);c.customAmount=false;c.render();break;
 case 'recharge-tab':c.rechargeTab=b.dataset.tab;c.render();break;
 case 'pay':{if(c.failPayment){c.failPayment=false;c.paymentError='订单创建失败：支付渠道暂时不可用，请稍后重试。';c.render();break;}const order=M.createOrder(c.state,c.amount);c.paymentError='';c.paymentFlow={orderId:order.id,orderNumber:order.number,amount:order.amount,payAmount:order.amount,paymentType:'alipay',expiresAt:Date.now()+30*60*1000,status:'pending'};c.state.paymentRecovery={...c.paymentFlow};c.save();c.render();U.toast('订单已创建，已进入原生支付承接');break;}
 case 'open-native-payment':U.toast('生产环境将由 sub 原生支付窗口、二维码或跳转页承接；当前原型不打开真实渠道。');break;
 case 'cancel-payment-order':D.confirmOrderCancel(c,id||c.paymentFlow?.orderId);break;
 case 'cancel-order':D.confirmOrderCancel(c,id);break;
 case 'order-detail':D.orderDetail(c,id);break;
 case 'go-orders':navigate('orders');break;
 case 'edit-profile':D.profileDialog(c,b.dataset.field);break;
 case 'send-code':{const email=modal.querySelector('[name=email]');if(!email.reportValidity())break;c.modal.sentTo=email.value.trim();b.textContent='重新发送';U.toast('演示验证码：123456（未发送真实邮件）');break;}
 case 'twofactor':D.security(c,'twofactor');break;
 case 'passkey':if(c.state.passkeyEnabled===true)D.security(c,'passkey');break;
 case 'remove-passkey':{c.state.profile.passkeys=c.state.profile.passkeys.filter(k=>k.id!==id);c.save();D.security(c,'passkey');c.render();break;}
 case 'avatar':{c.modal={type:'avatar'};U.showModal('更换头像',`<form id="avatar-form"><div class="modal-body"><p class="secondary">选择 JPG、PNG 或 WebP 图片，最大 2 MB。</p><div class="row" style="margin:24px 0"><span class="avatar big" id="avatar-preview">${U.esc(c.state.profile.name.slice(0,1))}</span><input type="file" name="avatar" accept="image/png,image/jpeg,image/webp" required aria-label="选择头像" data-change="avatar"></div>${U.errorBox()}</div>${U.formActions('保存头像')}</form>`);break;}
 case 'support':D.support(c);break;
 case 'demo':D.demo(c);break;
 case 'reset-demo':case 'empty-demo':{c.state=M.createSeed();if(a==='empty-demo'){c.state.keys=[];c.state.usage=[];c.state.orders=[];c.state.redemptions=[];}c.paymentFlow=null;c.paymentError='';delete c.state.paymentRecovery;c.save();c.keyFilter={search:'',route:'',status:''};c.keyPage=1;c.usageFilter=initialFilter();c.usageDraft=initialFilter();c.requestErrors={};c.failPayment=false;c.failStats=false;c.failGeo=false;c.failExport=false;c.checks={};navigate('ai');U.toast(a==='empty-demo'?'已切换到首次使用示例':'已恢复初始示例数据');break;}
 case 'fail-create':c.failCreate=true;U.closeModal();U.toast('下一次创建将显示失败，重试后恢复');break;
 case 'fail-payment':c.failPayment=true;c.paymentError='';U.closeModal();navigate('recharge');U.toast('下一次创建订单将显示真实失败反馈');break;
 case 'fail-stats':{c.failStats=true;U.closeModal();navigate('ai');U.toast('当前展示统计读取失败状态；重新检查线路可恢复');break;}
 case 'fail-usage-stats':{c.requestErrors.usageStats={message:'用量统计加载失败，请稍后重试。'};U.closeModal();navigate('usage');break;}
 case 'fail-usage-list':{c.requestErrors.usageList={message:'使用记录加载失败，请检查网络后重试。'};U.closeModal();navigate('usage');break;}
 case 'fail-geo':{c.failGeo=true;U.closeModal();navigate('usage');U.toast('下一次地区查询将显示失败状态');break;}
 case 'fail-export':{c.failExport=true;U.closeModal();navigate('usage');U.toast('下一次 CSV 导出将显示失败反馈');break;}
 }}catch(err){U.toast(err.message,true);}});
document.addEventListener('click',e=>{if(!e.target.closest('.user-bar')){const m=document.querySelector('#user-menu');if(m)m.hidden=true;}if(!e.target.closest('.brand-dropdown-simple'))closeDropdowns();if(c.keyRouteOpen&&!e.target.closest('.key-route-cell')){closeKeyRoute();c.render();}});
document.addEventListener('keydown',e=>{const root=e.target.closest('.brand-dropdown-simple'),menu=root?.querySelector('.brand-dropdown-menu'),trigger=root?.querySelector('.brand-dropdown-trigger');if(root&&['ArrowDown','ArrowUp','Home','End'].includes(e.key)){e.preventDefault();if(menu.hidden){trigger.click();return;}const options=[...menu.querySelectorAll('[role="option"]')],index=options.indexOf(document.activeElement),next=e.key==='Home'?0:e.key==='End'?options.length-1:(index+(e.key==='ArrowDown'?1:-1)+options.length)%options.length;options[next]?.focus({preventScroll:true});}if(e.key==='Escape'){const m=document.querySelector('#user-menu');if(m)m.hidden=true;if(root&&!menu.hidden){e.preventDefault();closeDropdowns();trigger.focus({preventScroll:true});}if(c.keyRouteOpen){closeKeyRoute();c.render();}}if(e.key==='Tab'&&root)closeDropdowns();});
document.addEventListener('input',e=>{const n=e.target;switch(n.dataset.input){case 'key-search':c.keyFilter.search=n.value;c.keyPage=1;closeKeyRoute();c.render();break;case 'key-route-search':c.keyRouteSearch=n.value;c.render();break;case 'route-search':{const routes=c.state.routes.filter(r=>(r.enabled||r.id===c.state.keys.find(k=>k.id===c.modal.id)?.routeId)&&(!c.modal.tool||r.tool===c.modal.tool)&&(r.name+' '+r.provider+' '+r.tool).toLowerCase().includes(n.value.toLowerCase()));modal.querySelector('#route-option-list').innerHTML=D.routeOptions(c,M.sortRoutesByQuality(routes),modal.querySelector('[name=routeId]').value);break;}case 'amount':{c.amount=Number(n.value);c.customAmount=true;c.paymentError='';document.querySelector('#credit-preview').textContent=U.money(c.amount);document.querySelector('#payment-preview').textContent=U.rmb(c.amount);document.querySelector('#pay-button').textContent='创建订单 '+U.rmb(c.amount);document.querySelectorAll('.amounts button').forEach(b=>{b.classList.toggle('selected',Number(b.dataset.amount)===c.amount);b.setAttribute('aria-pressed',String(Number(b.dataset.amount)===c.amount));});break;}}});
document.addEventListener('change',async e=>{const n=e.target;if(n.dataset.toggle){const d=document.getElementById(n.dataset.toggle);d.hidden=!n.checked;return;}try{switch(n.dataset.change){case 'detail-hours':c.detailHours=Number(n.value);D.details(c,c.modal?.tool);break;case 'key-filter':c.keyFilter[n.dataset.field]=n.value;c.keyPage=1;closeKeyRoute();c.render();break;case 'key-page-size':c.keyPageSize=Number(n.value);c.keyPage=1;closeKeyRoute();c.render();break;case 'usage-filter':clearUsageErrors();c.usageFilter[n.dataset.field]=n.value;c.usageDraft={...c.usageFilter};c.usagePage=1;c.updatedAt=new Date().toLocaleTimeString('en-GB');c.render();document.querySelector(`[data-change="usage-filter"][data-field="${n.dataset.field}"]`)?.focus();break;case 'usage-error-filter':clearUsageErrors();c.usageErrorFilter[n.dataset.field]=n.value;c.usagePage=1;c.render();break;case 'usage-date':c.usageDateDraft[n.dataset.field]=n.value;c.render();document.querySelector(`[data-change="usage-date"][data-field="${n.dataset.field}"]`)?.focus();break;case 'usage-page-size':c.usagePageSize=Number(n.value);c.usagePage=1;c.render();break;case 'granularity':c.granularity=n.value;c.render();break;case 'order-filter':c.orderFilter=n.value;c.render();break;case 'avatar':{const f=n.files[0];if(!f)return;if(!['image/png','image/jpeg','image/webp'].includes(f.type)||f.size>2*1024*1024){n.value='';throw Error('请选择不超过 2 MB 的 JPG、PNG 或 WebP 图片');}const reader=new FileReader();reader.onload=()=>{if(c.modal?.type!=='avatar')return;c.modal.avatar=reader.result;const p=document.querySelector('#avatar-preview');if(p)p.innerHTML=`<img src="${U.esc(reader.result)}" alt="头像预览">`;};reader.readAsDataURL(f);break;}}}catch(err){n.closest('dialog')?U.formError(err.message):U.toast(err.message,true);}});
document.addEventListener('submit',async e=>{e.preventDefault();const form=e.target;if(form.closest('dialog')&&form.id!=='avatar-form'){await D.submitDialog(c,form);return;}try{if(form.id==='redeem-form'){const raw=new FormData(form).get('code');M.redeem(c.state,raw);c.save();c.render();U.toast('兑换成功，账户权益已更新');}else if(form.id==='threshold-form'){const n=Number(new FormData(form).get('threshold'));if(!Number.isFinite(n)||n<0)throw Error('请输入有效的提醒阈值');c.state.profile.threshold=n;c.save();U.toast('提醒阈值已保存');}else if(form.id==='avatar-form'){if(!c.modal.avatar)throw Error('请选择图片并等待预览');c.state.profile.avatar=c.modal.avatar;c.save();U.closeModal();c.render();U.toast('头像已更新');}}catch(err){if(form.id==='redeem-form')document.querySelector('#redeem-error').textContent=err.message;else U.formError(err.message);}});
c.render();c.save();

document.addEventListener('keydown',e=>{if(e.key==='Escape'&&c.usageDateOpen){c.usageDateOpen=false;c.render();document.querySelector('[data-action=usage-date-toggle]')?.focus();}});
document.addEventListener('click',e=>{if(c.usageDateOpen&&!e.target.closest('.usage-date-picker')){c.usageDateOpen=false;c.render();}});
