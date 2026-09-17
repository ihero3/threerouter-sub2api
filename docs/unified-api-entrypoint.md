# 统一调用入口：base_url + api_key + model

目标：**客户端只关心三个东西**——站点地址、API Key、模型名。
至于这个模型是文本、生图还是生视频，由服务端按模型名自动判定并路由。

> 一句话：配好 `base_url` 与 `api_key`，剩下的只换 `model` 字符串。

## 一、端点一览（推荐用这 3 个）

| 端点 | 用途 | 响应契约 |
|---|---|---|
| `POST /v1/chat/completions` | 文本 | OpenAI `chat.completion` |
| `POST /v1/images/generations`（`/v1/images/edits` 同） | 图片 | OpenAI `ImagesResponse`：`{created, data:[{url\|b64_json}]}` |
| `POST /v1/videos/generations`（`/v1/videos` 同） | 视频 | 任务结构 `{id, status, url, ...}` |

这三个端点在任何分组（openai / composite / deepseek / kimi / zhipu …）下
**返回结构完全一致**，用户不需要知道自己被分在哪个组。

### 兼容保留（不推荐新接入）

| 端点 | 状态 | 说明 |
|---|---|---|
| `POST /v1/media/generations`、`POST /v1/media` | 兼容保留 | 图片/视频/音频三合一，返回自研任务结构 `{id,status,url}` |
| `POST /v1/generations` | 兼容保留 | 万能入口，按 model 分派；自研端点，OpenAI SDK 无对应方法 |
| `POST /v1/video-tasks` | **只读兼容，不再维护** | 历史视频任务链路（`video_tasks` 表）。已创建的任务仍可查询，新接入请走 `/v1/videos/generations` |
| `POST /v1/images/generations/async`、`POST /v1/images/edits/async` | 兼容保留 | 异步生图（依赖对象存储），见「高级用法」 |
| `POST /v1/images/batches` 及其子路由 | **并列能力，非降级** | 批量生图，场景真实存在但使用频率低，见「高级用法」 |

## 二、模型 → 模态分派规则

判定函数：`service.DispatchModelCapability(model)`（`internal/service/model_capability.go`）。
顺序为 **图片 → 视频 → 音频 → 文本**。

匹配方式是**子串包含**（不是精确枚举）：规则命中的是"模型名里含有这些关键词"，
所以 `qwen-image-3.0-pro`、`minimax-image-02` 这类新型号**自动覆盖，不需要逐个收录**。
无法识别的模型按**文本**处理——文本是默认链路，让上游给出权威错误比本地猜测安全。

| 模态 | 命中规则（子串/前缀） | 示例 |
|---|---|---|
| 图片 | 名含 `qwen-image`、`minimax-image`、`image-01`、`seedream`、`gpt-image`、`dall-e`、`wanx`、`t2i`、`-image`，或 `grok-imagine-image` | `qwen-image-3.0-pro`、`minimax-image-01`、`wanx2.1`、`doubao-seedream-4.0` |
| 视频 | 名含 `seedance`、`jimeng-video`、`grok-imagine-video`；前缀 `minimax-h*`（覆盖 hailuo/h1/h3/h4）、`minimax-video`；**`wan` 开头**且含 `video`/`t2v`/`i2v`，或 `wan2*`/`wan3*` 开头 | `wan3.0-video`、`wan2.2-t2v-plus`、`minimax-hailuo-02`、`seedance-1.0-pro` |
| 音频 | 名含 `tts`、`stt`、`whisper`、`speech`、`voice`、`cosyvoice`、`t2a`、`realtime` | `speech-01`、`whisper-1`、`cosyvoice-v2` |
| 文本 | 其余全部——任何不命中以上关键词的模型 | `gpt-4o`、`claude-*`、`deepseek-*`、`gemini-*`、`grok-4`、`minimax-m3`、`kimi-k3`、`qwen3.8-max`、`glm-5.3` |

