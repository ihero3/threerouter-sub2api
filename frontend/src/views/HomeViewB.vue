<template>
  <div class="min-h-screen bg-[#f6f8fb] text-slate-800">
    <!-- 顶部提示条：开发者预览 -->
    <div class="bg-slate-900 px-4 py-2 text-center text-xs text-slate-300">
      开发者预览版本 · 功能快速迭代，可能存在破坏性变更 ·
      <a href="https://github.com/deepseek-ai/deepseek-harness/blob/main/SAFETY.md" target="_blank" rel="noopener noreferrer" class="text-teal-300 underline underline-offset-2">使用前请阅读安全说明</a>
    </div>

    <!-- 顶部导航 -->
    <header class="sticky top-0 z-40 border-b border-slate-200/80 bg-white/90 backdrop-blur-lg">
      <nav class="mx-auto flex h-16 max-w-7xl items-center justify-between gap-4 px-4 sm:px-6">
        <a href="#top" class="flex min-w-0 items-center gap-2.5">
          <img :src="siteLogo || '/logo.webp'" alt="Logo" class="h-9 w-9 shrink-0 rounded-lg object-contain" />
          <span class="min-w-0 truncate text-sm font-bold text-slate-900 sm:text-base">Deepseek Harness for Threevideo</span>
        </a>
        <div class="hidden items-center gap-5 text-sm font-medium text-slate-600 xl:flex">
          <a href="#flow" class="transition-colors hover:text-teal-600">使用流程</a>
          <a href="#agent" class="transition-colors hover:text-teal-600">Agent 能力</a>
          <a href="#workbench" class="transition-colors hover:text-teal-600">创作工作台</a>
          <a href="#media" class="transition-colors hover:text-teal-600">生图生视频</a>
          <a href="#scenes" class="transition-colors hover:text-teal-600">应用场景</a>
          <a href="#faq" class="transition-colors hover:text-teal-600">常见问题</a>
        </div>
        <div class="flex shrink-0 items-center gap-2">
          <LocaleSwitcher v-if="showLanguageSwitcher" />
          <router-link :to="dashboardPath" class="hidden h-9 items-center rounded-lg px-3 text-sm font-medium text-slate-600 hover:bg-slate-100 sm:inline-flex">
            {{ isAuthenticated ? '控制台' : '登录' }}
          </router-link>
          <a href="https://www.threevideo.com/dsh.html" target="_blank" rel="noopener noreferrer" class="inline-flex h-9 items-center rounded-lg bg-teal-600 px-4 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-teal-700">下载客户端</a>
        </div>
      </nav>
    </header>

    <main id="top" class="relative">
      <!-- Hero -->
      <section class="relative overflow-hidden bg-gradient-to-b from-[#eef7f6] via-white to-white">
        <div class="pointer-events-none absolute -right-40 -top-40 h-[520px] w-[520px] rounded-full bg-teal-200/40 blur-3xl"></div>
        <div class="pointer-events-none absolute -left-32 top-40 h-80 w-80 rounded-full bg-cyan-100/60 blur-3xl"></div>
        <div class="relative mx-auto grid max-w-7xl items-center gap-12 px-4 py-16 sm:px-6 sm:py-24 lg:grid-cols-2 lg:gap-16">
          <div>
            <span class="inline-flex items-center gap-2 rounded-full border border-teal-200 bg-white/80 px-4 py-1.5 text-xs font-semibold text-teal-700 shadow-sm">
              <span class="h-1.5 w-1.5 animate-pulse rounded-full bg-teal-500"></span>
              基于 DeepSeek Harness（MIT 开源）构建
            </span>
            <h1 class="mt-6 text-4xl font-black leading-[1.15] tracking-tight text-slate-900 sm:text-5xl lg:text-[3.4rem]">
              一次登录，<br />
              <span class="bg-gradient-to-r from-teal-600 to-cyan-500 bg-clip-text text-transparent">写代码、生图、做视频</span>
            </h1>
            <p class="mt-6 max-w-xl text-base leading-relaxed text-slate-600 sm:text-lg">
              Deepseek Harness for Threevideo 是一款桌面 AI 应用：左侧对话交给 Agent 执行任务，右侧创作工作台完成快速创作、工作流编排与时间轴精剪。用 Threevideo 账号登录即完成配置，无需复制 API Key、无需填写接口地址。
            </p>
            <div class="mt-8 flex flex-wrap items-center gap-3">
              <a href="https://www.threevideo.com/dsh.html" target="_blank" rel="noopener noreferrer" class="inline-flex min-h-12 items-center justify-center gap-2 rounded-xl bg-teal-600 px-7 text-base font-semibold text-white shadow-lg shadow-teal-600/25 transition-colors hover:bg-teal-700">
                下载客户端
                <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M17 8l4 4m0 0l-4 4m4-4H3" /></svg>
              </a>
              <a href="https://www.threevideo.com/register" target="_blank" rel="noopener noreferrer" class="inline-flex min-h-12 items-center justify-center rounded-xl border border-slate-300 bg-white px-7 text-base font-semibold text-slate-700 transition-colors hover:bg-slate-50">创建账号</a>
            </div>
            <div class="mt-8 flex flex-wrap gap-x-5 gap-y-2 text-sm text-slate-500">
              <span class="inline-flex items-center gap-1.5"><span class="text-teal-600">✓</span> 免密钥配置</span>
              <span class="inline-flex items-center gap-1.5"><span class="text-teal-600">✓</span> 对话 + 创作工作台</span>
              <span class="inline-flex items-center gap-1.5"><span class="text-teal-600">✓</span> Windows / macOS / Linux</span>
            </div>
            <p class="mt-5 text-xs text-slate-400">DeepSeek Harness 由 DeepSeek AI 开源（MIT）；本产品为其桌面集成版本，与 DeepSeek AI 无隶属或背书关系。</p>
          </div>

          <!-- 产品界面展示 -->
          <div class="relative">
            <div class="absolute -inset-5 rounded-[2rem] bg-gradient-to-tr from-teal-200/50 to-cyan-100/40 blur-2xl"></div>
            <div class="relative overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-2xl shadow-slate-900/10">
              <div class="flex items-center gap-2 border-b border-slate-100 bg-slate-50 px-4 py-3">
                <span class="h-2.5 w-2.5 rounded-full bg-rose-400"></span>
                <span class="h-2.5 w-2.5 rounded-full bg-amber-400"></span>
                <span class="h-2.5 w-2.5 rounded-full bg-emerald-400"></span>
                <span class="ml-3 text-xs font-medium text-slate-400">Deepseek Harness for Threevideo · 客户端界面</span>
              </div>
              <img src="/media/DSH1-clean.png" alt="Deepseek Harness for Threevideo 桌面客户端界面" class="block w-full" />
            </div>
            <div class="absolute -bottom-5 -left-4 hidden rounded-xl border border-slate-200 bg-white px-4 py-3 shadow-xl sm:block">
              <div class="text-xs text-slate-400">左侧对话执行任务 · 右侧工作台完成创作</div>
              <div class="mt-1 text-sm font-bold text-slate-800">文本 / 图片 / 视频，一个窗口打通</div>
            </div>
          </div>
        </div>
      </section>

      <!-- 推广横幅（读取当前站点 promo 配置） -->
      <div class="mx-auto max-w-7xl px-4 sm:px-6">
        <PromoBanner class="mt-10" />
      </div>

      <!-- 全流程贯通 -->
      <section id="flow" class="mx-auto max-w-7xl px-4 py-20 sm:px-6">
        <div class="text-center">
          <p class="text-xs font-bold uppercase tracking-[0.2em] text-teal-600">WORKFLOW</p>
          <h2 class="mt-3 text-3xl font-bold text-slate-900 sm:text-4xl">从登录到交付，一气呵成</h2>
          <p class="mx-auto mt-4 max-w-2xl text-sm leading-relaxed text-slate-500 sm:text-base">配置、对话、创作、编排、导出串成一条完整流程，专注任务本身，而不是环境搭建。</p>
        </div>
        <div class="mt-12 grid gap-4 md:grid-cols-5">
          <div v-for="(step, i) in flowSteps" :key="step.title" class="relative rounded-2xl border border-slate-200 bg-white p-5 shadow-sm">
            <div class="mb-3 flex h-9 w-9 items-center justify-center rounded-xl bg-teal-50 text-sm font-black text-teal-600">{{ i + 1 }}</div>
            <h3 class="text-sm font-bold text-slate-900">{{ step.title }}</h3>
            <p class="mt-2 text-xs leading-relaxed text-slate-500">{{ step.desc }}</p>
            <div v-if="i < flowSteps.length - 1" class="absolute -right-3 top-1/2 hidden h-6 w-6 -translate-y-1/2 items-center justify-center rounded-full bg-teal-50 text-teal-500 md:flex">›</div>
          </div>
        </div>
      </section>

      <!-- 核心能力 -->
      <section id="agent" class="bg-white py-20">
        <div class="mx-auto max-w-7xl px-4 sm:px-6">
          <div class="text-center">
            <p class="text-xs font-bold uppercase tracking-[0.2em] text-teal-600">AGENT</p>
            <h2 class="mt-3 text-3xl font-bold text-slate-900 sm:text-4xl">真正动手的编程 Agent</h2>
            <p class="mx-auto mt-4 max-w-2xl text-sm leading-relaxed text-slate-500 sm:text-base">它不只是回答问题：交给它一个任务，它会读写文件、运行命令、维护待办计划，并在需要时调用子代理。</p>
          </div>

          <!-- 登录即配置 -->
          <div class="mt-14 grid items-center gap-10 lg:grid-cols-2">
            <div>
              <span class="inline-block rounded-full bg-teal-50 px-3 py-1 text-xs font-semibold text-teal-700">开始使用</span>
              <h3 class="mt-4 text-2xl font-bold text-slate-900 sm:text-3xl">登录即完成配置，无需复制密钥</h3>
              <p class="mt-4 leading-relaxed text-slate-600">用 Threevideo 账号登录，应用会自动创建或复用 API Key、注册模型路由、载入账户可用模型，并设定默认模型。网关地址已内置，你不需要粘贴密钥，也不需要填写接口地址。</p>
              <ul class="mt-6 space-y-3">
                <li v-for="item in loginPoints" :key="item" class="flex items-start gap-3 text-sm text-slate-600"><span class="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-teal-100 text-xs font-bold text-teal-700">✓</span>{{ item }}</li>
              </ul>
            </div>
            <div class="overflow-hidden rounded-2xl border border-slate-200 shadow-xl shadow-slate-900/5">
              <img src="/media/DSH1-clean.png" alt="登录后的客户端与账户信息" class="block w-full" />
            </div>
          </div>

          <!-- Agent 能力矩阵 -->
          <div class="mt-20 rounded-3xl bg-gradient-to-br from-slate-900 to-teal-950 p-8 text-white sm:p-12">
            <div class="grid gap-8 lg:grid-cols-[1fr_1.1fr] lg:items-center">
              <div>
                <span class="inline-block rounded-full bg-white/10 px-3 py-1 text-xs font-semibold text-teal-200">能力矩阵</span>
                <h3 class="mt-4 text-2xl font-bold sm:text-3xl">从执行到编排，都能交给它</h3>
                <p class="mt-4 leading-relaxed text-slate-300">工具以插件形式按需装配：文件、命令、搜索、子代理、MCP 都可以扩展。涉及风险操作时，会先请求你的确认；会话中断后还能从断点继续。</p>
                <div class="mt-6 flex flex-wrap gap-3">
                  <span class="rounded-lg bg-white/5 px-3 py-1.5 text-xs text-slate-200 ring-1 ring-inset ring-white/10">一切皆插件</span>
                  <span class="rounded-lg bg-white/5 px-3 py-1.5 text-xs text-slate-200 ring-1 ring-inset ring-white/10">断点续跑</span>
                  <span class="rounded-lg bg-white/5 px-3 py-1.5 text-xs text-slate-200 ring-1 ring-inset ring-white/10">分层沙箱</span>
                </div>
              </div>
              <div class="grid grid-cols-2 gap-3 sm:grid-cols-3">
                <div v-for="item in agentPoints" :key="item.title" class="rounded-xl bg-white/5 p-4 ring-1 ring-inset ring-white/10">
                  <div class="text-xl">{{ item.icon }}</div>
                  <div class="mt-2 text-sm font-semibold text-white">{{ item.title }}</div>
                  <div class="mt-1 text-xs leading-relaxed text-slate-400">{{ item.desc }}</div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- 创作工作台（本版本核心更新） -->
      <section id="workbench" class="mx-auto max-w-7xl px-4 py-20 sm:px-6">
        <div class="text-center">
          <p class="text-xs font-bold uppercase tracking-[0.2em] text-teal-600">WORKBENCH</p>
          <h2 class="mt-3 text-3xl font-bold text-slate-900 sm:text-4xl">右侧创作工作台，从想法直接到成片</h2>
          <p class="mx-auto mt-4 max-w-2xl text-sm leading-relaxed text-slate-500 sm:text-base">不只是"生成一张图/一段视频"：快速创作、可编排的专业工作流、时间轴精剪三种模式，都在同一个工作台里完成。</p>
        </div>

        <div class="mt-10 overflow-hidden rounded-3xl border border-slate-200 bg-white shadow-xl shadow-slate-900/5">
          <div class="flex flex-wrap items-center gap-2 border-b border-slate-100 bg-slate-50 px-4 py-3 sm:px-6">
            <button v-for="tab in workbenchTabs" :key="tab.key" type="button" @click="activeWorkbenchTab = tab.key" class="rounded-lg px-4 py-2 text-sm font-semibold transition-colors" :class="activeWorkbenchTab === tab.key ? 'bg-teal-600 text-white shadow-sm' : 'text-slate-500 hover:bg-slate-200/70'">{{ tab.label }}</button>
            <span class="ml-auto hidden text-xs text-slate-400 sm:inline">界面示意</span>
          </div>
          <div class="grid gap-8 p-6 sm:p-10 lg:grid-cols-2 lg:items-center">
            <div>
              <h3 class="text-2xl font-bold text-slate-900">{{ activeWorkbench.title }}</h3>
              <p class="mt-3 leading-relaxed text-slate-600">{{ activeWorkbench.desc }}</p>
              <ul class="mt-6 space-y-3">
                <li v-for="point in activeWorkbench.points" :key="point" class="flex items-start gap-3 text-sm text-slate-600">
                  <span class="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-teal-100 text-xs font-bold text-teal-700">✓</span>{{ point }}
                </li>
              </ul>
              <div class="mt-6 flex flex-wrap gap-2">
                <span v-for="tag in activeWorkbench.tags" :key="tag" class="rounded-lg bg-slate-100 px-3 py-1.5 text-xs font-medium text-slate-600">{{ tag }}</span>
              </div>
            </div>

            <!-- 工作台示意（非截图） -->
            <div class="rounded-2xl border border-slate-200 bg-slate-50 p-4 sm:p-6">
              <!-- 快速创作示意 -->
              <div v-if="activeWorkbenchTab === 'quick'" class="space-y-3">
                <div class="flex items-center justify-between rounded-xl bg-white px-4 py-3 shadow-sm ring-1 ring-slate-200">
                  <span class="text-xs font-semibold text-slate-600">选择模型</span>
                  <span class="rounded-md bg-teal-50 px-2 py-1 text-xs font-medium text-teal-700">qwen-image-3.0 ▾</span>
                </div>
                <div class="grid grid-cols-3 gap-2">
                  <div v-for="card in ['文生图', '图生图', '文生视频']" :key="card" class="rounded-xl bg-white px-3 py-4 text-center text-xs font-semibold text-slate-600 shadow-sm ring-1 ring-slate-200">{{ card }}</div>
                </div>
                <div class="flex items-center justify-between rounded-xl bg-white px-4 py-3 shadow-sm ring-1 ring-slate-200">
                  <span class="text-xs text-slate-500">尺寸 / 比例 / 时长</span>
                  <span class="text-xs font-semibold text-slate-700">参数下拉可选</span>
                </div>
                <div class="rounded-xl bg-teal-600 px-4 py-3 text-center text-sm font-semibold text-white">一键成片 · 剧本直达分镜</div>
              </div>
              <!-- 专业工作流示意 -->
              <div v-else-if="activeWorkbenchTab === 'flow'" class="space-y-3">
                <div class="grid grid-cols-2 gap-3 sm:grid-cols-3">
                  <div v-for="(node, i) in ['读取剧本', '拆分镜', '生成图片', '生成视频', '配音', '合成成片']" :key="node" class="relative rounded-xl bg-white px-3 py-3 text-center text-xs font-semibold text-slate-600 shadow-sm ring-1 ring-slate-200">
                    <span class="mr-1 text-teal-500">{{ i + 1 }}</span>{{ node }}
                  </div>
                </div>
                <div class="rounded-xl border border-dashed border-teal-300 bg-teal-50/60 px-4 py-3 text-xs text-teal-700">React Flow 画布 · 节点可运行 / 取消 / 重试 · 结果缓存与历史回看</div>
                <div class="rounded-xl bg-white px-4 py-3 text-xs text-slate-500 shadow-sm ring-1 ring-slate-200">内置 7 套模板 · 可保存为自己的工作流</div>
              </div>
              <!-- 时间轴精剪示意 -->
              <div v-else class="space-y-3">
                <div v-for="track in timelineTracks" :key="track.label" class="rounded-xl bg-white p-3 shadow-sm ring-1 ring-slate-200">
                  <div class="mb-2 text-xs font-semibold text-slate-600">{{ track.label }}</div>
                  <div class="flex gap-1">
                    <div v-for="(clip, i) in track.clips" :key="i" class="h-6 rounded" :class="clip.color" :style="{ flex: clip.weight }"></div>
                  </div>
                </div>
                <div class="rounded-xl bg-teal-50 px-4 py-3 text-xs text-teal-700">拖拽 / 分割 / 裁剪 / 变速 · 字幕批量导入 · 多比例切片导出</div>
              </div>
            </div>
          </div>
        </div>

        <div class="mt-6 grid gap-4 sm:grid-cols-3">
          <div v-for="item in workbenchHighlights" :key="item.title" class="rounded-2xl border border-slate-200 bg-white p-5">
            <div class="text-2xl">{{ item.icon }}</div>
            <h4 class="mt-3 font-bold text-slate-900">{{ item.title }}</h4>
            <p class="mt-1.5 text-sm leading-relaxed text-slate-500">{{ item.desc }}</p>
          </div>
        </div>
      </section>

      <!-- 图片与视频生成 -->
      <section id="media" class="bg-white py-20">
        <div class="mx-auto max-w-7xl px-4 sm:px-6">
          <div class="text-center">
            <p class="text-xs font-bold uppercase tracking-[0.2em] text-teal-600">GENERATE</p>
            <h2 class="mt-3 text-3xl font-bold text-slate-900 sm:text-4xl">同一个输入框，生成图片与视频</h2>
            <p class="mx-auto mt-4 max-w-2xl text-sm leading-relaxed text-slate-500 sm:text-base">在输入框下方切换图片或视频，描述你想要的画面，选择模型、尺寸、风格与时长。生成结果自动保存到工作区，并可直接送入工作台继续编辑。</p>
          </div>

          <div class="mt-8 flex flex-wrap justify-center gap-2">
            <button v-for="tab in mediaTabs" :key="tab.key" type="button" @click="activeMediaTab = tab.key" class="rounded-full px-5 py-2 text-sm font-semibold transition-colors" :class="activeMediaTab === tab.key ? 'bg-teal-600 text-white shadow-sm' : 'bg-slate-100 text-slate-600 hover:bg-slate-200'">{{ tab.label }}</button>
          </div>

          <div class="mt-8 grid items-center gap-8 rounded-3xl border border-slate-200 bg-slate-50 p-6 sm:p-10 lg:grid-cols-2">
            <div>
              <h3 class="text-xl font-bold text-slate-900">{{ activeTabContent.title }}</h3>
              <p class="mt-3 leading-relaxed text-slate-600">{{ activeTabContent.desc }}</p>
              <div class="mt-5 flex flex-wrap gap-2">
                <span v-for="tag in activeTabContent.tags" :key="tag" class="rounded-lg bg-white px-3 py-1.5 text-xs font-medium text-slate-600 ring-1 ring-inset ring-slate-200">{{ tag }}</span>
              </div>
            </div>
            <div class="overflow-hidden rounded-2xl bg-white shadow-lg ring-1 ring-slate-200">
              <img :src="activeTabContent.image" :alt="activeTabContent.title" class="block max-h-[420px] w-full object-cover" />
            </div>
          </div>

          <div class="mt-6 grid gap-4 sm:grid-cols-3">
            <div v-for="item in mediaHighlights" :key="item.title" class="rounded-2xl border border-slate-200 bg-white p-5">
              <div class="text-2xl">{{ item.icon }}</div>
              <h4 class="mt-3 font-bold text-slate-900">{{ item.title }}</h4>
              <p class="mt-1.5 text-sm leading-relaxed text-slate-500">{{ item.desc }}</p>
            </div>
          </div>
        </div>
      </section>

      <!-- 数据带 -->
      <section class="bg-teal-700 py-12 text-white">
        <div class="mx-auto grid max-w-7xl gap-8 px-4 text-center sm:grid-cols-3 sm:px-6">
          <div>
            <div class="text-3xl font-black sm:text-4xl">1$ ≈ 30M token</div>
            <div class="mt-2 text-sm text-teal-100">文本用量参考价（约）</div>
          </div>
          <div>
            <div class="text-3xl font-black sm:text-4xl">30 秒</div>
            <div class="mt-2 text-sm text-teal-100">视频生成时长上限（视模型而定）</div>
          </div>
          <div>
            <div class="text-3xl font-black sm:text-4xl">MIT 开源</div>
            <div class="mt-2 text-sm text-teal-100">底层 DeepSeek Harness 开放源码</div>
          </div>
        </div>
        <p class="mx-auto mt-8 max-w-3xl px-4 text-center text-xs text-teal-100/80">价格为参考值，可能变动；以账户内实际显示为准。图片、视频生成按其所用模型单独计价；视频时长以所选模型实际支持范围为准。</p>
      </section>

      <!-- 更多能力 -->
      <section class="mx-auto max-w-7xl px-4 py-20 sm:px-6">
        <div class="text-center">
          <p class="text-xs font-bold uppercase tracking-[0.2em] text-teal-600">AND MORE</p>
          <h2 class="mt-3 text-3xl font-bold text-slate-900 sm:text-4xl">不止于生成</h2>
          <p class="mx-auto mt-4 max-w-2xl text-sm leading-relaxed text-slate-500 sm:text-base">可扩展的工具生态、可恢复的会话、跨平台的桌面体验，以及分层的安全机制。</p>
        </div>
        <div class="mt-12 grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
          <div v-for="item in extraCapabilities" :key="item.title" class="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
            <div class="text-2xl">{{ item.icon }}</div>
            <h3 class="mt-4 font-bold text-slate-900">{{ item.title }}</h3>
            <p class="mt-2 text-sm leading-relaxed text-slate-500">{{ item.desc }}</p>
          </div>
        </div>
      </section>

      <!-- 应用场景 -->
      <section id="scenes" class="bg-white py-20">
        <div class="mx-auto max-w-7xl px-4 sm:px-6">
          <div class="text-center">
            <p class="text-xs font-bold uppercase tracking-[0.2em] text-teal-600">USE CASES</p>
            <h2 class="mt-3 text-3xl font-bold text-slate-900 sm:text-4xl">为创作者与开发者而生</h2>
            <p class="mx-auto mt-4 max-w-2xl text-sm leading-relaxed text-slate-500 sm:text-base">无论是日常开发、内容生产，还是团队协作，都能在同一套工作流里完成。</p>
          </div>
          <div class="mt-12 grid gap-5 sm:grid-cols-2 lg:grid-cols-4">
            <div v-for="scene in scenes" :key="scene.title" class="group overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm transition-transform hover:-translate-y-1">
              <div class="h-40 overflow-hidden bg-slate-100"><img :src="scene.image" :alt="scene.title" class="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105" /></div>
              <div class="p-5">
                <h3 class="font-bold text-slate-900">{{ scene.title }}</h3>
                <p class="mt-2 text-sm leading-relaxed text-slate-500">{{ scene.desc }}</p>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- 开源与合规 -->
      <section class="py-16">
        <div class="mx-auto max-w-5xl px-4 sm:px-6">
          <div class="overflow-hidden rounded-3xl border border-slate-200 bg-gradient-to-br from-slate-50 to-teal-50/50 p-8 sm:p-12">
            <h2 class="text-2xl font-bold text-slate-900 sm:text-3xl">架构开放，来源清晰</h2>
            <p class="mt-4 leading-relaxed text-slate-600">Deepseek Harness for Threevideo 基于 DeepSeek Harness 构建——这是 DeepSeek AI 开源的智能体框架，采用 MIT 许可证，以「一切皆插件」为架构理念。我们在其上加入了 Threevideo 登录、模型路由、图片视频生成与创作工作台等集成能力。</p>
            <div class="mt-6 flex flex-wrap gap-3">
              <a href="https://github.com/deepseek-ai/deepseek-harness" target="_blank" rel="noopener noreferrer" class="inline-flex h-10 items-center rounded-lg border border-slate-300 bg-white px-4 text-sm font-semibold text-slate-700 hover:bg-slate-50">查看上游开源项目</a>
              <a href="https://deepseek-harness.github.io/deepseek-harness/" target="_blank" rel="noopener noreferrer" class="inline-flex h-10 items-center rounded-lg border border-slate-300 bg-white px-4 text-sm font-semibold text-slate-700 hover:bg-slate-50">开发文档</a>
              <a href="https://github.com/deepseek-ai/deepseek-harness/blob/main/SAFETY.md" target="_blank" rel="noopener noreferrer" class="inline-flex h-10 items-center rounded-lg border border-slate-300 bg-white px-4 text-sm font-semibold text-slate-700 hover:bg-slate-50">安全说明</a>
            </div>
            <p class="mt-6 text-xs leading-relaxed text-slate-400">DeepSeek Harness 是深度求索公司的注册商标。本产品为第三方集成版本，未获 DeepSeek AI 的背书、运营或支持。本产品处于开发者预览阶段，会执行模型生成的代码与命令；沙箱与确认机制可降低风险，但不构成隔离保证。</p>
          </div>
        </div>
      </section>

      <!-- FAQ -->
      <section id="faq" class="mx-auto max-w-4xl px-4 py-20 sm:px-6">
        <div class="text-center">
          <p class="text-xs font-bold uppercase tracking-[0.2em] text-teal-600">FAQ</p>
          <h2 class="mt-3 text-3xl font-bold text-slate-900 sm:text-4xl">常见问题</h2>
        </div>
        <div class="mt-10 space-y-3">
          <details v-for="item in faqs" :key="item.q" class="group rounded-2xl border border-slate-200 bg-white px-6 py-5 shadow-sm">
            <summary class="flex cursor-pointer list-none items-center justify-between gap-4 font-semibold text-slate-900">
              {{ item.q }}
              <span class="text-teal-600 transition-transform group-open:rotate-45">+</span>
            </summary>
            <p class="mt-3 text-sm leading-relaxed text-slate-600">{{ item.a }}</p>
          </details>
        </div>
      </section>

      <!-- 最终 CTA -->
      <section class="bg-gradient-to-br from-teal-600 to-cyan-600 py-16 text-white">
        <div class="mx-auto max-w-4xl px-4 text-center sm:px-6">
          <h2 class="text-3xl font-black sm:text-4xl">一次登录，开始创作</h2>
          <p class="mt-4 text-teal-50">写代码、生图片、做视频、剪成片——一个账号，一个窗口。</p>
          <div class="mt-8 flex flex-wrap justify-center gap-3">
            <a href="https://www.threevideo.com/dsh.html" target="_blank" rel="noopener noreferrer" class="inline-flex min-h-12 items-center rounded-xl bg-white px-7 text-base font-bold text-teal-700 shadow-lg transition-colors hover:bg-teal-50">下载客户端</a>
            <a href="https://www.threevideo.com/register" target="_blank" rel="noopener noreferrer" class="inline-flex min-h-12 items-center rounded-xl border border-white/60 px-7 text-base font-semibold text-white transition-colors hover:bg-white/10">创建 Threevideo 账号</a>
          </div>
          <p class="mt-6 text-xs text-teal-100/80">开发者预览版本，可能存在破坏性变更。使用前请阅读安全说明。</p>
        </div>
      </section>
    </main>

    <PublicSiteFooter />
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { useSEO } from '@/composables/useSEO'
import { useLocaleAccess } from '@/composables/useLocaleAccess'
import { sanitizeUrl } from '@/utils/url'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import PromoBanner from '@/components/common/PromoBanner.vue'
import PublicSiteFooter from '@/components/layout/PublicSiteFooter.vue'

