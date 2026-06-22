<template>
  <AppLayout>
    <div class="mx-auto max-w-4xl space-y-6">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <router-link class="text-sm text-primary-600 hover:text-primary-700 dark:text-primary-400" to="/support/tickets">← {{ t('tickets.actions.backToList') }}</router-link>
          <h1 class="mt-2 text-2xl font-bold text-gray-900 dark:text-white">{{ ticket?.title || t('tickets.detail.title') }}</h1>
        </div>
        <router-link class="btn btn-secondary" to="/support/tickets/new">{{ t('tickets.actions.new') }}</router-link>
      </div>

      <div v-if="loading" class="flex justify-center py-12">
        <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      </div>

      <template v-else-if="ticket">
        <div class="card grid gap-4 p-5 sm:grid-cols-4">
          <div>
            <p class="text-xs text-gray-500">{{ t('tickets.fields.id') }}</p>
            <p class="mt-1 font-medium text-gray-900 dark:text-white">#{{ ticket.id }}</p>
          </div>
          <div>
            <p class="text-xs text-gray-500">{{ t('tickets.fields.category') }}</p>
            <p class="mt-1 font-medium text-gray-900 dark:text-white">{{ t(`tickets.categories.${ticket.category}`) }}</p>
          </div>
          <div>
            <p class="text-xs text-gray-500">{{ t('tickets.fields.priority') }}</p>
            <p class="mt-1 font-medium text-gray-900 dark:text-white">{{ t(`tickets.priorities.${ticket.priority}`) }}</p>
          </div>
          <div>
            <p class="text-xs text-gray-500">{{ t('tickets.fields.status') }}</p>
            <p class="mt-1 font-medium text-gray-900 dark:text-white">{{ t(`tickets.statuses.${ticket.status}`) }}</p>
          </div>
        </div>

        <div class="space-y-4">
          <div v-for="msg in ticket.messages || []" :key="msg.id" class="card p-5" :class="msg.author_type === 'admin' ? 'border-primary-200 dark:border-primary-900/40' : ''">
            <div class="mb-3 flex items-center justify-between gap-3">
              <span class="text-sm font-semibold" :class="msg.author_type === 'admin' ? 'text-primary-600 dark:text-primary-400' : 'text-gray-900 dark:text-white'">
                {{ msg.author_type === 'admin' ? t('tickets.author.admin') : t('tickets.author.user') }}
              </span>
              <span class="text-xs text-gray-500">{{ formatDateTime(msg.created_at) }}</span>
            </div>
            <p class="whitespace-pre-wrap text-sm leading-6 text-gray-700 dark:text-gray-300">{{ msg.content }}</p>
          </div>
        </div>

        <form v-if="ticket.status !== 'closed'" class="card space-y-4 p-5" @submit.prevent="submitReply">
          <label class="block space-y-2">
            <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('tickets.detail.reply') }}</span>
            <textarea v-model.trim="reply" class="input min-h-[120px] resize-y" required></textarea>
          </label>
          <div class="flex justify-end">
            <button class="btn btn-primary" type="submit" :disabled="replying">
              <Icon v-if="replying" name="refresh" size="sm" class="animate-spin" />
              <span>{{ replying ? t('tickets.actions.submitting') : t('tickets.actions.reply') }}</span>
            </button>
          </div>
        </form>

        <div v-else class="rounded-xl border border-gray-200 bg-gray-50 p-4 text-sm text-gray-500 dark:border-dark-700 dark:bg-dark-900 dark:text-dark-400">
          {{ t('tickets.detail.closedHint') }}
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import ticketsAPI from '@/api/tickets'
import type { Ticket } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const route = useRoute()
const appStore = useAppStore()
const loading = ref(true)
const replying = ref(false)
const ticket = ref<Ticket | null>(null)
const reply = ref('')

const ticketId = Number(route.params.id)

async function loadTicket() {
  loading.value = true
  try {
    ticket.value = await ticketsAPI.getMine(ticketId)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('tickets.messages.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function submitReply() {
  if (!reply.value) return
  replying.value = true
  try {
    await ticketsAPI.addMessage(ticketId, { content: reply.value })
    reply.value = ''
    await loadTicket()
    appStore.showSuccess(t('tickets.messages.replied'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('tickets.messages.replyFailed')))
  } finally {
    replying.value = false
  }
}

onMounted(loadTicket)
</script>
