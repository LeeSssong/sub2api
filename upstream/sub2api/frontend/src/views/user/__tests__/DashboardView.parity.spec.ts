import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import Dashboard from '../DashboardView.vue'
import { clearDashboardWorkspaceSnapshot } from '@/features/ai-tools/workspaceCache'
const mocks=vi.hoisted(()=>({groups:vi.fn(),rates:vi.fn(),keys:vi.fn(),snapshot:vi.fn(),check:vi.fn(),models:vi.fn(),push:vi.fn(),authUser:{id:7}}))
vi.mock('vue-router',()=>({useRouter:()=>({push:mocks.push})}))
vi.mock('@/stores/auth',()=>({useAuthStore:()=>({user:mocks.authUser})}))
vi.mock('@/api/groups',()=>({default:{getAvailable:mocks.groups,getUserGroupRates:mocks.rates}}))
vi.mock('@/api/keys',()=>({default:{list:mocks.keys}}))
vi.mock('@/features/monitor-v4/api',()=>({getHybridPerformanceSnapshot:mocks.snapshot}))
vi.mock('@/features/ai-tools/api',()=>({checkLines:mocks.check,getGroupModels:mocks.models}))
const groups=[{id:1,name:'GPT-Pro',platform:'openai',rate_multiplier:1,status:'active'},{id:2,name:'未关联线路',platform:'openai',rate_multiplier:.5,status:'active'}]
const metric=(id:number)=>({id,tool_ids:['codex'],success_rate:98,request_count:100,success_count:98,ttft_p50_ms:2160,latency_p50_ms:6500,real_request_count:100,real_success_count:98,source_updated_at:new Date().toISOString()})
const deferred=<T,>()=>{let resolve!:(value:T)=>void;let reject!:(reason?:unknown)=>void;const promise=new Promise<T>((res,rej)=>{resolve=res;reject=rej});return {promise,resolve,reject}}
const make=()=>mount(Dashboard,{global:{stubs:{AppLayout:{template:'<div><slot/></div>'},BaseDialog:{props:['show','title'],template:'<div v-if="show" role="dialog"><h3>{{title}}</h3><slot/><slot name="footer"/></div>'},CreateLineKeyDialog:{props:['show','initialGroupId'],template:'<div v-if="show" data-testid="create-key">{{initialGroupId}}</div>'}}}})
beforeEach(()=>{vi.clearAllMocks();clearDashboardWorkspaceSnapshot();mocks.models.mockResolvedValue([{group_id:1,supported_models:['gpt-5.4','gpt-5.2']},{group_id:2,supported_models:['gpt-5.4','custom-model']},{group_id:99,supported_models:['private-model']}]);mocks.groups.mockResolvedValue(groups);mocks.rates.mockResolvedValue({});mocks.keys.mockResolvedValue({items:[{id:1,group_id:1,status:'inactive',group:groups[0]}],total:1});mocks.snapshot.mockResolvedValue({generated_at:new Date().toISOString(),groups:groups.map(g=>metric(g.id))});mocks.check.mockResolvedValue([{group_id:1,status:'success',ttft_ms:1230}])})
describe('原型AI工具交互',()=>{
 it('summarizes each route health state with a count on the tool card',async()=>{
  const threeRoutes=[
   {id:1,name:'正常线路',platform:'openai',rate_multiplier:1,status:'active'},
   {id:2,name:'波动线路',platform:'openai',rate_multiplier:1,status:'active'},
   {id:3,name:'暂无数据线路',platform:'openai',rate_multiplier:1,status:'active'},
  ]
  mocks.groups.mockResolvedValue(threeRoutes)
  mocks.keys.mockResolvedValue({items:threeRoutes.map((group,index)=>({id:index+1,group_id:group.id,status:'active',group})),total:3})
  mocks.snapshot.mockResolvedValue({generated_at:new Date().toISOString(),groups:[
   {...metric(1),real_request_count:100,real_success_count:95},
   {...metric(2),real_request_count:100,real_success_count:80},
   {...metric(3),real_request_count:0,real_success_count:0},
  ]})
  const w=make();await flushPromises()
  const status=w.get('.tool-card')
  expect(status.find('[data-health-count="success"]').text()).toBe('正常运行 1')
  expect(status.find('[data-health-count="warning"]').text()).toBe('波动 1')
  expect(status.find('[data-health-count="muted"]').text()).toBe('暂无数据 1')
  expect(status.find('[data-health-count="danger"]').exists()).toBe(false)
  w.unmount()
 })

 it('invalidates one-hour health after detail failure and clears only the statistics error on recovery',async()=>{
  const w=make();await flushPromises()
  expect(w.get('.tool-status').text()).toContain('正常运行 2')
  mocks.snapshot.mockRejectedValueOnce(new Error('offline'))
  await w.get('button[aria-label="Codex 线路详情"]').trigger('click');await flushPromises()
  expect(w.get('.tool-status').text()).toContain('暂无数据 2')
  expect(w.get('.route-health').text()).toBe('暂无数据')
  expect(w.get('.statistics-error').text()).toContain('统计读取失败')
  await w.get('[role="dialog"]').findAll('button').find(b=>b.text()==='重试')!.trigger('click');await flushPromises()
  expect(w.get('.tool-status').text()).toContain('正常运行 2')
  expect(w.find('.statistics-error').exists()).toBe(false)
  w.unmount()
 })

 it.each([[0,'red'],[69.9,'red'],[70,'amber'],[89.9,'amber'],[90,'green'],[100,'green'],[null,'muted']] as const)('uses the shared success-rate tone for %s in both tables',async(rate,tone)=>{
  mocks.snapshot.mockResolvedValue({generated_at:new Date().toISOString(),groups:groups.map(g=>({...metric(g.id),success_rate:rate,request_count:rate===null?0:100,real_request_count:rate===null?0:1000,real_success_count:rate===null?0:rate*10}))})
  const w=make();await flushPromises()
  expect(w.get('.route-row .rate').attributes('data-tone')).toBe(tone)
  await w.get('button[aria-label="Codex 线路详情"]').trigger('click');await flushPromises()
  expect(w.get('.success-rate').attributes('data-tone')).toBe(tone)
  w.unmount()
 })
 it('keeps unavailable statistics neutral instead of implying success',async()=>{
  mocks.snapshot.mockRejectedValue(new Error('offline'))
  const w=make();await flushPromises()
  expect(w.get('.route-row .rate').text()).toBe('暂无数据')
  expect(w.get('.route-row .rate').attributes('data-tone')).toBe('muted')
  w.unmount()
 })
 it('weights card requests and keeps route status on one hour after selecting seven days',async()=>{
  mocks.snapshot.mockImplementation((window:string)=>Promise.resolve({generated_at:new Date().toISOString(),groups:groups.map(g=>({...metric(g.id),real_request_count:g.id===1?1:99,real_success_count:window==='7d'?0:g.id===1?0:99}))}))
  const w=make();await flushPromises()
  expect(w.get('.tool-status').find('[data-health-count="success"]').text()).toBe('正常运行 1')
  expect(w.get('.tool-status').find('[data-health-count="danger"]').text()).toBe('异常 1')
  expect(w.get('.route-row .route-name').text()).toContain('异常')
  await w.get('button[aria-label="Codex 线路详情"]').trigger('click');await flushPromises()
  await w.findAll('button').find(b=>b.text()==='近 7 天')!.trigger('click');await flushPromises()
  expect(w.findAll('.route-metrics-table tbody tr').find(r=>r.text().includes('GPT-Pro'))!.get('.route-health').text()).toBe('异常')
  expect(w.findAll('.route-metrics-table tbody tr').find(r=>r.text().includes('未关联线路'))!.get('.route-health').text()).toBe('正常运行')
  expect(w.text()).not.toMatch(/可用 ·|不可用 ·|0\/2 条/)
  w.unmount()
 })
 it('uses the effective multiplier with its suffix in both line tables',async()=>{
  mocks.rates.mockResolvedValue({1:0.12})
  const w=make();await flushPromises()
  expect(w.get('.route-row .rate-badge').text()).toBe('0.12x倍率')
  await w.get('button[aria-label="Codex 线路详情"]').trigger('click');await flushPromises()
  expect(w.get('[role="dialog"]').text()).toContain('0.12x倍率')
  expect(w.get('[role="dialog"]').text()).toContain('0.5x倍率')
  w.unmount()
 })
 it('keeps associate action even with keys and opens creation without navigation',async()=>{const w=make();await flushPromises();const b=w.findAll('button').find(b=>b.text()==='关联密钥')!;await b.trigger('click');expect(w.find('[data-testid="create-key"]').exists()).toBe(true);expect(mocks.push).not.toHaveBeenCalled();w.unmount()})
 it('only lists linked lines and starts checks without using historical latency',async()=>{const w=make();await flushPromises();const list=w.get('[aria-labelledby="routes-title"]');expect(list.text()).not.toContain('未关联线路');expect(list.text()).not.toContain('2.16s');await w.get('button[aria-label="检查线路"]').trigger('click');await flushPromises();expect(mocks.check.mock.calls[0][0]).toEqual([1]);expect(list.text()).toContain('1.23s');w.unmount()})
 it('shows all detail metrics and fetches selected period',async()=>{const w=make();await flushPromises();await w.get('button[aria-label="Codex 线路详情"]').trigger('click');await flushPromises();expect(w.get('[role="dialog"]').text()).toContain('首字 P50');expect(w.get('[role="dialog"]').text()).toContain('2.16s');await w.findAll('button').find(b=>b.text()==='近 7 天')!.trigger('click');await flushPromises();expect(mocks.snapshot.mock.calls.some(c=>c[0]==='7d')).toBe(true);w.unmount()})
 it('initial load failure shows retry instead of invented empty data',async()=>{mocks.groups.mockRejectedValue(new Error('offline'));const w=make();await flushPromises();expect(w.find('[role="alert"]').text()).toContain('重试');expect(w.find('.tool-grid').exists()).toBe(false);w.unmount()})
 it('shows the workspace before monitoring statistics finish loading',async()=>{const pending=deferred<{groups:ReturnType<typeof metric>[]}>();mocks.snapshot.mockReturnValue(pending.promise);const w=make();await flushPromises();expect(w.find('.tool-grid').exists()).toBe(true);expect(w.text()).not.toContain('正在读取 AI 工具');pending.resolve({generated_at:new Date().toISOString(),groups:groups.map(g=>metric(g.id))});await flushPromises();w.unmount()})
 it('keeps the workspace visible when monitoring statistics fail',async()=>{mocks.snapshot.mockRejectedValue(new Error('offline'));const w=make();await flushPromises();expect(w.find('.tool-grid').exists()).toBe(true);expect(w.find('[role="alert"]').exists()).toBe(true);expect(w.text()).toContain('暂无数据');w.unmount()})
 it('restores the last successful workspace immediately when returning to the page',async()=>{const first=make();await flushPromises();first.unmount();const pendingGroups=deferred<typeof groups>();mocks.groups.mockReturnValue(pendingGroups.promise);const second=make();await flushPromises();expect(second.find('.tool-grid').exists()).toBe(true);expect(second.text()).not.toContain('正在读取 AI 工具');pendingGroups.resolve(groups);await flushPromises();second.unmount()})
 it('falls back to the group platform when no explicit tool mapping exists',async()=>{
  const anthropic={id:3,name:'default',platform:'anthropic',rate_multiplier:1,status:'active'}
  mocks.groups.mockResolvedValue([groups[0],anthropic])
  mocks.keys.mockResolvedValue({items:[{id:1,group_id:1,status:'inactive',group:groups[0]},{id:2,group_id:3,status:'active',group:anthropic}],total:2})
  mocks.snapshot.mockResolvedValue({generated_at:new Date().toISOString(),groups:[metric(1),{...metric(3),tool_ids:[]}]})
  const w=make();await flushPromises()
  const cards=w.findAll('.tool-card')
  expect(cards[0].text()).toContain('已关联 1 把密钥')
  expect(cards[1].text()).toContain('已关联 1 把密钥')
  expect(cards[0].find('button').attributes('disabled')).toBeUndefined()
  expect(cards[1].find('button').attributes('disabled')).toBeUndefined()
  w.unmount()
 })
 it('preserves the last successful detail metrics after a same-window refresh fails',async()=>{const w=make();await flushPromises();await w.get('button[aria-label="Codex 线路详情"]').trigger('click');await flushPromises();mocks.snapshot.mockRejectedValue(new Error('offline'));await w.findAll('button').find(b=>b.text()==='近 1 小时')!.trigger('click');await flushPromises();expect(w.get('[role="dialog"]').text()).toContain('2.16s');expect(w.get('[role="dialog"]').text()).toContain('98%');w.unmount()})

 it('shows only current tool group models and switches official price tiers without changing fees',async()=>{
  mocks.rates.mockResolvedValue({1:0.12})
  const w=make();await flushPromises()
  await w.get('button[aria-label="Codex 价格与扣费说明"]').trigger('click');await flushPromises()
  const dialog=w.get('[role="dialog"]')
  expect(dialog.text()).toContain('OpenAI 官方定价')
  const rows=dialog.findAll('.model-pricing-table tbody tr')
  expect(rows.map(r=>r.find('th').text())).toEqual(['custom-model','gpt-5.2','gpt-5.4'])
  expect(dialog.text()).not.toContain('private-model')
  const gpt=()=>dialog.findAll('.model-pricing-table tbody tr').find(r=>r.find('th').text()==='gpt-5.4')!
  expect(gpt().findAll('td')[0].text()).toBe('$2.50')
  await dialog.findAll('button').find(b=>b.text()==='Batch')!.trigger('click')
  expect(gpt().findAll('td')[0].text()).toBe('$1.25')
  expect(gpt().findAll('td')[1].text()).toBe('$0.13')
  await dialog.findAll('button').find(b=>b.text()==='Ultrafast')!.trigger('click')
  expect(gpt().findAll('td')[0].text()).toBe('待核对')
  const fees=dialog.get('.fee-table')
  expect(fees.findAll('thead th').map(th=>th.text())).toEqual(['分组','支持模型','倍率','扣费标准'])
  expect(fees.text()).toContain('gpt-5.4、gpt-5.2')
  expect(fees.text()).toContain('模型基础费用 × 0.12')
  expect(dialog.text()).not.toContain('Input')
  w.unmount()
 })
 it('recovers model loading failure through retry without hiding the workspace',async()=>{
  mocks.models.mockRejectedValueOnce(new Error('offline'))
  const w=make();await flushPromises()
  await w.get('button[aria-label="Codex 价格与扣费说明"]').trigger('click');await flushPromises()
  expect(w.get('.tool-grid').exists()).toBe(true)
  expect(w.get('[role="dialog"]').text()).toContain('模型读取失败')
  await w.get('[role="dialog"]').findAll('button').find(b=>b.text()==='重试')!.trigger('click');await flushPromises()
  expect(w.get('[role="dialog"]').text()).toContain('gpt-5.4')
  expect(w.get('[role="dialog"]').text()).not.toContain('模型读取失败')
  w.unmount()
 })
 it('does not invent supported models for groups with no configuration',async()=>{
  mocks.models.mockResolvedValue([{group_id:1,supported_models:[]}])
  const w=make();await flushPromises()
  await w.get('button[aria-label="Codex 价格与扣费说明"]').trigger('click');await flushPromises()
  expect(w.get('[role="dialog"]').text()).toContain('尚未配置')
  expect(w.get('.fee-table').text()).toContain('待配置')
  expect(w.find('.model-pricing-table').exists()).toBe(false)
  w.unmount()
 })

})
