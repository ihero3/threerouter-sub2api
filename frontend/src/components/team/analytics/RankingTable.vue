<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

export interface RankingItem {
  name: string
  value: number
  change?: number
  secondaryValue?: string | number
  avatar?: string
}

type RankingType = 'department' | 'consumer' | 'model'

interface Props {
  title: string
  data: RankingItem[]
  type: RankingType
  loading?: boolean
  maxItems?: number
}

const props = withDefaults(defineProps<Props>(), {
  loading: false,
  maxItems: 10,
})

const emit = defineEmits<{
  (e: 'clickItem', item: RankingItem): void
}>()

const safeData = computed(() => (props.data ?? []).slice(0, props.maxItems))

const { t } = useI18n()

const typeConfig = computed<Record<RankingType, { icon: string; iconBg: string; valueLabel: string }>>(() => ({
  department: {
    icon: 'M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4',
    iconBg: 'bg-blue-100 text-blue-600 dark:bg-blue-900/30 dark:text-blue-400',
    valueLabel: t('team.analytics.usageAmount'),
  },
  consumer: {
    icon: 'M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z',
    iconBg: 'bg-violet-100 text-violet-600 dark:bg-violet-900/30 dark:text-violet-400',
    valueLabel: t('team.analytics.usageAmount'),
  },
  model: {
    icon: 'M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z',
    iconBg: 'bg-amber-100 text-amber-600 dark:bg-amber-900/30 dark:text-amber-400',
    valueLabel: t('team.analytics.callCount'),
  },
}))

const config = computed(() => typeConfig.value[props.type])

const getRankStyle = (index: number) => {
  if (index === 0) return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300 ring-2 ring-amber-200 dark:ring-amber-800'
  if (index === 1) return 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300 ring-2 ring-gray-200 dark:ring-gray-700'
  if (index === 2) return 'bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-300 ring-2 ring-orange-200 dark:ring-orange-800'
  return 'bg-gray-50 text-gray-500 dark:bg-dark-700 dark:text-gray-400'
}

const getBarColor = (index: number) => {
  if (index === 0) return 'bg-amber-500 dark:bg-amber-400'
  if (index === 1) return 'bg-gray-400 dark:bg-gray-500'
  if (index === 2) return 'bg-orange-400 dark:bg-orange-500'
  return 'bg-primary-400 dark:bg-primary-500'
}

const maxValue = computed(() => {
  if (!safeData.value.length) return 1
  return Math.max(...safeData.value.map((d) => d.value))
})

const formatNumber = (num: number): string => {
  if (num >= 1_000_000) return (num / 1_000_000).toFixed(1) + 'M'
  if (num >= 1_000) return (num / 1_000).toFixed(1) + 'K'
  return num.toLocaleString()
}
</script>

<template>
  <div
    class="overflow-hidden rounded-xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800"
  >
    <!-- Header -->
    <div
      class="flex items-center gap-3 border-b border-gray-200 px-5 py-4 dark:border-dark-700"
    >
      <div
        class="flex h-8 w-8 items-center justify-center rounded-lg"
        :class="config.iconBg"
      >
        <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" :d="config.icon" />
        </svg>
      </div>
      <h3 class="text-base font-semibold text-gray-900 dark:text-white">
        {{ title }}
      </h3>
      <span
        class="ml-auto rounded-full bg-gray-100 px-2.5 py-0.5 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-400"
      >
        {{ safeData.length }}
      </span>
    </div>

    <!-- Loading skeleton -->
    <div v-if="loading" class="divide-y divide-gray-100 dark:divide-dark-700">
      <div v-for="i in 5" :key="i" class="animate-pulse px-5 py-4">
        <div class="flex items-center gap-3">
          <div class="h-6 w-6 rounded-full bg-gray-200 dark:bg-dark-700"></div>
          <div class="h-4 flex-1 rounded bg-gray-200 dark:bg-dark-700"></div>
          <div class="h-4 w-16 rounded bg-gray-200 dark:bg-dark-700"></div>
        </div>
      </div>
    </div>

    <!-- Empty state -->
    <div
      v-else-if="!safeData.length"
      class="flex flex-col items-center justify-center px-5 py-12 text-center"
    >
      <svg
        class="mb-3 h-10 w-10 text-gray-300 dark:text-dark-600"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="1.5"
          d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
        />
      </svg>
      <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('common.noData') }}</p>
    </div>

    <!-- Ranking list -->
    <div v-else class="divide-y divide-gray-100 dark:divide-dark-700">
      <div
        v-for="(item, index) in safeData"
        :key="item.name"
        class="group cursor-pointer px-5 py-3.5 transition-colors hover:bg-gray-50/80 dark:hover:bg-dark-700/50"
        @click="emit('clickItem', item)"
      >
        <div class="flex items-center gap-3">
          <!-- Rank badge -->
          <div
            class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-xs font-bold"
            :class="getRankStyle(index)"
          >
            {{ index + 1 }}
          </div>

          <!-- Avatar / Icon -->
          <div
            v-if="item.avatar"
            class="flex h-8 w-8 shrink-0 items-center justify-center overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700"
          >
            <img :src="item.avatar" :alt="item.name" class="h-full w-full object-cover" />
          </div>
          <div
            v-else
            class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-xs font-medium"
            :class="config.iconBg"
          >
            {{ item.name.charAt(0).toUpperCase() }}
          </div>

          <!-- Name and details -->
          <div class="min-w-0 flex-1">
            <div class="flex items-center justify-between">
              <p class="truncate text-sm font-medium text-gray-900 dark:text-white">
                {{ item.name }}
              </p>
              <div class="flex items-center gap-2">
                <span class="text-sm font-semibold text-gray-900 dark:text-white">
                  {{ formatNumber(item.value) }}
                </span>
                <span
                  v-if="item.change !== undefined"
                  class="inline-flex items-center text-xs font-medium"
                  :class="
                    item.change > 0
                      ? 'text-emerald-600 dark:text-emerald-400'
                      : item.change < 0
                        ? 'text-rose-600 dark:text-rose-400'
                        : 'text-gray-400 dark:text-dark-400'
                  "
                >
                  <svg
                    v-if="item.change !== 0"
                    class="mr-0.5 h-3 w-3"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      :d="item.change > 0 ? 'M5 10l7-7m0 0l7 7m-7-7v18' : 'M19 14l-7 7m0 0l-7-7m7 7V3'"
                    />
                  </svg>
                  {{ item.change > 0 ? '+' : '' }}{{ item.change }}%
                </span>
              </div>
            </div>

            <!-- Progress bar -->
            <div class="mt-2 flex items-center gap-3">
              <div class="h-1.5 flex-1 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
                <div
                  class="h-full rounded-full transition-all duration-500"
                  :class="getBarColor(index)"
                  :style="{ width: maxValue > 0 ? `${(item.value / maxValue) * 100}%` : '0%' }"
                ></div>
              </div>
              <span
                v-if="item.secondaryValue !== undefined"
                class="shrink-0 text-xs text-gray-400 dark:text-dark-500"
              >
                {{ item.secondaryValue }}
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
