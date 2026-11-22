import { computed } from 'vue'
import { useAppStore, useAuthStore } from '@/stores'

export interface LocaleAccessInput {
  isAuthenticated: boolean
  isAdmin: boolean
  activeSite: string
}

/**
 * 语言切换入口的可见性规则：
 * - 管理员：所有站点可见
 * - 未登录访客：保留可见
 * - Site B（中文站）：所有用户可见
 * - 普通登录用户 + Site A：隐藏
 */
export function resolveLocaleSwitcherVisible(input: LocaleAccessInput): boolean {
  return input.isAdmin || !input.isAuthenticated || input.activeSite === 'site_b'
}

/**
 * 普通登录用户访问 Site A 时锁定英文；管理员、访客和 Site B 不受影响。
 */
export function resolveForceEnglishLocale(input: LocaleAccessInput): boolean {
  return input.isAuthenticated && !input.isAdmin && input.activeSite !== 'site_b'
}

export function useLocaleAccess() {
  const authStore = useAuthStore()
  const appStore = useAppStore()

  const localeAccessInput = (): LocaleAccessInput => ({
    isAuthenticated: authStore.isAuthenticated,
    isAdmin: authStore.isAdmin,
    activeSite: appStore.activeSite,
  })

  return {
    localeSwitcherVisible: computed(() => resolveLocaleSwitcherVisible(localeAccessInput())),
    forceEnglishLocale: computed(() => resolveForceEnglishLocale(localeAccessInput())),
  }
}
