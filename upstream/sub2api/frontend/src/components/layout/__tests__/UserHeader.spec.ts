import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import UserHeader from '../UserHeader.vue'
const state = vi.hoisted(() => ({
  app: { docUrl: '', cachedPublicSettings: { custom_menu_items: [{ id: 'guide', label: '接入指南' }] } },
  route: { name: 'Keys', params: { id: '' }, meta: { titleKey: 'keys.title', descriptionKey: 'keys.description' } }
}))
vi.mock('@/components/common/AnnouncementBell.vue', () => ({ default: { name: 'AnnouncementBell', template: '<button />' } }))
vi.mock('@/components/common/LocaleSwitcher.vue', () => ({ default: { name: 'LocaleSwitcher', template: '<button />' } }))
vi.mock('@/stores', () => ({ useAppStore: () => state.app }))
vi.mock('vue-router', () => ({ useRoute: () => state.route }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
const render = () => mount(UserHeader, { global: { stubs: { AnnouncementBell: true, LocaleSwitcher: true, Icon: true } } })
describe('UserHeader', () => {
  it('uses only the page title and keeps announcement and language controls', () => {
    const wrapper = render()
    expect(wrapper.get('h1').text()).toBe('keys.title')
    expect(wrapper.text()).not.toContain('keys.description')
    expect(wrapper.find('announcement-bell-stub').exists()).toBe(true)
    expect(wrapper.find('locale-switcher-stub').exists()).toBe(true)
    wrapper.unmount()
  })
  it('disables missing or unsafe docs links', () => {
    for (const url of ['', 'javascript:alert(1)']) {
      state.app.docUrl = url
      const wrapper = render()
      expect(wrapper.find('a').exists()).toBe(false)
      expect(wrapper.get('button.user-doc-link').attributes('disabled')).toBeDefined()
      wrapper.unmount()
    }
  })
  it('opens configured docs safely and uses custom page labels', () => {
    state.app.docUrl = 'https://example.com/docs'
    state.route.name = 'CustomPage'
    state.route.params.id = 'guide'
    const wrapper = render()
    expect(wrapper.get('h1').text()).toBe('接入指南')
    expect(wrapper.get('a').attributes('href')).toBe(state.app.docUrl)
    expect(wrapper.get('a').attributes('rel')).toContain('noopener')
    wrapper.unmount()
  })
})
