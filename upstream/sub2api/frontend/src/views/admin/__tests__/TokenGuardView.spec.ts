import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { groupsAPI } from '@/api/admin/groups'
import TokenGuardView from '../ops/TokenGuardView.vue'
import { getTokenGuardStatus, saveTokenGuardConfig, runTokenGuard, reloginTokenGuardAccount } from '@/api/admin/accountTokenGuard'
vi.mock('@/api/admin/groups', () => ({ default: {getAll: vi.fn().mockResolvedValue([])}, groupsAPI: {getAll: vi.fn().mockResolvedValue([])}, getAll: vi.fn().mockResolvedValue([]) }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('@/components/admin/operations/SmartOpsNav.vue', () => ({ default: { template: '<nav />' } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/api/admin/accountTokenGuard', () => ({ getTokenGuardStatus: vi.fn(), saveTokenGuardConfig: vi.fn(), runTokenGuard: vi.fn(), reloginTokenGuardAccount: vi.fn() }))
const config = {
  mode: 'native' as const, enabled: false, group_ids: [], interval_seconds: 300, probe_endpoint: '', probe_model: 'gpt-6-astra',
  probe_headers: { Authorization: 'probe-key' }, probe_timeout_seconds: 30, probe_concurrency: 1,
  max_probe_per_cycle: 10, auto_relogin: false, relogin_endpoint: '', relogin_headers: { 'X-Key': 'relogin-key' },
  relogin_accounts: [{ account_id: 42, email: 'owner@example.com', password: ' original,password ', mfa_secret: 'JBSWY3DPEHPK3PXP' }],
  restore_schedulable: false, fail_streak_threshold: 3, bark_key: 'bark-key', notify_on_fix: false, notify_on_fail: false,
}
beforeEach(() => {
  vi.resetAllMocks()
  vi.mocked(groupsAPI.getAll).mockResolvedValue([])
  vi.mocked(getTokenGuardStatus).mockResolvedValue({ config: structuredClone(config), available_accounts: [{ account_id: 42, account_name: 'Renamed account', email: 'owner@example.com' }, { account_id: 43, account_name: 'Other', email: 'other@example.com' }], accounts: [], events: [], runtime: { running: false, last_run: null, last_message: '', stats: {} } } as any)
  vi.mocked(saveTokenGuardConfig).mockImplementation(async value => value)
})
describe('credential guard native configuration', () => {
  it('preserves plaintext credentials and stable ID on unrelated save without running', async () => {
    const wrapper = mount(TokenGuardView); await flushPromises()
    expect(wrapper.text()).toContain('tokenGuard.plaintextHint')
    expect(wrapper.text()).not.toContain('tokenGuard.probeEndpoint')
    expect(wrapper.find('input[type="password"]').exists()).toBe(false)
    expect((wrapper.vm as any).dirty).toBe(false)
    ;(wrapper.vm as any).draft.interval_seconds = 600
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(saveTokenGuardConfig).toHaveBeenCalledWith({ ...config, interval_seconds: 600 })
    expect(runTokenGuard).not.toHaveBeenCalled()
    expect(reloginTokenGuardAccount).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('round trips passwords with whitespace and commas through editable inputs', async () => {
    const wrapper = mount(TokenGuardView); await flushPromises()
    const row = wrapper.get('[data-testid="credential-row"]')
    await row.findAll('input')[1].setValue('  changed,p,a,s,s  ')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(saveTokenGuardConfig).toHaveBeenCalledWith(expect.objectContaining({ relogin_accounts: [{ ...config.relogin_accounts[0], password: '  changed,p,a,s,s  ' }] }))
    wrapper.unmount()
  })
  it('requires account binding and every credential, prefills email, and supports removal', async () => {
    const wrapper = mount(TokenGuardView); await flushPromises()
    await wrapper.get('[data-testid="add-credential"]').trigger('click')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(saveTokenGuardConfig).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('tokenGuard.credentialsRequired')
    const row = wrapper.findAll('[data-testid="credential-row"]')[1]
    await row.get('select').setValue(43)
    expect((row.findAll('input')[0].element as HTMLInputElement).value).toBe('other@example.com')
    await row.findAll('input')[1].setValue('password')
    await wrapper.get('form').trigger('submit')
    expect(saveTokenGuardConfig).not.toHaveBeenCalled()
    await row.findAll('input')[2].setValue('JBSWY3DPEHPK3PXP')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(saveTokenGuardConfig).toHaveBeenCalledOnce()
    await row.get('button').trigger('click')
    expect(wrapper.findAll('[data-testid="credential-row"]')).toHaveLength(1)
    wrapper.unmount()
  })
  it('requires explicit binding for legacy ID zero and retains external settings', async () => {
    const wrapper = mount(TokenGuardView); await flushPromises()
    ;(wrapper.vm as any).draft.relogin_accounts[0].account_id = 0
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(saveTokenGuardConfig).not.toHaveBeenCalled()
    await wrapper.get('[data-testid="guard-mode"]').setValue('external')
    expect(wrapper.text()).toContain('tokenGuard.probeEndpoint')
    expect(wrapper.text()).toContain('tokenGuard.reloginHeaders')
    ;(wrapper.vm as any).draft.relogin_accounts[0].account_id = 42
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(saveTokenGuardConfig).toHaveBeenCalledWith({ ...config, mode: 'external' })
    wrapper.unmount()
  })
})

it('saves email notifications without requiring a Bark key', async () => {
 const wrapper = mount(TokenGuardView); await flushPromises()
 await wrapper.get('[data-testid="guard-email-enabled"]').setValue(true)
 await wrapper.get('[data-testid="guard-email-recipient"]').setValue('ops@example.com')
 ;(wrapper.vm as any).draft.bark_key = ''
 await wrapper.get('form').trigger('submit'); await flushPromises()
 expect(saveTokenGuardConfig).toHaveBeenCalledWith(expect.objectContaining({email_enabled: true, email_recipient: 'ops@example.com', bark_key: ''}))
 expect(runTokenGuard).not.toHaveBeenCalled()
 wrapper.unmount()
})
