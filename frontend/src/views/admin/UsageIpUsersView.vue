<template>
  <AppLayout>
    <div class="space-y-4 p-4 md:p-6">
      <div class="card p-4 md:p-6">
        <div class="flex flex-wrap items-center gap-3">
          <button class="btn btn-secondary px-3 py-1.5 text-sm" @click="goBack">
            <Icon name="arrowLeft" size="sm" class="mr-1" />
            {{ t('admin.usage.ipUsers.back') }}
          </button>
          <div class="min-w-0">
            <h1 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.usage.ipUsers.title') }}
            </h1>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
              <span class="font-mono text-primary-600 dark:text-primary-400">{{ ip }}</span>
            </p>
          </div>
          <span class="ml-auto text-sm text-gray-500 dark:text-dark-400">
            {{ t('admin.usage.ipUsers.total', { count: pagination.total }) }}
          </span>
        </div>
      </div>

      <div class="card overflow-hidden">
        <div class="overflow-auto">
          <table class="min-w-full text-sm">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="px-4 py-3 text-left font-medium text-gray-700 dark:text-dark-200">{{ t('admin.usage.ipUsers.columns.user') }}</th>
                <th class="px-4 py-3 text-left font-medium text-gray-700 dark:text-dark-200">{{ t('admin.usage.ipUsers.columns.userId') }}</th>
                <th class="px-4 py-3 text-left font-medium text-gray-700 dark:text-dark-200">{{ t('admin.usage.ipUsers.columns.lastUsedAt') }}</th>
                <th class="px-4 py-3 text-left font-medium text-gray-700 dark:text-dark-200">{{ t('admin.usage.ipUsers.columns.registeredAt') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="loading">
                <td colspan="4" class="px-4 py-8 text-center text-gray-400 dark:text-dark-500">...</td>
              </tr>
              <tr v-else-if="rows.length === 0">
                <td colspan="4" class="px-4 py-8 text-center text-gray-400 dark:text-dark-500">
                  {{ t('admin.usage.ipUsers.empty') }}
                </td>
              </tr>
              <tr
                v-for="row in rows"
                v-else
                :key="row.user_id"
                class="border-t border-gray-100 dark:border-dark-800"
              >
                <td class="px-4 py-2">
                  <button
                    class="font-medium text-primary-600 underline decoration-dashed underline-offset-2 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
                    :title="t('admin.usage.clickToViewBalance')"
                    @click="openBalanceHistory(row.user_id)"
                  >
                    {{ row.email || row.username || '-' }}
                  </button>
                  <span v-if="row.username" class="ml-1 text-xs text-gray-500 dark:text-dark-400">{{ row.username }}</span>
                </td>
                <td class="px-4 py-2">
                  <button
                    class="font-mono text-gray-600 hover:underline dark:text-dark-300"
                    :title="t('admin.usage.clickToViewBalance')"
                    @click="openBalanceHistory(row.user_id)"
                  >
                    #{{ row.user_id }}
                  </button>
                </td>
                <td class="px-4 py-2" :class="row.last_used_at ? 'text-gray-600 dark:text-dark-300' : 'text-gray-400 dark:text-dark-500'">
                  {{ row.last_used_at ? formatDateTime(row.last_used_at) : t('admin.affiliates.relations.columns.neverUsed') }}
                </td>
                <td class="px-4 py-2 text-gray-600 dark:text-dark-300">{{ formatDateTime(row.created_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <Pagination
        v-if="pagination.total > 0"
        :page="pagination.page"
        :total="pagination.total"
        :page-size="pagination.page_size"
        @update:page="handlePageChange"
        @update:pageSize="handlePageSizeChange"
      />
    </div>

    <UserBalanceHistoryModal
      :show="showBalanceHistoryModal"
      :user="balanceHistoryUser"
      @close="closeBalanceHistory"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import UserBalanceHistoryModal from '@/components/admin/user/UserBalanceHistoryModal.vue'
import { adminAPI } from '@/api/admin'
import { adminUsageAPI, type UsageIPUser } from '@/api/admin/usage'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { formatDateTime } from '@/utils/format'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()

const ip = ref(String(route.query.ip ?? ''))
const rows = ref<UsageIPUser[]>([])
const loading = ref(false)
const pagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0 })

// 余额历史弹窗：与使用记录页点击用户一致的效果
const showBalanceHistoryModal = ref(false)
const balanceHistoryUser = ref<any>(null)

const load = async () => {
  loading.value = true
  try {
    const res = await adminUsageAPI.listUsersByIP(ip.value, pagination.page, pagination.page_size)
    rows.value = res.items || []
    pagination.total = res.total
  } catch {
    appStore.showError(t('admin.usage.ipUsers.loadFailed'))
    rows.value = []
    pagination.total = 0
  } finally {
    loading.value = false
  }
}

const openBalanceHistory = async (userId: number) => {
  try {
    const user = await adminAPI.users.getById(userId, true)
    balanceHistoryUser.value = user
    showBalanceHistoryModal.value = true
  } catch {
    appStore.showError(t('admin.usage.failedToLoadUser'))
  }
}

const closeBalanceHistory = () => {
  showBalanceHistoryModal.value = false
  balanceHistoryUser.value = null
}

const handlePageChange = (page: number) => {
  pagination.page = page
  load()
}

const handlePageSizeChange = (size: number) => {
  pagination.page_size = size
  pagination.page = 1
  load()
}

const goBack = () => {
  if (window.history.length > 1) {
    router.back()
  } else {
    router.push({ name: 'AdminUsage' })
  }
}

onMounted(load)
</script>
