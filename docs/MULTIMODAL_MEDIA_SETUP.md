# 多模态媒体接入配置指南（视频 / 图片 / 音频）

本文档基于当前代码实现整理，说明如何通过 Sub2API / ThreeRouter 管理后台接入视频、图片、音频厂商，并为用户提供统一的多模态生成能力。

## 一、整体架构

用户统一通过以下入口调用（**推荐前三个**，任何分组下响应结构一致）：

| 端点 | 能力 |
|---|---|
| `POST /v1/images/generations`、`/v1/images/edits` | 生图（OpenAI 标准结构，同步出图） |
| `POST /v1/videos/generations`、`/v1/videos` | 生视频（异步任务；`?wait=N` 可同步等待，上限 180 秒） |
| `POST /v1/chat/completions` | 文本（收到媒体模型时自动转对应链路） |
| `POST /v1/audio/speech` | 文本 → 音频（同步返回音频字节） |
| `POST /v1/audio/transcriptions` | 语音 → 文本 |
| `POST /v1/audio/translations` | 语音 → 翻译 |
| `GET /v1/media/:id`、`/content` | 查询 / 下载产物 |

兼容保留（不推荐新接入）：`POST /v1/media`、`/v1/media/generations`（媒体总入口，自研响应
结构）、`POST /v1/generations`（万能入口）、`POST /v1/video-tasks`（**只读兼容，不再维护**，
历史任务仍可查）、`POST /v1/images/generations/async`（异步生图）、`POST /v1/images/batches`
（批量生图，并列能力非降级）。

> 内部实现上，视频与图片都收敛到 `MediaTaskService`（`media_tasks` 表），
> 由 `MediaWorker` 轮询、`media_adapter_bridge.go` 复用各厂商 adapter。

调用流程：请求 → 额度检查 → 按模型名识别类型（`MediaKindFromModel`）→ 选上游账号（同类型多账号自动轮转，最多 100 次）→ 调厂商 adapter → 异步轮询（`MediaWorker`）+ 超时保护 → 完成/失败结算。

## 二、账号配置通用步骤

每个厂商都复用 `Account`（账号）体系，无独立"媒体源"实体。

1. 后台 → 账号管理 → 新建账号
2. 平台选 `deepseek`（或 `composite`），类型 `apikey`
3. 填写 `base_url`（见下方厂商表）+ `api_key`
4. 配置 `model_mapping`（把用户约定模型名映射到厂商实际模型名）
5. 加入分组（`concrete` 或 `composite`）→ 调度器自动按模型名选号

> 模型白名单 / `model_mapping` 决定该账号承接哪些模型。**必须配置 `model_mapping`**，否则调度器可能误选普通文本账号。

## 三、各厂商配置明细

### 视频厂商

#### Seedance（火山方舟）
| 项 | 值 |
|---|---|
| 创建端点 | `POST /api/v3/contents/generations/tasks` |
| 查询端点 | `GET /api/v3/contents/generations/tasks/{id}` |
| 取消端点 | `DELETE /api/v3/contents/generations/tasks/{id}` |
| 适配模型 | 含 `seedance`、`doubao-seedance`、`jimeng-video` |

**配置**：
- `base_url`: `https://ark.cn-beijing.volces.com`
- `api_key`: 火山方舟 API Key
- `model_mapping`: `{"seedance-2.5": "doubao-seedance-2-5"}`
- 计价 family: `seedance`

#### MiniMax（H3 视频）
| 项 | 值 |
|---|---|
| 创建端点 | `POST /v2/video_generation` |
| 查询端点 | `GET /v2/query/video_generation/{id}` |
| 取消端点 | `DELETE /v2/video_generation/{id}` |
| 适配模型 | `minimax-hailuo`、`minimax-video`、`minimax-h3` |

**配置**：
- `base_url`: `https://api.minimax.cn`
- `api_key`: MiniMax API Key
- `model_mapping`: `{"minimax-h3": "MiniMax-H3"}`
- 计价 family: `minimax-video`

#### Wan（阿里 DashScope）
| 项 | 值 |
|---|---|
| 创建端点 | `POST /api/v1/services/aigc/video-generation/video-synthesis` |
| 查询端点 | `GET /api/v1/tasks/{id}` |
| 取消端点 | `DELETE /api/v1/tasks/{id}` |
| 适配模型 | `wan*` + `video/t2v/wan2/wan3`（**不含 `wanx*`，那是图像系列，走 Wan 图片链路**） |

