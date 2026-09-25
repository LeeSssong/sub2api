import type { PelicanShowcaseStats } from '@/api/pelicanShowcase'

export function pelicanStatsCount(stats?: PelicanShowcaseStats | null): string {
  if (!stats) return '— / —'
  return `${stats.success_count.toLocaleString()} / ${stats.total_count.toLocaleString()}`
}

export function pelicanStatsRate(stats?: PelicanShowcaseStats | null): string {
  if (!stats || stats.total_count === 0 || stats.success_rate === null || !Number.isFinite(stats.success_rate)) return '—'
  return `${stats.success_rate.toFixed(1)}%`
}
