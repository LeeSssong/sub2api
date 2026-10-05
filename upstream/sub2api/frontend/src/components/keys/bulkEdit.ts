import type { UpdateApiKeyRequest } from '@/types'

export type BulkField = 'group' | 'status' | 'quota' | 'expiry' | 'rate' | 'ip'
export interface BulkDraft {
  enabled: Record<BulkField, boolean>
  groupId: number | null
  status: 'active' | 'inactive'
  quota: number
  expiry: string
  rate5h: number
  rate1d: number
  rate7d: number
  whitelist: string
  blacklist: string
}
export const newBulkDraft = (): BulkDraft => ({
  enabled: { group: false, status: false, quota: false, expiry: false, rate: false, ip: false },
  groupId: null, status: 'active', quota: 0, expiry: '', rate5h: 0, rate1d: 0, rate7d: 0,
  whitelist: '', blacklist: ''
})
export function buildBulkPatch(draft: BulkDraft): UpdateApiKeyRequest {
  const patch: UpdateApiKeyRequest = {}
  const number = (value: number) => {
    if (value === null || value === undefined || !Number.isFinite(Number(value)) || Number(value) < 0) throw new Error('invalidNumber')
    return Number(value)
  }
  if (draft.enabled.group) {
    if (!draft.groupId) throw new Error('chooseGroup')
    patch.group_id = draft.groupId
  }
  if (draft.enabled.status) patch.status = draft.status
  if (draft.enabled.quota) patch.quota = number(draft.quota)
  if (draft.enabled.expiry) {
    if (draft.expiry && (!Number.isFinite(Date.parse(draft.expiry)) || Date.parse(draft.expiry) <= Date.now())) throw new Error('invalidExpiry')
    patch.expires_at = draft.expiry ? new Date(draft.expiry).toISOString() : ''
  }
  if (draft.enabled.rate) {
    patch.rate_limit_5h = number(draft.rate5h)
    patch.rate_limit_1d = number(draft.rate1d)
    patch.rate_limit_7d = number(draft.rate7d)
  }
  if (draft.enabled.ip) {
    const lines = (text: string) => text.split('\n').map(s => s.trim()).filter(Boolean)
    patch.ip_whitelist = lines(draft.whitelist)
    patch.ip_blacklist = lines(draft.blacklist)
  }
  if (!Object.keys(patch).length) throw new Error('chooseField')
  return patch
}
export async function applyBulkPatch(ids: number[], patch: UpdateApiKeyRequest, update: (id: number, patch: UpdateApiKeyRequest) => Promise<unknown>) {
  const succeeded: number[] = [], failed: number[] = []
  for (const id of [...new Set(ids)]) {
    try { await update(id, patch); succeeded.push(id) } catch { failed.push(id) }
  }
  return { succeeded, failed }
}
