import { mount } from '@vue/test-utils'
import { describe, it, expect } from 'vitest'
import RouteHistoryStrip from '../RouteHistoryStrip.vue'
const points = [95, 80, 50, null].map((rate, i) => ({
  group_id: 1, start: new Date(Date.UTC(2026, 9, 5, 10, i * 5)).toISOString(),
  end: new Date(Date.UTC(2026, 9, 5, 10, (i + 1) * 5)).toISOString(),
  request_count: rate === null ? 0 : 100, success_count: rate ?? 0,
  cache_hit_rate: i === 1 || rate === null ? null : 0.75, ttft_p50_ms: rate === null ? null : 1250,
}))
describe('RouteHistoryStrip metric chart', () => {
  it('does not fabricate round counts when an older API has no quality fields', async () => {
    const w = mount(RouteHistoryStrip, { props: { points, loading: false, error: false } })
    await w.get('[data-point="0"]').trigger('click')
    expect(w.get('.tooltip-row.degradation b').text()).toBe('0%')
    expect(w.get('.tooltip-row.degradation small').text()).toBe('')
    expect(w.get('.chart-tooltip').text()).toContain('无有效评分，按 0% 展示')
  })
  it('shows suspected degradation once per graded round and connects no-sample points at zero', async () => {
    const samples = points.map((p,i) => ({...p, graded_round_count: i===1 ? 0 : 4, suspected_degraded_round_count: i===0 ? 1 : 0}))
    const w = mount(RouteHistoryStrip, {props:{points:samples,loading:false,error:false}})
    expect(w.get('.chart-legend button.degradation').text()).toContain('疑似降智率')
    expect(w.get('[data-series="degradation"]').attributes('d')?.match(/M/g)).toHaveLength(1)
    expect(w.get('[data-series="degradation"]').attributes('d')?.match(/L/g)).toHaveLength(3)
    expect(w.findAll('g.degradation circle')).toHaveLength(4)
    await w.get('[data-point="0"]').trigger('click')
    expect(w.get('.tooltip-row.degradation b').text()).toBe('25%')
    expect(w.get('.tooltip-row.degradation small').text()).toBe('（1／4 轮）')
    expect(w.get('.chart-tooltip').text()).not.toContain('无有效评分')
    await w.get('[data-point="1"]').trigger('click')
    expect(w.get('.tooltip-row.degradation b').text()).toBe('0%')
    expect(w.get('.tooltip-row.degradation small').text()).toBe('（0／0 轮）')
    expect(w.get('.chart-tooltip').text()).toContain('无有效评分，按 0% 展示')
    expect(w.get('[data-point="1"]').attributes('aria-label')).toContain('疑似降智率 0%（无有效评分，按 0% 展示）')
    await w.get('.chart-legend button.degradation').trigger('click')
    expect(w.find('[data-series="degradation"]').exists()).toBe(false)
  })
  it('renders three separately scaled series and renders missing samples at zero with an explicit hint', () => {
    const w = mount(RouteHistoryStrip, { props: { points, loading: false, error: false } })
    expect(w.findAll('[data-series]')).toHaveLength(4)
    expect(w.get('[data-series="cache"]').attributes('d')?.match(/M/g)).toHaveLength(1)
    expect(w.get('[data-series="success"]').attributes('d')?.match(/L/g)).toHaveLength(3)
    expect(w.text()).toContain('100%')
    expect(w.text()).toContain('首字 P50')
    expect(w.get('[data-series="ttft"]').attributes('d')).not.toContain('NaN')
  })
  it('keeps point selection accessible without a repeated detail block', async () => {
    const w = mount(RouteHistoryStrip, { props: { points, loading: false, error: false } })
    expect(w.find('.chart-detail').exists()).toBe(false)
    await w.get('[data-point="0"]').trigger('click')
    expect(w.get('[data-point][aria-pressed="true"]').attributes('aria-label')).toContain('75%')
    expect(w.get('[data-point][aria-pressed="true"]').attributes('aria-label')).toContain('95%')
    expect(w.get('[data-point][aria-pressed="true"]').attributes('aria-label')).toContain('1.25s')
    await w.get('[data-point="0"]').trigger('keydown', { key: 'ArrowRight' })
    expect(w.get('[data-point][aria-pressed="true"]').attributes('aria-label')).toContain('缓存命中率 0%（无样本，按 0 展示）')
    await w.get('[data-point="3"]').trigger('click')
    expect(w.get('[data-point][aria-pressed="true"]').attributes('aria-label')).toContain('无请求')
    expect(w.get('[data-point][aria-pressed="true"]').attributes('aria-label')).toContain('无样本，按 0 展示')
  })
  it('toggles curves and shows a transient tooltip for hover, touch and keyboard',async()=>{
    const w=mount(RouteHistoryStrip,{props:{points,loading:false,error:false}})
    expect(w.find('.chart-tooltip').exists()).toBe(false)
    await w.get('.chart-legend button.cache').trigger('click')
    expect(w.find('[data-series="cache"]').exists()).toBe(false)
    await w.get('[data-point="0"]').trigger('mouseenter')
    expect(w.get('.chart-tooltip').text()).toContain('75%');expect(w.get('.chart-tooltip').text()).toContain('1.25s')
    await w.get('.chart-plot').trigger('mouseleave');expect(w.find('.chart-tooltip').exists()).toBe(false)
    await w.get('[data-point="1"]').trigger('click');expect(w.get('.tooltip-row.cache b').text()).toBe('0%');expect(w.get('.tooltip-note').text()).toContain('无样本')
    await w.get('.chart-plot').trigger('keydown',{key:'Escape'});expect(w.find('.chart-tooltip').exists()).toBe(false)
    await w.get('.chart-legend button.cache').trigger('click');expect(w.findAll('[data-series]')).toHaveLength(4)
  })
  it('keeps loading and failure distinct from no requests', () => {
    const w = mount(RouteHistoryStrip, { props: { points: [], loading: true, error: false } })
    expect(w.text()).toContain('读取')
    const failed = mount(RouteHistoryStrip, { props: { points: [], loading: false, error: true } })
    expect(failed.text()).toContain('读取失败')
    expect(failed.find('svg').exists()).toBe(false)
  })
})
