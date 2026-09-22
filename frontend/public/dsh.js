const I18N = {
  zh: {
    nav_home:"首页", nav_models:"模型", nav_feat:"功能", nav_demo:"演示", nav_start:"快速开始", nav_download:"免费下载",
    hero_tag:"Deepseek Harness for Threerouter · AI 文生图 / 文生视频",
    hero_title1:"对话即创作",
    hero_title2:"在对话中生成图片与视频",
    hero_sub:'<b>Deepseek Harness for Threerouter</b> 为对话模型注册 <b>generate_image</b> / <b>generate_video</b> 两个工具。模型在对话中自主决策调用，生成结果自动落地本地 <b>outputs文件夹</b>，图片内嵌渲染——<b>无需离开终端，无需切换窗口</b>。',
    hero_cta_mac:"下载 macOS 版", hero_cta2:"快速开始",
    stat1:"个创作工具", stat2:"家服务商热切换", stat3:"外部运行时依赖",
    app_cap:"在主界面中随时打字，模型自主调用 dsh-image-video",
    feat_h:"为什么是 dsh-image-video", feat_p:"轻量、即装即用的文生图 / 文生视频入口。两个工具、三家服务商、零外部依赖，热插拔即用。",
    v1_h:"自然语言触发", v1_p:"模型在对话中自主决策何时调用工具，无需手动指令。一句「帮我画…」即可创作。",
    v2_h:"多服务商热切换", v2_p:"万象 wanx、Seedance2.5、bxinle 一键切换 provider，HMR 即时生效，无需重启。",
    v3_h:"异步不阻塞对话", v3_p:"TaskManager 托管轮询生命周期，插件卸载自动取消排队任务、清理定时器，杜绝内存泄漏。",
    v4_h:"本地落地 + 内嵌渲染", v4_p:"图片字节持久化到 attachment，模型只见文本摘要，纯文本模型照常工作；UI 可内嵌渲染。",
    tools_h:"两个工具，覆盖图片与视频",
    badge_img:"同步 / 异步自适应", badge_vid:"始终异步轮询",
    row_cap:"能力", row_prov:"服务商", row_exec:"执行", row_render:"渲染",
    tool_img_cap:"文本生成图片（文生图）", tool_img_ex:"同步或异步（自动适配，纯文本模型照常工作）", tool_img_render:"图片经 presentationMeta 内嵌渲染；模型只见文本摘要",
    tool_vid_cap:"文本生成短视频（上限 10s）", tool_vid_ex:"始终异步轮询，不阻塞对话", tool_vid_render:"本地文件路径 + 源地址；后台轮询任务状态",
    show_h:"在对话里长出来的作品", show_p:"模型自主调用 dsh-image-video 生成的图像与视频示例，自动落地到本地 outputs文件夹，图片内嵌渲染。",
    show_img_h:"文生图 · generate_image", show_img_cap:"示例：", show_img_prompt:'"一只白色兔子坐在阳光明媚的花海中"',
    show_vid_h:"文生视频 · generate_video", show_vid_cap:"示例：", show_vid_prompt:'"花瓣随风飘落，镜头缓缓推进"',
    prov_h:"服务商矩阵", prov_p:"三家中立服务商，一个 provider 字段一键切换。",
    th_prov:"服务商", th_channel:"渠道", th_img:"文生图", th_vid:"文生视频", th_note:"备注",
    demo_h:"点一下，看它如何创作", demo_p:"选择一个示例提示词，观察模型自主调用工具并生成结果。",
    demo_idle:"// 等待你选择一个示例提示词…", demo_p1:"白兔在花海", demo_p2:"花瓣飘落视频", demo_p3:"星云中的宇航员",
    start_h:"一步开始创作", copy:"复制", faq_h:"常见问题",
    q1:"API Key 安全吗？会上传我的数据吗", a1:"API Key 仅在本地使用，不上传任何远程服务，本插件不附属于任何云厂商。生成由你配置的服务商模型完成，调用前请确认各服务商的服务条款。",
    q2:"生成质量由谁决定？", a2:"模型输出质量由服务商模型决定。插件负责正确传参、可靠轮询和统一异常分类，不改变模型本身的能力。",
    q3:"需要先安装 Deepseek Harness for Threerouter 吗？", a3:"dsh-image-video 是 Deepseek Harness for Threerouter 的插件，运行在 DSH 运行时中。使用桌面端可从 GUI 直接安装，也可通过 CLI 插件命令添加到任意 profile。",
    q4:"图片为什么不用 image 内容块给模型？", a4:"纯文本模型或网关不支持图片输入时，把图片作为 image 内容块注入工具结果会让下一轮请求直接 400。插件只向模型返回文本摘要，图片经 presentationMeta 持久化交给 UI 内嵌渲染。",
    footer_p:"基于 Deepseek Harness 构建，面向 Threerouter 的桌面客户端。dsh-image-video 插件让模型在对话中自主生成图片与视频。",
    f_docs:"文档", f_guide:"项目说明", f_faq:"帮助文档", f_contact:"联系我们",
    f_dl:"下载", f_win:"Windows x64", f_mac:"macOS Universal", f_rel:"GitHub Releases",
    tbl_dflt:"默认", tbl_adapter_ready:"适配器已实现", tbl_verified:"已验证",
    prov_wanx:"万象 wanx",
    tbl_bailian:"阿里云百炼", tbl_ark:"火山引擎 Ark",
    tbl_bxinle_notes:"统一 /v1/videos 端点；provider=bxinle 时图片回退到 wanx",
    tbl_wanx_notes:"wanx2.1-t2i-turbo / wan2.2-t2v-plus；视频始终异步",
    tbl_seedance_notes:"即梦 3.0；图片同步返回 URL，视频走异步任务",
    demo_cmd:'dsh --profile headless "为你创作一张图…"',
    gs_cm:'<span class="cm"># 下载并安装 Deepseek Harness for Threerouter</span><br><span class="cm"># Windows: 运行 DSH-Desktop-2.0.3-x64-Setup.exe</span><br><span class="cm"># macOS:   运行 DSH.Desktop-2.0.3-universal.dmg</span>',
    legal:"Deepseek Harness for Threerouter 是独立的社区开源项目，与深度求索不存在隶属、合作、授权或背书关系。dsh-image-video 为 MIT 协议开源，不附属于阿里云或火山引擎。使用各服务商 API 前请确认其服务条款。"
  },
  en: {
    nav_home:"Home", nav_models:"Models", nav_feat:"Features", nav_demo:"Demo", nav_start:"Get Started", nav_download:"Download",
    hero_tag:"Deepseek Harness for Threerouter · AI Text-to-Image / Text-to-Video",
    hero_title1:"Create in conversation",
    hero_title2:"Images & videos, right in chat",
    hero_sub:'<b>Deepseek Harness for Threerouter</b> registers two tools — <b>generate_image</b> / <b>generate_video</b> — for the model. The model calls them on its own mid-conversation; results land in local <b>outputs/</b> and images render inline — <b>no leaving the terminal, no switching windows</b>.',
    hero_cta_mac:"Download for macOS", hero_cta2:"Get Started",
    stat1:"creating tools", stat2:"hot-swappable providers", stat3:"external runtime deps",
    app_cap:"Type anytime in the main window — the model calls dsh-image-video on its own",
    feat_h:"Why dsh-image-video", feat_p:"A lightweight, plug-and-play entry for text-to-image and text-to-video. Two tools, three providers, zero extra dependencies.",
    v1_h:"Natural-language trigger", v1_p:"The model decides when to call the tool mid-conversation — no manual command. Just say “draw me…”.",
    v2_h:"Hot-swap providers", v2_p:"Switch provider between Wanx, Seedance2.5 and bxinle with one field. HMR takes effect instantly, no restart.",
    v3_h:"Async, non-blocking", v3_p:"TaskManager manages the polling lifecycle; unloading the plugin cancels queued tasks and timers to avoid leaks.",
    v4_h:"Local files + inline render", v4_p:"Image bytes persist to attachments; the model only sees a text summary, so text-only models keep working while UI renders inline.",
    tools_h:"Two tools for images and video",
    badge_img:"Sync / async adaptive", badge_vid:"Always async polling",
    row_cap:"Capability", row_prov:"Provider", row_exec:"Execution", row_render:"Render",
    tool_img_cap:"Generate images from text", tool_img_ex:"Sync or async (auto-adapts; text-only models keep working)", tool_img_render:"Rendered inline via presentationMeta; model sees only a text summary",
    tool_vid_cap:"Generate short videos (up to 10s)", tool_vid_ex:"Always async polling, never blocks the conversation", tool_vid_render:"Local file path + source URL; polls task status in background",
    show_h:"Work grown inside a conversation", show_p:"Sample images and videos generated by the model calling dsh-image-video, saved automatically to local outputsand rendered inline.",
    show_img_h:"Text-to-image · generate_image", show_img_cap:"Example:", show_img_prompt:'"A white rabbit sitting in a sunlit field of flowers"',
    show_vid_h:"Text-to-video · generate_video", show_vid_cap:"Example:", show_vid_prompt:'"Petals drift on the wind as the camera pushes in"',
    prov_h:"Provider matrix", prov_p:"Three neutral providers, one provider field to switch.",
    th_prov:"Provider", th_channel:"Channel", th_img:"Image", th_vid:"Video", th_note:"Notes",
    demo_h:"Click to see it create", demo_p:"Pick a sample prompt and watch the model call the tool and produce a result.",
    demo_idle:"// Waiting for you to pick a sample prompt…", demo_p1:"Rabbit in flowers", demo_p2:"Falling petals video", demo_p3:"Astronaut in a nebula",
    start_h:"Start creating in one steps", copy:"Copy", faq_h:"FAQ",
    q1:"Is my API key safe? Do you upload my data?", a1:"The API key is used only locally and never uploaded anywhere. This plugin is not affiliated with any cloud vendor. Generation runs on the provider you configure — review each provider's terms first.",
    q2:"Who decides generation quality?", a2:"Quality depends on the provider's model. The plugin handles correct params, reliable polling and unified error classification — it doesn't change the model itself.",
    q3:"Do I need Deepseek Harness for Threerouter first?", a3:"dsh-image-video is a plugin of Deepseek Harness for Threerouter, running inside the DSH runtime. Install it from the desktop GUI, or add it to any profile via the CLI plugin command.",
    q4:"Why not send the image as an image block?", a4:"If the text-only model or gateway can't take image input, injecting an image content block makes the next request return 400. The plugin returns only a text summary and persists the image via presentationMeta for the UI to render.",
    footer_p:"A desktop client for Threerouter built on Deepseek Harness. The dsh-image-video plugin lets the model generate images and videos on its own in conversation.",
    f_docs:"Docs", f_guide:"Project readme", f_faq:"Help", f_contact:"Contact",
    f_dl:"Download", f_win:"Windows x64", f_mac:"macOS Universal", f_rel:"GitHub Releases",
    tbl_dflt:"default", tbl_adapter_ready:"Adapter ready", tbl_verified:"Verified",
    prov_wanx:"wanx",
    tbl_bailian:"Alibaba Cloud Bailian", tbl_ark:"Volcano Engine Ark",
    tbl_bxinle_notes:"Unified /v1/videos endpoint; falls back to wanx for images when provider=bxinle",
    tbl_wanx_notes:"wanx2.1-t2i-turbo / wan2.2-t2v-plus; video is always async",
    tbl_seedance_notes:"Jimeng 3.0; images return URL synchronously, videos run as async tasks",
    demo_cmd:'dsh --profile headless "create an image for you…"',
    gs_cm:'<span class="cm"># Download and install Deepseek Harness for Threerouter</span><br><span class="cm"># Windows: run DSH-Desktop-2.0.3-x64-Setup.exe</span><br><span class="cm"># macOS:   run DSH.Desktop-2.0.3-universal.dmg</span>',
    legal:"Deepseek Harness for Threerouter is an independent community open-source project with no affiliation, partnership, endorsement or authorization from DeepSeek. dsh-image-video is MIT-licensed and not affiliated with Alibaba Cloud or Volcano Engine. Review each provider's terms before use."
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
