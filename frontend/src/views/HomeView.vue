<template>
  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="hasHomeContent" class="min-h-screen">
    <!-- iframe mode -->
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <!-- HTML mode - SECURITY: homeContent is admin-only setting, XSS risk is acceptable -->
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- Compact Home Page -->
  <div
    v-else-if="compactHomeEnabled"
    data-testid="compact-home"
    class="flex min-h-screen flex-col bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white"
  >
    <header class="border-b border-gray-200 px-4 py-4 sm:px-6 dark:border-dark-800">
      <nav class="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 sm:gap-4">
        <router-link to="/" class="flex min-w-0 flex-1 items-center gap-3" :aria-label="siteName + ' Home'">
          <img
            :src="siteLogo || '/logo.svg'"
            alt="Logo"
            class="h-9 w-9 shrink-0 rounded-lg object-contain"
          />
          <span class="min-w-0 truncate text-base font-semibold">{{ siteName }}</span>
        </router-link>
        <div class="flex max-w-full shrink-0 flex-wrap items-center justify-end gap-2">
          <LocaleSwitcher />
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>
         
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="inline-flex min-h-10 shrink-0 items-center justify-center rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-200"
          >
            {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main class="flex min-w-0 flex-1 items-center justify-center px-4 py-16 sm:px-6">
      <div class="min-w-0 max-w-2xl text-center">
        <img
          :src="siteLogo || '/logo.svg'"
          alt="Logo"
          class="mx-auto mb-6 h-20 w-20 rounded-2xl object-contain"
        />
        <h1 class="[overflow-wrap:anywhere] text-3xl font-bold md:text-4xl">{{ siteName }}</h1>
        <p class="mt-4 whitespace-pre-wrap [overflow-wrap:anywhere] text-base text-gray-600 dark:text-dark-300">{{ siteSubtitle }}</p>
        <router-link
          :to="isAuthenticated ? dashboardPath : '/login'"
          class="mt-8 inline-flex min-h-10 items-center justify-center rounded-lg bg-primary-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-primary-700"
        >
          {{ isAuthenticated ? t('home.goToDashboard') : t('home.login') }}
        </router-link>
      </div>
    </main>

    <footer class="min-w-0 border-t border-gray-200 px-4 py-5 text-center text-sm text-gray-500 [overflow-wrap:anywhere] sm:px-6 dark:border-dark-800 dark:text-dark-400">
      &copy; {{ currentYear }} {{ siteName }}
    </footer>
  </div>

  <!-- Default Home Page -->
  <div
    class="relative flex min-h-screen flex-col overflow-hidden bg-gradient-to-br from-white via-blue-50/30 to-cyan-50/30"
  >
    <!-- Background Decorations -->
    <div class="pointer-events-none absolute inset-0 overflow-hidden">
      <div
        class="absolute right-0 top-0 h-[600px] w-full bg-gradient-to-br from-blue-500/20 to-cyan-500/20 to-cyan-400/20 blur-3xl"
      ></div>
      <div
        class="absolute -bottom-40 left-0 h-full rounded-full bg-gradient-to-br from-emerald-400/15 to-teal-500/15 blur-3xl"
      ></div>
      <div
        class="absolute inset-0 bg-[linear-gradient(rgba(59,130,246,0.08)_1px,transparent_1px),linear-gradient(90deg,rgba(59,130,246,0.08)_1px,transparent_1px)] bg-[size:64px_64px]"
      ></div>
    </div>

    <!-- Header -->
    <header class="relative z-20 px-6 py-4">
      <nav class="mx-auto flex max-w-7xl items-center justify-between">
        <!-- Logo -->
        <router-link to="/" class="flex items-center gap-[12px] pl-0" aria-label="Three Router Home">
          <img 
            :src="LogoSvg"
            alt="Three Router Logo" 
            class="h-[32px] w-[32px] object-contain"
          />
          <span class="text-[16px] font-semibold text-[#021b4a]" style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;">Three Router</span>
        </router-link>
 

        <!-- Nav Actions -->
        <div class="flex flex-wrap items-center gap-3">
          <!-- Limited Time Banner -->
          <div class="hidden items-center gap-2 rounded-full bg-orange-100 px-4 py-2 sm:flex">
            <svg class="h-4 w-4 text-orange-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <span class="text-sm font-medium text-orange-700">🎁LIMITED TIME: $10 Free Credits (≈ 330M DeepSeek Tokens)</span>
          </div>

          <!-- Nav Links -->
          <!-- Deepseek Harness for Threerouter 详情页 -->
          <a
            href="/dsh.html"
            title="Deepseek Harness for Threerouter"
            class="group flex items-center gap-[8px] rounded-lg px-3 py-2 text-sm font-medium text-[var(--ds-harness-fg)] transition-colors hover:bg-gray-100"
          >
            <span class="shrink-0 inline-flex">
              <svg width="20" height="18" viewBox="0 0 26.634 19.6" fill="none" class="opacity-90">
                <path d="M26.5174 3.39471C26.235 3.2567 26.1137 3.52006 25.9487 3.65346C25.8923 3.69659 25.8446 3.75294 25.7969 3.80469C25.3846 4.24516 24.9027 4.53439 24.2737 4.49989C23.3536 4.44814 22.5682 4.73737 21.8735 5.44119C21.7258 4.57349 21.2353 4.0554 20.4889 3.72304C20.0985 3.55054 19.7034 3.37746 19.4297 3.00197C19.2388 2.73459 19.1865 2.43673 19.091 2.14289C19.0301 1.96579 18.9697 1.78466 18.7656 1.75418C18.5442 1.71968 18.4574 1.90541 18.3705 2.06067C18.0232 2.69549 17.8887 3.39471 17.9019 4.10313C17.9324 5.6965 18.6051 6.96556 19.9421 7.86834C20.0939 7.97184 20.133 8.07535 20.0852 8.22658C19.9938 8.53766 19.8857 8.83955 19.7903 9.15063C18.692 8.9994 18.0583 8.54571 17.4982 7.99772C16.5477 7.07827 15.6881 6.06336 14.6162 5.26869C14.3644 5.08296 14.1125 4.91045 13.8521 4.746C12.7584 3.68394 13.9952 2.81164 14.2816 2.70814C14.5812 2.60003 14.3857 2.22857 13.4179 2.23317C12.4502 2.2372 11.5646 2.56151 10.4359 2.99335C10.2708 3.05832 10.0972 3.10547 9.91951 3.14457C8.8954 2.95022 7.83162 2.90709 6.72069 3.03245C4.62877 3.26533 2.95777 4.25436 1.72954 5.94261C0.254043 7.97184 -0.0932678 10.2777 0.33167 12.6824C0.778458 15.2171 2.07225 17.3153 4.06008 18.9558C6.12152 20.6567 8.49577 21.4905 11.2047 21.3306C12.8498 21.2358 14.6812 21.0155 16.7473 19.2669C17.2682 19.5262 17.8151 19.6297 18.7219 19.7074C19.4205 19.7723 20.0933 19.6729 20.6143 19.5648C21.4302 19.3923 21.3739 18.6367 21.0789 18.4981C18.6874 17.3843 19.2124 17.8374 18.7351 17.4706C19.9501 16.033 21.8063 13.4776 22.379 9.99821C22.4353 9.61409 22.5072 9.073 22.4986 8.76192C22.494 8.57216 22.5377 8.49856 22.7545 8.47671C23.3536 8.40771 23.935 8.24383 24.4692 7.94999C26.0188 7.10357 26.6439 5.71318 26.7911 4.04678C26.8129 3.79204 26.7865 3.52869 26.5174 3.39471ZM13.0143 18.3946C10.6964 16.5724 9.5722 15.9726 9.10816 15.9985C8.67402 16.0244 8.75222 16.5212 8.84768 16.8449C8.94773 17.1646 9.07768 17.3849 9.25996 17.6655C9.38589 17.8512 9.47272 18.1272 9.13404 18.3348C8.38766 18.7965 7.08985 18.1796 7.0289 18.1491C5.51833 17.2595 4.25559 16.0853 3.36546 14.4793C2.50581 12.9337 2.0067 11.2753 1.92447 9.50542C1.90262 9.07818 2.02855 8.92695 2.45406 8.84932C3.01413 8.74582 3.59144 8.72397 4.15093 8.80619C6.51656 9.15178 8.53027 10.2092 10.2185 11.8848C11.1822 12.8388 11.9114 13.979 12.6623 15.0929C13.461 16.2757 14.3201 17.4027 15.4144 18.3268C15.8008 18.6505 16.109 18.8966 16.404 19.0783C15.5144 19.1778 14.0297 19.1991 13.0143 18.3958V18.3946ZM14.1252 11.2489C14.1252 11.0591 14.277 10.9079 14.4679 10.9079C14.511 10.9079 14.5501 10.9165 14.5852 10.9292C14.6329 10.9464 14.6766 10.9723 14.7111 11.0114C14.7721 11.0718 14.8066 11.158 14.8066 11.2489C14.8066 11.4386 14.6548 11.5899 14.4639 11.5899C14.273 11.5899 14.1252 11.4386 14.1252 11.2489ZM17.5759 13.0188C17.3545 13.1096 17.1331 13.1873 16.9203 13.1959C16.5903 13.2131 16.2303 13.0791 16.0348 12.9153C15.7312 12.6605 15.5139 12.5179 15.423 12.0734C15.3839 11.8837 15.4057 11.5899 15.4402 11.4214C15.5185 11.0585 15.4316 10.8257 15.1757 10.614C14.9676 10.4415 14.7025 10.3938 14.4115 10.3938C14.3029 10.3938 14.2034 10.3461 14.1292 10.3076C14.0079 10.2472 13.9078 10.096 14.0033 9.91023C14.0338 9.84985 14.1815 9.70322 14.216 9.67734C14.6111 9.45251 15.0665 9.52612 15.488 9.6946C15.8784 9.85445 16.174 10.1477 16.5989 10.5623C17.033 11.0631 17.1112 11.2011 17.3585 11.5772C17.554 11.871 17.7317 12.1729 17.8536 12.5185C17.9272 12.7341 17.8317 12.9107 17.5759 13.0188Z" fill="currentColor"/>
              </svg>
            </span>
            <span class="inline-flex items-start gap-[5px] min-w-0">
              <span class="shrink-0 inline-flex">
                <span class="inline-flex items-center rounded-[8px] p-[1px] min-w-0 max-w-full" style="background:linear-gradient(135deg, rgba(0,90,190,0.35) 0%, rgba(0,90,190,0.06) 35%, rgba(0,90,190,0.03) 65%, rgba(0,90,190,0.22) 100%);box-shadow:0 0 14px rgba(7,87,184,0.08)">
                  <span class="min-w-0 truncate pt-[4px] pb-[3px] rounded-[7px] font-mono text-[11px] font-medium leading-none px-[9px] text-[#0757b8]">Media</span>
                </span>
              </span>
            </span>
            <svg class="h-3.5 w-3.5 opacity-60" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
            </svg>
          </a>

          <button
            @click="router.push('/admin/models')"
            class="rounded-lg px-3 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-100"
          >
            {{ t('home.models') }}
          </button>
          <!-- Login / User Button -->
          <button
            @click="router.push(isAuthenticated ? dashboardPath : '/login')"
            class="shrink-0 rounded-lg bg-[#0757b8] px-5 py-2 text-sm font-semibold text-white shadow-lg shadow-blue-900/10 transition-colors hover:bg-[#064ea8] cursor-pointer"
          >
            {{ isAuthenticated ? (authStore.user?.username || (currentLang === 'zh' ? '控制台' : 'Dashboard')) : (currentLang === 'zh' ? '登录' : 'Login') }}
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
              <span class="text-sm font-medium text-gray-700">kimi-k3</span>
              <span class="rounded-full bg-emerald-100 px-2 py-0.5 text-xs font-medium text-emerald-600">{{ t('home.providers.supported') }}</span>
            </span>
            <span class="flex items-center gap-2 rounded-xl border border-orange-100 bg-white px-4 py-2.5">
              <span class="flex h-6 w-6 items-center justify-center rounded-full bg-orange-500 text-xs font-bold text-white">Q</span>
              <span class="text-sm font-medium text-gray-700">qwen3.8-max</span>
              <span class="rounded-full bg-emerald-100 px-2 py-0.5 text-xs font-medium text-emerald-600">{{ t('home.providers.supported') }}</span>
            </span>
            <span class="flex items-center gap-2 rounded-xl border border-blue-100 bg-white px-4 py-2.5">
              <span class="flex h-6 w-6 items-center justify-center rounded-full bg-blue-500 text-xs font-bold text-white">G</span>
              <span class="text-sm font-medium text-gray-700">glm-5.3</span>
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
        <section class="mb-16 overflow-hidden rounded-[2rem] border border-slate-200/70 bg-gradient-to-br from-white via-blue-50/60 to-cyan-50/60 p-10 text-slate-900 shadow-xl shadow-[#0757b8]/5 md:p-12">
          <div class="mb-10 flex flex-col justify-between gap-6 md:flex-row md:items-end">
            <div>
              <p class="text-sm font-bold uppercase tracking-[0.24em] text-[#20f5b4]">{{ t('home.easyrouterAdvantages.eyebrow') }}</p>
              <h2 class="mt-4 text-4xl font-black tracking-tight md:text-5xl">{{ t('home.easyrouterAdvantages.title') }}</h2>
            </div>
            <p class="max-w-xl text-base leading-relaxed text-slate-600 md:text-right">{{ t('home.easyrouterAdvantages.subtitle') }}</p>
          </div>

          <div class="grid gap-5 md:grid-cols-2 lg:grid-cols-5">
            <div class="rounded-3xl border border-emerald-200/70 bg-white p-6 transition-all hover:-translate-y-1 hover:bg-emerald-50 lg:col-span-1">
              <div class="mb-5 flex h-14 w-14 items-center justify-center rounded-2xl bg-emerald-100 text-emerald-600">
                <svg class="h-6 w-6" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z" /></svg>
              </div>
              <h3 class="text-lg font-bold">{{ t('home.easyrouterAdvantages.standardApi.title') }}</h3>
              <p class="mt-2 text-base leading-relaxed text-slate-600">{{ t('home.easyrouterAdvantages.standardApi.desc') }}</p>
            </div>
            <div class="rounded-3xl border border-slate-200/70 bg-white p-6 transition-all hover:-translate-y-1 hover:bg-slate-50 lg:col-span-1">
              <div class="mb-5 flex h-14 w-14 items-center justify-center rounded-2xl bg-emerald-100 text-emerald-600">
                <svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
              </div>
              <h3 class="text-lg font-bold">{{ t('home.easyrouterAdvantages.reliable.title') }}</h3>
              <p class="mt-2 text-base leading-relaxed text-slate-600">{{ t('home.easyrouterAdvantages.reliable.desc') }}</p>
            </div>
            <div class="rounded-3xl border border-slate-200/70 bg-white p-6 transition-all hover:-translate-y-1 hover:bg-slate-50 lg:col-span-1">
              <div class="mb-5 flex h-14 w-14 items-center justify-center rounded-2xl bg-cyan-100 text-cyan-600">
                <svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M13 10V3L4 14h7v7l9-11h-7z" /></svg>
              </div>
              <h3 class="text-lg font-bold">{{ t('home.easyrouterAdvantages.ultraFast.title') }}</h3>
              <p class="mt-2 text-base leading-relaxed text-slate-600">{{ t('home.easyrouterAdvantages.ultraFast.desc') }}</p>
            </div>
            <div class="rounded-3xl border border-slate-200/70 bg-white p-6 transition-all hover:-translate-y-1 hover:bg-slate-50 lg:col-span-1">
              <div class="mb-5 flex h-14 w-14 items-center justify-center rounded-2xl bg-emerald-100 text-emerald-600">
                <svg class="h-6 w-6" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8V7m0 10v-1m9-4a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
              </div>
              <h3 class="text-lg font-bold">{{ t('home.features.balanceQuota') }}</h3>
              <p class="mt-2 text-base leading-relaxed text-slate-600">{{ t('home.features.balanceQuotaDesc') }}</p>
            </div>
            <div class="rounded-3xl border border-slate-200/70 bg-white p-6 transition-all hover:-translate-y-1 hover:bg-slate-50 md:col-span-2 lg:col-span-1">
              <div class="mb-5 flex h-14 w-14 items-center justify-center rounded-2xl bg-emerald-100 text-emerald-600">
                <svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8V7m0 10v-1m9-4a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
              </div>
              <h3 class="text-lg font-bold text-slate-900">{{ t('home.easyrouterAdvantages.cheap.title') }}</h3>
              <p class="mt-2 text-base leading-relaxed text-slate-600">{{ t('home.easyrouterAdvantages.cheap.desc') }}</p>
            </div>
          </div>
        </section>
  <!-- Pricing & Direct Procurement Section -->
        <div class="mb-16">
          <!-- Price Advantage Section - Full Width Header -->
          <div class="mb-8 overflow-hidden rounded-3xl border border-[#20f5b4]/30 bg-white/95 shadow-2xl shadow-[#0757b8]/10 backdrop-blur dark:border-emerald-800/50 dark:bg-gray-900/80">
            <div class="relative px-8 py-8 sm:px-10">
              <div class="absolute inset-0 bg-[radial-gradient(circle_at_top_left,rgba(32,245,180,0.2),transparent_36%),linear-gradient(135deg,rgba(239,252,255,0.96),rgba(255,255,255,0.98))] dark:bg-[radial-gradient(circle_at_top_left,rgba(16,185,129,0.2),transparent_34%),linear-gradient(135deg,rgba(6,78,59,0.28),rgba(17,24,39,0.96))]"></div>
              <div class="relative z-10 grid gap-8 lg:grid-cols-[0.86fr_1.14fr] lg:items-center">
                <div class="relative z-10">
                  <p class="text-sm font-semibold uppercase tracking-[0.22em] text-[#0757b8] dark:text-emerald-400">
                    US Local Deployment + 97% Cost Cut
                  </p>
                  <h3 class="mt-3 text-3xl font-black leading-tight text-slate-900 dark:text-white md:text-4xl">
                    {{ t('home.hero.priceAdvantage') }}
                  </h3>
                  <p class="mt-3 text-base font-semibold leading-relaxed text-slate-700 dark:text-emerald-300">
                    {{ t('home.hero.priceReasons') }}
                  </p>
                  <div class="mt-6 grid grid-cols-2 gap-3">
                    <div class="rounded-2xl border border-[#0757b8]/10 bg-white/75 p-4">
                      <p class="text-xs font-bold uppercase tracking-[0.16em] text-slate-400">{{ t('home.hero.costChart.usRoute') }}</p>
                      <p class="mt-2 text-2xl font-black text-[#021b4a]">US-East / US-West</p>
                    </div>
                    <div class="rounded-2xl bg-gradient-to-br from-emerald-50 to-cyan-50 p-4 shadow-md ring-1 ring-emerald-200/60">
                      <p class="text-xs font-bold uppercase tracking-[0.16em] text-emerald-600">{{ t('home.hero.costChart.maxSaving') }}</p>
                      <p class="mt-2 text-4xl font-black leading-none text-emerald-600">97%</p>
                    </div>
                  </div>
                </div>

                <div class="rounded-[1.75rem] border border-slate-200/70 bg-white p-5 text-slate-900 shadow-lg ring-1 ring-slate-200/60">
                  <div class="mb-5 flex items-start justify-between gap-4">
                    <div>
                      <p class="text-xs font-black uppercase tracking-[0.22em] text-emerald-600">{{ t('home.hero.costChart.eyebrow') }}</p>
                      <h4 class="mt-2 text-xl font-black text-slate-900">{{ t('home.hero.costChart.title') }}</h4>
                    </div>
                    <span class="rounded-full bg-emerald-100 px-3 py-1 text-xs font-bold text-emerald-700 ring-1 ring-emerald-200">$/M input</span>
                  </div>
                  <div class="flex h-[340px] gap-3 md:h-[400px] md:gap-4">
                    <div class="relative flex flex-1 items-end gap-2 rounded-2xl border border-slate-200 bg-slate-50 p-3 md:gap-3 md:p-4">
                      <div v-for="item in costComparisonBars" :key="item.name" class="flex h-full flex-1 flex-col items-center justify-end gap-2 text-center">
                        <div class="mb-1 text-xs font-black leading-tight">
                          <span class="block text-emerald-600">${{ item.price }}</span>
                          <span v-if="item.saving" class="mt-0.5 block text-[10px] font-semibold uppercase tracking-wide text-emerald-700">-{{ item.saving }}%</span>
                        </div>
                        <div
                          class="cost-bar w-full rounded-t-xl transition-all duration-700 hover:brightness-110"
                          :class="item.isOpenSource ? 'bg-gradient-to-t from-emerald-500 to-cyan-400 shadow-[0_0_18px_rgba(16,185,129,0.25)]' : 'bg-gradient-to-t from-slate-500 to-slate-300'"
                          :style="{ height: item.height }"
                        ></div>
                        <div class="mt-1 w-full">
                          <p class="truncate text-[10px] font-bold leading-tight text-slate-600 md:text-xs" :title="item.name">{{ item.name }}</p>
                        </div>
                      </div>
                    </div>
                  </div>
                  <p class="mt-5 rounded-2xl border border-emerald-200 bg-emerald-50 p-4 text-sm font-semibold leading-relaxed text-emerald-800">
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
        <!-- Compliance & AI Governance Promo -->
        <div class="relative mt-16 mb-16 overflow-hidden rounded-[2rem] border border-[#0757b8]/10 bg-white/95 p-8 shadow-xl shadow-[#0757b8]/5 backdrop-blur md:p-10">
          <div class="pointer-events-none absolute -right-16 -top-16 h-64 w-64 rounded-full bg-[#0757b8]/10 blur-3xl"></div>
          <div class="pointer-events-none absolute -left-16 -bottom-16 h-64 w-64 rounded-full bg-[#20f5b4]/10 blur-3xl"></div>

          <!-- Header -->
          <div class="relative mb-8 flex flex-col items-center gap-3 text-center md:flex-row md:items-center md:justify-between md:text-left">
            <div class="flex flex-col items-center gap-4 md:flex-row md:items-center md:gap-5">
              <div class="flex h-14 w-14 flex-shrink-0 items-center justify-center rounded-2xl bg-gradient-to-br from-[#0757b8] to-[#20f5b4] text-white shadow-lg shadow-blue-500/20">
                <svg class="h-7 w-7" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                  <path stroke-linecap="round" stroke-linejoin="round" d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
                </svg>
              </div>
              <div>
                <div class="mb-1 inline-flex items-center gap-2 rounded-full bg-blue-50 px-3 py-1">
                  <span class="h-2 w-2 rounded-full bg-[#0757b8]"></span>
                  <span class="text-xs font-semibold text-[#0757b8]">{{ t('home.compliancePromo.badge') }}</span>
                </div>
                <p class="text-xs font-bold uppercase tracking-[0.22em] text-[#0757b8]">{{ t('home.compliancePromo.eyebrow') }}</p>
                <h2 class="mt-2 text-2xl font-black tracking-tight text-gray-950 md:text-3xl">{{ t('home.compliancePromo.title') }}</h2>
                <p class="mt-2 max-w-xl text-base leading-relaxed text-gray-500">{{ t('home.compliancePromo.description') }}</p>
              </div>
            </div>
            <button @click="router.push('/compliance')" class="rounded-full bg-gradient-to-r from-[#0757b8] to-[#20f5b4] px-8 py-4 text-base font-semibold text-white shadow-lg shadow-blue-500/20 transition-all hover:scale-105 hover:shadow-xl cursor-pointer">
              {{ t('home.compliancePromo.button') }}
            </button>
          </div>

          <!-- Feature Cards -->
          <div class="relative grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <div class="group rounded-2xl border border-blue-100 bg-gradient-to-br from-blue-50/60 to-white p-5 transition-all hover:shadow-lg">
              <div class="mb-3 flex h-10 w-10 items-center justify-center rounded-xl bg-blue-100 text-blue-600">
                <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4"/></svg>
              </div>
              <h3 class="text-sm font-bold text-gray-950">{{ t('home.compliancePromo.features.euai.title') }}</h3>
              <p class="mt-1 text-xs leading-relaxed text-gray-500">{{ t('home.compliancePromo.features.euai.desc') }}</p>
            </div>
            <div class="group rounded-2xl border border-purple-100 bg-gradient-to-br from-purple-50/60 to-white p-5 transition-all hover:shadow-lg">
              <div class="mb-3 flex h-10 w-10 items-center justify-center rounded-xl bg-purple-100 text-purple-600">
                <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/></svg>
              </div>
              <h3 class="text-sm font-bold text-gray-950">{{ t('home.compliancePromo.features.gdpr.title') }}</h3>
              <p class="mt-1 text-xs leading-relaxed text-gray-500">{{ t('home.compliancePromo.features.gdpr.desc') }}</p>
            </div>
            <div class="group rounded-2xl border border-emerald-100 bg-gradient-to-br from-emerald-50/60 to-white p-5 transition-all hover:shadow-lg">
              <div class="mb-3 flex h-10 w-10 items-center justify-center rounded-xl bg-emerald-100 text-emerald-600">
                <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/></svg>
              </div>
              <h3 class="text-sm font-bold text-gray-950">{{ t('home.compliancePromo.features.zdr.title') }}</h3>
              <p class="mt-1 text-xs leading-relaxed text-gray-500">{{ t('home.compliancePromo.features.zdr.desc') }}</p>
            </div>
            <div class="group rounded-2xl border border-amber-100 bg-gradient-to-br from-amber-50/60 to-white p-5 transition-all hover:shadow-lg">
              <div class="mb-3 flex h-10 w-10 items-center justify-center rounded-xl bg-amber-100 text-amber-600">
                <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M11.049 2.927c.3-.921 1.603-.921 1.902 0l1.519 4.674a1 1 0 00.95.69h4.915c.969 0 1.371 1.24.588 1.81l-3.976 2.888a1 1 0 00-.363 1.118l1.518 4.674c.3.922-.755 1.688-1.538 1.118l-3.976-2.888a1 1 0 00-1.176 0l-3.976 2.888c-.783.57-1.838-.197-1.538-1.118l1.518-4.674a1 1 0 00-.363-1.118l-3.976-2.888c-.784-.57-.38-1.81.588-1.81h4.914a1 1 0 00.951-.69l1.519-4.674z"/></svg>
              </div>
              <h3 class="text-sm font-bold text-gray-950">{{ t('home.compliancePromo.features.creds.title') }}</h3>
              <p class="mt-1 text-xs leading-relaxed text-gray-500">{{ t('home.compliancePromo.features.creds.desc') }}</p>
            </div>
            <div class="group rounded-2xl border border-cyan-100 bg-gradient-to-br from-cyan-50/60 to-white p-5 transition-all hover:shadow-lg">
              <div class="mb-3 flex h-10 w-10 items-center justify-center rounded-xl bg-cyan-100 text-cyan-600">
                <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"/></svg>
              </div>
              <h3 class="text-sm font-bold text-gray-950">{{ t('home.compliancePromo.features.templates.title') }}</h3>
              <p class="mt-1 text-xs leading-relaxed text-gray-500">{{ t('home.compliancePromo.features.templates.desc') }}</p>
            </div>
            <div class="group rounded-2xl border border-rose-100 bg-gradient-to-br from-rose-50/60 to-white p-5 transition-all hover:shadow-lg">
              <div class="mb-3 flex h-10 w-10 items-center justify-center rounded-xl bg-rose-100 text-rose-600">
                <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z"/></svg>
              </div>
              <h3 class="text-sm font-bold text-gray-950">{{ t('home.compliancePromo.features.risk.title') }}</h3>
              <p class="mt-1 text-xs leading-relaxed text-gray-500">{{ t('home.compliancePromo.features.risk.desc') }}</p>
            </div>
          </div>

          <!-- Certificate Badges -->
          <div class="relative mt-8 flex flex-wrap items-center justify-center gap-2 border-t border-gray-100 pt-6 md:justify-start">
            <span class="text-xs font-medium text-gray-500">{{ t('home.compliancePromo.certsLabel') }}</span>
            <span class="rounded-full bg-blue-50 px-3 py-1 text-xs font-semibold text-blue-700">GDPR</span>
            <span class="rounded-full bg-purple-50 px-3 py-1 text-xs font-semibold text-purple-700">EU AI Act</span>
            <span class="rounded-full bg-emerald-50 px-3 py-1 text-xs font-semibold text-emerald-700">ZDR</span>
            <span class="rounded-full bg-amber-50 px-3 py-1 text-xs font-semibold text-amber-700">DPA</span>
            <span class="rounded-full bg-rose-50 px-3 py-1 text-xs font-semibold text-rose-700">HIPAA</span>
          </div>
        </div>

        <div class="brand-reviews relative mt-16 mb-16 overflow-hidden rounded-[2rem] border border-slate-200/70 bg-gradient-to-br from-white via-blue-50/50 to-cyan-50/50 px-6 py-10 shadow-xl shadow-[#0757b8]/5 md:px-8 md:py-12">
          <div class="pointer-events-none absolute -right-24 -top-24 h-72 w-72 rounded-full bg-emerald-200/30 blur-3xl"></div>
          <div class="pointer-events-none absolute -left-24 bottom-0 h-72 w-72 rounded-full bg-blue-200/40 blur-3xl"></div>
          <div class="relative text-center mb-12">
            <p class="mb-3 text-xs font-bold uppercase tracking-[0.24em] text-[#0757b8]">{{ t('home.reviews.title') }}</p>
            <h2 class="text-3xl font-black tracking-tight text-slate-900 mb-4 md:text-4xl">{{ t('home.reviews.title') }}</h2>
            <p class="mx-auto max-w-2xl text-lg leading-relaxed text-slate-600">{{ t('home.reviews.subtitle') }}</p>
          </div>
          <div class="relative grid grid-cols-2 gap-4 md:grid-cols-4">
            <div class="flex flex-col items-center rounded-2xl border border-slate-200/70 bg-white p-6 transition-all hover:-translate-y-1 hover:bg-slate-50"><div class="mb-2 flex h-10 w-10 items-center justify-center rounded-xl bg-emerald-100 text-emerald-600"><svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6"/></svg></div><span class="text-2xl font-bold text-slate-900">{{ t('home.reviews.review1.name') }}</span><span class="text-xs text-slate-500">{{ t('home.reviews.review1.role') }}</span></div>
            <div class="flex flex-col items-center rounded-2xl border border-slate-200/70 bg-white p-6 transition-all hover:-translate-y-1 hover:bg-slate-50"><div class="mb-2 flex h-10 w-10 items-center justify-center rounded-xl bg-cyan-100 text-cyan-600"><svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M13 10V3L4 14h7v7l9-11h-7z"/></svg></div><span class="text-2xl font-bold text-slate-900">{{ t('home.reviews.review2.name') }}</span><span class="text-xs text-slate-500">{{ t('home.reviews.review2.role') }}</span></div>
            <div class="flex flex-col items-center rounded-2xl border border-slate-200/70 bg-white p-6 transition-all hover:-translate-y-1 hover:bg-slate-50"><div class="mb-2 flex h-10 w-10 items-center justify-center rounded-xl bg-emerald-100 text-emerald-600"><svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/></svg></div><span class="text-2xl font-bold text-slate-900">{{ t('home.reviews.review3.name') }}</span><span class="text-xs text-slate-500">{{ t('home.reviews.review3.role') }}</span></div>
            <div class="flex flex-col items-center rounded-2xl border border-slate-200/70 bg-white p-6 transition-all hover:-translate-y-1 hover:bg-slate-50"><div class="mb-2 flex h-10 w-10 items-center justify-center rounded-xl bg-amber-100 text-amber-600"><svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/></svg></div><span class="text-2xl font-bold text-slate-900">{{ t('home.reviews.review4.name') }}</span><span class="text-xs text-slate-500">{{ t('home.reviews.review4.role') }}</span></div>
            <div class="flex flex-col items-center rounded-2xl border border-slate-200/70 bg-white p-6 transition-all hover:-translate-y-1 hover:bg-slate-50"><div class="mb-2 flex h-10 w-10 items-center justify-center rounded-xl bg-purple-100 text-purple-600"><svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M11.049 2.927c.3-.921 1.603-.921 1.902 0l1.519 4.674a1 1 0 00.95.69h4.915c.969 0 1.371 1.24.588 1.81l-3.976 2.888a1 1 0 00-.363 1.118l1.518 4.674c.3.922-.755 1.688-1.538 1.118l-3.976-2.888a1 1 0 00-1.176 0l-3.976 2.888c-.783.57-1.838-.197-1.538-1.118l1.518-4.674a1 1 0 00-.363-1.118l-3.976-2.888c-.784-.57-.38-1.81.588-1.81h4.914a1 1 0 00.951-.69l1.519-4.674z"/></svg></div><span class="text-2xl font-bold text-slate-900">{{ t('home.reviews.review5.name') }}</span><span class="text-xs text-slate-500">{{ t('home.reviews.review5.role') }}</span></div>
            <div class="flex flex-col items-center rounded-2xl border border-slate-200/70 bg-white p-6 transition-all hover:-translate-y-1 hover:bg-slate-50"><div class="mb-2 flex h-10 w-10 items-center justify-center rounded-xl bg-rose-100 text-rose-600"><svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M14 10h4.764a2 2 0 011.789 2.894l-3.5 7A2 2 0 0115.263 21h-4.017c-.163 0-.326-.02-.485-.06L7 20m7-10V5a2 2 0 00-2-2h-.095c-.5 0-.905.405-.905.905 0 .714-.211 1.412-.608 2.006L7 11v9m7-10h-2M7 20H5a2 2 0 01-2-2v-6a2 2 0 012-2h2.5"/></svg></div><span class="text-2xl font-bold text-slate-900">{{ t('home.reviews.review6.name') }}</span><span class="text-xs text-slate-500">{{ t('home.reviews.review6.role') }}</span></div>
            <div class="flex flex-col items-center rounded-2xl border border-slate-200/70 bg-white p-6 transition-all hover:-translate-y-1 hover:bg-slate-50"><div class="mb-2 flex h-10 w-10 items-center justify-center rounded-xl bg-orange-100 text-orange-600"><svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M11.049 2.927c.3-.921 1.603-.921 1.902 0l1.519 4.674a1 1 0 00.95.69h4.915c.969 0 1.371 1.24.588 1.81l-3.976 2.888a1 1 0 00-.363 1.118l1.518 4.674c.3.922-.755 1.688-1.538 1.118l-3.976-2.888a1 1 0 00-1.176 0l-3.976 2.888c-.783.57-1.838-.197-1.538-1.118l1.518-4.674a1 1 0 00-.363-1.118l-3.976-2.888c-.784-.57-.38-1.81.588-1.81h4.914a1 1 0 00.951-.69l1.519-4.674z"/></svg></div><span class="text-2xl font-bold text-slate-900">{{ t('home.reviews.review7.name') }}</span><span class="text-xs text-slate-500">{{ t('home.reviews.review7.role') }}</span></div>
            <div class="flex flex-col items-center rounded-2xl border border-slate-200/70 bg-white p-6 transition-all hover:-translate-y-1 hover:bg-slate-50"><div class="mb-2 flex h-10 w-10 items-center justify-center rounded-xl bg-sky-100 text-sky-600"><svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"/></svg></div><span class="text-2xl font-bold text-slate-900">{{ t('home.reviews.review8.name') }}</span><span class="text-xs text-slate-500">{{ t('home.reviews.review8.role') }}</span></div>
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
                <div class="flex h-11 w-11 items-center justify-center rounded-2xl bg-emerald-50 text-emerald-600 ring-1 ring-emerald-200/60 shadow-sm">
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
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { setLocale, getLocale } from '@/i18n'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import type { CustomMenuItem } from '@/types'
import Icon from '@/components/icons/Icon.vue'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import LogoSvg from '@/assets/icons/logo.webp'
import { useSEO } from '@/composables/useSEO'
import { sanitizeUrl } from '@/utils/url'

const { t, locale } = useI18n()

const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()

const isAuthenticated = computed(() => authStore.isAuthenticated)
const dashboardPath = computed(() => authStore.isAuthenticated ? (authStore.isAdmin ? '/admin' : '/dashboard') : '/login')

// Check if home content is a URL (iframe mode) vs HTML content
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  if (!content) return false
  return /^https?:\/\//i.test(content)
})

