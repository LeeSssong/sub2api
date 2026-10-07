import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import type { AdminGroup } from '@/types'
import GroupRateMultipliersModal from './GroupRateMultipliersModal.vue'

const { getGroupRateMultipliers } = vi.hoisted(() => ({
  getGroupRateMultipliers: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: { groups: { getGroupRateMultipliers } },
}))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }),
}))
vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}))

describe('GroupRateMultipliersModal', () => {
  it('labels the final rate preview without changing the editable multiplier', async () => {
    getGroupRateMultipliers.mockResolvedValue([{
      user_id: 1,
      user_name: 'Test',
      user_email: 'test@example.invalid',
      user_notes: '',
      user_status: 'active',
      rate_multiplier: 0.12,
      rpm_override: null,
    }])
    const wrapper = mount(GroupRateMultipliersModal, {
      props: { show: false, group: { id: 1, name: 'Test', platform: 'openai', rate_multiplier: 0.5 } as AdminGroup },
      global: { stubs: {
        BaseDialog: { template: '<div><slot /></div>' },
        PlatformIcon: true,
        Icon: true,
        Pagination: true,
      } },
    })
    await wrapper.setProps({ show: true })
    await flushPromises()
    const batchInput = wrapper.findAll('input[type="number"]')[1]
    await batchInput.setValue('0.5')

    expect(wrapper.get('tbody tr td:nth-child(7)').text()).toBe('0.06x倍率')
    expect(wrapper.get<HTMLInputElement>('tbody tr input[type="number"]').element.value).toBe('0.12')
    wrapper.unmount()
  })
})
