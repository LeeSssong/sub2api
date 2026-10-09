/** Presentation metadata only; model availability and prices remain native.
 * Categories: https://developers.openai.com/api/docs/pricing
 * Dates: https://developers.openai.com/api/docs/changelog and model snapshot docs.
 * Verified 2026-10-07. Never invent release dates for private aliases.
 */
export const modelCategories = [
  { value: 'all', label: '全部模型' },
  { value: 'flagship', label: '旗舰模型' },
  { value: 'cyber', label: '网络安全模型' },
  { value: 'image', label: '图像生成模型' },
  { value: 'audio', label: '实时和音频生成模型' },
  { value: 'transcription', label: '转录模型' },
  { value: 'specialized', label: '专用模型' },
  { value: 'other', label: '其他模型' },
] as const
export type ModelCategory = typeof modelCategories[number]['value']

// Explicit IDs only. Dates describe API releases and released snapshots, not
// knowledge cutoffs or a gateway's synthetic /v1/models created timestamp.
const releases: Record<string, string> = Object.create(null)
const releaseGroups: [string, string[]][] = [
  ['2026-09-29', ['gpt-6.1-sol']],
  ['2026-09-22', ['gpt-6-sol', 'gpt-6-luna']],
  ['2026-09-08', ['gpt-image-2.5-sunburst', 'gpt-image-2.5-flare']],
  ['2026-09-03', ['gpt-6-astra']],
  ['2026-07-28', ['gpt-transcribe', 'gpt-live-transcribe']],
  ['2026-07-09', ['gpt-5.6-sol', 'gpt-5.6-terra', 'gpt-5.6-luna']],
  ['2026-07-06', ['gpt-realtime-2.1', 'gpt-realtime-2.1-mini']],
  ['2026-05-07', ['gpt-realtime-2', 'gpt-realtime-translate', 'gpt-realtime-whisper']],
  ['2026-04-24', ['gpt-5.5', 'gpt-5.5-pro']],
  ['2026-04-21', ['gpt-image-2']],
  ['2026-03-17', ['gpt-5.4-mini', 'gpt-5.4-nano']],
  ['2026-03-05', ['gpt-5.4', 'gpt-5.4-pro']],
  ['2026-03-03', ['gpt-5.3-chat-latest']],
  ['2026-02-24', ['gpt-5.3-codex']],
  ['2026-02-23', ['gpt-realtime-1.5', 'gpt-audio-1.5']],
  ['2026-01-14', ['gpt-5.2-codex']],
  ['2025-12-16', ['gpt-image-1.5', 'chatgpt-image-latest']],
  ['2025-12-15', ['gpt-4o-mini-transcribe', 'gpt-4o-mini-tts']],
  ['2025-12-11', ['gpt-5.2', 'gpt-5.2-pro', 'gpt-5.2-chat-latest']],
  ['2025-12-04', ['gpt-5.1-codex-max']],
  ['2025-11-13', ['gpt-5.1', 'gpt-5.1-codex', 'gpt-5.1-chat-latest', 'gpt-5.1-codex-mini']],
  ['2025-10-06', ['gpt-5-pro', 'gpt-realtime-mini', 'gpt-audio-mini', 'gpt-image-1-mini']],
  ['2025-09-23', ['gpt-5-codex']],
  ['2025-08-28', ['gpt-realtime']],
  ['2025-08-07', ['gpt-5', 'gpt-5-mini', 'gpt-5-nano']],
  ['2025-06-24', ['o3-deep-research', 'o4-mini-deep-research']],
  ['2025-06-10', ['o3-pro']],
  ['2025-05-15', ['codex-mini-latest']],
  ['2025-04-16', ['o3', 'o4-mini']],
  ['2025-04-14', ['gpt-4.1', 'gpt-4.1-mini', 'gpt-4.1-nano']],
  ['2025-03-20', ['gpt-4o-transcribe']],
  ['2025-03-19', ['o1-pro']],
  ['2025-03-11', ['gpt-4o-search-preview', 'gpt-4o-mini-search-preview']],
  ['2025-01-31', ['o3-mini']],
  ['2024-11-20', ['gpt-4o']],
  ['2024-10-17', ['gpt-4o-audio-preview']],
  ['2024-09-12', ['o1-preview', 'o1-mini']],
  ['2024-07-18', ['gpt-4o-mini']],
]
for (const [date, models] of releaseGroups) for (const model of models) releases[model] = date

export function modelCategory(model: string): Exclude<ModelCategory, 'all'> {
  const id = model.toLowerCase()
  // Match transcription before realtime: the pricing page places live
  // translation / realtime-whisper in its transcription section.
  if (/transcrib|whisper|realtime-translate/.test(id)) return 'transcription'
  if (/cyber|daybreak/.test(id)) return 'cyber'
  if (/^(gpt-image-|chatgpt-image-|dall-e-)/.test(id)) return 'image'
  if (/realtime|audio|(?:^|-)tts(?:-|$)|^gpt-live-/.test(id)) return 'audio'
  if (/codex|chat-latest|chatgpt-|search|rosalind/.test(id)) return 'specialized'
  if (/^gpt-[456](?:[.-]|$)|^o[134](?:-|$)/.test(id)) return 'flagship'
  return 'other'
}

// Snapshot IDs verified on their official model pages. A private alias or an
// arbitrary date suffix is not evidence of a release, even if its date parses.
const snapshots = [
  'gpt-5-2025-08-07', 'gpt-5.2-2025-12-11', 'gpt-5.4-2026-03-05',
  'gpt-5.4-mini-2026-03-17',
  'gpt-4o-2024-05-13', 'gpt-4o-2024-08-06', 'gpt-4o-2024-11-20',
  'gpt-4o-mini-2024-07-18', 'o3-2025-04-16', 'o4-mini-2025-04-16',
  'gpt-realtime-2025-08-28',
]
for (const id of snapshots) releases[id] = id.slice(-10)

function releaseDate(model: string): string {
  return releases[model] || ''
}
export function sortOpenAIModels(models: string[]): string[] {
  return [...models].sort((a, b) => releaseDate(b).localeCompare(releaseDate(a)) || a.localeCompare(b, 'en', { numeric: true }))
}

/** Site requests lead; verified release dates provide a stable fallback. */
export function sortModelsByPopularity(models: string[], stats: { model: string; requests: number }[]): string[] {
  const counts = new Map(stats.filter(stat => Number.isFinite(stat.requests) && stat.requests > 0).map(stat => [stat.model, stat.requests]))
  return sortOpenAIModels(models).sort((a, b) => (counts.get(b) || 0) - (counts.get(a) || 0))
}
