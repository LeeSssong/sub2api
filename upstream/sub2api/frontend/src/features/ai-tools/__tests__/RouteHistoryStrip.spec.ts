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
  it('renders three separately scaled series and leaves missing samples disconnected', () => {
    const w = mount(RouteHistoryStrip, { props: { points, loading: false, error: false } })
    expect(w.findAll('[data-series]')).toHaveLength(3)
    expect(w.get('[data-series="cache"]').attributes('d')?.match(/M/g)).toHaveLength(2)
    expect(w.get('[data-series="success"]').attributes('d')?.match(/L/g)).toHaveLength(2)
    expect(w.text()).toContain('100%')
    expect(w.text()).toContain('首字 P50')
    expect(w.get('[data-series="ttft"]').attributes('d')).not.toContain('NaN')
  })
  it('keeps point selection accessible without a repeated detail block', async () => {
    const w = mount(RouteHistoryStrip, { props: { points, loading: false, error: false } })
    expect(w.find('.chart-detail').exists()).toBe(false)
    await w.get('[data-point="0"]').trigger('click')
    expect(w.get('[aria-pressed="true"]').attributes('aria-label')).toContain('75%')
    expect(w.get('[aria-pressed="true"]').attributes('aria-label')).toContain('95%')
    expect(w.get('[aria-pressed="true"]').attributes('aria-label')).toContain('1.25s')
    await w.get('[data-point="0"]').trigger('keydown', { key: 'ArrowRight' })
    expect(w.get('[aria-pressed="true"]').attributes('aria-label')).toContain('缓存命中率 —')
    await w.get('[data-point="3"]').trigger('click')
    expect(w.get('[aria-pressed="true"]').attributes('aria-label')).toContain('无请求')
    expect(w.get('[aria-pressed="true"]').attributes('aria-label')).not.toContain('0%')
  })
  it('keeps loading and failure distinct from no requests', () => {
    const w = mount(RouteHistoryStrip, { props: { points: [], loading: true, error: false } })
    expect(w.text()).toContain('读取')
    const failed = mount(RouteHistoryStrip, { props: { points: [], loading: false, error: true } })
    expect(failed.text()).toContain('读取失败')
    expect(failed.find('svg').exists()).toBe(false)
  })
})
