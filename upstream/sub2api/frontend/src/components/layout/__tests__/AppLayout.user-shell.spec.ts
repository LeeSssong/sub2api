import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AppLayout from '../AppLayout.vue'

const state = vi.hoisted(() => ({ user: { role: 'user' } }))
vi.mock('@/stores', () => ({ useAppStore: () => ({ sidebarCollapsed: false }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => state }))
vi.mock('@/stores/onboarding', () => ({ useOnboardingStore: () => ({ setReplayCallback: vi.fn() }) }))
vi.mock('@/composables/useOnboardingTour', () => ({ useOnboardingTour: () => ({ replayTour: vi.fn() }) }))

describe('AppLayout regular user shell', () => {
  it('renders the fixed user header and the admin header in their respective roles', () => {
    for (const role of ['user', 'admin']) {
      state.user.role = role
      const wrapper = mount(AppLayout, { global: { stubs: { AppSidebar: true, AppHeader: true, UserHeader: true } } })
      expect(wrapper.find('app-header-stub').exists()).toBe(role === 'admin')
      expect(wrapper.find('user-header-stub').exists()).toBe(role === 'user')
      wrapper.unmount()
    }
  })
})
