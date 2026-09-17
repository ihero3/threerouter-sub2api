# 生图/生视频接口收敛 · 实现审计报告

> 审计范围：工作区全部未提交改动（基于 `c1841244f`），含接口收敛、模态规则、
> 管理端菜单清理、wire 注入修复等 30+ 文件
> 审计方式：逐文件静态走查 + 关键行为实测（重复 json tag 行为、未来版本号判定、
> 既有失败基线 stash 对照）
> 审计日期：2026-09-17 ｜ **未修改任何代码**

---

## 一、结论速览

| 级别 | 数量 | 一句话 |
|---|---|---|
| P0 安全 | 2 | 请求体无总上限（内存/磁盘 DoS）；产物下载无私网防护（二阶 SSRF） |
| P1 功能 | 5 | 错误消息提取恒空（重复 json tag）；多图存储互相覆盖；1 个测试未跟上行为变更；chat 桥接丢参考图；stream 请求报错 |
| P2 低优 | 6 | 幂等大 payload、wait 语义、空状态防御、4 个既有测试失败、文档双源等 |

**计费与越权专项检查：未发现漏洞。** 鉴权链、权限开关、额度预检、归属过滤、
条件更新防双结算均正确（见第五节）。

---

## 二、P0 · 安全漏洞（建议提交前处理）

### V1. 请求体无总体积上限 —— 内存 / 磁盘 DoS

**位置**：`internal/server/routes/gateway.go` `requestModelFromBody` →
`internal/pkg/httputil/body.go:46`（`io.Copy` 无上限）；
`internal/handler/media_gateway_images.go:239`（`ParseMultipartForm(32<<20)`）

**机理**（三个叠加面）：
1. `imagesHandler` 默认分支（composite/deepseek/kimi/zhipu/anthropic… 所有非
   openai/grok 组）先调 `requestModelFromBody`，它用 `ReadRequestBodyWithPrealloc`
   **无上限**把整个 body 读进内存。10MB 的 `MaxBytesReader`（`readMediaRequestBody`）
   在它**之后**才生效。认证用户发一个 1GB JSON → 1GB 堆内存。
2. `/v1/images/edits` multipart 路径：`readMediaRequestBody` 的 10MB 上限只包 JSON
   路径，multipart 完全绕过。`ParseMultipartForm(32MB)` 超出内存配额的部分落
   **磁盘临时文件**，总大小与文件数均无上限 → 磁盘打爆。
3. 对象存储开启时：单文件有 20MB 上限，但 `image[]` **文件数量无上限**，每个都
   `StoreMediaBytes` 入存储 → 存储容量 / 费用攻击。内联 6MB 预算只约束"无存储"
   场景，开了存储就不管总量。

