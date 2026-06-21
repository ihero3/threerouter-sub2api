<template>
  <div class="min-h-screen bg-gray-50 dark:bg-dark-950">
    <!-- Background Decoration -->
    <div class="pointer-events-none fixed inset-0 bg-mesh-gradient"></div>

    <!-- Sidebar -->
    <AppSidebar />

    <!-- Main Content Area -->
    <div
      class="relative min-h-screen transition-all duration-300"
      :class="[sidebarCollapsed ? 'lg:ml-[72px]' : 'lg:ml-64']"
    >
      <!-- Header -->
      <AppHeader />

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
              <a :href="currentLang === 'zh' ? 'readme-cn.html' : 'readme-en.html'" class="transition-colors hover:text-gray-700 dark:hover:text-gray-300">{{ t('home.footer.advantage') }}</a>
            </div>
            <div class="flex items-center gap-6">
              <a :href="currentLang === 'zh' ? 'help-cn.html' : 'help-en.html'" class="transition-colors hover:text-gray-700 dark:hover:text-gray-300">{{ t('home.footer.documentation') }}</a>
            </div>
              <div class="flex items-center gap-6">
            <a :href="currentLang === 'zh' ? 'help-cn.html#contact' : 'help-en.html#contact'" class="transition-colors hover:text-gray-700">{{ t('home.footer.contact') }}</a>
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
              <span class="text-gray-400 dark:text-gray-500">{{ contactInfo }}</span>
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
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { useOnboardingTour } from '@/composables/useOnboardingTour'
import { useOnboardingStore } from '@/stores/onboarding'
import type { CustomMenuItem } from '@/types'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const isAdmin = computed(() => authStore.user?.role === 'admin')

const currentYear = computed(() => new Date().getFullYear())
const siteName = computed(() => appStore.siteName)
const contactInfo = computed(() => appStore.contactInfo)
const currentLang = computed(() => (localStorage.getItem('sub2api_locale') as 'zh' | 'en') || 'en')
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
