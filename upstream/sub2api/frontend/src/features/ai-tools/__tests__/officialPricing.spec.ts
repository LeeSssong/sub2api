import { describe, expect, it } from 'vitest'
import { formatNativePrice, nativePriceRows } from '../officialPricing'

describe('原生参考价展示', () => {
  it('preserves free and very small prices without rounding them to zero', () => {
    expect(formatNativePrice(0)).toBe('$0.00')
    expect(formatNativePrice(1.25e-8)).toBe('$0.0125')
    expect(formatNativePrice(1e-15)).not.toBe('$0.00')
    expect(formatNativePrice(undefined)).toBe('—')
    expect(formatNativePrice(NaN)).toBe('—')
  })
  it('keeps native names and does not substitute prices for aliases or missing models', () => {
    const rows = nativePriceRows(['native-dated-alias', 'unpriced'], [{ group_id: 1, supported_models: ['native-dated-alias', 'unpriced'], official_pricing: {
      'native-dated-alias': { input_price: 4e-6, output_price: 12e-6, cache_read_price: null, cache_write_price: null },
      unpriced: null,
    } }])
    expect(rows[0].model).toBe('native-dated-alias')
    expect(rows[0].context).toBe('全部上下文')
    expect(rows[1].context).toBe('暂无参考价')
    expect(formatNativePrice(rows[1].prices.input_price)).toBe('—')
  })
})
