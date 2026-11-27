/**
 * 营销文案里的金额/额度换算工具。
 *
 * 规则：金额本身一律来自后台设置（`/settings/public` 的 new_user_credits /
 * invite_register_reward），i18n 文案只负责排版（`{amount}` / `{tokens}` 占位符），
 * 不再写死具体数字——否则后台改了配置，文案还停留在旧数字。
 */

/** DeepSeek 系模型的换算口径：$1 ≈ 3000 万 tokens。 */
export const DEEPSEEK_TOKENS_PER_USD = 30_000_000

function toPositiveAmount(amount: number | null | undefined): number {
  const value = Number(amount)
  if (!Number.isFinite(value) || value <= 0) return 0
  return Math.round(value * 100) / 100
}

/**
 * 美元金额 → 文案用的 `$6` / `$1.25`（整数不留多余的 `.00`）。
 * 非法或非正数返回空串，调用方应据此隐藏对应文案。
 */
export function formatPromoAmount(amount: number | null | undefined): string {
  const value = toPositiveAmount(amount)
  if (value <= 0) return ''
  return Number.isInteger(value) ? `$${value}` : `$${value.toFixed(2)}`
}

/**
 * 美元金额 → DeepSeek token 量级文案，如 `$6 → 180M`、`$15 → 450M`、`$40 → 1.2B`。
 * 非法或非正数返回空串。
 */
export function formatPromoTokens(amount: number | null | undefined): string {
  const value = toPositiveAmount(amount)
  if (value <= 0) return ''
  const tokens = value * DEEPSEEK_TOKENS_PER_USD
  const [unit, divisor] = tokens >= 1_000_000_000 ? ['B', 1_000_000_000] : ['M', 1_000_000]
  // 保留一位小数，整数则去掉 .0（180M / 1.2B）。
  const scaled = Math.round((tokens / divisor) * 10) / 10
  return `${Number.isInteger(scaled) ? scaled : scaled.toFixed(1)}${unit}`
}
