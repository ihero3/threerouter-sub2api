const I18N = {
  zh: {
    nav_home:"首页", nav_models:"模型", nav_feat:"功能", nav_demo:"演示", nav_start:"快速开始", nav_download:"免费下载",
    hero_tag:"Deepseek Harness for Threerouter · AI 文生图 / 文生视频",
    hero_title1:"对话即创作",
    hero_title2:"在对话中生成图片与视频",
    hero_sub:'<b>Deepseek Harness for Threerouter</b> 为对话模型注册 <b>文生图 / 文生视频</b> 两个工具。模型在对话中自主决策调用，生成结果自动落地本地 <b>outputs文件夹</b>，图片内嵌渲染——<b>无需离开终端，无需切换窗口</b>。',
    hero_cta_win:"下载 Windows 版", hero_cta_mac:"下载 macOS 版", hero_cta_linux:"下载 Linux 版", hero_cta2:"快速开始",
    stat1:"个创作工具", stat2:"项配置，登录即用", stat3:"外部运行时依赖",
    app_cap:"在主界面中随时打字，模型自主调用 dsh-image-video",
    feat_h:"为什么是 dsh-image-video", feat_p:"轻量、即装即用的文生图 / 文生视频入口。两个工具、登录即用、零外部依赖。",
    v1_h:"自然语言触发", v1_p:"模型在对话中自主决策何时调用工具，无需手动指令。一句「帮我画…」即可创作。",
    v2_h:"登录 Threerouter 即用", v2_p:"无需配置任何 Provider，登录 Threerouter 账号即可使用全部模型，渠道由平台统一调度。",
    v3_h:"异步不阻塞对话", v3_p:"TaskManager 托管轮询生命周期，插件卸载自动取消排队任务、清理定时器，杜绝内存泄漏。",
    v4_h:"本地落地 + 内嵌渲染", v4_p:"图片字节持久化到 attachment，模型只见文本摘要，纯文本模型照常工作；UI 可内嵌渲染。",
    tools_h:"两个工具，覆盖图片与视频",
    badge_img:"同步 / 异步自适应", badge_vid:"始终异步轮询",
    row_cap:"能力", row_prov:"模型来源", row_exec:"执行", row_render:"渲染", prov_unified:"Threerouter 平台统一提供，登录即用",
    tool_img_cap:"文本生成图片（文生图）", tool_img_ex:"同步或异步（自动适配，纯文本模型照常工作）", tool_img_render:"图片经 presentationMeta 内嵌渲染；模型只见文本摘要",
    tool_vid_cap:"文本生成短视频（上限 10s）", tool_vid_ex:"始终异步轮询，不阻塞对话", tool_vid_render:"本地文件路径 + 源地址；后台轮询任务状态",
    show_h:"在对话里长出来的作品", show_p:"模型自主调用 dsh-image-video 生成的图像与视频示例，自动落地到本地 outputs文件夹，图片内嵌渲染。",
    show_img_h:"文生图 · generate_image", show_img_cap:"示例：", show_img_prompt:'"一只白色兔子坐在阳光明媚的花海中"',
    show_vid_h:"文生视频 · generate_video", show_vid_cap:"示例：", show_vid_prompt:'"穿着裙子的美女，在冰上花样表演"',
    prov_h:"登录即用，无需配置", prov_p:"无需申请或配置任何 Provider，登录 Threerouter 后即可使用全部模型能力。",
    s1_h:"1. 登录 Threerouter", s1_p:"使用你的 Threerouter 账号登录客户端，一次认证即可。",
    s2_h:"2. 选择模型", s2_p:"图像与视频模型由平台统一提供，无需单独申请各服务商 Key。",
    s3_h:"3. 开始创作", s3_p:"在对话中让模型生图 / 生视频，结果自动保存到本地。",
    demo_h:"点一下，看它如何创作", demo_p:"选择一个示例提示词，观察模型自主调用工具并生成结果。",
    demo_idle:"// 等待你选择一个示例提示词…", demo_p1:"白兔在花海", demo_p2:"美女滑冰视频", demo_p3:"星云中的宇航员",
    start_h:"一步开始创作", copy:"复制", faq_h:"常见问题",
    q1:"API Key 安全吗？会上传我的数据吗", a1:"API Key 仅在本地使用，不上传任何远程服务。生成通过你的 Threerouter 账号由平台统一调度完成。",
    q2:"生成质量由谁决定？", a2:"模型输出质量由平台提供的模型决定。插件负责正确传参、可靠轮询和统一异常分类，不改变模型本身的能力。",
    q3:"需要先安装 Deepseek Harness for Threerouter 吗？", a3:"dsh-image-video 是 Deepseek Harness for Threerouter 的插件，运行在 DSH 运行时中。使用桌面端可从 GUI 直接安装，也可通过 CLI 插件命令添加到任意 profile。",
    footer_p:"基于 Deepseek Harness 构建，面向 Threerouter 的桌面客户端。dsh-image-video 插件让模型在对话中自主生成图片与视频。",
    f_docs:"文档", f_guide:"项目说明", f_faq:"帮助文档", f_contact:"联系我们",
    f_dl:"下载", f_win:"Windows x64", f_mac:"macOS Universal", f_linux:"Linux x64 (AppImage)", f_rel:"GitHub Releases",
    demo_cmd:'dsh --profile headless "为你创作一张图…"',
    gs_cm:'<span class="cm"># 下载并安装 Deepseek Harness for Threerouter</span><br><span class="cm"># Windows: 运行 dsh-threerouter-0.1.6-alpha.2-win-x64.exe</span><br><span class="cm"># macOS:   运行 dsh-threerouter-0.1.6-alpha.2-universal.dmg</span><br><span class="cm"># Linux:   运行 dsh-threerouter-0.1.6-alpha.2-linux-x64.AppImage（Ubuntu / Debian / Mint / Deepin / UOS / Fedora）</span>',
    legal:"Deepseek Harness for Threerouter 是独立的社区开源项目，与深度求索不存在隶属、合作、授权或背书关系。dsh-image-video 为 MIT 协议开源，生成能力由 Threerouter 平台统一提供。"
  },
  en: {
    nav_home:"Home", nav_models:"Models", nav_feat:"Features", nav_demo:"Demo", nav_start:"Get Started", nav_download:"Download",
    hero_tag:"Deepseek Harness for Threerouter · AI Text-to-Image / Text-to-Video",
    hero_title1:"Create in conversation",
    hero_title2:"Images & videos, right in chat",
    hero_sub:'<b>Deepseek Harness for Threerouter</b> registers two tools — <b>generate_image</b> / <b>generate_video</b> — for the model. The model calls them on its own mid-conversation; results land in local <b>outputs/</b> and images render inline — <b>no leaving the terminal, no switching windows</b>.',
    hero_cta_win:"Download for Windows", hero_cta_mac:"Download for macOS", hero_cta_linux:"Download for Linux", hero_cta2:"Get Started",
    stat1:"creating tools", stat2:"configs — just sign in", stat3:"external runtime deps",
    app_cap:"Type anytime in the main window — the model calls dsh-image-video on its own",
    feat_h:"Why dsh-image-video", feat_p:"A lightweight, plug-and-play entry for text-to-image and text-to-video. Two tools, sign in once, zero extra dependencies.",
    v1_h:"Natural-language trigger", v1_p:"The model decides when to call the tool mid-conversation — no manual command. Just say “draw me…”.",
    v2_h:"Sign in to Threerouter and go", v2_p:"sign in with your Threerouter account and every model is ready. Channels are managed by the platform.",
    v3_h:"Async, non-blocking", v3_p:"TaskManager manages the polling lifecycle; unloading the plugin cancels queued tasks and timers to avoid leaks.",
    v4_h:"Local files + inline render", v4_p:"Image bytes persist to attachments; the model only sees a text summary, so text-only models keep working while UI renders inline.",
    tools_h:"Two tools for images and video",
    badge_img:"Sync / async adaptive", badge_vid:"Always async polling",
    row_cap:"Capability", row_prov:"Models", row_exec:"Execution", row_render:"Render", prov_unified:"Provided by the Threerouter platform — just sign in",
    tool_img_cap:"Generate images from text", tool_img_ex:"Sync or async (auto-adapts; text-only models keep working)", tool_img_render:"Rendered inline via presentationMeta; model sees only a text summary",
    tool_vid_cap:"Generate short videos (up to 10s)", tool_vid_ex:"Always async polling, never blocks the conversation", tool_vid_render:"Local file path + source URL; polls task status in background",
    show_h:"Work grown inside a conversation", show_p:"Sample images and videos generated by the model calling dsh-image-video, saved automatically to local outputsand rendered inline.",
    show_img_h:"Text-to-image · generate_image", show_img_cap:"Example:", show_img_prompt:'"A white rabbit sitting in a sunlit field of flowers"',
    show_vid_h:"Text-to-video · generate_video", show_vid_cap:"Example:", show_vid_prompt:'"A beautiful woman in a dress performing figure skating on ice"',
    prov_h:"Sign in once — no provider setup", prov_p:"No provider keys to apply for or configure. Sign in to Threerouter and all model capabilities are ready.",
    s1_h:"1. Sign in to Threerouter", s1_p:"Sign in to the client with your Threerouter account — one-time auth.",
    s2_h:"2. Pick a model", s2_p:"Image and video models are provided by the platform — no separate provider keys needed.",
    s3_h:"3. Start creating", s3_p:"Ask the model to generate images or videos in chat; results save to local outputs/ automatically.",
    demo_h:"Click to see it create", demo_p:"Pick a sample prompt and watch the model call the tool and produce a result.",
    demo_idle:"// Waiting for you to pick a sample prompt…", demo_p1:"Rabbit in flowers", demo_p2:"Figure skating video", demo_p3:"Astronaut in a nebula",
    start_h:"Start creating in one steps", copy:"Copy", faq_h:"FAQ",
    q1:"Is my API key safe? Do you upload my data?", a1:"The API key is used only locally and never uploaded anywhere. Generation is dispatched by the platform through your Threerouter account.",
    q2:"Who decides generation quality?", a2:"Quality depends on the models provided by the platform. The plugin handles correct params, reliable polling and unified error classification — it doesn't change the model itself.",
    q3:"Do I need Deepseek Harness for Threerouter first?", a3:"dsh-image-video is a plugin of Deepseek Harness for Threerouter, running inside the DSH runtime. Install it from the desktop GUI, or add it to any profile via the CLI plugin command.",
    footer_p:"A desktop client for Threerouter built on Deepseek Harness. The dsh-image-video plugin lets the model generate images and videos on its own in conversation.",
    f_docs:"Docs", f_guide:"Project readme", f_faq:"Help", f_contact:"Contact",
    f_dl:"Download", f_win:"Windows x64", f_mac:"macOS Universal", f_linux:"Linux x64 (AppImage)", f_rel:"GitHub Releases",
    demo_cmd:'dsh --profile headless "create an image for you…"',
    gs_cm:'<span class="cm"># Download and install Deepseek Harness for Threerouter</span><br><span class="cm"># Windows: run dsh-threerouter-0.1.6-alpha.2-win-x64.exe</span><br><span class="cm"># macOS:   run dsh-threerouter-0.1.6-alpha.2-universal.dmg</span><br><span class="cm"># Linux:   run dsh-threerouter-0.1.6-alpha.2-linux-x64.AppImage (Ubuntu / Debian / Mint / Deepin / UOS / Fedora)</span>',
    legal:"Deepseek Harness for Threerouter is an independent community open-source project with no affiliation, partnership, endorsement or authorization from DeepSeek. dsh-image-video is MIT-licensed. Generation capabilities are provided by the Threerouter platform."
  }
};
let lang = localStorage.getItem('dsh-lang') || 'zh';

