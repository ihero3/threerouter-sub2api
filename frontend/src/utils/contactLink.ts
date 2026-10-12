// Resolve a clickable href from the free-form "contact info" setting.
// Returns null when the text is not a recognized link (e.g. "QQ: 123456789").
const URL_PATTERN = /https?:\/\/[^\s]+/i
const X_HANDLE_PATTERN = /(?:^|\s)(?:x|twitter)\s*[:：]\s*@?([A-Za-z0-9_]{1,15})(?=\s|$)/i
const EMAIL_PATTERN = /[^\s@]+@[^\s@]+\.[^\s@]+/

export function resolveContactHref(contact: string | null | undefined): string | null {
  const text = (contact ?? '').trim()
  if (!text) return null

  const url = text.match(URL_PATTERN)
  if (url) return url[0]

  const xHandle = text.match(X_HANDLE_PATTERN)
  if (xHandle) return `https://x.com/${xHandle[1]}`

  const email = text.match(EMAIL_PATTERN)
  if (email) return `mailto:${email[0]}`

  return null
}
