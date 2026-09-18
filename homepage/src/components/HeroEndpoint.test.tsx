import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { HeroEndpoint } from './HeroEndpoint'
afterEach(() => { cleanup(); vi.useRealTimers(); vi.unstubAllGlobals() })
it('copies only the configured base URL while the path rotates independently', async () => {
  vi.useFakeTimers()
  const writeText = vi.fn().mockResolvedValue(undefined)
  vi.stubGlobal('navigator', { clipboard: { writeText } })
  render(<HeroEndpoint origin="https://gateway.example" />)
  await act(async () => { fireEvent.click(screen.getByRole('button', { name: '复制 API 地址' })) })
  expect(writeText).toHaveBeenCalledWith('https://gateway.example')
  expect(screen.getByText('已复制')).toBeVisible()
  act(() => { vi.advanceTimersByTime(4400) })
  expect(document.querySelector('.hero-endpoint-path')).toHaveTextContent('/v1/messages')
  expect(screen.getByText('https://gateway.example')).toBeVisible()
})
it('reports a copy failure without claiming success', async () => {
  vi.stubGlobal('navigator', { clipboard: { writeText: vi.fn().mockRejectedValue(new Error('denied')) } })
  render(<HeroEndpoint origin="https://gateway.example" />)
  await act(async () => { fireEvent.click(screen.getByRole('button', { name: '复制 API 地址' })) })
  expect(screen.getByText('请选中复制')).toBeVisible()
  expect(screen.queryByText('已复制')).not.toBeInTheDocument()
})
