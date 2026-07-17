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
              <template v-if="model.category === 'hint'">
                <div class="flex flex-col items-center justify-center gap-2 py-5">
                  <div :class="['flex h-10 w-10 shrink-0 items-center justify-center rounded-lg', getProviderStyle(model.vendor).gradient]">
                    <svg class="h-5 w-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                      <path stroke-linecap="round" stroke-linejoin="round" :d="getProviderStyle(model.vendor).icon" />
                    </svg>
                  </div>
                  <p class="text-sm text-gray-600 text-left leading-relaxed max-w-[180px]">{{ t('admin.models.hint') }}</p>
                </div>
              </template>
              <template v-else>
                <div class="flex items-start gap-3">
                  <div :class="['flex h-10 w-10 shrink-0 items-center justify-center rounded-lg', getProviderStyle(model.vendor).gradient]">
                    <svg class="h-5 w-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                      <path stroke-linecap="round" stroke-linejoin="round" :d="getProviderStyle(model.vendor).icon" />
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
                    <p class="mt-1 text-sm text-gray-500">{{ getProviderDescription(model.provider) }}</p>
                  </div>
                </div>
                <div class="mt-4 flex items-center gap-2 text-xs text-gray-400">
                  <span class="rounded-full bg-gray-100 px-2 py-1">{{ getCategoryLabel(model.category) }}</span>
                  <span class="rounded-full bg-green-100 px-2 py-1 text-green-600">{{ t('admin.models.status.available') }}</span>
                </div>
                <div v-if="modelPricing[model.name]" class="mt-3 flex flex-wrap gap-3 text-xs">
                  <div class="flex items-center gap-1">
                    <span class="text-gray-500">{{ t('admin.models.pricing.input') }}:</span>
                    <span class="font-medium text-gray-700">{{ formatPrice(modelPricing[model.name].input_price) }}</span>
                  </div>
                  <div class="flex items-center gap-1">
                    <span class="text-gray-500">{{ t('admin.models.pricing.output') }}:</span>
                    <span class="font-medium text-gray-700">{{ formatPrice(modelPricing[model.name].output_price) }}</span>
                  </div>
                  <div class="flex items-center gap-1">
                    <span class="text-gray-500">{{ t('admin.models.pricing.approx') }}:</span>
                    <span class="font-medium text-gray-700">1$={{ modelUsdTokenRates[model.name] || '-' }}Tokens</span>
                  </div>
                </div>
              </template>
            </div>
          </div>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { getModelDefaultPricing } from '@/api/admin/channels'
import { perTokenToMTok } from '@/components/admin/channel/types'

const { t, locale } = useI18n()

const copiedModel = ref<string | null>(null)

interface Model {
  id: string
  name: string
  provider: string
  vendor: string
  category: string
  icon: string
}

interface ModelPricing {
  input_price?: number | null
  output_price?: number | null
  cache_write_price?: number | null
  cache_read_price?: number | null
}

const modelPricing = ref<Record<string, ModelPricing>>({})

const fetchModelPricing = async (modelName: string) => {
  try {
    const result = await getModelDefaultPricing(modelName)
    if (result.found) {
      modelPricing.value[modelName] = {
        input_price: perTokenToMTok(result.input_price),
        output_price: perTokenToMTok(result.output_price),
        cache_write_price: perTokenToMTok(result.cache_write_price),
        cache_read_price: perTokenToMTok(result.cache_read_price),
      }
    }
  } catch (error) {
    console.error(`Failed to fetch pricing for ${modelName}:`, error)
  }
}

onMounted(() => {
  models.value.forEach(model => {
    if (model.name && model.category !== 'hint') {
      fetchModelPricing(model.name)
    }
  })
})

const formatPrice = (price: number | null | undefined): string => {
  if (price === null || price === undefined) return '-'
  return `$${price.toFixed(2)}/MTokens`
}

const modelUsdTokenRates: Record<string, string> = {
  'deepseek-v4-pro': '29.49M',
  'kimi-k2.7': '6.18M',
  'minimax-m3': '3.40M',
  'qwen3.7-max': '3.06M',
  'glm-5.2': '3.35M',
  'seedance-2.0': '-',
  'gpt-image-2': '-',
}