// FAQPage 结构化数据 - 从 i18n FAQ 内容动态构建，支持中英文切换
const faqPageJsonLd = computed(() => ({
  '@context': 'https://schema.org',
  '@type': 'FAQPage',
  'mainEntity': [
    // 服务类 FAQ
    {
      '@type': 'Question',
      'name': t('home.easyrouterFaq.service.q1'),
      'acceptedAnswer': { '@type': 'Answer', 'text': t('home.easyrouterFaq.service.a1') }
    },
    {
      '@type': 'Question',
      'name': t('home.easyrouterFaq.service.q2'),
      'acceptedAnswer': { '@type': 'Answer', 'text': t('home.easyrouterFaq.service.a2') }
    },
    {
      '@type': 'Question',
      'name': t('home.easyrouterFaq.service.q3'),
      'acceptedAnswer': { '@type': 'Answer', 'text': t('home.easyrouterFaq.service.a3') }
    },
    {
      '@type': 'Question',
      'name': t('home.easyrouterFaq.service.q4'),
      'acceptedAnswer': { '@type': 'Answer', 'text': t('home.easyrouterFaq.service.a4') }
    },
    // 计费类 FAQ
    {
      '@type': 'Question',
      'name': t('home.easyrouterFaq.billing.q1'),
      'acceptedAnswer': { '@type': 'Answer', 'text': t('home.easyrouterFaq.billing.a1') }
    },
    {
      '@type': 'Question',
      'name': t('home.easyrouterFaq.billing.q2'),
      'acceptedAnswer': { '@type': 'Answer', 'text': t('home.easyrouterFaq.billing.a2') }
    },
    {
      '@type': 'Question',
      'name': t('home.easyrouterFaq.billing.q3'),
      'acceptedAnswer': { '@type': 'Answer', 'text': t('home.easyrouterFaq.billing.a3') }
    },
    {
      '@type': 'Question',
      'name': t('home.easyrouterFaq.billing.q4'),
      'acceptedAnswer': { '@type': 'Answer', 'text': t('home.easyrouterFaq.billing.a4') }
    },
    // 集成类 FAQ
    {
      '@type': 'Question',
      'name': t('home.easyrouterFaq.integration.q1'),
      'acceptedAnswer': { '@type': 'Answer', 'text': t('home.easyrouterFaq.integration.a1') }
    },
    {
      '@type': 'Question',
      'name': t('home.easyrouterFaq.integration.q2'),
      'acceptedAnswer': { '@type': 'Answer', 'text': t('home.easyrouterFaq.integration.a2') }
    },
    {
      '@type': 'Question',
      'name': t('home.easyrouterFaq.integration.q3'),
      'acceptedAnswer': { '@type': 'Answer', 'text': t('home.easyrouterFaq.integration.a3') }
    },
    {
      '@type': 'Question',
      'name': t('home.easyrouterFaq.integration.q4'),
      'acceptedAnswer': { '@type': 'Answer', 'text': t('home.easyrouterFaq.integration.a4') }
    }
  ]
}))
// Site settings - directly from appStore (already initialized from injected config)
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'AI API Gateway Platform')
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const hasHomeContent = computed(() => homeContent.value.trim().length > 0)
const compactHomeEnabled = computed(() => appStore.cachedPublicSettings?.compact_home_enabled === true)

