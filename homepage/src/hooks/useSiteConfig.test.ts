import { cleanup, renderHook, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useSiteConfig } from './useSiteConfig'

afterEach(() => {
  cleanup()
  document.querySelectorAll('link[rel="icon"], link[rel="shortcut icon"]').forEach((link) => link.remove())
  vi.unstubAllGlobals()
})

describe('useSiteConfig branding', () => {
  it('keeps the server favicon while public settings are loading', () => {
    document.head.innerHTML = '<link rel="icon" href="/favicon.ico">'
    vi.stubGlobal('fetch', vi.fn(() => new Promise<Response>(() => {})))

    renderHook(() => useSiteConfig())

    expect(document.querySelector('link[rel="icon"]')).toHaveAttribute('href', '/favicon.ico')
  })

  it.each([
    ['data:image/png;base64,aGVsbG8=', 'image/png'],
    ['/uploads/brand.svg', 'image/svg+xml'],
    ['/uploads/brand', null],
  ])('updates the favicon from public settings with the correct MIME for %s', async (siteLogo, mime) => {
    document.head.insertAdjacentHTML('beforeend', '<link rel="icon" type="image/webp" href="/old.webp"><link rel="shortcut icon" href="/old.ico">')
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => Response.json(
      new URL(String(input)).pathname === '/api/v1/settings/public'
        ? { code: 0, data: { site_logo: siteLogo } }
        : { version: 1 },
    )))

    renderHook(() => useSiteConfig())

    const expectedHref = siteLogo.startsWith('data:') ? '/branding/favicon/f7154e57' : siteLogo
    await waitFor(() => expect(document.querySelector('link[rel="icon"]')).toHaveAttribute('href', expectedHref))
    expect(document.querySelectorAll('link[rel="icon"], link[rel="shortcut icon"]')).toHaveLength(1)
    expect(document.querySelector('link[rel="icon"]')?.getAttribute('type')).toBe(mime)
  })

  it('restores the default favicon when the public logo is cleared', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => Response.json({ code: 0, data: { site_logo: '' } })))
    document.head.insertAdjacentHTML('beforeend', '<link rel="icon" href="/old.png">')

    renderHook(() => useSiteConfig())

    await waitFor(() => expect(document.querySelector('link[rel="icon"]')).toHaveAttribute('href', '/home-assets/xingqiao-logo-256-v1.webp'))
    expect(document.querySelector('link[rel="icon"]')).toHaveAttribute('type', 'image/webp')
  })
})
