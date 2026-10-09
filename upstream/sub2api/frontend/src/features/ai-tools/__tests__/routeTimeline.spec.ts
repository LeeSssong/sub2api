import { beforeEach, describe, expect, it, vi } from 'vitest'
import { getRouteTimeline } from '../routeTimeline'
const get = vi.hoisted(() => vi.fn())
vi.mock('@/api/client', () => ({ apiClient: { get } }))
const base = {group_id:7,start:'2026-10-06T10:00:00Z',end:'2026-10-06T10:05:00Z',request_count:0,success_count:0,cache_hit_rate:null,ttft_p50_ms:null}
function response(counts:Record<string,unknown>) {get.mockResolvedValue({data:{window:'1h',success_rate_basis:'ops_sla',points:[{...base,...counts}]}})}
describe('route patrol count contract', () => {
 beforeEach(() => get.mockReset())
 it.each([
  {usage_request_count:100,degraded_request_count:5,quality_snapshot_request_count:100},
  {usage_request_count:0,degraded_request_count:0,quality_snapshot_request_count:0},
 ])('accepts quality traffic counters: %j',async counts=>{
  response(counts)
  expect((await getRouteTimeline('1h',new AbortController().signal))[0]?.usage_request_count).toBe(counts.usage_request_count)
 })
 it.each([
  {usage_request_count:1,degraded_request_count:2,quality_snapshot_request_count:1},
  {usage_request_count:1,degraded_request_count:0,quality_snapshot_request_count:2},
  {usage_request_count:1,degraded_request_count:1,quality_snapshot_request_count:0},
  {usage_request_count:1,degraded_request_count:0},
  {usage_request_count:-1,degraded_request_count:0,quality_snapshot_request_count:0},
 ])('rejects impossible quality traffic counters: %j',async counts=>{
  response(counts)
  await expect(getRouteTimeline('1h',new AbortController().signal)).rejects.toThrow('Invalid quality traffic counts')
 })
 it.each([undefined,'logical_request'])('rejects a response without the SLA basis: %s',async basis=>{
  get.mockResolvedValue({data:{window:'1h',success_rate_basis:basis,points:[base]}})
  await expect(getRouteTimeline('1h',new AbortController().signal)).rejects.toThrow('Invalid timeline success rate basis')
 })
 it('accepts zero scored rounds and legacy responses without score fields', async () => {
  response({graded_round_count:0,suspected_degraded_round_count:0})
  expect((await getRouteTimeline('1h',new AbortController().signal))[0]?.graded_round_count).toBe(0)
  response({})
  expect(await getRouteTimeline('1h',new AbortController().signal)).toHaveLength(1)
 })
 it.each([
  {graded_round_count:1,suspected_degraded_round_count:2},
  {graded_round_count:-1,suspected_degraded_round_count:0},
  {graded_round_count:1.5,suspected_degraded_round_count:1},
  {graded_round_count:1},
  {graded_round_count:1,suspected_degraded_round_count:'1'},
 ])('rejects impossible/incomplete count pairs: %j',async counts => {
  response(counts)
  await expect(getRouteTimeline('1h',new AbortController().signal)).rejects.toThrow('Invalid graded patrol counts')
 })
})
