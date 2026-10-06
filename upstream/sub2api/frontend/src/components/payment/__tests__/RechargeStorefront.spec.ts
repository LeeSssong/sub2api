import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import RechargeStorefront from '../RechargeStorefront.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const shopUrl = 'https://catfk.com/shop/DLK8SNUJ'
const viewportSelector = '[data-test="storefront-viewport"]'
const toggleSelector = '[data-test="storefront-view-toggle"]'
const render = (url = shopUrl) => mount(RechargeStorefront, { props: { url } })

describe('RechargeStorefront product positioning', () => {
  it('initially positions the verified shop at products without changing its URL or external links', async () => {
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get(viewportSelector).attributes('data-view')).toBe('products')
    expect(wrapper.get(viewportSelector).element.scrollTop).toBe(400)
    expect(wrapper.get('iframe').attributes('src')).toBe(shopUrl)
    const link = wrapper.get('[data-test="storefront-open"]')
    expect(link.attributes('href')).toBe(shopUrl)
    expect(link.attributes('rel')).toBe('noopener noreferrer')
    expect(wrapper.get(toggleSelector).text()).toContain('redeem.storefrontShowFull')
    wrapper.unmount()
  })

  it.each([
    'https://example.com/shop',
    'https://catfk.com/shop/OTHER',
    'https://catfk.com/item/l1s3u1',
    'https://catfk.com.evil.example/shop/DLK8SNUJ',
    'http://catfk.com/shop/DLK8SNUJ',
  ])('leaves an unverified entry at the top: %s', async url => {
    const wrapper = render(url)
    await flushPromises()
    expect(wrapper.get(viewportSelector).attributes('data-view')).toBe('full')
    expect(wrapper.get(viewportSelector).element.scrollTop).toBe(0)
    expect(wrapper.find(toggleSelector).exists()).toBe(false)
    wrapper.unmount()
  })

  it('can show the full shop and return to products while preserving the same iframe', async () => {
    const wrapper = render()
    await flushPromises()
    const frame = wrapper.get('iframe').element
    await wrapper.get(toggleSelector).trigger('click')
    await flushPromises()
    expect(wrapper.get(viewportSelector).attributes('data-view')).toBe('full')
    expect(wrapper.get(viewportSelector).element.scrollTop).toBe(0)
    expect(wrapper.get(toggleSelector).text()).toContain('redeem.storefrontShowProducts')
    await wrapper.get(toggleSelector).trigger('click')
    await flushPromises()
    expect(wrapper.get(viewportSelector).element.scrollTop).toBe(400)
    expect(wrapper.get('iframe').element).toBe(frame)
    wrapper.unmount()
  })

  it('preserves a full-shop choice made before the first load finishes', async () => {
    const wrapper = render()
    await wrapper.get(toggleSelector).trigger('click')
    await wrapper.get('iframe').trigger('load')
    await flushPromises()
    expect(wrapper.get(viewportSelector).attributes('data-view')).toBe('full')
    expect(wrapper.get(viewportSelector).element.scrollTop).toBe(0)
    expect(wrapper.find(toggleSelector).exists()).toBe(true)
    wrapper.unmount()
  })

  it('positions the initial load but restores the full viewport on subsequent navigation', async () => {
    const wrapper = render()
    await wrapper.get('iframe').trigger('load')
    await flushPromises()
    expect(wrapper.get(viewportSelector).element.scrollTop).toBe(400)
    await wrapper.get('iframe').trigger('load')
    await flushPromises()
    expect(wrapper.get(viewportSelector).attributes('data-view')).toBe('full')
    expect(wrapper.get(viewportSelector).element.scrollTop).toBe(0)
    expect(wrapper.find(toggleSelector).exists()).toBe(false)
    wrapper.unmount()
  })

  it('resets positioning and navigation tracking when the configured URL changes', async () => {
    const wrapper = render()
    await wrapper.get('iframe').trigger('load')
    await wrapper.setProps({ url: 'https://example.com/shop' })
    await wrapper.get('iframe').trigger('load')
    await flushPromises()
    expect(wrapper.get(viewportSelector).attributes('data-view')).toBe('full')
    await wrapper.setProps({ url: shopUrl })
    await wrapper.get('iframe').trigger('load')
    await flushPromises()
    expect(wrapper.get(viewportSelector).element.scrollTop).toBe(400)
    wrapper.unmount()
  })
})
