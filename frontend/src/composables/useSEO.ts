/**
 * SEO 组合式函数
 * 动态管理页面的 SEO meta 标签、canonical 链接和 JSON-LD 结构化数据
 *
 * 功能：
 * 1. 动态创建/更新 meta 标签（description, keywords, og:*, twitter:*）
 * 2. 动态管理 canonical link 标签
 * 3. 支持 JSON-LD 结构化数据注入
 * 4. 支持 robots noindex 指令
 * 5. 同时支持 string 和 Ref<string> 类型的参数
 * 6. 路由变化时自动重新应用 SEO 配置
 */
import { watchEffect, onUnmounted, unref, type Ref } from 'vue'
import { useRoute } from 'vue-router'

/**
 * SEO 配置选项
 */
interface SEOOptions {
  /** 页面标题 */
  title?: string | Ref<string>
  /** 页面描述 */
  description?: string | Ref<string>
  /** 页面关键词 */
  keywords?: string | Ref<string>
  /** Open Graph 类型，默认 website */
  ogType?: string
  /** Open Graph 标题，未设置时回退到 title */
  ogTitle?: string | Ref<string>
  /** Open Graph 描述，未设置时回退到 description */
  ogDescription?: string | Ref<string>
  /** Open Graph 图片 URL */
  ogImage?: string | Ref<string>
  /** Open Graph 页面 URL */
  ogUrl?: string | Ref<string>
  /** Open Graph 站点名称 */
  ogSiteName?: string | Ref<string>
  /** canonical 链接 URL */
  canonicalUrl?: string | Ref<string>
  /** JSON-LD 结构化数据，支持单个对象或数组 */
  jsonLd?: object | object[] | Ref<object | object[]>
  /** 是否禁止搜索引擎索引 */
  noindex?: boolean
}

/** JSON-LD script 标签的唯一标识 */
const JSON_LD_ID = '__seo_json_ld__'

/**
 * 创建或更新 meta 标签
 * 如果对应属性的 meta 标签已存在则更新内容，不存在则创建并添加到 head
 *
 * @param attribute - 查询属性名（'name' 或 'property'）
 * @param key - 属性值（如 'description', 'og:title'）
 * @param content - meta 标签内容
 */
function upsertMeta(
  attribute: 'name' | 'property',
  key: string,
  content: string
): void {
  let element = document.head.querySelector(
    `meta[${attribute}="${key}"]`
  ) as HTMLMetaElement | null

  if (!element) {
    element = document.createElement('meta')
    element.setAttribute(attribute, key)
    document.head.appendChild(element)
  }
  element.setAttribute('content', content)
}

/**
 * 创建或更新 canonical link 标签
 *
 * @param href - canonical URL
 */
function upsertCanonical(href: string): void {
  let element = document.head.querySelector(
    'link[rel="canonical"]'
  ) as HTMLLinkElement | null

  if (!element) {
    element = document.createElement('link')
    element.setAttribute('rel', 'canonical')
    document.head.appendChild(element)
  }
  element.setAttribute('href', href)
}

/**
 * 创建或更新 JSON-LD 结构化数据 script 标签
 *
 * @param data - JSON-LD 数据对象或数组
 */
function upsertJsonLd(data: object | object[]): void {
  let element = document.getElementById(
    JSON_LD_ID
  ) as HTMLScriptElement | null

  if (!element) {
    element = document.createElement('script')
    element.id = JSON_LD_ID
    element.setAttribute('type', 'application/ld+json')
    document.head.appendChild(element)
  }

  // 过滤掉数组中可能为 null/undefined 的元素（Safari 对此零容错）
  const sanitized = Array.isArray(data)
    ? data.filter((item): item is object => item != null)
    : data

  try {
    element.textContent = JSON.stringify(sanitized)
  } catch (e) {
    console.warn('[SEO] Failed to serialize JSON-LD:', e)
  }
}

/**
 * 创建或更新 robots meta 标签
 *
 * @param noindex - 是否禁止索引
 */
function upsertRobots(noindex: boolean): void {
  upsertMeta('name', 'robots', noindex ? 'noindex, nofollow' : 'index, follow')
}

