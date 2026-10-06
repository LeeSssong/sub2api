import { describe, expect, it } from 'vitest'
import { modelCategory, sortOpenAIModels } from '../modelMetadata'
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
    expect(rows[0].tiers[0].context).toBe('全部上下文')
    expect(rows[1].tiers[0].context).toBe('暂无参考价')
    expect(formatNativePrice(rows[1].tiers[0].prices.input_price)).toBe('—')
  })
})

describe('官方模型分类和发布时间排序', () => {
  it('uses explicit release dates across families and keeps dated snapshots and unknown names', () => {
    const ids = ['codex-auto-review', 'gpt-5.2', 'gpt-4o-audio-preview', 'gpt-5.4', 'gpt-image-2', 'gpt-5.2-2025-12-11', 'gpt-5.4-mini']
    expect(sortOpenAIModels(ids)).toEqual(['gpt-image-2', 'gpt-5.4-mini', 'gpt-5.4', 'gpt-5.2', 'gpt-5.2-2025-12-11', 'gpt-4o-audio-preview', 'codex-auto-review'])
    expect(sortOpenAIModels(['private-model-2026-10-07', 'gpt-5.4-2099-01-01', 'gpt-6.1-sol'])[0]).toBe('gpt-6.1-sol')
    expect(ids[0]).toBe('codex-auto-review')
    expect(() => sortOpenAIModels(['gpt-5.4-2026-99-99', 'constructor', 'gpt-5.4'])).not.toThrow()
  })
  it.each([
    ['gpt-5.4', 'flagship'], ['gpt-5.6-cyber', 'cyber'], ['gpt-daybreak-blue-latest', 'cyber'],
    ['gpt-image-2', 'image'], ['gpt-4o-audio-preview', 'audio'], ['gpt-realtime', 'audio'],
    ['gpt-realtime-whisper', 'transcription'], ['gpt-realtime-translate', 'transcription'],
    ['gpt-4o-transcribe', 'transcription'], ['codex-auto-review', 'specialized'], ['custom-model', 'other'],
  ])('classifies %s using official model purposes', (id, category) => expect(modelCategory(id)).toBe(category))
})