const appStore = useAppStore()
const authStore = useAuthStore()
const { localeSwitcherVisible: showLanguageSwitcher } = useLocaleAccess()

const isAuthenticated = computed(() => authStore.isAuthenticated)
const dashboardPath = computed(() => authStore.isAuthenticated ? (authStore.isAdmin ? '/admin' : '/dashboard') : '/login')
const siteName = computed(() => appStore.activeSiteProfile?.name || appStore.cachedPublicSettings?.site_name || '三路由')
const siteLogo = computed(() => sanitizeUrl(appStore.activeSiteProfile?.logo || appStore.cachedPublicSettings?.site_logo || '', { allowRelative: true, allowDataUrl: true }))

const flowSteps = [
  { title: '登录账号', desc: 'Threevideo 账号登录，密钥与模型配置自动完成。' },
  { title: '描述任务', desc: '在左侧输入框说明需求，附上参考文件与图片。' },
  { title: '工作台创作', desc: '右侧切换快速创作、工作流画布或时间轴精剪。' },
  { title: '编排导出', desc: '一键成片、配音、多比例切片导出，结果落盘工作区。' },
  { title: '继续编辑', desc: '对话中的媒体卡片可发送到工作台或放入时间轴。' },
]

const loginPoints = [
  '自动创建或复用 API Key，无需手动复制',
  '自动注册模型路由，载入账户可用模型并设定默认模型',
  '账户胶囊显示余额与 API Key 状态；模型在输入框右侧或设置-模型页切换',
  '可通过邀请链接分享给团队成员',
]