// 合并所有 JSON-LD 结构化数据（WebSite + SoftwareApplication + FAQPage）
const homeJsonLd = computed(() => [
  {
    '@context': 'https://schema.org',
    '@type': 'WebSite',
    'name': 'ThreeRouter',
    'url': 'https://www.threerouter.com',
    'description': 'Enterprise-grade AI API management platform. One API key to access DeepSeek, Claude, GPT, Kimi, GLM and more.',
    'inLanguage': ['zh-CN', 'en']
  },
  {
    '@context': 'https://schema.org',
    '@type': 'SoftwareApplication',
    'name': 'ThreeRouter',
    'applicationCategory': 'DeveloperApplication',
    'operatingSystem': 'Cloud',
    'description': 'Unified AI API management platform, OpenAI-compatible, with access to DeepSeek, Claude, GPT and more.',
    'url': 'https://www.threerouter.com',
    'offers': {
      '@type': 'Offer',
      'price': '0',
      'priceCurrency': 'USD',
      'description': 'New users get $10 in free credits'
    }
  },
  // 防御：faqPageJsonLd 理论上始终返回对象，但过滤防空值以防 Safari 崩溃
  faqPageJsonLd.value
].filter(Boolean))

useSEO({
  title: 'ThreeRouter - Unified AI API Gateway | DeepSeek/Claude/GPT Multi-Model Management Platform',
  description: 'ThreeRouter is an enterprise-grade AI API management platform. One API key to access DeepSeek, Claude, GPT, Kimi, GLM and more. US-based deployment, OpenAI-compatible, up to 97% cost savings, P99 latency under 200ms.',
  keywords: 'AI API,API gateway,DeepSeek API,Claude API,GPT API,LLM API,OpenAI compatible,enterprise AI,Token management,AI proxy,US deployment',
  ogType: 'website',
  ogImage: 'https://www.threerouter.com/src/assets/icons/logo.webp',
  ogUrl: 'https://www.threerouter.com/home',
  ogSiteName: 'ThreeRouter',
  canonicalUrl: 'https://www.threerouter.com/home',
  jsonLd: homeJsonLd
})

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


// 首页默认英文界面，除非用户手动切换语言
let homeSavedLocale: string | null = null
let userSwitchedLang = false

function toggleLanguage() {
  userSwitchedLang = true
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
  // 首页强制英文（用户可手动切换）
  homeSavedLocale = getLocale()
  if (homeSavedLocale !== 'en') {
    setLocale('en')
  }
  // Fetch public settings to get contact info and custom menu items
  await appStore.fetchPublicSettings()
})

onBeforeUnmount(() => {
  // 离开首页时恢复用户原语言（如果用户没手动切换的话）
  if (homeSavedLocale && homeSavedLocale !== 'en' && !userSwitchedLang) {
    setLocale(homeSavedLocale)
  }
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