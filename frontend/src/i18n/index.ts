import { createI18n } from 'vue-i18n'

type LocaleCode = 'en' | 'zh'

type LocaleMessages = Record<string, any>

const LOCALE_KEY = 'sub2api_locale'
const DEFAULT_LOCALE: LocaleCode = 'en'

const localeLoaders: Record<LocaleCode, () => Promise<{ default: LocaleMessages }>> = {
  en: () => import('./locales/en'),
  zh: () => import('./locales/zh')
}

function isLocaleCode(value: string): value is LocaleCode {
  return value === 'en' || value === 'zh'
}

function getDefaultLocale(): LocaleCode {
  const saved = localStorage.getItem(LOCALE_KEY)
  if (saved && isLocaleCode(saved)) {
    return saved
  }
  // 检测浏览器语言
  const browserLang = navigator.language.toLowerCase()
  if (browserLang.startsWith('zh')) {
    return 'zh'
  }
  return DEFAULT_LOCALE
}

export const i18n = createI18n({
  legacy: false,
  locale: getDefaultLocale(),
  fallbackLocale: DEFAULT_LOCALE,
  messages: {},
  // 使用 tm() 获取对象/数组翻译，保持 t() 仅用于字符串
  // 禁用 HTML 消息警告 - 引导步骤使用富文本内容（driver.js 支持 HTML）
  // 这些内容是内部定义的，不存在 XSS 风险
  warnHtmlMessage: false,
  // 消息编译交给 vue-i18n 自带的 JIT 编译器（vite/vitest 配置里的
  // __INTLIFY_JIT_COMPILATION__ 开关），支持 {变量} 插值与 `单数 | 复数`。
  // 注意：不能用 messageCompiler: false —— 那样等于关闭编译，t() 会直接返回 key。
})

const loadedLocales = new Set<LocaleCode>()

export async function loadLocaleMessages(locale: LocaleCode): Promise<void> {
  const loader = localeLoaders[locale]
  // 始终用带版本号的 URL 强制 import，绕过 Vite 的模块缓存
  const module = await loader()
  // 始终重新注入最新字典（解决开发态下 Vite HMR 不会重新 import 的问题）
  i18n.global.setLocaleMessage(locale, module.default)
  loadedLocales.add(locale)
}

// 暴露一个手动强制重载函数给 DevTools 调用：window.__reloadLocales?.()
if (typeof window !== 'undefined') {
  ;(window as any).__reloadLocales = async () => {
    loadedLocales.clear()
    const enMod = await import(`./locales/en.ts?t=${Date.now()}`)
    const zhMod = await import(`./locales/zh.ts?t=${Date.now()}`)
    i18n.global.setLocaleMessage('en', enMod.default)
    i18n.global.setLocaleMessage('zh', zhMod.default)
    loadedLocales.add('en')
    loadedLocales.add('zh')
    console.info('[i18n] locales reloaded')
  }
}

if (import.meta.hot) {
  // i18n 自身不接受热更新，locale 文件变更时全页刷新最稳
  import.meta.hot.accept(['./locales/en', './locales/zh'], () => {
    // 让浏览器彻底丢弃旧的 i18n 状态
    if (typeof window !== 'undefined' && (window as any).__reloadLocales) {
      ;(window as any).__reloadLocales()
    }
  })
}

export async function initI18n(): Promise<void> {
  const current = getLocale()
  await loadLocaleMessages(current)
  document.documentElement.setAttribute('lang', current)
}

export async function setLocale(locale: string): Promise<void> {
  if (!isLocaleCode(locale)) {
    return
  }

  await loadLocaleMessages(locale)
  i18n.global.locale.value = locale
  localStorage.setItem(LOCALE_KEY, locale)
  document.documentElement.setAttribute('lang', locale)

  // 同步更新浏览器页签标题，使其跟随语言切换
  const { resolveRouteDocumentTitle } = await import('@/router/title')
  const { default: router } = await import('@/router')
  const { useAppStore } = await import('@/stores/app')
  const { useAuthStore } = await import('@/stores/auth')
  const { useAdminSettingsStore } = await import('@/stores/adminSettings')
  const route = router.currentRoute.value
  const appStore = useAppStore()
  const authStore = useAuthStore()
  const adminSettingsStore = useAdminSettingsStore()
  const customMenuItems = [
    ...(appStore.cachedPublicSettings?.custom_menu_items ?? []),
    ...(authStore.isAdmin ? adminSettingsStore.customMenuItems : []),
  ]
  document.title = resolveRouteDocumentTitle(route, appStore.siteName, customMenuItems)
}

export function getLocale(): LocaleCode {
  const current = i18n.global.locale.value
  return isLocaleCode(current) ? current : DEFAULT_LOCALE
}

export const availableLocales = [
  { code: 'en', name: 'English', flag: '🇺🇸' },
  { code: 'zh', name: '中文', flag: '🇨🇳' }
] as const

export default i18n
