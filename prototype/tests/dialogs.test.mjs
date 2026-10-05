import test from 'node:test';
import assert from 'node:assert/strict';
import * as M from '../model.mjs';
import * as Dialogs from '../dialogs.mjs';
const {routeDetailsContent,routeOptions,keyFormContent,closeDialog,profileDialog}=Dialogs;

test('线路详情移除顶部概要，展示状态、统一线路身份和最佳线路标签',()=>{
 const c={state:M.createSeed(),detailHours:1};
 const html=routeDetailsContent(c,'Codex');
 assert.doesNotMatch(html,/details-overview/);
 assert.match(html,/route-provider-logo/);
 assert.match(html,/GPT-Pro/);
 assert.match(html,/1\.0x/);
 assert.match(html,/可用/);
 assert.match(html,/不可用/);
 assert.match(html,/最佳线路/);
 assert.doesNotMatch(html,/<h3>线路指标<\/h3>/);
 assert.match(html,/<th scope="col">关联密钥<\/th>/);
 assert.doesNotMatch(html,/<th scope="col">关联状态<\/th>|<th scope="col">操作<\/th>/);
 assert.match(html,/已关联 · 2 把/);
 assert.match(html,/未关联 · 0 把/);
 assert.match(html,/关联密钥单元格/);
 assert.equal((html.match(/data-action="detail-create"/g)||[]).length,3);
 assert.equal((html.match(/>关联密钥<\/button>/g)||[]).length,3);
 assert.match(html,/detail-period-segment/);
 assert.match(html,/data-action="detail-hours"[^>]+data-hours="1"[^>]+aria-pressed="true"/);
 assert.doesNotMatch(html,/<select[^>]+线路统计时间/);
 assert.match(html,/>97%<\/strong>/);
 assert.doesNotMatch(html,/success-metric|--success:/);
 assert.ok(html.indexOf('GPT-Pro')<html.indexOf('GPT-经济'));
 assert.ok(html.indexOf('GPT-经济')<html.indexOf('GPT-极速'));
});

test('线路详情和创建线路选项复用成功率阈值，缺失数据保留占位',()=>{
 const c={state:M.createSeed(),detailHours:1};
 const details=routeDetailsContent(c,'Codex');
 assert.match(details,/class="success-rate data rate-high">97%<\/strong>/);
 assert.match(details,/class="success-rate data rate-medium">68%<\/strong>/);
 c.state.routes.find(r=>r.id==='gpt-fast').success=49;
 assert.match(routeDetailsContent(c,'Codex'),/class="success-rate data rate-low">49%<\/strong>/);
 const options=routeOptions(c,c.state.routes);
 assert.match(options,/近 1 小时稳定性 <span class="rate-low">49%<\/span>/);
 assert.match(options,/近 1 小时稳定性 <span class="rate-unknown">—<\/span>/);
 assert.doesNotMatch(options,/—%/);
});

test('创建密钥表单使用单列紧凑结构并保留全部限制项',()=>{
 const c={state:M.createSeed()};
 const html=keyFormContent(c,{});
 assert.match(html,/id="key-form" class="compact-key-form"/);
 assert.match(html,/key-form-stack/);
 assert.match(html,/key-settings-stack/);
 assert.doesNotMatch(html,/key-form-grid|key-settings-grid/);
 assert.ok(html.indexOf('>名称<')<html.indexOf('>线路<'));
 assert.match(html,/关联密钥 2 把/);
 assert.match(html,/关联密钥 0 把/);
 assert.match(html,/1\.0x倍率/);
 assert.match(html,/0\.8x倍率/);
 assert.match(html,/可用/);
 assert.match(html,/不可用/);
 assert.doesNotMatch(html,/>1\.0x<\/span>|>0\.8x<\/span>/);
 assert.doesNotMatch(html,/>已配置<|>未配置</);
 for(const label of ['线路','名称','自定义密钥','IP 限制','额度限制','速率限制','密钥有效期'])assert.match(html,new RegExp(label));
});

test('额度与速率重置明确确认范围，CCS 不伪报外部导入完成',()=>{
 const key=M.createSeed().keys[0];
 assert.match(Dialogs.resetKeyUsageContent(key,'quota'),/确认重置额度用量/);
 assert.match(Dialogs.resetKeyUsageContent(key,'rate'),/确认重置速率用量/);
 assert.match(Dialogs.importKeyContent(key),/不会自动写入外部应用/);
 assert.doesNotMatch(Dialogs.importKeyContent(key),/模拟导入|已完成模拟/);
});

