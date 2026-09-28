<template>
  <div class="relative flex min-h-screen flex-col overflow-hidden bg-gradient-to-br from-slate-50 to-blue-50 dark:from-dark-950 dark:to-dark-900">
    <!-- Header -->
    <header class="relative z-20 px-6 py-4">
      <nav class="mx-auto flex max-w-7xl items-center justify-between">
        <router-link to="/home" class="flex items-center gap-[12px] pl-0">
          <img :src="siteLogo || '/src/assets/icons/logo.webp'" alt="Three Router Logo" class="h-[32px] w-[32px] object-contain" />
          <span class="text-[16px] font-semibold text-[#021b4a] dark:text-white" style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;">{{ siteName }}</span>
        </router-link>

        <div class="flex items-center gap-3">
          <!-- Nav Links -->
          <button
            @click="router.push('/admin/models')"
            class="hidden rounded-lg px-3 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700 sm:inline"
          >
            {{ t('home.models') }}
          </button>
          <button
            @click="router.push('/admin')"
            class="hidden rounded-lg px-3 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700 sm:inline"
          >
            {{ t('home.dashboard') }}
          </button>

          <LocaleSwitcher />

          <button
            @click="toggleTheme"
            class="rounded-lg p-2 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-700 dark:hover:text-white"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>

          <button
            @click="router.push('/login')"
            class="rounded-lg bg-[#0757b8] px-5 py-2 text-sm font-semibold text-white shadow-lg shadow-blue-900/10 transition-colors hover:bg-[#064ea8]"
          >
            {{ t('home.login') }}
          </button>
        </div>
      </nav>
    </header>

    <!-- Main Content -->
    <main class="relative z-10 flex-1">
      <!-- Hero -->
      <section id="hero" class="relative px-6 pt-12 pb-20">
        <div class="mx-auto max-w-7xl">
          <div class="mb-4 inline-flex items-center gap-2 rounded-full bg-gradient-to-r from-[#20f5b4]/20 to-[#0757b8]/20 px-5 py-2.5">
            <span class="h-2.5 w-2.5 animate-pulse rounded-full bg-[#0757b8]"></span>
            <span class="text-sm font-semibold text-[#0757b8]">{{ t('enterprise.hero.badge') }}</span>
          </div>

          <div class="grid items-center gap-12 lg:grid-cols-5">
            <div class="lg:col-span-3">
              <h1 class="mb-5 text-3xl font-bold leading-tight text-gray-900 dark:text-white md:text-4xl lg:text-5xl">
                {{ t('enterprise.hero.title') }}<br>
                <span class="text-[#0757b8]">{{ t('enterprise.hero.subtitle') }}</span>
              </h1>
              <p class="mb-8 text-lg text-gray-600 dark:text-gray-300">
                {{ selectedRole === 'decision' ? t('enterprise.hero.decisionDesc') : t('enterprise.hero.employeeDesc') }}
              </p>
              <div class="flex flex-wrap gap-3">
                <button
                  v-if="selectedRole === 'decision'"
                  @click="router.push('/register')"
                  class="inline-flex items-center gap-2 rounded-full bg-gradient-to-r from-[#0757b8] to-[#20f5b4] px-8 py-3.5 text-base font-semibold text-white shadow-xl transition-all hover:scale-105"
                >
                  {{ t('enterprise.hero.ctaPrimary') }}
                </button>
                <button
                  v-else
                  @click="scrollTo('employee-apply')"
                  class="inline-flex items-center gap-2 rounded-full bg-gradient-to-r from-[#7c3aed] to-[#a78bfa] px-8 py-3.5 text-base font-semibold text-white shadow-xl transition-all hover:scale-105"
                >
                  {{ t('enterprise.hero.ctaEmployee') }}
                </button>
                <button
                  @click="scrollTo('compare')"
                  class="inline-flex items-center gap-2 rounded-full border border-gray-300 bg-white px-8 py-3.5 text-base font-semibold text-gray-700 transition-all hover:bg-gray-50 dark:border-dark-600 dark:bg-dark-800 dark:text-white dark:hover:bg-dark-700"
                >
                  {{ t('enterprise.hero.ctaSecondary') }}
                </button>
              </div>

              <div class="mt-10 grid grid-cols-2 gap-4 sm:grid-cols-4">
                <div class="rounded-xl border border-gray-200 bg-white/80 p-4 dark:border-dark-700 dark:bg-dark-800/60">
                  <div class="text-2xl font-bold text-[#0757b8]">{{ t('enterprise.hero.statCustomersValue') }}</div>
                  <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('enterprise.hero.statCustomers') }}</div>
                </div>
                <div class="rounded-xl border border-gray-200 bg-white/80 p-4 dark:border-dark-700 dark:bg-dark-800/60">
                  <div class="text-2xl font-bold text-[#0757b8]">{{ t('enterprise.hero.statRequestsValue') }}</div>
                  <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('enterprise.hero.statRequests') }}</div>
                </div>
                <div class="rounded-xl border border-gray-200 bg-white/80 p-4 dark:border-dark-700 dark:bg-dark-800/60">
                  <div class="text-2xl font-bold text-[#0757b8]">{{ t('enterprise.hero.statUptimeValue') }}</div>
                  <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('enterprise.hero.statUptime') }}</div>
                </div>
                <div class="rounded-xl border border-gray-200 bg-white/80 p-4 dark:border-dark-700 dark:bg-dark-800/60">
                  <div class="text-2xl font-bold text-[#0757b8]">{{ t('enterprise.hero.statLeaksValue') }}</div>
                  <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('enterprise.hero.statLeaks') }}</div>
                </div>
              </div>
            </div>

            <!-- Role Selector Cards -->
            <div class="lg:col-span-2 flex flex-col items-center">
              <p class="mb-4 text-sm font-medium text-gray-400 dark:text-gray-500">{{ t('enterprise.hero.roleLabel') }}</p>
              <div class="grid w-full max-w-md gap-4">
                <div
                  @click="selectedRole = 'decision'"
                  class="cursor-pointer rounded-2xl border-2 p-5 transition-all"
                  :class="selectedRole === 'decision'
                    ? 'border-[#0757b8] bg-gradient-to-br from-blue-50 to-white shadow-lg shadow-blue-900/10 dark:border-blue-400 dark:from-blue-900/20 dark:to-dark-800'
                    : 'border-gray-200 bg-white hover:border-blue-300 dark:border-dark-700 dark:bg-dark-800'"
                >
                  <div class="flex items-start gap-4">
                    <div class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl bg-blue-100 text-xl dark:bg-blue-900/30">🏢</div>
                    <div class="flex-1 min-w-0">
                      <h4 class="text-lg font-bold text-gray-900 dark:text-white">{{ t('enterprise.hero.roleDecisionTitle') }}</h4>
                      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('enterprise.hero.roleDecisionDesc') }}</p>
                      <p class="mt-0.5 text-xs text-gray-400 dark:text-gray-500">{{ t('enterprise.hero.roleDecisionFocus') }}</p>
                      <span class="mt-2 inline-block rounded-full bg-blue-100 px-2.5 py-0.5 text-xs font-semibold text-blue-700 dark:bg-blue-900/30 dark:text-blue-300">{{ t('enterprise.hero.roleDecisionTag') }}</span>
                    </div>
                    <div v-if="selectedRole === 'decision'" class="flex h-6 w-6 flex-shrink-0 items-center justify-center rounded-full bg-[#0757b8] text-xs font-bold text-white">✓</div>
                  </div>
                </div>
                <div
                  @click="selectedRole = 'employee'"
                  class="cursor-pointer rounded-2xl border-2 p-5 transition-all"
                  :class="selectedRole === 'employee'
                    ? 'border-purple-500 bg-gradient-to-br from-purple-50 to-white shadow-lg shadow-purple-900/10 dark:border-purple-400 dark:from-purple-900/20 dark:to-dark-800'
                    : 'border-gray-200 bg-white hover:border-purple-300 dark:border-dark-700 dark:bg-dark-800'"
                >
                  <div class="flex items-start gap-4">
                    <div class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl bg-purple-100 text-xl dark:bg-purple-900/30">👤</div>
                    <div class="flex-1 min-w-0">
                      <h4 class="text-lg font-bold text-gray-900 dark:text-white">{{ t('enterprise.hero.roleEmployeeTitle') }}</h4>
                      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('enterprise.hero.roleEmployeeDesc') }}</p>
                      <p class="mt-0.5 text-xs text-gray-400 dark:text-gray-500">{{ t('enterprise.hero.roleEmployeeFocus') }}</p>
                      <span class="mt-2 inline-block rounded-full bg-purple-100 px-2.5 py-0.5 text-xs font-semibold text-purple-700 dark:bg-purple-900/30 dark:text-purple-300">{{ t('enterprise.hero.roleEmployeeTag') }}</span>
                    </div>
                    <div v-if="selectedRole === 'employee'" class="flex h-6 w-6 flex-shrink-0 items-center justify-center rounded-full bg-purple-600 text-xs font-bold text-white">✓</div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- Authority -->
      <section id="authority" class="relative overflow-hidden bg-[#020b1f] px-6 py-20 text-white">
        <div class="pointer-events-none absolute -right-24 -top-24 h-72 w-72 rounded-full bg-[#20f5b4]/10 blur-3xl"></div>
        <div class="pointer-events-none absolute -left-24 bottom-0 h-72 w-72 rounded-full bg-[#0757b8]/20 blur-3xl"></div>
        <div class="relative mx-auto max-w-7xl text-center">
          <div class="mb-3 text-sm font-bold uppercase tracking-wider text-[#20f5b4]">{{ t('enterprise.authority.label') }}</div>
          <h2 class="mb-4 text-3xl font-bold md:text-4xl">{{ t('enterprise.authority.title') }}</h2>
          <p class="mx-auto mb-12 max-w-2xl text-slate-300">{{ t('enterprise.authority.subtitle') }}</p>

          <div class="grid gap-6 md:grid-cols-3">
            <div class="rounded-2xl border border-[#20f5b4]/20 bg-white/5 p-6 backdrop-blur">
              <div class="mb-4 text-4xl text-[#20f5b4]">"</div>
              <p class="mb-4 text-lg font-semibold leading-relaxed">{{ t('enterprise.authority.quote1') }}</p>
              <p class="text-sm text-slate-300">{{ t('enterprise.authority.quote1Desc') }}</p>
            </div>
            <div class="rounded-2xl border border-[#20f5b4]/20 bg-white/5 p-6 backdrop-blur">
              <div class="mb-4 text-4xl text-[#20f5b4]">"</div>
              <p class="mb-4 text-lg font-semibold leading-relaxed">{{ t('enterprise.authority.quote2') }}</p>
              <p class="text-sm text-slate-300">{{ t('enterprise.authority.quote2Desc') }}</p>
            </div>
            <div class="rounded-2xl border border-[#20f5b4]/20 bg-white/5 p-6 backdrop-blur">
              <div class="mb-4 text-4xl text-[#20f5b4]">"</div>
              <p class="mb-4 text-lg font-semibold leading-relaxed">{{ t('enterprise.authority.quote3') }}</p>
              <p class="text-sm text-slate-300">{{ t('enterprise.authority.quote3Desc') }}</p>
            </div>
          </div>
        </div>
      </section>

      <!-- Pain Points -->
      <section id="pain" class="px-6 py-20">
        <div class="mx-auto max-w-7xl text-center">
          <div class="mb-3 text-sm font-bold uppercase tracking-wider text-[#0757b8]">{{ t('enterprise.pain.label') }}</div>
          <h2 class="mb-4 text-3xl font-bold text-gray-900 dark:text-white md:text-4xl">{{ t('enterprise.pain.title') }}</h2>
          <p class="mx-auto mb-12 max-w-2xl text-gray-500 dark:text-gray-400">{{ t('enterprise.pain.subtitle') }}</p>

          <div class="grid gap-6 md:grid-cols-3">
            <div class="rounded-2xl border border-red-100 bg-red-50/50 p-6 dark:border-red-900/30 dark:bg-red-900/10">
              <div class="mb-4 inline-flex h-14 w-14 items-center justify-center rounded-2xl bg-red-100 text-2xl dark:bg-red-900/30">🔓</div>
              <h3 class="mb-3 text-xl font-bold text-gray-900 dark:text-white">{{ t('enterprise.pain.item1Title') }}</h3>
              <p class="text-gray-600 dark:text-gray-300">{{ t('enterprise.pain.item1Desc') }}</p>
            </div>
            <div class="rounded-2xl border border-amber-100 bg-amber-50/50 p-6 dark:border-amber-900/30 dark:bg-amber-900/10">
              <div class="mb-4 inline-flex h-14 w-14 items-center justify-center rounded-2xl bg-amber-100 text-2xl dark:bg-amber-900/30">🎭</div>
              <h3 class="mb-3 text-xl font-bold text-gray-900 dark:text-white">{{ t('enterprise.pain.item2Title') }}</h3>
              <p class="text-gray-600 dark:text-gray-300">{{ t('enterprise.pain.item2Desc') }}</p>
            </div>
            <div class="rounded-2xl border border-purple-100 bg-purple-50/50 p-6 dark:border-purple-900/30 dark:bg-purple-900/10">
              <div class="mb-4 inline-flex h-14 w-14 items-center justify-center rounded-2xl bg-purple-100 text-2xl dark:bg-purple-900/30">💨</div>
              <h3 class="mb-3 text-xl font-bold text-gray-900 dark:text-white">{{ t('enterprise.pain.item3Title') }}</h3>
              <p class="text-gray-600 dark:text-gray-300">{{ t('enterprise.pain.item3Desc') }}</p>
            </div>
          </div>
        </div>
      </section>

      <!-- Solution -->
      <section id="solution" class="bg-gray-50 px-6 py-20 dark:bg-dark-900/50">
        <div class="mx-auto max-w-7xl text-center">
          <div class="mb-3 text-sm font-bold uppercase tracking-wider text-[#0757b8]">{{ t('enterprise.solution.label') }}</div>
          <h2 class="mb-4 text-3xl font-bold text-gray-900 dark:text-white md:text-4xl">{{ t('enterprise.solution.title') }}</h2>
          <p class="mx-auto mb-12 max-w-2xl text-gray-500 dark:text-gray-400">{{ t('enterprise.solution.subtitle') }}</p>

          <div class="grid gap-6 lg:grid-cols-3">
            <div class="rounded-2xl bg-white p-6 text-left shadow-lg shadow-gray-100 dark:bg-dark-800 dark:shadow-none">
              <div class="mb-4 inline-flex rounded-full bg-emerald-100 px-3 py-1 text-sm font-semibold text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300">{{ t('enterprise.solution.card1Badge') }}</div>
              <h3 class="mb-2 text-xl font-bold text-gray-900 dark:text-white">{{ t('enterprise.solution.card1Title') }}</h3>
              <p class="mb-4 text-sm text-gray-500 dark:text-gray-400">{{ t('enterprise.solution.card1Desc') }}</p>
              <ul class="list-none space-y-2 text-sm text-gray-600 dark:text-gray-300">
                <li v-for="(point, idx) in solutionCard1Points" :key="idx" class="flex items-start gap-2">
                  <span class="text-emerald-500">✓</span>
                  <span>{{ point }}</span>
                </li>
              </ul>
              <button
                @click="scrollTo('cta')"
                class="mt-6 inline-flex w-full items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-[#0757b8] to-[#20f5b4] px-6 py-3 text-sm font-semibold text-white shadow-lg transition-all hover:scale-[1.02]"
              >
                {{ t('enterprise.solution.card1Btn') }}
                <span>→</span>
              </button>
            </div>
            <div class="rounded-2xl bg-white p-6 text-left shadow-lg shadow-gray-100 dark:bg-dark-800 dark:shadow-none">
              <div class="mb-4 inline-flex rounded-full bg-blue-100 px-3 py-1 text-sm font-semibold text-blue-700 dark:bg-blue-900/30 dark:text-blue-300">{{ t('enterprise.solution.card2Badge') }}</div>
              <h3 class="mb-2 text-xl font-bold text-gray-900 dark:text-white">{{ t('enterprise.solution.card2Title') }}</h3>
              <p class="mb-4 text-sm text-gray-500 dark:text-gray-400">{{ t('enterprise.solution.card2Desc') }}</p>
              <ul class="list-none space-y-2 text-sm text-gray-600 dark:text-gray-300">
                <li v-for="(point, idx) in solutionCard2Points" :key="idx" class="flex items-start gap-2">
                  <span class="text-blue-500">✓</span>
                  <span>{{ point }}</span>
                </li>
              </ul>
              <a
                :href="t('enterprise.solution.card2BtnLink')"
                class="mt-6 inline-flex w-full items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-[#0757b8] to-[#20f5b4] px-6 py-3 text-sm font-semibold text-white shadow-lg transition-all hover:scale-[1.02]"
              >
                {{ t('enterprise.solution.card2Btn') }}
                <span>→</span>
              </a>
            </div>
            <div class="rounded-2xl bg-white p-6 text-left shadow-lg shadow-gray-100 dark:bg-dark-800 dark:shadow-none">
              <div class="mb-4 inline-flex rounded-full bg-purple-100 px-3 py-1 text-sm font-semibold text-purple-700 dark:bg-purple-900/30 dark:text-purple-300">{{ t('enterprise.solution.card3Badge') }}</div>
              <h3 class="mb-2 text-xl font-bold text-gray-900 dark:text-white">{{ t('enterprise.solution.card3Title') }}</h3>
              <p class="mb-4 text-sm text-gray-500 dark:text-gray-400">{{ t('enterprise.solution.card3Desc') }}</p>
              <ul class="list-none space-y-2 text-sm text-gray-600 dark:text-gray-300">
                <li v-for="(point, idx) in solutionCard3Points" :key="idx" class="flex items-start gap-2">
                  <span class="text-purple-500">✓</span>
                  <span>{{ point }}</span>
                </li>
              </ul>
              <button
                @click="scrollTo('employee-apply')"
                class="mt-6 inline-flex w-full items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-[#0757b8] to-[#20f5b4] px-6 py-3 text-sm font-semibold text-white shadow-lg transition-all hover:scale-[1.02]"
              >
                {{ t('enterprise.solution.card3Btn') }}
                <span>→</span>
              </button>
            </div>
          </div>
        </div>
      </section>

      <!-- Team Features -->
      <section id="team" class="px-6 py-20">
        <div class="mx-auto max-w-7xl text-center">
          <div class="mb-3 text-sm font-bold uppercase tracking-wider text-[#0757b8]">{{ t('enterprise.team.label') }}</div>
          <h2 class="mb-4 text-3xl font-bold text-gray-900 dark:text-white md:text-4xl">{{ t('enterprise.team.title') }}</h2>
          <p class="mx-auto mb-12 max-w-2xl text-gray-500 dark:text-gray-400">{{ t('enterprise.team.subtitle') }}</p>

          <div class="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
            <div class="group rounded-2xl border border-gray-100 bg-white p-6 text-left transition-all hover:-translate-y-1 hover:shadow-lg dark:border-dark-700 dark:bg-dark-800">
              <div class="mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-blue-100 text-2xl text-blue-600 dark:bg-blue-900/30 dark:text-blue-300">🏢</div>
              <h3 class="mb-2 text-lg font-bold text-gray-900 dark:text-white">{{ t('enterprise.team.feature1Title') }}</h3>
              <p class="mb-4 text-sm text-gray-500 dark:text-gray-400">{{ t('enterprise.team.feature1Desc') }}</p>
              <ul class="list-none space-y-1.5 text-sm text-gray-600 dark:text-gray-300">
                <li v-for="(point, idx) in teamFeature1Points" :key="idx" class="flex items-start gap-2">
                  <span class="text-[#20f5b4]">✓</span>
                  <span>{{ point }}</span>
                </li>
              </ul>
            </div>
            <div class="group rounded-2xl border border-gray-100 bg-white p-6 text-left transition-all hover:-translate-y-1 hover:shadow-lg dark:border-dark-700 dark:bg-dark-800">
              <div class="mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-green-100 text-2xl text-green-600 dark:bg-green-900/30 dark:text-green-300">👤</div>
              <h3 class="mb-2 text-lg font-bold text-gray-900 dark:text-white">{{ t('enterprise.team.feature2Title') }}</h3>
              <p class="mb-4 text-sm text-gray-500 dark:text-gray-400">{{ t('enterprise.team.feature2Desc') }}</p>
              <ul class="list-none space-y-1.5 text-sm text-gray-600 dark:text-gray-300">
                <li v-for="(point, idx) in teamFeature2Points" :key="idx" class="flex items-start gap-2">
                  <span class="text-[#20f5b4]">✓</span>
                  <span>{{ point }}</span>
                </li>
              </ul>
            </div>
            <div class="group rounded-2xl border border-gray-100 bg-white p-6 text-left transition-all hover:-translate-y-1 hover:shadow-lg dark:border-dark-700 dark:bg-dark-800">
              <div class="mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-amber-100 text-2xl text-amber-600 dark:bg-amber-900/30 dark:text-amber-300">🔑</div>
              <h3 class="mb-2 text-lg font-bold text-gray-900 dark:text-white">{{ t('enterprise.team.feature3Title') }}</h3>
              <p class="mb-4 text-sm text-gray-500 dark:text-gray-400">{{ t('enterprise.team.feature3Desc') }}</p>
              <ul class="list-none space-y-1.5 text-sm text-gray-600 dark:text-gray-300">
                <li v-for="(point, idx) in teamFeature3Points" :key="idx" class="flex items-start gap-2">
                  <span class="text-[#20f5b4]">✓</span>
                  <span>{{ point }}</span>
                </li>
              </ul>
            </div>
            <div class="group rounded-2xl border border-gray-100 bg-white p-6 text-left transition-all hover:-translate-y-1 hover:shadow-lg dark:border-dark-700 dark:bg-dark-800">
              <div class="mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-cyan-100 text-2xl text-cyan-600 dark:bg-cyan-900/30 dark:text-cyan-300">📊</div>
              <h3 class="mb-2 text-lg font-bold text-gray-900 dark:text-white">{{ t('enterprise.team.feature4Title') }}</h3>
              <p class="mb-4 text-sm text-gray-500 dark:text-gray-400">{{ t('enterprise.team.feature4Desc') }}</p>
              <ul class="list-none space-y-1.5 text-sm text-gray-600 dark:text-gray-300">
                <li v-for="(point, idx) in teamFeature4Points" :key="idx" class="flex items-start gap-2">
                  <span class="text-[#20f5b4]">✓</span>
                  <span>{{ point }}</span>
                </li>
              </ul>
            </div>
            <div class="group rounded-2xl border border-gray-100 bg-white p-6 text-left transition-all hover:-translate-y-1 hover:shadow-lg dark:border-dark-700 dark:bg-dark-800">
              <div class="mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-purple-100 text-2xl text-purple-600 dark:bg-purple-900/30 dark:text-purple-300">🛡️</div>
              <h3 class="mb-2 text-lg font-bold text-gray-900 dark:text-white">{{ t('enterprise.team.feature5Title') }}</h3>
              <p class="mb-4 text-sm text-gray-500 dark:text-gray-400">{{ t('enterprise.team.feature5Desc') }}</p>
              <ul class="list-none space-y-1.5 text-sm text-gray-600 dark:text-gray-300">
                <li v-for="(point, idx) in teamFeature5Points" :key="idx" class="flex items-start gap-2">
                  <span class="text-[#20f5b4]">✓</span>
                  <span>{{ point }}</span>
                </li>
              </ul>
            </div>
            <div class="group rounded-2xl border border-gray-100 bg-white p-6 text-left transition-all hover:-translate-y-1 hover:shadow-lg dark:border-dark-700 dark:bg-dark-800">
              <div class="mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-emerald-100 text-2xl text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-300">💰</div>
              <h3 class="mb-2 text-lg font-bold text-gray-900 dark:text-white">{{ t('enterprise.team.feature6Title') }}</h3>
              <p class="mb-4 text-sm text-gray-500 dark:text-gray-400">{{ t('enterprise.team.feature6Desc') }}</p>
              <ul class="list-none space-y-1.5 text-sm text-gray-600 dark:text-gray-300">
                <li v-for="(point, idx) in teamFeature6Points" :key="idx" class="flex items-start gap-2">
                  <span class="text-[#20f5b4]">✓</span>
                  <span>{{ point }}</span>
                </li>
              </ul>
            </div>
          </div>
        </div>
      </section>

      <!-- Token Management -->
      <section id="token-management" class="relative overflow-hidden px-6 py-20">
        <div class="pointer-events-none absolute right-0 top-0 h-96 w-96 rounded-full bg-[#20f5b4]/5 blur-3xl"></div>
        <div class="pointer-events-none absolute left-0 bottom-0 h-96 w-96 rounded-full bg-[#0757b8]/5 blur-3xl"></div>
        <div class="relative mx-auto max-w-7xl">
          <div class="mb-12 text-center">
            <div class="mb-3 text-sm font-bold uppercase tracking-wider text-[#0757b8]">{{ t('enterprise.tokenManagement.label') }}</div>
            <h2 class="mb-4 text-3xl font-bold text-gray-900 dark:text-white md:text-4xl">{{ t('enterprise.tokenManagement.title') }}</h2>
            <p class="mx-auto mb-6 max-w-3xl text-gray-500 dark:text-gray-400">{{ t('enterprise.tokenManagement.subtitle') }}</p>
          </div>

          <!-- Core Metrics Row -->
          <div class="mb-10 grid gap-6 md:grid-cols-4">
            <div v-for="(metric, idx) in tokenMetrics" :key="idx" class="rounded-2xl border border-gray-100 bg-gradient-to-br from-white to-gray-50 p-6 text-center shadow-sm dark:border-dark-700 dark:from-dark-800 dark:to-dark-900">
              <div class="mb-2 text-3xl font-bold text-[#0757b8]">{{ metric.value }}</div>
              <div class="text-sm font-medium text-gray-600 dark:text-gray-400">{{ metric.label }}</div>
              <div class="mt-1 text-xs text-gray-400 dark:text-gray-500">{{ metric.desc }}</div>
            </div>
          </div>

          <!-- Feature Cards Grid -->
          <div class="grid gap-6 lg:grid-cols-3">
            <div v-for="(feature, idx) in tokenManagementFeatures" :key="idx" class="group rounded-2xl border border-gray-100 bg-white p-6 text-left transition-all hover:-translate-y-1 hover:shadow-lg dark:border-dark-700 dark:bg-dark-800">
              <div class="mb-4 flex h-12 w-12 items-center justify-center rounded-xl text-2xl" :class="feature.iconBg">
                {{ feature.icon }}
              </div>
              <h3 class="mb-2 text-lg font-bold text-gray-900 dark:text-white">{{ feature.title }}</h3>
              <p class="mb-4 text-sm text-gray-500 dark:text-gray-400">{{ feature.desc }}</p>
              <ul class="list-none space-y-1.5 text-sm text-gray-600 dark:text-gray-300">
                <li v-for="(point, pIdx) in feature.points" :key="pIdx" class="flex items-start gap-2">
                  <span class="text-[#20f5b4]">✓</span>
                  <span>{{ point }}</span>
                </li>
              </ul>
            </div>
          </div>

          <!-- Process Flow -->
          <div class="mt-12 rounded-2xl bg-gradient-to-r from-[#021b4a] to-[#0757b8] p-8 text-white">
            <h3 class="mb-8 text-center text-2xl font-bold">{{ t('enterprise.tokenManagement.processTitle') }}</h3>
            <div class="grid gap-6 md:grid-cols-4">
              <div v-for="(step, idx) in tokenProcessSteps" :key="idx" class="text-center">
                <div class="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-white/10 text-xl font-bold backdrop-blur">
                  {{ idx + 1 }}
                </div>
                <h4 class="mb-2 text-sm font-bold">{{ step.title }}</h4>
                <p class="text-xs text-white/70">{{ step.desc }}</p>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- Team Workflow -->
      <section id="team-workflow" class="bg-gray-50 px-6 py-20 dark:bg-dark-900/50">
        <div class="mx-auto max-w-7xl text-center">
          <div class="mb-3 text-sm font-bold uppercase tracking-wider text-[#0757b8]">{{ t('enterprise.teamWorkflow.label') }}</div>
          <h2 class="mb-4 text-3xl font-bold text-gray-900 dark:text-white md:text-4xl">{{ t('enterprise.teamWorkflow.title') }}</h2>
          <p class="mx-auto mb-12 max-w-2xl text-gray-500 dark:text-gray-400">{{ t('enterprise.teamWorkflow.subtitle') }}</p>

          <div class="grid gap-6 lg:grid-cols-3">
            <div class="group rounded-2xl border border-gray-100 bg-white p-6 text-left transition-all hover:-translate-y-1 hover:shadow-lg dark:border-dark-700 dark:bg-dark-800">
              <div class="mb-4 flex h-14 w-14 items-center justify-center rounded-xl bg-blue-100 text-3xl text-blue-600 dark:bg-blue-900/30 dark:text-blue-300">🏗️</div>
              <h3 class="mb-2 text-xl font-bold text-gray-900 dark:text-white">{{ t('enterprise.teamWorkflow.step1Title') }}</h3>
              <p class="mb-4 text-sm text-gray-500 dark:text-gray-400">{{ t('enterprise.teamWorkflow.step1Desc') }}</p>
              <ul class="list-none space-y-1.5 text-sm text-gray-600 dark:text-gray-300">
                <li v-for="(point, idx) in teamWorkflowStep1Points" :key="idx" class="flex items-start gap-2">
                  <span class="text-[#20f5b4]">✓</span>
                  <span>{{ point }}</span>
                </li>
              </ul>
            </div>
            <div class="group rounded-2xl border border-gray-100 bg-white p-6 text-left transition-all hover:-translate-y-1 hover:shadow-lg dark:border-dark-700 dark:bg-dark-800">
              <div class="mb-4 flex h-14 w-14 items-center justify-center rounded-xl bg-indigo-100 text-3xl text-indigo-600 dark:bg-indigo-900/30 dark:text-indigo-300">🔗</div>
              <h3 class="mb-2 text-xl font-bold text-gray-900 dark:text-white">{{ t('enterprise.teamWorkflow.step2Title') }}</h3>
              <p class="mb-4 text-sm text-gray-500 dark:text-gray-400">{{ t('enterprise.teamWorkflow.step2Desc') }}</p>
              <ul class="list-none space-y-1.5 text-sm text-gray-600 dark:text-gray-300">
                <li v-for="(point, idx) in teamWorkflowStep2Points" :key="idx" class="flex items-start gap-2">
                  <span class="text-[#20f5b4]">✓</span>
                  <span>{{ point }}</span>
                </li>
              </ul>
            </div>
            <div class="group rounded-2xl border border-gray-100 bg-white p-6 text-left transition-all hover:-translate-y-1 hover:shadow-lg dark:border-dark-700 dark:bg-dark-800">
              <div class="mb-4 flex h-14 w-14 items-center justify-center rounded-xl bg-emerald-100 text-3xl text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-300">📊</div>
              <h3 class="mb-2 text-xl font-bold text-gray-900 dark:text-white">{{ t('enterprise.teamWorkflow.step3Title') }}</h3>
              <p class="mb-4 text-sm text-gray-500 dark:text-gray-400">{{ t('enterprise.teamWorkflow.step3Desc') }}</p>
              <ul class="list-none space-y-1.5 text-sm text-gray-600 dark:text-gray-300">
                <li v-for="(point, idx) in teamWorkflowStep3Points" :key="idx" class="flex items-start gap-2">
                  <span class="text-[#20f5b4]">✓</span>
                  <span>{{ point }}</span>
                </li>
              </ul>
            </div>
          </div>
        </div>
      </section>

      <!-- Comparison -->
      <section id="compare" class="bg-gray-50 px-6 py-20 dark:bg-dark-900/50">
        <div class="mx-auto max-w-7xl text-center">
          <div class="mb-3 text-sm font-bold uppercase tracking-wider text-[#0757b8]">{{ t('enterprise.compare.label') }}</div>
          <h2 class="mb-4 text-3xl font-bold text-gray-900 dark:text-white md:text-4xl">{{ t('enterprise.compare.title') }}</h2>
          <p class="mx-auto mb-12 max-w-2xl text-gray-500 dark:text-gray-400">{{ t('enterprise.compare.subtitle') }}</p>

          <div class="overflow-hidden rounded-2xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
            <table class="w-full text-sm">
              <thead class="bg-gray-50 text-center text-xs uppercase tracking-wide text-gray-500 dark:bg-dark-700 dark:text-gray-400">
                <tr>
                  <th class="px-6 py-4 font-bold">{{ t('enterprise.compare.dimension') }}</th>
                  <th class="px-6 py-4 font-bold text-[#0757b8]">{{ t('enterprise.compare.us') }}</th>
                  <th class="px-6 py-4 font-bold text-gray-500">{{ t('enterprise.compare.them') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr v-for="key in compareKeys" :key="key">
                  <td class="px-6 py-4 font-medium text-gray-900 dark:text-white">{{ t(`enterprise.compare.rows.${key}.label`) }}</td>
                  <td class="px-6 py-4 text-gray-700 dark:text-gray-300">
                    <span class="mr-2 text-emerald-500">✓</span>{{ t(`enterprise.compare.rows.${key}.us`) }}
                  </td>
                  <td class="px-6 py-4 text-gray-500 dark:text-gray-400">
                    <span class="mr-2 text-red-500">✗</span>{{ t(`enterprise.compare.rows.${key}.them`) }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </section>

      <!-- Cases -->
      <section id="cases" class="px-6 py-20">
        <div class="mx-auto max-w-7xl text-center">
          <div class="mb-3 text-sm font-bold uppercase tracking-wider text-[#0757b8]">{{ t('enterprise.cases.label') }}</div>
          <h2 class="mb-4 text-3xl font-bold text-gray-900 dark:text-white md:text-4xl">{{ t('enterprise.cases.title') }}</h2>
          <p class="mx-auto mb-12 max-w-2xl text-gray-500 dark:text-gray-400">{{ t('enterprise.cases.subtitle') }}</p>

          <div class="grid gap-6 lg:grid-cols-3">
            <div class="rounded-2xl border border-gray-100 bg-white p-6 text-left shadow-sm dark:border-dark-700 dark:bg-dark-800">
              <div class="mb-4 flex items-center gap-3">
                <div class="flex h-10 w-10 items-center justify-center rounded-lg bg-gradient-to-br from-indigo-500 to-purple-600 text-sm font-bold text-white">{{ t('enterprise.cases.case1Initial') }}</div>
                <span class="font-semibold text-gray-900 dark:text-white">{{ t('enterprise.cases.case1Company') }}</span>
              </div>
              <h3 class="mb-2 text-lg font-bold text-gray-900 dark:text-white">{{ t('enterprise.cases.case1Title') }}</h3>
              <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('enterprise.cases.case1Desc') }}</p>
            </div>
            <div class="rounded-2xl border border-gray-100 bg-white p-6 text-left shadow-sm dark:border-dark-700 dark:bg-dark-800">
              <div class="mb-4 flex items-center gap-3">
                <div class="flex h-10 w-10 items-center justify-center rounded-lg bg-gradient-to-br from-pink-500 to-rose-500 text-sm font-bold text-white">{{ t('enterprise.cases.case2Initial') }}</div>
                <span class="font-semibold text-gray-900 dark:text-white">{{ t('enterprise.cases.case2Company') }}</span>
              </div>
              <h3 class="mb-2 text-lg font-bold text-gray-900 dark:text-white">{{ t('enterprise.cases.case2Title') }}</h3>
              <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('enterprise.cases.case2Desc') }}</p>
            </div>
            <div class="rounded-2xl border border-gray-100 bg-white p-6 text-left shadow-sm dark:border-dark-700 dark:bg-dark-800">
              <div class="mb-4 flex items-center gap-3">
                <div class="flex h-10 w-10 items-center justify-center rounded-lg bg-gradient-to-br from-cyan-500 to-blue-500 text-sm font-bold text-white">{{ t('enterprise.cases.case3Initial') }}</div>
                <span class="font-semibold text-gray-900 dark:text-white">{{ t('enterprise.cases.case3Company') }}</span>
              </div>
              <h3 class="mb-2 text-lg font-bold text-gray-900 dark:text-white">{{ t('enterprise.cases.case3Title') }}</h3>
              <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('enterprise.cases.case3Desc') }}</p>
            </div>
          </div>
        </div>
      </section>

      <!-- CTA -->
      <section id="cta" class="relative overflow-hidden bg-gradient-to-r from-[#021b4a] via-[#0757b8] to-[#20f5b4] px-6 py-20 text-white">
        <div class="pointer-events-none absolute -right-20 -top-20 h-60 w-60 rounded-full bg-white/10 blur-3xl"></div>
        <div class="pointer-events-none absolute -bottom-20 -left-20 h-60 w-60 rounded-full bg-white/10 blur-3xl"></div>
        <div class="relative mx-auto max-w-7xl text-center">
          <div class="mb-3 text-sm font-bold uppercase tracking-wider text-white/70">{{ t('enterprise.cta.label') }}</div>
          <h2 class="mb-4 text-3xl font-bold md:text-4xl">{{ t('enterprise.cta.title') }}</h2>
          <p class="mx-auto mb-8 max-w-xl text-white/80">{{ t('enterprise.cta.subtitle') }}</p>
          <div class="flex flex-wrap justify-center gap-4">
            <button
              @click="router.push('/register')"
              class="inline-flex items-center gap-2 rounded-full bg-white px-8 py-4 text-lg font-semibold text-[#0757b8] shadow-lg transition-all hover:scale-105"
            >
              {{ t('enterprise.cta.primary') }}
            </button>
            <button
              @click="scrollTo('employee-apply')"
              class="inline-flex items-center gap-2 rounded-full border border-white px-8 py-4 text-lg font-semibold text-white transition-all hover:bg-white/10"
            >
              {{ t('enterprise.cta.secondary') }}
            </button>
          </div>
        </div>
      </section>

      <!-- Employee Apply -->
      <section id="employee-apply" class="px-6 py-20">
        <div class="mx-auto max-w-7xl">
          <div class="mb-12 text-center">
            <div class="mb-3 text-sm font-bold uppercase tracking-wider text-[#0757b8]">{{ t('enterprise.employeeApply.label') }}</div>
            <h2 class="mb-4 text-3xl font-bold text-gray-900 dark:text-white md:text-4xl">{{ t('enterprise.employeeApply.title') }}</h2>
            <p class="mx-auto max-w-2xl text-gray-500 dark:text-gray-400">{{ t('enterprise.employeeApply.subtitle') }}</p>
          </div>

          <div class="grid gap-8 lg:grid-cols-2">
            <div class="rounded-2xl border border-gray-100 bg-white p-8 dark:border-dark-700 dark:bg-dark-800">
              <h3 class="mb-6 text-xl font-bold text-gray-900 dark:text-white">{{ t('enterprise.employeeApply.cardTitle') }}</h3>
              <button
                @click="generateApplication"
                class="w-full rounded-xl bg-gradient-to-r from-[#0757b8] to-[#20f5b4] py-4 text-lg font-semibold text-white shadow-lg transition-all hover:scale-[1.02]"
              >
                {{ t('enterprise.employeeApply.generateBtn') }}
              </button>
              <div v-if="applicationText" class="mt-6 rounded-xl border border-gray-100 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-900/50">
                <div class="mb-3 flex items-center justify-between">
                  <span class="font-semibold text-gray-900 dark:text-white">{{ t('enterprise.employeeApply.applicationLabel') }}</span>
                  <button
                    @click="copyApplication"
                    class="rounded-lg bg-[#0757b8] px-3 py-1.5 text-sm text-white"
                  >
                    {{ t('enterprise.employeeApply.copyBtn') }}
                  </button>
                </div>
                <p class="whitespace-pre-wrap text-sm leading-relaxed text-gray-600 dark:text-gray-300">{{ applicationText }}</p>
              </div>
            </div>

            <div class="space-y-4">
              <h3 class="text-xl font-bold text-gray-900 dark:text-white">{{ t('enterprise.employeeApply.benefitsTitle') }}</h3>
              <div v-for="(benefit, idx) in benefits" :key="idx" class="flex items-start gap-4 rounded-xl border border-gray-100 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
                <div class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl bg-blue-100 text-blue-600 dark:bg-blue-900/30 dark:text-blue-300">
                  {{ ['💡', '🚀', '🔒', '👥'][idx] }}
                </div>
                <div>
                  <div class="font-semibold text-gray-900 dark:text-white">{{ benefit.title }}</div>
                  <div class="text-sm text-gray-500 dark:text-gray-400">{{ benefit.desc }}</div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- Enterprise FAQ Section -->
      <section id="faq" class="px-6 py-16">
        <div class="mx-auto max-w-4xl">
          <div class="mb-10 text-center">
            <p class="text-xs font-bold uppercase tracking-[0.22em] text-[#0757b8]">{{ t('enterprise.faq.eyebrow') }}</p>
            <h2 class="mt-3 text-3xl font-black tracking-tight text-gray-950 dark:text-white md:text-4xl">{{ t('enterprise.faq.title') }}</h2>
            <p class="mx-auto mt-4 max-w-2xl text-sm leading-relaxed text-gray-500 dark:text-gray-400">{{ t('enterprise.faq.subtitle') }}</p>
          </div>

          <div class="space-y-3">
            <details
              v-for="i in 8"
              :key="i"
              class="group rounded-2xl border border-gray-200 bg-white/90 p-5 shadow-sm transition-all hover:border-[#0757b8]/30 hover:shadow-md dark:border-dark-700 dark:bg-dark-800/60"
            >
              <summary class="flex cursor-pointer items-center justify-between gap-4 list-none">
                <h3 class="text-base font-bold text-gray-950 dark:text-white">{{ t(`enterprise.faq.q${i}`) }}</h3>
                <span class="flex h-6 w-6 flex-shrink-0 items-center justify-center rounded-full bg-gray-100 text-gray-500 transition-transform group-open:rotate-180 dark:bg-dark-700 dark:text-gray-400">
                  <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" /></svg>
                </span>
              </summary>
              <p class="mt-3 text-sm leading-7 text-gray-600 dark:text-gray-300">{{ t(`enterprise.faq.a${i}`) }}</p>
            </details>
          </div>
        </div>
      </section>
    </main>

    <!-- Footer -->
    <footer class="relative z-10 border-t border-gray-100 bg-white px-6 py-12 dark:border-dark-700 dark:bg-dark-900">
      <div class="mx-auto max-w-7xl">
        <!-- Original enterprise footer content -->
        <div class="flex flex-col items-center justify-between gap-6 text-center sm:flex-row sm:text-left">
          <div>
            <div class="mb-3 flex items-center justify-center gap-3 sm:justify-start">
              <img :src="siteLogo || '/src/assets/icons/logo.webp'" alt="Logo" class="h-8 w-8 object-contain" />
              <span class="text-lg font-semibold text-[#021b4a] dark:text-white">{{ siteName }}</span>
            </div>
            <p class="max-w-md text-sm text-gray-500 dark:text-gray-400">{{ t('enterprise.footer.brandDesc') }}</p>
          </div>
          <div class="text-sm text-gray-400">
            <div>{{ t('enterprise.footer.slogan') }}</div>
          </div>
        </div>
        <!-- Home-style footer links -->
        <div class="mt-8 flex flex-col items-center justify-center gap-4 text-center text-sm text-gray-500 sm:flex-row sm:text-left">
          <p>&copy; {{ currentYear }} Three Router. {{ t('home.footer.allRightsReserved') }}</p>
          <div class="flex items-center gap-6">
            <a :href="currentLang === 'zh' ? 'readme-cn.html' : 'readme-en.html'" class="transition-colors hover:text-gray-700">{{ t('home.footer.advantage') }}</a>
          </div>
          <div class="flex items-center gap-6">
            <a :href="currentLang === 'zh' ? 'help-cn.html' : 'help-en.html'" class="transition-colors hover:text-gray-700">{{ t('home.footer.documentation') }}</a>
          </div>
          <div class="flex items-center gap-6">
            <a :href="currentLang === 'zh' ? 'help-cn.html#contact' : 'help-en.html#contact'" class="transition-colors hover:text-gray-700">{{ t('home.footer.contact') }}</a>
          </div>
        </div>
        <div class="mt-4 text-center">
          <p class="text-xs text-gray-400">{{ t('home.footer.legalNotice') }}</p>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import { useSEO } from '@/composables/useSEO'