**配置**：
- `base_url`: `https://<WorkspaceId>.cn-beijing.maas.aliyuncs.com`（必须带 WorkspaceId，不同地域不同）
- `api_key`: 阿里百炼 API Key
- `model_mapping`: `{"wan3.0-video": "wan3.0-video"}`
- 计价 family: `wan-video`

### 图片厂商

#### Seedance 图片（Seedream）
| 项 | 值 |
|---|---|
| 端点 | `POST /api/v3/images/generations` |
| 适配模型 | `seedream`、`doubao-seedream`、`seedance-image`、`jimeng-image` |

**配置**：
- `base_url`: `https://ark.cn-beijing.volces.com`
- `model_mapping`: `{"seedream-5.0": "doubao-seedream-4-0"}`

#### Wan 图片（t2i）
| 项 | 值 |
|---|---|
| 端点 | `POST /api/v1/services/aigc/multimodal-generation/generation` |
| 适配模型 | `wan2*`、`wanx*`、`qwen-image`、`t2i` |

**配置**：
- `base_url`: `https://<WorkspaceId>.cn-beijing.maas.aliyuncs.com`
- `model_mapping`: `{"wan2.6-t2i": "wan2.6-t2i"}`
- 请求体：`input.messages` + `parameters{size, n, prompt_extend}`

#### MiniMax 图片
| 项 | 值 |
|---|---|
| 端点 | `POST /v1/image_generation` |
| 适配模型 | `image-01`、`minimax-image`、`hailuo-image` |

**配置**：
- `base_url`: `https://api.minimax.cn`
- `model_mapping`: `{"image-01": "image-01"}`
- 请求体：`prompt` + `aspect_ratio` 或 `width/height`

### 音频厂商

#### MiniMax TTS（同步语音合成 T2A v2）
| 项 | 值 |
|---|---|
| 对外端点 | `POST /v1/audio/speech`（OpenAI 兼容，同步返回 `audio/mpeg` 音频字节） |
| 上游端点 | `POST /v1/t2a_v2` |
| 适配模型 | `speech-2.8-hd`、`speech-2.8-turbo`、`speech-2.6-hd`、`speech-2.6-turbo`、`speech-02-hd`、`speech-02-turbo`、`speech-01-hd`、`speech-01-turbo`；兼容旧别名 `minimax-tts`、`speech-01`、`tts` |
| 参数 | OpenAI 风格 `input`（文本）、`voice`、`speed`、`response_format`（`mp3` / `flac` / `pcm`）；MiniMax 原生 `voice_setting`、`audio_setting`、`pronunciation_dict`、`language_boost`、`text_normalization_mode` 等原样透传 |
| 返回 | 默认 `output_format=hex`，服务端解码后直接返回音频字节；显式传 `output_format=url` 时 302 到 24h 有效下载链接 |

**配置**：
- `base_url`: `https://api.minimaxi.com`（也可用文档默认 `https://api.minimax.cn`、华北 `https://api-bj.minimaxi.com`）
- `model_mapping`: 可选，用于把对外模型名映射到上游模型名

> 计费沿用分组音频价（按秒 / 百万字符），不按 `speech-*` 模型分别定价。
> 本端点只支持同步 HTTP；WebSocket / 双向流式暂不支持，请求里的 `stream` 会被强制为 `false`。
> `response_format` 指定 `flac` / `pcm` 时字节格式正确，但响应 `Content-Type` 目前固定 `audio/mpeg`，需客户端自行处理。

#### MiniMax 音色复刻 / 音色查询 / 异步长文本 TTS（原生兼容）

路径、请求体与 MiniMax 官方文档一致；把 `base_url` 换成本网关、`Authorization` 换成网关 API Key 即可。

| 能力 | 网关端点 | 上游端点 |
|---|---|---|
| 上传复刻/示例音频、异步文本文件 | `POST /v1/files/upload` | `POST /v1/files/upload` |
| 音色快速复刻 | `POST /v1/voice_clone` | `POST /v1/voice_clone` |
| 查询系统音色 + 复刻音色 | `POST /v1/get_voice` | `POST /v1/get_voice` |
| 创建异步语音合成任务 | `POST /v1/t2a_async_v2` | `POST /v1/t2a_async_v2` |
| 查询异步任务 | `GET /v1/query/t2a_async_query_v2?task_id=` | 同名 |
| 下载异步产物 | `GET /v1/files/retrieve_content?file_id=` | 同名 |

