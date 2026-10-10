<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex items-center justify-between">
        <div>
          <h1 class="text-3xl font-bold text-gray-900">{{ t('admin.models.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500">{{ t('admin.models.description') }}</p>
        </div>
      </div>

      <div
        v-if="!authStore.isAuthenticated"
        class="flex flex-col items-start justify-between gap-3 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 sm:flex-row sm:items-center"
      >
        <div class="flex items-center gap-2 text-sm text-amber-800">
          <svg class="h-5 w-5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
          </svg>
          <span>{{ t('admin.models.loginHint') }}</span>
        </div>
        <RouterLink
          to="/login"
          class="shrink-0 rounded-lg bg-amber-500 px-4 py-1.5 text-sm font-medium text-white transition-colors hover:bg-amber-600"
        >
          {{ t('admin.models.loginAction') }}
        </RouterLink>
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
                    <PlatformIcon
                      v-if="vendorIconPlatforms[model.vendor]"
                      :platform="vendorIconPlatforms[model.vendor]"
                      size="lg"
                      class="text-white"
                    />
                    <svg v-else class="h-5 w-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
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
                    <p class="mt-1 text-sm text-gray-500">{{ getProviderDescription(model) }}</p>
                  </div>
                </div>
                <div class="mt-4 flex items-center gap-2 text-xs text-gray-400">
                  <span class="rounded-full bg-gray-100 px-2 py-1">{{ getCategoryLabel(model.category) }}</span>
                  <span class="rounded-full bg-green-100 px-2 py-1 text-green-600">{{ t('admin.models.status.available') }}</span>
                </div>
                <div v-if="model.category !== 'hint'" class="mt-3 flex flex-wrap gap-3 text-xs">
                  <!-- Channel pricing (from /admin/channels/pricing), lowest price across channels -->
                  <template v-if="model.price">
                    <template v-if="model.price.kind === 'token'">
                      <div class="flex items-center gap-1">
                        <span class="text-gray-500">{{ t('admin.models.pricing.input') }}:</span>
                        <span class="font-medium text-gray-700">{{ formatPrice(model.price.input) }}</span>
                      </div>
                      <div class="flex items-center gap-1">
                        <span class="text-gray-500">{{ t('admin.models.pricing.output') }}:</span>
                        <span class="font-medium text-gray-700">{{ formatPrice(model.price.output) }}</span>
                      </div>
                      <div class="flex items-center gap-1">
                        <span class="text-gray-500">{{ t('admin.models.pricing.approx') }}:</span>
                        <span class="font-medium text-gray-700">1$≈{{ model.price.rate || '-' }}Tokens</span>
                      </div>
                    </template>
                    <template v-else>
                      <span class="text-gray-500">{{ t('admin.models.pricing.approx') }}:</span>
                      <span class="font-medium text-gray-700">{{ formatUnitPrice(model.price.price) }}$ {{ unitLabel(model.price.kind) }}</span>
                    </template>
                  </template>
                  <!-- Fallback: reference pricing -->
                  <template v-else-if="model.category === 'image'">
                    <span class="text-gray-500">{{ t('admin.models.pricing.approx') }}:</span>
                    <span class="font-medium text-gray-700">0.3$ per image</span>
                  </template>
                  <template v-else-if="model.category === 'multimodal'">
                    <span class="text-gray-500">{{ t('admin.models.pricing.approx') }}:</span>
                    <span class="font-medium text-gray-700">0.9$ per second</span>
                  </template>
                  <template v-else>
                    <!-- Dual pricing: 1.5折 and 9折 -->
                    <template v-if="getDualPricing(model.name)">
                      <div class="w-full space-y-1.5">
                        <div class="flex items-center justify-between">
                          <span class="rounded bg-red-100 px-1.5 py-0.5 text-[10px] font-medium text-red-600">1.5折</span>
                          <span class="text-gray-600 text-xs">
                            <span class="text-gray-500">{{ t('admin.models.pricing.input') }}:</span>
                            <span class="font-medium">{{ formatPrice(getDualPricing(model.name)!.tier15.input) }}</span>
                            <span class="mx-1 text-gray-400"> </span>
                            <span class="text-gray-500">{{ t('admin.models.pricing.output') }}:</span>
                            <span class="font-medium">{{ formatPrice(getDualPricing(model.name)!.tier15.output) }}</span>
                          </span>
                        </div>
                        <div class="flex items-center justify-between">
                          <span class="rounded bg-blue-100 px-1.5 py-0.5 text-[10px] font-medium text-blue-600">9折</span>
                          <span class="text-gray-600 text-xs">
                            <span class="text-gray-500">{{ t('admin.models.pricing.input') }}:</span>
                            <span class="font-medium">{{ formatPrice(getDualPricing(model.name)!.tier90.input) }}</span>
                            <span class="mx-1 text-gray-400"> </span>
                            <span class="text-gray-500">{{ t('admin.models.pricing.output') }}:</span>
                            <span class="font-medium">{{ formatPrice(getDualPricing(model.name)!.tier90.output) }}</span>
                          </span>
                        </div>
                      </div>
                    </template>
                    <!-- Standard pricing -->
                    <template v-else>
                      <div class="flex items-center gap-1">
                        <span class="text-gray-500">{{ t('admin.models.pricing.input') }}:</span>
                        <span class="font-medium text-gray-700">{{ formatPrice(modelPricing[model.name]?.input_price ?? fallbackPriceFromRate(modelUsdTokenRates[model.name])) }}</span>
                      </div>
                      <div class="flex items-center gap-1">
                        <span class="text-gray-500">{{ t('admin.models.pricing.output') }}:</span>
                        <span class="font-medium text-gray-700">{{ formatPrice(modelPricing[model.name]?.output_price ?? fallbackPriceFromRate(modelUsdTokenRates[model.name])) }}</span>
                      </div>
                      <div class="flex items-center gap-1">
                        <span class="text-gray-500">{{ t('admin.models.pricing.approx') }}:</span>
                        <span class="font-medium text-gray-700">1$≈{{ modelUsdTokenRates[model.name] || '-' }}Tokens</span>
                      </div>
                    </template>
                  </template>
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
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import PlatformIcon, { type VendorIconPlatform } from '@/components/common/PlatformIcon.vue'
import { perTokenToMTok } from '@/components/admin/channel/types'
import { useAuthStore } from '@/stores/auth'
import { apiClient } from '@/api/client'
import channelsAPI from '@/api/admin/channels'
import type { BillingMode, ChannelModelPricing } from '@/api/admin/channels'

const { t, locale } = useI18n()
const authStore = useAuthStore()

const copiedModel = ref<string | null>(null)

interface Model {
  id: string
  name: string
  provider: string
  vendor: string
  category: string
  icon: string
  price?: ChannelPriceView
}

/** Price view resolved from channel pricing (/admin/channels/pricing). */
type ChannelPriceView =
  | { kind: 'token'; input: number | null; output: number | null; rate: string | null }
  | { kind: 'per_request'; price: number | null }
  | { kind: 'image'; price: number | null }
  | { kind: 'video'; price: number | null }

/** Aggregated channel pricing for one model (lowest across all channels). */
interface ChannelAgg {
  mode: BillingMode
  platform: string
  input: number | null      // USD per MTokens
  output: number | null     // USD per MTokens
  perRequest: number | null // USD per request / image / second
}

interface ModelPricing {
  input_price?: number | null
  output_price?: number | null
  cache_write_price?: number | null
  cache_read_price?: number | null
}

const modelPricing = ref<Record<string, ModelPricing>>({})

// Claude/OpenAI 新模型仅在非 threerouter 域名时显示
const isThreerouterDomain = computed(() => {
  return typeof window !== 'undefined' && window.location.hostname.includes('threerouter')
})

const fetchModelPricing = async (modelName: string) => {
  try {
    // Use public endpoint when not authenticated, admin endpoint when authenticated
    const endpoint = authStore.isAuthenticated ? '/admin/channels/model-pricing' : '/models/pricing'
    const { data: result } = await apiClient.get(endpoint, { params: { model: modelName } })
    if (result.found) {
      modelPricing.value[modelName] = {
        input_price: perTokenToMTok(result.input_price),
        output_price: perTokenToMTok(result.output_price),
        cache_write_price: perTokenToMTok(result.cache_write_price),
        cache_read_price: perTokenToMTok(result.cache_read_price),
      }
    }
  } catch (error) {
    // Silently ignore pricing fetch errors
  }
}

// Channel pricing state: exact model name → aggregated price, plus wildcard patterns
const channelExactPrices = ref<Record<string, ChannelAgg>>({})
const channelWildcardPrices = ref<{ prefix: string; agg: ChannelAgg }[]>([])

const minNum = (a: number | null, b: number | null): number | null => {
  if (b === null) return a
  if (a === null) return b
  return Math.min(a, b)
}

// Fetch all channels and aggregate model pricing (lowest price wins across channels)
const fetchChannelPrices = async () => {
  if (!authStore.isAuthenticated) return
  try {
    const { items } = await channelsAPI.list(1, 1000)
    const exact: Record<string, ChannelAgg> = {}
    const wild: { prefix: string; agg: ChannelAgg }[] = []

    const entryAgg = (entry: ChannelModelPricing): ChannelAgg | null => {
      if (entry.billing_mode === 'token') {
        const input = entry.input_price != null ? perTokenToMTok(entry.input_price) : null
        const output = entry.output_price != null ? perTokenToMTok(entry.output_price) : null
        if (input === null && output === null) return null
        return { mode: 'token', platform: entry.platform, input, output, perRequest: null }
      }
      // per_request / image / video: unit price stored in per_request_price
      let perRequest = entry.per_request_price
      if (perRequest == null && entry.intervals && entry.intervals.length > 0) {
        const vals = entry.intervals
          .map(iv => iv.per_request_price)
          .filter((v): v is number => v != null && v > 0)
        if (vals.length > 0) perRequest = Math.min(...vals)
      }
      if (perRequest == null) return null
      return { mode: entry.billing_mode, platform: entry.platform, input: null, output: null, perRequest }
    }

    const mergeAgg = (cur: ChannelAgg, next: ChannelAgg): ChannelAgg => ({
      mode: cur.mode,
      platform: cur.platform,
      input: minNum(cur.input, next.input),
      output: minNum(cur.output, next.output),
      perRequest: minNum(cur.perRequest, next.perRequest),
    })

    for (const ch of items) {
      for (const entry of ch.model_pricing || []) {
        const agg = entryAgg(entry)
        if (!agg) continue
        for (const rawName of entry.models || []) {
          let name = String(rawName).trim().toLowerCase()
          // Normalize vendor-prefixed names, e.g. "deepseek-ai/deepseek-v4-pro" -> "deepseek-v4-pro",
          // so they match the same card instead of creating duplicates
          const slash = name.lastIndexOf('/')
          if (slash >= 0) name = name.slice(slash + 1)
          if (!name) continue
          if (name.endsWith('*')) {
            const prefix = name.slice(0, -1)
            if (!prefix) continue // bare '*' is a platform-level default, skip
            const found = wild.find(w => w.prefix === prefix)
            if (found) found.agg = mergeAgg(found.agg, agg)
            else wild.push({ prefix, agg })
          } else {
            exact[name] = exact[name] ? mergeAgg(exact[name], agg) : agg
          }
        }
      }
    }
    channelExactPrices.value = exact
    channelWildcardPrices.value = wild
  } catch (error) {
    // Silently ignore channel pricing fetch errors
  }
}

const getChannelAgg = (modelName: string): ChannelAgg | null => {
  const key = modelName.toLowerCase()
  const matches: ChannelAgg[] = []
  const exact = channelExactPrices.value[key]
  if (exact) matches.push(exact)
  for (const w of channelWildcardPrices.value) {
    if (key.startsWith(w.prefix)) matches.push(w.agg)
  }
  if (matches.length === 0) return null
  return matches.reduce((acc, cur) => ({
    mode: acc.mode,
    platform: acc.platform,
    input: minNum(acc.input, cur.input),
    output: minNum(acc.output, cur.output),
    perRequest: minNum(acc.perRequest, cur.perRequest),
  }))
}

const buildPriceView = (agg: ChannelAgg): ChannelPriceView => {
  if (agg.mode === 'token') {
    const p = agg.input ?? agg.output
    const rate = p !== null && p > 0 ? `${(1 / p).toFixed(2)}M` : null
    return { kind: 'token', input: agg.input, output: agg.output, rate }
  }
  if (agg.mode === 'image') return { kind: 'image', price: agg.perRequest }
  if (agg.mode === 'video') return { kind: 'video', price: agg.perRequest }
  return { kind: 'per_request', price: agg.perRequest }
}

onMounted(() => {
  fetchChannelPrices()
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

const formatUnitPrice = (price: number | null | undefined): string => {
  if (price === null || price === undefined) return '-'
  return String(parseFloat(price.toPrecision(4)))
}

const unitLabel = (kind: 'per_request' | 'image' | 'video'): string => {
  if (kind === 'image') return 'per image'
  if (kind === 'video') return 'per second'
  return 'per request'
}

const fallbackPriceFromRate = (rate: string | undefined): number | undefined => {
  if (!rate || rate === '-') return undefined
  const match = rate.match(/^([\d.]+)M$/)
  if (!match) return undefined
  const tokensPerDollar = parseFloat(match[1])
  if (tokensPerDollar <= 0) return undefined
  return 1 / tokensPerDollar
}

const modelUsdTokenRates: Record<string, string> = {
  'deepseek-v4-pro': '29.49M',
  'kimi-k3': '6.18M',
  'minimax-m3': '3.40M',
  'qwen3.8-max': '3.06M',
  'glm-5.3': '3.35M',
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
  'kimi-k3': {
    en: 'Kimi-K3 is Moonshot\'s open MoE flagship with 256K context, excelling in long-horizon coding and agent swarm (300 sub-agents) for complex, multi-step tasks.',
    zh: 'Kimi-K3 是月之暗面的开源 MoE 旗舰模型，拥有 256K 上下文，擅长长期编码和智能体集群（300 个子智能体）处理复杂多步骤任务。'
  },
  'qwen3.8-max': {
    en: 'Qwen3.8-Max is Alibaba\'s agent‑centric flagship with 1M context, top-tier coding, excelling in complex workflows and multi-framework generalization.',
    zh: 'Qwen3.8-Max 是阿里的智能体旗舰模型，拥有 100 万上下文、顶级编码能力和擅长复杂工作流和多框架泛化。'
  },
  'glm-5.3': {
    en: 'GLM-5.3 is Zhipu AI\'s open MoE flagship with 200K context, excelling in 8‑hour autonomous agentic coding and topping SWE‑Bench Pro for complex software engineering tasks.',
    zh: 'GLM-5.3 是智谱 AI 的开源 MoE 旗舰模型，拥有 200K 上下文，擅长 8 小时自主智能体编码，在 SWE-Bench Pro 复杂软件工程任务中排名第一。'
  },
  'seedance-2.0': {
    en: 'Seedance 2.0 accepts image, video, audio and text inputs, and generates, edits and extends videos with high-fidelity detail reproduction and stable character consistency — giving users director-level control.',
    zh: 'Seedance 2.0 支持图像、视频、音频、文本等多模态输入，具备视频生成、编辑与延长能力，可高精度还原物品细节、音色、风格与运镜，保持稳定角色特征，赋予使用者如同导演般的掌控权。'
  },
  'gpt-image-2': {
    en: 'GPT-Image-2 (ChatGPT Images 2.0), launched by OpenAI in April 2026, is a flagship image model with reasoning, accurate Chinese rendering, high-res output and batch generation.',
    zh: 'GPT-Image-2（ChatGPT 图像 2.0）是 OpenAI 于 2026 年 4 月发布的旗舰图像模型，具备推理能力、精准的中文渲染、高分辨率输出和批量生成功能。'
  },
  'claude-haiku-4-5-20251001': {
    en: 'Claude Haiku 4.5 is Anthropic\'s fast and affordable model, ideal for high-volume tasks with near-instant response times and strong tool-use capabilities.',
    zh: 'Claude Haiku 4.5 是 Anthropic 的快速高性价比模型，适合高频率任务，响应接近即时，具备强大的工具调用能力。'
  },
  'claude-opus-4-5-20251101': {
    en: 'Claude Opus 4.5 is Anthropic\'s flagship model for complex reasoning, deep analysis, and long-context understanding, excelling in coding and multi-step agentic workflows.',
    zh: 'Claude Opus 4.5 是 Anthropic 的旗舰模型，擅长复杂推理、深度分析和长上下文理解，在编码和多步骤智能体工作流中表现出色。'
  },
  'claude-opus-4-6': {
    en: 'Claude Opus 4.6 builds on 4.5 with improved reasoning, faster inference, and enhanced code generation for enterprise-grade agentic tasks.',
    zh: 'Claude Opus 4.6 在 4.5 基础上提升推理能力、推理速度和代码生成能力，适用于企业级智能体任务。'
  },
  'claude-opus-4-7': {
    en: 'Claude Opus 4.7 further refines reasoning depth and tool-use accuracy, setting new benchmarks in coding and complex problem-solving.',
    zh: 'Claude Opus 4.7 进一步提升推理深度和工具调用精度，在编码和复杂问题解决方面树立新标杆。'
  },
  'claude-opus-4-8': {
    en: 'Claude Opus 4.8 is the latest Opus iteration with state-of-the-art reasoning, expanded context handling, and superior agentic coding performance.',
    zh: 'Claude Opus 4.8 是最新的 Opus 迭代版本，具备最先进的推理能力、扩展的上下文处理和卓越的智能体编码性能。'
  },
  'claude-opus-5': {
    en: 'Claude Opus 5 is Anthropic\'s next-generation flagship, redefining the frontier of reasoning, creativity, and autonomous task completion.',
    zh: 'Claude Opus 5 是 Anthropic 的下一代旗舰模型，重新定义推理、创造力和自主任务完成的前沿。'
  },
  'claude-sonnet-4-6': {
    en: 'Claude Sonnet 4.6 balances performance and cost with strong coding, analysis, and vision capabilities, ideal for production-scale applications.',
    zh: 'Claude Sonnet 4.6 在性能和成本之间取得平衡，具备强大的编码、分析和视觉能力，适合生产级应用。'
  },
  'claude-sonnet-5': {
    en: 'Claude Sonnet 5 delivers flagship-level reasoning at mid-tier pricing, with enhanced multi-modal understanding and agentic workflow support.',
    zh: 'Claude Sonnet 5 以中端价格提供旗舰级推理能力，增强多模态理解和智能体工作流支持。'
  },
  'claude-fable-5': {
    en: 'Claude Fable 5 is Anthropic\'s premium creative model, specialized in long-form writing, storytelling, and complex creative tasks with exceptional quality.',
    zh: 'Claude Fable 5 是 Anthropic 的旗舰创意模型，专注于长篇写作、叙事和复杂创意任务，质量卓越。'
  },
  'gpt-5.4': {
    en: 'GPT-5.4 is OpenAI\'s production workhorse with 272K context, strong reasoning and function calling, ideal for general-purpose AI applications.',
    zh: 'GPT-5.4 是 OpenAI 的生产级模型，拥有 272K 上下文，推理和函数调用能力强，适合通用 AI 应用。'
  },
  'gpt-5.5': {
    en: 'GPT-5.5 enhances 5.4 with improved multi-modal understanding, faster inference, and better tool orchestration for complex workflows.',
    zh: 'GPT-5.5 在 5.4 基础上增强多模态理解、推理速度和工具编排能力，适用于复杂工作流。'
  },
  'gpt-5.6-luna': {
    en: 'GPT-5.6 Luna is OpenAI\'s fastest and most affordable model, recently price-cut by 80%, perfect for high-volume, high-frequency tasks.',
    zh: 'GPT-5.6 Luna 是 OpenAI 最快、最具性价比的模型，近期降价 80%，适合大规模、高频率任务。'
  },
  'gpt-5.6-terra': {
    en: 'GPT-5.6 Terra is the balanced mid-tier model with strong reasoning at reduced cost, ideal for everyday enterprise workloads.',
    zh: 'GPT-5.6 Terra 是均衡型中端模型，推理强且成本优化，适合日常企业工作负载。'
  },
  'gpt-5.6-sol': {
    en: 'GPT-5.6 Sol is OpenAI\'s flagship model with the deepest reasoning and best quality, featuring a new Fast mode with 2.5x speed improvement.',
    zh: 'GPT-5.6 Sol 是 OpenAI 的旗舰模型，推理最深、质量最佳，新增 Fast 模式速度提升 2.5 倍。'
  }
}

// Per-vendor fallback descriptions for channel models without a dedicated entry.
// Sourced from each vendor's official website (about/mission statements).
const vendorDescriptions: Record<string, { en: string; zh: string }> = {
  deepseek: {
    en: 'DeepSeek focuses on breakthroughs in large language models and reasoning, making world-class AGI affordable for everyone through efficient architectures and an open ecosystem.',
    zh: '深度求索（DeepSeek）专注于大语言模型与推理能力的底层突破，以更高效的架构和开放的生态，让每个人都能低成本使用世界一流的通用人工智能。'
  },
  zhipu: {
    en: 'Z.ai (Zhipu), spun off from Tsinghua University KEG, pursues the vision of "letting machines think like humans". GLM models are built for complex software engineering and long-horizon agent tasks with up to 1M context.',
    zh: '智谱（Z.ai）源自清华大学技术成果转化，以“让机器像人一样思考”为愿景。GLM 系列面向复杂软件工程与长程智能体任务，支持 1M 上下文。'
  },
  moonshot: {
    en: 'Moonshot AI seeks the optimal way to convert energy into intelligence. Kimi models are natively multimodal with 1M-token context, built for long-horizon coding, knowledge work, and deep reasoning.',
    zh: '月之暗面（Moonshot AI）以“寻求将能源转化为智能的最优解”为愿景。Kimi 系列原生多模态、支持 1M 上下文，面向长程编码、知识工作与深度推理。'
  },
  alibaba: {
    en: 'Qwen is Alibaba Tongyi Lab\'s model family spanning language, coding, reasoning and multimodal models — full-size, multimodal, and widely open-sourced.',
    zh: '通义千问（Qwen）是阿里巴巴通义实验室的大模型家族，覆盖大语言、编程、推理与多模态模型，全尺寸、多模态、广开源。'
  },
  minimax: {
    en: 'MiniMax is a global AI foundation model company with the mission "Intelligence with Everyone", building multimodal models with strong coding, agentic and ultra-long-context capabilities.',
    zh: 'MiniMax 是全球领先的通用人工智能科技公司，以“与所有人共创智能”为使命，自研多模态大模型具备强大的代码与 Agent 能力及超长上下文处理能力。'
  },
  bytedance: {
    en: 'Doubao is ByteDance\'s model family — flagship agent-grade general models for production tasks, with upgraded coding, agent and multimodal capabilities.',
    zh: '豆包（Doubao）是字节跳动的大模型家族，旗舰级 Agent 通用模型面向生产级任务，全面升级编程、智能体与多模态能力。'
  },
  anthropic: {
    en: 'Anthropic is an AI safety and research company building reliable, interpretable, and steerable AI systems (the Claude family).',
    zh: 'Anthropic 是一家专注 AI 安全的研究公司，致力于构建可靠、可解释、可操控的 AI 系统（Claude 系列）。'
  },
  openai: {
    en: 'OpenAI\'s mission is to ensure that artificial general intelligence (AGI) benefits all of humanity; GPT is its flagship general-purpose model family.',
    zh: 'OpenAI 的使命是确保通用人工智能（AGI）造福全人类；GPT 系列是其面向通用任务的旗舰模型家族。'
  }
}

// Dual pricing: 1.5折 (15%) and 9折 (90%) of official price
// Official prices sourced from https://api.huanxing.ai/pricing
interface DualPricing {
  officialInput: number  // USD per MTokens
  officialOutput: number // USD per MTokens
}
const dualPricingModels: Record<string, DualPricing> = {
  // Claude models (CNY → USD at ~7 CNY/USD)
  'claude-haiku-4-5-20251001': { officialInput: 1.00, officialOutput: 5.00 },
  'claude-opus-4-5-20251101': { officialInput: 5.00, officialOutput: 25.00 },
  'claude-opus-4-6': { officialInput: 5.00, officialOutput: 25.00 },
  'claude-opus-4-7': { officialInput: 5.00, officialOutput: 25.00 },
  'claude-opus-4-8': { officialInput: 5.00, officialOutput: 25.00 },
  'claude-opus-5': { officialInput: 5.00, officialOutput: 25.00 },
  'claude-sonnet-4-6': { officialInput: 3.00, officialOutput: 15.00 },
  'claude-sonnet-5': { officialInput: 2.00, officialOutput: 10.00 },
  'claude-fable-5': { officialInput: 10.00, officialOutput: 50.00 },
  // OpenAI GPT >5.4 models (official USD pricing)
  'gpt-5.4': { officialInput: 2.50, officialOutput: 10.00 },
  'gpt-5.5': { officialInput: 2.50, officialOutput: 15.00 },
  'gpt-5.6-luna': { officialInput: 1.00, officialOutput: 6.00 },
  'gpt-5.6-terra': { officialInput: 2.50, officialOutput: 15.00 },
  'gpt-5.6-sol': { officialInput: 5.00, officialOutput: 30.00 },
}

const getDualPricing = (modelName: string): { tier15: { input: number; output: number }; tier90: { input: number; output: number } } | null => {
  const pricing = dualPricingModels[modelName]
  if (!pricing) return null
  return {
    tier15: { input: pricing.officialInput * 0.15, output: pricing.officialOutput * 0.15 },
    tier90: { input: pricing.officialInput * 0.90, output: pricing.officialOutput * 0.90 },
  }
}

// Platform (from channel pricing) → vendor key used by providerStyles
const platformVendors: Record<string, string> = {
  anthropic: 'anthropic',
  openai: 'openai',
  gemini: 'default',
  antigravity: 'default',
  grok: 'default',
  kimi: 'moonshot',
  zhipu: 'zhipu',
  deepseek: 'deepseek',
  moonshot: 'moonshot',
  minimax: 'minimax',
  bytedance: 'bytedance',
  alibaba: 'alibaba',
  qwen: 'alibaba',
}

const billingModeCategory = (mode: BillingMode): string => {
  if (mode === 'image') return 'image'
  if (mode === 'video') return 'multimodal'
  return 'text'
}

// Infer the real vendor from the model name itself, e.g. "glm-5.2" served via
// an OpenAI-compatible channel should still show the Zhipu logo.
const modelNameVendorRules: Array<[RegExp, string]> = [
  [/deepseek/, 'deepseek'],
  [/glm/, 'zhipu'],
  [/(kimi|moonshot)/, 'moonshot'],
  [/(qwen|tongyi)/, 'alibaba'],
  [/(minimax|abab)/, 'minimax'],
  [/(doubao|seedance)/, 'bytedance'],
  [/claude/, 'anthropic'],
  [/(gemini|gemma)/, 'gemini'],
  [/^(gpt|chatgpt|o\d(-|$)|dall-e|whisper|sora)/, 'openai'],
]

const inferVendorFromModelName = (name: string): string | null => {
  const n = name.trim().toLowerCase()
  for (const [pattern, vendor] of modelNameVendorRules) {
    if (pattern.test(n)) return vendor
  }
  return null
}

const models = computed<Model[]>(() => {
  const base: Model[] = [
    { id: '1', name: 'deepseek-v4-pro', provider: 'deepseek-v4-pro', vendor: 'deepseek', category: 'text', icon: '' },
    { id: '2', name: 'minimax-m3', provider: 'minimax-m3', vendor: 'minimax', category: 'text', icon: '' },
    { id: '3', name: 'kimi-k3', provider: 'kimi-k3', vendor: 'moonshot', category: 'text', icon: '' },
    { id: '4', name: 'qwen3.8-max', provider: 'qwen3.8-max', vendor: 'alibaba', category: 'text', icon: '' },
    { id: '5', name: 'glm-5.3', provider: 'glm-5.3', vendor: 'zhipu', category: 'text', icon: '' },
    { id: '6', name: 'seedance-2.0', provider: 'seedance-2.0', vendor: 'bytedance', category: 'multimodal', icon: '' },
    { id: '8', name: 'gpt-image-2', provider: 'gpt-image-2', vendor: 'openai', category: 'image', icon: '' },
  ]

  // Claude >4.5 和 OpenAI GPT >5.4 仅在非 threerouter 域名时显示
  if (!isThreerouterDomain.value) {
    base.push(
      // Claude >4.5 (高版本在前)
      { id: '18', name: 'claude-fable-5', provider: 'claude-fable-5', vendor: 'anthropic', category: 'text', icon: '' },
      { id: '15', name: 'claude-opus-5', provider: 'claude-opus-5', vendor: 'anthropic', category: 'text', icon: '' },
      { id: '17', name: 'claude-sonnet-5', provider: 'claude-sonnet-5', vendor: 'anthropic', category: 'text', icon: '' },
      { id: '14', name: 'claude-opus-4-8', provider: 'claude-opus-4-8', vendor: 'anthropic', category: 'text', icon: '' },
      { id: '13', name: 'claude-opus-4-7', provider: 'claude-opus-4-7', vendor: 'anthropic', category: 'text', icon: '' },
      { id: '12', name: 'claude-opus-4-6', provider: 'claude-opus-4-6', vendor: 'anthropic', category: 'text', icon: '' },
      { id: '16', name: 'claude-sonnet-4-6', provider: 'claude-sonnet-4-6', vendor: 'anthropic', category: 'text', icon: '' },
      { id: '11', name: 'claude-opus-4-5-20251101', provider: 'claude-opus-4-5-20251101', vendor: 'anthropic', category: 'text', icon: '' },
      { id: '10', name: 'claude-haiku-4-5-20251001', provider: 'claude-haiku-4-5-20251001', vendor: 'anthropic', category: 'text', icon: '' },
      // OpenAI GPT >5.4 (高版本在前)
      { id: '23', name: 'gpt-5.6-sol', provider: 'gpt-5.6-sol', vendor: 'openai', category: 'text', icon: '' },
      { id: '22', name: 'gpt-5.6-terra', provider: 'gpt-5.6-terra', vendor: 'openai', category: 'text', icon: '' },
      { id: '21', name: 'gpt-5.6-luna', provider: 'gpt-5.6-luna', vendor: 'openai', category: 'text', icon: '' },
      { id: '20', name: 'gpt-5.5', provider: 'gpt-5.5', vendor: 'openai', category: 'text', icon: '' },
      { id: '19', name: 'gpt-5.4', provider: 'gpt-5.4', vendor: 'openai', category: 'text', icon: '' },
    )
  }

  // Attach channel pricing (lowest across channels) to known models
  for (const m of base) {
    if (m.category === 'hint') continue
    const agg = getChannelAgg(m.name)
    if (agg) m.price = buildPriceView(agg)
  }

  // Append models configured in channel pricing that are not listed yet (keep all existing cards)
  const known = new Set(base.map(m => m.name.toLowerCase()))
  const channelOnly: Model[] = Object.entries(channelExactPrices.value)
    .filter(([name]) => name && !known.has(name))
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([name, agg]) => ({
      id: `ch:${name}`,
      name,
      provider: name,
      // Prefer the vendor inferred from the model name itself — channel models
      // are often served via OpenAI-compatible platforms, which would otherwise
      // show the same OpenAI logo for every card.
      vendor: inferVendorFromModelName(name) || platformVendors[agg.platform] || 'default',
      category: billingModeCategory(agg.mode),
      icon: '',
      price: buildPriceView(agg),
    }))
  base.push(...channelOnly)

  // Domestic (CN) vendors first, then the rest; sorted by model name within each group
  const cnVendors = new Set(['alibaba', 'bytedance', 'deepseek', 'minimax', 'moonshot', 'zhipu'])
  base.sort((a, b) => {
    const group = (v: string) => (cnVendors.has(v) ? 0 : 1)
    return group(a.vendor) - group(b.vendor) || a.name.localeCompare(b.name)
  })

  base.push({ id: '9', name: '', provider: '', vendor: 'hint', category: 'hint', icon: '' })
  return base
})

const getProviderDescription = (model: Pick<Model, 'provider' | 'vendor'>) => {
  const desc = providerDescriptions[model.provider] || vendorDescriptions[model.vendor]
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
  anthropic: {
    gradient: 'bg-gradient-to-br from-orange-500 to-amber-600',
    icon: 'M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8zm-1-13h2v6h-2zm0 8h2v2h-2z'
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

// Vendor key -> official logo platform rendered by PlatformIcon.
// Vendors without an official logo keep the generic heroicon fallback.
const vendorIconPlatforms: Record<string, VendorIconPlatform> = {
  anthropic: 'anthropic',
  openai: 'openai',
  deepseek: 'deepseek',
  zhipu: 'zhipu',
  moonshot: 'kimi',
  minimax: 'minimax',
  alibaba: 'qwen',
  bytedance: 'bytedance',
  gemini: 'gemini',
  grok: 'grok',
  antigravity: 'antigravity',
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