import { sanitizeUrl } from '@/utils/url'

export const DEFAULT_SITE_LOGO = '/xingqiao/logo-brand.png'

/** Safari requires an HTTP URL for uploaded tab icons. */
export function faviconUrl(logo: string): string {
  if (!/^data:image\//i.test(logo)) return logo
  let hash = 2166136261
  for (const byte of new TextEncoder().encode(logo)) hash = Math.imul(hash ^ byte, 16777619)
  return `/branding/favicon/${(hash >>> 0).toString(16).padStart(8, '0')}`
}

export function updateFavicon(logoUrl: string): void {
  const sanitizedLogoUrl = logoUrl.trim()
    ? sanitizeUrl(logoUrl, { allowRelative: true, allowDataUrl: true })
    : DEFAULT_SITE_LOGO
  if (!sanitizedLogoUrl) return

  const links = Array.from(document.querySelectorAll<HTMLLinkElement>('link[rel="icon"], link[rel="shortcut icon"]'))
  let link = links.shift()
  if (!link) {
    link = document.createElement('link')
    document.head.appendChild(link)
  }
  links.forEach((duplicate) => duplicate.remove())
  link.rel = 'icon'
  const dataMime = sanitizedLogoUrl.match(/^data:(image\/[a-z0-9.+-]+)[;,]/i)?.[1]
  const extension = new URL(sanitizedLogoUrl, document.baseURI).pathname.split('.').pop()?.toLowerCase()
  const mimeByExtension: Record<string, string> = {
    svg: 'image/svg+xml', png: 'image/png', jpg: 'image/jpeg', jpeg: 'image/jpeg',
    ico: 'image/x-icon', webp: 'image/webp', gif: 'image/gif',
  }
  const mime = dataMime || mimeByExtension[extension || '']
  if (mime) link.type = mime
  else link.removeAttribute('type')
  link.href = faviconUrl(sanitizedLogoUrl)
}
