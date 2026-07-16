<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import LogoSvg from '@/assets/icons/logo.svg'
import { setLocale } from '@/i18n'

const { t, locale } = useI18n()
const router = useRouter()

const isDark = ref(false)
const currentLang = computed(() => {
  return locale.value
})

const toggleLanguage = async () => {
  const newLang = locale.value === 'zh' ? 'en' : 'zh'
  await setLocale(newLang)
}

const toggleTheme = () => {
  isDark.value = !isDark.value
}

const features = computed(() => [
  {
    icon: 'shield-check',
    title: t('compliance.features.euai.title'),
    description: t('compliance.features.euai.description'),
    highlights: [
      t('compliance.features.euai.highlight1'),
      t('compliance.features.euai.highlight2'),
      t('compliance.features.euai.highlight3'),
      t('compliance.features.euai.highlight4')
    ],
    color: 'blue'
  },
  {
    icon: 'file-check',
    title: t('compliance.features.gdpr.title'),
    description: t('compliance.features.gdpr.description'),
    highlights: [
      t('compliance.features.gdpr.highlight1'),
      t('compliance.features.gdpr.highlight2'),
      t('compliance.features.gdpr.highlight3'),
      t('compliance.features.gdpr.highlight4')
    ],
    color: 'purple'
  },
  {
    icon: 'lock',
    title: t('compliance.features.zdr.title'),
    description: t('compliance.features.zdr.description'),
    highlights: [
      t('compliance.features.zdr.highlight1'),
      t('compliance.features.zdr.highlight2'),
      t('compliance.features.zdr.highlight3'),
      t('compliance.features.zdr.highlight4')
    ],
    color: 'green'
  },
  {
    icon: 'heart-pulse',
    title: t('compliance.features.hipaa.title'),
    description: t('compliance.features.hipaa.description'),
    highlights: [
      t('compliance.features.hipaa.highlight1'),
      t('compliance.features.hipaa.highlight2'),
      t('compliance.features.hipaa.highlight3'),
      t('compliance.features.hipaa.highlight4')
    ],
    color: 'rose'
  },
  {
    icon: 'award',
    title: t('compliance.features.credentials.title'),
    description: t('compliance.features.credentials.description'),
    highlights: [
      t('compliance.features.credentials.highlight1'),
      t('compliance.features.credentials.highlight2'),
      t('compliance.features.credentials.highlight3'),
      t('compliance.features.credentials.highlight4')
    ],
    color: 'orange'
  },
  {
    icon: 'layers',
    title: t('compliance.features.templates.title'),
    description: t('compliance.features.templates.description'),
    highlights: [
      t('compliance.features.templates.highlight1'),
      t('compliance.features.templates.highlight2'),
      t('compliance.features.templates.highlight3'),
      t('compliance.features.templates.highlight4')
    ],
    color: 'cyan'
  },
  {
    icon: 'bar-chart',
    title: t('compliance.features.risk.title'),
    description: t('compliance.features.risk.description'),
    highlights: [
      t('compliance.features.risk.highlight1'),
      t('compliance.features.risk.highlight2'),
      t('compliance.features.risk.highlight3'),
      t('compliance.features.risk.highlight4')
    ],
    color: 'red'
  }
])

const complianceCertificates = computed(() => [
  { name: 'GDPR_COMPLIANCE', description: t('compliance.certificates.gdpr') },
  { name: 'EU_AI_ACT_ASSESSMENT', description: t('compliance.certificates.euai') },
  { name: 'ZERO_DATA_RETENTION', description: t('compliance.certificates.zdr') },
  { name: 'DPA_COMPLIANCE', description: t('compliance.certificates.dpa') },
  { name: 'HIPAA_COMPLIANCE', description: t('compliance.certificates.hipaa') },
  { name: 'SECURITY_CERTIFICATION', description: t('compliance.certificates.security') }
])

