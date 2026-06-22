<template>
  <AppLayout>
    <div class="space-y-6">
      <div>
        <h1 class="text-2xl font-bold text-gray-900 dark:text-white">{{ t('admin.tickets.title') }}</h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('admin.tickets.description') }}</p>
      </div>

      <div class="card p-4">
        <div class="grid gap-3 md:grid-cols-[1fr_180px_180px_auto]">
          <input v-model.trim="filters.search" class="input" :placeholder="t('admin.tickets.searchPlaceholder')" @keyup.enter="loadTickets" />
          <select v-model="filters.status" class="input">
            <option value="">{{ t('tickets.filters.allStatuses') }}</option>
            <option v-for="item in statusOptions" :key="item" :value="item">{{ t(`tickets.statuses.${item}`) }}</option>
          </select>
          <select v-model="filters.category" class="input">
            <option value="">{{ t('tickets.filters.allCategories') }}</option>
            <option v-for="item in categoryOptions" :key="item" :value="item">{{ t(`tickets.categories.${item}`) }}</option>
          </select>
          <button class="btn btn-primary" @click="loadTickets">{{ t('common.search') }}</button>
        </div>
      </div>

      <div class="grid gap-6 lg:grid-cols-[minmax(0,1fr)_420px]">
        <div class="card overflow-hidden">
          <div v-if="loading" class="flex justify-center py-12">
            <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
          </div>
          <div v-else-if="tickets.length === 0" class="p-10 text-center text-sm text-gray-500 dark:text-dark-400">{{ t('admin.tickets.empty') }}</div>
          <div v-else class="divide-y divide-gray-100 dark:divide-dark-800">
            <button v-for="item in tickets" :key="item.id" class="block w-full p-4 text-left transition hover:bg-gray-50 dark:hover:bg-dark-800/60" :class="selected?.id === item.id ? 'bg-primary-50 dark:bg-primary-900/10' : ''" @click="selectTicket(item.id)">
              <div class="flex items-start justify-between gap-3">
                <div>
                  <p class="font-medium text-gray-900 dark:text-white">#{{ item.id }} {{ item.title }}</p>
                  <p class="mt-1 text-xs text-gray-500">{{ item.contact }} · {{ formatDateTime(item.updated_at) }}</p>
                </div>
                <span class="badge whitespace-nowrap" :class="statusClass(item.status)">{{ t(`tickets.statuses.${item.status}`) }}</span>
              </div>
            </button>
          </div>
        </div>

        <div class="card min-h-[420px] p-5">
          <div v-if="!selected" class="flex h-full items-center justify-center text-sm text-gray-500 dark:text-dark-400">{{ t('admin.tickets.selectHint') }}</div>
          <div v-else class="space-y-5">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">#{{ selected.id }} {{ selected.title }}</h2>
              <p class="mt-1 text-xs text-gray-500">{{ selected.contact }}</p>
            </div>

            <div class="grid grid-cols-2 gap-3 text-sm">
              <div><span class="text-gray-500">{{ t('tickets.fields.category') }}：</span>{{ t(`tickets.categories.${selected.category}`) }}</div>
              <div><span class="text-gray-500">{{ t('tickets.fields.priority') }}：</span>{{ t(`tickets.priorities.${selected.priority}`) }}</div>
            </div>

            <label class="block space-y-2">
              <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('tickets.fields.status') }}</span>
              <select v-model="selectedStatus" class="input" @change="changeStatus">
                <option v-for="item in statusOptions" :key="item" :value="item">{{ t(`tickets.statuses.${item}`) }}</option>
              </select>
            </label>

            <div class="max-h-[360px] space-y-3 overflow-y-auto pr-1">
              <div v-for="msg in selected.messages || []" :key="msg.id" class="rounded-xl border border-gray-100 p-3 text-sm dark:border-dark-800">
                <div class="mb-2 flex justify-between gap-3 text-xs">
                  <span class="font-medium" :class="msg.author_type === 'admin' ? 'text-primary-600' : 'text-gray-700 dark:text-gray-300'">{{ msg.author_type === 'admin' ? t('tickets.author.admin') : t('tickets.author.user') }}</span>
                  <span class="text-gray-500">{{ formatDateTime(msg.created_at) }}</span>
                </div>
                <p class="whitespace-pre-wrap leading-6 text-gray-700 dark:text-gray-300">{{ msg.content }}</p>
              </div>
            </div>

            <form v-if="selected.status !== 'closed'" class="space-y-3" @submit.prevent="replyTicket">
              <textarea v-model.trim="reply" class="input min-h-[110px] resize-y" :placeholder="t('admin.tickets.replyPlaceholder')" required></textarea>
              <button class="btn btn-primary w-full" type="submit" :disabled="replying">{{ replying ? t('tickets.actions.submitting') : t('tickets.actions.reply') }}</button>
            </form>
          </div>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import adminTicketsAPI from '@/api/admin/tickets'
import type { Ticket, TicketCategory, TicketStatus } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(false)
const replying = ref(false)
const tickets = ref<Ticket[]>([])
const selected = ref<Ticket | null>(null)
const selectedStatus = ref<TicketStatus>('open')
const reply = ref('')

const statusOptions: TicketStatus[] = ['open', 'pending', 'answered', 'closed']
const categoryOptions: TicketCategory[] = ['account', 'billing', 'api', 'model', 'other']
const filters = reactive({ search: '', status: '' as TicketStatus | '', category: '' as TicketCategory | '' })

function statusClass(status: TicketStatus): string {
  switch (status) {
    case 'open': return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300'
    case 'pending': return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
    case 'answered': return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
    case 'closed': return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-dark-300'
  }
}

async function loadTickets() {
  loading.value = true
  try {
    const data = await adminTicketsAPI.list({ page: 1, page_size: 50, ...filters })
    tickets.value = data.items
    if (selected.value) {
      const exists = tickets.value.some(item => item.id === selected.value?.id)
      if (!exists) selected.value = null
    }
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('tickets.messages.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function selectTicket(id: number) {
  try {
    selected.value = await adminTicketsAPI.getById(id)
    selectedStatus.value = selected.value.status
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('tickets.messages.loadFailed')))
  }
}

async function changeStatus() {
  if (!selected.value) return
  try {
    selected.value = await adminTicketsAPI.updateStatus(selected.value.id, { status: selectedStatus.value })
    await loadTickets()
    appStore.showSuccess(t('admin.tickets.statusUpdated'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.tickets.statusUpdateFailed')))
  }
}

async function replyTicket() {
  if (!selected.value || !reply.value) return
  replying.value = true
  try {
    await adminTicketsAPI.addMessage(selected.value.id, { content: reply.value })
    reply.value = ''
    await selectTicket(selected.value.id)
    await loadTickets()
    appStore.showSuccess(t('tickets.messages.replied'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('tickets.messages.replyFailed')))
  } finally {
    replying.value = false
  }
}

onMounted(loadTickets)
</script>
