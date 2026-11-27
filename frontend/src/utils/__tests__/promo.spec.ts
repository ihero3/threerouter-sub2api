import { describe, expect, it } from 'vitest'

import { formatPromoAmount, formatPromoTokens } from '../promo'

describe('formatPromoAmount', () => {
  it('keeps integer amounts free of trailing zeros', () => {
    expect(formatPromoAmount(1)).toBe('$1')
    expect(formatPromoAmount(6)).toBe('$6')
  })

  it('keeps cents for non-integer amounts', () => {
    expect(formatPromoAmount(1.25)).toBe('$1.25')
    expect(formatPromoAmount(0.5)).toBe('$0.50')
  })

  it('returns an empty string when nothing is configured', () => {
    expect(formatPromoAmount(0)).toBe('')
    expect(formatPromoAmount(-3)).toBe('')
    expect(formatPromoAmount(Number.NaN)).toBe('')
    expect(formatPromoAmount(undefined)).toBe('')
  })
})

describe('formatPromoTokens', () => {
  it('converts credits to DeepSeek token estimates', () => {
    expect(formatPromoTokens(1)).toBe('30M')
    expect(formatPromoTokens(6)).toBe('180M')
    expect(formatPromoTokens(1.5)).toBe('45M')
  })

  it('switches to billions for large amounts', () => {
    expect(formatPromoTokens(40)).toBe('1.2B')
  })

  it('returns an empty string when nothing is configured', () => {
    expect(formatPromoTokens(0)).toBe('')
    expect(formatPromoTokens(Number.NaN)).toBe('')
  })
})
