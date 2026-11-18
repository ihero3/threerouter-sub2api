<template>
  <component :is="activeHome" />
</template>

<script setup lang="ts">
import { computed, defineAsyncComponent } from 'vue'
import { useAppStore } from '@/stores'

// 两套首页完全独立：A 站沿用原英文营销首页，B 站为独立的中文首页。
// 通过后台 active_site 全局开关分流；组件按激活站点懒加载，未激活的一套不加载。
const HomeViewA = defineAsyncComponent(() => import('@/views/HomeView.vue'))
const HomeViewB = defineAsyncComponent(() => import('@/views/HomeViewB.vue'))

const appStore = useAppStore()
const activeHome = computed(() =>
  appStore.activeSite === 'site_b' ? HomeViewB : HomeViewA,
)
</script>
