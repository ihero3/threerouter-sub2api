# Asynchronous Image Tasks

> **端点定位**：本文描述的是**兼容保留**能力（`POST /v1/images/generations/async` 等）。
> 首选端点仍是同步的 `POST /v1/images/generations`——它在任何分组下都返回 OpenAI 标准
> `ImagesResponse`，且已内置同步等待（异步图片上游最多等 120 秒）。
> 只有在需要"提交后立刻断开、稍后再取"时才用本套异步端点（对象存储配不配都能用，
> 区别见 [Enabling the feature](#enabling-the-feature)）。
> **需要幂等重试、或需要按 `request_id` 找回超时任务时，本套异步端点是唯一提供该保证的
> 路径**（详见下文 Idempotency 与 HTTP status code contract）。
> 批量场景另见 `POST /v1/images/batches`（并列能力，非降级）。

Asynchronous image tasks let clients submit long-running OpenAI-compatible image requests without keeping one HTTP connection open. This avoids proxy/CDN response timeouts such as Cloudflare 524 while preserving the existing image routing, billing, moderation, concurrency, and failover behavior.

## Endpoints

The authenticated gateway exposes both `/v1` paths and their existing no-prefix aliases:

```text
POST /v1/images/generations/async
POST /v1/images/edits/async
GET  /v1/images/tasks/{task_id}
GET  /v1/images/generations/by-request/{request_id}
```

The aliases are `/images/generations/async`, `/images/edits/async`, `/images/tasks/{task_id}`, and `/images/generations/by-request/{request_id}`.

Only OpenAI and Grok groups are supported. Requests use the same JSON or multipart payload as the corresponding synchronous endpoint. Streaming image requests are rejected because a polled task returns one final JSON result.

## Enabling the feature

异步图片任务有三种运行形态，取决于后台「Async image object storage」开关与凭证是否齐全：

| 形态 | 条件 | `/async` 提交 | 产物约定 |
| --- | --- | --- | --- |
| **完全离线** | 开关关闭 | `404 async image tasks are not enabled`，不创建任务、不写 Redis | — |
| **URL 透传** | 开关打开但凭证不全 | `202`，正常异步 | **产物必须是 http(s) 链接**；见下文 |
| **完全转存** | 开关打开且凭证齐全 | `202`，正常异步 | 全部图片转存对象存储，Redis 只留小 JSON |

第二行（URL 透传）是新形态，它解决的是「我想用异步，但不想为了异步专门配一套对象存储」。
对象存储真正要挡的是「几 MB 的内嵌产物躺进 Redis 24 小时」，不是 URL：上游返回的 http 链接
只有几十字节，直接存任务记录转发完全没问题。所以凭证不全不再整体禁用异步，而是降级为
"只接受 URL 类产物"。

URL 透传形态下，任务完成时会对产物做一次校验，以下情况明确失败（**不会被静默放行**）：

- 产物含 `b64_json` 或 `data:` URI —— 这类产物每张几 MB 且要在 Redis 里留存 24 小时，不能放行；
- `data` 数组为空、没有 `data` 字段、或某个条目既无 `url` 也无 `b64_json` ——
  默默 succeeded 就等于"扣了钱不给货"，宁可明确失败。

失败时任务的 `error.message` 会给出可执行的出路，例如
`image 0 is inline base64: configure image object storage, request response_format=url, or use the synchronous endpoint`。

为避免"上游生成完了（已经扣费）才发现存不下"，提交时会提前检查：没有对象存储而请求显式
要求 `response_format=b64_json`（JSON 与 multipart 两种写法都覆盖），直接 `400`，不生成、
不扣费。**这里刻意不做静默降级**（悄悄把 b64 换成 url）：客户端显式要什么就必须给什么，
给不出要报错，不能替它做主。

### From the admin UI (recommended)

**Admin → Backup → Async image object storage.** Saving the form takes effect immediately — the object-storage client is rebuilt on the next request, so there is no container restart.

Because the async image storage and the database backup share one S3 client, the form defaults to **reusing the backup S3 configuration**: it borrows the endpoint, region and credentials already configured above and keeps only its own bucket and prefix, so backups stay under `backups/` while images go to `images/`. Leave the bucket empty to use the backup bucket as well. Untick the box to point images at a completely separate account.

Saving requires step-up 2FA when that gate is enabled, for the same reason the backup S3 form does: changing the target redirects generated content to another account.

Turning the switch off stops new submissions but keeps already-accepted tasks pollable, so nothing in flight is stranded.

### From the config file

The admin setting takes precedence. When nothing has ever been saved there, the `image_storage` block in `config.yaml` is used instead, so deployments that enabled the feature before the admin UI existed keep working untouched.

Configure an S3-compatible object store (AWS S3, Cloudflare R2, Aliyun OSS, MinIO, …) in `config.yaml` (all keys also accept the `IMAGE_STORAGE_*` environment overrides):

```yaml
image_storage:
  enabled: true
  endpoint: "https://<account_id>.r2.cloudflarestorage.com"  # AWS 官方可留空
  region: "auto"
  bucket: "my-images"
  access_key_id: "..."
  secret_access_key: "..."
  prefix: "images/"
  force_path_style: false          # MinIO/path-style buckets set true
  public_base_url: ""              # set to return public_base_url/key直链; empty → presigned URL
  presign_expiry_hours: 24         # presigned link TTL when public_base_url is empty
  max_download_bytes: 33554432     # cap when re-hosting an upstream image URL (32MB)
```

When a task completes, each generated image is uploaded to the bucket and the result is rewritten to a compact form: `data[].url` points at the stored object (a permanent `public_base_url/key` link, or a time-limited presigned URL) and `b64_json` is removed. Only this small JSON is stored in Redis. If an upload fails, the task is marked `failed` rather than persisting the raw base64.

To support a different vendor beyond the S3-compatible client, implement the `service.ImageStorage` interface (`Save(ctx, key, contentType, data) (url, error)`) and provide it in place of the S3 implementation.

### Troubleshooting: the endpoints return 404 after enabling

`404 async image tasks are not enabled` 现在只有一个含义：**开关完全关闭**。
凭证不全不再产生这个 404——那种情况会降级为 URL 透传并正常接受任务（见上表）。

Check the startup log for:

```text
WARN image_storage is enabled but not fully configured; async image tasks fall back to url passthrough  missing_keys=[...]
```

出现这条日志**不代表功能不可用**，只代表降级为 URL 透传；`missing_keys` 列出了补齐凭证所需的键。
只有在日志里看到 `async image tasks are disabled` 才是真的没启用。

Note that releases **before v0.1.161 silently dropped `IMAGE_STORAGE_ENDPOINT`, `_BUCKET`, `_ACCESS_KEY_ID`, `_SECRET_ACCESS_KEY` and `_PUBLIC_BASE_URL`** when they were supplied only through the environment: those keys had no registered default, and viper cannot see an environment variable for a key it does not already know about. Deployments driven purely by `environment:` — which is what `deploy/docker-compose.yml` does by default — therefore reported `enabled: true` with empty credentials and 404'd on every async call. On an affected release the workaround is to also place the `image_storage` block in `/app/data/config.yaml` (copy it from `deploy/config.example.yaml`); once the keys exist in the file, the environment overrides apply normally.

Two further causes of a 404 that are unrelated to storage: the API key's group must be on the **OpenAI or Grok** platform (any other platform, or a key with no group at all, yields `Images API is not supported for this platform`), and a task may only be polled with the **same API key that submitted it** — polling with a different key of the same user returns `image task not found` by design.

## Submit a task

```bash
curl -i https://api.example.com/v1/images/generations/async \
  -H 'Authorization: Bearer sk-...' \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: req-8f2c1a94' \
  -d '{
    "model": "gpt-image-1",
    "prompt": "A lighthouse during a winter storm",
    "size": "1536x1024"
  }'
```

The server stores the initial task in Redis and responds with `202 Accepted`:

```json
{
  "id": "imgtask_0123456789abcdef",
  "task_id": "imgtask_0123456789abcdef",
  "object": "image.generation.task",
  "status": "processing",
  "request_id": "req-8f2c1a94",
  "created_at": 1784092800,
  "expires_at": 1784179200,
  "poll_url": "/v1/images/tasks/imgtask_0123456789abcdef"
}
```

`Location` contains the polling path and `Retry-After: 3` provides the recommended polling interval.

## Idempotency

Submitting the same work twice is the expensive failure mode for image generation: it burns upstream spend and the caller's balance twice over. Supply a unique key per logical request and the server guarantees it creates the task exactly once.

Two equivalent ways to send it — the header takes precedence when both are present:

```text
Idempotency-Key: <unique-request-id>          # header form
{ "request_id": "<unique-request-id>", ... }  # body form
```

Behaviour on repeat:

| Situation | Response |
|---|---|
| Same key, same payload | `202` with the **original** body (same `task_id`) plus `X-Idempotency-Replayed: true` |
| Same key, different payload | `409` `IDEMPOTENCY_KEY_CONFLICT` — the key was reused for different work |
| Same key, first request still in flight | `409` `IDEMPOTENCY_IN_PROGRESS` with `Retry-After` |

Replay is a normal `202`, not a `409`, because the caller's intent was fulfilled: the task exists and the same `task_id` is returned. `409` is reserved for genuine conflicts.

Keys are scoped per API key — the same unit that owns the task, so polling and recovery always agree. Keys are remembered for 24 hours; a key is reusable once its record expires.

The key is never forwarded upstream (OpenAI's API rejects unknown top-level arguments), and `request_id` is stripped from the body before the request is dispatched.

Omitting the key is allowed and preserves the legacy behaviour — every submission creates a new task. That is exactly the case that risks duplicate generation, so clients retrying on timeout should always send one.

### Same key, same sources, on every create endpoint

The two key sources above are accepted by every generation endpoint that creates a task in the unified media pipeline — the same client can switch endpoints without changing how it guards retries:

| Endpoint | Header key | Body `request_id` |
|---|---|---|
| `POST /v1/images/generations/async` | yes | yes |
| `POST /v1/images/generations` and `/v1/images/edits` | yes | yes |
| `POST /v1/media/generations` | yes | yes |
| `POST /v1/videos/generations` (and `/videos/edits`, `/videos/extensions`) | yes | yes |

Replays are marked with `X-Idempotency-Replayed: true` on all of them. The non-async endpoints answer `202` with a task id, so they are equally safe to poll via `GET /v1/videos/{task_id}` (or the images polling endpoint for image tasks).

> **Grok groups are not covered.** When the group's platform is `grok`, the entries above are
> forwarded to xAI's native Imagine API instead of the task pipeline, so no key is honoured
> and no task is stored. See "One exception: the Grok passthrough" below.

### One exception: the OpenAI-native passthrough

When the group's platform is `openai` **and** the model is an OpenAI-native image model (`gpt-image-*`), `POST /v1/images/generations` is a plain synchronous passthrough to the upstream and is **not** covered by the task pipeline: no key is honoured, no task is stored, and there is no `by-request` recovery. If the response is lost, the upstream generation still happened and it cannot be recovered — send such requests to `POST /v1/images/generations/async` when exactly-once semantics matter, or accept the duplicate-spend risk. Every other group/model combination goes through the task pipeline and honours both key sources.

### One exception: the Grok passthrough

When the group's platform is `grok`, `POST /v1/images/generations`, `/v1/images/edits`, `/v1/videos/generations`, `/v1/videos/edits` and `/v1/videos/extensions` are forwarded straight to xAI's Imagine API. xAI runs the job asynchronously and answers with its own `request_id`, which `GET /v1/videos/{request_id}` then looks up. There is no gateway task row, so **neither key source is honoured on this path** — a retry creates a second upstream job, exactly like the OpenAI-native passthrough above.

A body `request_id` is removed before the request is dispatched. xAI's request schema has no such field, so forwarding it would both leak the caller's key upstream and risk a `400` from a strict validator; on this path the field is simply inert. Use a `composite` group — or any other platform — when exactly-once semantics matter.

## Recover a task by request_id

If the submit response is lost to a network timeout, a dropped connection, or a `502`, the caller is left holding only the key. Fetch the original task with it rather than resubmitting:

```bash
curl https://api.example.com/v1/images/generations/by-request/req-8f2c1a94 \
  -H 'Authorization: Bearer sk-...'
```

This returns `200` with the same task object as the polling endpoint (including `request_id`), or `404` when the key is unknown — or was never successfully submitted, in which case there is no task to recover and the caller may safely resubmit with a fresh key.

## Client playbook: 怎么轮询、怎么最快拿到图

这一段是给客户端实现者的完整动作说明，照抄即可。

### 1. 提交：务必带幂等键

```http
POST /v1/images/generations/async
Authorization: Bearer sk-...
Idempotency-Key: <你这边生成的唯一键，例如 uuid v4>
Content-Type: application/json

{"model":"gpt-image-1","prompt":"..."}
```

不要带 `response_format=b64_json`——除非服务端配了对象存储，否则会被 `400` 拒绝（详见上文表格）。
不带这个字段时上游默认返回 `url`，正是 URL 透传形态需要的产物。

### 2. 轮询：间隔 3 秒，别更快

服务端会在 processing 期间返回 `Retry-After: 3`。**照这个值等，不要用 100ms 死循环**：

```text
interval = Retry-After（缺省 3 秒）
最多轮询到 expires_at（提交起 24 小时）
status == "processing" → 继续等
status == "succeeded"  → 立刻取图（见第 3 步）
status == "failed"     → 读 error.message，不要重试同一 request_id
```

CPU 与 QPS 都省下来了，而且 `succeeded` 的到达延迟几乎不受影响——因为服务端是在任
务完成那一刻就写好状态的，轮询只是去取，不是去催。

### 3. 取图：拿到 succeeded 就立刻下载，不要延迟

这是唯一需要客户端配合的点。

异步把"出图"和"取图"在时间上解耦了。转存模式下返回的是对象存储链接（长期有效）；
**URL 透传模式下返回的是上游厂商的原始链接，各家有效期不同（多为数小时到数天），
本仓库里没有逐家的实测依据**。所以规则很简单：

**一旦拿到 `status == succeeded`，立刻并发下载 `result.data[].url` 到你自己的存储，
不要再排队、不要再等用户点击、不要只把 URL 存数据库然后过几小时才用。**

推荐的落地方式：

```text
succeeded
  → 并发下载所有 url（各自的超时建议 30 秒，带 1 次重试）
  → 落到你自己的存储 / 本地文件
  → 之后再展示给用户的，用你自己那份
```

下载失败（403 / 404）说明上游链接已过期，此时：

- 幂等键不变，**重新提交一次**（重放会直接返回原任务的 `202`，不会重复扣费）；
- 如果任务已经过期查不到了（`IMAGE_TASK_NOT_FOUND`），才换新幂等键重新生成。

### 4. 伪代码（TypeScript）

```ts
async function generateImage(prompt: string): Promise<string[]> {
  const key = crypto.randomUUID();
  const submit = await fetch(`${BASE}/v1/images/generations/async`, {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${API_KEY}`,
      'Content-Type': 'application/json',
      'Idempotency-Key': key,
    },
    body: JSON.stringify({ model: 'gpt-image-1', prompt }),
  });
  // 超时/5xx 先按第 5 步找回，绝不盲目重提
  if (submit.status !== 202) throw new Error(`submit failed: ${submit.status}`);
  const task = await submit.json();

  let delay = 3000;
  for (;;) {
    await sleep(delay);
    const res = await fetch(`${BASE}${task.poll_url}`, {
      headers: { 'Authorization': `Bearer ${API_KEY}` },
    });
    if (res.status !== 200) throw new Error(`poll failed: ${res.status}`);
    const body = await res.json();
    if (body.status === 'succeeded') {
      // 立刻下载，别拖
      return await Promise.all(
        body.result.data.map((it: any) => downloadToOwnStorage(it.url)),
      );
    }
    if (body.status === 'failed') throw new Error(JSON.stringify(body.error));
    delay = Number(res.headers.get('Retry-After') ?? 3) * 1000;
  }
}
```

### 5. 提交后没拿到响应（超时/502/断线）

**不要重提。** 用同一个幂等键找回：

```http
GET /v1/images/generations/by-request/<同一个 key>
```

返回 `200` 就是原来的那个任务（带 `request_id`），继续轮询它的 `task_id` 即可；
返回 `404` 才说明当时没创建成功，可以安全地换新键重提。

原因：服务端对上游的调用刻意脱离了客户端取消链，客户端超时后任务仍在跑、仍会计费。
盲目重提 = 真的再生成一次 + 再扣一次费。

## HTTP status code contract

This table is the authoritative answer to "did my request create a task?" — the question a client must resolve before deciding to retry.

| Response | Task created? | Meaning |
|---|---|---|
| `202` | **Yes** | Accepted; poll `poll_url`. Also returned when an idempotent replay occurs (check `X-Idempotency-Replayed`). |
| `200` | **Yes** (already done) | Returned by the polling and by-request endpoints, not by submit. |
| `400` | No | Invalid parameters or malformed body. Rejected before any task is created. |
| `401` / `403` | No | Authentication or permission failure. Rejected before any task is created. |
| `404` | No | Feature disabled, unsupported platform, or unknown/foreign task ID. |
| `409` | **Depends** | Idempotency conflict. `IDEMPOTENCY_IN_PROGRESS`: a task **was** created — recover it by `request_id`. `IDEMPOTENCY_KEY_CONFLICT`: the key was reused with a different payload; the original task exists under that key. |
| `413` | No | Body exceeded the size limit. |
| `429` | No | Rate limited before task creation. Retry after the interval in `Retry-After`. |
| `500` / `502` / `503` | **Possibly** | The task may already exist — for example the response was lost after the task was stored, or the upstream call failed after submission. **Do not blind-retry.** Query `/images/generations/by-request/{request_id}` first. |
| Timeout / connection reset | **Possibly** | Same as above. |

### Why `5xx` and timeouts are "possibly"

Upstream generation deliberately runs detached from the client's connection (see `upstreamBase` in `MediaTaskService`): once a submission is accepted, the server finishes the work and records it even if the caller has already gone away. That is what prevents silently billing a generation the caller never learns about — but it also means an ambiguous failure can leave a real task behind.

The only safe client protocol is therefore:

```text
submit with Idempotency-Key
  → 202                : poll task_id
  → 409 IN_PROGRESS    : poll the task from the replayed response / by request_id
  → timeout / 5xx      : GET by-request/{request_id}, then poll; never resubmit blindly
