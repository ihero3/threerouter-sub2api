import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

type PlainObject = Record<string, unknown>

function collectKeys(value: unknown, prefix = ''): string[] {
  if (value === null || typeof value !== 'object') {
    return [prefix]
  }
  const keys: string[] = []
  for (const [key, child] of Object.entries(value as PlainObject)) {
    const nextPrefix = prefix ? `${prefix}.${key}` : key
    keys.push(...collectKeys(child, nextPrefix))
  }
  return keys
}

function resolveKey(locale: unknown, path: string): unknown {
  let cursor: unknown = locale
  for (const seg of path.split('.')) {
    if (cursor === null || typeof cursor !== 'object') return undefined
    cursor = (cursor as PlainObject)[seg]
  }
  return cursor
}

function templateKeys(relativePath: string): string[] {
  const source = readFileSync(new URL(relativePath, import.meta.url), 'utf8')
  const keys = new Set<string>()
  // 只取完整字面量 key，且要求 `t(` 前面不是标识符字符：
  //   - `emit('open', ...)` 里的 `t(` 是子串，负向后视可排除（`open` 不是 i18n key）；
  //   - `t('payment.methods.' + row.payment_type)` 是动态拼接，只拿到前缀没有意义，
  //     要求引号内以标识符结尾即可自然跳过。
  for (const match of source.matchAll(/(?<![\w$.])t\(\s*'([A-Za-z][A-Za-z0-9_]*(?:\.[A-Za-z0-9_]+)*)'/g)) {
    keys.add(match[1])
  }
  return [...keys].sort()
}

const VIEWS = [
  '../../views/admin/affiliates/AdminAffiliateRelationsView.vue',
  '../../views/admin/affiliates/AdminAffiliateRecordsTable.vue',
  '../../views/user/AffiliateView.vue',
]

describe('affiliate locale completeness', () => {
  it('keeps zh and en affiliate admin keys structurally identical', () => {
    expect(collectKeys(en.admin.affiliates).sort()).toEqual(collectKeys(zh.admin.affiliates).sort())
  })

  it('keeps zh and en affiliate dashboard keys structurally identical', () => {
    expect(collectKeys(en.dashboard.invitees).sort()).toEqual(collectKeys(zh.dashboard.invitees).sort())
  })

  // 模板里出现的每个字面量 i18n key 都必须在两种语言里都存在。
  // 缺失时 vue-i18n 会直接把 key 原文渲染到页面上（"admin.affiliates.relations.xxx"），
  // 构建期完全无感，只能靠这里兜住。
  it.each(VIEWS)('resolves every literal i18n key in %s', (view) => {
    const missing: string[] = []
    for (const key of templateKeys(view)) {
      if (resolveKey(en, key) === undefined) missing.push(`en:${key}`)
      if (resolveKey(zh, key) === undefined) missing.push(`zh:${key}`)
    }
    expect(missing).toEqual([])
  })

  it('keeps nav.affiliateRelations label in both locales', () => {
    expect(zh.nav.affiliateRelations).toBeTruthy()
    expect(en.nav.affiliateRelations).toBeTruthy()
  })
})
