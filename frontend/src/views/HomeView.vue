<template>
  <div
    class="relative flex min-h-screen flex-col overflow-hidden bg-gradient-to-br from-slate-50 via-blue-50/30 to-cyan-50"
  >
    <!-- Background Decorations -->
    <div class="pointer-events-none absolute inset-0 overflow-hidden">
      <div
        class="absolute right-0 top-0 h-[600px] w-[600px] rounded-full bg-gradient-to-br from-blue-500/20 to-cyan-400/20 blur-3xl"
      ></div>
      <div
        class="absolute -bottom-40 left-0 h-[500px] w-[500px] rounded-full bg-gradient-to-br from-emerald-400/15 to-teal-500/15 blur-3xl"
      ></div>
      <div
        class="absolute inset-0 bg-[linear-gradient(rgba(59,130,246,0.04)_1px,transparent_1px),linear-gradient(90deg,rgba(59,130,246,0.04)_1px,transparent_1px)] bg-[size:64px_64px]"
      ></div>
    </div>

    <!-- Header -->
    <header class="relative z-20 px-6 py-4">
      <nav class="mx-auto flex max-w-7xl items-center justify-between">
        <!-- Logo -->
        <div class="flex items-center gap-[12px] pl-0">
          <img 
            :src="LogoSvg"
            alt="Three Router Logo" 
            class="h-[32px] w-[32px] object-contain"
          />
          <span class="text-[16px] font-semibold text-black" style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;">Three Router</span>
        </div>

        <!-- Nav Actions -->
        <div class="flex items-center gap-3">
          <!-- Limited Time Banner -->
          <div class="hidden items-center gap-2 rounded-full bg-orange-100 px-4 py-2 sm:flex">
            <svg class="h-4 w-4 text-orange-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <span class="text-sm font-medium text-orange-700">LIMITED TIME: FREE 10$ TOKENS</span>
          </div>

          <!-- Language Toggle -->
          <button
            @click="toggleLanguage"
            class="rounded-lg p-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-100"
          >
            {{ currentLang === 'zh' ? '中文' : 'EN' }}
          </button>

          <!-- Theme Toggle -->
          <button
            @click="toggleTheme"
            class="rounded-lg p-2 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>

          <!-- Login Button -->
          <button @click="router.push('/login')" class="rounded-lg bg-gray-900 px-5 py-2 text-sm font-semibold text-white transition-colors hover:bg-gray-800 cursor-pointer">
            {{ currentLang === 'zh' ? '登录' : 'Login' }}
          </button>
        </div>
      </nav>
    </header>

    <!-- Main Content -->
    <main class="relative z-10 flex-1 px-6 py-16">
      <div class="mx-auto max-w-7xl">
        <!-- Hero Section -->
        <div class="mb-16 flex flex-col items-center justify-between gap-12 lg:flex-row lg:gap-16">
          <!-- Left: Text Content -->
          <div class="flex-1 text-center lg:text-left">
            <h1
              class="mb-4 text-4xl font-bold text-gray-900 md:text-5xl lg:text-6xl"
            >
              {{ t('home.hero.title') }}
            </h1>
            <p class="mb-6 text-lg text-gray-600 md:text-xl">
              {{ t('home.hero.subtitle') }}
            </p>

            <!-- Feature Tags -->
            <div class="mb-8 flex flex-wrap items-center justify-center gap-2 lg:justify-start">
              <span class="inline-flex items-center gap-1.5 rounded-full bg-orange-100 px-3 py-1.5 text-sm font-medium text-orange-700">
                <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                {{ t('home.hero.tags.discount') }}
              </span>
              <span class="inline-flex items-center gap-1.5 rounded-full bg-blue-100 px-3 py-1.5 text-sm font-medium text-blue-700">
                <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17.657 16.657L13.414 20.9a1.998 1.998 0 01-2.827 0l-4.244-4.243a8 8 0 1111.314 0z" />
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 11a3 3 0 11-6 0 3 3 0 016 0z" />
                </svg>
                {{ t('home.hero.tags.deployment') }}
              </span>
              <span class="inline-flex items-center gap-1.5 rounded-full bg-green-100 px-3 py-1.5 text-sm font-medium text-green-700">
                <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 7h6m0 10v-3m-3 3h.01M9 17h.01M9 14h.01M12 14h.01M15 11h.01M12 11h.01M9 11h.01M7 21h10a2 2 0 002-2V5a2 2 0 00-2-2H7a2 2 0 00-2 2v14a2 2 0 002 2z" />
                </svg>
                {{ t('home.hero.tags.freeTokens') }}
              </span>
            </div>

            <!-- CTA Button -->
            <button @click="router.push('/login')" class="inline-flex items-center gap-2 rounded-full bg-gradient-to-r from-green-500 to-emerald-600 px-8 py-3.5 text-base font-semibold text-white shadow-lg shadow-green-500/30 transition-all hover:scale-105 hover:shadow-xl cursor-pointer">
              {{ t('home.hero.cta') }}
              <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 8l4 4m0 0l-4 4m4-4H3" />
              </svg>
            </button>
          </div>

          <!-- Right: Terminal Animation -->
          <div class="flex flex-1 justify-center lg:justify-end">
            <div class="terminal-container">
              <div class="terminal-window">
                <div class="terminal-header">
                  <div class="terminal-buttons">
                    <span class="btn-close"></span>
                    <span class="btn-minimize"></span>
                    <span class="btn-maximize"></span>
                  </div>
                </div>
                <div class="terminal-body">
                  <div class="code-line line-1">
                    <span class="code-prompt">curl</span>
                    <span class="code-flag">-X POST</span>
                    <span class="code-url">https://threerouter.com/v1/images/generations</span>
                  </div>
                  <div class="code-line line-2">
                    <span class="code-header">Authorization:</span>
                    <span class="code-token">Bearer sk-xxxx</span>
                  </div>
                  <div class="code-line line-3">
                    <span class="code-brace">{</span>
                  </div>
                  <div class="code-line line-4">
                    <span class="code-indent"></span>
                    <span class="code-key">"model":</span>
                    <span class="code-string">"stabledream-5.0",</span>
                  </div>
                  <div class="code-line line-5">
                    <span class="code-indent"></span>
                    <span class="code-key">"prompt":</span>
                    <span class="code-string">"A futuristic city at sunset"</span>
                    <span class="code-price">200 OK$</span>
                  </div>
                  <div class="code-line line-6">
                    <span class="code-brace">}</span>
                  </div>
                  <div class="code-line line-7">
                    <span class="code-url">https://threerouter.com/generate/xxxxx</span>
                    <span class="code-price">200 OK$</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Features Grid -->
        <div class="mb-16 grid gap-6 md:grid-cols-3">
          <!-- Feature 1: Unified Gateway -->
          <div
            class="group rounded-2xl border border-gray-100 bg-white p-6 shadow-sm transition-all duration-300 hover:border-blue-200 hover:shadow-lg hover:shadow-blue-500/10"
          >
            <div
              class="mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-gradient-to-br from-blue-500 to-blue-600 shadow-lg shadow-blue-500/30"
            >
              <svg class="h-6 w-6 text-white" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z" />
              </svg>
            </div>
            <h3 class="mb-2 text-lg font-semibold text-gray-900">{{ t('home.features.unifiedGateway') }}</h3>
            <p class="text-sm text-gray-600">{{ t('home.features.unifiedGatewayDesc') }}</p>
          </div>

          <!-- Feature 2: Multi Account -->
          <div
            class="group rounded-2xl border border-gray-100 bg-white p-6 shadow-sm transition-all duration-300 hover:border-emerald-200 hover:shadow-lg hover:shadow-emerald-500/10"
          >
            <div
              class="mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-gradient-to-br from-emerald-500 to-teal-600 shadow-lg shadow-emerald-500/30"
            >
              <svg class="h-6 w-6 text-white" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
              </svg>
            </div>
            <h3 class="mb-2 text-lg font-semibold text-gray-900">{{ t('home.features.multiAccount') }}</h3>
            <p class="text-sm text-gray-600">{{ t('home.features.multiAccountDesc') }}</p>
          </div>

          <!-- Feature 3: Balance Quota -->
          <div
            class="group rounded-2xl border border-gray-100 bg-white p-6 shadow-sm transition-all duration-300 hover:border-purple-200 hover:shadow-lg hover:shadow-purple-500/10"
          >
            <div
              class="mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-gradient-to-br from-purple-500 to-violet-600 shadow-lg shadow-purple-500/30"
            >
              <svg class="h-6 w-6 text-white" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </div>
            <h3 class="mb-2 text-lg font-semibold text-gray-900">{{ t('home.features.balanceQuota') }}</h3>
            <p class="text-sm text-gray-600">{{ t('home.features.balanceQuotaDesc') }}</p>
          </div>
        </div>

        <!-- Supported AI Models -->
        <div class="mb-16 rounded-2xl border border-gray-100 bg-white p-8 shadow-sm">
          <div class="mb-6 text-center">
            <h3 class="mb-2 text-xl font-bold text-gray-900">{{ t('home.providers.title') }}</h3>
            <p class="text-sm text-gray-500">{{ t('home.providers.description') }}</p>
          </div>
          <div class="flex flex-wrap items-center justify-center gap-3">
            <span class="flex items-center gap-2 rounded-xl border border-cyan-100 bg-white px-4 py-2.5">
              <span class="flex h-6 w-6 items-center justify-center rounded-full bg-cyan-500 text-xs font-bold text-white">D</span>
              <span class="text-sm font-medium text-gray-700">deepseek-v4-pro</span>
              <span class="rounded-full bg-emerald-100 px-2 py-0.5 text-xs font-medium text-emerald-600">{{ t('home.providers.supported') }}</span>
            </span>
            <span class="flex items-center gap-2 rounded-xl border border-amber-100 bg-white px-4 py-2.5">
              <span class="flex h-6 w-6 items-center justify-center rounded-full bg-amber-500 text-xs font-bold text-white">M</span>
              <span class="text-sm font-medium text-gray-700">minimax-m3</span>
              <span class="rounded-full bg-emerald-100 px-2 py-0.5 text-xs font-medium text-emerald-600">{{ t('home.providers.supported') }}</span>
            </span>
            <span class="flex items-center gap-2 rounded-xl border border-indigo-100 bg-white px-4 py-2.5">
              <span class="flex h-6 w-6 items-center justify-center rounded-full bg-indigo-500 text-xs font-bold text-white">K</span>
              <span class="text-sm font-medium text-gray-700">kimi-k2.7</span>
              <span class="rounded-full bg-emerald-100 px-2 py-0.5 text-xs font-medium text-emerald-600">{{ t('home.providers.supported') }}</span>
            </span>
            <span class="flex items-center gap-2 rounded-xl border border-orange-100 bg-white px-4 py-2.5">
              <span class="flex h-6 w-6 items-center justify-center rounded-full bg-orange-500 text-xs font-bold text-white">Q</span>
              <span class="text-sm font-medium text-gray-700">qwen3.7-max</span>
              <span class="rounded-full bg-emerald-100 px-2 py-0.5 text-xs font-medium text-emerald-600">{{ t('home.providers.supported') }}</span>
            </span>
            <span class="flex items-center gap-2 rounded-xl border border-blue-100 bg-white px-4 py-2.5">
              <span class="flex h-6 w-6 items-center justify-center rounded-full bg-blue-500 text-xs font-bold text-white">G</span>
              <span class="text-sm font-medium text-gray-700">glm-5.2</span>
              <span class="rounded-full bg-emerald-100 px-2 py-0.5 text-xs font-medium text-emerald-600">{{ t('home.providers.supported') }}</span>
            </span>
            <span class="flex items-center gap-2 rounded-xl border border-rose-100 bg-white px-4 py-2.5">
              <span class="flex h-6 w-6 items-center justify-center rounded-full bg-rose-500 text-xs font-bold text-white">S</span>
              <span class="text-sm font-medium text-gray-700">seedance-2.0</span>
              <span class="rounded-full bg-emerald-100 px-2 py-0.5 text-xs font-medium text-emerald-600">{{ t('home.providers.supported') }}</span>
            </span>
            <span @click="router.push('/login')" class="flex items-center gap-2 rounded-xl border border-gray-200 bg-gray-50 px-4 py-2.5 opacity-60 cursor-pointer hover:opacity-100 transition-opacity">
              <span class="flex h-6 w-6 items-center justify-center rounded-full bg-gray-400 text-xs font-bold text-white">+</span>
              <span class="text-sm font-medium text-gray-500">{{ t('home.providers.more') }}</span>
              <span class="rounded-full bg-gray-200 px-2 py-0.5 text-xs font-medium text-gray-500">{{ t('home.providers.loginToView') }}</span>
            </span>
          </div>
        </div>

        <!-- CTA Banner -->
        <div class="relative overflow-hidden rounded-2xl bg-gradient-to-r from-blue-600 via-cyan-500 to-emerald-500 p-8 md:p-12">
          <div class="absolute -right-20 -top-20 h-60 w-60 rounded-full bg-white/10 blur-3xl"></div>
          <div class="absolute -bottom-20 -left-20 h-60 w-60 rounded-full bg-white/10 blur-3xl"></div>
          <div class="relative z-10 flex flex-col items-center justify-between gap-6 text-center md:flex-row">
            <div class="flex items-center gap-3">
              <svg class="h-8 w-8 text-white/90" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11.049 2.927c.3-.921 1.603-.921 1.902 0l1.519 4.674a1 1 0 00.95.69h4.915c.969 0 1.371 1.24.588 1.81l-3.976 2.888a1 1 0 00-.363 1.118l1.518 4.674c.3.922-.755 1.688-1.538 1.118l-3.976-2.888a1 1 0 00-1.176 0l-3.976 2.888c-.783.57-1.838-.197-1.538-1.118l1.518-4.674a1 1 0 00-.363-1.118l-3.976-2.888c-.784-.57-.38-1.81.588-1.81h4.914a1 1 0 00.951-.69l1.519-4.674z" />
              </svg>
              <div class="text-white">
                <p class="text-lg font-semibold">LIMITED TIME OFFER: Sign up today and get 10$ FREE tokens!</p>
                <p class="text-sm opacity-80">No credit card required. Expires in 7 days.</p>
              </div>
            </div>
            <button @click="router.push('/login')" class="rounded-full bg-white px-8 py-3 font-semibold text-blue-600 shadow-lg transition-all hover:scale-105 hover:shadow-xl cursor-pointer">
              Claim Your Free Tokens
            </button>
          </div>
        </div>

        <!-- User Reviews Section -->
        <div class="mt-16 mb-16">
          <div class="text-center mb-12">
            <h2 class="text-3xl font-bold text-gray-900 mb-4">{{ t('home.reviews.title') }}</h2>
            <p class="text-lg text-gray-600">{{ t('home.reviews.subtitle') }}</p>
          </div>

          <!-- Row 1: Reviews 1-7 (scroll left) -->
          <div class="relative overflow-hidden mb-6">
            <div class="testimonial-track-1 flex gap-6">
              <!-- First set (1-7) -->
              <div class="flex-shrink-0 w-80 rounded-2xl border border-gray-100 bg-white p-6 shadow-sm transition-all duration-300 hover:shadow-lg">
                <div class="flex items-start gap-4">
                  <img src="/people/1.jpg" alt="David Z." class="h-12 w-12 rounded-full object-cover" />
                  <div class="flex-1">
                    <p class="text-gray-700 mb-3">{{ t('home.reviews.review1.text') }}</p>
                    <div class="flex items-center gap-2">
                      <p class="font-semibold text-gray-900">{{ t('home.reviews.review1.name') }}</p>
                      <p class="text-sm text-gray-500">{{ t('home.reviews.review1.role') }}</p>
                    </div>
                  </div>
                </div>
              </div>
              <div class="flex-shrink-0 w-80 rounded-2xl border border-gray-100 bg-white p-6 shadow-sm transition-all duration-300 hover:shadow-lg">
                <div class="flex items-start gap-4">
                  <img src="/people/2.jpg" alt="Li" class="h-12 w-12 rounded-full object-cover" />
                  <div class="flex-1">
                    <p class="text-gray-700 mb-3">{{ t('home.reviews.review2.text') }}</p>
                    <div class="flex items-center gap-2">
                      <p class="font-semibold text-gray-900">{{ t('home.reviews.review2.name') }}</p>
                      <p class="text-sm text-gray-500">{{ t('home.reviews.review2.role') }}</p>
                    </div>
                  </div>
                </div>
              </div>
              <div class="flex-shrink-0 w-80 rounded-2xl border border-gray-100 bg-white p-6 shadow-sm transition-all duration-300 hover:shadow-lg">
                <div class="flex items-start gap-4">
                  <img src="/people/3.jpg" alt="Emily W." class="h-12 w-12 rounded-full object-cover" />
                  <div class="flex-1">
                    <p class="text-gray-700 mb-3">{{ t('home.reviews.review3.text') }}</p>
                    <div class="flex items-center gap-2">
                      <p class="font-semibold text-gray-900">{{ t('home.reviews.review3.name') }}</p>
                      <p class="text-sm text-gray-500">{{ t('home.reviews.review3.role') }}</p>
                    </div>
                  </div>
                </div>
              </div>
              <div class="flex-shrink-0 w-80 rounded-2xl border border-gray-100 bg-white p-6 shadow-sm transition-all duration-300 hover:shadow-lg">
                <div class="flex items-start gap-4">
                  <img src="/people/4.jpg" alt="Chen" class="h-12 w-12 rounded-full object-cover" />
                  <div class="flex-1">
                    <p class="text-gray-700 mb-3">{{ t('home.reviews.review4.text') }}</p>
                    <div class="flex items-center gap-2">
                      <p class="font-semibold text-gray-900">{{ t('home.reviews.review4.name') }}</p>
                      <p class="text-sm text-gray-500">{{ t('home.reviews.review4.role') }}</p>
                    </div>
                  </div>
                </div>
              </div>
              <div class="flex-shrink-0 w-80 rounded-2xl border border-gray-100 bg-white p-6 shadow-sm transition-all duration-300 hover:shadow-lg">
                <div class="flex items-start gap-4">
                  <img src="/people/5.jpg" alt="Sarah L." class="h-12 w-12 rounded-full object-cover" />
                  <div class="flex-1">
                    <p class="text-gray-700 mb-3">{{ t('home.reviews.review5.text') }}</p>
                    <div class="flex items-center gap-2">
                      <p class="font-semibold text-gray-900">{{ t('home.reviews.review5.name') }}</p>
                      <p class="text-sm text-gray-500">{{ t('home.reviews.review5.role') }}</p>
                    </div>
                  </div>
                </div>
              </div>
              <div class="flex-shrink-0 w-80 rounded-2xl border border-gray-100 bg-white p-6 shadow-sm transition-all duration-300 hover:shadow-lg">
                <div class="flex items-start gap-4">
                  <img src="/people/6.jpg" alt="Zhao" class="h-12 w-12 rounded-full object-cover" />
                  <div class="flex-1">
                    <p class="text-gray-700 mb-3">{{ t('home.reviews.review6.text') }}</p>
                    <div class="flex items-center gap-2">
                      <p class="font-semibold text-gray-900">{{ t('home.reviews.review6.name') }}</p>
                      <p class="text-sm text-gray-500">{{ t('home.reviews.review6.role') }}</p>
                    </div>
                  </div>
                </div>
              </div>
              <div class="flex-shrink-0 w-80 rounded-2xl border border-gray-100 bg-white p-6 shadow-sm transition-all duration-300 hover:shadow-lg">
                <div class="flex items-start gap-4">
                  <img src="/people/7.jpg" alt="Alice S." class="h-12 w-12 rounded-full object-cover" />
                  <div class="flex-1">
                    <p class="text-gray-700 mb-3">{{ t('home.reviews.review7.text') }}</p>
                    <div class="flex items-center gap-2">
                      <p class="font-semibold text-gray-900">{{ t('home.reviews.review7.name') }}</p>
                      <p class="text-sm text-gray-500">{{ t('home.reviews.review7.role') }}</p>
                    </div>
                  </div>
                </div>
              </div>
              <!-- Duplicate set for seamless scroll (1-7) -->
              <div class="flex-shrink-0 w-80 rounded-2xl border border-gray-100 bg-white p-6 shadow-sm transition-all duration-300 hover:shadow-lg">
                <div class="flex items-start gap-4">
                  <img src="/people/1.jpg" alt="David Z." class="h-12 w-12 rounded-full object-cover" />
                  <div class="flex-1">
                    <p class="text-gray-700 mb-3">{{ t('home.reviews.review1.text') }}</p>
                    <div class="flex items-center gap-2">
                      <p class="font-semibold text-gray-900">{{ t('home.reviews.review1.name') }}</p>
                      <p class="text-sm text-gray-500">{{ t('home.reviews.review1.role') }}</p>
                    </div>
                  </div>
                </div>
              </div>
              <div class="flex-shrink-0 w-80 rounded-2xl border border-gray-100 bg-white p-6 shadow-sm transition-all duration-300 hover:shadow-lg">
                <div class="flex items-start gap-4">
                  <img src="/people/2.jpg" alt="Li" class="h-12 w-12 rounded-full object-cover" />
                  <div class="flex-1">
                    <p class="text-gray-700 mb-3">{{ t('home.reviews.review2.text') }}</p>
                    <div class="flex items-center gap-2">
                      <p class="font-semibold text-gray-900">{{ t('home.reviews.review2.name') }}</p>
                      <p class="text-sm text-gray-500">{{ t('home.reviews.review2.role') }}</p>
                    </div>
                  </div>
                </div>
              </div>
              <div class="flex-shrink-0 w-80 rounded-2xl border border-gray-100 bg-white p-6 shadow-sm transition-all duration-300 hover:shadow-lg">
                <div class="flex items-start gap-4">
                  <img src="/people/3.jpg" alt="Emily W." class="h-12 w-12 rounded-full object-cover" />
                  <div class="flex-1">
                    <p class="text-gray-700 mb-3">{{ t('home.reviews.review3.text') }}</p>
                    <div class="flex items-center gap-2">
                      <p class="font-semibold text-gray-900">{{ t('home.reviews.review3.name') }}</p>
                      <p class="text-sm text-gray-500">{{ t('home.reviews.review3.role') }}</p>
                    </div>
                  </div>
                </div>
              </div>
              <div class="flex-shrink-0 w-80 rounded-2xl border border-gray-100 bg-white p-6 shadow-sm transition-all duration-300 hover:shadow-lg">
                <div class="flex items-start gap-4">
                  <img src="/people/4.jpg" alt="Chen" class="h-12 w-12 rounded-full object-cover" />
                  <div class="flex-1">
                    <p class="text-gray-700 mb-3">{{ t('home.reviews.review4.text') }}</p>
                    <div class="flex items-center gap-2">
                      <p class="font-semibold text-gray-900">{{ t('home.reviews.review4.name') }}</p>
                      <p class="text-sm text-gray-500">{{ t('home.reviews.review4.role') }}</p>
                    </div>
                  </div>
                </div>
              </div>
              <div class="flex-shrink-0 w-80 rounded-2xl border border-gray-100 bg-white p-6 shadow-sm transition-all duration-300 hover:shadow-lg">
                <div class="flex items-start gap-4">
                  <img src="/people/5.jpg" alt="Sarah L." class="h-12 w-12 rounded-full object-cover" />
                  <div class="flex-1">
                    <p class="text-gray-700 mb-3">{{ t('home.reviews.review5.text') }}</p>
                    <div class="flex items-center gap-2">
                      <p class="font-semibold text-gray-900">{{ t('home.reviews.review5.name') }}</p>
                      <p class="text-sm text-gray-500">{{ t('home.reviews.review5.role') }}</p>
                    </div>
                  </div>
                </div>
              </div>
              <div class="flex-shrink-0 w-80 rounded-2xl border border-gray-100 bg-white p-6 shadow-sm transition-all duration-300 hover:shadow-lg">
                <div class="flex items-start gap-4">
                  <img src="/people/6.jpg" alt="Zhao" class="h-12 w-12 rounded-full object-cover" />
                  <div class="flex-1">
                    <p class="text-gray-700 mb-3">{{ t('home.reviews.review6.text') }}</p>
                    <div class="flex items-center gap-2">
                      <p class="font-semibold text-gray-900">{{ t('home.reviews.review6.name') }}</p>
                      <p class="text-sm text-gray-500">{{ t('home.reviews.review6.role') }}</p>
                    </div>
                  </div>
                </div>
              </div>
              <div class="flex-shrink-0 w-80 rounded-2xl border border-gray-100 bg-white p-6 shadow-sm transition-all duration-300 hover:shadow-lg">
                <div class="flex items-start gap-4">
                  <img src="/people/7.jpg" alt="Alice S." class="h-12 w-12 rounded-full object-cover" />
                  <div class="flex-1">
                    <p class="text-gray-700 mb-3">{{ t('home.reviews.review7.text') }}</p>
                    <div class="flex items-center gap-2">
                      <p class="font-semibold text-gray-900">{{ t('home.reviews.review7.name') }}</p>
                      <p class="text-sm text-gray-500">{{ t('home.reviews.review7.role') }}</p>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- Row 2: Reviews 8-14 (scroll right) -->
          <div class="relative overflow-hidden">
            <div class="testimonial-track-2 flex gap-6">
              <!-- First set (8-14) -->
              <div class="flex-shrink-0 w-80 rounded-2xl border border-gray-100 bg-white p-6 shadow-sm transition-all duration-300 hover:shadow-lg">
                <div class="flex items-start gap-4">
                  <img src="/people/8.jpg" alt="Kevin Z." class="h-12 w-12 rounded-full object-cover" />
                  <div class="flex-1">
                    <p class="text-gray-700 mb-3">{{ t('home.reviews.review8.text') }}</p>
                    <div class="flex items-center gap-2">
                      <p class="font-semibold text-gray-900">{{ t('home.reviews.review8.name') }}</p>
                      <p class="text-sm text-gray-500">{{ t('home.reviews.review8.role') }}</p>
                    </div>
                  </div>
                </div>
              </div>
              <div class="flex-shrink-0 w-80 rounded-2xl border border-gray-100 bg-white p-6 shadow-sm transition-all duration-300 hover:shadow-lg">
                <div class="flex items-start gap-4">
                  <img src="/people/9.jpg" alt="Wu" class="h-12 w-12 rounded-full object-cover" />
                  <div class="flex-1">
                    <p class="text-gray-700 mb-3">{{ t('home.reviews.review9.text') }}</p>
                    <div class="flex items-center gap-2">
                      <p class="font-semibold text-gray-900">{{ t('home.reviews.review9.name') }}</p>
                      <p class="text-sm text-gray-500">{{ t('home.reviews.review9.role') }}</p>
                    </div>
                  </div>
                </div>
              </div>
              <div class="flex-shrink-0 w-80 rounded-2xl border border-gray-100 bg-white p-6 shadow-sm transition-all duration-300 hover:shadow-lg">
                <div class="flex items-start gap-4">
                  <img src="/people/10.jpg" alt="Zhang" class="h-12 w-12 rounded-full object-cover" />
                  <div class="flex-1">
                    <p class="text-gray-700 mb-3">{{ t('home.reviews.review10.text') }}</p>
                    <div class="flex items-center gap-2">
                      <p class="font-semibold text-gray-900">{{ t('home.reviews.review10.name') }}</p>
                      <p class="text-sm text-gray-500">{{ t('home.reviews.review10.role') }}</p>
                    </div>
                  </div>
                </div>
              </div>
              <div class="flex-shrink-0 w-80 rounded-2xl border border-gray-100 bg-white p-6 shadow-sm transition-all duration-300 hover:shadow-lg">
                <div class="flex items-start gap-4">
                  <img src="/people/11.jpg" alt="Wang" class="h-12 w-12 rounded-full object-cover" />
                  <div class="flex-1">
                    <p class="text-gray-700 mb-3">{{ t('home.reviews.review11.text') }}</p>
                    <div class="flex items-center gap-2">
                      <p class="font-semibold text-gray-900">{{ t('home.reviews.review11.name') }}</p>
                      <p class="text-sm text-gray-500">{{ t('home.reviews.review11.role') }}</p>
                    </div>
                  </div>
                </div>
              </div>
              <div class="flex-shrink-0 w-80 rounded-2xl border border-gray-100 bg-white p-6 shadow-sm transition-all duration-300 hover:shadow-lg">
                <div class="flex items-start gap-4">
                  <img src="/people/12.jpg" alt="Liu" class="h-12 w-12 rounded-full object-cover" />
                  <div class="flex-1">
                    <p class="text-gray-700 mb-3">{{ t('home.reviews.review12.text') }}</p>
                    <div class="flex items-center gap-2">
                      <p class="font-semibold text-gray-900">{{ t('home.reviews.review12.name') }}</p>
                      <p class="text-sm text-gray-500">{{ t('home.reviews.review12.role') }}</p>
                    </div>
                  </div>
                </div>
              </div>
              <div class="flex-shrink-0 w-80 rounded-2xl border border-gray-100 bg-white p-6 shadow-sm transition-all duration-300 hover:shadow-lg">
                <div class="flex items-start gap-4">
                  <img src="/people/13.jpg" alt="Yang" class="h-12 w-12 rounded-full object-cover" />
                  <div class="flex-1">
                    <p class="text-gray-700 mb-3">{{ t('home.reviews.review13.text') }}</p>
                    <div class="flex items-center gap-2">
                      <p class="font-semibold text-gray-900">{{ t('home.reviews.review13.name') }}</p>
                      <p class="text-sm text-gray-500">{{ t('home.reviews.review13.role') }}</p>
                    </div>
                  </div>
                </div>
              </div>
              <div class="flex-shrink-0 w-80 rounded-2xl border border-gray-100 bg-white p-6 shadow-sm transition-all duration-300 hover:shadow-lg">
                <div class="flex items-start gap-4">
                  <img src="/people/14.jpg" alt="Huang" class="h-12 w-12 rounded-full object-cover" />
                  <div class="flex-1">
                    <p class="text-gray-700 mb-3">{{ t('home.reviews.review14.text') }}</p>
                    <div class="flex items-center gap-2">
                      <p class="font-semibold text-gray-900">{{ t('home.reviews.review14.name') }}</p>
                      <p class="text-sm text-gray-500">{{ t('home.reviews.review14.role') }}</p>
                    </div>
                  </div>
                </div>
              </div>
              <!-- Duplicate set for seamless scroll (8-14) -->
              <div class="flex-shrink-0 w-80 rounded-2xl border border-gray-100 bg-white p-6 shadow-sm transition-all duration-300 hover:shadow-lg">
                <div class="flex items-start gap-4">
                  <img src="/people/8.jpg" alt="Kevin Z." class="h-12 w-12 rounded-full object-cover" />
                  <div class="flex-1">
                    <p class="text-gray-700 mb-3">{{ t('home.reviews.review8.text') }}</p>
                    <div class="flex items-center gap-2">
                      <p class="font-semibold text-gray-900">{{ t('home.reviews.review8.name') }}</p>
                      <p class="text-sm text-gray-500">{{ t('home.reviews.review8.role') }}</p>
                    </div>
                  </div>
                </div>
              </div>
              <div class="flex-shrink-0 w-80 rounded-2xl border border-gray-100 bg-white p-6 shadow-sm transition-all duration-300 hover:shadow-lg">
                <div class="flex items-start gap-4">
                  <img src="/people/9.jpg" alt="Wu" class="h-12 w-12 rounded-full object-cover" />
                  <div class="flex-1">
                    <p class="text-gray-700 mb-3">{{ t('home.reviews.review9.text') }}</p>
                    <div class="flex items-center gap-2">
                      <p class="font-semibold text-gray-900">{{ t('home.reviews.review9.name') }}</p>
                      <p class="text-sm text-gray-500">{{ t('home.reviews.review9.role') }}</p>
                    </div>
                  </div>
                </div>
              </div>
              <div class="flex-shrink-0 w-80 rounded-2xl border border-gray-100 bg-white p-6 shadow-sm transition-all duration-300 hover:shadow-lg">
                <div class="flex items-start gap-4">
                  <img src="/people/10.jpg" alt="Zhang" class="h-12 w-12 rounded-full object-cover" />
                  <div class="flex-1">
                    <p class="text-gray-700 mb-3">{{ t('home.reviews.review10.text') }}</p>
                    <div class="flex items-center gap-2">
                      <p class="font-semibold text-gray-900">{{ t('home.reviews.review10.name') }}</p>
                      <p class="text-sm text-gray-500">{{ t('home.reviews.review10.role') }}</p>
                    </div>
                  </div>
                </div>
              </div>
              <div class="flex-shrink-0 w-80 rounded-2xl border border-gray-100 bg-white p-6 shadow-sm transition-all duration-300 hover:shadow-lg">
                <div class="flex items-start gap-4">
                  <img src="/people/11.jpg" alt="Wang" class="h-12 w-12 rounded-full object-cover" />
                  <div class="flex-1">
                    <p class="text-gray-700 mb-3">{{ t('home.reviews.review11.text') }}</p>
                    <div class="flex items-center gap-2">
                      <p class="font-semibold text-gray-900">{{ t('home.reviews.review11.name') }}</p>
                      <p class="text-sm text-gray-500">{{ t('home.reviews.review11.role') }}</p>
                    </div>
                  </div>
                </div>
              </div>
              <div class="flex-shrink-0 w-80 rounded-2xl border border-gray-100 bg-white p-6 shadow-sm transition-all duration-300 hover:shadow-lg">
                <div class="flex items-start gap-4">
                  <img src="/people/12.jpg" alt="Liu" class="h-12 w-12 rounded-full object-cover" />
                  <div class="flex-1">
                    <p class="text-gray-700 mb-3">{{ t('home.reviews.review12.text') }}</p>
                    <div class="flex items-center gap-2">
                      <p class="font-semibold text-gray-900">{{ t('home.reviews.review12.name') }}</p>
                      <p class="text-sm text-gray-500">{{ t('home.reviews.review12.role') }}</p>
                    </div>
                  </div>
                </div>
              </div>
              <div class="flex-shrink-0 w-80 rounded-2xl border border-gray-100 bg-white p-6 shadow-sm transition-all duration-300 hover:shadow-lg">
                <div class="flex items-start gap-4">
                  <img src="/people/13.jpg" alt="Yang" class="h-12 w-12 rounded-full object-cover" />
                  <div class="flex-1">
                    <p class="text-gray-700 mb-3">{{ t('home.reviews.review13.text') }}</p>
                    <div class="flex items-center gap-2">
                      <p class="font-semibold text-gray-900">{{ t('home.reviews.review13.name') }}</p>
                      <p class="text-sm text-gray-500">{{ t('home.reviews.review13.role') }}</p>
                    </div>
                  </div>
                </div>
              </div>
              <div class="flex-shrink-0 w-80 rounded-2xl border border-gray-100 bg-white p-6 shadow-sm transition-all duration-300 hover:shadow-lg">
                <div class="flex items-start gap-4">
                  <img src="/people/14.jpg" alt="Huang" class="h-12 w-12 rounded-full object-cover" />
                  <div class="flex-1">
                    <p class="text-gray-700 mb-3">{{ t('home.reviews.review14.text') }}</p>
                    <div class="flex items-center gap-2">
                      <p class="font-semibold text-gray-900">{{ t('home.reviews.review14.name') }}</p>
                      <p class="text-sm text-gray-500">{{ t('home.reviews.review14.role') }}</p>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </main>

    <!-- Footer -->
    <footer class="relative z-10 border-t border-gray-100 px-6 py-8">
      <div class="mx-auto max-w-7xl">
        <div class="flex flex-col items-center justify-center gap-4 text-center text-sm text-gray-500 sm:flex-row sm:text-left">
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
            <span class="text-gray-400">{{ contactInfo }}</span>
          </div>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { setLocale } from '@/i18n'
