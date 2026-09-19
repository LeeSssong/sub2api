import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'

const source = readFileSync('src/components/layout/AppSidebar.vue', 'utf8')

describe('performance monitor navigation contract', () => {
  it('uses the custom page path and removes the legacy fixed monitor path', () => {
    expect(source).toContain("t('nav.performanceMonitor')")
    expect(source).toContain("id: 'performance-monitor'")
    expect(source).toContain('PerformanceMonitorIcon')
    expect(source).toContain("item.id === 'performance-monitor' ? PerformanceMonitorIcon : null")
  })

  it('does not expose the retired scheduler log page', () => {
    expect(source).not.toContain("path: '/admin/scheduler-logs'")
    expect(source).toContain("path: '/admin/settings', label: t('nav.settings'), icon: CogIcon")
  })
})