test('密钥表单提供原生三个额度窗口和 IP 白黑名单',()=>{
 const html=keyFormContent({state:M.createSeed()},{});
 for(const field of ['IP 白名单','IP 黑名单','5 小时额度','1 天额度','7 天额度'])assert.match(html,new RegExp(field));
 assert.doesNotMatch(html,/每分钟请求数/);
});

test('AI 工具创建密钥复用同一线路选项展示，但只包含当前工具线路',()=>{
 const c={state:M.createSeed()};
 const html=keyFormContent(c,{tool:'Codex'});
 assert.match(html,/GPT-Pro/);
 assert.match(html,/GPT-经济/);
 assert.match(html,/GPT-极速/);
 assert.match(html,/1\.0x倍率/);
 assert.match(html,/可用/);
 assert.doesNotMatch(html,/Claude-常规|Grok-特惠|DeepSeek/);
});

test('从线路详情进入创建密钥后关闭会返回原线路详情',async()=>{
 const originalDocument=globalThis.document;
 const modal={open:true,className:'',innerHTML:'',scrollTop:0,classList:{add(){}},querySelector(){return null;}};
 globalThis.document={querySelector(selector){return selector==='#modal'?modal:null;}};
 try{
  const c={state:M.createSeed(),detailHours:1,modal:{type:'key',tool:'Codex',returnTo:{type:'details',tool:'Codex'}}};
  closeDialog(c);
  assert.equal(c.modal.type,'details');
  assert.equal(c.modal.tool,'Codex');
  assert.match(modal.innerHTML,/Codex 线路详情/);
  await Promise.resolve();
 }finally{
  globalThis.document=originalDocument;
 }
});

test('客服弹窗沿用原生 QQ 群与 contact_info，不再展示工单表单',async()=>{
 const originalDocument=globalThis.document;
 const modal={
  open:false,className:'',innerHTML:'',scrollTop:0,
  showModal(){this.open=true;},
  querySelector(){return null;}
 };
 globalThis.document={activeElement:null,querySelector(selector){return selector==='#modal'?modal:null;}};
 try{
  const {support}=await import('../dialogs.mjs');
  support({state:{contactInfo:'',profile:{email:'awen@example.com'}},modal:null});
  assert.match(modal.innerHTML,/qq-group-1080152144\.png/);
  assert.match(modal.innerHTML,/1080152144/);
  assert.match(modal.innerHTML,/复制群号/);
  assert.match(modal.innerHTML,/暂未配置其他联系方式/);
  support({state:{contactInfo:'工作日 09:00–18:00：support@example.com'},modal:null});
  assert.match(modal.innerHTML,/工作日 09:00–18:00：support@example\.com/);
  assert.doesNotMatch(modal.innerHTML,/暂未配置其他联系方式/);
  assert.doesNotMatch(modal.innerHTML,/support-form|问题类型|问题描述|保存问题/);
 }finally{
  globalThis.document=originalDocument;
 }
});


test('密码弹窗按 passwordSet 区分首设与修改合同',async()=>{
 const originalDocument=globalThis.document;
 const modal={open:false,className:'',innerHTML:'',scrollTop:0,showModal(){this.open=true;},querySelector(){return null;}};
 globalThis.document={activeElement:null,querySelector(selector){return selector==='#modal'?modal:null;}};
 try{
  const first={state:M.createSeed(),modal:null};
  first.state.profile.passwordSet=false;
  profileDialog(first,'password');
  assert.match(modal.innerHTML,/<h2 id="modal-title">设置密码<\/h2>/);
  assert.match(modal.innerHTML,/name="email"/);
  assert.match(modal.innerHTML,/name="code"/);
  assert.match(modal.innerHTML,/data-action="send-code"/);
  assert.match(modal.innerHTML,/name="password"/);
  assert.doesNotMatch(modal.innerHTML,/name="currentPassword"|身份验证信息|DEMO/);

  const change={state:M.createSeed(),modal:null};
  modal.open=false;
  profileDialog(change,'password');
  assert.match(modal.innerHTML,/<h2 id="modal-title">修改密码<\/h2>/);
  assert.match(modal.innerHTML,/name="currentPassword"/);
  assert.match(modal.innerHTML,/name="password"/);
  assert.doesNotMatch(modal.innerHTML,/name="email"|name="code"|data-action="send-code"|身份验证信息|DEMO/);
 }finally{
  globalThis.document=originalDocument;
 }
});
