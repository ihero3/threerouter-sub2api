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

describe('governance locale completeness', () => {
  it('keeps zh and en governance keys structurally identical', () => {
    const zhKeys = collectKeys(zh.admin.governance).sort()
    const enKeys = collectKeys(en.admin.governance).sort()
    expect(enKeys).toEqual(zhKeys)
  })

  it('exposes industry template tab and copy in both locales', () => {
    expect(zh.admin.governance.tabs.templates).toBeTruthy()
    expect(en.admin.governance.tabs.templates).toBeTruthy()
    expect(zh.admin.governance.templates.apply).toBeTruthy()
    expect(en.admin.governance.templates.apply).toBeTruthy()
  })

  it('exposes content moderation rule tab and copy in both locales', () => {
    expect(zh.admin.governance.tabs.rules).toBeTruthy()
    expect(en.admin.governance.tabs.rules).toBeTruthy()
    expect(zh.admin.governance.rules.strategy).toBeTruthy()
    expect(en.admin.governance.rules.strategy).toBeTruthy()
    expect(zh.admin.governance.rules.saveSuccess).toBeTruthy()
    expect(en.admin.governance.rules.saveSuccess).toBeTruthy()
  })
})
