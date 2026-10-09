import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { createI18n } from 'vue-i18n'
import zh from '@/i18n/locales/zh/dashboard'
import en from '@/i18n/locales/en/dashboard'
import TokenUsageTrend from '../TokenUsageTrend.vue'
vi.mock('vue-chartjs', () => ({ Line: { props: ['data', 'options'], template: '<div />' } }))
const runtimeMessages = (value: Record<string, any>): any => Object.fromEntries(Object.entries(value).map(([key, item]) => [key, typeof item === 'string' ? () => item : runtimeMessages(item)]))
describe('token trend language switching', () => {
  it('updates legends and tooltip text in place using the selected locale', async () => {
    const i18n = createI18n({ legacy: false, locale: 'en', messages: { en: runtimeMessages(en), zh: runtimeMessages(zh) }, missingWarn: false, fallbackWarn: false })
    const wrapper = mount(TokenUsageTrend, { props: { trendData: [{ date: '2026-10-02', requests: 1, input_tokens: 500, output_tokens: 100, cache_creation_tokens: 0, cache_read_tokens: 1500, cost: 50.79, actual_cost: 6.1 }] }, global: { plugins: [i18n] } })
    const state = () => (wrapper.vm as any).$?.setupState
    for (const locale of ['zh', 'en', 'zh'] as const) {
      i18n.global.locale.value = locale
      await nextTick()
      const data = state().chartData
      const expected = locale === 'zh' ? ['输入', '输出', '缓存创建', '缓存读取', '缓存命中率'] : [en.usage.in, en.usage.out, en.usage.cacheCreationTokensLabel, en.usage.cacheReadTokensLabel, en.usage.cacheHitRate]
      expect(data.datasets.map((d: any) => d.label)).toEqual(expected)
      const callbacks = state().lineOptions.plugins.tooltip.callbacks
      expect(callbacks.label({ dataset: data.datasets[0], raw: 500 })).toBe(`${expected[0]}: 500`)
      expect(callbacks.label({ dataset: data.datasets[4], raw: 75 })).toBe(`${expected[4]}: 75.0%`)
      expect(callbacks.footer([{ dataIndex: 0 }])).toBe(`${i18n.global.t('usage.detail.actualCost')}: $6.10 | ${i18n.global.t('usage.detail.standardCost')}: $50.79`)
    }
    wrapper.unmount()
  })
})
