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
                    <path stroke-linecap="round" stroke-linejoin="round" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" />
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
}

const models = ref<Model[]>([
  { id: '1', name: 'Sendaunce2.0', provider: 'ByteDance', category: 'video' },
  { id: '2', name: 'glm5.1', provider: 'zhipu', category: 'text' },
  { id: '3', name: 'Gemini Ultra', provider: 'Google', category: 'multimodal' },
  { id: '4', name: 'DALL-E 3', provider: 'OpenAI', category: 'image' },
  { id: '5', name: 'Stable Diffusion', provider: 'Stability AI', category: 'image' },
  { id: '6', name: 'Whisper', provider: 'OpenAI', category: 'audio' },
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