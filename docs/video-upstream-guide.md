# 视频上游源集成指南

本文档说明如何向 ThreeRouter 系统添加新的视频生成上游源，以及用户如何通过 API 使用这些视频源。

> **端点定位**：用户侧请用 `POST /v1/videos/generations`（或 `/v1/videos`）。
> 除 Grok 原生链路外，所有视频模型已收敛到统一媒体链路 `MediaTaskService`，
> 不再要求"模型必须在已知厂商白名单里"——能不能做由账号池决定。
> 支持 `?wait=N`（上限 180 秒）同步等待，省去客户端轮询。
> `POST /v1/video-tasks` 属于**只读兼容、不再维护**的老链路：已创建的任务仍可查询
> （`vid_` 开头的 ID 会先查 `media_tasks` 再回退 `video_tasks`），新接入请勿使用。
> 新增上游适配器仍在 `VideoAdapter` 体系内实现，由 `media_adapter_bridge.go` 桥接进统一链路。

## 一、已有上游源一览

| 上游源 | 模型命名前缀 | 是否已实现 | 适配器类 |
|--------|-------------|-----------|---------|
| **Seedance** (火山引擎) | `seedance-*`, `doubao-seedance-*`, `jimeng-video-*` | 已实现 | `SeedanceVideoAdapter` |
| **MiniMax Hailuo / H3** | `minimax-hailuo-*`, `minimax-video-*`, `minimax-h3-*` | 已实现 | `MiniMaxVideoAdapter` |
| **Wan** (阿里通义万相) | `wan*-*-video-*`, `wan*-*t2v-*` | 已实现 | `WanVideoAdapter` |
| **Grok Imagine Video** (xAI) | `grok-imagine-video-*`, `grok-video-*` | 已实现 | `OpenAIVideoAdapter` (通用) |
| **其他 OpenAI 兼容源** | 任何模型名 | 已实现 | `OpenAIVideoAdapter` (fallback) |

> **注意**：Kimi / 月之暗面（Moonshot）目前没有公开的视频生成 API，本系统不支持 `kimi-h3-*` 视频模型。

## 二、添加新视频上游源（开发者）

### 2.1 整体架构

```
用户请求 → VideoGatewayHandler → VideoTaskService → 选通道 → ModelMapping 翻译 → VideoAdapterRegistry → 具体 VideoAdapter → 上游 API
```

- `VideoAdapter` 接口: `Create(ctx, account, req)` 和 `GetResult(ctx, account, upstreamTaskID)`
- `VideoAdapterRegistry`: 按平台/模型名匹配，显式适配器优先，fallback 到 `OpenAIVideoAdapter`
- 写在一个文件 `video_adapters_vendors.go` 中，通过 `vendorVideoAdapter` 基类减少重复代码

### 2.2 添加步骤（三步）

#### 步骤 1：实现适配器

打开 `backend/internal/service/video_adapters_vendors.go`，添加新类型：

```go
type MyNewVideoAdapter struct{ vendorVideoAdapter }

func NewMyNewVideoAdapter() *MyNewVideoAdapter {
    a := &MyNewVideoAdapter{}
    a.vendorVideoAdapter = newVendorVideoAdapter("my-new-vendor")
    a.supports = func(platform, model string) bool {
        m := strings.ToLower(strings.TrimSpace(model))
        return strings.HasPrefix(m, "my-model-")
    }
    a.buildCreate = buildMyNewVideoCreateBody
    a.parseCreate = parseMyNewVideoCreateResult
    a.buildCreateURL = buildMyNewVideoCreateURL
    a.buildQuery = buildMyNewVideoQueryURL
    a.parseQuery = parseMyNewVideoQueryResult
    return a
}
```

需要实现的函数：

