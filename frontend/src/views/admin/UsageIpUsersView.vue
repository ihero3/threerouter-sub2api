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
                <th class="px-4 py-3 text-left font-medium text-gray-700 dark:text-dark-200">{{ t('admin.usage.ipUsers.columns.balance') }}</th>
                <th class="px-4 py-3 text-left font-medium text-gray-700 dark:text-dark-200">{{ t('admin.usage.ipUsers.columns.userId') }}</th>
                <th class="px-4 py-3 text-left font-medium text-gray-700 dark:text-dark-200">{{ t('admin.usage.ipUsers.columns.lastUsedAt') }}</th>
                <th class="px-4 py-3 text-left font-medium text-gray-700 dark:text-dark-200">{{ t('admin.usage.ipUsers.columns.registeredAt') }}</th>
                <th class="px-4 py-3 text-left font-medium text-gray-700 dark:text-dark-200">{{ t('admin.usage.ipUsers.columns.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="loading">
                <td colspan="6" class="px-4 py-8 text-center text-gray-400 dark:text-dark-500">...</td>
              </tr>
              <tr v-else-if="rows.length === 0">
                <td colspan="6" class="px-4 py-8 text-center text-gray-400 dark:text-dark-500">
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
                  <!-- 用户属性标记：已删除 / 充过值 / 看过次数 / 提过工单，仅管理端展示 -->
                  <span
                    v-if="userMarkLabels(row).length > 0"
                    class="ml-1 text-xs text-amber-600 dark:text-amber-400"
                  >
                    （{{ userMarkLabels(row).join('、') }}）
                  </span>
                </td>
                <td class="px-4 py-2">
                  <span
                    class="tabular-nums"
                    :class="row.balance > 0 ? 'text-gray-700 dark:text-dark-200' : 'text-gray-400 dark:text-dark-500'"
                  >
                    ${{ (row.balance ?? 0).toFixed(2) }}
                  </span>
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
                <td class="px-4 py-2">
                  <!-- 与用户管理页的「禁止/启用」同一套接口与规则：admin 账号不可在此禁用 -->
                  <button
                    v-if="row.role !== 'admin'"
                    type="button"
                    class="inline-flex items-center gap-1 rounded-md border px-2 py-1 text-xs font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50"
                    :class="row.status === 'active'
                      ? 'border-orange-200 text-orange-600 hover:bg-orange-50 dark:border-orange-500/30 dark:text-orange-400 dark:hover:bg-orange-500/10'
                      : 'border-green-200 text-green-600 hover:bg-green-50 dark:border-green-500/30 dark:text-green-400 dark:hover:bg-green-500/10'"
                    :disabled="togglingUserId === row.user_id"
                    @click="handleToggleStatus(row)"
                  >
                    <Icon :name="row.status === 'active' ? 'ban' : 'checkCircle'" size="xs" />
                    {{ row.status === 'active' ? t('admin.users.disable') : t('admin.users.enable') }}
                  </button>
                  <span v-else class="text-xs text-gray-400 dark:text-dark-500">-</span>
                </td>
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
      @deposit="handleDeposit"
      @withdraw="handleWithdraw"
      @deleted="handleUserDeleted"
    />

    <UserBalanceModal
      :show="showBalanceModal"
      :user="balanceUser"
      :operation="balanceOperation"
      @close="closeBalanceModal"
      @success="handleBalanceSuccess"
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
import UserBalanceModal from '@/components/admin/user/UserBalanceModal.vue'
import type { AdminUser } from '@/types'
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
const balanceHistoryUser = ref<AdminUser | null>(null)

// 充值 / 退款弹窗
const showBalanceModal = ref(false)
const balanceUser = ref<AdminUser | null>(null)
const balanceOperation = ref<'add' | 'subtract'>('add')

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

// 充值 / 退款：由余额历史弹窗触发，打开余额调整弹窗
const handleDeposit = () => {
  if (!balanceHistoryUser.value) return
  balanceUser.value = balanceHistoryUser.value
  balanceOperation.value = 'add'
  showBalanceModal.value = true
}

const handleWithdraw = () => {
  if (!balanceHistoryUser.value) return
  balanceUser.value = balanceHistoryUser.value
  balanceOperation.value = 'subtract'
  showBalanceModal.value = true
}

const closeBalanceModal = () => {
  showBalanceModal.value = false
  balanceUser.value = null
}

// 调整成功后刷新列表，并同步弹窗里展示的余额
const handleBalanceSuccess = async () => {
  load()
  if (!balanceHistoryUser.value) return
  try {
    balanceHistoryUser.value = await adminAPI.users.getById(balanceHistoryUser.value.id, true)
  } catch {
    // 忽略刷新失败，保留原有展示
  }
}

// 删除用户成功后刷新列表
const handleUserDeleted = () => {
  load()
}

// 正在切换状态的用户 ID：避免重复点击并发打同一个接口
const togglingUserId = ref<number | null>(null)

// 与用户管理页完全一致的行为：点击即切换 active / disabled，不弹二次确认，
// admin 账号不展示该按钮（后端也会拒绝禁用管理员）。
const handleToggleStatus = async (row: UsageIPUser) => {
  if (row.role === 'admin' || togglingUserId.value !== null) return
  const nextStatus = row.status === 'active' ? 'disabled' : 'active'
  togglingUserId.value = row.user_id
  try {
    await adminAPI.users.toggleStatus(row.user_id, nextStatus)
    appStore.showSuccess(
      nextStatus === 'active' ? t('admin.users.userEnabled') : t('admin.users.userDisabled')
    )
    // 重新拉取：列表按最后使用时间排序，状态以服务端为准最稳
    await load()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.users.failedToToggle'))
  } finally {
    togglingUserId.value = null
  }
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

// 用户名称后面的属性标记：已删除、充过值、看过次数、提过工单，用顿号分隔。
const userMarkLabels = (row: UsageIPUser): string[] => {
  const labels: string[] = []
  if (row.deleted) {
    labels.push(t('admin.usage.ipUsers.marks.deleted'))
  }
  if (row.recharged) {
    labels.push(t('admin.usage.ipUsers.marks.recharged'))
  }
  if (row.looked_count > 0) {
    labels.push(t('admin.usage.ipUsers.marks.looked', { count: row.looked_count }))
  }
  if (row.ticket_flagged) {
    labels.push(t('admin.usage.ipUsers.marks.ticket'))
  }
  return labels
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