function applyLang() {
  document.documentElement.lang = lang === 'zh' ? 'zh-CN' : 'en';
  document.querySelectorAll('[data-i18n]').forEach(el => {
    const k = el.getAttribute('data-i18n');
    if (I18N[lang][k] != null) el.innerHTML = I18N[lang][k];
  });
  document.querySelectorAll('.demo-trigger').forEach(el => {
    const k = el.getAttribute('data-key');
    if (I18N[lang][k]) el.textContent = I18N[lang][k];
  });
  document.getElementById('lang-zh').classList.toggle('active', lang === 'zh');
  document.getElementById('lang-en').classList.toggle('active', lang === 'en');
  localStorage.setItem('dsh-lang', lang);
}
document.getElementById('lang-zh').addEventListener('click', () => { lang = 'zh'; applyLang(); });
document.getElementById('lang-en').addEventListener('click', () => { lang = 'en'; applyLang(); });

const io = new IntersectionObserver(entries => entries.forEach(e => {
  if (e.isIntersecting) { e.target.classList.add('in'); io.unobserve(e.target); }
}), { threshold: .12 });
document.querySelectorAll('.reveal').forEach(el => io.observe(el));

document.querySelectorAll('.copy').forEach(btn => {
  btn.addEventListener('click', () => {
    const t = document.getElementById(btn.getAttribute('data-copy-target'));
    navigator.clipboard.writeText(t.innerText).then(() => {
      btn.textContent = lang === 'zh' ? '已复制' : 'Copied';
      setTimeout(() => { btn.textContent = I18N[lang].copy; }, 1400);
    });
  });
});