- **`buildCreate`**: 将 `VideoCreateRequest` 转成上游请求体 JSON
- **`parseCreate`**: 解析上游创建响应，提取 `task_id` 和 `status`
- **`buildCreateURL`**: 返回上游创建任务的 URL
- **`buildQuery`**: 根据 `upstreamTaskID` 返回查询任务状态的 URL
- **`parseQuery`**: 解析上游查询响应，提取 `video_url`, `status` 等
- **`createHeaders`**: (可选) 自定义请求头

#### 步骤 2：更新模型检测

**a) `backend/internal/service/video_adapter.go`** — `IsKnownVideoVendorModel` 函数：

```go
case strings.HasPrefix(m, "my-model-"):
    return true
```

**b) `backend/internal/service/video_billing.go`** — `CanonicalVideoModelPriceFamily` 函数：

```go
case strings.HasPrefix(m, "my-model-"):
    return VideoPriceFamilyMyModel
```

同时添加对应价格族常量：

```go
const (
    VideoPriceFamilyMyModel = "my-model"
    // ...
)
```

#### 步骤 3：注册到 Wire

在 `backend/internal/service/wire.go` 中：

```go
func ProvideVideoAdapter() *VideoAdapterRegistry {
    return NewVideoAdapterRegistry(
        NewSeedanceVideoAdapter(),
        NewMiniMaxVideoAdapter(),
        NewWanVideoAdapter(),
        NewMyNewVideoAdapter(),  // 新增
        NewOpenAIVideoAdapter(), // 最后一位是 fallback
    )
}
```

同时注册到 `ProvideMediaAdapter` 的 `NewVideoAsMediaAdapter` 中。

### 2.3 已有适配器 API 契约参考

| 适配器 | 创建路径 | 查询路径 | 请求体格式 | 响应格式 |
|--------|---------|---------|-----------|---------|
| Seedance | `POST /api/v3/contents/generations/tasks` | `GET /api/v3/contents/generations/tasks/{id}` | 带 `content` 数组 | 返回 `task_id` |
| MiniMax | `POST /v2/video_generation` | `GET /v2/query/video_generation/{id}` | 带 `content` 数组 | 返回 `data.task_id` |
| Wan | `POST /api/v1/services/aigc/...` | `GET /api/v1/tasks/{id}` | `model` + `input` + `parameters` | 返回 `output.task_id` |

## 三、添加上游账号（管理员）

### 3.1 通过管理后台添加

1. 进入 **管理后台 → 账号管理**
2. 点击 **"添加账号"**
3. 填写凭证：

| 字段 | 说明 |
|------|------|
| base_url | 上游 API 地址（如 `https://ark.cn-beijing.volces.com`） |
| api_key | 上游 API Key |
| model_mapping | 用户模型名 → 上游实际模型名，如 `{"seedance-2.5": "seedance-2.5-t2v"}` |
| video_create_path | (可选) 自定义创建路径 |
| video_query_path | (可选) 自定义查询路径 |

### 3.2 各上游源配置参考

#### Seedance（火山引擎）

| 配置项 | 建议值 |
|--------|--------|
| base_url | `https://ark.cn-beijing.volces.com` |
| model_mapping | `{"seedance-2.5": "seedance-2.5-t2v"}` |

#### Wan（阿里通义万相）

| 配置项 | 建议值 |
|--------|--------|
| base_url | `https://dashscope.aliyuncs.com` |
| model_mapping | `{"wan2.1-video": "wan2.1-t2v-turbo"}` |
| 备注 | 需要额外 Header: `X-DashScope-Async: enable`（适配器已自动添加） |

#### MiniMax Hailuo / H3

| 配置项 | 建议值 |
|--------|--------|
| base_url | `https://api.minimax.cn`（官方 V2 文档的 server） |
| model_mapping | `{"minimax-h3": "MiniMax-H3"}` |

MiniMax H3 要点：