const agentPoints = [
  { icon: '📝', title: '读写文件', desc: '在工作区内读取、编辑与搜索文件' },
  { icon: '⌨️', title: '运行命令', desc: '按权限策略执行 Shell / PowerShell' },
  { icon: '🗂️', title: '规划待办', desc: '维护多步骤任务清单与计划' },
  { icon: '🤝', title: '子代理协作', desc: '派生或分叉子代理并行处理任务' },
  { icon: '🔌', title: 'MCP 扩展', desc: 'stdio / HTTP 接入外部工具服务' },
  { icon: '⏱️', title: '断点续跑', desc: '会话持久化，中断后可从断点继续' },
]

type WorkbenchTabKey = 'quick' | 'flow' | 'timeline'
const activeWorkbenchTab = ref<WorkbenchTabKey>('quick')
const workbenchTabs: { key: WorkbenchTabKey; label: string }[] = [
  { key: 'quick', label: '快速创作' },
  { key: 'flow', label: '专业工作流' },
  { key: 'timeline', label: '时间轴精剪' },
]
const workbenchContent: Record<WorkbenchTabKey, { title: string; desc: string; points: string[]; tags: string[] }> = {
  quick: {
    title: '快速创作：选模型、填提示词、出片',
    desc: '模型能力卡展示擅长场景与参数上限；剧本粘贴一次，即可拆成分镜并串成流水线一键成片。融合生图支持角色、场景、物品多选取材。',
    points: [
      '剧本 → 分镜表 → 逐镜生成 → 一键成片',
      '融合生图：8 种姿态 + 7 种镜头预设',
      '最多 6 张参考图，拖拽排序；支持续写与尾帧',
      '对话中的图片/视频卡片可一键送入工作台',
    ],
    tags: ['一键成片', '融合生图', '多参考图', '模型能力卡'],
  },
  flow: {
    title: '专业工作流：把创作排成可复用画布',
    desc: '基于 React Flow 的可视化画布，内置 12 个可运行节点与 7 套模板。支持运行、取消、重试、单节点执行、结果缓存与历史回看。',
    points: [
      '节点级瞬时失败自动重试，长任务更稳',
      '读取剧本、拆分镜、生成图片/视频、配音、合成可串成一条链',
      '画布与连线持久化，回来接着改',
      '可保存为自己的工作流模板',
    ],
    tags: ['React Flow 画布', '12 个节点', '7 套模板', '结果缓存'],
  },
  timeline: {
    title: '时间轴精剪：成片之后继续打磨',
    desc: '视频、音频、字幕三轨编辑：拖拽、分割、复制、裁剪、转场、变速与淡入淡出；支持 SRT/VTT 批量字幕与 TTS 配音。',
    points: [
      '一条成片导出 9:16 / 1:1 / 4:3 多比例切片',
      '字幕批量导入，多轨对齐调整',
      'TTS 配音 + ffmpeg 合成，素材与成片同源',
      '导出历史可在工作台内回看',
    ],
    tags: ['三轨剪辑', '批量字幕', '多比例切片', '配音合成'],
  },
}
const activeWorkbench = computed(() => workbenchContent[activeWorkbenchTab.value])
const timelineTracks = [
  { label: '视频轨', clips: [{ color: 'bg-teal-300', weight: 2 }, { color: 'bg-teal-200', weight: 1 }, { color: 'bg-teal-400', weight: 3 }] },
  { label: '音频轨', clips: [{ color: 'bg-cyan-300', weight: 3 }, { color: 'bg-cyan-200', weight: 3 }] },
  { label: '字幕轨', clips: [{ color: 'bg-slate-300', weight: 1 }, { color: 'bg-slate-200', weight: 2 }, { color: 'bg-slate-300', weight: 3 }] },
]
const workbenchHighlights = [
  { icon: '🎬', title: '一键成片', desc: '从剧本到分镜到成片，不必在多个工具之间搬运素材。' },
  { icon: '🧩', title: '节点式编排', desc: '把创作步骤排成画布，失败可重试，结果可复用。' },
  { icon: '📤', title: '一条成片多比例', desc: '按平台需要导出横屏、竖屏与方形版本。' },
]