const industryTemplates = computed(() => [
  { name: 'healthcare', label: t('compliance.templates.healthcare'), description: t('compliance.templates.healthcareDesc') },
  { name: 'finance', label: t('compliance.templates.finance'), description: t('compliance.templates.financeDesc') },
  { name: 'education', label: t('compliance.templates.education'), description: t('compliance.templates.educationDesc') },
  { name: 'ecommerce', label: t('compliance.templates.ecommerce'), description: t('compliance.templates.ecommerceDesc') }
])
</script>

<template>
  <div :class="['relative flex min-h-screen flex-col overflow-hidden', isDark ? 'bg-gray-900' : 'bg-gradient-to-br from-slate-50 via-blue-50/30 to-cyan-50']">
    <div v-if="!isDark" class="pointer-events-none absolute inset-0 overflow-hidden">
      <div class="absolute right-0 top-0 h-96 w-96 bg-gradient-to-br from-blue-500/20 to-cyan-400/20 blur-3xl"></div>
      <div class="absolute -bottom-40 left-0 h-96 w-96 bg-gradient-to-br from-emerald-400/15 to-teal-500/15 blur-3xl"></div>
      <div class="absolute inset-0 bg-[linear-gradient(rgba(59,130,246,0.03)_1px,transparent_1px),linear-gradient(90deg,rgba(59,130,246,0.03)_1px,transparent_1px)] bg-[size:64px_64px]"></div>
    </div>

    <header class="relative z-20 px-6 py-4">
      <nav class="mx-auto flex max-w-7xl items-center justify-between">
        <div class="flex items-center gap-3">
          <img :src="LogoSvg" alt="Three Router Logo" class="h-8 w-8 object-contain" />
          <span :class="['text-lg font-semibold', isDark ? 'text-white' : 'text-[#021b4a]']">ThreeRouter</span>
        </div>
        <div class="flex items-center gap-4">
          <button @click="toggleLanguage" :class="['rounded-lg px-4 py-2 text-sm font-medium transition-colors', isDark ? 'text-gray-300 hover:bg-gray-800' : 'text-gray-700 hover:bg-gray-100']">
            {{ currentLang === 'zh' ? 'EN' : '中文' }}
          </button>
          <button @click="toggleTheme" :class="['rounded-lg p-2 transition-colors', isDark ? 'text-gray-300 hover:bg-gray-800' : 'text-gray-500 hover:bg-gray-100']">
            <svg v-if="isDark" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
            </svg>
            <svg v-else class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
            </svg>
          </button>
          <button @click="router.push('/login')" :class="['rounded-lg px-5 py-2 text-sm font-semibold shadow-lg transition-colors', isDark ? 'bg-blue-600 text-white hover:bg-blue-700 shadow-blue-900/30' : 'bg-[#0757b8] text-white hover:bg-[#064ea8] shadow-blue-900/10']">
            {{ t('compliance.login') }}
          </button>
        </div>
      </nav>
    </header>

    <main class="relative z-10 flex-1 px-6 pt-16">
      <div class="mx-auto flex max-w-7xl flex-col">
        <div class="mb-16 flex flex-col items-center text-center">
          <div :class="['mb-6 inline-flex items-center gap-2 rounded-full px-5 py-2.5', isDark ? 'bg-blue-900/30' : 'bg-gradient-to-r from-blue-100/50 to-cyan-100/50']">
            <span :class="['h-2.5 w-2.5 animate-pulse rounded-full', isDark ? 'bg-blue-400' : 'bg-blue-500']"></span>
            <span :class="['text-base font-semibold', isDark ? 'text-blue-300' : 'text-blue-700']">{{ t('compliance.hero.badge') }}</span>
          </div>
          <h1 :class="['mb-5 text-4xl font-bold md:text-5xl lg:text-6xl', isDark ? 'text-white' : 'text-gray-900']">
            {{ t('compliance.hero.title') }}
          </h1>
          <p :class="['mb-10 max-w-3xl text-xl', isDark ? 'text-gray-400' : 'text-gray-600']">
            {{ t('compliance.hero.subtitle') }}
          </p>
          <div class="mb-12 flex flex-col gap-4 sm:flex-row">
            <button @click="router.push('/governance')" :class="['rounded-xl px-8 py-4 text-lg font-semibold shadow-xl transition-all', isDark ? 'bg-blue-600 text-white hover:bg-blue-700 hover:shadow-blue-900/40' : 'bg-[#0757b8] text-white hover:bg-[#064ea8] hover:shadow-blue-900/20']">
              {{ t('compliance.hero.ctaPrimary') }}
            </button>
            <button @click="router.push('/home')" :class="['rounded-xl px-8 py-4 text-lg font-semibold transition-all', isDark ? 'bg-gray-800 text-gray-200 hover:bg-gray-700' : 'bg-white text-gray-700 hover:bg-gray-50']">
              {{ t('compliance.hero.ctaSecondary') }}
            </button>
          </div>
        </div>

        <div class="mb-20 grid grid-cols-1 gap-8 md:grid-cols-2 lg:grid-cols-3">
          <div v-for="(feature, index) in features" :key="index" :class="['group relative rounded-2xl p-8 transition-all duration-300', isDark ? 'bg-gray-800/50 hover:bg-gray-800' : 'bg-white/80 hover:bg-white']" :style="{ boxShadow: isDark ? '0 4px 24px rgba(0,0,0,0.3)' : '0 4px 24px rgba(59,130,246,0.08)' }">
            <div :class="['mb-5 inline-flex h-12 w-12 items-center justify-center rounded-xl', 
              feature.color === 'blue' ? (isDark ? 'bg-blue-900/40' : 'bg-blue-100') :
              feature.color === 'purple' ? (isDark ? 'bg-purple-900/40' : 'bg-purple-100') :
              feature.color === 'green' ? (isDark ? 'bg-green-900/40' : 'bg-green-100') :
              feature.color === 'rose' ? (isDark ? 'bg-rose-900/40' : 'bg-rose-100') :
              feature.color === 'orange' ? (isDark ? 'bg-orange-900/40' : 'bg-orange-100') :
              feature.color === 'cyan' ? (isDark ? 'bg-cyan-900/40' : 'bg-cyan-100') :
              (isDark ? 'bg-red-900/40' : 'bg-red-100')
            ]">
              <svg v-if="feature.icon === 'shield-check'" :class="['h-6 w-6', feature.color === 'blue' ? (isDark ? 'text-blue-400' : 'text-blue-600') : feature.color === 'purple' ? (isDark ? 'text-purple-400' : 'text-purple-600') : feature.color === 'green' ? (isDark ? 'text-green-400' : 'text-green-600') : feature.color === 'orange' ? (isDark ? 'text-orange-400' : 'text-orange-600') : feature.color === 'cyan' ? (isDark ? 'text-cyan-400' : 'text-cyan-600') : (isDark ? 'text-red-400' : 'text-red-600')]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
              </svg>
              <svg v-else-if="feature.icon === 'file-check'" :class="['h-6 w-6', feature.color === 'blue' ? (isDark ? 'text-blue-400' : 'text-blue-600') : feature.color === 'purple' ? (isDark ? 'text-purple-400' : 'text-purple-600') : feature.color === 'green' ? (isDark ? 'text-green-400' : 'text-green-600') : feature.color === 'orange' ? (isDark ? 'text-orange-400' : 'text-orange-600') : feature.color === 'cyan' ? (isDark ? 'text-cyan-400' : 'text-cyan-600') : (isDark ? 'text-red-400' : 'text-red-600')]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
              <svg v-else-if="feature.icon === 'lock'" :class="['h-6 w-6', feature.color === 'blue' ? (isDark ? 'text-blue-400' : 'text-blue-600') : feature.color === 'purple' ? (isDark ? 'text-purple-400' : 'text-purple-600') : feature.color === 'green' ? (isDark ? 'text-green-400' : 'text-green-600') : feature.color === 'rose' ? (isDark ? 'text-rose-400' : 'text-rose-600') : feature.color === 'orange' ? (isDark ? 'text-orange-400' : 'text-orange-600') : feature.color === 'cyan' ? (isDark ? 'text-cyan-400' : 'text-cyan-600') : (isDark ? 'text-red-400' : 'text-red-600')]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
              </svg>
              <svg v-else-if="feature.icon === 'heart-pulse'" :class="['h-6 w-6', feature.color === 'blue' ? (isDark ? 'text-blue-400' : 'text-blue-600') : feature.color === 'purple' ? (isDark ? 'text-purple-400' : 'text-purple-600') : feature.color === 'green' ? (isDark ? 'text-green-400' : 'text-green-600') : feature.color === 'rose' ? (isDark ? 'text-rose-400' : 'text-rose-600') : feature.color === 'orange' ? (isDark ? 'text-orange-400' : 'text-orange-600') : feature.color === 'cyan' ? (isDark ? 'text-cyan-400' : 'text-cyan-600') : (isDark ? 'text-red-400' : 'text-red-600')]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z" />
              </svg>
              <svg v-else-if="feature.icon === 'award'" :class="['h-6 w-6', feature.color === 'blue' ? (isDark ? 'text-blue-400' : 'text-blue-600') : feature.color === 'purple' ? (isDark ? 'text-purple-400' : 'text-purple-600') : feature.color === 'green' ? (isDark ? 'text-green-400' : 'text-green-600') : feature.color === 'orange' ? (isDark ? 'text-orange-400' : 'text-orange-600') : feature.color === 'cyan' ? (isDark ? 'text-cyan-400' : 'text-cyan-600') : (isDark ? 'text-red-400' : 'text-red-600')]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 3v4M3 5h4M6 17v4m-2-2h4m5-16l2.286 6.857L21 12l-5.714 2.143L13 21l-2.286-6.857L5 12l5.714-2.143L13 3z" />
              </svg>
              <svg v-else-if="feature.icon === 'layers'" :class="['h-6 w-6', feature.color === 'blue' ? (isDark ? 'text-blue-400' : 'text-blue-600') : feature.color === 'purple' ? (isDark ? 'text-purple-400' : 'text-purple-600') : feature.color === 'green' ? (isDark ? 'text-green-400' : 'text-green-600') : feature.color === 'orange' ? (isDark ? 'text-orange-400' : 'text-orange-600') : feature.color === 'cyan' ? (isDark ? 'text-cyan-400' : 'text-cyan-600') : (isDark ? 'text-red-400' : 'text-red-600')]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
              </svg>
              <svg v-else :class="['h-6 w-6', feature.color === 'blue' ? (isDark ? 'text-blue-400' : 'text-blue-600') : feature.color === 'purple' ? (isDark ? 'text-purple-400' : 'text-purple-600') : feature.color === 'green' ? (isDark ? 'text-green-400' : 'text-green-600') : feature.color === 'orange' ? (isDark ? 'text-orange-400' : 'text-orange-600') : feature.color === 'cyan' ? (isDark ? 'text-cyan-400' : 'text-cyan-600') : (isDark ? 'text-red-400' : 'text-red-600')]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
              </svg>
            </div>
            <h3 :class="['mb-3 text-xl font-semibold', isDark ? 'text-white' : 'text-gray-900']">{{ feature.title }}</h3>
            <p :class="['mb-4 text-sm', isDark ? 'text-gray-400' : 'text-gray-600']">{{ feature.description }}</p>
            <ul class="space-y-2">
              <li v-for="(highlight, hIndex) in feature.highlights" :key="hIndex" :class="['flex items-start gap-2 text-sm', isDark ? 'text-gray-300' : 'text-gray-700']">
                <span :class="['mt-1 h-1.5 w-1.5 flex-shrink-0 rounded-full', feature.color === 'blue' ? 'bg-blue-500' : feature.color === 'purple' ? 'bg-purple-500' : feature.color === 'green' ? 'bg-green-500' : feature.color === 'orange' ? 'bg-orange-500' : feature.color === 'cyan' ? 'bg-cyan-500' : 'bg-red-500']"></span>
                {{ highlight }}
              </li>
            </ul>
          </div>
        </div>

        <div :class="['mb-20 rounded-2xl p-10', isDark ? 'bg-gray-800/50' : 'bg-white/80']" :style="{ boxShadow: isDark ? '0 4px 24px rgba(0,0,0,0.3)' : '0 4px 24px rgba(59,130,246,0.08)' }">
          <div class="mb-8 text-center">
            <h2 :class="['mb-4 text-3xl font-bold', isDark ? 'text-white' : 'text-gray-900']">{{ t('compliance.certificates.title') }}</h2>
            <p :class="['text-lg', isDark ? 'text-gray-400' : 'text-gray-600']">{{ t('compliance.certificates.description') }}</p>
          </div>
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-5">
            <div v-for="(cert, index) in complianceCertificates" :key="index" :class="['rounded-xl p-6 text-center transition-colors', isDark ? 'bg-gray-700/50 hover:bg-gray-700' : 'bg-gray-50 hover:bg-gray-100']">
              <div :class="['mx-auto mb-4 flex h-10 w-10 items-center justify-center rounded-full', index === 0 ? (isDark ? 'bg-blue-900/40' : 'bg-blue-100') : index === 1 ? (isDark ? 'bg-purple-900/40' : 'bg-purple-100') : index === 2 ? (isDark ? 'bg-green-900/40' : 'bg-green-100') : index === 3 ? (isDark ? 'bg-orange-900/40' : 'bg-orange-100') : (isDark ? 'bg-cyan-900/40' : 'bg-cyan-100')]">
                <svg v-if="index === 0" :class="['h-5 w-5', isDark ? 'text-blue-400' : 'text-blue-600']" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <svg v-else-if="index === 1" :class="['h-5 w-5', isDark ? 'text-purple-400' : 'text-purple-600']" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4" />
                </svg>
                <svg v-else-if="index === 2" :class="['h-5 w-5', isDark ? 'text-green-400' : 'text-green-600']" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
                </svg>
                <svg v-else-if="index === 3" :class="['h-5 w-5', isDark ? 'text-orange-400' : 'text-orange-600']" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                </svg>
                <svg v-else :class="['h-5 w-5', isDark ? 'text-cyan-400' : 'text-cyan-600']" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                </svg>
              </div>
              <div :class="['mb-2 text-xs font-semibold uppercase tracking-wider', isDark ? 'text-gray-500' : 'text-gray-400']">{{ cert.name }}</div>
              <div :class="['text-sm', isDark ? 'text-gray-300' : 'text-gray-700']">{{ cert.description }}</div>
            </div>
          </div>
        </div>

        <div :class="['mb-20 rounded-2xl p-10', isDark ? 'bg-gray-800/50' : 'bg-white/80']" :style="{ boxShadow: isDark ? '0 4px 24px rgba(0,0,0,0.3)' : '0 4px 24px rgba(59,130,246,0.08)' }">
          <div class="mb-8 text-center">
            <h2 :class="['mb-4 text-3xl font-bold', isDark ? 'text-white' : 'text-gray-900']">{{ t('compliance.samples.title') }}</h2>
            <p :class="['text-lg', isDark ? 'text-gray-400' : 'text-gray-600']">{{ t('compliance.samples.description') }}</p>
          </div>
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div @click="router.push('/compliance/eu-ai-act-report')" :class="['cursor-pointer rounded-xl p-6 transition-all', isDark ? 'bg-gray-700/50 hover:bg-gray-700 hover:border-blue-500' : 'bg-gray-50 hover:bg-gray-100 hover:border-blue-500']" style="border: 2px solid transparent;">
              <div class="mb-4 flex items-center justify-between">
                <div class="flex items-center gap-3">
                  <div :class="['flex h-10 w-10 items-center justify-center rounded-lg', isDark ? 'bg-purple-900/40' : 'bg-purple-100']">
                    <svg :class="['h-5 w-5', isDark ? 'text-purple-400' : 'text-purple-600']" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4" />
                    </svg>
                  </div>
                  <div>
                    <h3 :class="['font-semibold', isDark ? 'text-white' : 'text-gray-900']">{{ t('compliance.samples.euAiAct.title') }}</h3>
                    <p :class="['text-xs', isDark ? 'text-gray-500' : 'text-gray-500']">{{ t('compliance.samples.euAiAct.description') }}</p>
                  </div>
                </div>
                <svg :class="['h-5 w-5', isDark ? 'text-gray-500' : 'text-gray-400']" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                </svg>
              </div>
            </div>
            <div @click="router.push('/compliance/gdpr-ropa-report')" :class="['cursor-pointer rounded-xl p-6 transition-all', isDark ? 'bg-gray-700/50 hover:bg-gray-700 hover:border-green-500' : 'bg-gray-50 hover:bg-gray-100 hover:border-green-500']" style="border: 2px solid transparent;">
              <div class="mb-4 flex items-center justify-between">
                <div class="flex items-center gap-3">
                  <div :class="['flex h-10 w-10 items-center justify-center rounded-lg', isDark ? 'bg-green-900/40' : 'bg-green-100']">
                    <svg :class="['h-5 w-5', isDark ? 'text-green-400' : 'text-green-600']" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                    </svg>
                  </div>
                  <div>
                    <h3 :class="['font-semibold', isDark ? 'text-white' : 'text-gray-900']">{{ t('compliance.samples.gdprRopa.title') }}</h3>
                    <p :class="['text-xs', isDark ? 'text-gray-500' : 'text-gray-500']">{{ t('compliance.samples.gdprRopa.description') }}</p>
                  </div>
                </div>
                <svg :class="['h-5 w-5', isDark ? 'text-gray-500' : 'text-gray-400']" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                </svg>
              </div>
            </div>
          </div>
        </div>

        <div :class="['mb-20 rounded-2xl p-10', isDark ? 'bg-gray-800/50' : 'bg-white/80']" :style="{ boxShadow: isDark ? '0 4px 24px rgba(0,0,0,0.3)' : '0 4px 24px rgba(59,130,246,0.08)' }">
          <div class="mb-8 text-center">
            <h2 :class="['mb-4 text-3xl font-bold', isDark ? 'text-white' : 'text-gray-900']">{{ t('compliance.templates.title') }}</h2>
            <p :class="['text-lg', isDark ? 'text-gray-400' : 'text-gray-600']">{{ t('compliance.templates.description') }}</p>
          </div>
          <div class="grid grid-cols-1 gap-6 md:grid-cols-2">
            <div v-for="(template, index) in industryTemplates" :key="index" :class="['rounded-xl p-6 transition-colors', isDark ? 'bg-gray-700/50 hover:bg-gray-700' : 'bg-gray-50 hover:bg-gray-100']">
              <div class="mb-4 flex items-center justify-between">
                <div :class="['inline-flex items-center gap-2 rounded-full px-4 py-1.5', index === 0 ? (isDark ? 'bg-green-900/30' : 'bg-green-100') : index === 1 ? (isDark ? 'bg-blue-900/30' : 'bg-blue-100') : index === 2 ? (isDark ? 'bg-purple-900/30' : 'bg-purple-100') : (isDark ? 'bg-orange-900/30' : 'bg-orange-100')]">
                  <svg :class="['h-4 w-4', index === 0 ? (isDark ? 'text-green-400' : 'text-green-600') : index === 1 ? (isDark ? 'text-blue-400' : 'text-blue-600') : index === 2 ? (isDark ? 'text-purple-400' : 'text-purple-600') : (isDark ? 'text-orange-400' : 'text-orange-600')]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 5a1 1 0 011-1h14a1 1 0 011 1v2a1 1 0 01-1 1H5a1 1 0 01-1-1V5zM4 13a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H5a1 1 0 01-1-1v-6zM16 13a1 1 0 011-1h2a1 1 0 011 1v6a1 1 0 01-1 1h-2a1 1 0 01-1-1v-6z" />
                  </svg>
                  <span :class="['text-sm font-semibold', index === 0 ? (isDark ? 'text-green-300' : 'text-green-700') : index === 1 ? (isDark ? 'text-blue-300' : 'text-blue-700') : index === 2 ? (isDark ? 'text-purple-300' : 'text-purple-700') : (isDark ? 'text-orange-300' : 'text-orange-700')]">{{ template.label }}</span>
                </div>
                <span :class="['rounded-full bg-gray-200 px-3 py-1 text-xs font-medium', isDark ? 'bg-gray-600 text-gray-300' : 'text-gray-600']">{{ t('compliance.templates.apply') }}</span>
              </div>
              <p :class="['text-sm', isDark ? 'text-gray-400' : 'text-gray-600']">{{ template.description }}</p>
            </div>
          </div>
        </div>

        <div :class="['mb-20 rounded-2xl p-10', isDark ? 'bg-gradient-to-r from-blue-900/30 to-purple-900/30' : 'bg-gradient-to-r from-blue-50 to-purple-50']">
          <div class="flex flex-col items-center text-center">
            <h2 :class="['mb-4 text-3xl font-bold', isDark ? 'text-white' : 'text-gray-900']">{{ t('compliance.cta.title') }}</h2>
            <p :class="['mb-8 max-w-2xl text-lg', isDark ? 'text-gray-400' : 'text-gray-600']">{{ t('compliance.cta.description') }}</p>
            <div class="flex flex-col gap-4 sm:flex-row">
              <button @click="router.push('/governance')" :class="['rounded-xl px-8 py-4 text-lg font-semibold shadow-xl transition-all', isDark ? 'bg-blue-600 text-white hover:bg-blue-700 hover:shadow-blue-900/40' : 'bg-[#0757b8] text-white hover:bg-[#064ea8] hover:shadow-blue-900/20']">
                {{ t('compliance.cta.primary') }}
              </button>
              <button @click="router.push('/login')" :class="['rounded-xl px-8 py-4 text-lg font-semibold transition-all', isDark ? 'bg-gray-700 text-gray-200 hover:bg-gray-600' : 'bg-white text-gray-700 hover:bg-gray-50']">
                {{ t('compliance.cta.secondary') }}
              </button>
            </div>
          </div>
        </div>

        <footer :class="['rounded-2xl p-8 text-center', isDark ? 'bg-gray-800/50' : 'bg-white/80']" :style="{ boxShadow: isDark ? '0 4px 24px rgba(0,0,0,0.3)' : '0 4px 24px rgba(59,130,246,0.08)' }">
          <div class="mb-4 flex items-center justify-center gap-3">
            <img :src="LogoSvg" alt="Three Router Logo" class="h-6 w-6 object-contain" />
            <span :class="['text-lg font-semibold', isDark ? 'text-white' : 'text-[#021b4a]']">ThreeRouter</span>
          </div>
          <p :class="['text-sm', isDark ? 'text-gray-500' : 'text-gray-500']">{{ t('compliance.footer') }}</p>
        </footer>
      </div>
    </main>
  </div>
</template>