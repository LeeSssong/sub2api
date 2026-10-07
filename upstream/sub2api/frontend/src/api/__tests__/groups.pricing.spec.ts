import { beforeEach, describe, expect, it, vi } from 'vitest'
const get = vi.hoisted(() => vi.fn())
vi.mock('@/api/client', () => ({ apiClient: { get } }))
import { getAvailable, getUserGroupRates } from '../groups'
import { getGroupModels } from '@/features/ai-tools/api'

beforeEach(() => get.mockReset())
describe('价格弹窗原生 API 契约', () => {
  it('reads native groups with request cancellation', async () => {
    const signal = new AbortController().signal
    const groups = [{ id: 1, tool_ids: ['claude'] }]
    get.mockResolvedValue({ data: groups })
    await expect(getAvailable(signal)).resolves.toEqual(groups)
    expect(get).toHaveBeenCalledWith('/groups/available', { signal })
  })
  it('scopes model discovery to the selected native groups', async () => {
    const signal = new AbortController().signal
    get.mockResolvedValue({ data: [{ group_id: 2, supported_models: ['native-alias'] }] })
    await expect(getGroupModels(signal, [2, 4])).resolves.toEqual([{ group_id: 2, supported_models: ['native-alias'] }])
    expect(get).toHaveBeenCalledWith('/groups/available-models', { signal, params: { group_ids: '2,4' } })
  })
  it('normalizes absent custom rates and preserves zero rates', async () => {
    get.mockResolvedValueOnce({ data: null }).mockResolvedValueOnce({ data: { 2: 0 } })
    await expect(getUserGroupRates()).resolves.toEqual({})
    await expect(getUserGroupRates()).resolves.toEqual({ 2: 0 })
    expect(get).toHaveBeenCalledWith('/groups/rates', { signal: undefined })
  })
})