- 创建路径 `POST /v2/video_generation`，查询 `GET /v2/query/video_generation/{task_id}`。
- `content` 数组里必须有一个非空 `text`。
- **图生视频**：图片项为 `{"type":"image_url","role":"first_frame","image_url":{"url":"..."}}`。不传 `role` 且只有一张图时，上游默认按 `first_frame` 处理。
- **首尾帧**：`first_frame` 与 `last_frame` 必须成对出现。
- **多模态参考**：`reference_image` / `reference_video` / `reference_audio`（视频≤3 个、音频≤3 个）。
- `resolution`：`768P` 或 `2K`（H3）；`MiniMax-H3-Max` 支持 `480P`/`768P`，不支持 `2K`。
- `duration`：H3 为 4~15 秒整数。
- `ratio`：文生视频必须显式指定且不能为 `adaptive`；图生视频恒为 `adaptive`。

## 四、用户 API 使用指南

Seedance / Wan / MiniMax 有两套接口都可用，功能相同（创建任务 → 轮询 → 302 取内容），区别是路由路径、模型识别方式和任务存储表：

| 对比项 | `/v1/media/generations` | `/v1/videos/generations` |
|--------|------------------------|--------------------------|
| 适用模型 | 图片 / 视频 / 音频统一入口 | 仅视频 |
| 模型识别 | 按 `model` 名自动推断 kind，也支持 `media_kind` 手动指定 | 通过 `IsKnownVideoVendorModel` 识别，仅已注册前缀 |
| 任务存储表 | `media_tasks` | `video_tasks` |
| 响应字段 | `url` | `url`（相同） |
| 适配器 | `VideoAsMediaAdapter`（桥接到同一批 VideoAdapter） | 直接使用 `VideoAdapter` |
| 计费 | `calculateMediaCost`（视频用同一套价格族） | `calculateVideoCost` |
| 推荐场景 | 混合用途（既有图片又有视频）| 纯视频、需要 `/v1/videos/*` 路由兼容性 |

两个入口最终都走同一批 Seedance / Wan / MiniMax 适配器，上游行为和计费逻辑一致。

### 4.1 推荐入口：POST /v1/media/generations

```bash
curl -X POST https://api.threerouter.com/v1/media/generations \
  -H "Authorization: Bearer <your_api_key>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "seedance-2.5",
    "prompt": "一只在沙滩上奔跑的狗，夕阳，电影感",
    "resolution": "720p",
    "ratio": "16:9",
    "duration": 5
  }'
```

**响应**：

```json
{
  "id": "mt_abc123def456",
  "status": "processing",
  "model": "seedance-2.5",
  "created_at": "2026-09-07T10:00:00Z"
}
```

> 本地任务 ID 前缀：`media_tasks` 表用 `mt_`，`video_tasks` 表用 `vid_`。

### 4.2 兼容入口：POST /v1/videos/generations

对 OpenAI 风格客户端更友好。请求中携带已注册的 vendor 模型名（如 `seedance-2.5`）时，路由层会自动转入 `VideoGateway`，与 `/media` 入口等效。

```bash
curl -X POST https://api.threerouter.com/v1/videos/generations \
  -H "Authorization: Bearer <your_api_key>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "seedance-2.5",
    "prompt": "一只在沙滩上奔跑的狗，夕阳，电影感",
    "resolution": "720p",
    "duration": 5
  }'
```

### 4.3 查询任务状态

两个入口分别用各自的查询路由，`local_id` 不能跨接口混用：

```bash
# 如果创建时调的是 /media/generations，查这里
curl https://api.threerouter.com/v1/media/mt_abc123def456 \
  -H "Authorization: Bearer <your_api_key>"

# 如果创建时调的是 /videos/generations，查这里
curl https://api.threerouter.com/v1/videos/vid_abc123def456 \
  -H "Authorization: Bearer <your_api_key>"
```

**响应（完成时）**：