const { t, tm, locale } = useI18n()
const router = useRouter()
const appStore = useAppStore()

// FAQPage 结构化数据 - 从 i18n FAQ 内容动态构建，支持中英文切换
const enterpriseFaqJsonLd = computed(() => ({
  '@context': 'https://schema.org',
  '@type': 'FAQPage',
  'mainEntity': [
    { '@type': 'Question', 'name': t('enterprise.faq.q1'), 'acceptedAnswer': { '@type': 'Answer', 'text': t('enterprise.faq.a1') } },
    { '@type': 'Question', 'name': t('enterprise.faq.q2'), 'acceptedAnswer': { '@type': 'Answer', 'text': t('enterprise.faq.a2') } },
    { '@type': 'Question', 'name': t('enterprise.faq.q3'), 'acceptedAnswer': { '@type': 'Answer', 'text': t('enterprise.faq.a3') } },
    { '@type': 'Question', 'name': t('enterprise.faq.q4'), 'acceptedAnswer': { '@type': 'Answer', 'text': t('enterprise.faq.a4') } },
    { '@type': 'Question', 'name': t('enterprise.faq.q5'), 'acceptedAnswer': { '@type': 'Answer', 'text': t('enterprise.faq.a5') } },
    { '@type': 'Question', 'name': t('enterprise.faq.q6'), 'acceptedAnswer': { '@type': 'Answer', 'text': t('enterprise.faq.a6') } },
    { '@type': 'Question', 'name': t('enterprise.faq.q7'), 'acceptedAnswer': { '@type': 'Answer', 'text': t('enterprise.faq.a7') } },
    { '@type': 'Question', 'name': t('enterprise.faq.q8'), 'acceptedAnswer': { '@type': 'Answer', 'text': t('enterprise.faq.a8') } },
  ]
}))

