<template>
  <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div class="min-w-0">
        <p class="text-sm font-medium text-gray-900 dark:text-white">
          {{ t('admin.settings.site.imageLibrary.title') }}
        </p>
        <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.settings.site.imageLibrary.hint') }}
        </p>
      </div>
      <div class="flex shrink-0 items-center gap-2">
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          :disabled="loading"
          @click="loadImages"
        >
          <Icon name="refresh" size="sm" class="mr-1.5" :stroke-width="2" />
          {{ t('admin.settings.site.imageLibrary.refresh') }}
        </button>
        <label
          class="btn btn-secondary btn-sm cursor-pointer"
          :class="{ 'pointer-events-none opacity-60': uploading }"
        >
          <input
            type="file"
            class="hidden"
            accept="image/jpeg,image/png,image/gif,image/webp"
            multiple
            @change="handleFileInput"
          />
          <Icon name="upload" size="sm" class="mr-1.5" :stroke-width="2" />
          {{
            uploading
              ? t('admin.settings.site.imageLibrary.uploading')
              : t('admin.settings.site.imageLibrary.upload')
          }}
        </label>
      </div>
    </div>

    <p v-if="error" class="mt-2 text-xs text-red-500">{{ error }}</p>
    <p
      v-else-if="!loading && items.length === 0"
      class="mt-3 text-xs text-gray-500 dark:text-gray-400"
    >
      {{ t('admin.settings.site.imageLibrary.empty') }}
    </p>

    <ul v-if="items.length > 0" class="mt-3 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
      <li
        v-for="item in items"
        :key="item.name"
        class="rounded-lg border border-gray-200 p-2 dark:border-dark-700"
      >
        <a :href="item.url" target="_blank" rel="noopener noreferrer" class="block">
          <img
            :src="item.thumb_url"
            :alt="item.name"
            loading="lazy"
            class="h-24 w-full rounded-md bg-gray-50 object-contain dark:bg-dark-800"
          />
        </a>
        <div class="mt-2 flex items-center justify-between gap-2">
          <span
            class="truncate text-xs text-gray-500 dark:text-gray-400"
            :title="item.name"
          >
            {{ formatSize(item.size) }} · {{ formatDate(item.created_at) }}
          </span>
          <div class="flex shrink-0 items-center gap-1">
            <button
              type="button"
              class="rounded px-1.5 py-0.5 text-xs text-primary-600 hover:bg-primary-50 dark:text-primary-400 dark:hover:bg-dark-700"
              @click="copyURL(item)"
            >
              {{
                copiedName === item.name
                  ? t('admin.settings.site.imageLibrary.copied')
                  : t('admin.settings.site.imageLibrary.copyUrl')
              }}
            </button>
            <button
              type="button"
              class="rounded px-1.5 py-0.5 text-xs text-primary-600 hover:bg-primary-50 dark:text-primary-400 dark:hover:bg-dark-700"
              @click="emit('insert', item.url)"
            >
              {{ t('admin.settings.site.imageLibrary.insert') }}
            </button>
          </div>
        </div>
      </li>
    </ul>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'
import { adminBlogAPI, type AdminUploadedImage } from '@/api/blogs'
import { extractApiErrorMessage } from '@/utils/apiError'

const emit = defineEmits<{
  insert: [url: string]
}>()

const MAX_UPLOAD_SIZE = 10 * 1024 * 1024

const { t } = useI18n()
const { copyToClipboard } = useClipboard()

const items = ref<AdminUploadedImage[]>([])
const loading = ref(false)
const uploading = ref(false)
const error = ref('')
const copiedName = ref('')

async function loadImages() {
  loading.value = true
  error.value = ''
  try {
    const result = await adminBlogAPI.listImages()
    items.value = result.items
  } catch (err) {
    error.value = extractApiErrorMessage(err, t('admin.settings.site.imageLibrary.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function handleFileInput(event: Event) {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files || [])
  input.value = ''
  if (files.length === 0) return

  error.value = ''
  uploading.value = true
  try {
    for (const file of files) {
      if (file.size > MAX_UPLOAD_SIZE) {
        error.value = t('admin.settings.site.imageLibrary.tooLarge', { name: file.name })
        continue
      }
      if (!/^image\/(jpeg|png|gif|webp)$/.test(file.type)) {
        error.value = t('admin.settings.site.imageLibrary.unsupported', { name: file.name })
        continue
      }
      await adminBlogAPI.uploadImage(file)
    }
    await loadImages()
  } catch (err) {
    error.value = extractApiErrorMessage(err, t('admin.settings.site.imageLibrary.uploadFailed'))
  } finally {
    uploading.value = false
  }
}

async function copyURL(item: AdminUploadedImage) {
  const absolute = new URL(item.url, window.location.origin).toString()
  if (await copyToClipboard(absolute)) {
    copiedName.value = item.name
    window.setTimeout(() => {
      if (copiedName.value === item.name) copiedName.value = ''
    }, 2000)
  }
}

function formatSize(bytes: number): string {
  if (bytes >= 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)}MB`
  return `${Math.max(1, Math.round(bytes / 1024))}KB`
}

function formatDate(iso: string): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

onMounted(loadImages)
</script>
