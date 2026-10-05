import { describe, expect, it } from 'vitest'
import { fitKeyRows, placeKeyMenu } from './keysViewport'

describe('single-screen key inventory', () => {
  it('fits complete rows and keeps header and horizontal scrollbar space', () => {
    expect(fitKeyRows(460, 44, 64)).toBe(6)
    expect(fitKeyRows(320, 44, 80)).toBe(3)
    expect(fitKeyRows(90, 44, 64)).toBe(1)
  })
  it('opens below the trigger even when the space above is larger', () => {
    expect(placeKeyMenu({ left: 300, bottom: 550 }, 1440, 768)).toEqual({ top: 554, left: 300, maxHeight: 206, width: 420 })
  })
  it('fits narrow screens and bounds the menu to remaining viewport height', () => {
    expect(placeKeyMenu({ left: 250, bottom: 640 }, 375, 740)).toEqual({ top: 644, left: 8, maxHeight: 88, width: 359 })
  })
})