// 合并 Service + FAQPage 结构化数据
const enterpriseJsonLd = computed(() => [
  {
    '@context': 'https://schema.org',
    '@type': 'Service',
    'name': 'ThreeRouter Enterprise AI API Management',
    'description': 'Enterprise-grade AI API management platform with fine-grained token governance, team quota control, and security compliance audit.',
    'url': 'https://www.threerouter.com/enterprise',
    'serviceType': 'AI API Gateway & Token Management',
    'provider': {
      '@type': 'Organization',
      'name': 'ThreeRouter',
      'url': 'https://www.threerouter.com'
    },
    'areaServed': 'Global',
    'audience': {
      '@type': 'BusinessAudience',
      'name': 'Enterprise IT & Engineering Teams'
    }
  },
  enterpriseFaqJsonLd.value
])

useSEO({
  title: 'Enterprise - ThreeRouter | Enterprise AI API Management & Fine-Grained Token Governance',
  description: 'ThreeRouter for enterprises: unified team management, security and compliance, fine-grained token governance. Enterprise AI API gateway with department quotas, API key control, cost attribution, and compliance audit — track every token.',
  keywords: 'enterprise AI,enterprise API management,token governance,team management,API gateway,enterprise compliance,API key management,cost control,token quota,usage monitoring',
  ogType: 'website',
  ogImage: 'https://www.threerouter.com/src/assets/icons/logo.webp',
  ogUrl: 'https://www.threerouter.com/enterprise',
  ogSiteName: 'ThreeRouter',
  canonicalUrl: 'https://www.threerouter.com/enterprise',
  jsonLd: enterpriseJsonLd
})