type MediaTabKey = 'image' | 'video' | 'reference'
const activeMediaTab = ref<MediaTabKey>('image')
const mediaTabs: { key: MediaTabKey; label: string }[] = [
  { key: 'image', label: '图片生成' },
  { key: 'video', label: '视频生成' },
  { key: 'reference', label: '参考图创作' },
]
const mediaContent: Record<MediaTabKey, { title: string; desc: string; tags: string[]; image: string }> = {
  image: {
    title: '文生图 / 图生图，风格尺寸随心选',
    desc: '支持 1:1、4:3、3:4、16:9、9:16 等尺寸，以及摄影、插画、3D 渲染、动漫、水墨等风格。默认模型 qwen-image-3.0，图生图编辑默认 qwen-image-3.0-pro；可切换 Threevideo 聚合、百炼 wanx、MiniMax、火山 seedance 等服务商。',
    tags: ['1024×1024 默认', '5 种画幅', '5 类风格', '多服务商'],
    image: '/media/DSH2.png',
  },
  video: {
    title: '文生视频，让想法动起来',
    desc: '支持 16:9、9:16、1:1 画幅；界面预设 3–10 秒，底层最高支持 30 秒（视模型而定）。可选 MiniMax H3 等模型与 480P / 768P / 2K 分辨率，wan3.0-video 支持参考视频编辑。',
    tags: ['16:9 / 9:16 / 1:1', '预设 3–10 秒 · 最高 30 秒', '480P / 768P / 2K'],
    image: '/media/DSH4.png',
  },
  reference: {
    title: '参考图驱动，保留你想要的画面',
    desc: '上传参考图可实现图生图、图生视频或视频编辑，保留原画面中的人物、动作与构图，只替换需要变化的部分；也支持续写、尾帧与真人开关，最多可传 6 张参考图。',
    tags: ['图生图', '图生视频', '视频编辑', '续写 / 尾帧'],
    image: '/media/DSH3.png',
  },
}
const activeTabContent = computed(() => mediaContent[activeMediaTab.value])