```

Because a `409 IN_PROGRESS` carries the `Retry-After` for the lock window and a replay returns the original `202` body, a client that always reuses the same key converges on exactly one task no matter how many times it retries.

## Poll a task

Use the same API key that submitted the task:

```bash
curl https://api.example.com/v1/images/tasks/imgtask_0123456789abcdef \
  -H 'Authorization: Bearer sk-...'
```

While work is in progress:

```json
{
  "id": "imgtask_0123456789abcdef",
  "task_id": "imgtask_0123456789abcdef",
  "object": "image.generation.task",
  "status": "processing",
  "created_at": 1784092800,
  "expires_at": 1784179200
}
```

On success, `result` mirrors the synchronous image API body. The difference between the two enabled states is what `data[].url` points at:

- **完全转存（配了对象存储）**：每张图都已上传到对象存储，`data[].url` 指向存好的对象
  （长期直链或 presigned），`b64_json` 已剔除——所以不管上游返回的是 url 还是 base64，
  最终统一变成紧凑的托管链接。
- **URL 透传（没配对象存储）**：`data[].url` 原样是**上游厂商的链接**，不做任何搬运。
  链接时效性由上游决定，客户端应立刻取走（见上一段 Client playbook）。

```json
{
  "id": "imgtask_0123456789abcdef",
  "task_id": "imgtask_0123456789abcdef",
  "object": "image.generation.task",
  "status": "succeeded",
  "legacy_status": "completed",
  "http_status": 200,
  "image_url": "https://...",
  "result": {
    "created": 1784092923,
    "data": [{"url": "https://..."}]
  },
  "created_at": 1784092800,
  "completed_at": 1784092923,
  "expires_at": 1784179323
}
```

### Status values

`status` uses the client-facing vocabulary; `legacy_status` carries the original internal value so that callers already branching on `completed` keep working.

| `status` | `legacy_status` | Meaning |
|---|---|---|
| `processing` | `processing` | Accepted and running. Keep polling. |
| `succeeded` | `completed` | Finished; read `result` / `image_url`. |
| `failed` | `failed` | Finished unsuccessfully; read `error`. |

`accepted`, `queued`, and `cancelled` are **not produced**: a submission starts executing immediately (there is no queue stage) and cancellation is not currently exposed. Clients should treat these as unreachable rather than waiting for them.

For URL responses, `image_url` mirrors the first `data[].url` for simple clients. On failure, the task reaches `failed` and exposes the original OpenAI-compatible error object where available:

```json
{
  "id": "imgtask_0123456789abcdef",
  "task_id": "imgtask_0123456789abcdef",
  "object": "image.generation.task",
  "status": "failed",
  "legacy_status": "failed",
  "http_status": 502,
  "error": {
    "type": "api_error",
    "message": "Upstream request failed"
  },
  "created_at": 1784092800,
  "completed_at": 1784092923,
  "expires_at": 1784179323
}
```

All submit and poll responses include `Cache-Control: no-store`, preventing a CDN from caching the `processing` state. Tasks and results expire 24 hours after their latest state update. A task executes for at most 30 minutes.

Task ownership is scoped to both user and API key. Unknown task IDs and IDs owned by another key both return `404`, avoiding task-existence disclosure. Polling remains available when the completed generation used the key's remaining balance; normal authentication, disabled-key, user, IP, and group checks still apply.