两个细节：
- 媒体端点内部用的 `MediaKindFromModel`（`media_adapter.go`）关键词与上表一致，
  但**未收录模型默认判为 video**——语义是"交给账号池决定能不能做"；
  而上面的 `DispatchModelCapability` 对未收录模型判为**文本**。两者用途不同，不算矛盾。
- 子串匹配是双刃剑：`-image`、`voice`、`speech` 这类宽泛关键词可能误伤名字里
  碰巧带这些词的模型。手动加规则时优先用**厂商前缀 + HasPrefix**，少用宽泛子串。

### 文本模型为什么不会被误判成图片 / 视频

国内厂商的文本模型名字里往往带厂商前缀和版本号，看着和媒体模型很像，但**媒体规则
命中的是完整关键词，不是厂商名**：

| 文本模型 | 为什么不进媒体链路 |
|---|---|
| `minimax-m3` / `minimax-m2` | 图片规则要 `minimax-image`，视频要 `minimax-h*` / `minimax-video`；`minimax-m*` 都不匹配 |
| `kimi-k3` / `kimi-k2` / `moonshot-v1-*` | 无任何媒体关键词 |
| `qwen3.8-max` / `qwen-plus` / `qwen-long` | 图片规则要 `qwen-image`；只含 `qwen` 不算 |
| `glm-5.3` / `glm-4v` | `glm-4v` 的 `v` 是 vision（视觉理解，走 chat），不是 video |

反过来，**带 `v` 不等于视频**：`glm-4v`、`qwen-vl-max` 都是视觉语言模型，通过
`/v1/chat/completions` 调用，判为文本是正确的。

这些归属已固化在 `media_dispatch_rules_test.go` 的 `TestTextModelsAreNotMisrouted` 与
`TestTextModelsStayTextAcrossFutureVersions` 里——覆盖了 `minimax-m4`、`kimi-k4`、
`qwen9.9-max`、`glm-10.0` 等未来型号。以后有人加了宽泛关键词把文本模型带偏，测试会直接失败。
- `wan` 前缀下藏着两个厂商系列：**`wan2*`/`wan3*`（视频）与 `wanx*`（图像，
  通义万相）**。曾因共用前缀被一并收进视频规则，导致 `wanx2.1` 生图请求掉进视频链路，
  已在 `media_dispatch_rules_test.go` 里加回归测试守住。

### 规则维护在哪、怎么改

| 内容 | 文件 | 函数 |
|---|---|---|
| 分派入口（图→视→音→文） | `internal/service/model_capability.go` | `DispatchModelCapability` |
| 图片规则 | `internal/service/media_adapter.go` | `IsKnownImageVendorModel` |
| 视频规则 | `internal/service/video_adapter.go` | `IsKnownVideoVendorModel` |
| 音频规则 | `internal/service/media_adapter.go` | `IsKnownAudioVendorModel` |
| 媒体端点分派 | `internal/service/media_adapter.go` | `MediaKindFromModel` |

可以手动改，就是普通 Go 函数的关键词匹配。注意三件事：

1. **改完必须重新编译并重启才生效**（本项目不用容器）：
   ```bash
   cd /home/ubuntu/threerouter-sub2api/backend && go build -o sub2api ./cmd/server
   sudo install -m 0755 sub2api /opt/sub2api/sub2api && sudo systemctl restart sub2api
   ```
2. **图片与视频规则要同步改**。`MediaKindFromModel` 与 `DispatchModelCapability`
   是两套函数、共用同一份关键词语义，只改一处会让同一个模型在两个入口走不同链路
   （`TestDispatchAgreesWithMediaKindForKnownModels` 就是守这条的）。
3. **加完补一个用例**到 `internal/service/media_dispatch_rules_test.go`，
   否则下次有人改关键词把你的规则冲掉了没人知道。

## 三、用法示例

### OpenAI SDK（推荐，零改造）

