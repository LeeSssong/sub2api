import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import PelicanShowcaseView from '../PelicanShowcaseView.vue'
import type { PelicanShowcaseItem, PelicanShowcaseView as ShowcaseData } from '@/api/pelicanShowcase'

const { getShowcase, getShowcaseItem, removeShowcaseItem, showError, showSuccess, auth } = vi.hoisted(() => ({
  getShowcase: vi.fn(),
  getShowcaseItem: vi.fn(),
  removeShowcaseItem: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  auth: { isAdmin: false },
}))
vi.mock('@/api/pelicanShowcase', () => ({ getShowcase, getShowcaseItem, removeShowcaseItem }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string, named?: Record<string, unknown>) => (named ? `${key} ${JSON.stringify(named)}` : key) }),
}))

const item = (id: number, groupId: number): PelicanShowcaseItem => ({
  id, group_id: groupId, model_id: 'gpt-6-astra', reasoning_effort: 'high', latency_ms: 42300,
  generated_at: '2026-09-24T08:30:00Z',
})
const showcase = (overrides: Partial<ShowcaseData> = {}): ShowcaseData => ({
  enabled: true,
  max_items: 20,
  retention_days: 7,
  groups: [
    { id: 1, name: 'Claude Max', platform: 'anthropic', items: Array.from({ length: 10 }, (_, i) => item(100 + i, 1)) },
    { id: 2, name: 'GPT Plus', platform: 'openai', items: [item(200, 2)] },
    { id: 3, name: 'Empty', platform: 'gemini', items: [] },
  ],
  ...overrides,
})

const withStats = () => {
  const data = showcase()
  Object.assign(data, {
    // Group memberships overlap: the server total is deliberately not their sum.
    stats: { success_count: 6, total_count: 8, success_rate: 75 },
    stats_window: {
      from: '2026-09-25T08:00:00Z', to: '2026-09-26T08:00:00Z',
      coverage_started_at: '2026-09-24T08:00:00Z', complete: true,
    },
  })
  Object.assign(data.groups[0], { stats: { success_count: 5, total_count: 7, success_rate: 500 / 7 } })
  Object.assign(data.groups[1], { stats: { success_count: 5, total_count: 6, success_rate: 500 / 6 } })
  Object.assign(data.groups[2], { stats: { success_count: 0, total_count: 0, success_rate: null } })
  return data
}

const mountView = () => mount(PelicanShowcaseView, {
  global: {
    stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      Icon: true,
      PlatformIcon: true,
      EmptyState: { props: ['title', 'description'], template: '<div class="empty-state">{{ title }}</div>' },
      ConfirmDialog: {
        props: ['show'], emits: ['confirm', 'cancel'],
        template: '<div v-if="show" class="confirm"><button class="confirm-yes" @click="$emit(\'confirm\')" /></div>',
      },
      BaseDialog: {
        props: ['show', 'title'], emits: ['close'],
        template: '<div v-if="show" class="dialog"><h3>{{ title }}</h3><slot /><slot name="footer" /></div>',
      },
    },
  },
})

// The shared test setup installs an observer that never fires; cards here are on screen.
class OnScreenObserver {
  constructor(private readonly callback: IntersectionObserverCallback) {}
  observe(target: Element) {
    this.callback([{ isIntersecting: true, target } as IntersectionObserverEntry], this as unknown as IntersectionObserver)
  }
  disconnect() {}
  unobserve() {}
}

// Cards report being on screen after a short dwell (see PelicanShowcaseCard); then their HTML is fetched.
async function settle() {
  await flushPromises()
  await vi.advanceTimersByTimeAsync(1000)
  await flushPromises()
}

let wrapper: ReturnType<typeof mountView>
beforeEach(() => {
  vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
  vi.stubGlobal('IntersectionObserver', OnScreenObserver)
  getShowcase.mockReset()
  getShowcaseItem.mockReset().mockImplementation(async (id: number) => ({
    ...item(id, 0),
    response_text: id === 201 ? '21' : `<svg data-item="${id}"></svg>`,
  }))
  removeShowcaseItem.mockReset().mockResolvedValue(undefined)
  showError.mockReset()
  showSuccess.mockReset()
  auth.isAdmin = false
})
afterEach(() => {
  wrapper?.unmount()
  vi.unstubAllGlobals()
  vi.useRealTimers()
})

