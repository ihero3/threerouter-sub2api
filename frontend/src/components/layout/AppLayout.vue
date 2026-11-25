<template>
  <div class="min-h-screen bg-gray-50 dark:bg-dark-950">
    <!-- Background Decoration -->
    <div class="pointer-events-none fixed inset-0 bg-mesh-gradient"></div>

    <!-- Sidebar (only when authenticated) -->
    <AppSidebar v-if="isAuthenticated" />

    <!-- Main Content Area -->
    <div
      class="relative min-h-screen transition-all duration-300"
      :class="[isAuthenticated ? (sidebarCollapsed ? 'lg:ml-[72px]' : 'lg:ml-64') : '']"
    >
      <!-- Header (only when authenticated) -->
      <AppHeader v-if="isAuthenticated" />

      <!-- Account disabled notice: 禁用账号仍可登录控制台，但不能调用模型 API -->
      <div
        v-if="showAccountDisabledNotice"
        class="border-b border-amber-200 bg-amber-50 dark:border-amber-900/60 dark:bg-amber-950/40"
      >
        <div class="mx-auto flex max-w-7xl flex-col gap-3 px-4 py-4 md:flex-row md:items-start md:justify-between md:px-6">
          <div class="flex gap-3">
            <svg class="mt-0.5 h-5 w-5 shrink-0 text-amber-600 dark:text-amber-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v4m0 4h.01M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0Z" />
            </svg>
            <div>
              <p class="text-sm font-semibold text-amber-900 dark:text-amber-100">
                {{ t('common.accountDisabled.title') }}
              </p>
              <p class="mt-1 text-sm text-amber-800 dark:text-amber-200">
                {{ t('common.accountDisabled.reason') }}
              </p>
              <p class="mt-1 text-xs text-amber-700 dark:text-amber-300">
                {{ t('common.accountDisabled.rechargeHint') }} {{ t('common.accountDisabled.ticketHint') }}
              </p>
            </div>
          </div>
          <div class="flex shrink-0 flex-wrap gap-2 md:pl-4">
            <button
              type="button"
              class="rounded-lg bg-amber-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-amber-700"
              @click="router.push('/purchase')"
            >
              {{ t('common.accountDisabled.recharge') }}
            </button>
            <button
              type="button"
              class="rounded-lg border border-amber-300 bg-white px-3 py-2 text-sm font-medium text-amber-800 transition-colors hover:bg-amber-100 dark:border-amber-700 dark:bg-transparent dark:text-amber-200 dark:hover:bg-amber-900/40"
              @click="router.push('/support/tickets/new')"
            >
              {{ t('common.accountDisabled.ticket') }}
            </button>
          </div>
        </div>
      </div>

      <!-- Simple header for unauthenticated users -->
      <div v-if="!isAuthenticated" class="sticky top-0 z-40 border-b border-gray-100 bg-white/80 backdrop-blur-lg dark:border-dark-700 dark:bg-dark-900/80">
        <div class="mx-auto flex h-16 max-w-7xl items-center justify-between px-4 md:px-6">
          <div class="flex items-center gap-2">
            <img :src="siteLogo" alt="Logo" class="h-8 w-8 rounded-lg" v-if="siteLogo" />
            <span class="text-lg font-bold text-gray-900 dark:text-gray-100">{{ siteName }}</span>
          </div>
          <div class="flex items-center gap-3">
            <button
              @click="router.push('/home')"
              class="rounded-lg px-3 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700"
            >
              {{ currentLang === 'zh' ? '首页' : 'Home' }}
            </button>
            <button
              @click="router.push('/login')"
              class="rounded-lg bg-primary-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-primary-700"
            >
              {{ currentLang === 'zh' ? '登录' : 'Login' }}
            </button>
            <button
              @click="toggleLanguage"
              class="rounded-lg p-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700"
            >
              {{ currentLang === 'zh' ? 'EN' : '中文' }}
            </button>
          </div>
        </div>
      </div>

      <!-- Main Content -->
      <main class="p-4 md:p-6 lg:p-8">
        <slot />
      </main>

      <!-- Footer -->
      <PublicSiteFooter />
    </div>
  </div>
</template>

<script setup lang="ts">
import '@/styles/onboarding.css'
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { useOnboardingTour } from '@/composables/useOnboardingTour'
import { useOnboardingStore } from '@/stores/onboarding'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'
import PublicSiteFooter from './PublicSiteFooter.vue'

const { locale, t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const isAdmin = computed(() => authStore.user?.role === 'admin')
const isAuthenticated = computed(() => authStore.isAuthenticated)
// 禁用账号仍可登录控制台，但需要通过横幅明确告知原因与自助解封入口。
const showAccountDisabledNotice = computed(
  () => isAuthenticated.value && authStore.user?.status === 'disabled' && !isAdmin.value
)

const siteName = computed(() => appStore.siteName)
const siteLogo = computed(() => appStore.siteLogo || '')
const currentLang = computed<'zh' | 'en'>(() => (locale.value === 'zh' ? 'zh' : 'en'))

const toggleLanguage = () => {
  locale.value = locale.value === 'zh' ? 'en' : 'zh'
}

const { replayTour } = useOnboardingTour({
  storageKey: isAdmin.value ? 'admin_guide' : 'user_guide',
  autoStart: true
})

const onboardingStore = useOnboardingStore()

onMounted(async () => {
  onboardingStore.setReplayCallback(replayTour)
  // Fetch public settings to get contact info and custom menu items
  await appStore.fetchPublicSettings()
})

defineExpose({ replayTour })
</script>
