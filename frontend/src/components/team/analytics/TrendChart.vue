<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Chart as ChartJS,
  CategoryScale,
  Filler,
  Legend,
  LineElement,
  LinearScale,
  PointElement,
  Title,
  Tooltip,
  BarElement,
  BarController,
  LineController,
} from 'chart.js'
import { Line, Bar } from 'vue-chartjs'
import type { ChartComponentRef } from 'vue-chartjs'

ChartJS.register(
  Title,
  Tooltip,
  Legend,
  LineElement,
  BarElement,
  LinearScale,
  PointElement,
  CategoryScale,
  Filler,
  BarController,
  LineController
)

export interface TrendDataPoint {
  label: string
  requests: number
  tokens: number
  cost: number
}

type ChartType = 'line' | 'bar'

interface Props {
  data: TrendDataPoint[]
  type?: ChartType
  loading?: boolean
  height?: number
  showLegend?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  type: 'line',
  loading: false,
  height: 280,
  showLegend: true,
})

const chartRef = ref<ChartComponentRef | null>(null)

const { t } = useI18n()

const isDarkMode = computed(() => document.documentElement.classList.contains('dark'))

const colors = computed(() => ({
  requests: '#3b82f6',
  requestsAlpha: 'rgba(59, 130, 246, 0.1)',
  tokens: '#10b981',
  tokensAlpha: 'rgba(16, 185, 129, 0.1)',
  cost: '#f59e0b',
  costAlpha: 'rgba(245, 158, 11, 0.1)',
  grid: isDarkMode.value ? '#374151' : '#f3f4f6',
  text: isDarkMode.value ? '#9ca3af' : '#6b7280',
}))

const chartData = computed(() => {
  const labels = props.data.map((d) => d.label)
  return {
    labels,
    datasets: [
      {
        label: t('team.analytics.requests'),
        data: props.data.map((d) => d.requests),
        borderColor: colors.value.requests,
        backgroundColor: colors.value.requestsAlpha,
        fill: true,
        tension: 0.4,
        pointRadius: 3,
        pointHoverRadius: 6,
        pointBackgroundColor: colors.value.requests,
        pointBorderColor: '#fff',
        pointBorderWidth: 2,
      },
      {
        label: t('team.analytics.tokens'),
        data: props.data.map((d) => d.tokens),
        borderColor: colors.value.tokens,
        backgroundColor: colors.value.tokensAlpha,
        fill: true,
        tension: 0.4,
        pointRadius: 3,
        pointHoverRadius: 6,
        pointBackgroundColor: colors.value.tokens,
        pointBorderColor: '#fff',
        pointBorderWidth: 2,
        yAxisID: 'y1',
      },
      {
        label: t('team.analytics.cost'),
        data: props.data.map((d) => d.cost),
        borderColor: colors.value.cost,
        backgroundColor: colors.value.costAlpha,
        fill: true,
        tension: 0.4,
        pointRadius: 3,
        pointHoverRadius: 6,
        pointBackgroundColor: colors.value.cost,
        pointBorderColor: '#fff',
        pointBorderWidth: 2,
        yAxisID: 'y1',
      },
    ],
  }
})

const options = computed(() => {
  const c = colors.value
  return {
    responsive: true,
    maintainAspectRatio: false,
    interaction: { intersect: false, mode: 'index' as const },
    plugins: {
      legend: {
        display: props.showLegend,
        position: 'top' as const,
        align: 'end' as const,
        labels: {
          color: c.text,
          usePointStyle: true,
          boxWidth: 8,
          font: { size: 11 },
          padding: 16,
        },
      },
      tooltip: {
        backgroundColor: isDarkMode.value ? '#1f2937' : '#ffffff',
        titleColor: isDarkMode.value ? '#f3f4f6' : '#111827',
        bodyColor: isDarkMode.value ? '#d1d5db' : '#4b5563',
        borderColor: c.grid,
        borderWidth: 1,
        padding: 12,
        displayColors: true,
        callbacks: {
          label: (context: any) => {
            let label = context.dataset.label || ''
            if (label) label += ': '
            const val = context.parsed.y
            if (val !== null && val !== undefined) {
              if (context.dataset.label === t('team.analytics.cost')) {
                label += '$' + val.toFixed(4)
              } else {
                label += val.toLocaleString()
              }
            }
            return label
          },
        },
      },
    },
    scales: {
      x: {
        type: 'category' as const,
        grid: { display: false },
        ticks: {
          color: c.text,
          font: { size: 11 },
          maxTicksLimit: 8,
          autoSkip: true,
        },
      },
      y: {
        type: 'linear' as const,
        display: true,
        position: 'left' as const,
        grid: { color: c.grid, borderDash: [4, 4] as number[] },
        ticks: {
          color: c.text,
          font: { size: 10 },
          callback: (value: any) => {
            const num = typeof value === 'number' ? value : Number(value)
            if (num >= 1_000_000) return (num / 1_000_000).toFixed(1) + 'M'
            if (num >= 1_000) return (num / 1_000).toFixed(1) + 'K'
            return num.toLocaleString()
          },
        },
      },
      y1: {
        type: 'linear' as const,
        display: true,
        position: 'right' as const,
        grid: { display: false },
        ticks: {
          color: c.text,
          font: { size: 10 },
          callback: (value: any) => {
            const num = typeof value === 'number' ? value : Number(value)
            if (num >= 1_000_000) return (num / 1_000_000).toFixed(1) + 'M'
            if (num >= 1_000) return (num / 1_000).toFixed(1) + 'K'
            return num.toLocaleString()
          },
        },
      },
    },
  }
})

