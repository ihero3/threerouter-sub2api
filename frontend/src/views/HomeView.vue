<template>
  <div
    class="relative flex min-h-screen flex-col overflow-hidden bg-gradient-to-br from-slate-50ia-bbblu5-50/300/30cyan-505050"
  >
    <!-- Background Decorations -->
    <div class="pointer-events-none absolute inset-0 overflow-hidden">
      <div
        class="absolute righth600pxrn-0ull0bg-gradie0t-to-g-bradientrto-brgradient-to-br mrom-blue-500/20 to-cyan-l00e-500/20 to-cyan-l00e-500/20 to-cyan-400/20 blur-3xl"
      ></div>
      <div
        class="absolute -bottomm40fleftx-0px] 0o0nded-fu0l bg-gradieg-nradieg-nradienttto-br from-emerald-4o0/1b to-teal-500om-emerald-4o0/1b to-teal-500om-emerald-400/15 to-teal-500/15 blur-3xl"
      ></div>
      <div
        class="absolute inset-0 bg-[linear-gradient(rgba(599030,2262646444)_1px,transparent_1px),linear-gradient(90deg,rgb59513012162,624,404)_1px,transparent_1px)] bg-[s64646646464px]"
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
          <span class="text-[16px] font-semibold text-[#021b4a]" style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;">Three Router</span>
        </div>

        <!-- Nav Actions -->
        <div class="flex items-center gap-3">
          <!-- Limited Time Banner -->
          <div class="hidden items-center gap-2 rounded-full bg-orange-100 px-4 py-2 sm:flex">
            <svg class="h-4 w-4 text-orange-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <span class="text-sm font-medium text-orange-700">🎁LIMITED TIME: $10 Free Credits (≈ 330M DeepSeek Tokens)</span>
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
          <button @click="router.push('/login')" class="rounded-lg bg-[#0757b8] px-5 py-2 text-sm font-semibold text-white shadow-lg shadow-blue-900/10 transition-colors hover:bg-[#064ea8] cursor-pointer">
            {{ currentLang === 'zh' ? '登录' : 'Login' }}
          </button>
        </div>
      </nav>
    </header>

    <!-- Main Content -->
    <main class="relative z-10 flex-1 px-6  pt-16">
      <div class="mx-auto flex max-w-7xl flex-col">
        <!-- Hero Section -->
        <div class="mb-16 flex flex-col items-center justify-between gap-12 lg:flex-row lg:gap-16">
          <!-- Left: Text Content -->
          <div class="flex-1 text-center lg:text-left">
            <!-- Slogan -->
            <div class="mb-4 inline-flex items-center gap-2 rounded-full bg-gradient-to-r from-[#20f5b4]/20 to-[#0757b8]/20 px-5 py-2.5">
              <span class="h-2.5 w-2.5 animate-pulse rounded-full bg-[#20f5b4]"></span>
              <span class="text-base font-semibold text-[#0757b8]">{{ t('home.hero.slogan') }}</span>
            </div>
            <h1
              class="mb-5 text-5xl font-bold text-gray-900 md:text-6xl lg:text-7xl"
            >
              {{ t('home.hero.title') }}
            </h1>
            <p class="mb-8 text-xl text-gray-600 md:text-2xl">
              {{ t('home.hero.subtitle') }}
            </p>

            <!-- Feature Tags -->
            <div class="mb-10 flex flex-wrap items-center justify-center gap-3 lg:justify-start">
              <span class="inline-flex items-center gap-2 rounded-full bg-blue-100 px-4 py-2 text-base font-medium text-blue-700">
                <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17.657 16.657L13.414 20.9a1.998 1.998 0 01-2.827 0l-4.244-4.243a8 8 0 1111.314 0z" />
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 11a3 3 0 11-6 0 3 3 0 016 0z" />
                </svg>
                {{ t('home.hero.tags.deployment') }}
              </span>
              <span class="inline-flex items-center gap-2 rounded-full bg-green-100 px-4 py-2 text-base font-medium text-green-700">
                <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 7h6m0 10v-3m-3 3h.01M9 17h.01M9 14h.01M12 14h.01M15 11h.01M12 11h.01M9 11h.01M7 21h10a2 2 0 002-2V5a2 2 0 00-2-2H7a2 2 0 00-2 2v14a2 2 0 002 2z" />
                </svg>
                {{ t('home.hero.tags.freeTokens') }}
              </span>
            </div>

            <!-- CTA Button -->
            <button @click="router.push('/login')" class="inline-flex items-center gap-2.5 rounded-full bg-gradient-to-r from-[#0757b8] to-[#20f5b4] px-10 py-4 text-lg font-semibold text-white shadow-xl shadow-[#0757b8]/20 transition-all hover:scale-105 hover:shadow-2xl cursor-pointer">
              {{ t('home.hero.cta') }}
              <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 8l4 4m0 0l-4 4m4-4H3" />
              </svg>
            </button>

            <!-- Code Hint -->
            <p class="mt-4 text-sm text-gray-400">{{ t('home.hero.codeHint') }}</p>

          </div>

          <!-- Right: Code Example -->
          <div class="justify-center lg:justify-end">
            <div class="terminal-container">
              <div class="terminal-window">
                <div class="terminal-header">
                  <div class="terminal-buttons">
                    <span class="btn-close"></span>
                    <span class="btn-minimize"></span>
                    <span class="btn-maximize"></span>
                  </div>
                  <span class="terminal-title">main.py</span>
                </div>
                <div class="terminal-body code-example-body">
                  <div class="code-line line-1">
                    <span class="code-comment"># Before</span>
                  </div>
                  <div class="code-line line-2">
                    <span class="code-key">client</span>
                    <span class="code-operator">=</span>
                    <span class="code-class">OpenAI</span>
                    <span class="code-paren">(</span><span class="code-key">api_key</span><span class="code-operator">=</span><span class="code-string">"sk-..."</span><span class="code-paren">)</span>
                  </div>
                  <div class="code-line line-3 code-blank"></div>
                  <div class="code-line line-4">
                    <span class="code-comment"># After — same SDK, same call signature</span>
                  </div>
                  <div class="code-line line-5">
                    <span class="code-key">client</span>
                    <span class="code-operator">=</span>
                    <span class="code-class">OpenAI</span>
                    <span class="code-paren">(</span>
                  </div>
                  <div class="code-line line-6">
                    <span class="code-indent-s"></span>
                    <span class="code-key">base_url</span><span class="code-operator">=</span><span class="code-string">"https://threerouter.com/v1"</span><span class="code-paren">,</span>
                  </div>
                  <div class="code-line line-7">
                    <span class="code-indent-s"></span>
                    <span class="code-key">api_key</span><span class="code-operator">=</span><span class="code-string">"tr-..."</span>
                  </div>
                  <div class="code-line line-8">
                    <span class="code-paren">)</span>
                  </div>
                  <div class="code-line line-9 code-blank"></div>
                  <div class="code-line line-10">
                    <span class="code-comment"># Everything else stays the same</span>
                  </div>
                  <div class="code-line line-11">
                    <span class="code-key">response</span>
                    <span class="code-operator">=</span>
                    <span class="code-key">client</span><span class="code-paren">.</span><span class="code-func">chat</span><span class="code-paren">.</span><span class="code-func">completions</span><span class="code-paren">.</span><span class="code-func">create</span><span class="code-paren">(</span>
                  </div>
                  <div class="code-line line-12">
                    <span class="code-indent-s"></span><span class="code-key">model</span><span class="code-operator">=</span><span class="code-string">"deepseek-v4-pro"</span><span class="code-paren">,</span>
                  </div>
                  <div class="code-line line-13">
                    <span class="code-indent-s"></span><span class="code-key">messages</span><span class="code-operator">=</span><span class="code-paren">[{</span><span class="code-string">"role"</span><span class="code-paren">:</span>
                    <span class="code-string">"user"</span><span class="code-paren">,</span>
                    <span class="code-string">"content"</span><span class="code-paren">:</span>
                    <span class="code-string">"Hello"</span><span class="code-paren">}]</span>
                  </div>
                  <div class="code-line line-14">
                    <span class="code-paren">)</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Supported AI Models -->
        <div class=" mb-16 rounded-2xl border border-gray-100 bg-white p-8 shadow-sm">
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
            <span class="flex items-center gap-2 rounded-xl border border-emerald-100 bg-white px-4 py-2.5">
              <span class="flex h-6 w-6 items-center justify-center rounded-full bg-emerald-500 text-xs font-bold text-white">G</span>
              <span class="text-sm font-medium text-gray-700">gpt-image-2</span>
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
        <!-- Core Advantages -->
        <section class="mb-16 overflow-hidden rounded-[2rem] border border-[#20f5b4]/20 bg-[#020b1f] p-10 text-white shadow-2xl shadow-[#0757b8]/20 md:p-12">
          <div class="mb-10 flex flex-col justify-between gap-6 md:flex-row md:items-end">
            <div>
              <p class="text-sm font-bold uppercase tracking-[0.24em] text-[#20f5b4]">{{ t('home.easyrouterAdvantages.eyebrow') }}</p>
              <h2 class="mt-4 text-4xl font-black tracking-tight md:text-5xl">{{ t('home.easyrouterAdvantages.title') }}</h2>
            </div>
            <p class="max-w-xl text-base leading-relaxed text-slate-300 md:text-right">{{ t('home.easyrouterAdvantages.subtitle') }}</p>
          </div>

          <div class="grid gap-5 md:grid-cols-2 lg:grid-cols-5">
            <div class="rounded-3xl border border-[#20f5b4]/20 bg-[#20f5b4]/[0.08] p-6 backdrop-blur transition-all hover:-translate-y-1 hover:bg-[#20f5b4]/[0.12] lg:col-span-1">
              <div class="mb-5 flex h-14 w-14 items-center justify-center rounded-2xl bg-[#20f5b4]/15 text-[#20f5b4]">
                <svg class="h-6 w-6" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z" /></svg>
              </div>
              <h3 class="text-lg font-bold">{{ t('home.easyrouterAdvantages.standardApi.title') }}</h3>
              <p class="mt-2 text-base leading-relaxed text-slate-300">{{ t('home.easyrouterAdvantages.standardApi.desc') }}</p>
            </div>
            <div class="rounded-3xl border border-white/10 bg-white/[0.06] p-6 backdrop-blur transition-all hover:-translate-y-1 hover:bg-white/[0.09] lg:col-span-1">
              <div class="mb-5 flex h-14 w-14 items-center justify-center rounded-2xl bg-emerald-400/15 text-emerald-300">
                <svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
              </div>
              <h3 class="text-lg font-bold">{{ t('home.easyrouterAdvantages.reliable.title') }}</h3>
              <p class="mt-2 text-base leading-relaxed text-slate-300">{{ t('home.easyrouterAdvantages.reliable.desc') }}</p>
            </div>
            <div class="rounded-3xl border border-white/10 bg-white/[0.06] p-6 backdrop-blur transition-all hover:-translate-y-1 hover:bg-white/[0.09] lg:col-span-1">
              <div class="mb-5 flex h-14 w-14 items-center justify-center rounded-2xl bg-cyan-400/15 text-cyan-300">
                <svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M13 10V3L4 14h7v7l9-11h-7z" /></svg>
              </div>
              <h3 class="text-lg font-bold">{{ t('home.easyrouterAdvantages.ultraFast.title') }}</h3>
              <p class="mt-2 text-base leading-relaxed text-slate-300">{{ t('home.easyrouterAdvantages.ultraFast.desc') }}</p>
            </div>
            <div class="rounded-3xl border border-white/10 bg-white/[0.06] p-6 backdrop-blur transition-all hover:-translate-y-1 hover:bg-white/[0.09] lg:col-span-1">
              <div class="mb-5 flex h-14 w-14 items-center justify-center rounded-2xl bg-[#20f5b4]/15 text-[#20f5b4]">
                <svg class="h-6 w-6" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8V7m0 10v-1m9-4a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
              </div>
              <h3 class="text-lg font-bold">{{ t('home.features.balanceQuota') }}</h3>
              <p class="mt-2 text-base leading-relaxed text-slate-300">{{ t('home.features.balanceQuotaDesc') }}</p>
            </div>
            <div class="rounded-3xl border border-emerald-400/30 bg-emerald-400/10 p-6 backdrop-blur transition-all hover:-translate-y-1 hover:bg-emerald-400/15 md:col-span-2 lg:col-span-1">
              <div class="mb-5 flex h-14 w-14 items-center justify-center rounded-2xl bg-emerald-400/20 text-emerald-200">
                <svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8V7m0 10v-1m9-4a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
              </div>
              <h3 class="text-lg font-bold text-emerald-100">{{ t('home.easyrouterAdvantages.cheap.title') }}</h3>
              <p class="mt-2 text-base leading-relaxed text-emerald-100/80">{{ t('home.easyrouterAdvantages.cheap.desc') }}</p>
            </div>
          </div>
        </section>
  <!-- Pricing & Direct Procurement Section -->
        <div class="mb-16">
          <!-- Price Advantage Section - Full Width Header -->
          <div class="mb-8 overflow-hidden rounded-3xl border border-[#20f5b4]/30 bg-white/95 shadow-2xl shadow-[#0757b8]/10 backdrop-blur dark:border-emerald-800/50 dark:bg-gray-900/80">
            <div class="relative px-8 py-8 sm:px-10">
              <div class="absolute inset-0 bg-[radial-gradient(circle_at_top_left,rgba(32,245,180,0.2),transparent_36%),linear-gradient(135deg,rgba(239,252,255,0.96),rgba(255,255,255,0.98))] dark:bg-[radial-gradient(circle_at_top_left,rgba(16,185,129,0.2),transparent_34%),linear-gradient(135deg,rgba(6,78,59,0.28),rgba(17,24,39,0.96))]"></div>
              <div class="relative grid gap-8 lg:grid-cols-[0.86fr_1.14fr] lg:items-center">
                <div>
                  <p class="text-sm font-semibold uppercase tracking-[0.22em] text-[#0757b8] dark:text-emerald-400">
                    US Local Deployment + 97% Cost Cut
                  </p>
                  <h3 class="mt-3 text-3xl font-black leading-tight text-gray-950 dark:text-white md:text-4xl">
                    {{ t('home.hero.priceAdvantage') }}
                  </h3>
                  <p class="mt-3 text-base font-medium text-emerald-700 dark:text-emerald-300">
                    {{ t('home.hero.priceReasons') }}
                  </p>
                  <div class="mt-6 grid grid-cols-2 gap-3">
                    <div class="rounded-2xl border border-[#0757b8]/10 bg-white/75 p-4">
                      <p class="text-xs font-bold uppercase tracking-[0.16em] text-slate-400">{{ t('home.hero.costChart.usRoute') }}</p>
                      <p class="mt-2 text-2xl font-black text-[#021b4a]">US-East / US-West</p>
                    </div>
                    <div class="rounded-2xl bg-[#021b4a] p-4 text-white shadow-lg shadow-[#0757b8]/20">
                      <p class="text-xs font-bold uppercase tracking-[0.16em] text-emerald-200">{{ t('home.hero.costChart.maxSaving') }}</p>
                      <p class="mt-2 text-4xl font-black leading-none text-[#20f5b4]">97%</p>
                    </div>
                  </div>
                </div>

                <div class="rounded-[1.75rem] border border-slate-200/70 bg-slate-950 p-5 text-white shadow-2xl shadow-[#021b4a]/25">
                  <div class="mb-5 flex items-start justify-between gap-4">
                    <div>
                      <p class="text-xs font-black uppercase tracking-[0.22em] text-[#20f5b4]">{{ t('home.hero.costChart.eyebrow') }}</p>
                      <h4 class="mt-2 text-xl font-black">{{ t('home.hero.costChart.title') }}</h4>
                    </div>
                    <span class="rounded-full bg-[#20f5b4]/15 px-3 py-1 text-xs font-bold text-[#20f5b4]">$/M input</span>
                  </div>
                  <div class="flex h-[340px] gap-3 md:h-[400px] md:gap-4">
                    <div class="relative flex flex-1 items-end gap-2 rounded-2xl border border-white/10 bg-white/5 p-3 md:gap-3 md:p-4">
                      <div v-for="item in costComparisonBars" :key="item.name" class="flex h-full flex-1 flex-col items-center justify-end gap-2 text-center">
                        <div class="mb-1 text-xs font-black leading-tight">
                          <span class="block text-[#20f5b4]">${{ item.price }}</span>
                          <span v-if="item.saving" class="mt-0.5 block text-[10px] uppercase tracking-wide text-emerald-200">-{{ item.saving }}%</span>
                        </div>
                        <div
                          class="cost-bar w-full rounded-t-xl transition-all duration-700 hover:brightness-110"
                          :class="item.isOpenSource ? 'bg-gradient-to-t from-[#20f5b4] to-cyan-300 shadow-[0_0_18px_rgba(32,245,180,0.35)]' : 'bg-gradient-to-t from-slate-500 to-slate-300'"
                          :style="{ height: item.height }"
                        ></div>
                        <div class="mt-1 w-full">
                          <p class="truncate text-[10px] font-bold leading-tight text-slate-200 md:text-xs" :title="item.name">{{ item.name }}</p>
                        </div>
                      </div>
                    </div>
                  </div>
                  <p class="mt-5 rounded-2xl border border-[#20f5b4]/20 bg-[#20f5b4]/10 p-4 text-sm font-semibold leading-relaxed text-emerald-100">
                    {{ t('home.hero.costChart.note') }}
                  </p>
                </div>
              </div>
            </div>
          </div>

          <!-- Two Column Layout -->
          <div class="grid gap-8 lg:grid-cols-[1.2fr_0.8fr]">
            <!-- Left: Pricing Table -->
            <div class="rounded-3xl border border-gray-100 bg-white p-8 shadow-xl shadow-gray-100">
              <div class="mb-5 flex items-center gap-3">
                <div class="flex h-10 w-10 items-center justify-center rounded-xl bg-emerald-50">
                  <svg class="h-5 w-5 text-emerald-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8V7m0 10v-1m9-4a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                </div>
                <h4 class="text-xl font-bold text-gray-900">{{ t('home.hero.pricingSection.tableHeader.title') }}</h4>
              </div>
              <div class="overflow-hidden rounded-2xl border border-gray-100">
                <table class="w-full text-sm">
                  <thead class="bg-gray-50 text-xs uppercase tracking-wide text-gray-500">
                    <tr>
                      <th class="px-5 py-4 text-left font-bold">{{ t('home.hero.pricingSection.tableHeader.model') }}</th>
                      <th class="px-5 py-4 text-left font-bold">{{ t('home.hero.pricingSection.tableHeader.capability') }}</th>
                      <th class="px-5 py-4 text-right font-bold whitespace-nowrap">{{ t('home.hero.pricingSection.tableHeader.price') }}</th>
                      <th class="px-5 py-4 text-right font-bold whitespace-nowrap">{{ t('home.hero.pricingSection.tableHeader.relative') }}</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-gray-50 bg-white">
                    <tr class="cursor-pointer transition-all duration-200 hover:bg-gray-50 hover:shadow-md hover:shadow-gray-100">
                      <td class="px-5 py-3 font-semibold text-gray-900">{{ t('home.hero.pricingSection.models.gpt55.name') }}</td>
                      <td class="px-5 py-3 leading-relaxed text-gray-500">{{ t('home.hero.pricingSection.models.gpt55.capability') }}</td>
                      <td class="px-5 py-3 text-right font-semibold text-gray-900">$5.00</td>
                      <td class="px-5 py-3 text-right text-gray-400">{{ t('home.hero.pricingSection.baseline') }}</td>
                    </tr>
                    <tr class="cursor-pointer transition-all duration-200 hover:bg-gray-50 hover:shadow-md hover:shadow-gray-100">
                      <td class="px-5 py-3 font-semibold text-gray-900">{{ t('home.hero.pricingSection.models.claudeOpus48.name') }}</td>
                      <td class="px-5 py-3 leading-relaxed text-gray-500">{{ t('home.hero.pricingSection.models.claudeOpus48.capability') }}</td>
                      <td class="px-5 py-3 text-right font-semibold text-gray-900">$5.00</td>
                      <td class="px-5 py-3 text-right text-gray-400">{{ t('home.hero.pricingSection.baseline') }}</td>
                    </tr>
                    <tr class="cursor-pointer transition-all duration-200 hover:bg-emerald-50 hover:shadow-md hover:shadow-emerald-100">
                      <td class="px-5 py-3 font-semibold text-emerald-600">{{ t('home.hero.pricingSection.models.glm51.name') }}</td>
                      <td class="px-5 py-3 leading-relaxed text-gray-500">{{ t('home.hero.pricingSection.models.glm51.capability') }}</td>
                      <td class="px-5 py-3 text-right font-bold text-emerald-600">$1.40</td>
                      <td class="px-5 py-3 text-right"><span class="rounded-full bg-emerald-100 px-2.5 py-1 text-xs font-semibold text-emerald-600">{{ t('home.hero.pricingSection.discount', { percent: 72 }) }}</span></td>
                    </tr>
                    <tr class="cursor-pointer transition-all duration-200 hover:bg-emerald-50 hover:shadow-md hover:shadow-emerald-100">
                      <td class="px-5 py-3 font-semibold text-emerald-600">{{ t('home.hero.pricingSection.models.qwen37Max.name') }}</td>
                      <td class="px-5 py-3 leading-relaxed text-gray-500">{{ t('home.hero.pricingSection.models.qwen37Max.capability') }}</td>
                      <td class="px-5 py-3 text-right font-bold text-emerald-600">$2.50</td>
                      <td class="px-5 py-3 text-right"><span class="rounded-full bg-emerald-100 px-2.5 py-1 text-xs font-semibold text-emerald-600">{{ t('home.hero.pricingSection.discount', { percent: 50 }) }}</span></td>
                    </tr>
                    <tr class="cursor-pointer transition-all duration-200 hover:bg-emerald-50 hover:shadow-md hover:shadow-emerald-100">
                      <td class="px-5 py-3 font-semibold text-emerald-600">{{ t('home.hero.pricingSection.models.kimiK26.name') }}</td>
                      <td class="px-5 py-3 leading-relaxed text-gray-500">{{ t('home.hero.pricingSection.models.kimiK26.capability') }}</td>
                      <td class="px-5 py-3 text-right font-bold text-emerald-600">$0.95</td>
                      <td class="px-5 py-3 text-right"><span class="rounded-full bg-emerald-100 px-2.5 py-1 text-xs font-semibold text-emerald-600">{{ t('home.hero.pricingSection.discount', { percent: 81 }) }}</span></td>
                    </tr>
                    <tr class="cursor-pointer transition-all duration-200 hover:bg-emerald-50 hover:shadow-md hover:shadow-emerald-100">
                      <td class="px-5 py-3 font-semibold text-emerald-600">{{ t('home.hero.pricingSection.models.minimaxM3.name') }}</td>
                      <td class="px-5 py-3 leading-relaxed text-gray-500">{{ t('home.hero.pricingSection.models.minimaxM3.capability') }}</td>
                      <td class="px-5 py-3 text-right font-bold text-emerald-600">$0.60</td>
                      <td class="px-5 py-3 text-right"><span class="rounded-full bg-emerald-100 px-2.5 py-1 text-xs font-semibold text-emerald-600">{{ t('home.hero.pricingSection.discount', { percent: 88 }) }}</span></td>
                    </tr>
                    <tr class="cursor-pointer transition-all duration-200 hover:bg-emerald-50 hover:shadow-md hover:shadow-emerald-100">
                      <td class="px-5 py-3 font-semibold text-emerald-600">{{ t('home.hero.pricingSection.models.deepseekV4Flash.name') }}</td>
                      <td class="px-5 py-3 leading-relaxed text-gray-500">{{ t('home.hero.pricingSection.models.deepseekV4Flash.capability') }}</td>
                      <td class="px-5 py-3 text-right font-bold text-emerald-600">$0.14</td>
                      <td class="px-5 py-3 text-right"><span class="rounded-full bg-emerald-100 px-2.5 py-1 text-xs font-bold text-emerald-700">{{ t('home.hero.pricingSection.discount', { percent: 97 }) }}</span></td>
                    </tr>
                    <tr class="cursor-pointer transition-all duration-200 hover:bg-emerald-50 hover:shadow-md hover:shadow-emerald-100">
                      <td class="px-5 py-3 font-semibold text-emerald-600">{{ t('home.hero.pricingSection.models.deepseekV4Pro.name') }}</td>
                      <td class="px-5 py-3 leading-relaxed text-gray-500">{{ t('home.hero.pricingSection.models.deepseekV4Pro.capability') }}</td>
                      <td class="px-5 py-3 text-right font-bold text-emerald-600">$0.42</td>
                      <td class="px-5 py-3 text-right"><span class="rounded-full bg-emerald-100 px-2.5 py-1 text-xs font-semibold text-emerald-600">{{ t('home.hero.pricingSection.discount', { percent: 92 }) }}</span></td>
                    </tr>
                    <tr class="cursor-pointer transition-all duration-200 hover:bg-amber-50 hover:shadow-md hover:shadow-amber-100">
                      <td class="px-5 py-3 font-semibold text-amber-600">{{ t('home.hero.pricingSection.models.seedance20.name') }}</td>
                      <td class="px-5 py-3 leading-relaxed text-gray-500">{{ t('home.hero.pricingSection.models.seedance20.capability') }}</td>
                      <td class="px-5 py-3 text-right font-bold text-amber-600">$0.10/s</td>
                      <td class="px-5 py-3 text-right"><span class="rounded-full bg-amber-100 px-2.5 py-1 text-xs font-semibold text-amber-600">Video</span></td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>

            <!-- Right: Direct Procurement -->
            <div class="rounded-3xl border border-[#c7f5e6]/40 bg-gradient-to-br from-[#f0fdf9] via-[#f0f9ff] to-[#f0fdf9] p-8 shadow-xl shadow-[#0757b8]/5">
              <div class="mb-6 flex justify-center">
                <span class="inline-flex items-center gap-2 rounded-full border-2 border-orange-300 bg-orange-50 px-6 py-2 text-xs font-bold uppercase tracking-[0.16em] text-orange-600">
                  <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"/>
                  </svg>
                  {{ t('home.hero.pricingSection.enterpriseBadge') }}
                </span>
              </div>
              <h3 class="mb-4 text-2xl font-serif font-bold leading-tight text-center text-gray-900">
                {{ t('home.hero.pricingSection.directProcurement') }}
              </h3>
              <p class="mb-8 text-base leading-relaxed text-center text-gray-600">
                {{ t('home.hero.pricingSection.directProcurementDesc') }}
              </p>

              <!-- Cloud Providers Grid -->
              <div class="mb-8 grid grid-cols-3 gap-x-6 gap-y-6">
                <div class="flex flex-col items-center gap-2">
                  <div class="text-xl font-bold tracking-tight text-amber-600">AWS</div>
                  <span class="text-xs font-semibold text-gray-500">AWS</span>
                </div>
                <div class="flex flex-col items-center gap-2">
                  <img src="/googlecloud-color.svg" alt="Google Cloud" class="h-11 w-11" />
                  <span class="text-xs font-semibold text-gray-500">Google Cloud</span>
                </div>
                <div class="flex flex-col items-center gap-2">
                  <img src="/azure-color.svg" alt="Azure" class="h-11 w-11" />
                  <span class="text-xs font-semibold text-gray-500">Azure</span>
                </div>
                <div class="flex flex-col items-center gap-2">
                  <img src="/bedrock-color.svg" alt="Amazon Bedrock" class="h-11 w-11" />
                  <span class="text-xs font-semibold text-gray-500">Bedrock</span>
                </div>
                <div class="flex flex-col items-center gap-2">
                  <img src="/vertexai-color.svg" alt="Google Vertex AI" class="h-11 w-11" />
                  <span class="text-xs font-semibold text-gray-500">Vertex AI</span>
                </div>
                <div class="flex flex-col items-center gap-2">
                  <img src="/azureai-color.svg" alt="Azure AI Studio" class="h-11 w-11" />
                  <span class="text-xs font-semibold text-gray-500">Azure AI</span>
                </div>
                <div class="flex flex-col items-center gap-2">
                  <img src="/alibabacloud-color.svg" alt="Alibaba Cloud" class="h-11 w-11" />
                  <span class="text-xs font-semibold text-gray-500">Alibaba</span>
                </div>
                <div class="flex flex-col items-center gap-2">
                  <img src="/volcengine-color.svg" alt="Volcengine" class="h-11 w-11" />
                  <span class="text-xs font-semibold text-gray-500">Volcengine</span>
                </div>
                <div class="flex flex-col items-center gap-2">
                  <img src="/moonshot.svg" alt="Moonshot AI" class="h-11 w-11" />
                  <span class="text-xs font-semibold text-gray-500">Moonshot</span>
                </div>
              </div>

              <!-- Additional Benefits -->
              <div class="rounded-2xl bg-white/80 p-6">
                <h4 class="mb-4 text-lg font-semibold text-blue-600">{{ t('home.hero.pricingSection.benefits.title') }}</h4>
                <ul class="space-y-3 text-base text-gray-600">
                  <li class="flex items-start gap-3">
                    <svg class="mt-0.5 h-5 w-5 flex-shrink-0 text-emerald-500" fill="currentColor" viewBox="0 0 20 20">
                      <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                    </svg>
                    {{ t('home.hero.pricingSection.benefits.compliant') }}
                  </li>
                  <li class="flex items-start gap-3">
                    <svg class="mt-0.5 h-5 w-5 flex-shrink-0 text-emerald-500" fill="currentColor" viewBox="0 0 20 20">
                      <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                    </svg>
                    {{ t('home.hero.pricingSection.benefits.stable') }}
                  </li>
                  <li class="flex items-start gap-3">
                    <svg class="mt-0.5 h-5 w-5 flex-shrink-0 text-emerald-500" fill="currentColor" viewBox="0 0 20 20">
                      <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                    </svg>
                    {{ t('home.hero.pricingSection.benefits.traceable') }}
                  </li>
                  <li class="flex items-start gap-3">
                    <svg class="mt-0.5 h-5 w-5 flex-shrink-0 text-emerald-500" fill="currentColor" viewBox="0 0 20 20">
                      <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                    </svg>
                    {{ t('home.hero.pricingSection.benefits.secure') }}
                  </li>
                </ul>
              </div>
            </div>
          </div>
        </div>


        <!-- CTA Banner -->
        <div class="relative overflow-hidden rounded-[2rem] bg-gradient-to-r from-[#021b4a] via-[#0757b8] to-[#20f5b4] p-10 shadow-2xl shadow-[#0757b8]/20 md:p-14">
          <div class="absolute -right-20 -top-20 h-60 w-60 rounded-full bg-white/10 blur-3xl"></div>
          <div class="absolute -bottom-20 -left-20 h-60 w-60 rounded-full bg-white/10 blur-3xl"></div>
          <div class="relative z-10 flex flex-col items-center justify-between gap-7 text-center md:flex-row">
            <div class="flex items-center gap-4">
              <svg class="h-10 w-10 text-white/90" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11.049 2.927c.3-.921 1.603-.921 1.902 0l1.519 4.674a1 1 0 00.95.69h4.915c.969 0 1.371 1.24.588 1.81l-3.976 2.888a1 1 0 00-.363 1.118l1.518 4.674c.3.922-.755 1.688-1.538 1.118l-3.976-2.888a1 1 0 00-1.176 0l-3.976 2.888c-.783.57-1.838-.197-1.538-1.118l1.518-4.674a1 1 0 00-.363-1.118l-3.976-2.888c-.784-.57-.38-1.81.588-1.81h4.914a1 1 0 00.951-.69l1.519-4.674z" />
              </svg>
              <div class="text-white">
                <p class="text-xl font-semibold">{{ t('home.cta.title') }}</p>
                <p class="text-base opacity-80">{{ t('home.cta.description') }}</p>
              </div>
            </div>
            <button @click="router.push('/login')" class="rounded-full bg-white px-10 py-4 text-lg font-semibold text-blue-600 shadow-lg transition-all hover:scale-105 hover:shadow-xl cursor-pointer">
              {{ t('home.cta.button') }}
            </button>
          </div>
        </div>
        <div class="brand-reviews relative mt-16 mb-16 overflow-hidden rounded-[2rem] border border-[#20f5b4]/20 bg-[#020b1f] px-6 py-10 shadow-2xl shadow-[#0757b8]/20 md:px-8 md:py-12">
          <div class="pointer-events-none absolute -right-24 -top-24 h-72 w-72 rounded-full bg-[#20f5b4]/20 blur-3xl"></div>
          <div class="pointer-events-none absolute -left-24 bottom-0 h-72 w-72 rounded-full bg-[#0757b8]/30 blur-3xl"></div>
          <div class="relative text-center mb-12">
            <p class="mb-3 text-xs font-bold uppercase tracking-[0.24em] text-[#20f5b4]">{{ t('home.reviews.title') }}</p>
            <h2 class="text-3xl font-black tracking-tight text-white mb-4 md:text-4xl">{{ t('home.reviews.title') }}</h2>
            <p class="mx-auto max-w-2xl text-lg leading-relaxed text-blue-100/75">{{ t('home.reviews.subtitle') }}</p>
          </div>
          <div class="relative grid grid-cols-2 gap-4 md:grid-cols-4">
            <div class="flex flex-col items-center rounded-2xl border border-[#20f5b4]/20 bg-white/[0.06] p-6 backdrop-blur transition-all hover:-translate-y-1 hover:bg-white/[0.09]"><div class="mb-2 flex h-10 w-10 items-center justify-center rounded-xl bg-[#20f5b4]/15 text-[#20f5b4]"><svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6"/></svg></div><span class="text-2xl font-bold text-white">{{ t('home.reviews.review1.name') }}</span><span class="text-xs text-slate-400">{{ t('home.reviews.review1.role') }}</span></div>
            <div class="flex flex-col items-center rounded-2xl border border-[#20f5b4]/20 bg-white/[0.06] p-6 backdrop-blur transition-all hover:-translate-y-1 hover:bg-white/[0.09]"><div class="mb-2 flex h-10 w-10 items-center justify-center rounded-xl bg-cyan-400/15 text-cyan-300"><svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M13 10V3L4 14h7v7l9-11h-7z"/></svg></div><span class="text-2xl font-bold text-white">{{ t('home.reviews.review2.name') }}</span><span class="text-xs text-slate-400">{{ t('home.reviews.review2.role') }}</span></div>
            <div class="flex flex-col items-center rounded-2xl border border-[#20f5b4]/20 bg-white/[0.06] p-6 backdrop-blur transition-all hover:-translate-y-1 hover:bg-white/[0.09]"><div class="mb-2 flex h-10 w-10 items-center justify-center rounded-xl bg-emerald-400/15 text-emerald-300"><svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/></svg></div><span class="text-2xl font-bold text-white">{{ t('home.reviews.review3.name') }}</span><span class="text-xs text-slate-400">{{ t('home.reviews.review3.role') }}</span></div>
            <div class="flex flex-col items-center rounded-2xl border border-[#20f5b4]/20 bg-white/[0.06] p-6 backdrop-blur transition-all hover:-translate-y-1 hover:bg-white/[0.09]"><div class="mb-2 flex h-10 w-10 items-center justify-center rounded-xl bg-amber-400/15 text-amber-300"><svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/></svg></div><span class="text-2xl font-bold text-white">{{ t('home.reviews.review4.name') }}</span><span class="text-xs text-slate-400">{{ t('home.reviews.review4.role') }}</span></div>
            <div class="flex flex-col items-center rounded-2xl border border-[#20f5b4]/20 bg-white/[0.06] p-6 backdrop-blur transition-all hover:-translate-y-1 hover:bg-white/[0.09]"><div class="mb-2 flex h-10 w-10 items-center justify-center rounded-xl bg-purple-400/15 text-purple-300"><svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M11.049 2.927c.3-.921 1.603-.921 1.902 0l1.519 4.674a1 1 0 00.95.69h4.915c.969 0 1.371 1.24.588 1.81l-3.976 2.888a1 1 0 00-.363 1.118l1.518 4.674c.3.922-.755 1.688-1.538 1.118l-3.976-2.888a1 1 0 00-1.176 0l-3.976 2.888c-.783.57-1.838-.197-1.538-1.118l1.518-4.674a1 1 0 00-.363-1.118l-3.976-2.888c-.784-.57-.38-1.81.588-1.81h4.914a1 1 0 00.951-.69l1.519-4.674z"/></svg></div><span class="text-2xl font-bold text-white">{{ t('home.reviews.review5.name') }}</span><span class="text-xs text-slate-400">{{ t('home.reviews.review5.role') }}</span></div>
            <div class="flex flex-col items-center rounded-2xl border border-[#20f5b4]/20 bg-white/[0.06] p-6 backdrop-blur transition-all hover:-translate-y-1 hover:bg-white/[0.09]"><div class="mb-2 flex h-10 w-10 items-center justify-center rounded-xl bg-rose-400/15 text-rose-300"><svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M14 10h4.764a2 2 0 011.789 2.894l-3.5 7A2 2 0 0115.263 21h-4.017c-.163 0-.326-.02-.485-.06L7 20m7-10V5a2 2 0 00-2-2h-.095c-.5 0-.905.405-.905.905 0 .714-.211 1.412-.608 2.006L7 11v9m7-10h-2M7 20H5a2 2 0 01-2-2v-6a2 2 0 012-2h2.5"/></svg></div><span class="text-2xl font-bold text-white">{{ t('home.reviews.review6.name') }}</span><span class="text-xs text-slate-400">{{ t('home.reviews.review6.role') }}</span></div>
            <div class="flex flex-col items-center rounded-2xl border border-[#20f5b4]/20 bg-white/[0.06] p-6 backdrop-blur transition-all hover:-translate-y-1 hover:bg-white/[0.09]"><div class="mb-2 flex h-10 w-10 items-center justify-center rounded-xl bg-orange-400/15 text-orange-300"><svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M11.049 2.927c.3-.921 1.603-.921 1.902 0l1.519 4.674a1 1 0 00.95.69h4.915c.969 0 1.371 1.24.588 1.81l-3.976 2.888a1 1 0 00-.363 1.118l1.518 4.674c.3.922-.755 1.688-1.538 1.118l-3.976-2.888a1 1 0 00-1.176 0l-3.976 2.888c-.783.57-1.838-.197-1.538-1.118l1.518-4.674a1 1 0 00-.363-1.118l-3.976-2.888c-.784-.57-.38-1.81.588-1.81h4.914a1 1 0 00.951-.69l1.519-4.674z"/></svg></div><span class="text-2xl font-bold text-white">{{ t('home.reviews.review7.name') }}</span><span class="text-xs text-slate-400">{{ t('home.reviews.review7.role') }}</span></div>
            <div class="flex flex-col items-center rounded-2xl border border-[#20f5b4]/20 bg-white/[0.06] p-6 backdrop-blur transition-all hover:-translate-y-1 hover:bg-white/[0.09]"><div class="mb-2 flex h-10 w-10 items-center justify-center rounded-xl bg-sky-400/15 text-sky-300"><svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"/></svg></div><span class="text-2xl font-bold text-white">{{ t('home.reviews.review8.name') }}</span><span class="text-xs text-slate-400">{{ t('home.reviews.review8.role') }}</span></div>
          </div>
        </div>

        <!-- Customer Testimonials -->
        <div class="relative mb-16 overflow-hidden rounded-[2rem] border border-[#0757b8]/10 bg-white/95 p-8 shadow-xl shadow-[#0757b8]/5 backdrop-blur md:p-10 dark:border-slate-700 dark:bg-slate-900/90">
          <div class="mb-10 text-center">
            <p class="text-xs font-black uppercase tracking-[0.22em] text-[#0757b8] dark:text-emerald-400">{{ t('home.testimonials.eyebrow') }}</p>
            <h2 class="mt-3 text-3xl font-black tracking-tight text-gray-950 dark:text-white md:text-4xl">{{ t('home.testimonials.title') }}</h2>
            <p class="mx-auto mt-4 max-w-2xl text-sm leading-relaxed text-gray-500 dark:text-slate-400">{{ t('home.testimonials.subtitle') }}</p>
          </div>
          <div class="grid gap-5 md:grid-cols-2 lg:grid-cols-3">
            <div class="group relative overflow-hidden rounded-2xl border border-slate-200/70 bg-white p-6 shadow-sm transition-all hover:-translate-y-1 hover:shadow-xl dark:border-slate-700 dark:bg-slate-800/60">
              <div class="absolute -right-6 -top-6 h-20 w-20 rounded-full bg-[#20f5b4]/10 blur-2xl group-hover:bg-[#20f5b4]/20"></div>
              <div class="relative">
                <div class="mb-4 flex items-center gap-3">
                  <div class="flex h-12 w-12 items-center justify-center rounded-full bg-gradient-to-br from-[#20f5b4] to-[#0757b8] text-lg font-black text-white">A</div>
                  <div>
                    <p class="font-bold text-gray-950 dark:text-white">{{ t('home.testimonials.t1.name') }}</p>
                    <p class="text-xs text-slate-500 dark:text-slate-400">{{ t('home.testimonials.t1.role') }}</p>
                  </div>
                </div>
                <div class="mb-3 flex text-amber-400">★★★★★</div>
                <p class="text-sm leading-relaxed text-slate-600 dark:text-slate-300">{{ t('home.testimonials.t1.quote') }}</p>
              </div>
            </div>
            <div class="group relative overflow-hidden rounded-2xl border border-slate-200/70 bg-white p-6 shadow-sm transition-all hover:-translate-y-1 hover:shadow-xl dark:border-slate-700 dark:bg-slate-800/60">
              <div class="absolute -right-6 -top-6 h-20 w-20 rounded-full bg-[#0757b8]/10 blur-2xl group-hover:bg-[#0757b8]/20"></div>
              <div class="relative">
                <div class="mb-4 flex items-center gap-3">
                  <div class="flex h-12 w-12 items-center justify-center rounded-full bg-gradient-to-br from-cyan-400 to-blue-600 text-lg font-black text-white">M</div>
                  <div>
                    <p class="font-bold text-gray-950 dark:text-white">{{ t('home.testimonials.t2.name') }}</p>
                    <p class="text-xs text-slate-500 dark:text-slate-400">{{ t('home.testimonials.t2.role') }}</p>
                  </div>
                </div>
                <div class="mb-3 flex text-amber-400">★★★★★</div>
                <p class="text-sm leading-relaxed text-slate-600 dark:text-slate-300">{{ t('home.testimonials.t2.quote') }}</p>
              </div>
            </div>
            <div class="group relative overflow-hidden rounded-2xl border border-slate-200/70 bg-white p-6 shadow-sm transition-all hover:-translate-y-1 hover:shadow-xl dark:border-slate-700 dark:bg-slate-800/60">
              <div class="absolute -right-6 -top-6 h-20 w-20 rounded-full bg-[#20f5b4]/10 blur-2xl group-hover:bg-[#20f5b4]/20"></div>
              <div class="relative">
                <div class="mb-4 flex items-center gap-3">
                  <div class="flex h-12 w-12 items-center justify-center rounded-full bg-gradient-to-br from-emerald-400 to-teal-600 text-lg font-black text-white">S</div>
                  <div>
                    <p class="font-bold text-gray-950 dark:text-white">{{ t('home.testimonials.t3.name') }}</p>
                    <p class="text-xs text-slate-500 dark:text-slate-400">{{ t('home.testimonials.t3.role') }}</p>
                  </div>
                </div>
                <div class="mb-3 flex text-amber-400">★★★★★</div>
                <p class="text-sm leading-relaxed text-slate-600 dark:text-slate-300">{{ t('home.testimonials.t3.quote') }}</p>
              </div>
            </div>
          </div>
        </div>

        <!-- FAQ Section -->
        <section class="mb-16 rounded-[2rem] border border-[#0757b8]/10 bg-white/90 p-8 shadow-xl shadow-[#0757b8]/5 backdrop-blur md:p-10">
          <div class="mb-8 text-center">
            <p class="text-xs font-bold uppercase tracking-[0.22em] text-[#0757b8]">{{ t('home.easyrouterFaq.eyebrow') }}</p>
            <h2 class="mt-3 text-3xl font-black tracking-tight text-gray-950 md:text-4xl">{{ t('home.easyrouterFaq.title') }}</h2>
            <p class="mx-auto mt-4 max-w-2xl text-sm leading-relaxed text-gray-500">{{ t('home.easyrouterFaq.subtitle') }}</p>
          </div>

          <div class="grid gap-6 lg:grid-cols-3">
            <div class="rounded-3xl border border-blue-100 bg-blue-50/50 p-5">
              <div class="mb-5 flex items-center gap-3">
                <div class="flex h-11 w-11 items-center justify-center rounded-2xl bg-blue-600 text-white shadow-lg shadow-blue-500/20">
                  <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" /></svg>
                </div>
                <h3 class="text-lg font-bold text-gray-950">{{ t('home.easyrouterFaq.tabs.service') }}</h3>
              </div>
              <div class="space-y-3">
                <div class="rounded-2xl bg-white p-4 shadow-sm"><h4 class="font-semibold text-gray-950">{{ t('home.easyrouterFaq.service.q1') }}</h4><p class="mt-2 text-sm leading-6 text-gray-600">{{ t('home.easyrouterFaq.service.a1') }}</p></div>
                <div class="rounded-2xl bg-white p-4 shadow-sm"><h4 class="font-semibold text-gray-950">{{ t('home.easyrouterFaq.service.q2') }}</h4><p class="mt-2 text-sm leading-6 text-gray-600">{{ t('home.easyrouterFaq.service.a2') }}</p></div>
                <div class="rounded-2xl bg-white p-4 shadow-sm"><h4 class="font-semibold text-gray-950">{{ t('home.easyrouterFaq.service.q3') }}</h4><p class="mt-2 text-sm leading-6 text-gray-600">{{ t('home.easyrouterFaq.service.a3') }}</p></div>
                <div class="rounded-2xl bg-white p-4 shadow-sm"><h4 class="font-semibold text-gray-950">{{ t('home.easyrouterFaq.service.q4') }}</h4><p class="mt-2 text-sm leading-6 text-gray-600">{{ t('home.easyrouterFaq.service.a4') }}</p></div>
              </div>
            </div>

            <div class="rounded-3xl border border-emerald-100 bg-emerald-50/50 p-5">
              <div class="mb-5 flex items-center gap-3">
                <div class="flex h-11 w-11 items-center justify-center rounded-2xl bg-emerald-600 text-white shadow-lg shadow-emerald-500/20">
                  <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8V7m0 10v-1m9-4a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
                </div>
                <h3 class="text-lg font-bold text-gray-950">{{ t('home.easyrouterFaq.tabs.billing') }}</h3>
              </div>
              <div class="space-y-3">
                <div class="rounded-2xl bg-white p-4 shadow-sm"><h4 class="font-semibold text-gray-950">{{ t('home.easyrouterFaq.billing.q1') }}</h4><p class="mt-2 text-sm leading-6 text-gray-600">{{ t('home.easyrouterFaq.billing.a1') }}</p></div>
                <div class="rounded-2xl bg-white p-4 shadow-sm"><h4 class="font-semibold text-gray-950">{{ t('home.easyrouterFaq.billing.q2') }}</h4><p class="mt-2 text-sm leading-6 text-gray-600">{{ t('home.easyrouterFaq.billing.a2') }}</p></div>
                <div class="rounded-2xl bg-white p-4 shadow-sm"><h4 class="font-semibold text-gray-950">{{ t('home.easyrouterFaq.billing.q3') }}</h4><p class="mt-2 text-sm leading-6 text-gray-600">{{ t('home.easyrouterFaq.billing.a3') }}</p></div>
                <div class="rounded-2xl bg-white p-4 shadow-sm"><h4 class="font-semibold text-gray-950">{{ t('home.easyrouterFaq.billing.q4') }}</h4><p class="mt-2 text-sm leading-6 text-gray-600">{{ t('home.easyrouterFaq.billing.a4') }}</p></div>
              </div>
            </div>

            <div class="rounded-3xl border border-[#20f5b4]/20 bg-[#f0fffb] p-5">
              <div class="mb-5 flex items-center gap-3">
                <div class="flex h-11 w-11 items-center justify-center rounded-2xl bg-[#021b4a] text-[#20f5b4] shadow-lg shadow-[#0757b8]/20">
                  <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M8 9l3 3-3 3m5 0h3M5 5h14a2 2 0 012 2v10a2 2 0 01-2 2H5a2 2 0 01-2-2V7a2 2 0 012-2z" /></svg>
                </div>
                <h3 class="text-lg font-bold text-gray-950">{{ t('home.easyrouterFaq.tabs.integration') }}</h3>
              </div>
              <div class="space-y-3">
                <div class="rounded-2xl bg-white p-4 shadow-sm"><h4 class="font-semibold text-gray-950">{{ t('home.easyrouterFaq.integration.q1') }}</h4><p class="mt-2 text-sm leading-6 text-gray-600">{{ t('home.easyrouterFaq.integration.a1') }}</p></div>
                <div class="rounded-2xl bg-white p-4 shadow-sm"><h4 class="font-semibold text-gray-950">{{ t('home.easyrouterFaq.integration.q2') }}</h4><p class="mt-2 text-sm leading-6 text-gray-600">{{ t('home.easyrouterFaq.integration.a2') }}</p></div>
                <div class="rounded-2xl bg-white p-4 shadow-sm"><h4 class="font-semibold text-gray-950">{{ t('home.easyrouterFaq.integration.q3') }}</h4><p class="mt-2 text-sm leading-6 text-gray-600">{{ t('home.easyrouterFaq.integration.a3') }}</p></div>
                <div class="rounded-2xl bg-white p-4 shadow-sm"><h4 class="font-semibold text-gray-950">{{ t('home.easyrouterFaq.integration.q4') }}</h4><p class="mt-2 text-sm leading-6 text-gray-600">{{ t('home.easyrouterFaq.integration.a4') }}</p></div>
              </div>
            </div>
          </div>
        </section>
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
import { setLocale } from '@/i18n'
import { useAppStore } from '@/stores'
import type { CustomMenuItem } from '@/types'
import Icon from '@/components/icons/Icon.vue'
import LogoSvg from '@/assets/icons/logo.svg'

