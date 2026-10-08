import { describe, expect, it } from 'vitest'
import { formatBalanceFixed, formatCurrency, formatMoneyFixed, formatUsdMoney } from '@/utils/format'

describe('money formatters', () => {
  it('formats valid money values with exactly two fractional digits', () => {
    expect(formatBalanceFixed(90.5)).toBe('90.50')
    expect(formatBalanceFixed(0)).toBe('0.00')
    expect(formatBalanceFixed(0.001)).toBe('0.00')
    expect(formatBalanceFixed(-1.2)).toBe('-1.20')
    expect(formatBalanceFixed(1.005)).toBe('1.01')
    expect(formatBalanceFixed(0.01146136)).toBe('0.01')
  })

  it('keeps native precision for sub-cent currency values', () => {
    expect(formatCurrency(0.00558)).toBe('$0.005580')
    expect(formatUsdMoney(0.01146136, 8)).toBe('$0.01146136')
  })

  it('does not turn unavailable values into a fabricated zero', () => {
    expect(formatBalanceFixed(null)).toBe('—')
    expect(formatBalanceFixed(undefined)).toBe('—')
    expect(formatBalanceFixed(Number.NaN)).toBe('—')
    expect(formatBalanceFixed(Number.POSITIVE_INFINITY)).toBe('—')
    expect(formatCurrency(null)).toBe('$0.00')
  })

  it('omits the currency symbol when a value is unavailable', () => {
    expect(formatUsdMoney(undefined)).toBe('—')
    expect(formatUsdMoney(Number.NaN)).toBe('—')
    expect(formatUsdMoney(90.5)).toBe('$90.50')
    expect(formatUsdMoney(0.001)).toBe('$0.0010')
    expect(formatUsdMoney(1234.5)).toBe('$1.23K')
    expect(formatMoneyFixed(0.01146136, 8)).toBe('0.01146136')
  })
})