const mediaHighlights = [
  { icon: '🖼️', title: '结果就在对话里', desc: '生成的图片与视频直接显示在会话中，可定位到工作区文件。' },
  { icon: '🎞️', title: '直接送入工作台', desc: '对话媒体卡片可发送到工作台或放入时间轴继续编辑。' },
  { icon: '📁', title: '统一输出目录', desc: '生成内容保存到工作区 outputs 目录，方便后续继续使用。' },
]

const extraCapabilities = [
  { icon: '🔌', title: 'MCP 工具生态', desc: '支持 stdio 与 streamable-http 传输，可接入外部工具服务；默认关闭，按需在配置中启用。' },
  { icon: '🌐', title: '浏览器自动化', desc: '可接入 Playwright、Chrome DevTools 等 provider，让 Agent 在网页里执行操作（实验性能力）。' },
  { icon: '🖥️', title: '桌面操作', desc: '通过 CUA 驱动执行桌面级操作，覆盖需要图形界面的任务（实验性能力）。' },
  { icon: '🛡️', title: '分层安全机制', desc: '只读 / 工作区写入 / 完全访问三档沙箱，危险操作需确认并留下审批记录。' },
  { icon: '💻', title: '三平台客户端', desc: '提供 Windows、macOS 与 Linux（AppImage / deb）安装包，桌面体验一致。' },
  { icon: '🔁', title: '稳定与自愈', desc: '多层重试覆盖瞬时失败：请求、模型调用与工作流节点均有重试策略。' },
]

