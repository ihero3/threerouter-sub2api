<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import LogoSvg from '@/assets/icons/logo.svg'

const { t } = useI18n()
const router = useRouter()

const isDark = ref(false)
const currentLang = computed(() => {
  return localStorage.getItem('locale') || 'zh'
})

const toggleLanguage = () => {
  const newLang = currentLang.value === 'zh' ? 'en' : 'zh'
  localStorage.setItem('locale', newLang)
  location.reload()
}

const downloadPDF = () => {
  alert(t('whitepaper.download'))
}

const chapters = [
  {
    id: 'intro',
    title: t('whitepaper.chapters.intro.title'),
    description: t('whitepaper.chapters.intro.description'),
    sections: [
      { title: t('whitepaper.chapters.intro.section1'), content: t('whitepaper.chapters.intro.content1') },
      { title: t('whitepaper.chapters.intro.section2'), content: t('whitepaper.chapters.intro.content2') },
      { title: t('whitepaper.chapters.intro.section3'), content: t('whitepaper.chapters.intro.content3') },
    ]
  },
  {
    id: 'euai',
    title: t('whitepaper.chapters.euai.title'),
    description: t('whitepaper.chapters.euai.description'),
    sections: [
      { title: t('whitepaper.chapters.euai.section1'), content: t('whitepaper.chapters.euai.content1') },
      { title: t('whitepaper.chapters.euai.section2'), content: t('whitepaper.chapters.euai.content2') },
      { title: t('whitepaper.chapters.euai.section3'), content: t('whitepaper.chapters.euai.content3') },
    ]
  },
  {
    id: 'gdpr',
    title: t('whitepaper.chapters.gdpr.title'),
    description: t('whitepaper.chapters.gdpr.description'),
    sections: [
      { title: t('whitepaper.chapters.gdpr.section1'), content: t('whitepaper.chapters.gdpr.content1') },
      { title: t('whitepaper.chapters.gdpr.section2'), content: t('whitepaper.chapters.gdpr.content2') },
      { title: t('whitepaper.chapters.gdpr.section3'), content: t('whitepaper.chapters.gdpr.content3') },
    ]
  },
  {
    id: 'architecture',
    title: t('whitepaper.chapters.architecture.title'),
    description: t('whitepaper.chapters.architecture.description'),
    sections: [
      { title: t('whitepaper.chapters.architecture.section1'), content: t('whitepaper.chapters.architecture.content1') },
      { title: t('whitepaper.chapters.architecture.section2'), content: t('whitepaper.chapters.architecture.content2') },
      { title: t('whitepaper.chapters.architecture.section3'), content: t('whitepaper.chapters.architecture.content3') },
    ]
  },
  {
    id: 'implementation',
    title: t('whitepaper.chapters.implementation.title'),
    description: t('whitepaper.chapters.implementation.description'),
    sections: [
      { title: t('whitepaper.chapters.implementation.section1'), content: t('whitepaper.chapters.implementation.content1') },
      { title: t('whitepaper.chapters.implementation.section2'), content: t('whitepaper.chapters.implementation.content2') },
      { title: t('whitepaper.chapters.implementation.section3'), content: t('whitepaper.chapters.implementation.content3') },
    ]
  },
]
</script>

