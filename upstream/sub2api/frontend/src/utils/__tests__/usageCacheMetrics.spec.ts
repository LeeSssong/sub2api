import { describe, expect, it } from 'vitest'

import { calculateCacheHitRate, calculateCacheSavings } from '../usageCacheMetrics'

describe('usage cache metrics', () => {
  it('calculates cache hit rate with the native prompt-token denominator', () => {
    expect(calculateCacheHitRate({
      input_tokens: 389,
      cache_creation_tokens: 0,
      cache_read_tokens: 96_800,
    })).toBeCloseTo(0.995997, 6)
  })

  it('includes cache creation tokens in the cache hit denominator', () => {
    expect(calculateCacheHitRate({
      input_tokens: 200,
      cache_creation_tokens: 300,
      cache_read_tokens: 500,
    })).toBe(0.5)
  })

  it('calculates user savings from the recorded input price and cache-read cost', () => {
    expect(calculateCacheSavings({
      input_tokens: 4_000,
      input_cost: 0.02,
      cache_read_tokens: 100_000,
      cache_read_cost: 0.05,
      rate_multiplier: 0.5,
    })).toBeCloseTo(0.225, 12)
  })

  it('does not estimate savings without a recorded input unit price', () => {
    expect(calculateCacheSavings({
      input_tokens: 0,
      input_cost: 0,
      cache_read_tokens: 100_000,
      cache_read_cost: 0.05,
      rate_multiplier: 1,
    })).toBeNull()
  })

  it('clamps negative savings to zero', () => {
    expect(calculateCacheSavings({
      input_tokens: 1_000,
      input_cost: 0.001,
      cache_read_tokens: 1_000,
      cache_read_cost: 0.002,
      rate_multiplier: 1,
    })).toBe(0)
  })
})
