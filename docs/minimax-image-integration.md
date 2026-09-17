# MiniMax 图片生成（image-01）接入说明

适用范围：ThreeRouter / sub2api 聚合平台，MiniMax 文生图与图生图（`image-01`）。
对应代码：`backend/internal/service/media_vendor_image_adapter.go`（`MiniMaxImageAdapter`）。

---

## 一、支持现状

代码里**已内置** MiniMax 图片 adapter 并已注册（`wire.go` → `ProvideMediaAdapter`），但本次接入前存在
一个会导致**完全拿不到图**的 P0 缺陷，已修复。当前状态：

| 能力 | 状态 | 说明 |
|------|------|------|
| 文生图 | ✅ | 走 `POST /v1/images/generations`（返回 OpenAI 标准结构；`/v1/media/generations` 兼容保留） |
| 图生图（subject_reference） | ✅ | `/v1/images/edits` 传 `image`（multipart 或 URL），自动映射 |
| 宽高比 / 自定义尺寸 | ✅ | `resolution` 支持 `16:9` 或 `1024x1024` |
| 多张（n，1-9） | ✅ | 下发 + 按张计费 + 返回全部 URL |
| base64 返回 | ✅ | 转 data URI |
| 业务失败识别 | ✅ | HTTP 200 + `base_resp.status_code != 0` 判定为失败 |

### 已修复的 P0：响应解析完全错位

MiniMax 的响应 `data` 是**对象**不是数组：

```json
{
  "id": "03ff...",
  "data": { "image_urls": ["https://.../a.jpeg", "https://.../b.jpeg"] },
  "base_resp": { "status_code": 0, "status_msg": "success" }
}
```

原实现按 `data[0].url` 解析（OpenAI 风格），`data` 是 map 导致类型断言失败 → `InlineURL` 恒为空。
**表现**：任务状态成功但没有图，且因为 HTTP 200 不会触发 failover。

另外补上了业务失败判定：MiniMax 用 HTTP 200 + `base_resp.status_code != 0` 表达失败
（如内容安全拦截 1026），原来会被当成成功。

---

## 二、接入步骤

### 1. 添加账号

平台选 **openai**（本项目没有独立的 minimax 平台常量，MiniMax 走 OpenAI 兼容通道）。

| 字段 | 值 |
|------|-----|
| 平台 | `openai` |
| 类型 | API Key |
| base_url | `https://api.minimax.chat` （国际站用 `https://api.minimax.io`） |
| api_key | MiniMax 接口密钥 |

> **base_url 不要带 `/v1`**。adapter 会拼接成 `{base_url}/v1/image_generation`。
> 如需覆盖路径，在账号凭证里加 `video_create_path`（该凭证名沿用视频通道，图片 adapter 也读它）。

账号需能服务模型 `image-01`（否则选号阶段报 `no available account for model image-01`）。
可用 `model_mapping` 做别名映射，例如把客户端的 `minimax-image-01` 映射到上游 `image-01`。

### 2. 分组与计费

- 分组需允许该模型；图片计费走分组的**图片价格**（1K / 2K / 4K 三档）。
- 未配置分组图片价时回落到系统默认价，建议显式配置。
- 注意：媒体链路的异步图片**不走渠道定价**（只有分组定价），与视频异步链路不同。

### 3. 对象存储（强烈建议）

MiniMax 返回的图片 URL **有效期 24 小时**。配置了对象存储后，平台会自动下载转存为稳定 URL；
未配置时用户拿到的是会过期的临时链接。

---

## 三、调用方式

### 文生图

```bash
curl -X POST https://<你的域名>/v1/media/generations \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "image-01",
    "prompt": "一只戴墨镜的橘猫在海边冲浪",
    "resolution": "16:9",
    "n": 1
  }'
```

响应（图片是**同步**返回的，无需轮询）：

```json
{
  "id": "med_xxx",
  "status": "succeeded",
  "model": "image-01",
  "url": "https://.../a.jpeg",
  "urls": ["https://.../a.jpeg", "https://.../b.jpeg"],
  "created_at": "2026-09-13T10:00:00Z"
}
```

