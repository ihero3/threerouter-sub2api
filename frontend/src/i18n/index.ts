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
  // 禁用 ICU 消息编译器，使用基础插值模式（{变量}）
  // 避免 '@' 符号被误解析为 linked message 语法
  messageCompiler: false
})

const loadedLocales = new Set<LocaleCode>()

// 运行时版本（无编译器）下，纯字符串消息不会被插值，`{n}` 会原样渲染。
// 这里在装载词典时把字符串编译成 vue-i18n 认可的消息函数：
// 不生成代码、不使用 eval，因此不违反 CSP unsafe-eval 限制。
const MESSAGE_TOKEN = /\{(\w+)\}|\{'([^']*)'\}/g

type MessageContextLike = {
  // vue-i18n v10+ 的 JIT 上下文把 named/list 暴露成函数，旧版则是对象/数组
  named?: Record<string, unknown> | ((key: string) => unknown)
  list?: unknown[] | ((index: number) => unknown)
  plural?: number | (() => number)
}

type CompiledMessage = (ctx: MessageContextLike) => string

function resolveNamed(ctx: MessageContextLike, key: string): unknown {
  const named = ctx?.named
  if (typeof named === 'function') return named(key)
  return named?.[key]
}

function resolveList(ctx: MessageContextLike, index: number): unknown {
  const list = ctx?.list
  if (typeof list === 'function') return list(index)
  return list?.[index]
}

function resolvePlural(ctx: MessageContextLike): number {
  const plural = ctx?.plural
  const value = typeof plural === 'function' ? plural() : plural
  return typeof value === 'number' ? value : 0
}

function compileBranch(template: string): CompiledMessage {
  return (ctx) => {
    return template.replace(MESSAGE_TOKEN, (match, namedKey, literal) => {
      // {'@'} / {'{'} 这类写法表示字面量，直接输出花括号内内容
      if (literal !== undefined) return literal
      if (namedKey !== undefined) {
        const namedValue = resolveNamed(ctx, namedKey)
        if (namedValue !== undefined) return String(namedValue)
        const index = Number(namedKey)
        if (Number.isInteger(index)) {
          const listValue = resolveList(ctx, index)
          if (listValue !== undefined) return String(listValue)
        }
      }
      return match
    })
  }
}

function compileMessage(template: string): CompiledMessage {
  // 只有同时出现占位符与 `|` 才按 vue-i18n 的 `单数 | 复数` 语法处理，
  // 避免把文案里作为普通字符的 `|` 误当成分支分隔符
  if (!template.includes('|') || !template.includes('{')) return compileBranch(template)
  const branches = template.split('|').map((item) => compileBranch(item))
  return (ctx) => {
    const index = Math.min(Math.max(resolvePlural(ctx), 0), branches.length - 1)
    return branches[index](ctx)
  }
}

function compileMessageTree(value: unknown): unknown {
  if (typeof value === 'string') {
    // 不含占位符的消息保持字符串，减少无谓的函数调用
    if (!value.includes('{')) return value
    return compileMessage(value)
  }
  // 数组消息通常通过 tm() 取原始结构（如富文本段落），保持原样，避免组件拿到函数
  if (Array.isArray(value)) return value
  if (value && typeof value === 'object') {
    const result: Record<string, unknown> = {}
    for (const [key, item] of Object.entries(value as Record<string, unknown>)) {
      result[key] = compileMessageTree(item)
    }
    return result
  }
  return value
}

function prepareMessages(messages: LocaleMessages): LocaleMessages {
  return compileMessageTree(messages) as LocaleMessages
}

export async function loadLocaleMessages(locale: LocaleCode): Promise<void> {
  const loader = localeLoaders[locale]
  // 始终用带版本号的 URL 强制 import，绕过 Vite 的模块缓存
  const module = await loader()
  // 始终重新注入最新字典（解决开发态下 Vite HMR 不会重新 import 的问题）
  i18n.global.setLocaleMessage(locale, prepareMessages(module.default))
  loadedLocales.add(locale)
}

// 暴露一个手动强制重载函数给 DevTools 调用：window.__reloadLocales?.()
if (typeof window !== 'undefined') {
  ;(window as any).__reloadLocales = async () => {
    loadedLocales.clear()
    const enMod = await import(`./locales/en.ts?t=${Date.now()}`)
    const zhMod = await import(`./locales/zh.ts?t=${Date.now()}`)
    i18n.global.setLocaleMessage('en', prepareMessages(enMod.default))
    i18n.global.setLocaleMessage('zh', prepareMessages(zhMod.default))
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
