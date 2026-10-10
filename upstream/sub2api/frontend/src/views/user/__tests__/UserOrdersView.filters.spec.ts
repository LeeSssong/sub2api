import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import UserOrdersView from '../UserOrdersView.vue'
import Select from '@/components/common/Select.vue'
import Pagination from '@/components/common/Pagination.vue'

const api = vi.hoisted(() => ({ getMyOrders: vi.fn(), getRefundEligibleProviders: vi.fn() }))
const app = vi.hoisted(() => ({ showError: vi.fn(), cachedPublicSettings: {} as { payment_enabled?: boolean } }))
const push = vi.hoisted(() => vi.fn())
vi.mock('@/api/payment', () => ({ paymentAPI: api }))
vi.mock('@/stores', () => ({ useAppStore: () => app }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key })
}))
enableAutoUnmount(afterEach)

beforeEach(() => {
  vi.clearAllMocks()
  app.cachedPublicSettings = {}
  api.getMyOrders.mockResolvedValue({ data: { items: [], total: 100 } })
  api.getRefundEligibleProviders.mockResolvedValue({ data: { provider_instance_ids: [] } })
})

async function openOrders() {
  const wrapper = mount(UserOrdersView, {
    global: { stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      OrderTable: true, BaseDialog: true, Icon: true, Pagination: true, teleport: true
    } }
  })
  await flushPromises()
  wrapper.getComponent(Pagination).vm.$emit('update:page', 4)
  await flushPromises()
  return wrapper
}

describe('order status filtering', () => {
  it.each([false, true, undefined])('returns to the available recharge entry when payment is enabled=%s', async (enabled) => {
    app.cachedPublicSettings = { payment_enabled: enabled }
    const wrapper = await openOrders()
    await wrapper.get('[data-testid="back-to-recharge"]').trigger('click')
    expect(push).toHaveBeenCalledWith(enabled === false ? '/redeem' : '/purchase')
  })

  it('loads the first page with the selected status', async () => {
    const wrapper = await openOrders()
    const select = wrapper.getComponent(Select)
    await select.get('button').trigger('click')
    await select.findAll('[role="option"]').find(option => option.text() === 'payment.status.pending')!.trigger('click')
    await flushPromises()
    expect(api.getMyOrders).toHaveBeenLastCalledWith({ page: 1, page_size: 20, status: 'PENDING' })
    expect(wrapper.getComponent(Pagination).props('page')).toBe(1)
    expect(api.getMyOrders).toHaveBeenCalledTimes(3)
  })

  it('keeps the current page on manual refresh', async () => {
    const wrapper = await openOrders()
    await wrapper.get('[title="common.refresh"]').trigger('click')
    await flushPromises()
    expect(api.getMyOrders).toHaveBeenLastCalledWith({ page: 4, page_size: 20, status: undefined })
  })
})