```python
from openai import OpenAI
client = OpenAI(base_url="https://your-site/v1", api_key="sk-xxx")

# 文本
client.chat.completions.create(model="gpt-4o", messages=[{"role": "user", "content": "hi"}])

# 生图：qwen-image / minimax-image / image-01 都能用，且拿到的是标准 ImagesResponse
resp = client.images.generate(model="qwen-image-3.0", prompt="一只猫", size="1024x1024")
print(resp.data[0].url)

# 要 base64 而不是 URL
resp = client.images.generate(
    model="minimax-image-01", prompt="一只猫",
    size="1024x1024", response_format="b64_json",
)
open("cat.png", "wb").write(base64.b64decode(resp.data[0].b64_json))
```

### curl

```bash
# 生图（model 可省略，缺省 qwen-image-3.0）
curl -X POST https://your-site/v1/images/generations \
  -H "Authorization: Bearer sk-xxx" -H "Content-Type: application/json" \
  -d '{"model":"qwen-image-3.0","prompt":"一只在屋顶上的猫","size":"1024x1024"}'

# 生视频（异步；带 ?wait=180 可同步等待最多 180 秒）
curl -X POST "https://your-site/v1/videos/generations?wait=180" \
  -H "Authorization: Bearer sk-xxx" -H "Content-Type: application/json" \
  -d '{"model":"seedance-1.0-pro","prompt":"海浪拍打礁石","resolution":"1080p"}'
```

### 图生图（/v1/images/edits）

支持 `multipart/form-data`（OpenAI SDK 的默认写法）与 JSON 两种：

```python
client.images.edit(
    model="qwen-image-3.0",
    image=open("cat.png", "rb"),
    prompt="换成水墨风格",
)
```

上传的文件会被转成 data URI 交给统一媒体链路。

## 四、生图端点行为约定

| 项 | 约定 |
|---|---|
| 响应结构 | `{created, data:[{url}]}`，与 OpenAI 完全一致；多图（n>1）时 `data` 有多项 |
| 成功状态码 | `200`（此前媒体链路固定回 202，OpenAI SDK 会校验失败） |
| `response_format=url` | 返回可访问的 `url`（未配置对象存储时，部分上游回的是 data URI） |
| `response_format=b64_json` | 返回**裸 base64**（不含 `data:` 前缀），由服务端下载/解码后转码 |
| 默认模型 | 未传 `model` 时使用 `qwen-image-3.0` |
| 失败 | OpenAI 错误结构 `{"error":{"type":...,"message":...}}`，`message` 带上上游原因 |
| 超时未出图 | `504` + 任务 ID，提示轮询 `GET /v1/media/{id}` |

`response_format` 由平台消费后删除，**不会透传给上游**（各家该字段语义不一，
原样下发会被当成未知参数拒绝）。

## 五、生视频端点行为约定

| 项 | 约定 |
|---|---|
| 路由 | 除 Grok 原生链路外，所有视频模型统一走 `MediaTaskService`（`media_tasks` 表） |
| 默认行为 | 异步：`202` + `{id, status:"processing"}`，用 `GET /v1/videos/{id}` 轮询 |
| 同步等待 | `?wait=N`（秒，上限 **180**）服务端内部轮询，N 秒内出结果直接返回，省去客户端写轮询 |
| 历史任务 | `vid_` 开头的任务会先查 `media_tasks`，查不到再回退 `video_tasks`，老任务不会 404 |
| 产物下载 | `GET /v1/videos/{id}/content` → 302 到实际地址 |
| 模态校验 | 图片/音频模型打到本端点会被 **400** 拦在创建任务之前，并提示该用哪个端点 |

最后一条是刻意设计的：任务的 ID 前缀由模态决定（视频是 `vid_`），而
`GET /v1/videos/{id}` 只对 `vid_` 做媒体表路由。若放图片模型进来，会建出一个
`img_` 任务——费用照扣、任务照跑，客户端却永远查不到状态。未知模型仍然放行，
能不能做交给账号池决定。

