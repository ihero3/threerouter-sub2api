<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI } from '@/api/admin'
import type { MediaTask, MediaTaskStatus } from '@/api/admin/mediaTasks'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()

const tasks = ref<MediaTask[]>([])
const loading = ref(false)
const cancelingIds = ref(new Set<number>())

const filters = reactive<{ user_id: number | undefined; status: MediaTaskStatus | ''; media_kind: string }>({
  user_id: undefined,
  status: '',
  media_kind: ''
})

const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})

const confirmDialog = reactive({ visible: false, loading: false, taskId: 0 })

const statusFilterOptions = computed(() => [
  { value: '', label: t('common.all') },
  { value: 'processing', label: t('admin.mediaTasks.status.processing') },
  { value: 'succeeded', label: t('admin.mediaTasks.status.succeeded') },
  { value: 'failed', label: t('admin.mediaTasks.status.failed') },
  { value: 'cancelled', label: t('admin.mediaTasks.status.cancelled') }
])

const mediaKindOptions = computed(() => [
  { value: '', label: t('common.all') },
  { value: 'video', label: 'Video' },
  { value: 'image', label: 'Image' },
  { value: 'audio', label: 'Audio' }
])

const columns = computed(() => [
  { key: 'local_id', label: t('admin.mediaTasks.columns.taskId'), sortable: false, width: '180px' },
  { key: 'media_kind', label: 'Kind', sortable: false, width: '80px' },
  { key: 'public_model', label: t('admin.mediaTasks.columns.model'), sortable: false, width: '180px' },
  { key: 'status', label: t('admin.mediaTasks.columns.status'), sortable: false, width: '110px' },
  { key: 'user_id', label: t('admin.mediaTasks.columns.user'), sortable: false, width: '90px' },
  { key: 'account_id', label: t('admin.mediaTasks.columns.channel'), sortable: false, width: '90px' },
  { key: 'resolution', label: t('admin.mediaTasks.columns.resolution'), sortable: false, width: '120px' },
  { key: 'media_url', label: 'URL', sortable: false, width: '110px' },
  { key: 'cost_usd', label: t('admin.mediaTasks.columns.cost'), sortable: false, width: '100px' },
  { key: 'error_message', label: t('admin.mediaTasks.columns.error'), sortable: false },
  { key: 'created_at', label: t('admin.mediaTasks.columns.createdAt'), sortable: true, width: '180px' },
  { key: 'actions', label: t('common.actions'), sortable: false, width: '90px', align: 'right' as const }
])

let loadController: AbortController | null = null

const statusBadgeClass = (status: MediaTaskStatus) => {
  switch (status) {
    case 'processing': return 'badge-info'
    case 'succeeded': return 'badge-success'
    case 'failed': return 'badge-error'
    case 'cancelled': return 'badge-gray'
    default: return 'badge-gray'
  }
}

const statusLabel = (status: MediaTaskStatus) => t(`admin.mediaTasks.status.${status}`)

const loadTasks = async () => {
  if (loadController) loadController.abort()
  loadController = new AbortController()
  loading.value = true
  try {
    const response = await adminAPI.mediaTasks.list({
      user_id: filters.user_id,
      page: pagination.page,
      page_size: pagination.page_size,
      status: filters.status || undefined,
      media_kind: filters.media_kind || undefined,
      signal: loadController.signal
    })
    tasks.value = response.items
    pagination.total = response.total
    pagination.pages = response.pages
  } catch (err: any) {
    if (err?.name === 'CanceledError' || err?.code === 'ERR_CANCELED') return
    console.error('Failed to load media tasks:', err)
  } finally {
    loading.value = false
  }
}

const handleFilterChange = () => {
  pagination.page = 1
  loadTasks()
}

const handlePageChange = (page: number) => {
  const validPage = Math.max(1, Math.min(page, pagination.pages || 1))
  pagination.page = validPage
  loadTasks()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.page_size = pageSize
  pagination.page = 1
  loadTasks()
}

const handleCancel = (task: MediaTask) => {
  confirmDialog.taskId = task.id
  confirmDialog.visible = true
}

const confirmCancel = async () => {
  confirmDialog.loading = true
  try {
    cancelingIds.value.add(confirmDialog.taskId)
    await adminAPI.mediaTasks.cancel(confirmDialog.taskId)
    confirmDialog.visible = false
    await loadTasks()
  } catch (err: any) {
    console.error('Failed to cancel task:', err)
  } finally {
    cancelingIds.value.delete(confirmDialog.taskId)
    confirmDialog.loading = false
  }
}

