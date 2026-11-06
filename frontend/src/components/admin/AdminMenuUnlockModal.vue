<template>
  <BaseDialog
    :show="show"
    :title="t('admin.menuLock.title')"
    width="narrow"
    @close="handleClose"
  >
    <form class="space-y-4" @submit.prevent="submit">
      <div class="flex items-start gap-3 rounded-lg bg-gray-50 p-3 dark:bg-dark-700">
        <span
          class="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-primary-100 text-primary-600 dark:bg-primary-900/30 dark:text-primary-400"
        >
          <Icon name="lock" size="md" />
        </span>
        <p class="text-sm leading-5 text-gray-600 dark:text-gray-300">
          {{ t('admin.menuLock.description', { menu: menuLabel }) }}
        </p>
      </div>

      <div>
        <label class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t('admin.menuLock.passwordLabel') }}
        </label>
        <input
          ref="passwordInput"
          v-model="password"
          type="password"
          autocomplete="off"
          class="input w-full"
          :placeholder="t('admin.menuLock.passwordPlaceholder')"
          :disabled="submitting"
        />
        <p v-if="errorMessage" class="mt-1.5 text-sm text-red-600 dark:text-red-400">
          {{ errorMessage }}
        </p>
      </div>
    </form>

    <template #footer>
      <button type="button" class="btn btn-secondary" :disabled="submitting" @click="handleClose">
        {{ t('common.cancel') }}
      </button>
      <button
        type="button"
        class="btn btn-primary"
        :disabled="!password.trim() || submitting"
        @click="submit"
      >
        {{ submitting ? t('common.submitting') : t('admin.menuLock.unlock') }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { unlockAdminMenu } from '@/api/admin/menuLock'

interface Props {
  show: boolean
  menuPath: string
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'unlocked'): void
}>()

const { t } = useI18n()

const password = ref('')
const submitting = ref(false)
const errorMessage = ref('')
const passwordInput = ref<HTMLInputElement | null>(null)

const MENU_LABEL_KEYS: Record<string, string> = {
  '/admin/accounts': 'nav.accounts',
  '/admin/channels': 'nav.channelManagement',
  '/admin/groups': 'nav.groups',
  '/admin/settings': 'nav.settings',
}

const menuLabel = computed(() => {
  const key = MENU_LABEL_KEYS[props.menuPath]
  return key ? t(key) : props.menuPath
})

watch(
  () => props.show,
  async (visible) => {
    if (!visible) return
    password.value = ''
    errorMessage.value = ''
    await nextTick()
    passwordInput.value?.focus()
  },
  { immediate: true }
)

function handleClose() {
  if (submitting.value) return
  emit('close')
}

async function submit() {
  const value = password.value.trim()
  if (!value || submitting.value) return

  submitting.value = true
  errorMessage.value = ''
  try {
    await unlockAdminMenu(value, props.menuPath)
    emit('unlocked')
  } catch (error) {
    const err = error as { status?: number; message?: string }
    errorMessage.value =
      err.status === 403 || err.status === 400
        ? t('admin.menuLock.incorrectPassword')
        : err.message || t('admin.menuLock.incorrectPassword')
    password.value = ''
    await nextTick()
    passwordInput.value?.focus()
  } finally {
    submitting.value = false
  }
}
</script>
