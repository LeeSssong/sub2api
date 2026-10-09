import type { CustomMenuItem } from '@/types'
import { sanitizeUrl } from '@/utils/url'

/** Reuse the existing public storefront configuration without adding auth query parameters. */
export function getRechargeStorefront(items: CustomMenuItem[] = []): CustomMenuItem | null {
  const item = items.find(item => item.id === 'xingqiao-storefront' && item.visibility === 'user')
  if (!item || item.page_slug) return null
  const url = sanitizeUrl(item.url)
  return url ? { ...item, url } : null
}