import { useAppStore } from '@/stores'
import type { CustomMenuItem } from '@/types'
import Icon from '@/components/icons/Icon.vue'
import LogoSvg from '@/assets/icons/logo.svg'

const { t } = useI18n()

const router = useRouter()
const appStore = useAppStore()

const isDark = ref(false)
const currentYear = computed(() => new Date().getFullYear())
const currentLang = ref<'zh' | 'en'>((localStorage.getItem('sub2api_locale') as 'zh' | 'en') || 'en')

// Get public settings
const contactInfo = computed(() => appStore.contactInfo)
const customMenuItems = computed<CustomMenuItem[]>(() => {
  const settings = appStore.cachedPublicSettings
  if (!settings || !settings.custom_menu_items) return []
  // Filter only user-visible menu items
  return settings.custom_menu_items.filter(item => item.visibility === 'user')
})

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

function toggleLanguage() {
  const newLang = currentLang.value === 'zh' ? 'en' : 'zh'
  currentLang.value = newLang
  setLocale(newLang)
}

function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (savedTheme === 'dark' || (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
}

onMounted(async () => {
  initTheme()
  // Fetch public settings to get contact info and custom menu items
  await appStore.fetchPublicSettings()
})
</script>

<style scoped>
.terminal-container {
  position: relative;
}

.terminal-window {
  width: 440px;
  background: linear-gradient(145deg, #1a1a2e 0%, #16213e 100%);
  border-radius: 16px;
  box-shadow:
    0 30px 60px -15px rgba(0, 0, 0, 0.4),
    0 0 0 1px rgba(255, 255, 255, 0.05);
  overflow: hidden;
}

.terminal-header {
  display: flex;
  align-items: center;
  padding: 10px 16px;
  background: rgba(26, 26, 46, 0.9);
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.terminal-buttons {
  display: flex;
  gap: 6px;
}

.terminal-buttons span {
  width: 12px;
  height: 12px;
  border-radius: 50%;
}

.btn-close {
  background: #ff5f56;
}

.btn-minimize {
  background: #ffbd2e;
}

.btn-maximize {
  background: #27ca40;
}

.terminal-body {
  padding: 20px;
  font-family: 'JetBrains Mono', 'Fira Code', ui-monospace, monospace;
  font-size: 13px;
  line-height: 1.8;
}

.code-line {
  display: flex;
  align-items: center;
  gap: 8px;
  opacity: 0;
  animation: line-appear 0.4s ease forwards;
  flex-wrap: wrap;
}

.line-1 { animation-delay: 0.3s; }
.line-2 { animation-delay: 0.8s; }
.line-3 { animation-delay: 1.3s; }
.line-4 { animation-delay: 1.8s; }
.line-5 { animation-delay: 2.3s; }
.line-6 { animation-delay: 2.8s; }
.line-7 { animation-delay: 3.3s; }

@keyframes line-appear {
  from {
    opacity: 0;
    transform: translateY(4px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.code-prompt {
  color: #3b82f6;
  font-weight: 600;
}

.code-flag {
  color: #8b5cf6;
}

.code-url {
  color: #06b6d4;
}

.code-header {
  color: #9ca3af;
}

.code-token {
  color: #10b981;
}

.code-brace {
  color: #fbbf24;
}

.code-indent {
  width: 24px;
}

.code-key {
  color: #f472b6;
}

.code-string {
  color: #a3e635;
}

.code-price {
  color: #22c55e;
  background: rgba(34, 197, 94, 0.15);
  padding: 2px 8px;
  border-radius: 4px;
  font-weight: 600;
  font-size: 11px;
  margin-left: 8px;
}

@media (max-width: 768px) {
  .terminal-window {
    width: 100%;
    max-width: 400px;
  }
}

/* Testimonial scroll animation - Row 1 (scroll left) */
.testimonial-track-1 {
  display: flex;
  animation: scroll-left 30s linear infinite;
}
.testimonial-track-1:hover {
  animation-play-state: paused;
}
@keyframes scroll-left {
  0% {
    transform: translateX(0);
  }
  100% {
    transform: translateX(-50%);
  }
}

/* Testimonial scroll animation - Row 2 (scroll right) */
.testimonial-track-2 {
  display: flex;
  animation: scroll-right 30s linear infinite;
}
.testimonial-track-2:hover {
  animation-play-state: paused;
}
@keyframes scroll-right {
  0% {
    transform: translateX(-50%);
  }
  100% {
    transform: translateX(0);
  }
}
</style>