/**
 * 移除 JSON-LD script 标签
 */
function removeJsonLd(): void {
  const element = document.getElementById(JSON_LD_ID)
  if (element) {
    element.remove()
  }
}

/**
 * SEO 组合式函数
 *
 * 在组件 setup 中调用，自动管理页面 SEO 标签。
 * 支持响应式参数（Ref），当 Ref 值变化时自动更新对应标签。
 * 组件卸载时停止监听并可选地清理 JSON-LD 数据。
 *
 * @example
 * ```ts
 * // 基本用法
 * useSEO({
 *   title: '首页 - ThreeRouter',
 *   description: 'AI API 统一网关',
 *   keywords: 'AI API, API网关',
 *   ogImage: 'https://www.threerouter.com/og-image.png',
 *   canonicalUrl: 'https://www.threerouter.com/home'
 * })
 *
 * // 响应式用法
 * const title = ref('首页')
 * useSEO({
 *   title,
 *   ogTitle: title,
 *   jsonLd: {
 *     '@context': 'https://schema.org',
 *     '@type': 'WebSite',
 *     name: 'ThreeRouter',
 *     url: 'https://www.threerouter.com'
 *   }
 * })
 * ```
 *
 * @param options - SEO 配置选项
 */
export function useSEO(options: SEOOptions) {
  const route = useRoute()

  const stop = watchEffect(() => {
    // 依赖路由路径，路由变化时重新应用 SEO
    void route.fullPath

    // Safari 对 watchEffect 内的未捕获异常零容错，整体 try-catch 防止渲染管线中断
    try {
      const title = unref(options.title)
      const description = unref(options.description)
      const keywords = unref(options.keywords)
      const ogType = options.ogType
      const ogTitle = unref(options.ogTitle) ?? title
      const ogDescription = unref(options.ogDescription) ?? description
      const ogImage = unref(options.ogImage)
      const ogUrl = unref(options.ogUrl)
      const ogSiteName = unref(options.ogSiteName)
      const canonicalUrl = unref(options.canonicalUrl)
      const jsonLd = unref(options.jsonLd)
      const noindex = options.noindex

      // 页面标题
      if (title) {
        document.title = title
      }

      // 基础 meta 标签
      if (description) {
        upsertMeta('name', 'description', description)
      }
      if (keywords) {
        upsertMeta('name', 'keywords', keywords)
      }

      // Open Graph 标签
      if (ogType) {
        upsertMeta('property', 'og:type', ogType)
      }
      if (ogTitle) {
        upsertMeta('property', 'og:title', ogTitle)
      }
      if (ogDescription) {
        upsertMeta('property', 'og:description', ogDescription)
      }
      if (ogImage) {
        upsertMeta('property', 'og:image', ogImage)
      }
      if (ogUrl) {
        upsertMeta('property', 'og:url', ogUrl)
      }
      if (ogSiteName) {
        upsertMeta('property', 'og:site_name', ogSiteName)
      }

      // Twitter Card 标签
      upsertMeta('name', 'twitter:card', 'summary_large_image')
      if (ogTitle) {
        upsertMeta('name', 'twitter:title', ogTitle)
      }
      if (ogDescription) {
        upsertMeta('name', 'twitter:description', ogDescription)
      }
      if (ogImage) {
        upsertMeta('name', 'twitter:image', ogImage)
      }

      // canonical 链接
      if (canonicalUrl) {
        upsertCanonical(canonicalUrl)
      }

      // JSON-LD 结构化数据（空数组视为未提供，避免注入空 script）
      const jsonLdValue = Array.isArray(jsonLd) ? (jsonLd.length > 0 ? jsonLd : null) : jsonLd
      if (jsonLdValue) {
        upsertJsonLd(jsonLdValue)
      }

      // robots 指令
      if (noindex !== undefined) {
        upsertRobots(noindex)
      }
    } catch (e) {
      console.warn('[SEO] Failed to apply SEO settings:', e)
    }
  })

  // 组件卸载时停止监听并清理 JSON-LD
  onUnmounted(() => {
    stop()
    removeJsonLd()
  })

  return { stop }
}
