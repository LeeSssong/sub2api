import { beforeEach, describe, expect, it, vi } from 'vitest'
const { post } = vi.hoisted(() => ({ post: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { post } }))
import { importData } from '@/api/admin/accounts'

describe('account data admission request', () => {
  beforeEach(() => { post.mockReset().mockResolvedValue({ data: {} }) })
  it('forwards root admission and native formal groups without nesting in the data bundle', async () => {
    const request = { data: { proxies: [], accounts: [] }, skip_default_group_bind: true, group_ids: [1, 2], admission: { enabled: true as const } }
    await importData(request)
    expect(post).toHaveBeenCalledWith('/admin/accounts/data', request)
  })
  it('preserves the old request without new optional fields', async () => {
    const request = { data: { proxies: [], accounts: [] }, skip_default_group_bind: true }
    await importData(request)
    expect(post).toHaveBeenCalledWith('/admin/accounts/data', request)
  })
})
