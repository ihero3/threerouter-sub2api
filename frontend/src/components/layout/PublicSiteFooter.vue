<template>
  <footer class="relative z-10 border-t border-gray-100 px-6 py-8">
    <div class="mx-auto max-w-7xl">
      <div class="flex flex-col items-center justify-center gap-4 text-center text-sm text-gray-500 sm:flex-row sm:text-left">
        <p>&copy; {{ currentYear }} Three Router. {{ t('home.footer.allRightsReserved') }}</p>
        <div class="flex items-center gap-6">
          <a :href="currentLang === 'zh' ? '/readme-cn.html' : '/readme-en.html'" class="transition-colors hover:text-gray-700">{{ t('home.footer.advantage') }}</a>
        </div>
        <div class="flex items-center gap-6">
          <a :href="currentLang === 'zh' ? '/help-cn.html' : '/help-en.html'" class="transition-colors hover:text-gray-700">{{ t('home.footer.documentation') }}</a>
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
            class="transition-colors hover:text-gray-700"
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
            class="text-gray-400 transition-colors hover:text-gray-700"
          >{{ contactInfo }}</a>
          <span v-else class="text-gray-400">{{ contactInfo }}</span>
        </div>
      </div>
      <div class="mt-4 flex items-center justify-center gap-6">
        <button
          @click="router.push('/admin')"
          class="text-sm font-medium text-gray-500 transition-colors hover:text-gray-700"
        >
          {{ t('home.dashboard') }}
        </button>
        <button
          @click="router.push('/enterprise')"
          class="text-sm font-medium text-gray-500 transition-colors hover:text-gray-700"
        >
          {{ t('home.enterprise') }}
        </button>
        <button
          @click="router.push('/whitepaper')"
          class="text-sm font-medium text-gray-500 transition-colors hover:text-gray-700"
        >
          {{ t('whitepaper.type') }}
        </button>
        <button
          @click="router.push('/compliance')"
          class="text-sm font-medium text-gray-500 transition-colors hover:text-gray-700"
        >
          {{ t('home.footer.compliance') }}
        </button>
        <button
          @click="toggleLanguage"
          class="text-sm font-medium text-gray-500 transition-colors hover:text-gray-700"
        >
          {{ currentLang === 'zh' ? '中文' : 'EN' }}
        </button>
        <button
          @click="router.push('/blog')"
          class="text-sm font-medium text-gray-500 transition-colors hover:text-gray-700"
        >
          {{ t('nav.blog') }}
        </button>
      </div>
      <div class="mt-4 text-center">
        <p class="text-xs text-gray-400">{{ t('home.footer.legalNotice') }}</p>
      </div>
    </div>
  </footer>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { setLocale } from '@/i18n'
import { useAppStore } from '@/stores'
import type { CustomMenuItem } from '@/types'
import { resolveContactHref } from '@/utils/contactLink'

const { t, locale } = useI18n()
const router = useRouter()
const appStore = useAppStore()

const currentYear = computed(() => new Date().getFullYear())
const currentLang = computed<'zh' | 'en'>(() => (locale.value === 'zh' ? 'zh' : 'en'))

const contactInfo = computed(() => appStore.contactInfo)
const contactHref = computed(() => resolveContactHref(contactInfo.value))
const customMenuItems = computed<CustomMenuItem[]>(() => {
  const settings = appStore.cachedPublicSettings
  if (!settings || !settings.custom_menu_items) return []
  return settings.custom_menu_items.filter(item => item.visibility === 'user')
})

function toggleLanguage() {
  const newLang = currentLang.value === 'zh' ? 'en' : 'zh'
  setLocale(newLang)
}
</script>