const demoOut = document.getElementById('demo-out');
const imgSrcMap = { demo_p1: 'media/DSH2.png', demo_p3: 'media/DSH4.png' };
const frameStyle = 'margin-top:12px;border-radius:10px;max-width:340px;display:block;box-shadow:0 12px 30px rgba(0,0,0,.5)';
const vidFrame = '<video src="media/dsh.mp4" controls playsinline preload="metadata" poster="media/DSH3.png" style="' + frameStyle + '"></video>';
document.querySelectorAll('.demo-trigger').forEach(btn => {
  btn.addEventListener('click', () => {
    const sample = btn.getAttribute('data-sample');
    const pLabel = btn.textContent.trim();
    const tool = sample === 'vid' ? 'generate_video' : 'generate_image';
    const during = lang === 'zh' ? '// 正在生成…' : '// generating…';
    const done = lang === 'zh' ? '✓ 已保存到 outputs/' : '✓ saved to outputs/';
    demoOut.innerHTML = '<span class="tok">> ' + tool + '</span> { prompt:"' + pLabel + '" }\n'
      + '<span class="cm">' + during + '</span>';
    setTimeout(() => {
      const imgFrame = '<img src="' + (imgSrcMap[btn.getAttribute('data-key')] || 'media/DSH2.png') + '" alt="example" style="' + frameStyle + '">';
      demoOut.innerHTML += '\n<span class="tok">' + done + '</span>outputs/sample.' + (sample === 'vid' ? 'mp4' : 'png') + '\n'
        + (sample === 'vid' ? vidFrame : imgFrame);
      demoOut.scrollTop = demoOut.scrollHeight;
    }, 1100);
  });
});

applyLang();
