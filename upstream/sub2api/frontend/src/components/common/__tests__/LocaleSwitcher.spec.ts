import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'

import LocaleSwitcher from '../LocaleSwitcher.vue'

const setLocale = vi.fn()
const localeState = { value: 'en' }

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    locale: localeState,
  }),
}))

vi.mock('@/i18n', () => ({
  setLocale: (...args: unknown[]) => setLocale(...args),
  availableLocales: [
    { code: 'en', name: 'English', flag: '🇺🇸' },
    { code: 'zh', name: '中文', flag: '🇨🇳' },
  ],
}))

vi.mock('@/components/icons/Icon.vue', () => ({
  default: { name: 'Icon', template: '<span class="icon" />' },
}))

describe('LocaleSwitcher', () => {
  afterEach(() => {
    setLocale.mockReset()
    localeState.value = 'en'
    document.body.innerHTML = ''
  })

  it('exposes the native English and Chinese options', async () => {
    const wrapper = mount(LocaleSwitcher, { attachTo: document.body })

    expect(wrapper.find('[data-testid="locale-switcher"]').exists()).toBe(true)
    await wrapper.get('button').trigger('click')

    const labels = wrapper.findAll('[role="option"]').map((node) => node.text().replace(/\s+/g, ' ').trim())
    expect(labels).toEqual(['🇺🇸English', '🇨🇳中文'])
    wrapper.unmount()
  })

  it('opens the menu upward when placement is top', async () => {
    const wrapper = mount(LocaleSwitcher, {
      attachTo: document.body,
      props: { placement: 'top' },
    })

    await wrapper.get('button').trigger('click')
    expect(wrapper.find('[role="listbox"]').classes()).toContain('bottom-full')
    wrapper.unmount()
  })

  it('switches to Chinese through the native control', async () => {
    setLocale.mockResolvedValue(undefined)
    const wrapper = mount(LocaleSwitcher, { attachTo: document.body })

    await wrapper.get('button').trigger('click')
    const chinese = wrapper.findAll('[role="option"]').at(1)
    await chinese?.trigger('click')

    expect(setLocale).toHaveBeenCalledWith('zh')
    wrapper.unmount()
  })
})
