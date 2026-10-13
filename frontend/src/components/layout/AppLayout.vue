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

      <!-- Simple header for unauthenticated users -->
      <div v-else class="sticky top-0 z-40 border-b border-gray-100 bg-white/80 backdrop-blur-lg dark:border-dark-700 dark:bg-dark-900/80">
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

const { locale } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const isAdmin = computed(() => authStore.user?.role === 'admin')
const isAuthenticated = computed(() => authStore.isAuthenticated)

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