const { t, locale } = useI18n()

const router = useRouter()
const appStore = useAppStore()

const isDark = ref(false)
const currentYear = computed(() => new Date().getFullYear())
const currentLang = computed<'zh' | 'en'>(() => (locale.value === 'zh' ? 'zh' : 'en'))

const costComparisonBars = [
  { name: 'GPT-5.5', price: '5.00', height: '100%', isOpenSource: false },
  { name: 'Claude Opus 4.8', price: '5.00', height: '100%', isOpenSource: false },
  { name: 'DeepSeek V4-Pro', price: '0.42', height: '8.4%', saving: 92, isOpenSource: true },
  { name: 'DeepSeek V4-Flash', price: '0.14', height: '3%', saving: 97, isOpenSource: true },
  { name: 'MiniMax-M3', price: '0.60', height: '12%', saving: 88, isOpenSource: true }
]

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
.cost-bar {
  animation: cost-bar-grow 0.9s cubic-bezier(0.22, 1, 0.36, 1) both;
  transform-origin: left center;
}

@keyframes cost-bar-grow {
  from {
    transform: scaleX(0);
  }
  to {
    transform: scaleX(1);
  }
}

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
  color: #22d3ee;
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

.brand-reviews :deep(.testimonial-track-1 > div),
.brand-reviews :deep(.testimonial-track-2 > div) {
  border-color: rgba(32, 245, 180, 0.18) !important;
  background: rgba(255, 255, 255, 0.08) !important;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.18) !important;
  backdrop-filter: blur(16px);
}

