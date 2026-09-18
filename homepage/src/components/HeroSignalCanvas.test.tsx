import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { applyHomepageTheme } from '../themeBootstrap'
import { HeroSignalCanvas } from './HeroSignalCanvas'

function createCanvasContext() {
  return {
    clearRect: vi.fn(),
    fillText: vi.fn(),
    measureText: vi.fn((text: string) => ({ width: text.length * 7 })),
    setTransform: vi.fn(),
    save: vi.fn(),
    restore: vi.fn(),
    textBaseline: 'middle',
    font: '',
    fillStyle: '',
    globalAlpha: 1,
    shadowBlur: 0,
    shadowColor: '',
  }
}

describe('HeroSignalCanvas', () => {
  beforeEach(() => {
    document.documentElement.dataset.theme = 'dark'
  })

  afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
  })

  it('draws upright single characters in downward columns without horizontal drift', () => {
    const context = createCanvasContext()
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(context as unknown as CanvasRenderingContext2D)
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue({ width: 800, height: 600, left: 0, top: 0 } as DOMRect)
    let tick: FrameRequestCallback = () => {}
    vi.stubGlobal('requestAnimationFrame', vi.fn((callback: FrameRequestCallback) => { tick = callback; return 7 }))
    render(<HeroSignalCanvas active direction="down" label="竖向信号" />)
    const before = context.fillText.mock.calls.map(call => [...call])
    expect(before.every(call => String(call[0]).length === 1)).toBe(true)
    const columnPositions = [...new Set(before.map(call => Number(call[1])))]
    const firstColumnPositions = before
      .filter(call => Number(call[1]) === columnPositions[0])
      .map(call => Number(call[2]))
    expect(columnPositions[1]! - columnPositions[0]!).toBeCloseTo(600 / 36)
    expect(firstColumnPositions[1]! - firstColumnPositions[0]!).toBe(7)
    context.fillText.mockClear()
    tick(16)
    const after = context.fillText.mock.calls
    expect(after[0]![1]).toBe(before[0]![1])
    expect(Number(after[0]![2])).toBeGreaterThan(Number(before[0]![2]))
  })

  it('renders three layers and redraws when the homepage theme changes', () => {
    const context = createCanvasContext()
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(context as unknown as CanvasRenderingContext2D)
    vi.stubGlobal('matchMedia', vi.fn().mockReturnValue({
      matches: false,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    }))
    vi.stubGlobal('requestAnimationFrame', vi.fn(() => 7))
    vi.stubGlobal('cancelAnimationFrame', vi.fn())

    render(<HeroSignalCanvas active label="实时信号" />)
    const signal = screen.getByRole('img', { name: '实时信号' })
    expect(signal).toHaveAttribute('data-signal-layers', '3')
    const drawsBeforeThemeChange = context.clearRect.mock.calls.length

    applyHomepageTheme('light')

    expect(context.clearRect.mock.calls.length).toBeGreaterThan(drawsBeforeThemeChange)
  })

  it('draws one static layered frame without starting RAF for reduced motion', () => {
    const context = createCanvasContext()
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(context as unknown as CanvasRenderingContext2D)
    vi.stubGlobal('matchMedia', vi.fn().mockReturnValue({
      matches: true,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    }))
    const requestAnimationFrame = vi.fn(() => 7)
    vi.stubGlobal('requestAnimationFrame', requestAnimationFrame)
    vi.stubGlobal('cancelAnimationFrame', vi.fn())

    render(<HeroSignalCanvas active label="实时信号" />)

    expect(context.fillText).toHaveBeenCalled()
    expect(requestAnimationFrame).not.toHaveBeenCalled()
    expect(screen.getByRole('img', { name: '实时信号' })).toHaveAttribute('data-canvas-active', 'false')
  })
})