<template>
  <div :class="['relative flex min-h-screen flex-col overflow-hidden', isDark ? 'bg-gray-900' : 'bg-white']">
    <header class="relative z-20 px-6 py-4 border-b" :class="isDark ? 'border-gray-800' : 'border-gray-100'">
      <nav class="mx-auto flex max-w-6xl items-center justify-between">
        <div class="flex items-center gap-3">
          <img :src="LogoSvg" alt="Three Router Logo" class="h-8 w-8 object-contain" />
          <span :class="['text-lg font-semibold', isDark ? 'text-white' : 'text-[#021b4a]']">ThreeRouter</span>
        </div>
        <div class="flex items-center gap-4">
          <button @click="toggleLanguage" :class="['rounded-lg px-4 py-2 text-sm font-medium transition-colors', isDark ? 'text-gray-300 hover:bg-gray-800' : 'text-gray-700 hover:bg-gray-100']">
            {{ currentLang === 'zh' ? 'EN' : '中文' }}
          </button>
          <button @click="router.push('/compliance')" :class="['rounded-lg px-5 py-2 text-sm font-semibold shadow-lg transition-colors', isDark ? 'bg-blue-600 text-white hover:bg-blue-700' : 'bg-[#0757b8] text-white hover:bg-[#064ea8]']">
            {{ t('whitepaper.back') }}
          </button>
        </div>
      </nav>
    </header>

    <main class="relative z-10 flex-1 px-6 py-10">
      <div class="mx-auto max-w-5xl">
        <div :class="['mb-12 rounded-2xl p-10 text-center', isDark ? 'bg-gray-800/50' : 'bg-gradient-to-r from-blue-50 to-purple-50']">
          <div :class="['mb-6 inline-flex items-center gap-2 rounded-full px-5 py-2.5', isDark ? 'bg-blue-900/30' : 'bg-blue-100']">
            <span :class="['h-2.5 w-2.5 rounded-full', isDark ? 'bg-blue-400' : 'bg-blue-500']"></span>
            <span :class="['text-sm font-semibold uppercase tracking-wider', isDark ? 'text-blue-300' : 'text-blue-700']">{{ t('whitepaper.type') }}</span>
          </div>
          <h1 :class="['mb-4 text-4xl font-bold', isDark ? 'text-white' : 'text-gray-900']">
            {{ t('whitepaper.title') }}
          </h1>
          <p :class="['mb-8 max-w-3xl mx-auto text-lg', isDark ? 'text-gray-400' : 'text-gray-600']">
            {{ t('whitepaper.subtitle') }}
          </p>
          <div class="flex flex-col gap-4 sm:flex-row justify-center">
            <button @click="downloadPDF" :class="['rounded-xl px-8 py-4 text-lg font-semibold shadow-xl transition-all', isDark ? 'bg-blue-600 text-white hover:bg-blue-700' : 'bg-[#0757b8] text-white hover:bg-[#064ea8]']">
              {{ t('whitepaper.downloadBtn') }}
            </button>
          </div>
        </div>

        <div class="grid grid-cols-1 lg:grid-cols-12 gap-8">
          <div class="lg:col-span-3">
            <div :class="['sticky top-10 rounded-xl p-6', isDark ? 'bg-gray-800/50' : 'bg-gray-50']">
              <h3 :class="['mb-4 text-lg font-semibold', isDark ? 'text-white' : 'text-gray-900']">{{ t('whitepaper.toc') }}</h3>
              <nav class="space-y-2">
                <a v-for="chapter in chapters" :key="chapter.id" :href="'#' + chapter.id" :class="['block rounded-lg px-4 py-3 text-sm transition-colors', isDark ? 'text-gray-300 hover:bg-gray-700' : 'text-gray-700 hover:bg-gray-100']">
                  {{ chapter.title }}
                </a>
              </nav>
            </div>
          </div>

          <div class="lg:col-span-9 space-y-12">
            <section v-for="chapter in chapters" :key="chapter.id" :id="chapter.id">
              <div :class="['rounded-xl p-8', isDark ? 'bg-gray-800/30' : 'bg-white']" :style="{ boxShadow: isDark ? '0 2px 12px rgba(0,0,0,0.2)' : '0 2px 12px rgba(59,130,246,0.05)' }">
                <h2 :class="['mb-4 text-2xl font-bold', isDark ? 'text-white' : 'text-gray-900']">{{ chapter.title }}</h2>
                <p :class="['mb-6 text-sm', isDark ? 'text-gray-400' : 'text-gray-600']">{{ chapter.description }}</p>
                
                <div class="space-y-6">
                  <div v-for="(section, index) in chapter.sections" :key="index">
                    <h3 :class="['mb-2 text-lg font-semibold', isDark ? 'text-gray-200' : 'text-gray-800']">{{ section.title }}</h3>
                    <p :class="['text-sm leading-relaxed', isDark ? 'text-gray-400' : 'text-gray-600']">{{ section.content }}</p>
                  </div>
                </div>
              </div>
            </section>
          </div>
        </div>

        <footer :class="['mt-12 rounded-xl p-8 text-center', isDark ? 'bg-gray-800/30' : 'bg-gray-50']">
          <div class="mb-4 flex items-center justify-center gap-3">
            <img :src="LogoSvg" alt="Three Router Logo" class="h-6 w-6 object-contain" />
            <span :class="['text-lg font-semibold', isDark ? 'text-white' : 'text-[#021b4a]']">ThreeRouter</span>
          </div>
          <p :class="['text-sm', isDark ? 'text-gray-500' : 'text-gray-500']">{{ t('whitepaper.footer') }}</p>
        </footer>
      </div>
    </main>
  </div>
</template>