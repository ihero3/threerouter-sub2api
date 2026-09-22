<template>
  <AppLayout>
    <div class="space-y-6 p-4 md:p-6">
      <div class="flex flex-wrap items-center gap-2">
        <button
          class="btn px-3 py-1.5 text-sm"
          :class="activeTab === 'relations'
            ? 'bg-primary-500 text-white'
            : 'btn-secondary text-gray-600 dark:text-dark-300'"
          @click="activeTab = 'relations'"
        >{{ t('admin.affiliates.relations.tabs.relations') }}</button>
        <button
          class="btn px-3 py-1.5 text-sm"
          :class="activeTab === 'unsourced'
            ? 'bg-primary-500 text-white'
            : 'btn-secondary text-gray-600 dark:text-dark-300'"
          @click="switchToUnsourced"
        >{{ t('admin.affiliates.relations.tabs.unsourced') }}</button>
      </div>

      <template v-if="activeTab === 'relations'">
        <div class="card p-4 md:p-6">
          <div class="flex flex-wrap items-center gap-2">
            <div class="relative min-w-[240px] flex-1">
              <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input
                v-model="keyword"
                type="text"
                class="input pl-10"
                :placeholder="t('admin.affiliates.relations.searchPlaceholder')"
                @keyup.enter="searchUser"
              />
            </div>
            <button class="btn btn-primary px-4" :disabled="loading" @click="searchUser">
              {{ loading ? t('admin.affiliates.relations.searching') : t('admin.affiliates.relations.search') }}
            </button>
          </div>

          <div v-if="candidates.length > 1" class="mt-4 space-y-1 rounded-lg border border-gray-200 p-3 dark:border-dark-700">
            <p class="mb-2 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.affiliates.relations.pickUser') }}</p>
            <button
              v-for="c in candidates"
              :key="c.id"
              class="flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-sm hover:bg-gray-50 dark:hover:bg-dark-800"
              @click="loadRelations(c.id)"
            >
              <span class="font-mono text-xs text-gray-500 dark:text-dark-400">#{{ c.id }}</span>
              <span class="text-gray-900 dark:text-white">{{ c.email || '-' }}</span>
              <span class="text-gray-500 dark:text-dark-400">{{ c.username || '-' }}</span>
            </button>
          </div>
        </div>

        <div v-if="loading" class="card p-4 text-sm text-gray-500 md:p-6 dark:text-dark-400">
          {{ t('admin.affiliates.relations.searching') }}
        </div>

        <div v-if="relation" class="card p-4 md:p-6">
          <div class="flex flex-wrap items-baseline gap-2">
            <span class="font-mono text-sm text-gray-900 dark:text-white">#{{ relation.user.user_id }}</span>
            <span class="font-medium text-gray-900 dark:text-white">{{ relation.user.email || '-' }}</span>
            <span class="text-sm text-gray-500 dark:text-dark-400">{{ relation.user.username || '-' }}</span>
            <span class="text-xs text-gray-400 dark:text-dark-500">{{ formatDateTime(relation.user.created_at) }}</span>
          </div>
          <div class="mt-2 text-sm">
            <span v-if="relation.inviter" class="text-gray-600 dark:text-dark-300">
              {{ t('admin.affiliates.relations.inviter') }}：
              <button class="font-mono text-primary-600 underline dark:text-primary-400" @click="loadRelations(relation.inviter.user_id)">
                #{{ relation.inviter.user_id }}
              </button>
              {{ relation.inviter.email }}
              <span v-if="relation.bound_at" class="ml-2 text-xs text-gray-400 dark:text-dark-500">
                {{ t('admin.affiliates.relations.boundAt') }} {{ formatDateTime(relation.bound_at) }}
              </span>
            </span>
            <span v-else class="font-medium text-amber-600 dark:text-amber-400">
              {{ t('admin.affiliates.relations.noInviter') }}
            </span>
          </div>
        </div>

        <div v-if="relation" class="card p-4 md:p-6">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.affiliates.relations.chainUp') }}</h3>
          <div v-if="relation.ancestors.length === 0" class="mt-3 text-sm text-gray-500 dark:text-dark-400">
            {{ t('admin.affiliates.relations.chainEmpty') }}
          </div>
          <div v-else class="mt-3 flex flex-wrap items-center gap-2">
            <template v-for="(node, index) in relation.ancestors" :key="node.user_id">
              <button
                class="rounded-lg bg-gray-50 px-3 py-2 text-left hover:bg-gray-100 dark:bg-dark-800 dark:hover:bg-dark-700"
                @click="loadRelations(node.user_id)"
              >
                <div class="font-mono text-xs text-gray-900 dark:text-white">#{{ node.user_id }}</div>
                <div class="text-xs text-gray-600 dark:text-dark-300">{{ node.email || '-' }}</div>
                <div class="text-xs text-gray-400 dark:text-dark-500">
                  {{ ancestorLabel(node, index) }}
                </div>
              </button>
              <span class="text-gray-400 dark:text-dark-500">→</span>
            </template>
            <div class="rounded-lg bg-primary-50 px-3 py-2 dark:bg-primary-500/10">
              <div class="font-mono text-xs text-gray-900 dark:text-white">#{{ relation.user.user_id }} {{ t('admin.affiliates.relations.currentUser') }}</div>
              <div class="text-xs text-gray-600 dark:text-dark-300">{{ relation.user.email || '-' }}</div>
            </div>
          </div>
        </div>

        <div v-if="relation" class="card p-4 md:p-6">
          <div class="flex items-baseline justify-between">
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.affiliates.relations.descendants') }}</h3>
            <span class="text-xs text-gray-500 dark:text-dark-400">
              {{ t('admin.affiliates.relations.descendantCount', { count: relation.descendant_count }) }}
            </span>
          </div>
          <p v-if="relation.chain_truncated" class="mt-2 text-xs text-amber-600 dark:text-amber-400">
            {{ t('admin.affiliates.relations.truncated') }}
          </p>
          <div v-if="relation.descendants.length === 0" class="mt-3 text-sm text-gray-500 dark:text-dark-400">
            {{ t('admin.affiliates.relations.descendantsEmpty') }}
          </div>
          <div v-else class="mt-3 overflow-x-auto">
            <table class="w-full min-w-[520px] text-left text-sm">
              <thead>
                <tr class="border-b border-gray-200 text-gray-500 dark:border-dark-700 dark:text-dark-400">
                  <th class="px-2 py-2 font-medium">{{ t('admin.affiliates.relations.columns.depth') }}</th>
                  <th class="px-2 py-2 font-medium">{{ t('admin.affiliates.relations.columns.user') }}</th>
                  <th class="px-2 py-2 font-medium">{{ t('admin.affiliates.relations.columns.createdAt') }}</th>
                  <th class="px-2 py-2 text-right font-medium">{{ t('admin.affiliates.relations.columns.rebate') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="node in relation.descendants"
                  :key="`${node.depth}-${node.user_id}`"
                  class="border-b border-gray-100 last:border-b-0 dark:border-dark-800"
                >
                  <td class="px-2 py-2">
                    <span
                      class="inline-flex rounded-full px-2 py-0.5 text-xs"
                      :class="node.depth === 1
                        ? 'bg-blue-100 text-blue-700 dark:bg-blue-500/15 dark:text-blue-300'
                        : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-dark-300'"
                    >{{ depthLabel(node.depth) }}</span>
                  </td>
                  <td class="px-2 py-2">
                    <button class="text-left hover:underline" @click="loadRelations(node.user_id)">
                      <span class="font-mono text-xs text-gray-500 dark:text-dark-400">#{{ node.user_id }}</span>
                      <span class="ml-1 text-gray-900 dark:text-white">{{ node.email || '-' }}</span>
                      <span v-if="node.username" class="ml-1 text-xs text-gray-500 dark:text-dark-400">{{ node.username }}</span>
                    </button>
                  </td>
                  <td class="px-2 py-2 text-gray-600 dark:text-dark-300">{{ formatDateTime(node.created_at) }}</td>
                  <td class="px-2 py-2 text-right font-mono text-gray-900 dark:text-white">${{ formatAmount(node.rebate_amount) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <p class="mt-3 text-xs text-gray-400 dark:text-dark-500">{{ t('admin.affiliates.relations.rebateHint') }}</p>
        </div>
      </template>

      <template v-else>
        <div class="card p-4 md:p-6">
          <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('admin.affiliates.relations.unsourced.description') }}</p>
          <div class="mt-4 flex flex-wrap items-center gap-2">
            <div class="relative min-w-[240px] flex-1">
              <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input v-model="unsourcedSearch" type="text" class="input pl-10" :placeholder="t('admin.affiliates.relations.searchPlaceholder')" @input="debounceLoadUnsourced" />
            </div>
            <button class="btn btn-secondary px-2 md:px-3" :disabled="unsourcedLoading" :title="t('common.refresh')" @click="loadUnsourced">
              <Icon name="refresh" size="md" :class="unsourcedLoading ? 'animate-spin' : ''" />
            </button>
          </div>

          <div class="mt-4 grid gap-3 sm:grid-cols-2">
            <div class="rounded-lg bg-gray-50 p-4 dark:bg-dark-800">
              <div class="text-sm text-gray-500 dark:text-dark-400">{{ t('admin.affiliates.relations.unsourced.total') }}</div>
              <div class="text-2xl font-medium text-gray-900 dark:text-white">{{ unsourcedTotal.toLocaleString() }}</div>
            </div>
          </div>

          <div v-if="!unsourcedLoading && unsourced.length === 0" class="mt-4 rounded-xl border border-dashed border-gray-300 p-6 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400">
            {{ t('admin.affiliates.relations.unsourced.empty') }}
          </div>
          <div v-else class="mt-4 overflow-x-auto">
            <table class="w-full min-w-[560px] text-left text-sm">
              <thead>
                <tr class="border-b border-gray-200 text-gray-500 dark:border-dark-700 dark:text-dark-400">
                  <th class="px-2 py-2 font-medium">{{ t('admin.affiliates.relations.unsourced.columns.user') }}</th>
                  <th class="px-2 py-2 font-medium">{{ t('admin.affiliates.relations.unsourced.columns.createdAt') }}</th>
                  <th class="px-2 py-2 text-right font-medium">{{ t('admin.affiliates.relations.unsourced.columns.balance') }}</th>
                  <th class="px-2 py-2 text-right font-medium">{{ t('admin.affiliates.relations.unsourced.columns.totalRecharged') }}</th>
                  <th class="px-2 py-2 font-medium">{{ t('admin.affiliates.relations.unsourced.columns.profile') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item in unsourced" :key="item.user_id" class="border-b border-gray-100 last:border-b-0 dark:border-dark-800">
                  <td class="px-2 py-2">
                    <button class="text-left hover:underline" @click="showRelationsOf(item.user_id)">
                      <span class="font-mono text-xs text-gray-500 dark:text-dark-400">#{{ item.user_id }}</span>
                      <span class="ml-1 text-gray-900 dark:text-white">{{ item.email || '-' }}</span>
                    </button>
                  </td>
                  <td class="px-2 py-2 text-gray-600 dark:text-dark-300">{{ formatDateTime(item.created_at) }}</td>
                  <td class="px-2 py-2 text-right font-mono text-gray-900 dark:text-white">${{ formatAmount(item.balance) }}</td>
                  <td class="px-2 py-2 text-right font-mono text-gray-900 dark:text-white">${{ formatAmount(item.total_recharged) }}</td>
                  <td class="px-2 py-2">
                    <span class="text-xs" :class="item.has_affiliate_profile ? 'text-gray-500 dark:text-dark-400' : 'text-amber-600 dark:text-amber-400'">
                      {{ item.has_affiliate_profile
                        ? t('admin.affiliates.relations.unsourced.hasProfile')
                        : t('admin.affiliates.relations.unsourced.noProfile') }}
                    </span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <div class="mt-4">
            <Pagination
              v-if="unsourcedTotal > 0"
              :page="unsourcedPage"
              :total="unsourcedTotal"
              :page-size="unsourcedPageSize"
              @update:page="handleUnsourcedPageChange"
              @update:pageSize="handleUnsourcedPageSizeChange"
            />
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import { affiliatesAPI, type AffiliateInviteRelation, type AffiliateRelationNode, type AffiliateUnsourcedUser, type SimpleUser } from '@/api/admin/affiliates'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'
import { extractI18nErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const route = useRoute()
const appStore = useAppStore()

type TabKey = 'relations' | 'unsourced'

const activeTab = ref<TabKey>('relations')
const keyword = ref('')
const loading = ref(false)
const relation = ref<AffiliateInviteRelation | null>(null)
const candidates = ref<SimpleUser[]>([])

const unsourced = ref<AffiliateUnsourcedUser[]>([])
const unsourcedTotal = ref(0)
const unsourcedPage = ref(1)
const unsourcedPageSize = ref(20)
const unsourcedSearch = ref('')
const unsourcedLoading = ref(false)
let debounceTimer: ReturnType<typeof setTimeout> | null = null

function formatAmount(value: number | null | undefined): string {
  return Number(value || 0).toFixed(2)
}

function depthLabel(depth: number): string {
  return t('admin.affiliates.relations.depthLevel', { n: depth })
}

// ancestorLabel 区分三种身份：链路顶端（自己也没上级）、直接邀请人、更远的上级。
// 注意 ancestors 是按 depth 降序返回的，index 0 是链路最顶端。
function ancestorLabel(node: AffiliateRelationNode, index: number): string {
  if (index === 0) return t('admin.affiliates.relations.chainTop')
  if (node.depth === 1) return t('admin.affiliates.relations.ancestorDirect')
  return t('admin.affiliates.relations.ancestorLevel', { n: node.depth })
}

async function loadRelations(userId: number) {
  if (!userId || userId <= 0) return
  loading.value = true
  candidates.value = []
  // 先清空再请求：否则新用户查不到（例如 ID 不存在/已删除）时会继续显示上一个用户的关系，
  // 管理员会误以为是当前查询结果。
  relation.value = null
  try {
    relation.value = await affiliatesAPI.getInviteRelations(userId)
    keyword.value = String(userId)
  } catch (error) {
    appStore.showError(extractI18nErrorMessage(error, t, 'admin.affiliates', t('admin.affiliates.relations.loadFailed')))
  } finally {
    loading.value = false
  }
}

// searchUser 支持两种输入：纯数字按 user_id 直接查；否则走用户搜索再取结果。
async function searchUser() {
  const raw = keyword.value.trim()
  if (!raw) {
    appStore.showError(t('admin.affiliates.relations.invalidInput'))
    return
  }
  if (/^\d+$/.test(raw)) {
    await loadRelations(Number(raw))
    return
  }
  loading.value = true
  try {
    const users = await affiliatesAPI.lookupUsers(raw)
    if (users.length === 0) {
      appStore.showError(t('admin.affiliates.relations.notFound'))
      return
    }
    if (users.length === 1) {
      await loadRelations(users[0].id)
      return
    }
    candidates.value = users
  } catch (error) {
    appStore.showError(extractI18nErrorMessage(error, t, 'admin.affiliates', t('admin.affiliates.relations.loadFailed')))
  } finally {
    loading.value = false
  }
}

function showRelationsOf(userId: number) {
  activeTab.value = 'relations'
  void loadRelations(userId)
}

async function loadUnsourced() {
  unsourcedLoading.value = true
  try {
    const res = await affiliatesAPI.listUnsourcedUsers({
      page: unsourcedPage.value,
      page_size: unsourcedPageSize.value,
      search: unsourcedSearch.value || undefined,
    })
    unsourced.value = res.items
    unsourcedTotal.value = res.total
  } catch (error) {
    appStore.showError(extractI18nErrorMessage(error, t, 'admin.affiliates', t('admin.affiliates.relations.loadFailed')))
  } finally {
    unsourcedLoading.value = false
  }
}

function debounceLoadUnsourced() {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => {
    unsourcedPage.value = 1
    void loadUnsourced()
  }, 300)
}

function handleUnsourcedPageChange(page: number) {
  unsourcedPage.value = page
  void loadUnsourced()
}

function handleUnsourcedPageSizeChange(size: number) {
  unsourcedPageSize.value = size
  unsourcedPage.value = 1
  void loadUnsourced()
}

function switchToUnsourced() {
  activeTab.value = 'unsourced'
  if (unsourced.value.length === 0 && !unsourcedLoading.value) {
    void loadUnsourced()
  }
}

// 监听路由参数而不是只在 mounted 时读一次：从用户管理页反复点「邀请关系」
// （或浏览器前进/后退）时 user_id 变化但组件可能复用，不监听就会停在旧数据上。
watch(
  () => route.query.user_id,
  (raw) => {
    if (typeof raw === 'string' && /^\d+$/.test(raw)) {
      void loadRelations(Number(raw))
    }
  },
  { immediate: true },
)
</script>
