# ThreeRouter 生图 API 使用指南

ThreeRouter 是模型聚合网关：一套 OpenAI 兼容 API，后端自动路由到 OpenAI / xAI Grok / 火山方舟（豆包 Seedream）/ 阿里通义（Qwen、Wan）/ MiniMax / Gemini 等多家服务商，调用方无需对接各家 SDK。

## 快速开始

**Base URL**：`https://www.threerouter.com/v1`（本地开发 `http://localhost:8080/v1`）

**认证**：在后台「API 密钥」页创建密钥，请求头携带：

```bash
Authorization: Bearer sk-xxxxxxxx
```

（也支持 `x-api-key: sk-xxx`）

**第一个请求（文生图）**：

```bash
curl https://www.threerouter.com/v1/images/generations \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "一只在星空下奔跑的橘猫，赛博朋克风格",
    "size": "1024x1024",
    "n": 1,
    "response_format": "b64_json"
  }'
```

**响应**（OpenAI Images 标准格式）：

```json
{
  "created": 1726100000,
  "data": [
    { "b64_json": "iVBORw0KGgo...(Base64 图片数据)" }
  ]
}
```

`response_format` 传 `url` 时返回 `{"url": "https://..."}`，URL 有效期有限请及时转存。

## 端点一览

| 方法 | 路径 | 用途 |
|---|---|---|
| POST | `/v1/images/generations` | 文生图（同步） |
| POST | `/v1/images/edits` | 图生图 / 局部编辑（蒙版） |
| POST | `/v1/images/generations/async` | 异步生图提交 |
| GET  | `/v1/images/tasks/:task_id` | 异步任务结果轮询 |
| POST | `/v1/images/batches` | 批量生图（一次几百上千条） |
| GET  | `/v1/images/batches/:id/download` | 批量结果 ZIP 打包下载 |
| POST | `/v1/media/generations` | 统一媒体入口（图/视频/音频） |

## 请求参数（/images/generations）

| 参数 | 说明 |
|---|---|
| `model` | 模型名，见下表；不传默认 `gpt-image-2` |
| `prompt` | 图片描述，必填 |
| `n` | 生成张数，默认 1 |
| `size` | `"1024x1024"` / `"2048x2048"` / `"1K"` `"2K"` `"4K"` / `"auto"` |
| `response_format` | `b64_json`（默认）或 `url` |
| `quality`、`background`、`output_format` | 可选，透传上游（部分模型支持） |

## 支持的模型

| 模型 | 厂商 | 说明 |
|---|---|---|
| `gpt-image-1` / `gpt-image-2` | OpenAI | 默认模型，支持文生图 + 图生图 |
| `grok-imagine-image` / `grok-imagine-edit` | xAI | 文生图 / 编辑 |
| `doubao-seedream-4-0` | 火山方舟（豆包） | 中文场景效果好 |
| `qwen-image`、`wan2.x-t2i` | 阿里通义 | |
| `minimax-image-01`、`hailuo-image` | MiniMax | |
| `gemini-3-pro-image`、`gemini-2.5-flash-image` | Google | 批量生图主推 |

以上模型在同一端点可用，切换只需改 `model` 字段。

## 图生图 / 编辑（/images/edits）

两种传图方式：

**方式一：JSON + 图片 URL / Base64**

```bash
curl https://www.threerouter.com/v1/images/edits \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "把背景换成海滩",
    "images": [{ "image_url": "data:image/png;base64,..." }],
    "size": "1024x1024"
  }'
```

**方式二：multipart 文件上传**

```bash
curl https://www.threerouter.com/v1/images/edits \
  -H "Authorization: Bearer sk-xxx" \
  -F model=gpt-image-2 \
  -F "prompt=把背景换成海滩" \
  -F "image=@/path/to/input.png"
```

可选 `mask`（JSON 的 `mask.image_url` / multipart 的 `mask` 文件）指定编辑区域，未蒙版区域保持不变。

## 异步生图（推荐生产使用）

生图耗时数秒到数十秒，长 Prompt 或大批量建议异步：

```bash
# 1. 提交
curl https://www.threerouter.com/v1/images/generations/async \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-image-2", "prompt": "...", "size": "1024x1024"}'
# → {"id": "task_abc", "status": "processing", "poll_url": "/v1/images/tasks/task_abc"}

# 2. 轮询（建议 2~5 秒一次）
curl https://www.threerouter.com/v1/images/tasks/task_abc \
  -H "Authorization: Bearer sk-xxx"
# → status: processing → succeeded；succeeded 后 result 中含图片 URL
```

任务结果保留 24 小时。

## 计费规则

- **按张计费**：`费用 = 模型单价(按 1K/2K/4K 档) × 张数 × 分组倍率`，档位按实际输出分辨率归档（≤1024 为 1K，≤2048 为 2K，更高为 4K）
- 各模型单价见控制台「模型价格」页；分组可配置独立图片价
- 批量生图享折扣（默认 5 折），提交时预冻结额度，任务结束后多退少补
- 请求前会校验余额，余额不足返回 402/403

## 常见错误

| 状态码 | 含义 |
|---|---|
| 401 | API Key 无效或未传 |
| 403 | 无该分组/模型权限，或未开通生图（`allow_image_generation`） |
| 400 | 参数错误（如非图片模型调用了 images 端点） |
| 429 | 触发限流，请稍后重试 |
| 502/504 | 上游服务商超时/失败，可重试 |

## 注意事项

1. **Prompt 内容安全**：请求会经过内容审核，违规 Prompt 会被拒绝
2. **图片转存**：`url` 模式返回的链接有时效，批量/异步结果请及时下载
3. **不支持 `file_id`**：图生图请用 `images[].image_url`（Base64 data URL 或公网可访问 URL）
4. 所有端点兼容 OpenAI Images API 结构，已有 OpenAI SDK 只需改 `base_url` 和 `api_key` 即可直接使用
