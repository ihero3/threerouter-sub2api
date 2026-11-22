import { describe, expect, it, vi } from 'vitest'

const storeState = vi.hoisted(() => ({
  isAuthenticated: false,
  isAdmin: false,
  activeSite: 'site_a',
}))

vi.mock('@/stores', () => ({
  useAuthStore: () => ({
    get isAuthenticated() {
      return storeState.isAuthenticated
    },
    get isAdmin() {
      return storeState.isAdmin
    },
  }),
  useAppStore: () => ({
    get activeSite() {
      return storeState.activeSite
    },
  }),
}))

import {
  resolveForceEnglishLocale,
  resolveLocaleSwitcherVisible,
  useLocaleAccess,
  type LocaleAccessInput,
} from '../useLocaleAccess'

function input(overrides: Partial<LocaleAccessInput> = {}): LocaleAccessInput {
  return {
    isAuthenticated: true,
    isAdmin: false,
    activeSite: 'site_a',
    ...overrides,
  }
}

describe('locale access rules', () => {
  it('keeps the switcher visible for unauthenticated visitors', () => {
    expect(resolveLocaleSwitcherVisible(input({ isAuthenticated: false }))).toBe(true)
  })

  it('keeps the switcher visible for admins on every site', () => {
    expect(resolveLocaleSwitcherVisible(input({ isAdmin: true }))).toBe(true)
    expect(resolveLocaleSwitcherVisible(input({ isAdmin: true, activeSite: 'site_b' }))).toBe(true)
  })

  it('shows the switcher to regular users on Site B', () => {
    expect(resolveLocaleSwitcherVisible(input({ activeSite: 'site_b' }))).toBe(true)
  })

  it('hides the switcher from regular users on Site A', () => {
    expect(resolveLocaleSwitcherVisible(input())).toBe(false)
  })

  it('forces English only for regular authenticated users on Site A', () => {
    expect(resolveForceEnglishLocale(input())).toBe(true)
    expect(resolveForceEnglishLocale(input({ isAuthenticated: false }))).toBe(false)
    expect(resolveForceEnglishLocale(input({ isAdmin: true }))).toBe(false)
    expect(resolveForceEnglishLocale(input({ activeSite: 'site_b' }))).toBe(false)
  })

  it('derives both rules from the active stores', () => {
    storeState.isAuthenticated = true
    storeState.isAdmin = false
    storeState.activeSite = 'site_a'

    const { localeSwitcherVisible, forceEnglishLocale } = useLocaleAccess()

    expect(localeSwitcherVisible.value).toBe(false)
    expect(forceEnglishLocale.value).toBe(true)
  })
})