## 六、chat/completions 的跨模态行为

把图片/视频/音频模型送进 `chat/completions` 时，服务端会转到对应链路，
**并把结果包装成 `chat.completion` 结构**返回，避免 OpenAI SDK 解析崩溃：

```json
{
  "id": "chatcmpl-xxx",
  "object": "chat.completion",
  "model": "qwen-image-3.0",
  "choices": [{
    "index": 0,
    "message": {
      "role": "assistant",
      "content": "![result-1](https://.../a.png)\n\n分辨率：1024x1024"
    },
    "finish_reason": "stop"
  }]
}
```

- 图片同步返回 → `content` 是 Markdown 图片链接（多张则多行）
- 视频异步任务 → `content` 给出任务状态与 ID，提示用 `GET /v1/media/{id}` 轮询
- 上游失败 → 转成 OpenAI 错误结构，SDK 抛出可读异常

**注意**：这是兼容层。生产环境仍建议用对应模态的标准端点，
跨模态调用拿不到原生字段（如 `usage` 里的图片计量）。

## 七、高级用法：批量与异步生图

批量生图是**并列能力**，不是被降级的能力：代码与文档都保留，只是放在高级场景里。

| 场景 | 端点 | 说明 |
|---|---|---|
| 单张/少量出图 | `POST /v1/images/generations` | 首选，同步返回 |
| 一次几十上百张 | `POST /v1/images/batches` | 批量任务 + 子路由（列表 / 条目 / 下载 / 取消 / 删除） |
| 需要异步 + 对象存储 | `POST /v1/images/generations/async` | 返回 `{task_id, poll_url}`，需开启对象存储 |

## 八、排障：选不到通道时

选号阶段失败会返回 `503 capacity_error`，message 形如：

```
当前分组没有可服务模型 qwen-image-3.0 的可用账号：请在该分组内添加支持此模型
的账号并挂上该模型，或改用 composite 分组由平台按模型自动选择通道
```

这一步发生在计费之前，任务表与用量明细都不会有记录，因此错误信息是唯一线索。
最常见的两类原因：

1. 分组内账号没挂该模型（或 `model_mapping` 缺失）
2. 分组平台与该模型不匹配 —— 用 **composite** 分组可让平台按模型自动选通道

## 八·补、生图端点的错误码

生图端点在"创建任务之前"就会做校验，因此下面这些错误**都不会产生费用**：

| 状态码 | type | 触发条件 |
|---|---|---|
| `400` | `invalid_request_error` | model 不是图片模型（已知视频模型会提示改用 `/v1/videos/generations`；其余提示可省略 model 用默认值） |
| `409` | `task_cancelled` | 任务被取消。不是上游故障，重新提交即可 |
| `502` | `upstream_error` | 上游失败、产物缺 URL，或 `b64_json` 转码失败 |
| `504` | `timeout_error` | 等待窗口内还没出图（默认 120 秒）。响应里带任务 ID 与轮询地址，任务仍在继续 |

`response_format=b64_json` 是契约而非建议：服务端会下载产物转 base64 返回，
转不出来就报 `502`，**不会静默换成 url** —— 客户端拿 url 当 base64 解码会直接崩。

## 九、相关文件

- `internal/service/model_capability.go` — 模态分派器
- `internal/server/routes/unified_dispatch.go` — 统一入口与 chat 响应适配
- `internal/server/routes/gateway.go` — 端点注册与分派接线
- `internal/handler/media_gateway_images.go` — 生图 OpenAI 响应适配（含 b64_json）
- `internal/handler/media_gateway_handler.go` — 统一媒体创建流程与 `?wait` 等待
- `internal/service/media_task_wait.go` — 有上限的同步等待
- `internal/service/media_adapter.go` / `video_adapter.go` — 厂商模型识别函数