.brand-reviews :deep(p.text-gray-700) {
  color: rgba(219, 234, 254, 0.86) !important;
  line-height: 1.7;
}

.brand-reviews :deep(p.font-semibold) {
  color: #ffffff !important;
}

.brand-reviews :deep(p.text-sm.text-gray-500) {
  color: rgba(32, 245, 180, 0.72) !important;
}

.brand-reviews :deep(img) {
  border: 2px solid rgba(32, 245, 180, 0.42);
  box-shadow: 0 0 0 4px rgba(32, 245, 180, 0.08);
}

/* Code Example Styles */
.code-example-body {
  text-align: left;
  padding: 16px 20px;
}

.code-example-body .code-line {
  margin-bottom: 2px;
  line-height: 1.7;
  font-size: 13px;
  font-family: 'Menlo', 'Monaco', 'Courier New', monospace;
}

.code-example-body .code-blank {
  height: 6px;
}

.code-example-body .code-comment {
  color: #6b7280;
}

.code-example-body .code-key {
  color: #67e8f9;
}

.code-example-body .code-operator {
  color: #94a3b8;
}

.code-example-body .code-class {
  color: #a78bfa;
}

.code-example-body .code-paren {
  color: #e2e8f0;
}

.code-example-body .code-string {
  color: #a3e635;
}

.code-example-body .code-func {
  color: #e2e8f0;
}

.code-example-body .code-indent-s {
  padding-left: 16px;
  display: inline-block;
}
</style>