const currentYear = computed(() => new Date().getFullYear())
const currentLang = computed<'zh' | 'en'>(() => (locale.value === 'zh' ? 'zh' : 'en'))

const selectedRole = ref<'decision' | 'employee'>('decision')

const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'ThreeRouter')
const siteLogo = computed(() => appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '')

const solutionCard1Points = computed(() => tm('enterprise.solution.card1Points') as unknown as string[])
const solutionCard2Points = computed(() => tm('enterprise.solution.card2Points') as unknown as string[])
const solutionCard3Points = computed(() => tm('enterprise.solution.card3Points') as unknown as string[])

const teamFeature1Points = computed(() => tm('enterprise.team.feature1Points') as unknown as string[])
const teamFeature2Points = computed(() => tm('enterprise.team.feature2Points') as unknown as string[])
const teamFeature3Points = computed(() => tm('enterprise.team.feature3Points') as unknown as string[])
const teamFeature4Points = computed(() => tm('enterprise.team.feature4Points') as unknown as string[])
const teamFeature5Points = computed(() => tm('enterprise.team.feature5Points') as unknown as string[])
const teamFeature6Points = computed(() => tm('enterprise.team.feature6Points') as unknown as string[])

const teamWorkflowStep1Points = computed(() => tm('enterprise.teamWorkflow.step1Points') as unknown as string[])
const teamWorkflowStep2Points = computed(() => tm('enterprise.teamWorkflow.step2Points') as unknown as string[])
const teamWorkflowStep3Points = computed(() => tm('enterprise.teamWorkflow.step3Points') as unknown as string[])

