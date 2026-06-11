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
                <div :class="['flex h-10 w-10 shrink-0 items-center justify-center rounded-lg', getProviderStyle(model.provider).gradient]">
                  <svg class="h-5 w-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" :d="getProviderStyle(model.provider).icon" />
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
  { id: '1', name: 'deepseek-v4', provider: 'DeepSeek V4 is a cutting-edge MoE-based flagship model, excelling in coding, reasoning, and long-context tasks with robust tool-use capabilities for complex workflows.', category: 'text', icon: '' },
  { id: '2', name: 'MiniMax-M3', provider: 'MiniMax‑M3 is a frontier open‑weight model with 1M context, native multimodality, and top coding/agent abilities, built on the MSA sparse attention architecture.', category: 'text', icon: '' },
  { id: '3', name: 'kimi-k2.6', provider: 'Kimi-K2.6 is Moonshot’s open MoE flagship with 256K context, excelling in long-horizon coding and agent swarm (300 sub-agents) for complex, multi-step tasks.', category: 'text', icon: '' },
  { id: '4', name: 'qwen3.7-max', provider: 'Qwen3.7-Max is Alibaba’s agent‑centric flagship with 1M context, top-tier coding, and 35-hour autonomous execution, excelling in complex workflows and multi-framework generalization.', category: 'text', icon: '' },
  { id: '5', name: 'glm-5.1', provider: 'GLM-5.1 is Zhipu AI’s open MoE flagship with 200K context, excelling in 8‑hour autonomous agentic coding and topping SWE‑Bench Pro for complex software engineering tasks.', category: 'text', icon: '' },
  { id: '6', name: 'seedance-2.0', provider: 'Seedance-2.0 is ByteDance’s multimodal video flagship with unified audio-video generation, excelling in cinematic quality, precise camera control and physics-aware motion for professional content.', category: 'multimodal', icon: '' },
  { id: '7', name: 'gpt-5.6', provider: 'GPT-5.6 is OpenAI’s agent-native flagship with 1M context, excelling in autonomous coding, tool orchestration and long-horizon workflows, topping benchmarks for complex real-world tasks.', category: 'text', icon: '' },
])

const providerStyles: Record<string, { gradient: string; icon: string }> = {
  bytedance: {
    gradient: 'bg-gradient-to-br from-rose-500 to-orange-500',
    icon: 'M8 5v14l11-7z'
  },
  zhipu: {
    gradient: 'bg-gradient-to-br from-blue-500 to-indigo-500',
    icon: 'M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z'
  },
  deepseek: {
    gradient: 'bg-gradient-to-br from-cyan-500 to-teal-500',
    icon: 'M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0zM10 7v3m0 0v3m0-3h3m-3 0H7'
  },
  moonshot: {
    gradient: 'bg-gradient-to-br from-indigo-500 to-purple-600',
    icon: 'M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z'
  },
  openai: {
    gradient: 'bg-gradient-to-br from-emerald-500 to-green-500',
    icon: 'M13 10V3L4 14h7v7l9-11h-7z'
  },
  default: {
    gradient: 'bg-gradient-to-br from-purple-500 to-blue-500',
    icon: 'M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5'
  }
}

const getProviderStyle = (provider: string) => {
  return providerStyles[provider] || providerStyles.default
}

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