describe('PelicanShowcaseView', () => {
  it('uses deduplicated server totals and switches the summary with the group filter', async () => {
    getShowcase.mockResolvedValue(withStats())
    wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('[data-testid="showcase-statistics-count"]').text()).toBe('6 / 8')
    expect(wrapper.get('[data-testid="showcase-statistics-rate"]').text()).toBe('75.0%')
    expect(wrapper.get('[data-testid="showcase-group-stats-1"]').text()).toContain('5 / 7')
    expect(wrapper.get('[data-testid="showcase-group-stats-1"]').text()).toContain('71.4%')
    await wrapper.get('[data-testid="showcase-tab-2"]').trigger('click')
    expect(wrapper.get('[data-testid="showcase-statistics-count"]').text()).toBe('5 / 6')
    expect(wrapper.get('[data-testid="showcase-statistics-rate"]').text()).toBe('83.3%')
    expect(wrapper.get('[data-testid="showcase-statistics"]').text()).toContain('GPT Plus')
  })

  it('distinguishes no tests from completed tests that all failed', async () => {
    const data = withStats()
    Object.assign(data.groups[1], { stats: { success_count: 0, total_count: 6, success_rate: 0 } })
    getShowcase.mockResolvedValue(data)
    wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="showcase-tab-3"]').trigger('click')
    expect(wrapper.get('[data-testid="showcase-statistics-count"]').text()).toBe('0 / 0')
    expect(wrapper.get('[data-testid="showcase-statistics-rate"]').text()).toBe('—')
    expect(wrapper.get('[data-testid="showcase-statistics"]').text()).toContain('pelicanShowcase.statistics.noTests')
    await wrapper.get('[data-testid="showcase-tab-2"]').trigger('click')
    expect(wrapper.get('[data-testid="showcase-statistics-count"]').text()).toBe('0 / 6')
    expect(wrapper.get('[data-testid="showcase-statistics-rate"]').text()).toBe('0.0%')
  })

  it('keeps artwork usable when statistics are absent and does not infer counts from cards', async () => {
    getShowcase.mockResolvedValue(showcase())
    wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('[data-testid="showcase-statistics-count"]').text()).toBe('— / —')
    expect(wrapper.get('[data-testid="showcase-statistics-rate"]').text()).toBe('—')
    expect(wrapper.get('[data-testid="showcase-statistics"]').text()).toContain('pelicanShowcase.statistics.unavailable')
    expect(wrapper.findAll('[data-testid="pelican-showcase-card"]').length).toBeGreaterThan(0)
  })

  it('labels incomplete coverage with its actual collection start', async () => {
    const data = withStats()
    Object.assign(data, { stats_window: {
      from: '2026-09-25T08:00:00Z', to: '2026-09-26T08:00:00Z',
      coverage_started_at: '2026-09-26T06:00:00Z', complete: false,
    } })
    getShowcase.mockResolvedValue(data)
    wrapper = mountView()
    await flushPromises()
    const coverage = wrapper.get('[data-testid="showcase-statistics-coverage"]')
    expect(coverage.text()).toContain('pelicanShowcase.statistics.partialCoverage')
    expect(coverage.text()).toContain('2026')
    expect(wrapper.get('[data-testid="showcase-statistics-window"]').text()).toContain('pelicanShowcase.statistics.sinceEnabled')
  })

  it('marks statistics unavailable when refresh fails instead of presenting stale values as current', async () => {
    getShowcase.mockResolvedValueOnce(withStats()).mockRejectedValueOnce(new Error('offline'))
    wrapper = mountView()
    await flushPromises()
    await wrapper.get('button[aria-label="common.refresh"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="showcase-statistics-count"]').text()).toBe('— / —')
    expect(wrapper.get('[data-testid="showcase-group-stats-1"]').text()).toContain('— / —')
    expect(wrapper.findAll('[data-testid="pelican-showcase-card"]').length).toBeGreaterThan(0)
    expect(showError).toHaveBeenCalled()
  })

  it('explains that the gallery is closed without asking for items', async () => {
    getShowcase.mockResolvedValue(showcase({ enabled: false, groups: [] }))
    wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('.empty-state').text()).toBe('pelicanShowcase.disabled.title')
    expect(wrapper.findAll('[data-testid="pelican-showcase-card"]')).toHaveLength(0)
    expect(getShowcaseItem).not.toHaveBeenCalled()
  })

  it('lists every group as one row with the gallery rules and loads visible cards in a sandbox', async () => {
    getShowcase.mockResolvedValue(showcase())
    wrapper = mountView()
    await settle()

    expect(wrapper.get('[data-testid="showcase-keep-rule"]').text()).toContain('"count":20')
    expect(wrapper.get('[data-testid="showcase-retention-rule"]').text()).toContain('"days":7')
    expect(wrapper.findAll('[role="tab"]').map((tab) => tab.text())).toEqual([
      'pelicanShowcase.allGroups', 'Claude Max10', 'GPT Plus1', 'Empty0',
    ])
    expect(wrapper.get('[data-testid="showcase-group-3"]').text()).toContain('pelicanShowcase.groupEmpty')

    // Every item of a group sits in its row, in the order received (newest first); no paging.
    const firstRow = wrapper.get('[data-testid="showcase-group-1"] [data-testid="pelican-showcase-row"]')
    expect(firstRow.findAll('iframe').map((frame) => frame.attributes('srcdoc').match(/data-item="(\d+)"/)?.[1]))
      .toEqual(Array.from({ length: 10 }, (_, i) => String(100 + i)))
    expect(wrapper.find('[data-testid="showcase-group-3"] [data-testid="pelican-showcase-row"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="showcase-group-1"] [role="scrollbar"]').attributes('aria-label'))
      .toBe('pelicanShowcase.scrollLabel {"group":"Claude Max"}')

    const cards = wrapper.findAll('[data-testid="pelican-showcase-card"]')
    expect(cards).toHaveLength(11)
    expect(cards[0].classes()).toContain('shrink-0')
    expect(getShowcaseItem).toHaveBeenCalledTimes(11)
    const frame = cards[0].get('iframe')
    expect(frame.attributes('sandbox')).toBe('allow-scripts')
    expect(frame.attributes('referrerpolicy')).toBe('no-referrer')
    expect(frame.attributes('srcdoc')).toContain('Content-Security-Policy')
    expect(frame.attributes('srcdoc')).toContain('data-item="100"')
    expect(cards[0].text()).toContain('gpt-6-astra')
    expect(cards[0].text()).toContain('"seconds":"42.3"')
    expect(cards[0].text()).toContain('pelicanShowcase.efforts.high')

    await wrapper.get('[data-testid="showcase-tab-2"]').trigger('click')
    expect(wrapper.find('[data-testid="showcase-group-1"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="showcase-group-2"]').exists()).toBe(true)
  })

  it('fetches HTML only for cards that reach the viewport', async () => {
    vi.unstubAllGlobals() // back to the setup observer, which never reports a card as visible
    getShowcase.mockResolvedValue(showcase())
    wrapper = mountView()
    await settle()
    expect(wrapper.findAll('[data-testid="pelican-showcase-card"]')).toHaveLength(11)
    expect(getShowcaseItem).not.toHaveBeenCalled()
  })

  it('shows a readable state for output without HTML and for failed loads', async () => {
    getShowcase.mockResolvedValue(showcase({
      groups: [{ id: 2, name: 'GPT Plus', platform: 'openai', items: [item(201, 2), item(202, 2)] }],
    }))
    getShowcaseItem.mockImplementation(async (id: number) => {
      if (id === 202) throw new Error('boom')
      return { ...item(id, 2), response_text: '21' }
    })
    wrapper = mountView()
    await settle()
    const cards = wrapper.findAll('[data-testid="pelican-showcase-card"]')
    expect(cards[0].find('iframe').exists()).toBe(false)
    expect(cards[0].text()).toContain('pelicanShowcase.invalidHtml')
    expect(cards[1].text()).toContain('pelicanShowcase.itemLoadError')

    // Refresh retries a failed card even though it already reported being on screen.
    getShowcaseItem.mockImplementation(async (id: number) => ({ ...item(id, 2), response_text: `<svg data-item="${id}"></svg>` }))
    await wrapper.get('button[aria-label="common.refresh"]').trigger('click')
    await flushPromises()
    expect(getShowcaseItem).toHaveBeenCalledTimes(3)
    expect(wrapper.findAll('[data-testid="pelican-showcase-card"]')[1].get('iframe').attributes('srcdoc')).toContain('data-item="202"')
  })

  it('defaults each preview to fit and preserves statistics when artwork is removed', async () => {
    getShowcase.mockResolvedValue(withStats())
    wrapper = mountView()
    await settle()

    await wrapper.get('[data-testid="showcase-group-2"] [data-testid="pelican-showcase-card"] button').trigger('click')
    await flushPromises()
    const dialog = wrapper.get('[data-testid="showcase-preview"]')
    expect(dialog.get('iframe').attributes('srcdoc')).toContain('data-item="200"')
    expect(dialog.get('iframe').attributes('sandbox')).toBe('allow-scripts')
    expect(dialog.get('[data-testid="showcase-preview-fit"]').attributes('aria-pressed')).toBe('true')
    await dialog.get('[data-testid="showcase-preview-actual"]').trigger('click')
    expect(dialog.get('[data-testid="showcase-preview-actual"]').attributes('aria-pressed')).toBe('true')
    expect(wrapper.find('[data-testid="showcase-remove"]').exists()).toBe(false)
    wrapper.unmount()

    auth.isAdmin = true
    wrapper = mountView()
    await settle()
    await wrapper.get('[data-testid="showcase-group-2"] [data-testid="pelican-showcase-card"] button').trigger('click')
    await wrapper.get('[data-testid="showcase-remove"]').trigger('click')
    await wrapper.get('.confirm-yes').trigger('click')
    await flushPromises()
    expect(removeShowcaseItem).toHaveBeenCalledWith(200)
    expect(showSuccess).toHaveBeenCalledWith('pelicanShowcase.removed')
    expect(wrapper.find('[data-testid="showcase-preview"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="showcase-group-2"]').text()).toContain('pelicanShowcase.groupEmpty')
    expect(wrapper.get('[data-testid="showcase-statistics-count"]').text()).toBe('6 / 8')
    expect(wrapper.get('[data-testid="showcase-group-stats-2"]').text()).toContain('5 / 6')
  })
})
