# 媒体渠道（上游账号）接入清单

> 面向运维/后台配置。「模型已被识别」不等于「能调用」——后台没有可用渠道时，
> 请求会在选号阶段失败并返回 `503 capacity_error`，message 形如：
>
> ```
> 当前分组没有可服务模型 qwen-image-3.0 的可用账号：请在该分组内添加支持此模型
> 的账号并挂上该模型，或改用 composite 分组由平台按模型自动选择通道
> ```
>
> 这一步发生在计费之前，任务表与用量明细都不会有记录，因此错误信息是唯一线索。
> 本文列出千问 / MiniMax / 火山等媒体上游的开户参数。

## 一、账号通用要求

| 配置项                    | 值                              |
| ---------------------- | ------------------------------ |
| 类型                     | `upstream`（Base URL + API Key） |
| 平台                     | 见第二节，**必须与分组平台的选号池一致**         |
| 凭证 `base_url`          | 厂商 API 根地址（不要带尾部 `/`）          |
| 凭证 `api_key`           | 厂商密钥                           |
| 模型登记                   | 只登记该账号负责的媒体模型，避免被文本请求选中        |
| 可选 `video_create_path` | 覆盖默认调用路径（一般不用）                 |

适配器按**模型名**匹配（与账号平台无关），路径由适配器决定，所以 `base_url` 只需给到厂商根域。

## 二、平台怎么选（关键，先分清两个「平台」）

- **分组平台**：创建分组时下拉框选的那个（OpenAI / DeepSeek / Composite…是**并列选项**）。它决定分组**从哪个账号池选号**。
- **账号平台**：添加上游账号时填的平台字段。它决定这个账号**归入哪个池子**。

「deepseek 承载」说的是**账号平台**，不是说分组要选 DeepSeek：

- **openai 分组** → 媒体上游账号的**账号平台填 `openai`**（推荐，配置最少）
- **Composite 分组** → 分组平台选 Composite；千问 / MiniMax / 火山等国产媒体账号的**账号平台填 `deepseek`**（沿用视频厂商的承载约定）。若配了显式复合路由，以路由指定的目标平台为准
- **DeepSeek 分组** → 只是纯 DeepSeek 文本分组，**不要**用它承载媒体账号

## 三、厂商参数

### 阿里千问图像 3.0（qwen-image）

| 项          | 值                                                        |
| ---------- | -------------------------------------------------------- |
| 模型名        | `qwen-image-3.0`、`qwen-image-3.0-pro`                    |
| `base_url` | `https://dashscope.aliyuncs.com`（走 DashScope 原生多模态接口）    |
| 实际调用路径     | `/api/v1/services/aigc/multimodal-generation/generation` |
| 单张张数上限     | 6（超出自动收敛）                                                |
| 参考图上限      | 3 张                                                      |
| 客户端超时      | ≥ 600s（默认开 prompt_extend + 思考模式）                         |

> 若改用 OpenAI 兼容模式（`https://dashscope.aliyuncs.com/compatible-mode/v1`），  
> 会被通用兜底适配器以 `/v1/images/generations` 调用；推荐用上面的原生路径，  
> 因为能拿到 `usage.output_width/height`，计费按真实尺寸而非请求值。

### MiniMax 生图（image-01）

| 项          | 值                                                           |
| ---------- | ----------------------------------------------------------- |
| 模型名        | `minimax-image-01`、`image-01`                               |
| `base_url` | `https://api.minimax.chat`（国际站用 `https://api.minimaxi.com`） |
| 实际调用路径     | `/v1/image_generation`                                      |
| 单张张数上限     | 9                                                           |
| 计费注意       | 官方 `aspect_ratio` 1:1 等价 1K，按 1K 档计费，不会多收                   |

### 火山方舟 Seedream / Seedance

| 项          | 值                                   |
| ---------- | ----------------------------------- |
| 生图模型       | `doubao-seedream-*`、`seedream-*`    |
| 生视频模型      | `seedance-*`、`doubao-seedance-*`    |
| `base_url` | `https://ark.cn-beijing.volces.com` |
| 生图路径       | `/api/v3/images/generations`        |

### MiniMax 生视频（H3 / H3 Max）

| 项 | 值 |
| --- | --- |
| 模型名 | `MiniMax-H3`、`MiniMax-H3-Max`、`minimax-hailuo-*`、`minimax-video-*`（`minimax-h*` 通配已覆盖） |
| `base_url` | `https://api.minimax.cn` |
| 创建路径 | `/v2/video_generation` |
| 查询路径 | `/v2/query/video_generation/{task_id}` |
| `resolution` | H3：`768P` / `2K`；H3 Max：`480P` / `768P`（传 2K 会被本地 400 拦下） |
| `duration` | H3：4～15 秒整数；H3 Max：5～15 秒整数 |
| `ratio` | 文生视频必填且不能是 `adaptive`（服务端缺省补 `16:9`）；带素材时补 `adaptive` |
| 素材上限 | 图片 ≤9 张 / 视频 ≤3 段 / 音频 ≤3 段，混合共 ≤12 个；单图 30MB、单视频 50MB、单音频 15MB、请求体 64MB（**推荐用 URL 传素材，别传 base64**） |

### 其他

MiniMax 视频（`minimax-hailuo-*`）、Wan 视频（`wan*-t2v`、`wan*-i2v`、`wan*-video`）沿用已有视频渠道配置，与生图共用同一账号即可。

> **提示**：客户端提交超时设成 60 秒会导致 `502 context canceled`（连接断开把上游
> 请求连坐掐断）。服务端已加固为「上游请求不跟随客户端断连」，但渠道验收时仍建议
> 用 ≥300 秒的超时，见 `client-api-guide.md` 第 5 节。

## 四、验收步骤

1. 后台建账号 → 状态 active、可调度。
2. 用该分组的 API Key 调一次 `POST /v1/media/generations`（见 `client-api-guide.md`）。
3. 预期：返回 `202` 且带 `id`；图片同步出 `url`，视频轮询到 `succeeded`。
4. 若返回 `503 capacity_error` → 渠道未匹配（平台不一致或模型未登记）。
5. 若返回 `403 permission_error` → 分组「允许生成图片」未开（生图类任务）。
6. 若返回 `composite target platform unknown` → composite 分组解析不出目标平台，  
   为模型配置显式复合路由，或按第二节把账号平台对齐。
