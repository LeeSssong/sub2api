export const DEFAULT_SITE_LOGO = '/home-assets/xingqiao-logo-256-v1.webp'

export function resolveSiteLogo(value: unknown): string {
  const logo = typeof value === 'string' ? value.trim() : ''
  if (logo.startsWith('/') && !logo.startsWith('//') && !logo.includes('\\')) return logo
  if (/^data:image\/[a-z0-9.+-]+(?:;[^,]*)?,/i.test(logo)) return logo
  if (/^https?:\/\//i.test(logo)) {
    try {
      return new URL(logo).href
    } catch { /* Use the bundled logo for invalid URLs. */ }
  }
  return DEFAULT_SITE_LOGO
}

export function updateFavicon(logo: string): void {
  const links = Array.from(document.querySelectorAll<HTMLLinkElement>('link[rel="icon"], link[rel="shortcut icon"]'))
  const link = links.shift() ?? document.createElement('link')
  links.forEach((duplicate) => duplicate.remove())
  link.rel = 'icon'
  const dataMime = logo.match(/^data:(image\/[a-z0-9.+-]+)[;,]/i)?.[1]
  const extension = new URL(logo, document.baseURI).pathname.split('.').pop()?.toLowerCase()
  const mimeByExtension: Record<string, string> = {
    svg: 'image/svg+xml', png: 'image/png', jpg: 'image/jpeg', jpeg: 'image/jpeg',
    ico: 'image/x-icon', webp: 'image/webp', gif: 'image/gif',
  }
  const mime = dataMime || mimeByExtension[extension || '']
  if (mime) link.type = mime
  else link.removeAttribute('type')
  let href = logo
  if (/^data:image\//i.test(logo)) {
    let hash = 2166136261
    for (const byte of new TextEncoder().encode(logo)) hash = Math.imul(hash ^ byte, 16777619)
    href = `/branding/favicon/${(hash >>> 0).toString(16).padStart(8, '0')}`
  }
  link.setAttribute('href', href)
  if (!link.isConnected) document.head.appendChild(link)
}