**说明**：预读模式本身是既有代码，但这次改动让默认分支**所有** /v1/images/* 请求
都走预读，且 multipart/edits 是全新入口——暴露面是本次扩大的。

**修复方案**（不动架构，纯增量）：
- 路由预读前统一包一层 `http.MaxBytesReader(c.Writer, c.Request.Body, 32<<20)`
  （与单文件 20MB 上限匹配，multipart 多文件时再放大到如 64MB）；
- `openAIImageBodyFromMultipart` 开头按 `Content-Length` 预检；
- 校验文件数：`len(form.File["image[]"])+len(form.File["image"]) <= 4`（对齐
  千问 6 / MiniMax 9 的 n 上限取保守值）。

### V2. 产物下载无私网地址防护 —— 二阶 SSRF

**位置**：`internal/service/media_task_service.go` `downloadMediaBytes`；
调用方 `maybeStoreMedia`、`media_gateway_images.go` `imageBase64Payload`

**机理**：下载 URL 来自上游响应（`MediaURL`/`MediaURLs`）。账号由管理员配置，
属半可信；但若某个上游账号被劫持、或上游被诱导返回
`http://169.254.169.254/latest/meta-data/` 或 `http://10.0.0.5:8080/admin` 这类
地址，网关会**替用户去拉内网资源**，并且：
- `?response_format=b64_json` 场景下，拉到的内容直接 base64 **回传给用户**；
- `maybeStoreMedia` 场景下拉到的内容被转存并可被用户下载。

这是典型的二阶 SSRF（需要恶意/被劫持的上游才能触发，故评 P0 末位而非最严重）。

**修复方案**：
- `downloadMediaBytes` 内解析目标 IP（含跟随的每一跳 redirect），拒绝
  回环 / 私网 / 链路本地 / 保留段（127/8、10/8、172.16/12、192.168/16、169.254/16、::1、fc00::/7 等）；
- 或轻量版：仅允许 http/https 且 host 为账号 `base_url` 域名白名单的 URL。

---

## 三、P1 · 功能 bug

### B1. 错误消息提取恒为空 —— 重复 json tag（既有 bug，实测确认）

**位置**：`internal/server/routes/unified_dispatch.go:211-220`
`chatCompatWriter.ErrorMessage`

```go
var payload struct {
    Error struct {
        Message string `json:"message"`
    } `json:"error"`
    ErrorString string `json:"error"`   // ← 与上一字段重复 tag "error"
}
```

**实测证据**（独立 Go 程序验证）：两个字段同 tag `error` 时，`json.Unmarshal`
**两个字段都不会被填充**——无论 `{"error":{"message":"boom"}}` 还是
`{"error":"plain"}`，解码后两个字段都是空串且不报错。

**后果**：`runMediaAsChat` 的错误路径（`unified_dispatch.go:173-178`）拿到的
message 恒为空，兜底成 `http.StatusText(status)`。用户通过 chat 端点调图片模型
失败时，看到的错误是 "Bad Gateway" 而不是真实原因（比如"免费额度已用尽"）。
**排障时这 exactly 是你最需要的那条信息。**

**修复方案**：分两次解析（先试 object 再试 string），或给 `ErrorString` 换
`json:"-"` 手动处理。同时 `go vet` 已在报这条（`unified_dispatch.go:219`），
修复后 vet 恢复干净。

### B2. 多图对象存储互相覆盖 —— n>1 时全部变成最后一张（既有 bug，影响放大）

**位置**：`internal/service/media_task_service.go:265-281`（多 URL 落库循环）+
`mediaStorageKey`（:615）

**机理**：`mediaStorageKey` 只由 `LocalID(+UpstreamTaskID)` 的 sha1 + 扩展名决定。
同一任务 n>1 张图在循环里逐个 `maybeStoreMedia` → **同一个存储 key** → 后写覆盖
先写 → 所有 stored URL 指向同一个对象（最后一张图）。

**影响**：本次收敛后 `/v1/images/generations` 返回 `data[]` 数组，`n>1` + 对象
存储开启时**必现**——用户花 n 张图的钱，拿到 n 个指向同一张图的 URL。

**修复方案**：key 追加序号或 URL hash：
`mediaStorageKey(record, ct) + "-" + strconv.Itoa(i)`，或 `sha1(LocalID+rawURL)`。
注意保留"同一 URL 重复轮询不重复存储"的幂等语义（用 URL hash 天然满足）。

### B3. `TestGatewayRoutesNonGrokVideosAreRejectedAtPlatformGate` 未跟上行为变更

**位置**：`internal/server/routes/gateway_test.go:378`

断言"非 grok 平台 POST /v1/videos/generations 回 404"——这正是本次**刻意移除**
的平台门（现在统一走媒体链路，实际回 401 是因为测试没注入 API key 上下文）。
**提交前必须更新此测试**（改为断言进入媒体链路），否则 CI 必红。

### B4. chat→媒体桥接丢弃参考图（image_url parts）

**位置**：`internal/server/routes/unified_dispatch.go` `lastUserMessageText` /
`mediaMessageContentText`

只提取 `type:"text"` 的 parts，`type:"image_url"` 被静默丢弃。用户在 chat 端点发
图生图请求（content 里带图片）时，参考图丢失，**静默降级成纯文生图**——用户拿
到的图和预期完全不同，且没有任何报错。

**修复方案**：`rewriteChatRequestForMedia` 里把最后一条 user 消息中的
`image_url.url` 收集进 `image` 字段（单个为 string，多个为数组，与媒体链路
`image` 字段契约一致）。

### B5. `stream: true` 的 chat 请求进媒体链路返回普通 JSON

`runMediaAsChat` 恒返回非流式 `chat.completion`。SDK 用 `stream=True` 调图片模型
时会按 SSE 解析 JSON → 直接报错。建议：检测 `payload["stream"]==true` 时返回
400 带明确提示（"media models do not support streaming"），比静默给错格式好。

---

## 四、P2 · 低优先 / 防御性

| # | 问题 | 位置 | 建议 |
|---|---|---|---|
| P1 | 幂等 payload 可能含 ~6MB data URI 存进 Redis | `mediaCreateIdempotent` `Payload: body` | payload 只存 hash/摘要，重放时按 localID 回查 |
| P2 | `wait=0` 时生图端点回 504 timeout_error（语义上并未超时） | `media_gateway_images.go:119-124` | 可接受；或改 202+任务结构（会破坏 OpenAI 语义，倾向保持） |
| P3 | 默认 120s 等待可能超过反代超时（nginx 常见 60s proxy_read_timeout） | 部署层 | `deploy/` 文档标注调大 proxy_read_timeout，或默认降到 60s |
| P4 | `createResult.Status` 为空串时任务卡非终态（`UpdateStatusIfProcessing` 只认 `processing`） | `media_task_service.go:247` | 落库前 `if status=="" {status="processing"}`；现适配器都设值，纯防御 |
| P5 | routes 包 4 个既有失败（auth panic、codex manifest、prompt-audit coverage） | 该包此前**从未编译过**（`gateway_key_billing_test.go` 构造器签名不匹配），修好后才暴露 | 单独排期修；`prompt_audit_route_coverage` 列的 7 条路由都是老路由，与本次无关 |
| P6 | `docs/IMAGE_API_GUIDE.md` 与根目录同名文件内容近乎重复 | 两份 5.8KB | 删 docs/ 那份（root 的是已跟踪正本），避免双源漂移 |

---

## 五、专项检查结论（未发现问题的部分）

这部分是用户最关心的"实现上有没有漏洞"，逐项核过：

| 检查项 | 结论 | 证据 |
|---|---|---|
| **鉴权完整性** | ✅ | `CreateImages` 与 `Create` 共用 `createMediaTask`：API key → subject → 图片权限开关 → 额度预检，一条不少；multipart 归一化发生在鉴权之前但只做格式转换，不产生副作用 |
| **越权（IDOR）** | ✅ | `Get`/`GetContent` 用 `GetTask(ctx, id, userID)` 带归属过滤；`HasLocalTask` 显式比对 `record.UserID == userID`；别人的任务查 media 表不中 → 回退 video_tasks 也带 userID → 404，无信息泄露 |
| **计费双扣** | ✅（本轮修对的） | `UpdateStatusIfProcessing` 条件更新（`WHERE status='processing'`）+ `claimed` 判定：并发轮询 / 轮询 vs 取消 / 超时 vs 轮询，只有赢家结算一次 |
| **失败留痕** | ✅ | 同步失败（三种 kind）、选号失败、异步失败均写 0 费用 usage_logs；选号失败带分组平台 warn 日志 |
| **b64_json 契约** | ✅ | 真 base64（下载转码 / data URI 剥前缀），20MB 上限 + Content-Length 预拦 + 多读 1 字节防截断；转码失败显式 502 不静默换 url |
| **内联预算** | ✅ | 按编码后体积扣（4/3 膨胀已计入）；上传超限多读 1 字节识别，不静默截断 |
| **存储 key 注入** | ✅ | `mediaStorageKey` = sha1 hex + 白名单扩展名，contentType 只影响扩展名，无路径穿越 |
| **模态分派** | ✅ | 40+ 文本模型实测全判 text；wanx 已归位图片；两分派函数对已收录模型一致性有测试钉死 |
| **wire 注入** | ✅ | `MediaTask` 赋值遗漏已修（此前 `/admin/media-tasks` 三个接口 nil panic，菜单是坏的） |
| **超时资源** | ✅ | `?wait` 上限 180s + ticker（不堆积 timer）+ ctx 取消即返回；任务结算与客户端断开解耦（persistCtx） |

---

## 六、修复顺序建议

1. **提交前必做**：B3（更新被行为变更作废的测试）——否则 CI 红。
2. **尽快**：V1（体积上限三件套，纯增量 ~30 行）、B1（重复 tag，~10 行，顺手消
   vet 告警）、B2（存储 key 加序号，~5 行）。这三个都是小改动大收益。
3. **次优**：V2（私网 IP 拒绝列表，~40 行）、B4（image_url 桥接，~30 行）。
4. **排期**：B5、P1-P6。

---

*本报告仅审计未修改；所有"实测"结论均有独立验证（重复 tag 行为用独立 Go 程序
复现；既有测试失败用 `git stash` 基线对照确认）。*
