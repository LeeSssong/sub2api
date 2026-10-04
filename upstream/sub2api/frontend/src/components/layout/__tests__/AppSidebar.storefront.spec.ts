import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import { useAppStore } from '@/stores/app'
import AppSidebar from '../AppSidebar.vue'

vi.mock('vue-i18n', async (original) => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/composables/useBatchImageAccess', () => ({
  useBatchImageAccess: () => ({ canUseBatchImage: false, refreshBatchImageAccess: async () => {} }),
}))

const storefront = {
  id: 'xingqiao-storefront', label: '云猫兑换码充值',
  url: 'https://catfk.com/shop/DLK8SNUJ', visibility: 'user' as const, sort_order: 90,
  icon_svg: '<svg viewBox="0 0 24 24"><rect x="3" y="5" width="18" height="14" /></svg>',
}
let wrapper: VueWrapper | undefined

afterEach(() => { wrapper?.unmount() })

async function setup(items = [storefront]) {
  const pinia = createPinia()
  const app = useAppStore(pinia)
  app.$patch({ publicSettingsLoaded: true, cachedPublicSettings: { custom_menu_items: items } })
  const router = createRouter({ history: createMemoryHistory(), routes: [
    { path: '/:pathMatch(.*)*', component: { template: '<div />' } },
  ] })
  await router.push('/dashboard')
  await router.isReady()
  wrapper = mount(AppSidebar, { global: {
    plugins: [pinia, router],
    stubs: { VersionBadge: true, LocaleSwitcher: true, ContactSupportDialog: true },
  } })
  return { app, router, sidebar: wrapper }
}

describe('regular user storefront menu', () => {
  it('restores the configured storefront after keys and navigates to its original embedded page', async () => {
    const { sidebar, router } = await setup([
      { ...storefront, id: 'xingqiao-support', label: '联系客服', url: 'md:support' },
      storefront,
    ])
    expect(sidebar.findAll('nav a').map(link => link.attributes('href'))).toEqual([
      '/dashboard', '/usage', '/keys', '/custom/xingqiao-storefront',
    ])
    const link = sidebar.get('a[href="/custom/xingqiao-storefront"]')
    expect(link.text()).toBe('云猫兑换码充值')
    expect(link.find('svg rect').exists()).toBe(true)
    expect(link.attributes('target')).toBeUndefined()
    await link.trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/custom/xingqiao-storefront')
    expect(link.classes()).toContain('sidebar-link-active')
  })

  it('follows loaded settings, role visibility and collapsed tooltip without inventing a menu', async () => {
    const { sidebar, app } = await setup([])
    expect(sidebar.find('a[href="/custom/xingqiao-storefront"]').exists()).toBe(false)
    app.$patch({ cachedPublicSettings: { custom_menu_items: [storefront] }, sidebarCollapsed: true })
    await nextTick()
    expect(sidebar.get('a[href="/custom/xingqiao-storefront"]').attributes('title')).toBe('云猫兑换码充值')
    app.$patch({ cachedPublicSettings: { custom_menu_items: [{ ...storefront, visibility: 'admin' }] } })
    await nextTick()
    expect(sidebar.find('a[href="/custom/xingqiao-storefront"]').exists()).toBe(false)
  })

  it('mounts intelligence tests only through configured user menus', async () => {
    const {sidebar,app}=await setup([])
    const item={...storefront,id:'intelligence-test',label:'智商检测',url:'/intelligence-test'}
    app.$patch({cachedPublicSettings:{custom_menu_items:[item],pelican_showcase_enabled:true}})
    await nextTick()
    expect(sidebar.get('a[href="/intelligence-test"]').text()).toBe('智商监测')
    expect(sidebar.find('a[href="/pelican-showcase"]').exists()).toBe(false)
    app.$patch({cachedPublicSettings:{custom_menu_items:[{...item,visibility:'admin'}]}})
    await nextTick();expect(sidebar.find('a[href="/intelligence-test"]').exists()).toBe(false)
  })
})
