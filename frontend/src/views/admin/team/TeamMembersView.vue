<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Page Header -->
      <div class="flex items-center justify-between">
        <div>
          <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
            {{ t('admin.team.members.title') }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            {{ t('admin.team.members.subtitle') }}
          </p>
        </div>
        <button @click="showCreateModal = true" class="btn btn-primary">
          <Icon name="plus" size="md" class="mr-2" />
          {{ t('admin.team.members.addMember') }}
        </button>
      </div>

      <!-- Stats Cards -->
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <div class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800">
          <div class="flex items-center gap-3">
            <div class="flex h-10 w-10 items-center justify-center rounded-lg bg-primary-100 dark:bg-primary-900/30">
              <Icon name="users" size="md" class="text-primary-600 dark:text-primary-400" />
            </div>
            <div>
              <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('admin.team.members.totalMembers') }}</p>
              <p class="text-xl font-semibold text-gray-900 dark:text-white">{{ stats.total }}</p>
            </div>
          </div>
        </div>
        <div class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800">
          <div class="flex items-center gap-3">
            <div class="flex h-10 w-10 items-center justify-center rounded-lg bg-green-100 dark:bg-green-900/30">
              <Icon name="checkCircle" size="md" class="text-green-600 dark:text-green-400" />
            </div>
            <div>
              <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('admin.team.members.activeMembers') }}</p>
              <p class="text-xl font-semibold text-gray-900 dark:text-white">{{ stats.active }}</p>
            </div>
          </div>
        </div>
        <div class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800">
          <div class="flex items-center gap-3">
            <div class="flex h-10 w-10 items-center justify-center rounded-lg bg-purple-100 dark:bg-purple-900/30">
              <Icon name="shield" size="md" class="text-purple-600 dark:text-purple-400" />
            </div>
            <div>
              <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('admin.team.members.admins') }}</p>
              <p class="text-xl font-semibold text-gray-900 dark:text-white">{{ stats.admins }}</p>
            </div>
          </div>
        </div>
        <div class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800">
          <div class="flex items-center gap-3">
            <div class="flex h-10 w-10 items-center justify-center rounded-lg bg-amber-100 dark:bg-amber-900/30">
              <Icon name="clock" size="md" class="text-amber-600 dark:text-amber-400" />
            </div>
            <div>
              <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('admin.team.members.pending') }}</p>
              <p class="text-xl font-semibold text-gray-900 dark:text-white">{{ stats.pending }}</p>
            </div>
          </div>
        </div>
      </div>

      <!-- Members Table -->
      <div class="rounded-xl border border-gray-200 bg-white dark:border-dark-600 dark:bg-dark-800">
        <div class="border-b border-gray-200 p-4 dark:border-dark-600">
          <div class="flex flex-wrap items-center gap-3">
            <div class="relative w-full md:w-64">
              <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input
                v-model="searchQuery"
                type="text"
                :placeholder="t('admin.team.members.searchPlaceholder')"
                class="input pl-10"
                @input="handleSearch"
              />
            </div>
            <div class="w-full sm:w-36">
              <Select
                v-model="filters.role"
                :options="roleOptions"
                @change="applyFilter"
              />
            </div>
            <div class="w-full sm:w-36">
              <Select
                v-model="filters.status"
                :options="statusOptions"
                @change="applyFilter"
              />
            </div>
          </div>
        </div>

        <DataTable
          :columns="columns"
          :data="members"
          :loading="loading"
          :actions-count="3"
        >
          <template #cell-name="{ row }">
            <div class="flex items-center gap-3">
              <div class="flex h-9 w-9 items-center justify-center rounded-full bg-primary-100 text-sm font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">
                {{ getInitials(row.user?.name || '?') }}
              </div>
              <div>
                <p class="font-medium text-gray-900 dark:text-white">{{ row.user?.name || '-' }}</p>
                <p class="text-xs text-gray-500 dark:text-dark-400">{{ row.user?.email || '-' }}</p>
              </div>
            </div>
          </template>

          <template #cell-role="{ value }">
            <span :class="['badge', value === 'admin' ? 'badge-purple' : value === 'manager' ? 'badge-blue' : 'badge-gray']">
              {{ t('admin.team.members.roles.' + value) }}
            </span>
          </template>

          <template #cell-department="{ value }">
            <span class="text-sm text-gray-700 dark:text-gray-300">{{ value || '-' }}</span>
          </template>

          <template #cell-status="{ value }">
            <div class="flex items-center gap-1.5">
              <span :class="['inline-block h-2 w-2 rounded-full', value === 'active' ? 'bg-green-500' : value === 'pending' ? 'bg-amber-500' : 'bg-red-500']"></span>
              <span class="text-sm text-gray-700 dark:text-gray-300">{{ t('admin.team.members.status.' + value) }}</span>
            </div>
          </template>

          <template #cell-joinedAt="{ value }">
            <span class="text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(value) }}</span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center gap-1">
              <button
                @click="handleEdit(row)"
                class="rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400"
                :title="t('common.edit')"
              >
                <Icon name="edit" size="sm" />
              </button>
              <button
                @click="handleToggleStatus(row)"
                class="rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400"
                :title="row.status === 'active' ? t('common.disabled') : t('common.enabled')"
              >
                <Icon :name="row.status === 'active' ? 'ban' : 'checkCircle'" size="sm" />
              </button>
              <button
                @click="handleDelete(row)"
                class="rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
                :title="t('common.delete')"
              >
                <Icon name="trash" size="sm" />
              </button>
            </div>
          </template>

          <template #empty>
            <EmptyState
              :title="t('admin.team.members.noMembers')"
              :description="t('admin.team.members.addFirstMember')"
              :action-text="t('admin.team.members.addMember')"
              @action="showCreateModal = true"
            />
          </template>
        </DataTable>

        <div class="border-t border-gray-200 p-4 dark:border-dark-600">
          <Pagination
            v-if="pagination.total > 0"
            :page="pagination.page"
            :total="pagination.total"
            :page-size="pagination.pageSize"
            @update:page="handlePageChange"
            @update:pageSize="handlePageSizeChange"
          />
        </div>
      </div>
    </div>

    <MemberInviteDialog
      :show="showCreateModal"
      :loading="inviteLoading"
      @close="showCreateModal = false"
      @submit="handleInviteMember"
    />

    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('admin.team.members.deleteMember')"
      :message="t('admin.team.members.deleteConfirm', { name: deletingMember?.user?.name })"
      :danger="true"
      @confirm="confirmDelete"
      @cancel="showDeleteDialog = false"
    />

    <BaseDialog
      :show="showEditDialog"
      :title="t('admin.team.members.editRole')"
      width="normal"
      @close="closeEditDialog"
    >
      <div class="space-y-5">
        <div>
          <label class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.team.members.roleLabel') }}
          </label>
          <Select
            v-model="editForm.role"
            :options="editRoleOptions"
            class="w-full"
          />
          <p class="mt-1.5 text-xs text-gray-500 dark:text-dark-400">
            {{ editRoleDescription }}
          </p>
        </div>
      </div>

      <template #footer>
        <div class="flex justify-end space-x-3">
          <button
            @click="closeEditDialog"
            type="button"
            class="rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 dark:border-dark-600 dark:bg-dark-700 dark:text-gray-200 dark:hover:bg-dark-600 dark:focus:ring-offset-dark-800"
          >
            {{ t('common.cancel') }}
          </button>
          <button
            @click="confirmEditRole"
            type="button"
            :disabled="editLoading"
            class="rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 dark:focus:ring-offset-dark-800"
          >
            {{ t('common.save') }}
          </button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useTeamContext } from '@/composables/useTeamContext'
