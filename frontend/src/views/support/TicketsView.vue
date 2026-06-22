<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 class="text-2xl font-bold text-gray-900 dark:text-white">{{ t('tickets.my.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('tickets.my.description') }}</p>
        </div>
        <router-link class="btn btn-primary" to="/support/tickets/new">{{ t('tickets.actions.new') }}</router-link>
      </div>

      <div class="card overflow-hidden">
        <div v-if="loading" class="flex justify-center py-12">
          <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
        </div>
        <div v-else-if="tickets.length === 0" class="p-10 text-center text-sm text-gray-500 dark:text-dark-400">
          {{ t('tickets.my.empty') }}
        </div>
        <div v-else class="overflow-x-auto">
          <table class="w-full min-w-[760px] text-left text-sm">
            <thead class="bg-gray-50 text-gray-500 dark:bg-dark-900 dark:text-dark-400">
              <tr>
                <th class="px-4 py-3 font-medium">{{ t('tickets.fields.id') }}</th>
                <th class="px-4 py-3 font-medium">{{ t('tickets.fields.title') }}</th>
                <th class="px-4 py-3 font-medium">{{ t('tickets.fields.category') }}</th>
                <th class="px-4 py-3 font-medium">{{ t('tickets.fields.priority') }}</th>
                <th class="px-4 py-3 font-medium">{{ t('tickets.fields.status') }}</th>
                <th class="px-4 py-3 font-medium">{{ t('tickets.fields.updatedAt') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in tickets" :key="item.id" class="border-t border-gray-100 hover:bg-gray-50 dark:border-dark-800 dark:hover:bg-dark-800/60">
                <td class="px-4 py-3 text-gray-500">#{{ item.id }}</td>
                <td class="px-4 py-3">
                  <router-link class="font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400" :to="`/support/tickets/${item.id}`">{{ item.title }}</router-link>
                </td>
                <td class="px-4 py-3 text-gray-700 dark:text-gray-300">{{ t(`tickets.categories.${item.category}`) }}</td>
                <td class="px-4 py-3 text-gray-700 dark:text-gray-300">{{ t(`tickets.priorities.${item.priority}`) }}</td>
                <td class="px-4 py-3"><span class="badge" :class="statusClass(item.status)">{{ t(`tickets.statuses.${item.status}`) }}</span></td>
                <td class="px-4 py-3 text-gray-500">{{ formatDateTime(item.updated_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import ticketsAPI from '@/api/tickets'
import type { Ticket, TicketStatus } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(true)
const tickets = ref<Ticket[]>([])

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
    const data = await ticketsAPI.listMine(1, 50)
    tickets.value = data.items
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('tickets.messages.loadFailed')))
  } finally {
    loading.value = false
  }
}

onMounted(loadTickets)
</script>
