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
      <footer class="border-t border-gray-100 dark:border-dark-700 px-6 py-8">
        <div class="mx-auto max-w-7xl">
          <div class="flex flex-col items-center justify-center gap-4 text-center text-sm text-gray-500 dark:text-gray-400 sm:flex-row sm:text-left">
            <p>&copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}</p>
            <div class="flex items-center gap-6">
              <a :href="currentLang === 'zh' ? '/readme-cn.html' : '/readme-en.html'" class="transition-colors hover:text-gray-700 dark:hover:text-gray-300">{{ t('home.footer.advantage') }}</a>
            </div>
            <div class="flex items-center gap-6">
              <a :href="currentLang === 'zh' ? '/help-cn.html' : '/help-en.html'" class="transition-colors hover:text-gray-700 dark:hover:text-gray-300">{{ t('home.footer.documentation') }}</a>
            </div>
              <div class="flex items-center gap-6">
            <a :href="currentLang === 'zh' ? '/help-cn.html#contact' : '/help-en.html#contact'" class="transition-colors hover:text-gray-700">{{ t('home.footer.contact') }}</a>
          </div>
            <!-- Custom Menu Items -->
            <div v-if="customMenuItems.length > 0" class="flex items-center gap-6">
              <a
                v-for="item in customMenuItems"
                :key="item.id"
                :href="item.url"
                class="transition-colors hover:text-gray-700 dark:hover:text-gray-300"
                target="_blank"
                rel="noopener noreferrer"
              >
                {{ item.label }}
              </a>
            </div>
            <!-- Contact Info -->
            <div v-if="contactInfo" class="flex items-center gap-6">
              <a
                v-if="contactHref"
                :href="contactHref"
                target="_blank"
                rel="noopener noreferrer"
                class="text-gray-400 transition-colors hover:text-gray-700 dark:text-gray-500 dark:hover:text-gray-300"
              >{{ contactInfo }}</a>
              <span v-else class="text-gray-400 dark:text-gray-500">{{ contactInfo }}</span>
            </div>
          </div>
        </div>
      </footer>
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
import type { CustomMenuItem } from '@/types'
import { resolveContactHref } from '@/utils/contactLink'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'

const { t, locale } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const isAdmin = computed(() => authStore.user?.role === 'admin')
const isAuthenticated = computed(() => authStore.isAuthenticated)

const currentYear = computed(() => new Date().getFullYear())
const siteName = computed(() => appStore.siteName)
const siteLogo = computed(() => appStore.siteLogo || '')
const contactInfo = computed(() => appStore.contactInfo)
const contactHref = computed(() => resolveContactHref(contactInfo.value))
const currentLang = computed<'zh' | 'en'>(() => (locale.value === 'zh' ? 'zh' : 'en'))

const toggleLanguage = () => {
  locale.value = locale.value === 'zh' ? 'en' : 'zh'
}
const customMenuItems = computed<CustomMenuItem[]>(() => {
  const settings = appStore.cachedPublicSettings
  if (!settings || !settings.custom_menu_items) return []
  // Filter based on user role
  return settings.custom_menu_items.filter(item => 
    isAdmin.value ? item.visibility === 'admin' : item.visibility === 'user'
  )
})

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
