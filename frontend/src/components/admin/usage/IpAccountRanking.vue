<template>
  <!-- 用量页"IP 排行"tab 内容：无卡片外观，依赖父级统一卡片；时间范围复用页面级筛选栏 -->
  <div>
    <!-- Toolbar -->
    <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 px-4 py-3 dark:border-dark-700/50 sm:px-6">
      <div class="min-w-0">
        <p class="text-xs text-gray-400 dark:text-gray-500">{{ t('admin.usage.ipRanking.subtitle') }}</p>
        <!-- 服务端把窗口硬限制在 7 天内；筛选范围更长时明确告知实际统计区间，避免误读 -->
        <p v-if="windowClamped" class="mt-1 text-xs text-amber-600 dark:text-amber-400">
          {{ t('admin.usage.ipRanking.windowClamped') }}
        </p>
      </div>
      <div class="flex items-center gap-3">
        <span v-if="!loading && total > 0" class="text-xs text-gray-400 dark:text-gray-500">
          {{ t('admin.usage.ipRanking.ipCount', { count: total }) }}
        </span>
      </div>
    </div>

    <!-- Table -->
    <div class="overflow-x-auto">
      <table class="w-full min-w-max divide-y divide-gray-200 dark:divide-dark-700">
        <thead class="bg-gray-50 dark:bg-dark-800">
          <tr>
            <th class="w-16 px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400 sm:px-6">#</th>
            <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">
              {{ t('admin.usage.ipRanking.columns.ip') }}
            </th>
            <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">
              {{ t('admin.usage.ipRanking.columns.accounts') }}
            </th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
          <tr v-if="loading">
            <td colspan="3" class="py-12 text-center">
              <LoadingSpinner />
            </td>
          </tr>
          <tr v-else-if="items.length === 0">
            <td colspan="3" class="py-12 text-center text-sm text-gray-400">
              {{ t('admin.dashboard.noDataAvailable') }}
            </td>
          </tr>
          <tr
            v-for="(item, index) in items"
            v-else
            :key="item.ip_address"
            class="cursor-pointer transition-colors hover:bg-gray-50 dark:hover:bg-dark-700/40"
            :title="t('admin.usage.ipRanking.rowHint')"
            @click="$emit('select-ip', item.ip_address)"
          >
            <td class="px-4 py-3 sm:px-6">
              <span
                v-if="rankOf(index) <= 3"
                class="inline-flex h-6 w-6 items-center justify-center rounded-full text-xs font-semibold"
                :class="RANK_BADGE_CLASSES[rankOf(index) - 1]"
              >{{ rankOf(index) }}</span>
              <span v-else class="inline-block w-6 text-center text-sm tabular-nums text-gray-400">{{ rankOf(index) }}</span>
            </td>
            <td class="px-4 py-3 font-mono text-sm font-medium text-gray-700 dark:text-gray-200">{{ item.ip_address }}</td>
            <td class="whitespace-nowrap px-4 py-3 text-right text-sm font-medium tabular-nums text-gray-900 dark:text-gray-100">
              {{ item.account_count.toLocaleString() }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <Pagination
      v-if="total > pageSize"
      :page="page"
      :total="total"
      :page-size="pageSize"
      @update:page="onPageChange"
      @update:pageSize="onPageSizeChange"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { listIPAccountRanking, type UsageIPAccountRank } from '@/api/admin/usage'
import Pagination from '@/components/common/Pagination.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'

const props = defineProps<{
  startDate: string
  endDate: string
}>()

defineEmits<{ (e: 'select-ip', ip: string): void }>()

const { t } = useI18n()

const RANK_BADGE_CLASSES = [
  'bg-amber-100 text-amber-700 dark:bg-amber-500/20 dark:text-amber-400',
  'bg-gray-200 text-gray-600 dark:bg-gray-500/20 dark:text-gray-300',
  'bg-orange-100 text-orange-700 dark:bg-orange-500/20 dark:text-orange-400',
]

const items = ref<UsageIPAccountRank[]>([])
const loading = ref(false)
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
let reqSeq = 0

// 翻页后名次要跟着页偏移，否则第 2 页又从 1 开始
const rankOf = (index: number) => (page.value - 1) * pageSize.value + index + 1

// 服务端最多统计 7 天：7 个自然日 = 首尾相差 6 天，超过就说明窗口被收紧
const windowClamped = computed(() => {
  if (!props.startDate || !props.endDate) return false
  const start = new Date(`${props.startDate}T00:00:00`).getTime()
  const end = new Date(`${props.endDate}T00:00:00`).getTime()
  if (Number.isNaN(start) || Number.isNaN(end)) return false
  return end - start > 6 * 24 * 60 * 60 * 1000
})

const load = async () => {
  const seq = ++reqSeq
  loading.value = true
  try {
    const res = await listIPAccountRanking({
      start_date: props.startDate || undefined,
      end_date: props.endDate || undefined,
      page: page.value,
      page_size: pageSize.value,
    })
    if (seq !== reqSeq) return
    items.value = res.items || []
    total.value = res.total || 0
  } catch {
    if (seq !== reqSeq) return
    items.value = []
    total.value = 0
  } finally {
    if (seq === reqSeq) loading.value = false
  }
}

// 时间范围变化回到第一页；只改页大小时保留语义上的位置没有意义，也回到第一页
const reload = () => {
  page.value = 1
  load()
}

const onPageChange = (next: number) => {
  page.value = next
  load()
}

const onPageSizeChange = (next: number) => {
  pageSize.value = next
  reload()
}

// Reload when the shared date range changes.
watch(() => [props.startDate, props.endDate], reload, { immediate: true })

defineExpose({ reload })
</script>