const providerDescriptions: Record<string, { en: string; zh: string }> = {
  'deepseek-v4-pro': {
    en: 'DeepSeek V4 is a cutting-edge MoE-based flagship model, excelling in coding, reasoning, and long-context tasks with robust tool-use capabilities for complex workflows.',
    zh: 'DeepSeek V4 是基于 MoE 架构的前沿旗舰模型，在编码、推理和长上下文任务中表现出色，具备强大的工具调用能力。'
  },
  'minimax-m3': {
    en: 'MiniMax‑M3 is a frontier open‑weight model with 1M context, native multimodality, and top coding/agent abilities, built on the MSA sparse attention architecture.',
    zh: 'MiniMax-M3 是前沿开源权重模型，拥有 100 万上下文、原生多模态和顶级编码/智能体能力，基于 MSA 稀疏注意力架构。'
  },
  'kimi-k2.7': {
    en: 'Kimi-K2.6 is Moonshot\'s open MoE flagship with 256K context, excelling in long-horizon coding and agent swarm (300 sub-agents) for complex, multi-step tasks.',
    zh: 'Kimi-K2.6 是月之暗面的开源 MoE 旗舰模型，拥有 256K 上下文，擅长长期编码和智能体集群（300 个子智能体）处理复杂多步骤任务。'
  },
  'qwen3.7-max': {
    en: 'Qwen3.7-Max is Alibaba\'s agent‑centric flagship with 1M context, top-tier coding, excelling in complex workflows and multi-framework generalization.',
    zh: 'Qwen3.7-Max 是阿里的智能体旗舰模型，拥有 100 万上下文、顶级编码能力和擅长复杂工作流和多框架泛化。'
  },
  'glm-5.2': {
    en: 'GLM-5.1 is Zhipu AI\'s open MoE flagship with 200K context, excelling in 8‑hour autonomous agentic coding and topping SWE‑Bench Pro for complex software engineering tasks.',
    zh: 'GLM-5.1 是智谱 AI 的开源 MoE 旗舰模型，拥有 200K 上下文，擅长 8 小时自主智能体编码，在 SWE-Bench Pro 复杂软件工程任务中排名第一。'
  },
  'seedance-2.0': {
    en: 'Contact support via ticket after recharge. Premium video models require dedicated service.',
    zh: '充值后通过工单联系使用，好的视频模型就要专人服务。'
  },
  'gpt-image-2': {
    en: 'GPT-Image-2 (ChatGPT Images 2.0), launched by OpenAI in April 2026, is a flagship image model with reasoning, accurate Chinese rendering, high-res output and batch generation.',
    zh: 'GPT-Image-2（ChatGPT 图像 2.0）是 OpenAI 于 2026 年 4 月发布的旗舰图像模型，具备推理能力、精准的中文渲染、高分辨率输出和批量生成功能。'
  }
}

const models = ref<Model[]>([
  { id: '1', name: 'deepseek-v4-pro', provider: 'deepseek-v4-pro', vendor: 'deepseek', category: 'text', icon: '' },
  { id: '2', name: 'minimax-m3', provider: 'minimax-m3', vendor: 'minimax', category: 'text', icon: '' },
  { id: '3', name: 'kimi-k2.7', provider: 'kimi-k2.7', vendor: 'moonshot', category: 'text', icon: '' },
  { id: '4', name: 'qwen3.7-max', provider: 'qwen3.7-max', vendor: 'alibaba', category: 'text', icon: '' },
  { id: '5', name: 'glm-5.2', provider: 'glm-5.2', vendor: 'zhipu', category: 'text', icon: '' },
  { id: '6', name: 'seedance-2.0', provider: 'seedance-2.0', vendor: 'bytedance', category: 'multimodal', icon: '' },
  { id: '8', name: 'gpt-image-2', provider: 'gpt-image-2', vendor: 'openai', category: 'image', icon: '' },
  { id: '9', name: '', provider: '', vendor: 'hint', category: 'hint', icon: '' },
])

const getProviderDescription = (providerKey: string) => {
  const desc = providerDescriptions[providerKey]
  if (!desc) return ''
  return locale.value === 'zh' ? desc.zh : desc.en
}

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
  minimax: {
    gradient: 'bg-gradient-to-br from-amber-500 to-yellow-500',
    icon: 'M13 10V3L4 14h7v7l9-11h-7z'
  },
  alibaba: {
    gradient: 'bg-gradient-to-br from-orange-500 to-red-500',
    icon: 'M3.055 11H5a2 2 0 012 2v1a2 2 0 002 2 2 2 0 012 2v2.945M8 3.935V5.5A2.5 2.5 0 0010.5 8h.5a2 2 0 012 2 2 2 0 104 0 2 2 0 012-2h1.064M15 20.488V18a2 2 0 012-2h3.064M21 12a9 9 0 11-18 0 9 9 0 0118 0z'
  },
  default: {
    gradient: 'bg-gradient-to-br from-purple-500 to-blue-500',
    icon: 'M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5'
  },
  hint: {
    gradient: 'bg-gradient-to-br from-blue-400 to-indigo-500',
    icon: 'M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z'
  }
}

const getProviderStyle = (vendor: string) => {
  return providerStyles[vendor] || providerStyles.default
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