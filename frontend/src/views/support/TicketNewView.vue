<template>
  <AppLayout>
    <div class="mx-auto max-w-3xl space-y-6">
      <div>
        <h1 class="text-2xl font-bold text-gray-900 dark:text-white">{{ t('tickets.new.title') }}</h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('tickets.new.description') }}</p>
      </div>

      <form class="card space-y-5 p-6" @submit.prevent="submitTicket">
        <div class="grid gap-4 md:grid-cols-2">
          <label class="space-y-2">
            <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('tickets.fields.contact') }}</span>
            <input v-model.trim="form.contact" class="input" type="text" :placeholder="t('tickets.placeholders.contact')" required />
          </label>
          <label class="space-y-2">
            <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('tickets.fields.title') }}</span>
            <input v-model.trim="form.title" class="input" type="text" :placeholder="t('tickets.placeholders.title')" maxlength="200" required />
          </label>
        </div>

        <div class="grid gap-4 md:grid-cols-2">
          <label class="space-y-2">
            <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('tickets.fields.category') }}</span>
            <select v-model="form.category" class="input">
              <option v-for="item in categoryOptions" :key="item" :value="item">{{ t(`tickets.categories.${item}`) }}</option>
            </select>
          </label>
          <label class="space-y-2">
            <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('tickets.fields.priority') }}</span>
            <select v-model="form.priority" class="input">
              <option v-for="item in priorityOptions" :key="item" :value="item">{{ t(`tickets.priorities.${item}`) }}</option>
            </select>
          </label>
        </div>

        <label class="space-y-2 block">
          <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('tickets.fields.content') }}</span>
          <textarea v-model.trim="form.content" class="input min-h-[180px] resize-y" :placeholder="t('tickets.placeholders.content')" required></textarea>
        </label>

        <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <p class="text-xs text-gray-500 dark:text-dark-400">
            {{ authStore.isAuthenticated ? t('tickets.new.loggedHint') : t('tickets.new.guestHint') }}
          </p>
          <button class="btn btn-primary" type="submit" :disabled="submitting">
            <Icon v-if="submitting" name="refresh" size="sm" class="animate-spin" />
            <span>{{ submitting ? t('tickets.actions.submitting') : t('tickets.actions.submit') }}</span>
          </button>
        </div>
      </form>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import ticketsAPI from '@/api/tickets'
import { useAppStore, useAuthStore } from '@/stores'
import type { TicketCategory, TicketPriority } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()

const categoryOptions: TicketCategory[] = ['account', 'billing', 'api', 'model', 'other']
const priorityOptions: TicketPriority[] = ['low', 'normal', 'high', 'urgent']
const submitting = ref(false)

const form = reactive({
  contact: authStore.user?.email || '',
  title: '',
  category: 'other' as TicketCategory,
  priority: 'normal' as TicketPriority,
  content: ''
})

async function submitTicket() {
  submitting.value = true
  try {
    const ticket = await ticketsAPI.create({ ...form }, authStore.isAuthenticated)
    appStore.showSuccess(t('tickets.messages.created'))
    if (authStore.isAuthenticated) {
      await router.push(`/support/tickets/${ticket.id}`)
    } else {
      await router.push('/home')
    }
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('tickets.messages.createFailed')))
  } finally {
    submitting.value = false
  }
}
</script>
