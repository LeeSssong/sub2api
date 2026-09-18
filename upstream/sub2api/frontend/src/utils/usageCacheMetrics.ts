interface CacheHitRateInput {
  input_tokens: number
  cache_creation_tokens: number
  cache_read_tokens: number
}

interface CacheSavingsInput {
  input_tokens: number
  input_cost: number
  cache_read_tokens: number
  cache_read_cost: number
  rate_multiplier: number
}

const finiteNonNegative = (value: number): number | null =>
  Number.isFinite(value) && value >= 0 ? value : null

export function calculateCacheHitRate(row: CacheHitRateInput): number | null {
  const inputTokens = finiteNonNegative(row.input_tokens)
  const cacheCreationTokens = finiteNonNegative(row.cache_creation_tokens)
  const cacheReadTokens = finiteNonNegative(row.cache_read_tokens)
  if (inputTokens == null || cacheCreationTokens == null || cacheReadTokens == null || cacheReadTokens === 0) {
    return null
  }

  const promptTokens = inputTokens + cacheCreationTokens + cacheReadTokens
  return promptTokens > 0 ? cacheReadTokens / promptTokens : null
}

export function calculateCacheSavings(row: CacheSavingsInput): number | null {
  const inputTokens = finiteNonNegative(row.input_tokens)
  const inputCost = finiteNonNegative(row.input_cost)
  const cacheReadTokens = finiteNonNegative(row.cache_read_tokens)
  const cacheReadCost = finiteNonNegative(row.cache_read_cost)
  const rateMultiplier = finiteNonNegative(row.rate_multiplier)
  if (
    inputTokens == null || inputTokens === 0 || inputCost == null ||
    cacheReadTokens == null || cacheReadTokens === 0 || cacheReadCost == null || rateMultiplier == null
  ) {
    return null
  }

  const uncachedEquivalentCost = cacheReadTokens * (inputCost / inputTokens)
  return Math.max(0, uncachedEquivalentCost - cacheReadCost) * rateMultiplier
}
