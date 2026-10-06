import type { PlazaOfficialPricing } from '@/api/modelPlaza'
import type { GroupModels } from './api'
import { formatScaled } from '@/utils/pricing'

export const priceColumns = [
  { key: 'input_price', label: '输入' },
  { key: 'cache_read_price', label: '缓存输入' },
  { key: 'cache_write_price', label: '缓存写入（5分钟）' },
  { key: 'cache_write_1h_price', label: '缓存写入（1小时）' },
  { key: 'output_price', label: '输出' },
] as const
type PriceKey = typeof priceColumns[number]['key']
type Prices = Partial<Record<PriceKey, number | null>>

export function formatNativePrice(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(value) || value < 0) return '—'
  return formatScaled(value, 1_000_000, 2)
}

/** Render all native tiers. Model IDs remain verbatim; aliases resolve on the server. */
export function nativePriceRows(models: string[], groups: GroupModels[]) {
  const prices = new Map<string, PlazaOfficialPricing>()
  for (const group of groups) {
    for (const [model, price] of Object.entries(group.official_pricing || {})) {
      if (price && !prices.has(model)) prices.set(model, price)
    }
  }
  return models.flatMap(model => {
    const price = prices.get(model)
    const intervals = price?.intervals
    if (intervals?.length) return intervals.map((interval, index) => ({
      key: `${model}:${index}`, model,
      context: interval.tier_label || `${interval.min_tokens.toLocaleString()}–${interval.max_tokens?.toLocaleString() ?? '不限'} Token`,
      prices: interval as Prices,
    }))
    return [{ key: model, model, context: price ? '全部上下文' : '暂无参考价', prices: (price || {}) as Prices }]
  })
}
