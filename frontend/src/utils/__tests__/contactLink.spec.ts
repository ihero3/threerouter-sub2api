import { describe, expect, it } from 'vitest'
import { resolveContactHref } from '../contactLink'

describe('resolveContactHref', () => {
  it('resolves X / Twitter handles to x.com links', () => {
    expect(resolveContactHref('x:@ihero1020')).toBe('https://x.com/ihero1020')
    expect(resolveContactHref('Twitter: @ihero1020')).toBe('https://x.com/ihero1020')
  })

  it('passes through explicit URLs', () => {
    expect(resolveContactHref('https://t.me/threerouter')).toBe('https://t.me/threerouter')
  })

  it('turns bare emails into mailto links', () => {
    expect(resolveContactHref('support@example.com')).toBe('mailto:support@example.com')
  })

  it('returns null for unrecognized plain text', () => {
    expect(resolveContactHref('QQ: 123456789')).toBeNull()
    expect(resolveContactHref('')).toBeNull()
    expect(resolveContactHref(undefined)).toBeNull()
  })
})