```json
{
  "id": "mt_abc123def456",
  "status": "succeeded",
  "model": "seedance-2.5",
  "url": "https://upstream-cdn.com/video.mp4",
  "thumbnail_url": "https://upstream-cdn.com/thumb.jpg",
  "duration_sec": 5,
  "resolution": "720p",
  "created_at": "2026-09-07T10:00:00Z",
  "finished_at": "2026-09-07T10:01:00Z"
}
```

### 4.4 302 取内容

```bash
# /media 入口
curl -L https://api.threerouter.com/v1/media/mt_abc123def456/content \
  -H "Authorization: Bearer <your_api_key>"

# /videos 入口
curl -L https://api.threerouter.com/v1/videos/vid_abc123def456/content \
  -H "Authorization: Bearer <your_api_key>"
```

### 4.5 请求参数（两套接口一致）

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `model` | string | 是 | 模型名，如 `seedance-2.5` / `minimax-h3` / `wan2.1-video` |
| `prompt` | string | 否 | 文本提示词 |
| `negative_prompt` | string | 否 | 负面提示词 |
| `resolution` | string | 否 | 分辨率：`480p` / `720p` / `1080p` |
| `ratio` | string | 否 | 宽高比：`16:9` / `9:16` / `1:1` |
| `duration` | int | 否 | 视频时长（秒） |
| `image` | string / array / object | 否 | 参考图（图生视频）。支持裸 URL、URL 数组、`{"url": "..."}` |
| `image_url` | string | 否 | 参考图 URL，OpenAI 风格别名 |
| `image_urls` | array | 否 | 多张参考图 URL |
| `video_url` | string | 否 | 参考视频 URL |
| `audio_url` | string | 否 | 参考音频 URL |
| `media` | array | 否 | 多模态素材列表 |
| `seed` | int | 否 | 随机种子 |

`/media/generations` 额外支持 `media_kind` 字段（`"image"` / `"video"` / `"audio"`）手动指定类型，适用于模型名无法自动识别的场景。`/videos/generations` 不需要也不识别此字段。

### 4.6 `media` 素材格式

```json
{
  "media": [
    {"type": "first_frame", "url": "https://threerouter.com/start.jpg"},
    {"type": "reference_video", "url": "https://threerouter.com/motion.mp4"},
    {"type": "reference_audio", "url": "https://threerouter.com/audio.mp3"}
  ]
}
```

| type | 说明 |
|------|------|
| `first_frame` | 首帧参考图 |
| `last_frame` | 尾帧参考图 |
| `reference_image` | 普通参考图 |
| `reference_video` | 参考视频 |
| `reference_audio` | 参考音频 |

## 五、API 端点与状态

### 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/v1/media/generations` | 统一媒体生成入口（推荐） |
| `GET` | `/v1/media/{id}` | 查询媒体任务状态 |
| `GET` | `/v1/media/{id}/content` | 302 重定向到媒体文件 |
| `POST` | `/v1/videos/generations` | 视频兼容入口（vendor 模型自动转入 VideoGateway） |
| `GET` | `/v1/videos/{task_id}` | 查询视频任务状态 |
| `GET` | `/v1/videos/{task_id}/content` | 302 重定向到视频文件 |
| `POST` | `/v1/video-tasks` | 专用视频任务入口（非 OpenAI 兼容路径） |

### 状态

| 状态 | 说明 |
|------|------|
| `processing` | 任务正在进行中，请轮询 |
| `succeeded` | 任务完成，`url` 字段包含视频链接 |
| `failed` | 任务失败，`error` 字段包含错误信息 |
| `cancelled` | 任务已被取消 |

## 六、图生视频用法（重要）

如果你之前遇到“任务成功但图片被忽略”或“直接 502”，基本是参考图字段写错导致的。
系统现在同时接受下面几种写法，都会正确转成各厂商的图生视频结构：

### 6.1 推荐的统一写法

