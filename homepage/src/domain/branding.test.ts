import { describe, expect, it } from 'vitest'
import { updateFavicon } from './branding'

describe('uploaded Safari favicon', () => {
  it('re-registers an icon already present in the document', () => {
    document.head.innerHTML = '<link rel="icon" href="/branding/favicon/02039e14">'
    const previous = document.querySelector('link[rel="icon"]')
    updateFavicon('data:image/png;base64,AA==')
    expect(previous?.isConnected).toBe(false)
    expect(document.querySelector('link[rel="icon"]')?.getAttribute('href')).toBe('/branding/favicon/02039e14')
  })
  it('uses a versioned HTTP image URL while preserving the original MIME', () => {
    document.head.innerHTML = '<link rel="icon" href="/old.ico">'
    updateFavicon('data:image/png;base64,AA==')
    const icon = document.querySelector('link[rel="icon"]')
    expect(icon?.getAttribute('href')).toBe('/branding/favicon/02039e14')
    expect(icon?.getAttribute('type')).toBe('image/png')
  })
})
