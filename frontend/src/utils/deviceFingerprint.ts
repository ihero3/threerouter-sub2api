/**
 * 注册设备指纹。
 *
 * 把浏览器侧相对稳定的 5 类信号压成一个 32 位十六进制摘要：
 *   UA + 时区 + 语言 + 屏幕 + canvas 渲染哈希。
 *
 * 用途：关联「同一台设备换不同 IP 批量注册」的行为。这个摘要不上传原始
 * 信号、不含任何可还原的个人标识，服务端还会再混合一次 User-Agent 后落库。
 *
 * 说明：摘要由前端上报，理论上可以被主动伪造/随机化；它的定位是抬高对抗
 * 成本、把跨 IP 的注册串起来，不是密码学意义上的强身份认证。隐私模式下
 * canvas 可能不可用或每次变化，此时仍会返回基于其余 4 类信号的摘要。
 */

const FP_STORAGE_KEY = 'registration_device_fp'
const FP_HEX_PATTERN = /^[0-9a-f]{32}$/

/** FNV-1a 32 位；纯同步实现，不依赖 crypto.subtle（非 HTTPS 环境不可用）。 */
function fnv1a(input: string, seed: number): number {
  let hash = seed >>> 0
  for (let i = 0; i < input.length; i++) {
    hash ^= input.charCodeAt(i) & 0xff
    hash = Math.imul(hash, 0x01000193) >>> 0
  }
  return hash >>> 0
}

/** canvas 渲染差异：字体、抗锯齿、GPU 光栅化都会让不同设备产生不同像素。 */
function canvasSignal(): string {
  try {
    if (typeof document === 'undefined') return 'unavailable'
    const canvas = document.createElement('canvas')
    canvas.width = 280
    canvas.height = 64
    const ctx = canvas.getContext('2d')
    if (!ctx) return 'unavailable'

    ctx.textBaseline = 'top'
    ctx.font = '15px "Arial"'
    ctx.fillStyle = '#f2603c'
    ctx.fillRect(2, 2, 120, 24)
    ctx.fillStyle = '#173b6b'
    ctx.fillText('ThreeRouter device signal \u2603\ud83d\ude80', 4, 6)
    ctx.font = '13px "Times New Roman"'
    ctx.fillStyle = 'rgba(24, 96, 172, 0.72)'
    ctx.fillText('0123456789abcdef', 6, 32)

    const gradient = ctx.createLinearGradient(0, 0, 280, 0)
    gradient.addColorStop(0, '#ff6b3d')
    gradient.addColorStop(0.5, '#3d8bff')
    gradient.addColorStop(1, '#26c281')
    ctx.fillStyle = gradient
    ctx.fillRect(0, 52, 280, 8)

    return canvas.toDataURL()
  } catch {
    return 'unavailable'
  }
}

function collectSignals(): string {
  const nav = typeof navigator === 'undefined' ? undefined : navigator
  const scr = typeof window !== 'undefined' ? window.screen : undefined
  const screenParts = scr
    ? [
        scr.width,
        scr.height,
        scr.availWidth,
        scr.availHeight,
        scr.colorDepth,
        scr.pixelDepth,
        typeof window !== 'undefined' ? window.devicePixelRatio : 0
      ].join('x')
    : 'unknown'

  let timezone = 'unknown'
  try {
    timezone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'unknown'
  } catch {
    // 极老浏览器不支持 resolvedOptions，保留 unknown，不影响其余信号。
  }

  return [
    `ua:${nav?.userAgent || 'unknown'}`,
    `tz:${timezone}`,
    `lang:${nav?.language || 'unknown'}`,
    `langs:${nav?.languages?.join(',') || 'unknown'}`,
    `screen:${screenParts}`,
    `canvas:${canvasSignal()}`
  ].join('|')
}

function hashSignals(raw: string): string {
  // 4 组不同 seed 的 FNV-1a 拼成 128 bit / 32 位 hex，降低单段碰撞带来的误关联。
  const seeds = [0x811c9dc5, 0x9e3779b9, 0x85ebca6b, 0xc2b2ae35]
  return seeds.map((seed) => fnv1a(raw, seed).toString(16).padStart(8, '0')).join('')
}

/**
 * 返回当前会话稳定的设备指纹；采集完全失败时返回空串（服务端会降级处理）。
 * 结果缓存在 sessionStorage，避免同一注册流程重复绘制 canvas。
 */
export function getDeviceFingerprint(): string {
  try {
    const cached = sessionStorage.getItem(FP_STORAGE_KEY)
    if (cached && FP_HEX_PATTERN.test(cached)) return cached
  } catch {
    // sessionStorage 被禁用时直接走实时计算。
  }

  let fingerprint = ''
  try {
    fingerprint = hashSignals(collectSignals())
  } catch {
    fingerprint = ''
  }

  if (fingerprint) {
    try {
      sessionStorage.setItem(FP_STORAGE_KEY, fingerprint)
    } catch {
      // 缓存失败不影响本次使用。
    }
  }
  return fingerprint
}

/** 仅供测试：清除会话内缓存，强制重新采集。 */
export function clearDeviceFingerprintCache(): void {
  try {
    sessionStorage.removeItem(FP_STORAGE_KEY)
  } catch {
    // ignore
  }
}
