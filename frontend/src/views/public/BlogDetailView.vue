<template>
  <div class="min-h-screen bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white">
    <PublicSiteHeader />

    <div class="mx-auto flex max-w-6xl gap-6 px-4 py-8 sm:px-6 lg:py-10">
      <!-- 左侧窄列：返回列表 + 文章标题列表 -->
      <aside class="hidden w-56 flex-shrink-0 md:block">
        <div class="sticky top-6 space-y-3">
          <RouterLink
            to="/blog"
            class="flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm font-medium text-gray-700 shadow-sm transition hover:bg-gray-50 dark:border-dark-700 dark:bg-dark-900 dark:text-dark-200 dark:hover:bg-dark-800"
          >
            <span aria-hidden="true">←</span>
            {{ t('blog.backToList') }}
          </RouterLink>

          <nav class="rounded-xl border border-gray-200 bg-white p-2 shadow-sm dark:border-dark-800 dark:bg-dark-900">
            <p class="px-2 py-1.5 text-xs font-semibold uppercase tracking-wide text-gray-400 dark:text-dark-500">
              {{ t('blog.title') }}
            </p>
            <ul class="mt-1 space-y-0.5">
              <li v-for="item in blogList" :key="item.id">
                <RouterLink
                  :to="`/blog/${item.id}`"
                  :title="item.title"
                  class="block truncate rounded-md px-2 py-1.5 text-sm transition"
                  :class="item.id === currentId ? 'bg-primary-50 font-semibold text-primary-700 dark:bg-primary-900/30 dark:text-primary-300' : 'text-gray-600 hover:bg-gray-100 dark:text-dark-300 dark:hover:bg-dark-800'"
                >
                  {{ item.title }}
                </RouterLink>
              </li>
            </ul>
          </nav>
        </div>
      </aside>

      <!-- 右侧正文 -->
      <main class="min-w-0 flex-1">
        <div v-if="loading" class="flex min-h-[320px] items-center justify-center">
          <div class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600"></div>
        </div>

        <div v-else-if="loadError" class="rounded-xl border border-red-200 bg-red-50 p-6 text-sm text-red-700 dark:border-red-900/40 dark:bg-red-950/40 dark:text-red-200">
          {{ loadError }}
        </div>

        <article v-else-if="blog" class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-dark-800 dark:bg-dark-900 sm:p-8">
          <h1 class="text-2xl font-bold text-gray-900 dark:text-white sm:text-3xl">
            {{ blog.title }}
          </h1>
          <div class="mt-3 flex flex-wrap items-center gap-3 text-sm text-gray-500 dark:text-dark-400">
            <span>{{ formatDateTime(blog.published_at || blog.created_at) }}</span>
            <template v-if="blog.tags">
              <span class="text-gray-300 dark:text-dark-700">·</span>
              <span class="rounded-full bg-gray-100 px-2 py-0.5 text-gray-700 dark:bg-dark-800 dark:text-dark-200">
                {{ blog.tags }}
              </span>
            </template>
          </div>
          <div
            v-if="blog.cover_image"
            class="mt-6 overflow-hidden rounded-xl border border-gray-200 dark:border-dark-800"
          >
            <img :src="blog.cover_image" :alt="blog.title" class="w-full object-cover" />
          </div>
          <!-- 富文本内容：服务端返回 HTML，渲染前用 DOMPurify 净化（防 XSS） -->
          <div
            v-if="isRichText"
            class="prose prose-sm mt-6 max-w-none text-gray-800 dark:prose-invert dark:text-dark-100"
            v-html="sanitizedContent"
          />
          <!-- 兼容早期纯文本（非 HTML）博客内容 -->
          <div
            v-else
            class="prose prose-sm mt-6 max-w-none whitespace-pre-wrap text-gray-800 dark:prose-invert dark:text-dark-100"
          >
            {{ blog.content }}
          </div>
        </article>
      </main>
    </div>

    <PublicSiteFooter />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, RouterLink } from 'vue-router'
import DOMPurify from 'dompurify'
import { publicBlogAPI } from '@/api'
import { formatDateTime } from '@/utils/format'
import type { UserBlog } from '@/types'
import PublicSiteHeader from '@/components/layout/PublicSiteHeader.vue'
import PublicSiteFooter from '@/components/layout/PublicSiteFooter.vue'
import { setLocale, getLocale } from '@/i18n'
import { useSEO } from '@/composables/useSEO'

const { t } = useI18n()
const route = useRoute()

