import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { h } from 'vue'
import { useAuthStore } from '@/stores/auth'
import BaseDialog from '@/components/common/BaseDialog.vue'
import AppLayout from '../AppLayout.vue'

vi.mock('@/composables/useOnboardingTour', () => ({ useOnboardingTour: () => ({ replayTour: vi.fn() }) }))

let wrapper: VueWrapper | undefined
afterEach(() => { wrapper?.unmount() })

async function setup(role: 'user' | 'admin', path: string) {
  const pinia = createPinia()
  useAuthStore(pinia).$patch({ user: { id: 7, role } })
  const router = createRouter({ history: createMemoryHistory(), routes: [
    { path: '/:pathMatch(.*)*', component: { template: '<div />' } },
  ] })
  await router.push(path)
  wrapper = mount(AppLayout, {
    attachTo: document.body,
    slots: { default: () => h(BaseDialog, { show: true, title: '线路详情' }) },
    global: { plugins: [pinia, router], stubs: { AppSidebar: true, AppHeader: true, UserHeader: true } },
  })
  await flushPromises()
  return { layout: wrapper, router }
}

describe('AppLayout workspace surface', () => {
  it.each(['user', 'admin'] as const)('uses the same user page and dialog theme for %s', async role => {
    const { layout } = await setup(role, '/keys')
    expect(layout.classes()).toContain('user-app-shell')
    expect(layout.get('main').classes()).toContain('user-workspace')
    expect(layout.find('.admin-main-frame').exists()).toBe(false)
    expect(document.querySelector('[role="dialog"]')?.classList.contains('xq-dialog')).toBe(true)
  })

  it('switches the page and teleported dialog theme when an admin moves between workspaces', async () => {
    const { layout, router } = await setup('admin', '/admin/dashboard')
    expect(layout.classes()).toContain('brand-admin-shell')
    expect(layout.get('main').classes()).toContain('admin-workspace')
    expect(document.querySelector('[role="dialog"]')?.classList.contains('xq-dialog')).toBe(false)
    await router.push('/profile')
    await flushPromises()
    expect(layout.classes()).toContain('user-app-shell')
    expect(layout.get('main').classes()).toContain('user-workspace')
    expect(document.querySelector('[role="dialog"]')?.classList.contains('xq-dialog')).toBe(true)
    await router.push('/admin/accounts')
    await flushPromises()
    expect(layout.classes()).toContain('brand-admin-shell')
    expect(document.querySelector('[role="dialog"]')?.classList.contains('xq-dialog')).toBe(false)
  })
})
