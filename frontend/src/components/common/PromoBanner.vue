<template>
  <section
    class="promo-banner group relative cursor-pointer overflow-hidden rounded-3xl border border-white/10 bg-gradient-to-br from-[#04285f] via-[#0757b8] to-[#0a7fc4] p-6 text-white shadow-2xl shadow-[#0757b8]/30 transition-transform duration-200 hover:scale-[1.01] sm:p-8"
    role="button"
    tabindex="0"
    @click="goRecharge"
    @keydown.enter.prevent="goRecharge"
    @keydown.space.prevent="goRecharge"
  >
    <!-- Decorative glows -->
    <div class="pointer-events-none absolute -right-16 -top-24 h-56 w-56 rounded-full bg-[#20f5b4]/30 blur-3xl"></div>
    <div class="pointer-events-none absolute -bottom-24 -left-12 h-56 w-56 rounded-full bg-[#20f5b4]/20 blur-3xl"></div>
    <div class="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_top_right,rgba(32,245,180,0.18),transparent_55%)]"></div>

    <div class="relative">
      <div class="mb-5 flex flex-wrap items-center gap-x-3 gap-y-2">
        <span class="inline-flex items-center gap-1.5 rounded-full bg-[#20f5b4] px-3 py-1 text-xs font-bold uppercase tracking-wider text-[#04285f]">
          <span class="h-1.5 w-1.5 animate-pulse rounded-full bg-[#04285f]"></span>
          {{ t('promo.badge') }}
        </span>
        <h2 class="text-xl font-black leading-tight sm:text-2xl">{{ t('promo.title') }}</h2>
        <span class="ml-auto inline-flex items-center gap-1.5 rounded-full bg-white/15 px-3.5 py-1.5 text-xs font-semibold ring-1 ring-inset ring-white/25 backdrop-blur-sm transition-colors group-hover:bg-white/25">
          {{ t('promo.cta') }}
          <svg class="h-3.5 w-3.5 transition-transform group-hover:translate-x-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M17 8l4 4m0 0l-4 4m4-4H3" />
          </svg>
        </span>
      </div>

      <div class="promo-grid grid gap-3">
        <div
          v-for="item in items"
          :key="item.key"
          class="flex items-center gap-3 rounded-2xl bg-white/10 px-4 py-3 ring-1 ring-inset ring-white/15 backdrop-blur-sm transition-colors hover:bg-white/15"
        >
          <span class="text-2xl leading-none">{{ item.emoji }}</span>
          <span class="text-sm font-semibold leading-snug sm:text-base">{{ t(`promo.items.${item.key}`) }}</span>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

const { t } = useI18n()
const router = useRouter()

// 点击广告进入后台充值页面（未登录时由路由守卫跳转到登录页）。
function goRecharge() {
  router.push('/purchase')
}

const items = [
  { key: 'newUser', emoji: '🎁' },
  { key: 'refer', emoji: '👥' },
  { key: 'topUp1', emoji: '💸' },
  { key: 'topUp10', emoji: '💸' },
  { key: 'deepseek', emoji: '⚡' },
  { key: 'freedom', emoji: '🔥' },
] as const
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