**账号粘性与 ID 规则**：
- `file_id` / `task_id` **保持上游原始数字**（官方 SDK 可按 int64 解析）。网关在 Redis 里保存「数字 ID → 上游账号 / 用户 / 计费上下文」的 7 天映射；查询、下载必须由同一用户发起。
- 复刻成功后返回的 `voice_id` 是**网关音色 ID**（`mmxvc_` 前缀），不是上游 voice_id。把它传给 `/v1/audio/speech` 的 `voice` 或 `/v1/t2a_async_v2` 的 `voice_setting.voice_id`，网关会解出账号并固定打回同一上游账号，避免多账号轮转导致「音色不存在」。响应同时带 `upstream_voice_id` 供排障。
- `POST /v1/get_voice` 的复刻/生成音色同样会被包装成网关音色 ID；系统音色保持原值，可直接使用。
- `/v1/files/upload` 额外接受 `model` 字段用于选账号（原生文档没有该字段；缺省按常用 T2A 模型回退）。`purpose` 支持 `voice_clone`、`prompt_audio`、`t2a_async_input`。
- `clone_prompt.prompt_audio` 与 `file_id` 必须来自本网关上传，且属于同一上游账号。

**计费**：
- 音色复刻本身不计费；带 `text` 试听时按 `extra_info.usage_characters` 计费（分组「TTS 每百万字符价格」）。
- 异步 TTS 在**创建任务成功时**按上游返回的 `usage_characters` 计费，避免「只提交不查询」让网关单方面承担上游成本；查询成功时用同一稳定 request id 再结算一次是幂等的。失败/过期任务目前不自动退款。
- 复刻音色若 7 天内未被正式调用，上游会自动删除（MiniMax 规则）。

> 以上能力依赖 Redis（归属映射）。Redis 不可用时返回 `base_resp.status_code=1000`，不会把请求打到不确定的账号。
> WebSocket 实时 TTS 与文生音色（voice generation）管理接口仍未中转；`/v1/get_voice` 可查询到已有音色。

#### 火山 TTS
| 项 | 值 |
|---|---|
| 端点 | `POST /api/v3/tts` |
| 适配模型 | `volc-tts`、`volcano-tts`、`doubao-tts` |

**配置**：
- `base_url`: `https://ark.cn-beijing.volces.com`
- `model_mapping`: `{"volc-tts": "<火山音色ID>"}`

#### 阿里 DashScope TTS（`qwen-tts` 旧协议）
| 项 | 值 |
|---|---|
| 端点 | `POST /api/v1/services/aigc/multimodal-generation/generation` |
| 适配模型 | `aliyun-tts`、`dashscope-tts`、`qwen-tts`（旧版千问-TTS） |

**配置**：
- `base_url`: `https://<WorkspaceId>.cn-beijing.maas.aliyuncs.com`
- `model_mapping`: `{"qwen-tts": "qwen-tts"}`

> CosyVoice 已不在本协议：`cosyvoice-*` 统一交给下面的「千问 Qwen TTS」，走
> `SpeechSynthesizer` 端点（官方非实时文档的现行链路）。

#### 千问 Qwen TTS（非实时语音合成 / 音频生成）
| 项 | 值 |
|---|---|
| 对外端点 | `POST /v1/audio/speech`（OpenAI 兼容，同步返回音频字节） |
| 上游端点 | Qwen-Audio-TTS / CosyVoice / tts-next：`POST /api/v1/services/audio/tts/SpeechSynthesizer`；Qwen3-TTS：`POST /api/v1/services/aigc/multimodal-generation/generation` |
| 适配模型 | `qwen-audio-3.1-tts-flash`、`qwen-audio-3.1-tts-plus`、`qwen-audio-3.0-tts-flash`、`qwen-audio-3.0-tts-plus`；`qwen-audio-3.1-tts-next`；`cosyvoice-v3.5-plus`、`cosyvoice-v3.5-flash`、`cosyvoice-v3-plus`、`cosyvoice-v3-flash`、`cosyvoice-v2`；`qwen3-tts-flash` |
| 参数 | OpenAI 风格 `input`（文本）、`voice`、`response_format`、`speed`；映射为千问的 `input.text`、`input.voice`、`input.format`、`input.rate`。千问原生 `instruction`、`language_hints`、`hot_fix`、`seed`、`enable_ssml`、`enable_markdown_filter`、`language_type`、`instructions` 等原样透传到 `input` |
| 音频生成参数 | `qwen-audio-3.1-tts-next` 使用 `input.text_prompt`（最多 3000 字符）；参考音频通过 `references: [{"audio_url": "..."}]` / `[{"audio_data": "data:audio/wav;base64,..."}]`，或用统一字段 `audio_url` / `audio_urls`，按顺序对应提示词里的 `@voice1`、`@voice2`、`@voice3`（最多 3 条，每条 ≤30s / 10MB） |
| 返回 | 上游返回 `output.audio.url` 临时链接，服务端二次下载后直接返回音频字节；`Content-Type` 按真实格式声明（`audio/wav` / `audio/mpeg` / `audio/pcm` / `audio/ogg`）。下载失败判失败且不计费 |

