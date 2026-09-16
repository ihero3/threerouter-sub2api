# 客户端接入文档：一个 base_url，所有模型

> 面向调用方（客户端开发者）。你只需要三样东西：**站点地址、API Key、模型名**。
> 文本、生图、生视频、音频由服务端按 `model` 自动判定并路由，客户端不需要按模态挑端点。

## 0. 三件套

| 项目 | 值 | 说明 |
|---|---|---|
| `base_url` | `https://<你的站点域名>/v1` | **必须以 `/v1` 结尾**，站点域名由服务方提供 |
| `api_key` | `sk-xxxxxx` | 后台「API 密钥」页创建 |
| `model` | 见[第 3 节模型清单](#3-模型--模态对照表) | 大小写不敏感 |

请求头固定为：

```
Authorization: Bearer sk-xxxxxx
Content-Type: application/json
```

---

## 1. 端点一览

| 端点 | 用途 | 建议 |
|---|---|---|
| `POST /v1/chat/completions` | 文本对话 | ✅ OpenAI SDK 原生支持 |
| `POST /v1/images/generations` | 生图 / 图生图 | ✅ OpenAI SDK 原生支持 |
| `POST /v1/videos/generations` | 生视频（提交任务） | 提交后需轮询 |
| `POST /v1/audio/speech` | 语音合成 | 同步返回音频字节 |
| `POST /v1/media/generations` | **媒体总入口**（图/视频/音频三合一） | ✅ 推荐 |
| `POST /v1/generations` | **万能入口**（文本也走） | 自研端点，SDK 需 `client.post` |
| `GET /v1/media/{id}` | 查任务状态与结果 | 视频必用 |
| `GET /v1/media/{id}/content` | 下载产物内容（302 跳转） | 可选 |

**选型建议**

- 用 OpenAI 官方 SDK → 按模态用标准方法，**只换 `model` 就能切换厂商**（文本/图片零改造）。
- 想真正「一个端点吃所有模型」→ 用 `/v1/generations`，代价是 SDK 没有对应方法，需要手写 HTTP。
- 视频是异步的，**任何端点提交后都要轮询** `GET /v1/media/{id}`。

---

## 2. 快速开始

### curl

```bash
# 文本
curl https://<站点>/v1/chat/completions \
  -H "Authorization: Bearer sk-xxx" -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o","messages":[{"role":"user","content":"你好"}]}'

# 生图（阿里千问）
curl https://<站点>/v1/images/generations \
  -H "Authorization: Bearer sk-xxx" -H "Content-Type: application/json" \
  -d '{"model":"qwen-image-3.0-pro","prompt":"一只在屋顶上的橘猫","size":"1024x1024"}'

# 生视频（提交后拿到 id 再轮询）
curl https://<站点>/v1/media/generations \
  -H "Authorization: Bearer sk-xxx" -H "Content-Type: application/json" \
  -d '{"model":"seedance-1.0-pro","prompt":"海浪拍打礁石","resolution":"1080p","duration":10}'
```

### Python（OpenAI SDK）

```python
from openai import OpenAI

client = OpenAI(base_url="https://<站点>/v1", api_key="sk-xxx", timeout=600)

# 文本
client.chat.completions.create(model="gpt-4o", messages=[{"role": "user", "content": "hi"}])

# 生图：qwen-image / minimax-image-01 / gpt-image-2 写法完全一致
img = client.images.generate(model="qwen-image-3.0-pro", prompt="一只橘猫", size="1024x1024")
print(img.data[0].url)
```

### Node（OpenAI SDK）

```js
import OpenAI from "openai";
const client = new OpenAI({ baseURL: "https://<站点>/v1", apiKey: "sk-xxx", timeout: 600_000 });

const img = await client.images.generate({
  model: "minimax-image-01",
  prompt: "一只橘猫",
  size: "1024x1024",
});
```

---

## 3. 模型 → 模态对照表

服务端按模型名判定模态，判定顺序为 **图片 → 视频 → 音频 → 文本**；未收录的模型一律按**文本**处理（让上游给出权威错误，而不是本地拦掉）。

| 模态 | 模型 | 端点 |
|---|---|---|
| 图片 | `qwen-image-3.0`、`qwen-image-3.0-pro`（阿里千问图像 3.0） | `/v1/images/generations` |
| 图片 | `minimax-image-01`、`image-01`（MiniMax） | 同上 |
| 图片 | `gpt-image-2`、`gpt-image-1`、`dall-e-3` | 同上 |
| 图片 | `doubao-seedream-*`、`seedream-*`、`grok-imagine-image` | 同上 |
| 视频 | `seedance-*`、`doubao-seedance-*`、`minimax-hailuo-*`、`minimax-h*`（含 `minimax-h3`）、`grok-imagine-video` | `/v1/videos/generations` 或 `/v1/media/generations` |
| 视频 | `wan*-t2v`（文生视频）、`wan*-i2v`（图生视频）、`wan*-video`（全能：文生 / 图生首帧·首尾帧 / 参考生视频） | 同上 |
| 音频 | `tts-*`、`speech-01`、`whisper-*` | `/v1/audio/speech` |
| 文本 | 其余全部（`gpt-4o`、`claude-*`、`deepseek-*`、`gemini-*` …） | `/v1/chat/completions` |

> 新增模型会持续补充，判定逻辑在 `service.DispatchModelCapability`。

> **模型名会自动归一到厂商官方值**：客户端写 `minimax-image-01` 时，网关发给 MiniMax 的
> `model` 会自动转成官方枚举 `image-01`（MiniMax `/v1/image_generation` 只接受
> `image-01` / `image-01-live`）。因此**两种写法都可以**，计费与日志里记录的仍是客户端传的 `minimax-image-01`。

---

## 4. 生图

### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `model` | string | 是 | 见第 3 节 |
| `prompt` | string | 是 | 提示词 |
| `size` / `resolution` | string | 否 | 二者等价。支持 `1024x1024`、`1280x720`、`1:1`、`16:9`、`2K`、`4K` 等写法。**不传时按上游返回的真实尺寸计费** |
| `n` | int | 否 | 生成张数。千问上限 6、MiniMax 上限 9，超出自动收敛 |
| `image` / `image_url` / `image_urls` | string \| array | 否 | 图生图参考图，支持裸 URL、`{"url": "..."}`、数组 |
| `negative_prompt` | string | 否 | 负面提示词 |
| `seed` | int | 否 | 随机种子 |

其余厂商专有字段（如 `prompt_extend`、`watermark`、`subject_reference`）会原样透传给上游。

### 响应

```json
{
  "id": "img_xxxx",
  "status": "succeeded",
  "model": "qwen-image-3.0-pro",
  "url": "https://.../a.png",
  "urls": ["https://.../a.png", "https://.../b.png"],
  "created_at": "2026-09-13T10:00:00Z"
}
```

走 `/v1/images/generations` 时返回 OpenAI 标准结构 `{"created": ..., "data": [{"url": ...}]}`。

- 图片是**同步**返回：`status` 直接是 `succeeded`，`url` 可直接使用。
- `n>1` 时创建响应与**轮询接口都返回全量 `urls`**（已落库）。
- 产物 URL 为**临时签名地址**，请自行转存。

### 计费口径

按**张数 × 分辨率档位**计费（1K / 2K / 4K）。不传 `size` 时以上游回传的真实输出尺寸为准，不会按默认档多收。

每一次生图调用都会在「使用记录」里留一条 `image` 计费模式的明细（`n>1` 时按实际出图张数记 `image_count`），成功按实际张数扣费、失败记 0 费用行并退还预扣，与生视频、文本调用的口径一致。

---

## 5. 生视频

### 超时设置（重要，别设 60 秒）

视频**提交**这一步只是向上游登记任务、拿回 task_id，理论上很快，但厂商侧排队或
大素材上传都可能让它到分钟级（MiniMax 单图上限 30MB、请求体上限 64MB）。

客户端超时设得太短（典型是 60 秒）会**两败俱伤**：客户端拿到「请求超时」，服务端
同时报 `502 context canceled` —— 因为连接一断，已经在飞的上游请求被连带掐断。
极端情况下上游任务其实建好了，却因为记录没写下去成了孤儿任务。

| 环节 | 建议超时 |
|---|---|
| 提交（`POST /v1/videos/generations`、`/v1/media/generations`） | **≥ 300 秒**，保险起见 600 秒 |
| 每次轮询（`GET /v1/media/{id}`） | 30 秒即可 |
| 整体等待成片 | 10 分钟以上（长视频模型本身就要几分钟） |

> 服务端已做加固：上游请求不再绑定客户端连接的生命周期，客户端断开后仍会跑完
> 并落库。但这只是兜底，客户端该给的超时还是要给足。

### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `model` | string | 是 | 见第 3 节 |
| `prompt` | string | 是 | 提示词 |
| `resolution` / `size` | string | 否 | `480p` / `720p` / `1080p` |
| `duration` / `duration_sec` / `seconds` | int \| string | 否 | 时长（秒）。支持 `10`、`"10"`、`"10s"`、`"10 秒"`；`auto` / `-1` / 不传 = 用模型默认 |
| `image` / `image_url` | string | 否 | 图生视频**首帧** |
| `media` | array | 否 | 精细控制（wan 全能系列）：`[{"type":"first_frame","url":"..."},{"type":"last_frame","url":"..."},{"type":"reference_image","url":"..."}]`，分别对应首帧 / 首尾帧 / 参考生视频 |
| `seed` | int | 否 | 随机种子 |

> 时长不做上限钳制（部分模型支持 30 秒）。若客户端传值超过模型能力，**以上游真实返回时长计费**。
>
> **wan3.0-video（万相 3.0 全能）**：一个模型统一支持文生视频、图生视频（首帧/首尾帧）和参考生视频，最长 30 秒、30fps。用法示例：
>
> ```json
> // 图生视频（首帧）
> {"model":"wan3.0-video","prompt":"猫跳起来","image":"https://.../cat.png"}
> // 首尾帧
> {"model":"wan3.0-video","prompt":"猫从左跳到右",
>  "media":[{"type":"first_frame","url":"https://.../a.png"},
>           {"type":"last_frame","url":"https://.../b.png"}]}
> ```

> **MiniMax-H3 / MiniMax-H3-Max（全能参考视频）**：一个模型统一支持文生视频（T2V）、
> 图生视频（首帧 / 首尾帧）和参考生视频（参考图 / 参考视频 / 参考音频，最多 12 个文件）。
>
> 注意它家对 `resolution` 和 `ratio` 的要求跟别家不一样：
>
> | 模型 | resolution | duration | ratio |
> |---|---|---|---|
> | `MiniMax-H3` | `768P` / `2K` | 4～15 秒（整数） | 文生视频**必填**且不能是 `adaptive` |
> | `MiniMax-H3-Max` | `480P` / `768P` | 5～15 秒（整数） | 同上 |
>
> `ratio` 缺失时服务端会自动兜底（文生视频补 `16:9`、带素材补 `adaptive`），
> 但**建议显式传**，避免上游按默认比例出片。传了非法档位会返回
> 400 `invalid_request_error`（如给 H3 Max 传 `2K`）。
>
> ```json
> // 文生视频
> {"model":"MiniMax-H3","prompt":"镜头缓缓推向窗外街道","duration":5,"resolution":"768P","ratio":"16:9"}
> // 图生视频（首帧），ratio 由图片决定
> {"model":"MiniMax-H3","prompt":"画面中的人开始跳舞","image":"https://.../a.png","duration":5}
> ```

### 异步流程（必读）

```bash
# 1. 提交：立即返回 202，带 id
POST /v1/videos/generations   ->  {"id":"vid_xxx","status":"processing"}

# 2. 轮询（建议 3~5 秒一次，最长 10 分钟）
GET  /v1/media/vid_xxx        ->  {"id":"vid_xxx","status":"succeeded","url":"https://.../a.mp4"}

# 3. 下载（可选，302 跳转到真实地址）
GET  /v1/media/vid_xxx/content
```

`status` 取值：`processing` / `succeeded` / `failed` / `cancelled`；失败时带 `error` 字段。

**计费口径**：按**秒**计费，时长取上游真实值。任务失败不计费。

---

## 6. 音频

```bash
curl https://<站点>/v1/audio/speech \
  -H "Authorization: Bearer sk-xxx" -H "Content-Type: application/json" \
  -d '{"model":"tts-1","input":"你好，世界","voice":"alloy"}' --output out.mp3
```

同步返回音频字节（`audio/mpeg`）；异步任务回退到统一任务流程，按第 5 节轮询。

音频同样会记入「使用记录」（`per_request` 计费模式），按分组音频配置计价：配了每秒价按秒、否则按分钟。openai 组与 composite 组都统计。

---

## 7. 两个统一入口

### `/v1/generations`（一个端点吃所有模型）

```bash
# 同一端点，只换 model
-d '{"model":"gpt-4o","messages":[{"role":"user","content":"你好"}]}'          # 文本
-d '{"model":"qwen-image-3.0-pro","prompt":"一只猫","size":"1024x1024"}'        # 图片
-d '{"model":"seedance-1.0-pro","prompt":"海浪","resolution":"1080p"}'          # 视频
```

### `/v1/chat/completions` 的跨模态兜底

把图片/视频模型送进 chat 端点时，服务端会转到对应链路，并把结果**包装成 `chat.completion` 结构**，避免 SDK 解析崩溃：

```json
{"id":"chatcmpl-xxx","object":"chat.completion","model":"qwen-image-3.0-pro",
 "choices":[{"index":0,"message":{"role":"assistant","content":"![result-1](https://.../a.png)"}}]}
```

这是**兼容层**，拿不到原生字段（如图片张数计量），生产环境建议用对应模态的标准端点。

---

## 8. 错误码

统一返回 OpenAI 错误结构：

```json
{"error": {"message": "...", "type": "invalid_request_error", "code": "..."}}
```

| HTTP | `type` | 含义 / 处理建议 |
|---|---|---|
| 400 | `invalid_request_error` | 参数不合法（缺 `model`、`size` 档位非法）。检查请求体 |
| 400 | `invalid_request_error` | 消息为 `This response_format type is unavailable now`：上游不支持你发的 `response_format` 类型。DeepSeek 系已由网关自动降级为 `json_object`；若仍出现，说明该上游其它结构化类型也不支持，请改用 `json_object` |
| 401 | `authentication_error` | API Key 无效 |
| 403 | `permission_error` | 分组未开通该能力（如生图权限未开）。联系服务方开通 |
| 404 | `not_found_error` | 端点在当前分组平台下不支持 |
| 429 | `rate_limit_error` | 限流，按 `Retry-After` 退避重试。详见下方「429 处理」 |
| 402 / 配额类 | `billing_error` | 余额或平台配额不足，充值后重试 |
| 503 | `capacity_error` | 暂无可用渠道（上游账号全部不可用），稍后重试 |
| 502 | `api_error` | 消息含 `canceled before upstream responded`：**客户端提前断开**，把提交超时调到 300 秒以上。含 `timed out after` 则是上游确实没响应，可重试 |
| 5xx | `api_error` / `upstream_error` | 上游故障，可重试；保留 `id` 便于排查 |

### 结构化输出（`response_format`）

各上游对 OpenAI 结构化输出的支持不一致，网关已按上游能力自动适配：

| 上游模型 | `json_schema`（严格 schema） | `json_object`（合法 JSON） |
|---|---|---|
| DeepSeek（`deepseek-*`） | ❌ 上游会直接 400 `This response_format type is unavailable now`；**网关自动降级为 `json_object`**，并保证 prompt 内出现 "json" 关键词（缺失时自动补一条 system 提示，附带你发送的 schema 作为参考） | ✅ |
| Kimi（`kimi-*`） | ✅ 原生支持 | ✅ |
| OpenAI（`gpt-*` 等） | ✅ | ✅ |

对客户端的要求：

1. **仍然要自己校验返回值**。降级后上游只保证「输出是合法 JSON」，字段是否完全符合你的 schema 由模型尽力而为；业务侧请保留一次 `json.loads` + 字段校验。
2. 如果字段必须严格一致，**优先把 schema 的关键字段说明也写进 prompt**（不要只靠 `response_format`），降级链路上这最稳。
3. 想拿到明确的字段约束，请选用支持 `json_schema` 的模型（如 Kimi / OpenAI 系），而不是依赖降级。

### 429 处理（限流）

限流是**正常保护**，不是服务故障。响应同时带 `Retry-After` 头（秒）和可读文案：

```json
{"error": {
  "message": "Rate limit exceeded: current limit is 30 requests/minute, please retry after 42 seconds. 当前限额 30 次/分钟，请 42 秒后重试。如需更高限额请联系服务方调整 / Contact your provider to raise this limit.",
  "type": "rate_limit_error",
  "code": "rate_limit_exceeded"
}}
```

正确处理：

1. **读 `Retry-After` 头再退避**，不要用固定间隔猛重试。OpenAI 官方 SDK 会自动读这个头并重试，手写 HTTP 客户端要自己实现。
2. **不要用「立即重试」**：被拒绝的请求同样计入窗口计数，紧耦合重试会让计数继续上涨、窗口更难腾出空间，表现为「一直报错」。
3. 长期撞限说明当前的次/分钟额度不够用，联系服务方调整，而不是在客户端加重试次数。

---

## 9. 客户端适配检查清单

- [ ] `base_url` 以 `/v1` 结尾（不是站点根域）
- [ ] **生图**超时 ≥ 600 秒（千问图像默认开 prompt_extend + 思考模式，官方建议 600s 起）
- [ ] **生视频提交**超时 ≥ 300 秒（保险 600 秒）—— 设成 60 秒会拿到「请求超时」，
      服务端同时出现 `502 context canceled`
- [ ] 视频必须实现轮询，不能假设同步返回；单次轮询 30 秒超时即可
- [ ] 产物 URL 是临时签名，及时转存
- [ ] 生图失败 403 时提示用户联系服务方开通生图权限
- [ ] **429 必须读 `Retry-After` 头退避**，不要立即重试（见第 8 节「429 处理」）
- [ ] 对 5xx 做指数退避，并用 `Idempotency-Key` 头防重复扣费（媒体创建支持幂等）

---

## 10. 服务方建组说明（运营侧，客户端可跳过）

先分清两个「平台」，选型就清楚了：

- **分组平台**（创建分组时下拉框里的 OpenAI / Gemini / Grok / Kimi / Zhipu GLM / DeepSeek / Composite…）：决定**从哪个账号池里选号**（`SelectAccountForModel` 用 `group.Platform` 过滤）。Composite 与 DeepSeek 是**并列的分组平台选项**，不是从属关系。
- **账号平台**（添加上游账号时填的平台字段）：决定这个账号**归入哪个池子**。

两条底层规则：

1. **适配器按「模型名」匹配，与账号平台无关**——`qwen-image-*` 永远走千问适配器，`minimax-image-01 / image-01` 永远走 MiniMax 适配器，不会因为账号平台是 openai 就发错协议。
2. **分组平台决定选号池**：openai 分组只选 `platform=openai` 的账号；composite 分组先按模型解析目标平台，再去对应池子选号。

| 分组平台 | 账号怎么填 | 媒体可用性 |
|---|---|---|
| **openai** | 把千问 / MiniMax / 火山等上游账号的**账号平台都填 openai**（upstream 类型，base_url + api_key） | 媒体全通，配置最少 |
| **composite** | 分组平台选 Composite；各厂商账号添加进该分组，**账号平台按厂商对齐**：千问 / MiniMax / 火山等国产媒体厂商填 **DeepSeek**（沿用视频厂商的承载约定），OpenAI 系填 openai，Grok 系填 grok | 媒体全通，且支持跨厂商混用 |
| deepseek | 账号平台填 DeepSeek | 只是纯 DeepSeek 文本分组，**不适合**当媒体承载（媒体账号应挂在 openai 或 composite 分组下） |
| 其他（anthropic / gemini …） | 对应平台 | 仅限该平台原生能力 |

**结论**

- 追求「一个分组跑通所有模型、配置最少」→ 选 **openai 分组平台**，把所有媒体上游账号的账号平台都填 openai。
- 追求「同一分组跨厂商混用（claude + gemini + 国内厂商）」→ 选 **Composite 分组平台**，账号平台按厂商对齐（国产媒体厂商填 DeepSeek，仅指账号字段）；解析不出目标平台会报 `composite target platform unknown`。

**渠道（上游账号）是前置条件**：模型能被识别 ≠ 能调用。后台必须存在对应上游账号且声明了该模型，否则返回 503 `capacity_error`。

**开户注意**：媒体账号只登记媒体模型（通过 `model_mapping`），避免被文本请求选中；账号凭证需要 `base_url` + `api_key` 两项。

---

## 11. 相关文档

- `docs/unified-api-entrypoint.md` — 统一入口的设计与实现细节
- `docs/qwen-image-integration.md` — 阿里千问图像接入说明
- `docs/minimax-image-integration.md` — MiniMax 生图接入说明
- `docs/video-billing-audit.md` — 视频计费口径审计
