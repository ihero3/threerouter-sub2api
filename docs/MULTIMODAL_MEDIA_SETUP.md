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

#### MiniMax TTS
| 项 | 值 |
|---|---|
| 端点 | `POST /v1/t2a_v2` |
| 适配模型 | `minimax-tts`、`speech-01`、`tts` |

**配置**：
- `base_url`: `https://api.minimax.cn`
- `model_mapping`: `{"minimax-tts": "MiniMax-TTS"}`

#### 火山 TTS
| 项 | 值 |
|---|---|
| 端点 | `POST /api/v3/tts` |
| 适配模型 | `volc-tts`、`volcano-tts`、`doubao-tts` |

**配置**：
- `base_url`: `https://ark.cn-beijing.volces.com`
- `model_mapping`: `{"volc-tts": "<火山音色ID>"}`

#### 阿里 TTS（cosyvoice / qwen-tts）
| 项 | 值 |
|---|---|
| 端点 | `POST /api/v1/services/aigc/multimodal-generation/generation` |
| 适配模型 | `aliyun-tts`、`dashscope-tts`、`cosyvoice`、`qwen-tts` |

**配置**：
- `base_url`: `https://<WorkspaceId>.cn-beijing.maas.aliyuncs.com`
- `model_mapping`: `{"cosyvoice": "cosyvoice-v1"}`

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

> 音频按秒计价：若配置 `audio_price_per_sec`，音频生成按秒计费且独立于视频；未配置时回退按分钟（`audio_realtime_price_per_min`）。

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
