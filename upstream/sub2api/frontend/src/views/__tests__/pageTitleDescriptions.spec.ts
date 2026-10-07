import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = (path: string) => readFileSync(`src/${path}`, 'utf8')

describe('page title descriptions', () => {
  it('renders shared user and app headers with the title alone', () => {
    expect(source('components/user/UserPageHeader.vue')).not.toContain('description')
    expect(source('components/layout/AppHeader.vue')).not.toContain('pageDescription')

    for (const page of ['UsageView', 'KeysView', 'PaymentView', 'RedeemView', 'UserOrdersView', 'ProfileView']) {
      expect(source(`views/user/${page}.vue`)).not.toMatch(/<UserPageHeader[^>]*description=/)
    }
  })

  it('omits explanatory text immediately below independent page titles', () => {
    const descriptions = [
      ['views/admin/AccountMonitorView.vue', "t('admin.accountMonitor.description')"],
      ['views/admin/AccountProfitabilityView.vue', "t('admin.accountProfitability.description')"],
      ['views/admin/BusinessOverviewView.vue', '按用户实际扣费与上游实际成本查看站内经营结果。'],
      ['views/admin/RiskControlView.vue', "t('admin.riskControl.description')"],
      ['views/admin/SchedulerLogsView.vue', "t('admin.schedulerLogs.description')"],
      ['views/admin/ChannelMonitorView.vue', "t('channelMonitorV2.admin.descriptionV1')"],
      ['features/channel-monitor-v2/MonitorSettingsPanel.vue', "t('channelMonitorV2.settings.description')"],
      ['views/admin/PluginsView.vue', 't("admin.plugins.description")'],
      ['views/setup/SetupWizardView.vue', "t('setup.description')"],
      ['features/prompt-audit/PromptAuditView.vue', "t('admin.promptAudit.description')"],
      ['components/modelPlaza/ModelPlazaContent.vue', "t('modelPlaza.description')"],
      ['views/KeyUsageView.vue', "t('keyUsage.subtitle')"],
    ] as const

    for (const [path, description] of descriptions) {
      expect(source(path), path).not.toContain(description)
    }
  })

  it('retains operational status and financial units', () => {
    expect(source('views/user/ChannelStatusV2View.vue')).toContain('channelMonitorV2.updatedTo')
    expect(source('features/monitor-v2/MonitorV2View.vue')).toContain('monitorV2.updatedAt')
    expect(source('views/admin/BusinessOverviewView.vue')).toContain('经营口径 CNY / ¥')
  })
})
