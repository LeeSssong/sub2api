import { describe, expect, it, vi } from 'vitest'
import { useAppSurface } from '../useAppSurface'

const state = vi.hoisted(() => ({
  route: { path: '/keys', meta: {} as Record<string, unknown>, params: {} as Record<string, string> },
  auth: { isObserver: false },
  app: { cachedPublicSettings: { custom_menu_items: [] as { id: string; visibility: string }[] } },
  admin: { customMenuItems: [] as { id: string; visibility: string }[] },
}))
vi.mock('vue-router', () => ({ useRoute: () => state.route }))
vi.mock('@/stores', () => ({
  useAuthStore: () => state.auth,
  useAppStore: () => state.app,
  useAdminSettingsStore: () => state.admin,
}))

describe('application surface', () => {
  it.each(['/dashboard', '/keys', '/usage', '/profile', '/purchase', '/redeem', '/orders', '/subscriptions', '/available-channels', '/intelligence-test', '/custom/performance-monitor'])('uses the user surface for %s', path => {
    state.route = { path, meta: {}, params: {} }
    state.auth.isObserver = false
    expect(useAppSurface().isUserSurface.value).toBe(true)
  })

  it.each(['/admin/dashboard', '/admin/accounts', '/admin/usage'])('uses the admin surface for %s', path => {
    state.route = { path, meta: {}, params: {} }
    expect(useAppSurface().isUserSurface.value).toBe(false)
  })

  it('preserves the observer usage management surface', () => {
    state.route = { path: '/usage', meta: {}, params: {} }
    state.auth.isObserver = true
    expect(useAppSurface().isUserSurface.value).toBe(false)
    state.auth.isObserver = false
  })

  it('preserves admin-only custom pages outside the admin prefix', () => {
    state.route = { path: '/custom/operations', meta: {}, params: { id: 'operations' } }
    state.admin.customMenuItems = [{ id: 'operations', visibility: 'admin' }]
    expect(useAppSurface().isUserSurface.value).toBe(false)
    state.admin.customMenuItems = []
  })
})