onBeforeUnmount(() => {
  if (loadController) loadController.abort()
})
</script>

<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <div class="flex-1 sm:max-w-64">
            <input v-model.number="filters.user_id" type="number" :placeholder="t('admin.mediaTasks.userIdPlaceholder')" class="input" min="1" @change="handleFilterChange" />
          </div>
          <Select v-model="filters.media_kind" :options="mediaKindOptions" class="w-40" :placeholder="'Kind'" @change="handleFilterChange" />
          <Select v-model="filters.status" :options="statusFilterOptions" class="w-40" :placeholder="t('common.status')" @change="handleFilterChange" />
          <div class="flex flex-1 flex-wrap items-center justify-end gap-2">
            <button @click="loadTasks" :disabled="loading" class="btn btn-secondary" :title="t('common.refresh')">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="tasks" :loading="loading" row-key="id" default-sort-key="created_at" default-sort-order="desc">
          <template #cell-local_id="{ value, row }">
            <div class="min-w-0">
              <code class="truncate text-xs text-gray-700 dark:text-gray-300">{{ value }}</code>
              <div class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">#{{ row.id }}</div>
            </div>
          </template>
          <template #cell-media_kind="{ value }">
            <span class="badge badge-gray">{{ value }}</span>
          </template>
          <template #cell-public_model="{ row }">
            <div class="text-sm">
              <div class="font-medium text-gray-900 dark:text-white">{{ row.public_model }}</div>
              <div class="text-xs text-gray-500 dark:text-dark-400">→ {{ row.upstream_model }}</div>
            </div>
          </template>
          <template #cell-status="{ value }">
            <span :class="['badge', statusBadgeClass(value)]">{{ statusLabel(value) }}</span>
          </template>
          <template #cell-media_url="{ row }">
            <a v-if="row.media_url" :href="row.media_url" target="_blank" rel="noopener noreferrer" class="text-xs text-primary-600 hover:text-primary-700 dark:text-primary-400">
              <Icon name="externalLink" size="sm" class="inline" />
              {{ t('admin.mediaTasks.openMedia') }}
            </a>
            <span v-else class="text-xs text-gray-400">-</span>
          </template>
          <template #cell-cost_usd="{ value }">
            <span class="text-sm text-gray-700 dark:text-gray-300">${{ Number(value || 0).toFixed(4) }}</span>
          </template>
          <template #cell-error_message="{ value }">
            <span v-if="value" class="line-clamp-2 max-w-xs text-xs text-red-600 dark:text-red-400" :title="value">{{ value }}</span>
            <span v-else class="text-xs text-gray-400">-</span>
          </template>
          <template #cell-created_at="{ value, row }">
            <div class="text-sm text-gray-700 dark:text-gray-300">
              <div>{{ formatDateTime(value) }}</div>
              <div v-if="row.finished_at" class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.mediaTasks.finishedAt') }}: {{ formatDateTime(row.finished_at) }}</div>
            </div>
          </template>
          <template #cell-actions="{ row }">
            <button v-if="row.status === 'processing'" @click="handleCancel(row)" :disabled="cancelingIds.has(row.id)" class="btn btn-sm btn-secondary" :title="t('admin.mediaTasks.cancel')">
              <Icon name="x" size="sm" :class="cancelingIds.has(row.id) ? 'animate-spin' : ''" />
              {{ t('admin.mediaTasks.cancel') }}
            </button>
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination :page="pagination.page" :page-size="pagination.page_size" :total="pagination.total" :pages="pagination.pages" @change="handlePageChange" @page-size-change="handlePageSizeChange" />
      </template>

      <template #empty>
        <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('common.noData') }}</p>
      </template>
    </TablePageLayout>
    <ConfirmDialog
      :show="confirmDialog.visible"
      :title="t('admin.mediaTasks.cancelConfirmTitle')"
      :message="t('admin.mediaTasks.cancelConfirmMessage', { id: confirmDialog.taskId })"
      :loading="confirmDialog.loading"
      @confirm="confirmCancel"
      @close="confirmDialog.visible = false"
    />
  </AppLayout>
</template>
