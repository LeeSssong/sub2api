import type { Group } from '@/types'
import type { MonitorV4Group } from '@/features/monitor-v4/types'
import { metricLabel, toolIdsForGroup, routeHealth, routeHealthTone, routeSuccessLabel } from '@/features/ai-tools/model'
import { successRateTone } from '@/features/monitor-v4/successRate'
import { formatMultiplierLabel } from '@/utils/formatters'

export interface LineOption {
  [key: string]: unknown
  value: number
  label: string
  group: Group
  description: string | null
  platform: Group['platform']
  rate: number | null
  rateLabel: string
  linkedCount: number | null
  healthKind: string
  statusLabel: string
  successLabel: string
  successTone: ReturnType<typeof successRateTone>
  ttftLabel: string
}

export function resolveLineRate(group: Group, rates: Record<number, number>): number | null {
  const userRate = rates[group.id]
  const rate = Number.isFinite(userRate) ? userRate : group.rate_multiplier
  return Number.isFinite(rate) ? rate : null
}

export function formatLineRate(rate: number | null): string {
  return formatMultiplierLabel(rate)
}

export function buildLineOptions(
  groups: Group[],
  rates: Record<number, number>,
  metrics: Map<number, MonitorV4Group>,
  linkedCounts: Map<number, number> | null,
  toolId?: string,
  metricsGeneratedAt?: string | null,
  now = Date.now()
): LineOption[] {
  return groups
    .filter(group => !toolId || (group.status === 'active' && toolIdsForGroup(group, metrics.get(group.id)).includes(toolId)))
    .map(group => {
      const rate = resolveLineRate(group, rates)
      const metric = metrics.get(group.id)
      const health = routeHealth(metric, now, metricsGeneratedAt)
      return {
        value: group.id,
        label: group.name,
        group,
        description: group.description,
        platform: group.platform,
        rate,
        rateLabel: formatLineRate(rate),
        linkedCount: linkedCounts?.get(group.id) ?? (linkedCounts ? 0 : null),
        healthKind: health.kind,
        statusLabel: health.text,
        successLabel: routeSuccessLabel(health),
        successTone: routeHealthTone(health),
        ttftLabel: metricLabel(metric?.ttft_p50_ms)
      }
    })
}
