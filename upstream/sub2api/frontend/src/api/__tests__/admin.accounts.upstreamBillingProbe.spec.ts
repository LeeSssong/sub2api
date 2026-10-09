import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post, put, del } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  del: vi.fn()
}))

vi.mock('@/api/client', () => ({
  apiClient: { get, post, put, delete: del }
}))

import {
  getNewAPIUpstreamConfig,
  previewNewAPIUpstreamConfig,
  saveNewAPIUpstreamConfig,
  deleteNewAPIUpstreamConfig,
  getUpstreamBillingProbeSettings,
  probeUpstreamBilling,
  probeUpstreamBillingBatch,
  setUpstreamBillingProbeEnabled,
  updateProcurementCost,
  updateUpstreamBillingProbeSettings,
} from '@/api/admin/accounts'

describe('admin account upstream billing probe API', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    put.mockReset()
    del.mockReset()
  })

  it('uses the config endpoints and preserves explicit account and token selections', async () => {
    const config = { account_id: 7, site_url: 'https://upstream.example', configured: false, encryption_key_configured: true, accounts: [] }
    const result = { site_url: config.site_url, user_id: 42, wallet: { amount: 12.34, unit: 'USD' }, accounts: [] }
    const payload = { user_id: 42, access_token: 'fixture-secret', account_ids: [7, 8], token_selections: { '8': 19 } }
    get.mockResolvedValueOnce({ data: config })
    post.mockResolvedValueOnce({ data: result })
    put.mockResolvedValueOnce({ data: result })
    del.mockResolvedValueOnce({ data: { account_id: 7, configured: false } })
    await expect(getNewAPIUpstreamConfig(7)).resolves.toEqual(config)
    await expect(previewNewAPIUpstreamConfig(7, payload)).resolves.toEqual(result)
    await expect(saveNewAPIUpstreamConfig(7, payload)).resolves.toEqual(result)
    await expect(deleteNewAPIUpstreamConfig(7)).resolves.toEqual({ account_id: 7, configured: false })
    expect(get).toHaveBeenCalledWith('/admin/accounts/7/upstream-billing-probe/config')
    expect(post).toHaveBeenCalledWith('/admin/accounts/7/upstream-billing-probe/config/preview', payload)
    expect(put).toHaveBeenCalledWith('/admin/accounts/7/upstream-billing-probe/config', payload)
    expect(del).toHaveBeenCalledWith('/admin/accounts/7/upstream-billing-probe/config')
  })

  it('reads and updates global settings', async () => {
    const settings = { enabled: true, interval_minutes: 30 }
    get.mockResolvedValueOnce({ data: settings })
    put.mockResolvedValueOnce({ data: settings })

    await expect(getUpstreamBillingProbeSettings()).resolves.toEqual(settings)
    await expect(updateUpstreamBillingProbeSettings(settings)).resolves.toEqual(settings)
    expect(get).toHaveBeenCalledWith('/admin/accounts/upstream-billing-probe/settings')
    expect(put).toHaveBeenCalledWith('/admin/accounts/upstream-billing-probe/settings', settings)
  })

  it('uses dedicated account and batch endpoints', async () => {
    const result = { account_id: 7, snapshot: { status: 'unsupported' } }
    put.mockResolvedValueOnce({ data: {} })
    post.mockResolvedValueOnce({ data: result })
    post.mockResolvedValueOnce({ data: { results: [result] } })

    await setUpstreamBillingProbeEnabled(7, true)
    await expect(probeUpstreamBilling(7)).resolves.toEqual(result)
    await expect(probeUpstreamBillingBatch([7])).resolves.toEqual([result])

    expect(put).toHaveBeenCalledWith('/admin/accounts/7/upstream-billing-probe', { enabled: true })
    expect(post).toHaveBeenNthCalledWith(1, '/admin/accounts/7/upstream-billing-probe')
    expect(post).toHaveBeenNthCalledWith(2, '/admin/accounts/upstream-billing-probe/batch', { account_ids: [7] })
  })

  it('uses the explicit cost-dialog session idempotency key', async () => {
    put.mockResolvedValueOnce({ data: { account_id: 7 } })
    await expect(updateProcurementCost(7, 4, 60, 'procurement-session-1')).resolves.toEqual({ account_id: 7 })
    expect(put).toHaveBeenCalledWith('/admin/accounts/7', {
      procurement_cost_cny: 4,
      estimated_usable_quota_usd: 60,
    }, { headers: { 'Idempotency-Key': 'procurement-session-1' } })
  })

  it('normalizes the interceptor partial-success rejection into a typed procurement result', async () => {
    put.mockRejectedValueOnce({ status: 202, code: 202, message: '采购成本已保存，但账号刷新失败，请刷新页面确认', reason: 'procurement_saved_readback_failed', metadata: { request_id: 'procurement-session-2' }, data: { id: 7, procurement_cost_cny: 4, estimated_usable_quota_usd: 60 } })
    await expect(updateProcurementCost(7, 4, 60, 'procurement-session-2')).resolves.toMatchObject({
      id: 7, procurement_cost_cny: 4, estimated_usable_quota_usd: 60,
      procurement_readback_status: 'failed', procurement_reason: 'procurement_saved_readback_failed',
      procurement_request_id: 'procurement-session-2',
    })
  })

})