**配置**：
- `base_url`: `https://maas.qianwenaiapi.com`（官方）或阿里云百炼 MaaS 专属端点（**不带 `/v1`**）
- 默认值：Qwen-Audio-TTS 音色 `longanhuan_v3.1`；CosyVoice 音色 `longanyang`；Qwen3-TTS 音色 `Cherry`；tts-next 无系统音色，默认 48kHz / 双声道
- 默认输出 `wav`、24kHz，可用 `voice` / `response_format` / `sample_rate` 覆盖

> 三套协议不要混：Qwen-Audio-TTS / CosyVoice / tts-next 走 `SpeechSynthesizer`；
> `qwen3-tts-flash` 走 `multimodal-generation`；旧 `qwen-tts` 走上一节「阿里 DashScope TTS」。
> 全部为**非流式 HTTP**：SSE 流式、WebSocket 实时（`qwen3-tts-flash-realtime`）以及音色复刻 /
> 音色设计（`voice-enrollment`）的管理接口暂未中转；复刻音色可直接把创建好的 `voice` ID 传进来。
> 上游临时 URL 允许 http / https；下载超时 120s、上限 64MB。
> `/v1/audio/speech` 请求体上限仍是 10MB，Base64 参考音频较大时请改用公网 URL。

#### DeepSeek 官方 TTS（`/v1/tools/tts`，按字符计费）
| 项 | 值 |
|---|---|
| 对外端点 | `POST /v1/audio/speech`（OpenAI 兼容，同步返回音频字节） |
| 上游端点 | `POST {base_url}/v1/tools/tts` |
| 适配模型 | `deepseek-tts`（推荐，也是唯一对外约定的名字）、`ds-tts`、`deepseek-speech*` |
| 参数 | 文本 `input`（OpenAI 风格）或 `text`（DeepSeek 原生）；音色 `voice` / `voice_id` 取 `stella`（女声，默认）或 `tide`（男声）；格式 `format` / `response_format` 只接受 `wav`（默认）或 `pcm`，传 `mp3` 会回退 `wav` |
| 返回 | 音频裸字节，`Content-Type` 按真实格式回 `audio/wav` 或 `audio/pcm` |
| 计费 | **按字符**：分组「TTS 每百万字符价格」(`audio_tts_price_per_million_chars`)。中文汉字算 2 字符，其它字符算 1 |

**配置**：
- 账号平台 `deepseek`（沿用现有 DeepSeek 文本账号最省事）或 `openai`（承载约定，配置最少）
- `base_url`: `https://api.deepseek.com`（**不带 `/v1`**，adapter 自己拼 `/v1/tools/tts`）
- `api_key`: DeepSeek 平台密钥
- `model_mapping`: `{"deepseek-tts": "deepseek-tts"}` —— **deepseek 平台账号必须配**，否则账号只服务
  官方文本模型白名单（`deepseek-flash` 等），`deepseek-tts` 会被判为"无可用账号"
- 分组模型白名单放行 `deepseek-tts`；分组计价按官方单价折算成「每百万字符」：
  官方 10,000 字符 $0.45（限时 $0.225）→ 每百万字符 **45**（限时 **22.5**）

