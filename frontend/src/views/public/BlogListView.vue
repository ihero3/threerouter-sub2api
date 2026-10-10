<template>
  <div class="min-h-screen bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white">
    <PublicSiteHeader />

    <main class="mx-auto max-w-4xl px-4 py-8 sm:px-6 lg:py-10">
      <div class="mb-6 flex items-end justify-between">
        <div>
          <h1 class="text-2xl font-bold text-gray-900 dark:text-white sm:text-3xl">
            {{ t('blog.title') }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            {{ t('blog.subtitle') }}
          </p>
        </div>
        <div class="w-40">
          <input
            v-model="searchQuery"
            type="text"
            :placeholder="t('blog.searchPlaceholder')"
            class="input"
            @input="handleSearch"
          />
        </div>
      </div>

      <div v-if="loading && blogs.length === 0" class="flex min-h-[240px] items-center justify-center">
        <div class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600"></div>
      </div>

      <div v-else-if="blogs.length === 0" class="rounded-xl border border-dashed border-gray-300 bg-white p-10 text-center text-sm text-gray-500 dark:border-dark-700 dark:bg-dark-900 dark:text-dark-400">
        {{ t('blog.empty') }}
      </div>

      <ul v-else class="space-y-4">
        <li
          v-for="b in blogs"
          :key="b.id"
          class="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-sm transition hover:shadow-md dark:border-dark-800 dark:bg-dark-900"
        >
          <RouterLink :to="`/blog/${b.id}`" class="group flex flex-col sm:flex-row">
            <div
              v-if="b.cover_image"
              class="h-44 w-full shrink-0 overflow-hidden bg-gray-100 dark:bg-dark-800 sm:h-40 sm:w-64"
            >
              <img
                :src="b.cover_image"
                alt=""
                loading="lazy"
                class="h-full w-full object-cover transition duration-300 group-hover:scale-105"
              />
            </div>
            <div class="min-w-0 flex-1 p-5">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ b.title }}
              </h2>
              <p v-if="b.summary" class="mt-1 line-clamp-2 text-sm text-gray-600 dark:text-dark-300">
                {{ b.summary }}
              </p>
              <p v-else class="mt-1 line-clamp-2 text-sm text-gray-600 dark:text-dark-300">
                {{ b.content }}
              </p>
              <div class="mt-3 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
                <span>{{ formatDateTime(b.published_at || b.created_at) }}</span>
                <template v-if="b.tags">
                  <span class="text-gray-300 dark:text-dark-700">·</span>
                  <span class="rounded-full bg-gray-100 px-2 py-0.5 text-gray-700 dark:bg-dark-800 dark:text-dark-200">
                    {{ b.tags }}
                  </span>
                </template>
              </div>
            </div>
          </RouterLink>
        </li>
      </ul>

      <div v-if="total > pageSize" class="mt-6 flex items-center justify-center gap-2">
        <button
          class="btn btn-secondary"
          :disabled="page <= 1 || loading"
          @click="changePage(page - 1)"
        >
          {{ t('common.previous') }}
        </button>
        <span class="text-sm text-gray-600 dark:text-dark-300">
          {{ page }} / {{ totalPages }}
        </span>
        <button
          class="btn btn-secondary"
          :disabled="page >= totalPages || loading"
          @click="changePage(page + 1)"
        >
          {{ t('common.next') }}
        </button>
      </div>
    </main>

    <PublicSiteFooter />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { publicBlogAPI } from '@/api'
import { formatDateTime } from '@/utils/format'
import PublicSiteHeader from '@/components/layout/PublicSiteHeader.vue'
import PublicSiteFooter from '@/components/layout/PublicSiteFooter.vue'
import { setLocale, getLocale } from '@/i18n'

const { t } = useI18n()

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

const blogs = ref<import('@/types').UserBlog[]>([])
const loading = ref(false)
const searchQuery = ref('')
const page = ref(1)
const pageSize = ref(10)
const total = ref(0)

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))

let searchTimer: ReturnType<typeof setTimeout> | null = null

async function loadBlogs() {
  loading.value = true
  try {
    const result = await publicBlogAPI.list({
      page: page.value,
      page_size: pageSize.value,
      search: searchQuery.value.trim() || undefined
    })
    blogs.value = result.items as import('@/types').UserBlog[]
    total.value = result.total
  } catch (err) {
    console.error('Failed to load blogs', err)
    blogs.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    page.value = 1
    loadBlogs()
  }, 300)
}

function changePage(next: number) {
  if (next < 1 || next > totalPages.value) return
  page.value = next
  loadBlogs()
}

onMounted(loadBlogs)
</script>
