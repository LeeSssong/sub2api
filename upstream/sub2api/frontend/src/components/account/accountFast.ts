export function readFastModels(extra?: Record<string, unknown>): string {
  const models = extra?.openai_fast_models
  return Array.isArray(models) ? models.filter((model): model is string => typeof model === 'string').join('\n') : ''
}

export function writeFastCapability(extra: Record<string, unknown>, supported: boolean, models: string): void {
  if (!supported) {
    delete extra.openai_fast_supported
    delete extra.openai_fast_models
    return
  }
  extra.openai_fast_supported = true
  const selected = [...new Set(models.split(/[\n,，]/).map(model => model.trim()).filter(Boolean))]
  if (selected.length) extra.openai_fast_models = selected
  else delete extra.openai_fast_models
}