const barChartData = computed(() => {
  const labels = props.data.map((d) => d.label)
  return {
    labels,
    datasets: [
      {
        label: t('team.analytics.requests'),
        data: props.data.map((d) => d.requests),
        backgroundColor: colors.value.requests,
        borderRadius: 4,
        barPercentage: 0.7,
      },
      {
        label: t('team.analytics.tokens'),
        data: props.data.map((d) => d.tokens),
        backgroundColor: colors.value.tokens,
        borderRadius: 4,
        barPercentage: 0.7,
      },
      {
        label: t('team.analytics.cost'),
        data: props.data.map((d) => d.cost),
        backgroundColor: colors.value.cost,
        borderRadius: 4,
        barPercentage: 0.7,
      },
    ],
  }
})

const barOptions = computed(() => {
  const c = colors.value
  return {
    responsive: true,
    maintainAspectRatio: false,
    interaction: { intersect: false, mode: 'index' as const },
    plugins: {
      legend: {
        display: props.showLegend,
        position: 'top' as const,
        align: 'end' as const,
        labels: {
          color: c.text,
          usePointStyle: true,
          boxWidth: 8,
          font: { size: 11 },
          padding: 16,
        },
      },
      tooltip: {
        backgroundColor: isDarkMode.value ? '#1f2937' : '#ffffff',
        titleColor: isDarkMode.value ? '#f3f4f6' : '#111827',
        bodyColor: isDarkMode.value ? '#d1d5db' : '#4b5563',
        borderColor: c.grid,
        borderWidth: 1,
        padding: 12,
        displayColors: true,
        callbacks: {
          label: (context: any) => {
            let label = context.dataset.label || ''
            if (label) label += ': '
            const val = context.parsed.y
            if (val !== null && val !== undefined) {
              if (context.dataset.label === t('team.analytics.cost')) {
                label += '$' + val.toFixed(4)
              } else {
                label += val.toLocaleString()
              }
            }
            return label
          },
        },
      },
    },
    scales: {
      x: {
        type: 'category' as const,
        grid: { display: false },
        ticks: {
          color: c.text,
          font: { size: 11 },
          maxTicksLimit: 8,
          autoSkip: true,
        },
      },
      y: {
        type: 'linear' as const,
        display: true,
        position: 'left' as const,
        grid: { color: c.grid, borderDash: [4, 4] as number[] },
        ticks: {
          color: c.text,
          font: { size: 10 },
          callback: (value: any) => {
            const num = typeof value === 'number' ? value : Number(value)
            if (num >= 1_000_000) return (num / 1_000_000).toFixed(1) + 'M'
            if (num >= 1_000) return (num / 1_000).toFixed(1) + 'K'
            return num.toLocaleString()
          },
        },
      },
    },
  }
})

// Watch for dark mode changes
watch(isDarkMode, () => {
  // Chart.js will re-render with new colors on next update
})

function downloadChart() {
  const chart: any = chartRef.value?.chart
  if (!chart || typeof chart.toBase64Image !== 'function') return
  const url = chart.toBase64Image('image/png', 1)
  const a = document.createElement('a')
  a.href = url
  a.download = `trend-chart-${new Date().toISOString().slice(0, 19).replace(/[:T]/g, '-')}.png`
  a.click()
}

defineExpose({
  chartRef,
  downloadChart,
})
</script>

<template>
  <div
    class="overflow-hidden rounded-xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800"
  >
    <!-- Header -->
    <div
      class="flex items-center justify-between border-b border-gray-200 px-5 py-4 dark:border-dark-700"
    >
      <div class="flex items-center gap-2">
        <svg
          class="h-4 w-4 text-primary-500"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M7 12l3-3 3 3 4-4M8 21l4-4 4 4M3 4h18M4 4h16v12a1 1 0 01-1 1H5a1 1 0 01-1-1V4z"
          />
        </svg>
        <h3 class="text-base font-semibold text-gray-900 dark:text-white">趋势分析</h3>
      </div>
      <button
        type="button"
        class="inline-flex items-center gap-1 rounded-lg border border-gray-200 bg-white px-2.5 py-1.5 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-50 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300 dark:hover:bg-dark-800"
        @click="downloadChart"
      >
        <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"
          />
        </svg>
        下载
      </button>
    </div>

    <!-- Chart area -->
    <div class="p-5">
      <!-- Loading -->
      <div
        v-if="loading"
        class="flex items-center justify-center"
        :style="{ height: `${height}px` }"
      >
        <div class="flex items-center gap-2 text-sm text-gray-400 dark:text-dark-400">
          <svg
            class="h-5 w-5 animate-spin"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <circle
              class="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              stroke-width="4"
            ></circle>
            <path
              class="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
            ></path>
          </svg>
          加载中...
        </div>
      </div>

      <!-- Empty state -->
      <div
        v-else-if="!props.data || props.data.length === 0"
        class="flex flex-col items-center justify-center"
        :style="{ height: `${height}px` }"
      >
        <svg
          class="mb-2 h-10 w-10 text-gray-300 dark:text-dark-600"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="1.5"
            d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z"
          />
        </svg>
        <p class="text-sm text-gray-500 dark:text-dark-400">暂无数据</p>
      </div>

      <!-- Chart -->
      <div v-else :style="{ height: `${height}px` }">
        <Line
          v-if="type === 'line'"
          ref="chartRef"
          :data="chartData"
          :options="options"
        />
        <Bar v-else ref="chartRef" :data="barChartData" :options="barOptions" />
      </div>
    </div>
  </div>
</template>
