import { describe, it, expect, vi } from 'vitest'
import { newBulkDraft, buildBulkPatch, applyBulkPatch } from '../bulkEdit'
describe('bulk key updates', () => {
  it('requires an explicit field and never includes untouched fields', () => {
    const d = newBulkDraft()
    expect(() => buildBulkPatch(d)).toThrow('chooseField')
    d.enabled.status = true; d.status = 'inactive'
    expect(buildBulkPatch(d)).toEqual({ status: 'inactive' })
  })
  it('supports explicit clearing without changing identity or resetting consumption', () => {
    const d = newBulkDraft()
    d.enabled = { group: false, status: false, quota: true, expiry: true, rate: true, ip: true }
    expect(buildBulkPatch(d)).toEqual({ quota: 0, expires_at: '', rate_limit_5h: 0, rate_limit_1d: 0, rate_limit_7d: 0, ip_whitelist: [], ip_blacklist: [] })
  })
  it('rejects invalid numeric limits and past dates', () => {
    const d = newBulkDraft(); d.enabled.quota = true; d.quota = -1
    expect(() => buildBulkPatch(d)).toThrow('invalidNumber')
    d.quota = 0; d.enabled.expiry = true; d.expiry = '2000-01-01'
    expect(() => buildBulkPatch(d)).toThrow('invalidExpiry')
  })
  it('builds all supported fields with trimmed IP rules', () => {
    const d = newBulkDraft()
    d.enabled = { group: true, status: true, quota: true, expiry: true, rate: true, ip: true }
    d.groupId = 7; d.status = 'inactive'; d.quota = 20; d.expiry = '2099-01-01T00:00:00Z'
    d.rate5h = 2; d.rate1d = 5; d.rate7d = 10; d.whitelist = ' 192.0.2.1\n\n198.51.100.0/24 '; d.blacklist = '203.0.113.1'
    expect(buildBulkPatch(d)).toEqual({ group_id: 7, status: 'inactive', quota: 20, expires_at: '2099-01-01T00:00:00.000Z', rate_limit_5h: 2, rate_limit_1d: 5, rate_limit_7d: 10, ip_whitelist: ['192.0.2.1', '198.51.100.0/24'], ip_blacklist: ['203.0.113.1'] })
    d.groupId = null
    expect(() => buildBulkPatch(d)).toThrow('chooseGroup')
  })
  it('retains only failed ids for retry and never resubmits successes', async () => {
    const update = vi.fn().mockResolvedValueOnce({}).mockRejectedValueOnce(new Error('denied')).mockResolvedValueOnce({})
    const result = await applyBulkPatch([1, 2, 1, 3], { status: 'inactive' }, update)
    expect(result).toEqual({ succeeded: [1, 3], failed: [2] })
    expect(update.mock.calls.map(c => c[0])).toEqual([1, 2, 3])
    update.mockClear(); update.mockResolvedValue({})
    await applyBulkPatch(result.failed, { status: 'inactive' }, update)
    expect(update.mock.calls.map(c => c[0])).toEqual([2])
  })
})