const tokenMetrics = computed(() => tm('enterprise.tokenManagement.metrics') as unknown as Array<{ value: string; label: string; desc: string }>)
const tokenManagementFeatures = computed(() => tm('enterprise.tokenManagement.features') as unknown as Array<{ icon: string; iconBg: string; title: string; desc: string; points: string[] }>)
const tokenProcessSteps = computed(() => tm('enterprise.tokenManagement.processSteps') as unknown as Array<{ title: string; desc: string }>)

const isDark = ref(document.documentElement.classList.contains('dark'))

onMounted(() => {
  appStore.fetchPublicSettings().catch(() => {})
})

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

function scrollTo(id: string) {
  const el = document.getElementById(id)
  if (el) el.scrollIntoView({ behavior: 'smooth' })
}

const compareKeys = ['entity', 'security', 'transparency', 'sla', 'invoice', 'support', 'price']

const benefits = computed(() => {
  return tm('enterprise.employeeApply.benefits') as unknown as Array<{ title: string; desc: string }>
})

const applicationText = ref('')

function generateApplication() {
  const template = tm('enterprise.employeeApply.applicationTemplate') as unknown as {
    greeting: string; intro: string; reason1: string; reason2: string; reason3: string; reason4: string; closing: string;
  }
  applicationText.value = [
    template.greeting,
    '',
    template.intro,
    '',
    template.reason1,
    template.reason2,
    template.reason3,
    template.reason4,
    '',
    template.closing,
  ].join('\n')
}

function copyApplication() {
  if (!applicationText.value) return
  navigator.clipboard.writeText(applicationText.value)
  alert(t('enterprise.employeeApply.copySuccess'))
}
</script>