import { teamAPI } from '@/api/team'
import type { TeamMember } from '@/api/team'
import { formatDateTime } from '@/utils/format'
import Icon from '@/components/icons/Icon.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Select from '@/components/common/Select.vue'
import MemberInviteDialog from '@/components/team/MemberInviteDialog.vue'
import type { Column } from '@/components/common/types'

const { t } = useI18n()
const appStore = useAppStore()

// Types - TeamMember imported from @/api/team

// Data
const allMembers = ref<TeamMember[]>([])
const loading = ref(false)
const error = ref<string | null>(null)
const searchQuery = ref('')

// Locally filtered view of the current page's members (backend does not
// support search/filter params, so we apply them client-side).
const members = computed(() => {
  let result = allMembers.value
  const query = searchQuery.value.trim().toLowerCase()
  if (query) {
    result = result.filter((m) => {
      const name = (m.user?.name || '').toLowerCase()
      const email = (m.user?.email || '').toLowerCase()
      return name.includes(query) || email.includes(query)
    })
  }
  if (filters.role) {
    result = result.filter((m) => m.role === filters.role)
  }
  if (filters.status) {
    result = result.filter((m) => m.status === filters.status)
  }
  return result
})

const stats = computed(() => ({
  total: allMembers.value.length,
  active: allMembers.value.filter(m => m.status === 'active').length,
  admins: allMembers.value.filter(m => m.role === 'admin').length,
  pending: allMembers.value.filter(m => m.status === 'pending').length
}))