const scenes = [
  { title: '开发者', desc: '把重复的开发任务交给 Agent，专注在真正需要判断的工作上。', image: '/media/DSH1-clean.png' },
  { title: '内容创作', desc: '用文字生成插画与短片，再进时间轴精剪成完整作品。', image: '/media/DSH2.png' },
  { title: '产品与运营', desc: '在同一工作区里准备配图、演示素材与多比例版本。', image: '/media/DSH4.png' },
  { title: '团队协作', desc: '共享邀请链接与模型配置，让成员快速进入同一套工作流。', image: '/media/DSH3.png' },
]

const faqs = [
  { q: '需要自己准备 API Key 吗？', a: '不需要。使用 Threevideo 账号登录后，应用会自动创建或复用 API Key，并完成模型路由与默认模型配置。' },
  { q: '在哪里切换模型？', a: '输入框右侧的模型选择器，或设置中的「模型」页；侧边栏账户胶囊展示的是余额与 API Key 状态。' },
  { q: '可以使用哪些模型？', a: '可以使用 Threevideo 账户下可用的模型。应用会在登录时载入模型列表；底层也支持 DeepSeek、OpenAI 兼容与 Anthropic 等多种提供方。' },
  { q: '能生成视频吗？最长多久？', a: '可以。界面预设 3–10 秒，底层最高支持 30 秒，实际以所选模型为准；也支持以参考图驱动生成、编辑或续写视频。' },
  { q: '什么是创作工作台？', a: '右侧的创作区，提供快速创作、专业工作流画布与时间轴精剪三种模式。可以把剧本拆成分镜一键成片，也可以用节点画布编排流程、在时间轴里加字幕和配音。' },
  { q: '怎么收费？', a: '通过 Threevideo 账户余额结算。文本用量参考约 1$ ≈ 30M token，图片与视频按所选模型单独计价。价格为参考值，可能变动，请以账户内实际显示为准。' },
  { q: '是开源的吗？', a: '底层的 DeepSeek Harness 由 DeepSeek AI 以 MIT 许可证开源。本产品是在其上加入 Threevideo 集成与创作工作台的桌面版本，处于开发者预览阶段。' },
  { q: '支持哪些操作系统？', a: '提供 Windows、macOS 与 Linux 安装包（Linux 包含 AppImage 与 deb）。' },
  { q: '使用安全吗？', a: '本产品会执行模型生成的代码与命令，并提供三档沙箱与危险操作确认机制以降低风险，但不构成隔离保证。使用前请阅读安全说明，并避免处理敏感凭据。' },
]

useSEO({
  title: siteName.value + ' - Deepseek Harness for Threevideo 桌面 AI 应用',
  description: '一次登录，写代码、生图、做视频。左侧对话交给 Agent 执行任务，右侧创作工作台完成快速创作、工作流编排与时间轴精剪。',
  keywords: 'Deepseek Harness,Threevideo,AI 桌面应用,编程 Agent,创作工作台,一键成片,AI 生图,AI 生视频,DeepSeek Harness,智能体框架',
  ogType: 'website',
  ogSiteName: siteName.value,
})
</script>