> `urls` 仅在 `n > 1` 时出现。轮询接口 `GET /v1/media/:id` 只返回首张 `url`
> （多张列表未落库，仅创建响应携带）。

### 图生图

```bash
-d '{
  "model": "image-01",
  "prompt": "女孩在图书馆的窗户前，看向远方",
  "resolution": "16:9",
  "image": "https://cdn.example.com/ref.jpg"
}'
```

`image` / `image_url` / `image_urls` 会自动映射为 MiniMax 的 `subject_reference`。
官方**每次仅支持一张参考图**，多张时取第一张。如需自定义 `type`（非 `character`），
可直接传 `subject_reference` 字段，此时不再自动覆盖。

### 自定义像素尺寸

`resolution` 传 `WxH` 时走 `width` / `height`（仅 `image-01` 支持）：

```json
{ "model": "image-01", "prompt": "...", "resolution": "1024x1024" }
```

取值范围 `[512, 2048]` 且必须是 **8 的倍数**（MiniMax 约束，平台不额外校验，非法值由上游返回错误）。
`width/height` 与 `aspect_ratio` 同时存在时，上游以 `aspect_ratio` 优先。

---

## 四、与 gpt-image-2 的调用差异

**调用方式已经一致**：MiniMax 图片模型现在可以直接打 `/v1/images/generations`
（`/v1/images/edits`），响应由平台改写成 OpenAI 标准结构，客户端零改造。

| | gpt-image-2 | MiniMax image-01 |
|---|---|---|
| 端点 | `POST /v1/images/generations` | **同一个端点**（`/v1/media/generations` 转为兼容保留） |
| 返回模式 | 同步，直接 `data[0].url` / `b64_json` | 同样是 `{created, data:[{url}]}`；`b64_json` 也支持 |
| 尺寸参数 | `size` | `resolution`（`size` 作为别名已兼容） |
| 尺寸写法 | `1024x1024` / `auto` | `16:9` 或 `1024x1024`；`auto` 视为未指定 |
| 图生图 | `POST /v1/images/edits`（multipart） | 同一端点 + `image` 字段 |
| 多张 | `n` | `n`（1-9，按张计费） |

路由仍按**模型名**决定：`gpt-image-2` 不会命中 MiniMax adapter，反之亦然——
客户端换的只有 `model` 字符串，端点与 body 写法完全一样。

> 客户端写 `minimax-image-01` 时网关会自动归一成官方枚举 `image-01`
> （见「已修复的 P0：模型名枚举」），两种写法都能出图。

`size` 别名是为了降低迁移成本而加的：照搬 OpenAI 写法传 `size` 现在能被正确识别，
而不是像之前那样被静默丢弃（上游按默认比例出图、计费按默认档）。

---

## 五、计费注意事项

1. **按张计费**：`n` 现在会下发并按张计费。此前上游出 N 张、平台只收 1 张，属系统性少收。
2. **宽高比按真实像素分档**：
   - `1:1` → 1024x1024 → **1K**
   - `16:9` → 1280x720 → **2K**
   - `9:16` / `21:9` / `4:3` / `3:2` / `2:3` / `3:4` → **2K**

   修复前所有宽高比一律回落默认 2K 档，`1:1`（实为 1K）按 2 倍价格收费。
   若你的分组只配了 2K 价未配 1K 价，`1:1` 会沿降档链（4K→2K→1K）回退并打
   `image_pricing_tier_fallback` 告警，建议补齐 1K 价。
3. **不传尺寸时按 2K 收费**（与上游默认 1:1 实际出 1K 不一致，偏高）。
   想按实收，显式传 `resolution: "1:1"` 或 `"1024x1024"`。

---

## 六、其他注意事项

- **不要传 `response_format: "base64"`**，除非确实需要：base64 会转成 data URI，
  体积大且无法被对象存储转存。默认（不传）即为 url，是推荐用法。
- **URL 24 小时过期**，见第二节第 3 点。
- **`prompt` 最长 1500 字符**，超出由上游报错。
- **`image-01-live` 与 `style` 参数**：adapter 未做特殊处理，需要时通过 Extra 透传。
- **失败 failover**：HTTP 4xx/5xx 会触发换号重试；业务失败（`base_resp.status_code != 0`）
  当前按失败处理，不换号。
