import { describe, expect, it } from 'vitest'
import {
  BILLING_MODE_IMAGE,
  BILLING_MODE_TOKEN,
  BILLING_MODE_VIDEO,
  getDisplayBillingMode,
  isImageUsage,
  isVideoUsage,
  videoUnitPrice
} from '../billingMode'

describe('billingMode helpers', () => {
  it('prefers explicit video mode over image_count', () => {
    expect(
      getDisplayBillingMode({ image_count: 1, billing_mode: BILLING_MODE_VIDEO })
    ).toBe(BILLING_MODE_VIDEO)
    expect(isImageUsage({ image_count: 1, billing_mode: BILLING_MODE_VIDEO })).toBe(false)
  })

  it('detects video usage by video_count or billing_mode', () => {
    expect(isVideoUsage({ video_count: 1, billing_mode: BILLING_MODE_VIDEO })).toBe(true)
    expect(isVideoUsage({ video_count: 0, billing_mode: BILLING_MODE_VIDEO })).toBe(true)
    expect(isVideoUsage({ video_count: 1, billing_mode: null })).toBe(true)
    expect(isVideoUsage({ video_count: 0, billing_mode: BILLING_MODE_TOKEN })).toBe(false)
    expect(isVideoUsage(null)).toBe(false)
  })

  it('computes per-video unit price', () => {
    expect(videoUnitPrice({ video_count: 1, total_cost: 0.5 })).toBe(0.5)
    expect(videoUnitPrice({ video_count: 0, total_cost: 0.5 })).toBe(0)
    expect(videoUnitPrice(null)).toBe(0)
  })

  it('infers image when image_count set and mode missing', () => {
    expect(getDisplayBillingMode({ image_count: 2, billing_mode: null })).toBe(BILLING_MODE_IMAGE)
  })

  it('keeps token mode even with image_count', () => {
    expect(
      getDisplayBillingMode({ image_count: 1, billing_mode: BILLING_MODE_TOKEN })
    ).toBe(BILLING_MODE_TOKEN)
  })
})
