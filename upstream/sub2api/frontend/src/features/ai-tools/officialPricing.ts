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
export const contextPriceColumns = priceColumns.filter(column => column.key !== 'cache_write_1h_price').map(column => ({
  ...column, label: column.key === 'cache_write_price' ? '缓存写入' : column.label,
}))
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
  return models.map(model => {
    const price = prices.get(model)
    const intervals = price?.intervals
    const tiers = intervals?.length ? intervals.map((interval, index) => ({
      label: intervals.length === 2 ? (index === 0 ? '短' : '长') : `第${index + 1}档`,
      context: interval.tier_label || contextRange(interval.min_tokens, interval.max_tokens),
      prices: interval as Prices,
    })) : [{ label: '', context: price ? '全部上下文' : '暂无参考价', prices: (price || {}) as Prices }]
    return { key: model, model, tiers }
  })
}

function contextRange(min: number, max: number | null) {
  const tokens = (value: number) => value >= 1000 ? `${value / 1000}K` : String(value)
  if (max == null) return `>${tokens(min)}`
  return min === 0 ? `≤${tokens(max)}` : `${tokens(min)}–${tokens(max)}`
}

/** A shared header range is safe only when every tiered model has that range. */
export function nativeContextGroups(rows: ReturnType<typeof nativePriceRows>) {
  const count = Math.max(1, ...rows.map(row => row.tiers.length))
  const tieredRows = rows.filter(row => row.tiers[0].label)
  return Array.from({ length: count }, (_, index) => {
    const contexts = new Set<string | undefined>(tieredRows.map(row => row.tiers[index]?.context))
    return {
      index,
      label: count === 2 ? (index === 0 ? '短上下文' : '长上下文') : count === 1 ? (tieredRows.length ? '上下文' : '全部上下文') : `第${index + 1}档上下文`,
      context: contexts.size === 1 && !contexts.has(undefined) ? [...contexts][0] : '',
    }
  })
}