> 官方该接口**没有 model 参数**，`deepseek-tts` 只是本网关的路由标签，不会下发上游。
> 不支持：语速调节、断点续传、WebSocket 流式（`wss://.../v1/tools/tts`）。
> 单请求上限 10,000 计费字符 / 64 KiB 请求体 / 30 分钟音频；官方 429（并发>50）、503（繁忙）、
> 504（合成超时）会按上游故障自动切下一个账号。

### OpenAI 兼容（聚合站 / TTS / Whisper）
| 项 | 值 |
|---|---|
| 音频端点 | `POST /v1/audio/speech`、`/v1/audio/transcriptions`、`/v1/audio/translations` |
| 图片端点 | `POST /v1/images/generations` |
| 视频端点 | `POST /v1/videos/generations` |

**配置**：
- `base_url`: OpenAI 兼容聚合站根域名（如 `https://api.example.com`）
- `model_mapping`: `{"my-tts": "tts-1", "my-whisper": "whisper-1"}`

## 四、计价配置

在分组（Group）设置里配置媒体单价：

| 类型 | 配置字段 | 口径 |
|---|---|---|
| 视频 | `video_model_prices`（family: seedance / minimax-video / wan-video）+ `video_price_480p/720p/1080p` | 每秒 × 分辨率 |
| 图片 | `image_price_1k/2k/4k` | 每张 × 尺寸档 |
| 音频 | `audio_realtime_price_per_min` + `audio_price_per_sec` | 按分钟或按秒 |
| 按字符计费的 TTS | `audio_tts_price_per_million_chars`（后台「TTS 每百万字符价格」） | 每百万计费字符（DeepSeek TTS） |

> 音频按秒计价：若配置 `audio_price_per_sec`，音频生成按秒计费且独立于视频；未配置时回退按分钟（`audio_realtime_price_per_min`）。
> **按字符计费的厂商（DeepSeek TTS）优先走 `audio_tts_price_per_million_chars`，时长完全不参与**：
> 它的官方计价单位是字符，按时长折算会让同一段文本的费用差几十倍。未配该字段时回退默认
> 15 USD/百万字符，请按厂商价目表改（DeepSeek 官方 10,000 字符 $0.45 → 45）。

### 预扣费
- 创建任务时按"估算单价 × 时长"预扣（写入 `reserved_cost`）
- 完成时按实际多退少补；失败/取消全额退回
- 需要 `quota` 场景，无需单独开关

### 资源转存
- 媒体产物 URL 自动转存到对象存储（需启用 `image_storage` 配置）
- 未启用/失败时回退上游签名 URL（24h 过期）

## 五、验证建议（连通性）

配好一个厂商后，用一个最小请求验证：

**文生图（Wan）**：
```bash
curl -X POST https://<你的域名>/v1/media/generations \
  -H "Authorization: Bearer <你的API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{"model":"wan2.6-t2i","prompt":"一只白猫"}'
```

**文本转音频**：
```bash
curl -X POST https://<你的域名>/v1/audio/speech \
  -H "Authorization: Bearer <你的API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{"model":"tts-1","input":"你好"}'
```

**文本转音频（DeepSeek 官方 TTS）**：
```bash
# 返回 audio/wav 音频字节；--output 直接落盘为 wav
curl -X POST https://<你的域名>/v1/audio/speech \
  -H "Authorization: Bearer <你的API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{"model":"deepseek-tts","input":"你好，这是 DeepSeek TTS。","voice":"stella"}' \
  --output speech.wav
```

**文生视频（Seedance）**：
```bash
curl -X POST https://<你的域名>/v1/media/generations \
  -H "Authorization: Bearer <你的API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{"model":"seedance-2.5","prompt":"夕阳下的海边","duration":5,"resolution":"720p"}'
```

## 六、注意事项

1. **必须配置 `model_mapping`**：调度器按映射选择账号，否则可能误选文本账号。
2. **Wan 的 `base_url` 必须带 WorkspaceId**：不同地域（北京/新加坡/东京/法兰克福/弗吉尼亚）域名不同。
3. **厂商 cancel 协议差异**：当前统一对任务端点发 DELETE；个别厂商（火山/阿里视频）可能需要专用 cancel endpoint，若取消失败不影响本地任务状态。
4. **预扣费需要分组单价**：若分组未配置视频/音频单价，预扣为 0（退化为完成时直接扣实际费用）。
5. **图片/音频多数同步返回 URL**：视频是异步任务（需 `MediaWorker` 轮询），图片/音频大多创建即返回。
