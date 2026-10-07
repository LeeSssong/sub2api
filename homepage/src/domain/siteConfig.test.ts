import { describe, expect, it, vi } from 'vitest'
import { DEFAULT_SITE_CONFIG, loadSiteConfig } from './siteConfig'

describe('loadSiteConfig', () => {
  it.each([
    '/uploads/new-logo.png',
    'https://cdn.example.com/logo.svg',
    'http://cdn.example.com/logo.webp',
    'data:image/png;base64,aGVsbG8=',
  ])('loads the public brand logo %s independently of the static config', async (siteLogo) => {
    const fetcher = vi.fn(async (input: RequestInfo | URL) => {
      if (new URL(String(input)).pathname === '/api/v1/settings/public') {
        return Response.json({ code: 0, data: { site_logo: siteLogo } })
      }
      return new Response('', { status: 404 })
    })

    expect(await loadSiteConfig(fetcher, 'https://api.example.com')).toEqual({
      ...DEFAULT_SITE_CONFIG,
      apiOrigin: 'https://api.example.com',
      siteLogo,
    })
    expect(fetcher).toHaveBeenCalledWith(new URL('https://api.example.com/api/v1/settings/public'), {
      cache: 'no-store',
      headers: { Accept: 'application/json' },
    })
  })

  it.each(['', 'javascript:alert(1)', 'data:text/html,<h1>logo</h1>', '//cdn.example.com/logo.png', null])(
    'falls back for an empty or unsafe public logo %s while keeping static settings', async (siteLogo) => {
      const fetcher = vi.fn(async (input: RequestInfo | URL) => Response.json(
        new URL(String(input)).pathname === '/api/v1/settings/public'
          ? { code: 0, data: { site_logo: siteLogo } }
          : { version: 1, apiOrigin: 'https://gateway.example.com', support: { qqGroup: '1234567890' } },
      ))

      expect(await loadSiteConfig(fetcher, 'https://api.example.com')).toEqual({
        ...DEFAULT_SITE_CONFIG,
        apiOrigin: 'https://gateway.example.com',
        support: { qqGroup: '1234567890' },
        siteLogo: '/home-assets/xingqiao-logo-256-v1.webp',
      })
    },
  )

  it('retains static configuration when the public settings request fails', async () => {
    const fetcher = vi.fn(async (input: RequestInfo | URL) => {
      if (new URL(String(input)).pathname === '/api/v1/settings/public') throw new Error('offline')
      return Response.json({ version: 1, apiOrigin: 'https://gateway.example.com' })
    })

    expect(await loadSiteConfig(fetcher, 'https://api.example.com')).toEqual({
      ...DEFAULT_SITE_CONFIG,
      apiOrigin: 'https://gateway.example.com',
      siteLogo: '/home-assets/xingqiao-logo-256-v1.webp',
    })
  })

  it('falls back without reports and retains the Xingqiao QQ group', async () => {
    const fetcher = vi.fn().mockRejectedValue(new Error('offline'))

    await expect(loadSiteConfig(fetcher, 'https://api.example.com')).resolves.toEqual({
      ...DEFAULT_SITE_CONFIG,
      apiOrigin: 'https://api.example.com',
    })
  })

  it('keeps only complete HTTPS third-party reports', async () => {
    const fetcher = vi.fn().mockResolvedValue(new Response(JSON.stringify({
      version: 1,
      apiOrigin: '',
      support: { qqGroup: '1080152144' },
      thirdPartyReports: [
        {
          id: 'modeloc-1',
          provider: 'MODELOC',
          title: '模型真实性报告',
          url: 'https://modeloc.com/r/report',
          status: 'verified',
        },
        {
          id: 'unsafe',
          provider: 'MODELOC',
          title: '不安全',
          url: 'http://example.com',
          status: 'verified',
        },
      ],
    })))

    const config = await loadSiteConfig(fetcher, 'https://api.example.com')

    expect(config.thirdPartyReports.map((report) => report.id)).toEqual(['modeloc-1'])
    expect(config.apiOrigin).toBe('https://api.example.com')
  })
})
