// 前端蜜罐工具：在注册相关页面统一读取隐藏蜜罐 input 的值。
// 人类用户看不见也不会填这个 input，值始终是空字符串；
// bot 自动填表时会把它填上（非空），后端据此静默拒绝注册。
// 字段名刻意避开 honeypot/bot/token 等关键词降低被探测概率。

export const HONEYPOT_FIELD_NAME = 'company_website'

/** 读当前页面里隐藏蜜罐 input 的值；找不到或未填返回空字符串。 */
export function getHoneypotValue(): string {
  if (typeof document === 'undefined') return ''
  const el = document.querySelector<HTMLInputElement>(
    `input[name="${HONEYPOT_FIELD_NAME}"]`,
  )
  return el?.value?.trim() ?? ''
}