const columns: Column[] = [
  { key: 'name', label: t('admin.team.members.columns.name'), sortable: true },
  { key: 'role', label: t('admin.team.members.columns.role'), sortable: true },
  { key: 'department', label: t('admin.team.members.columns.department'), sortable: true },
  { key: 'status', label: t('admin.team.members.columns.status'), sortable: true },
  { key: 'joinedAt', label: t('admin.team.members.columns.joinedAt'), sortable: true },
  { key: 'actions', label: t('admin.team.members.columns.actions'), sortable: false }
]

const filters = reactive({
  role: '',
  status: ''
})

const roleOptions = computed(() => [
  { value: '', label: t('admin.team.members.allRoles') },
  { value: 'admin', label: t('admin.team.members.roles.admin') },
  { value: 'manager', label: t('admin.team.members.roles.manager') },
  { value: 'member', label: t('admin.team.members.roles.member') }
])

const statusOptions = computed(() => [
  { value: '', label: t('admin.team.members.allStatus') },
  { value: 'active', label: t('admin.team.members.status.active') },
  { value: 'pending', label: t('admin.team.members.status.pending') },
  { value: 'inactive', label: t('admin.team.members.status.inactive') }
])

const pagination = reactive({
  page: 1,
  pageSize: 10,
  total: 0,
  pages: 1
})

const showCreateModal = ref(false)
const showDeleteDialog = ref(false)
const deletingMember = ref<TeamMember | null>(null)
const inviteLoading = ref(false)

// Edit dialog state
const showEditDialog = ref(false)
const editingMember = ref<TeamMember | null>(null)
const editForm = reactive({
  role: 'member'
})
const editLoading = ref(false)

// Roles selectable when editing a member (owner is excluded).
const editRoleOptions = computed(() => [
  { value: 'admin', label: t('admin.team.members.roles.admin') },
  { value: 'manager', label: t('admin.team.members.roles.manager') },
  { value: 'member', label: t('admin.team.members.roles.member') },
  { value: 'viewer', label: t('admin.team.members.roles.viewer') }
])

const editRoleDescription = computed(() => {
  switch (editForm.role) {
    case 'admin':
      return t('admin.team.members.roleDescriptions.admin')
    case 'manager':
      return t('admin.team.members.roleDescriptions.manager')
    case 'member':
      return t('admin.team.members.roleDescriptions.member')
    case 'viewer':
      return t('admin.team.members.roleDescriptions.viewer')
    default:
      return ''
  }
})

const { teamId, fetchCurrentTeam } = useTeamContext()

