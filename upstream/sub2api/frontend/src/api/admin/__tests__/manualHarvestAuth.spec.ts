import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
vi.mock('../../client', () => ({ apiClient: {}, buildApiUrl: (path: string) => `/api/v1${path}` }))

describe('manual harvest SSE authentication', () => {
  beforeEach(() => {
    vi.resetModules()
    vi.stubEnv('VITE_AUTH_STORAGE_PREFIX', 'test_station_')
    localStorage.clear()
    localStorage.setItem('auth_token', 'other-environment-token')
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, status: 200, body: { getReader: () => ({ read: async () => ({ done: true }) }) } }))
  })
  afterEach(() => { vi.unstubAllEnvs(); vi.unstubAllGlobals(); localStorage.clear() })
  it('uses the configured namespace for manual capture streams', async () => {
    localStorage.setItem('test_station_auth_token', 'scoped-token')
    const { streamManualCodexHarvest } = await import('../accounts')
    await streamManualCodexHarvest(42, {} as any, vi.fn())
    expect(vi.mocked(fetch).mock.calls[0][1]?.headers).toMatchObject({ Authorization: 'Bearer scoped-token' })
  })
  it('does not fall back to another environment token', async () => {
    const { streamManualCodexHarvest } = await import('../accounts')
    await streamManualCodexHarvest(42, {} as any, vi.fn())
    expect(vi.mocked(fetch).mock.calls[0][1]?.headers).not.toHaveProperty('Authorization')
  })
})