// 博客页面强制英文界面
let savedLocale: string | null = null
onMounted(() => {
  savedLocale = getLocale()
  if (savedLocale !== 'en') {
    setLocale('en')
  }
})
onBeforeUnmount(() => {
  if (savedLocale && savedLocale !== 'en') {
    setLocale(savedLocale)
  }
})

const blog = ref<UserBlog | null>(null)
const loading = ref(false)
const loadError = ref('')

// 左侧标题列表（用于文章间快速切换）
const blogList = ref<UserBlog[]>([])
const currentId = computed(() => Number(route.params.id))

// 富文本编辑器保存的内容是 HTML；纯文本（早期数据）需保持原样展示
const isRichText = computed(
  () => !!blog.value?.content && /<(p|div|h[1-6]|ul|ol|img|blockquote|pre|table)\b/i.test(blog.value.content)
)

const sanitizedContent = computed(() => {
  const html = blog.value?.content || ''
  if (!isRichText.value) return ''
  return DOMPurify.sanitize(html, {
    ADD_TAGS: ['img'],
    ADD_ATTR: ['target', 'rel', 'style', 'src', 'alt', 'width', 'height']
  })
})

async function loadBlog(id: number) {
  loading.value = true
  loadError.value = ''
  try {
    blog.value = await publicBlogAPI.getByID(id)
  } catch (err) {
    console.error('Failed to load blog', err)
    blog.value = null
    loadError.value = t('blog.notFound')
  } finally {
    loading.value = false
  }
}

// 拉取已发布文章标题列表（左侧窄列用，最多 50 篇）
async function loadBlogList() {
  try {
    const res = await publicBlogAPI.list({ page: 1, page_size: 50, status: 'published' })
    blogList.value = (res.items as UserBlog[]).filter((b) => b.id > 0)
  } catch (err) {
    console.error('Failed to load blog list', err)
    blogList.value = []
  }
}

onMounted(() => {
  const id = Number(route.params.id)
  if (id > 0) loadBlog(id)
  loadBlogList()
})

watch(
  () => route.params.id,
  (val) => {
    const id = Number(val)
    if (id > 0) loadBlog(id)
  }
)

// ---------- SEO：按文章动态生成（站点侧文案为英文，标题/摘要跟随文章内容） ----------
const SITE_ORIGIN = 'https://www.threerouter.com'

/** 从富文本/纯文本中提取纯文本摘要（用于 meta description） */
function extractExcerpt(source: string, maxLen = 160): string {
  if (!source) return ''
  let text = source
  if (/<(p|div|h[1-6]|ul|ol|img|blockquote|pre|table)\b/i.test(source)) {
    const doc = new DOMParser().parseFromString(source, 'text/html')
    text = doc.body.textContent || ''
  }
  text = text.replace(/\s+/g, ' ').trim()
  return text.length > maxLen ? `${text.slice(0, maxLen - 1)}…` : text
}

const blogDescription = computed(() => {
  if (!blog.value) return ''
  return blog.value.summary?.trim() || extractExcerpt(blog.value.content)
})

const blogCanonicalUrl = computed(() => `${SITE_ORIGIN}/blog/${currentId.value}`)

// BlogPosting 结构化数据，文章加载成功后注入
const blogJsonLd = computed(() => {
  if (!blog.value) return []
  const data: Record<string, unknown> = {
    '@context': 'https://schema.org',
    '@type': 'BlogPosting',
    'headline': blog.value.title,
    'description': blogDescription.value,
    'url': blogCanonicalUrl.value,
    'mainEntityOfPage': blogCanonicalUrl.value,
    'datePublished': blog.value.published_at || blog.value.created_at,
    'dateModified': blog.value.updated_at || blog.value.published_at || blog.value.created_at,
    'publisher': {
      '@type': 'Organization',
      'name': 'ThreeRouter',
      'url': SITE_ORIGIN
    }
  }
  if (blog.value.cover_image) data['image'] = blog.value.cover_image
  if (blog.value.tags?.trim()) data['keywords'] = blog.value.tags.trim()
  return [data]
})

useSEO({
  title: computed(() => (blog.value ? `${blog.value.title} - ThreeRouter` : '')),
  description: blogDescription,
  ogType: 'article',
  ogUrl: blogCanonicalUrl,
  ogImage: computed(() => blog.value?.cover_image || `${SITE_ORIGIN}/src/assets/icons/logo.webp`),
  ogSiteName: 'ThreeRouter',
  canonicalUrl: blogCanonicalUrl,
  jsonLd: blogJsonLd
})
</script>