const fetchMembers = async () => {
  if (!teamId.value) return
  loading.value = true
  error.value = null
  try {
    const response = await teamAPI.listMembers(teamId.value, {
      page: pagination.page,
      page_size: pagination.pageSize
    })
    allMembers.value = response.items || []
    pagination.total = response.total || 0
    pagination.pages = response.pages || 1
  } catch (err: any) {
    error.value = err.message || t('common.error')
    appStore.showError(t('admin.team.members.fetchFailed'))
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await fetchCurrentTeam()
  fetchMembers()
})

const handleInviteMember = async (payload: { email: string; role: string }) => {
  inviteLoading.value = true
  try {
    if (!teamId.value) {
      await fetchCurrentTeam()
    }
    if (!teamId.value) {
      appStore.showError(t('admin.team.members.fetchFailed'))
      return
    }
    await teamAPI.inviteMember(teamId.value, payload)
    appStore.showSuccess(t('admin.team.members.inviteSuccess'))
    showCreateModal.value = false
    fetchMembers()
  } catch (err: any) {
    const msg = err?.response?.data?.message || err?.message || t('common.error')
    appStore.showError(msg)
  } finally {
    inviteLoading.value = false
  }
}

const getInitials = (name: string): string => {
  return name.split(' ').map(n => n[0]).join('').toUpperCase().slice(0, 2)
}

let searchTimeout: ReturnType<typeof setTimeout>
const handleSearch = () => {
  clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    // Search is performed locally via the `members` computed; just reset to
    // the first page so the user starts browsing filtered results from the top.
    pagination.page = 1
  }, 300)
}

const applyFilter = () => {
  // Filtering is performed locally via the `members` computed; reset to the
  // first page for consistent browsing.
  pagination.page = 1
}

const handlePageChange = (page: number) => {
  pagination.page = page
  fetchMembers()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.pageSize = pageSize
  pagination.page = 1
  fetchMembers()
}

const handleEdit = (member: TeamMember) => {
  editingMember.value = member
  editForm.role = member.role
  showEditDialog.value = true
}

const handleToggleStatus = async (member: TeamMember) => {
  if (!teamId.value) return
  const newStatus = member.status === 'active' ? 'inactive' : 'active'
  try {
    await teamAPI.updateMemberStatus(teamId.value, member.id, newStatus)
    appStore.showSuccess(
      newStatus === 'active' ? t('admin.team.members.memberEnabled') : t('admin.team.members.memberDisabled')
    )
    fetchMembers()
  } catch (err: any) {
    const msg = err?.response?.data?.message || err?.message || t('common.error')
    appStore.showError(msg)
  }
}

const closeEditDialog = () => {
  showEditDialog.value = false
  editingMember.value = null
}

const confirmEditRole = async () => {
  if (!editingMember.value || !teamId.value) return
  const member = editingMember.value
  if (member.role === editForm.role) {
    closeEditDialog()
    return
  }
  editLoading.value = true
  try {
    await teamAPI.updateMemberRole(teamId.value, member.id, editForm.role)
    appStore.showSuccess(t('admin.team.members.roleUpdated'))
    showEditDialog.value = false
    editingMember.value = null
    fetchMembers()
  } catch (err: any) {
    const msg = err?.response?.data?.message || err?.message || t('common.error')
    appStore.showError(msg)
  } finally {
    editLoading.value = false
  }
}

const handleDelete = (member: TeamMember) => {
  deletingMember.value = member
  showDeleteDialog.value = true
}

const confirmDelete = async () => {
  if (!deletingMember.value || !teamId.value) return
  const member = deletingMember.value
  try {
    await teamAPI.removeMember(teamId.value, member.id)
    appStore.showSuccess(t('admin.team.members.memberDeleted'))
    showDeleteDialog.value = false
    deletingMember.value = null
    fetchMembers()
  } catch (err: any) {
    const msg = err?.response?.data?.message || err?.message || t('common.error')
    appStore.showError(msg)
    showDeleteDialog.value = false
    deletingMember.value = null
  }
}
</script>