```json
{
  "model": "minimax-h3",
  "prompt": "镜头缓慢推进，人物眨眼",
  "image": "https://cdn.example.com/start.png",
  "resolution": "2K",
  "duration": 5,
  "ratio": "adaptive"
}
```

### 6.2 其他等价写法

```json
{ "image": ["https://cdn.example.com/a.png"] }
{ "image": { "url": "https://cdn.example.com/a.png" } }
{ "image_url": "https://cdn.example.com/a.png" }
{ "image_urls": ["https://cdn.example.com/a.png"] }
```

### 6.3 多模态参考（首尾帧 / 参考视频 / 参考音频）

```json
{
  "model": "minimax-h3",
  "prompt": "角色说话：Follow the wind",
  "media": [
    { "type": "first_frame", "url": "https://cdn.example.com/start.png" },
    { "type": "last_frame",  "url": "https://cdn.example.com/end.png" },
    { "type": "reference_video", "url": "https://cdn.example.com/motion.mp4" },
    { "type": "reference_audio", "url": "https://cdn.example.com/voice.mp3" }
  ],
  "resolution": "2K",
  "duration": 5
}
```

### 6.4 各厂商的转换结果

| 厂商 | 上游字段 | 说明 |
|------|---------|------|
| MiniMax H3 | `content[].image_url` + `role=first_frame` | 首图自动成为首帧 |
| Seedance | `content[].image_url` + `role=first_frame` | 同上 |
| Wan | `input.media[].type=first_frame` | DashScope 结构 |

### 6.5 注意事项

- 参考图必须是公网可访问 URL，或厂商支持的 `data:` / file_id 形式。
- MiniMax H3 的 `resolution` 用 `768P` / `2K`，不是 `720p` / `1080p`。传标准档位上游会拒绝。
- 文生视频（不带图片）时 MiniMax H3 的 `ratio` 必须显式指定，不能是 `adaptive`。
- 图生视频时 `ratio` 会被上游忽略并按 `adaptive` 处理，传具体值不会报错。

## 七、已知问题与建议（上线前）

### 7.1 上游 base_url 未做 URL 校验

视频适配器直接拼接管理员填写的 `base_url` 发起 HTTP 请求，没有经过 `urlvalidator.ValidateHTTPURL`。如果管理员账号被盗，攻击者可配置 `base_url` 指向内网地址进行 SSRF。

**建议**：在视频适配器 `baseURL()` 中加入 `urlvalidator.ValidateHTTPURL`，至少拒绝 `http://localhost`、`http://127.0.0.1`、`http://10.*` 等私网地址。

### 7.2 上游返回的 `video_url` / `thumbnail_url` 未校验

上游 API 返回的视频 URL 会直接写入数据库，并通过 `/v1/videos/{task_id}/content` 302 重定向给用户。如果上游被劫持或返回恶意 URL，用户会被重定向到任意外部地址。

**建议**：对上游返回的 URL 做 scheme + 域名白名单校验，或者至少拒绝 `javascript:` / `data:` 协议。

### 7.3 配额预扣可能少扣

创建任务时按预估时长预扣费用，但部分上游实际生成时长可能与请求不符。结算时会按实际时长补扣或退款（`settleMediaReservedQuota`），这个逻辑是对的。但如果 `calculateVideoCost` 报错导致 `cost = 0`，则结算时实际费用按 0 处理，不会补扣。

**建议**：当 `costErr != nil` 时改为使用预估费用（`record.ReservedCost`）结算，而不是 0。

### 7.4 任务列表接口未做用户隔离（管理后台除外）

`ListTasksByUserID` 按 `userID` 过滤，安全。但管理后台的 `ListAdmin` 没有额外的权限检查，依赖 handler 层的 admin middleware。请确认部署时 admin 路由已挂载 auth 中间件。

### 7.5 `video_url` 返回字段名

用户响应中视频 URL 字段为 `url`（非 `video_url`），与部分客户端预期可能不一致。文档中已用 `url`，保持一致即可。

