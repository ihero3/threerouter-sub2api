import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { clearDeviceFingerprintCache, getDeviceFingerprint } from '../deviceFingerprint'

const FP_KEY = 'registration_device_fp'
const HEX32 = /^[0-9a-f]{32}$/

describe('getDeviceFingerprint', () => {
  beforeEach(() => {
    clearDeviceFingerprintCache()
    sessionStorage.clear()
    // jsdom 不实现 canvas；返回 null 模拟隐私模式，避免 not-implemented 噪声。
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(null)
  })

  afterEach(() => {
    vi.restoreAllMocks()
    clearDeviceFingerprintCache()
  })

  it('returns a 32-char lowercase hex digest', () => {
    expect(getDeviceFingerprint()).toMatch(HEX32)
  })

  it('is stable across calls in the same session', () => {
    const first = getDeviceFingerprint()
    expect(getDeviceFingerprint()).toBe(first)
    clearDeviceFingerprintCache()
    expect(getDeviceFingerprint()).toBe(first)
  })

  it('caches the digest in sessionStorage', () => {
    const fp = getDeviceFingerprint()
    expect(sessionStorage.getItem(FP_KEY)).toBe(fp)
  })

  it('recomputes when the cached value is malformed', () => {
    sessionStorage.setItem(FP_KEY, 'not-a-fingerprint')
    expect(getDeviceFingerprint()).toMatch(HEX32)
    expect(sessionStorage.getItem(FP_KEY)).toMatch(HEX32)
  })

  it('includes the canvas rendering result in the digest', () => {
    const fakeCtx = {
      textBaseline: '',
      font: '',
      fillStyle: '',
      fillRect: vi.fn(),
      fillText: vi.fn(),
      createLinearGradient: () => ({ addColorStop: vi.fn() }),
    }
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(
      fakeCtx as unknown as CanvasRenderingContext2D,
    )
    const toDataURL = vi.spyOn(HTMLCanvasElement.prototype, 'toDataURL')
    toDataURL.mockReturnValue('data:image/png;base64,device-A')
    const first = getDeviceFingerprint()
    toDataURL.mockReturnValue('data:image/png;base64,device-B')
    clearDeviceFingerprintCache()
    expect(getDeviceFingerprint()).not.toBe(first)
  })

  it('changes when the browser signals change', () => {
    const original = navigator.userAgent
    const first = getDeviceFingerprint()
    Object.defineProperty(navigator, 'userAgent', {
      configurable: true,
      value: `${original}-different-device`,
    })
    try {
      clearDeviceFingerprintCache()
      expect(getDeviceFingerprint()).not.toBe(first)
    } finally {
      Object.defineProperty(navigator, 'userAgent', { configurable: true, value: original })
      clearDeviceFingerprintCache()
    }
  })
})
