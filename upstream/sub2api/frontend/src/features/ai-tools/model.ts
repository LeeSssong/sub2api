import type { ApiKey, Group } from '@/types'
import type { MonitorV4Group } from '@/features/monitor-v4/types'
export const tools = [
  { id: 'codex', label: 'Codex', platform: 'openai', type: 'AI 编程' },
  { id: 'claude', label: 'Claude Code', platform: 'anthropic', type: 'AI 编程' },
  { id: 'grok', label: 'Grok', platform: 'grok', type: '通用 AI' },
  { id: 'deepseek', label: 'DeepSeek', platform: 'deepseek', type: '通用 AI' },
] as const
export type Tool = typeof tools[number]
const SNAPSHOT_FRESHNESS_MS = 7 * 60 * 1000
export function toolIdsForGroup(group: Group, metric?: MonitorV4Group): string[] {
  if (group.tool_ids?.length) return group.tool_ids
  if (group.tool_ids === undefined && metric?.tool_ids?.length) return metric.tool_ids
  const fallback = tools.find(tool => tool.platform === group.platform)
  return fallback ? [fallback.id] : []
}
export function linkedCounts(keys: ApiKey[]): Map<number, number> {
  const counts = new Map<number,number>()
  for (const key of keys) if (key.group_id) counts.set(key.group_id,(counts.get(key.group_id)||0)+1)
  return counts
}
export function configuredLines(groups: Group[], keys: ApiKey[]): Group[] {
  const byID = new Map(groups.map(group=>[group.id,group]))
  for (const key of keys) if (key.group && !byID.has(key.group.id)) byID.set(key.group.id,key.group)
  const counts = linkedCounts(keys)
  return [...byID.values()].filter(group=>counts.has(group.id))
}
export type RouteHealth = { kind: 'success' | 'warning' | 'danger' | 'muted'; text: string; rate: number | null }
const noRouteData = (): RouteHealth => ({kind:'muted',text:'暂无数据',rate:null})
function freshSnapshot(now: number, generatedAt?: string | null): boolean {
  if (generatedAt === undefined) return true
  const observed = generatedAt ? Date.parse(generatedAt) : NaN
  return Number.isFinite(observed) && observed <= now && now - observed <= SNAPSHOT_FRESHNESS_MS
}
function validRealCounts(metric?: MonitorV4Group): metric is MonitorV4Group {
  return !!metric && Number.isSafeInteger(metric.real_request_count) && metric.real_request_count >= 0
    && Number.isSafeInteger(metric.real_success_count) && metric.real_success_count >= 0 && metric.real_success_count <= metric.real_request_count
}
export function routeHealth(metric?: MonitorV4Group, now = Date.now(), snapshotGeneratedAt?: string | null): RouteHealth {
  if (!freshSnapshot(now,snapshotGeneratedAt) || !validRealCounts(metric) || metric.real_request_count === 0) return noRouteData()
  const rate = metric.real_success_count / metric.real_request_count * 100
  return rate >= 90 ? {kind:'success',text:'正常运行',rate} : rate >= 70 ? {kind:'warning',text:'波动',rate} : {kind:'danger',text:'异常',rate}
}
export function aggregateRouteHealth(metrics: (MonitorV4Group | undefined)[], now = Date.now(), snapshotGeneratedAt?: string | null): RouteHealth {
  if (!freshSnapshot(now,snapshotGeneratedAt) || metrics.some(m=>!validRealCounts(m))) return noRouteData()
  const real_request_count = metrics.reduce((sum,m)=>sum+(m?.real_request_count ?? 0),0)
  const real_success_count = metrics.reduce((sum,m)=>sum+(m?.real_success_count ?? 0),0)
  return routeHealth({real_request_count,real_success_count} as MonitorV4Group)
}
// Keep a value below a threshold visibly below it (89.99 must not display as 90%).
export function routeSuccessLabel(health: RouteHealth): string {
  if (health.rate === null) return '暂无数据'
  const rounded = Number(health.rate.toFixed(2))
  const displayed = health.rate < 70 && rounded >= 70 ? 69.99 : health.rate < 90 && rounded >= 90 ? 89.99 : rounded
  return `${displayed}%`
}
export function routeHealthTone(health: RouteHealth): 'green' | 'amber' | 'red' | 'muted' {
  return {success:'green',warning:'amber',danger:'red',muted:'muted'}[health.kind] as 'green' | 'amber' | 'red' | 'muted'
}
export function compareQuality(a: Group,b: Group, metrics: Map<number,MonitorV4Group>, rates: Record<number,number>,now=Date.now(),snapshotGeneratedAt?:string|null): number {
  const am=metrics.get(a.id),bm=metrics.get(b.id)
  return (routeHealth(bm,now,snapshotGeneratedAt).rate??-1)-(routeHealth(am,now,snapshotGeneratedAt).rate??-1)
    || (bm?.real_request_count??0)-(am?.real_request_count??0)
    || (am?.ttft_p50_ms??Infinity)-(bm?.ttft_p50_ms??Infinity)
    || (am?.latency_p50_ms??Infinity)-(bm?.latency_p50_ms??Infinity)
    || (rates[a.id]??a.rate_multiplier)-(rates[b.id]??b.rate_multiplier) || a.id-b.id
}
export interface WeightedRouteRanking {
  group: Group
  score: number
  successRate: number
  cacheHitRate: number
  ttftScore: number
  ttftP50Ms: number
}
function weightedMetric(metric: MonitorV4Group | undefined, now: number, snapshotGeneratedAt?: string | null): metric is MonitorV4Group & { cache_hit_rate: number; ttft_p50_ms: number } {
  return freshSnapshot(now, snapshotGeneratedAt)
    && validRealCounts(metric)
    && metric.real_request_count > 0
    && typeof metric.cache_hit_rate === 'number'
    && Number.isFinite(metric.cache_hit_rate)
    && metric.cache_hit_rate >= 0
    && metric.cache_hit_rate <= 1
    && typeof metric.ttft_p50_ms === 'number'
    && Number.isFinite(metric.ttft_p50_ms)
    && metric.ttft_p50_ms >= 0
}
export function rankWeightedRoutes(
  groups: Group[],
  metrics: Map<number, MonitorV4Group>,
  now = Date.now(),
  snapshotGeneratedAt?: string | null,
): WeightedRouteRanking[] {
  const candidates = groups
    .filter(group => group.status === 'active')
    .flatMap(group => {
      const metric = metrics.get(group.id)
      return weightedMetric(metric, now, snapshotGeneratedAt) ? [{ group, metric }] : []
    })
  if (!candidates.length) return []
  const ttfts = candidates.map(item => item.metric.ttft_p50_ms)
  const fastest = Math.min(...ttfts)
  const slowest = Math.max(...ttfts)
  const range = slowest - fastest
  return candidates.map(({ group, metric }) => {
    const successRate = metric.real_success_count / metric.real_request_count * 100
    const cacheHitRate = metric.cache_hit_rate * 100
    const ttftScore = range === 0 ? 100 : (slowest - metric.ttft_p50_ms) / range * 100
    return {
      group,
      score: successRate * 0.5 + cacheHitRate * 0.3 + ttftScore * 0.2,
      successRate,
      cacheHitRate,
      ttftScore,
      ttftP50Ms: metric.ttft_p50_ms,
    }
  }).sort((a, b) => b.score - a.score || b.successRate - a.successRate || b.cacheHitRate - a.cacheHitRate || a.ttftP50Ms - b.ttftP50Ms || a.group.id - b.group.id)
}
export const metricLabel = (ms?: number|null) => ms == null ? '—' : `${(ms/1000).toFixed(2)}s`
export const providerIcon = (platform: string) => `/xingqiao/providers/${platform === 'grok' ? 'xai' : platform}.svg`
export const platformLabel = (platform: string) => ({openai:'OpenAI',anthropic:'Anthropic',grok:'xAI',deepseek:'DeepSeek'}[platform]||platform)
