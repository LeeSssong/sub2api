import { mount } from '@vue/test-utils'
import { describe, it, expect } from 'vitest'
import RouteHistoryStrip from '../RouteHistoryStrip.vue'

const point = {
  group_id: 7, start: '2026-10-09T10:00:00Z', end: '2026-10-09T10:05:00Z',
  request_count: 120, success_count: 100, cache_hit_rate: null, ttft_p50_ms: null,
  usage_request_count: 100, degraded_request_count: 5, quality_snapshot_request_count: 100,
}
describe('route nondegradation traffic', () => {
  it('shows 95% from usage traffic rather than SLA or patrol rounds', async () => {
    const w = mount(RouteHistoryStrip, { props: { points: [point], loading: false, error: false } })
    expect(w.get('.chart-legend button.degradation').text()).toContain('不降智率')
    await w.get('[data-point="0"]').trigger('click')
    expect(w.get('.tooltip-row.degradation b').text()).toBe('95%')
    expect(w.get('.tooltip-row.degradation small').text()).toBe('（95／100 次）')
  })
  it('shows 100% only with usage snapshots and no degraded traffic', async () => {
    const w = mount(RouteHistoryStrip, { props: { points: [{ ...point, degraded_request_count: 0 }], loading: false, error: false } })
    await w.get('[data-point="0"]').trigger('click')
    expect(w.get('.tooltip-row.degradation b').text()).toBe('100%')
  })
  it.each([
    { ...point, usage_request_count: 0, degraded_request_count: 0, quality_snapshot_request_count: 0 },
    { ...point, quality_snapshot_request_count: 80 },
    { group_id: 7, start: point.start, end: point.end, request_count: 100, success_count: 100, cache_hit_rate: null, ttft_p50_ms: null },
  ])('does not fabricate a percentage when the denominator or snapshot is missing', async p => {
    const w = mount(RouteHistoryStrip, { props: { points: [p], loading: false, error: false } })
    await w.get('[data-point="0"]').trigger('click')
    expect(w.get('.tooltip-row.degradation b').text()).toBe('暂无数据')
    expect(w.findAll('g.degradation circle')).toHaveLength(0)
  })
})
