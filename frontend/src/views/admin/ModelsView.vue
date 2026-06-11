<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex items-center justify-between">
        <div>
          <h1 class="text-3xl font-bold text-gray-900">{{ t('admin.models.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500">{{ t('admin.models.description') }}</p>
        </div>
      </div>

      <div class="card">
        <div class="p-6">
          <div class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
            <div
              v-for="model in models"
              :key="model.id"
              class="group rounded-xl border border-gray-200 bg-white p-4 shadow-sm transition-all hover:border-primary-200 hover:shadow-md"
            >
              <div class="flex items-start gap-3">
                <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-gradient-to-br from-purple-500 to-blue-500">
                  <svg class="h-5 w-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" :d="model.icon" />
                  </svg>
                </div>
                <div class="min-w-0 flex-1">
                  <div class="flex items-center gap-2">
                    <h3 class="truncate font-semibold text-gray-900">{{ model.name }}</h3>
                    <button
                      @click="copyModelName(model.name)"
                      class="p-1 hover:bg-gray-100 rounded transition-colors"
                      :title="t('admin.models.copy')"
                    >
                      <svg v-if="copiedModel !== model.name" class="h-4 w-4 text-gray-400 hover:text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                      </svg>
                      <svg v-else class="h-4 w-4 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                      </svg>
                    </button>
                  </div>
                  <p class="mt-1 text-sm text-gray-500">{{ model.provider }}</p>
                </div>
              </div>
              <div class="mt-4 flex items-center gap-2 text-xs text-gray-400">
                <span class="rounded-full bg-gray-100 px-2 py-1">{{ getCategoryLabel(model.category) }}</span>
                <span class="rounded-full bg-green-100 px-2 py-1 text-green-600">{{ t('admin.models.status.available') }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'

const { t } = useI18n()

const copiedModel = ref<string | null>(null)

interface Model {
  id: string
  name: string
  provider: string
  category: string
  icon: string
}

const models = ref<Model[]>([
  { id: '1', name: 'dola-seed-2.0-pro', provider: 'bytedance', category: 'text', icon: 'M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5' },
  { id: '2', name: 'glm5.1', provider: 'zhipu', category: 'text', icon: 'M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8l-6-6zm-1 2v10H8v-2h5V4zm3 10h-5v2h5v-2zm0-4h-5v2h5v-2z' },
  { id: '3', name: 'deepseek-v4-pro', provider: 'deepseek', category: 'text', icon: 'M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5' },
  { id: '4', name: 'deepseek-v4-flash', provider: 'deepseek', category: 'text', icon: 'M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z' },
  { id: '5', name: 'Kimi-K2.5', provider: 'moonshot', category: 'text', icon: 'M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4' },
  { id: '6', name: 'gpt-oss', provider: 'openai', category: 'text', icon: 'M9 19V6l12-3v13M9 19c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zm12-3c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zM9 10l12-3' },
  { id: '7', name: 'dreamina-seedance-2-0', provider: 'bytedance', category: 'multimodal', icon: 'M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5' },
  { id: '8', name: 'dreamina-seedance-2-0-fast', provider: 'bytedance', category: 'multimodal', icon: 'M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5' },
  { id: '9', name: 'bytedance-seedance-1-5-pro', provider: 'bytedance', category: 'multimodal', icon: 'M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5' },
  { id: '10', name: 'bytedance-seedance-1-0', provider: 'bytedance', category: 'multimodal', icon: 'M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5' },
  { id: '11', name: 'bytedance-seedance-1-0-pro', provider: 'bytedance', category: 'multimodal', icon: 'M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5' },
])

const categoryLabels: Record<string, string> = {
  text: 'admin.models.categories.text',
  image: 'admin.models.categories.image',
  audio: 'admin.models.categories.audio',
  multimodal: 'admin.models.categories.multimodal'
}

const getCategoryLabel = (category: string) => {
  return t(categoryLabels[category] || category)
}

const copyModelName = async (name: string) => {
  try {
    await navigator.clipboard.writeText(name)
    copiedModel.value = name
    setTimeout(() => {
      copiedModel.value = null
    }, 2000)
  } catch (err) {
    console.error('Failed to copy:', err)
  }
}
</script>