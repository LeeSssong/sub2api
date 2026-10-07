import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'

const source = readFileSync('src/components/layout/AppSidebar.vue', 'utf8')

describe('performance monitor navigation contract', () => {
  it('does not restore the legacy fixed monitor path', () => {
    expect(source).not.toContain("path: '/monitor', label: t('nav.channelStatus')")
  })

  it('does not expose the retired scheduler log page', () => {
    expect(source).not.toContain("path: '/admin/scheduler-logs'")
    expect(source).toContain("path: '/admin/settings', label: t('nav.settings'), icon: CogIcon")
  })
})
