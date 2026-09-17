# 阿里千问图像生成与编辑 3.0 接入指南

适用模型：`qwen-image-3.0`、`qwen-image-3.0-pro`（文生图 T2I + 图生图/图像编辑 I2I）。

## 一、支持现状

| 项 | 状态 |
|----|------|
| adapter | `WanImageAdapter`（DashScope 同步协议），`Supports` 已包含 `qwen-image` |
| 端点 | `POST /v1/images/generations`（推荐）/ `POST /v1/media/generations`（兼容保留） |
| 路由 | 媒体网关按**模型名**匹配 adapter，与账号平台无关 |
| 可用性 | 本轮修复前**跑不通**（3 个 P0），修复后可用 |

### 本轮修复的问题（已用官方契约写测试验证）

| # | 问题 | 后果 | 修复 |
|---|------|------|------|
| P0-1 | `size` 用 OpenAI 的 `x` 分隔原样下发 | DashScope 要求 `宽*高`，参数非法被拒 | `normalizeDashScopeImageSize` 自动转换 |
| P0-2 | 图生图参考图未下发 | `content` 里只有 `text`，I2I 静默退化成 T2I | 参考图写入 `content` 的 `image` 项，排在文本前 |
| P0-3 | 只按 HTTP 状态码判失败 | DashScope 失败是 **HTTP 200 + `code`/`message`**，被当成"成功但没图" | 识别 `code` 字段判失败，错误信息带错误码 |
| 增强 | `n` 硬编码 1 | 千问支持 1-6 张，多张请求被降为 1 张 | 下发实际张数，上限收敛到 6 |
| 增强 | 只取 `choices[0]` | n>1 时其余图全丢 | 收集全部 URL，创建响应返回 `urls` |
| 增强 | 超时 120 秒 | 官方建议 600 秒起，长图会在上游正常生成时提前断开 | 图片 adapter 超时提到 600 秒 |

## 二、和 gpt-image-2 用法一样吗？——**调用方式已经一样**

OpenAI 风格客户端现在可以零改造接入：`/v1/images/generations` 收到千问模型时会转
统一媒体链路，并由 `MediaGateway.CreateImages` 把结果改写成 OpenAI 标准 `ImagesResponse`。

| | gpt-image-2 | 千问图像 3.0 |
|---|---|---|
| 端点 | `/v1/images/generations` | **同一个端点**（不再是 `/v1/media/generations`） |
| 尺寸参数 | `size` | `size` / `resolution` 等价 |
| 返回 | 同步 `data[0].url` | 同样是 `{created, data:[{url}]}`；`b64_json` 也支持 |
| 图生图 | `/v1/images/edits`（multipart） | 同一端点，multipart 上传文件会自动转成 data URI |

```python
resp = client.images.generate(model="qwen-image-3.0", prompt="一只橘猫", size="1024x1024")
print(resp.data[0].url)
```

> 千问的产物 URL 有效期约 24 小时，需要长期保存请开启对象存储转存或自行下载。

## 三、添加步骤

1. **渠道平台**：选 `openai`（项目没有 dashscope 平台常量；媒体链路按模型名匹配 adapter，
   平台只影响分组选号）
2. **base_url**：`https://dashscope.aliyuncs.com`
   - 或业务空间专属域名 `https://{WorkspaceId}.cn-beijing.maas.aliyuncs.com`（官方推荐，稳定性更好）
   - **不要带 `/api/v1`**，adapter 会拼 `/api/v1/services/aigc/multimodal-generation/generation`
3. **地域一致性**：模型、Endpoint URL、API Key 必须属于**同一地域**，跨地域调用直接失败
4. **模型映射**：渠道的 model_mapping 里加 `qwen-image-3.0` / `qwen-image-3.0-pro`
5. **分组定价**：配图片价 1K / 2K / 4K。千问默认输出 1024×1024（1K），官方计量按
   面积 ≤2,250,000 为 1K 档，与之吻合

## 四、请求参数映射

| 客户端传参 | 下发给 DashScope | 说明 |
|---|---|---|
| `prompt` | `input.messages[0].content[].text` | 必填，支持中英文 |
| `image` / `image_url` / `image_urls` | `content[].image` | 图生图，最多 3 张，排在 text 前 |
| `resolution` 或 `size` | `parameters.size` | `1024x1024` 自动转 `1024*1024` |
| `n` | `parameters.n` | 1-6，超出收敛到 6 |
| `seed` | `parameters.seed` | |
| 其他（如 `negative_prompt` / `prompt_extend` / `watermark` / `enable_thinking`） | `parameters.*` 直接透传 | 百炼扩展字段 |

## 五、注意点

1. **URL 只有 24 小时有效期**（图片和异步 task_id 都是）。强烈建议配对象存储，
   平台会自动转存；否则用户拿到的是会过期的临时链
2. **不传 `resolution` 时模型自动推荐分辨率**。计费会取上游 `usage.output_width/height`
   的真实尺寸定档，不会一律按 2K 收
3. **出图慢**：`prompt_extend` 与 `enable_thinking` 默认开启会显著增加耗时。
   超时已调到 600 秒；若仍不够，可让用户在 `extra` 里传 `enable_thinking: false`
4. **图生图每次最多 3 张参考图**，超出部分会被丢弃（官方限制）
5. **多张 URL 全量返回**：创建响应带 `urls`，且 `media_tasks.media_urls` 已落库，
   轮询 `GET /v1/media/:id` 同样返回完整 `urls`（不再只给首张）
6. **异步模式未接入**：官方异步端点（`X-DashScope-Async: enable` + task_id 轮询）
   当前不支持，走的是同步端点。若后续出现大量超时再补

## 六、计费口径

与视频一致遵循"按真实扣费"：

1. 张数：上游实际返回的图片数优先于请求的 `n`（上游可能因安全策略少出图）
2. 档位：上游 `usage.output_width × output_height` 优先于请求 `resolution`，
   都没有才回落默认 2K 档
3. 长边定档（图片惯例）：1024 → 1K，>1024 → 2K/4K

## 七、相关文件

- `backend/internal/service/media_vendor_image_adapter.go` — Wan/DashScope adapter
- `backend/internal/service/media_adapter.go` — `MediaCreateResult`（新增 `UpstreamSize`）
- `backend/internal/service/media_task_service.go` — 真实尺寸回填与按张计费
- `backend/internal/service/media_task_test.go` — 契约测试
