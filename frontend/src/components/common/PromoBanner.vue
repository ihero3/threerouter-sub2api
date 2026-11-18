<template>
  <section
    class="promo-banner group relative cursor-pointer overflow-hidden rounded-3xl border border-white/10 bg-gradient-to-br from-[#0f766e] via-[#0d9488] to-[#14b8a6] p-6 text-white shadow-2xl shadow-[#0d9488]/30 transition-transform duration-200 hover:scale-[1.01] sm:p-8"
    role="button"
    tabindex="0"
    @click="goRecharge"
    @keydown.enter.prevent="goRecharge"
    @keydown.space.prevent="goRecharge"
  >
    <!-- Decorative glows -->
    <div class="pointer-events-none absolute -right-16 -top-24 h-56 w-56 rounded-full bg-[#5eead4]/30 blur-3xl"></div>
    <div class="pointer-events-none absolute -bottom-24 -left-12 h-56 w-56 rounded-full bg-[#2dd4bf]/25 blur-3xl"></div>
    <div class="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_top_right,rgba(94,234,212,0.22),transparent_55%)]"></div>

    <div class="relative">
      <div class="mb-5 flex flex-wrap items-center gap-x-3 gap-y-2">
        <span class="inline-flex items-center gap-1.5 rounded-full bg-[#5eead4] px-3 py-1 text-xs font-bold uppercase tracking-wider text-[#042f2e]">
          <span class="h-1.5 w-1.5 animate-pulse rounded-full bg-[#042f2e]"></span>
          {{ promoBadge }}
        </span>
        <h2 class="text-xl font-black leading-tight sm:text-2xl">{{ promoTitle }}</h2>
        <span class="ml-auto inline-flex items-center gap-1.5 rounded-full bg-white/15 px-3.5 py-1.5 text-xs font-semibold ring-1 ring-inset ring-white/25 backdrop-blur-sm transition-colors group-hover:bg-white/25">
          {{ promoCta }}
          <svg class="h-3.5 w-3.5 transition-transform group-hover:translate-x-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M17 8l4 4m0 0l-4 4m4-4H3" />
          </svg>
        </span>
      </div>

      <div class="promo-grid grid gap-3">
        <div
          v-for="(item, index) in promoItems"
          :key="index"
          class="flex items-center gap-3 rounded-2xl bg-white/10 px-4 py-3 ring-1 ring-inset ring-white/15 backdrop-blur-sm transition-colors hover:bg-white/15"
        >
          <span class="text-2xl leading-none">{{ item.emoji }}</span>
          <span class="text-sm font-semibold leading-snug sm:text-base">{{ item.text }}</span>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAppStore } from '@/stores'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()

// 点击广告进入后台充值页面（未登录时由路由守卫跳转到登录页）。
function goRecharge() {
  router.push('/purchase')
}

const defaultItems = [
  { key: 'newUser', emoji: '🎁' },
  { key: 'refer', emoji: '👥' },
  { key: 'topUp10', emoji: '💸' },
  { key: 'deepseek', emoji: '⚡' },
] as const

// B 站可在后台配置自己的推广文案；A 站未配置时回落到 i18n。
const sitePromo = computed(() => appStore.activeSiteProfile?.promo)
const promoBadge = computed(() => sitePromo.value?.badge?.trim() || t('promo.badge'))
const promoTitle = computed(() => sitePromo.value?.title?.trim() || t('promo.title'))
const promoCta = computed(() => sitePromo.value?.cta?.trim() || t('promo.cta'))
const promoItems = computed(() => {
  const configured = sitePromo.value?.items?.filter(i => i && i.trim()) ?? []
  if (configured.length > 0) {
    return configured.map((text, i) => ({ emoji: ['🎁', '👥', '💸', '⚡'][i % 4], text }))
  }
  return defaultItems.map(item => ({ emoji: item.emoji, text: t(`promo.items.${item.key}`) }))
})
</script>

<style scoped>
/* 使用容器查询而非视口断点：广告可能出现在窄容器（如登录卡片）里，
   按自身宽度自适应列数，避免被挤压变形。 */
.promo-banner {
  container-type: inline-size;
}

.promo-grid {
  grid-template-columns: 1fr;
}

@container (min-width: 380px) {
  .promo-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@container (min-width: 760px) {
  .promo-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}
</style>
