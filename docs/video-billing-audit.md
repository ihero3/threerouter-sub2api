# 视频生成计费逻辑审计报告

> 审计范围：`backend/internal/service` 下视频生成相关的计价、预扣、结算、落库全链路
> 审计方式：静态代码走查（未执行、未修改任何代码）
> 审计日期：2026-09-12

---

## 一、结论速览

| 问题 | 答案 |
|------|------|
| 视频按什么计费？ | **按秒计费**：`每秒单价 × 时长 × 视频数`，**不统计 token** |
| 走哪条通道？ | 同步链路走统一 `applyUsageBilling` 原子扣费，与普通模型**同表 `usage_logs`**（`billing_mode = "video"`） |
| 和普通模型一样吗？ | **扣费/落库/幂等一致**，但**计价口径不同**（时长+分辨率，而非 token），**价格来源优先级也不同** |
| 能走渠道定价吗？ | **只能部分走**。同步网关链路支持；**异步视频任务链路（Seedance / Wan / MiniMax / Hailuo）完全不查渠道定价** |
| 有没有问题？ | 有。**2 个 P0 + 2 个 P1 + 若干 P2**，其中 1 个是会导致静默错价的确定性 bug |

---

## 二、计费口径：按秒，不是按 token

`BillingService.CalculateVideoCost`（`billing_service.go:1962`）：

```go
perSecondPrice := s.getVideoUnitPrice(model, resolution, groupConfig)
totalCost := perSecondPrice * float64(durationSeconds) * float64(videoCount)
actualCost := totalCost * rateMultiplier
```

计费三要素：

1. **分辨率** — 归一化到 `480p / 720p / 1080p`（`video_billing_resolution.go:37-48`）
2. **时长（秒）** — `<=0` 按默认 **8 秒**；下限 1 秒；**上限硬编码 15 秒**（`video_billing_resolution.go:13-32`）
3. **费率倍率** — `resolveVideoRateMultiplier`：分组开启 `VideoRateIndependent` 时用 `VideoRateMultiplier`，否则用用户/分组通用倍率（`image_billing_multiplier.go:13-21`）

模型 → 价格族的归一化（`video_billing.go:24-46`）：Grok Imagine Video / 1.5、Seedance、MiniMax、Wan 各自一个族，用于查 `groups.video_model_prices`。

---

## 三、两条计费链路

### 链路 A：同步网关视频（Grok Imagine，含 `/v1/videos` 与 Grok 媒体视频）

入口 `calculateOpenAIVideoCost`（`openai_gateway_usage.go:781-845`），**五级优先**：

| 顺序 | 条件 | 计价方式 | 代码位置 |
|------|------|---------|---------|
| 1 | `resolved.Source == group && Mode == video`（分组 ModelPricing 卡） | `CalculateCostUnified`，`UsageUnits = 数量 × 时长` | `:795-805` |
| 2 | 分组已配视频价（`video_price_*` 或 `video_model_prices`） | `CalculateVideoCost`（每秒价） | `:806-809` |
| 3 | 分组快照疑似不完整 → 回源 DB 刷新后重试步骤 2 | 同上 | `:810-816` |
| 4 | `resolved.Source == channel` 且 Mode ∈ {`per_request`, `image`, `video`} | `CalculateCostUnified`；**Mode=video 时 units=数量×时长，per_request/image 时 units=数量（不乘时长）** | `:817-842` |
| 5 | 兜底 | `CalculateVideoCost`（默认价） | `:844` |

渠道价命中后走 `calculatePerRequestCost`（`billing_service.go:1508-1541`）：`SizeTier = 分辨率` → `GetRequestTierPrice` 按 `tier_label` 精确匹配 → 未命中回退 `DefaultPerRequestPrice` → `单价 × units × 倍率`。

**即渠道 video 模式下，`per_request_price` / interval 的价格语义是「每秒单价」，会再乘以时长。**

### 链路 B：异步视频任务（`/v1/video` 与 `/v1/media kind=video`）

统一结算在 `video_task_billing.go`。

**创建时**（`video_task_service.go:214-219`）：

```go
cost, _ := estimateVideoTaskCost(ctx, ..., publicModel, req.Resolution, req.DurationSec)
record.ReservedCost = &cost
_ = s.apiKeyService.UpdateQuotaUsed(ctx, apiKeyID, cost)   // 仅预扣 Key 配额
```

**成功终态**（`settleVideoTaskSuccess`，`:114-194`）：

1. `CalculateVideoCost` 按实际时长算费
2. `applyUsageBilling` 原子扣费（余额 / 订阅 / Key 配额 / 账号配额 + 缓存同步 + 低余额通知）
3. 写 `usage_logs`（`billing_mode=video`，`video_count` / `video_resolution` / `video_duration_seconds` 齐备）
4. 退还创建时的预扣（净效果 = 实际费用）
5. 幂等键固定为 `video-task:<local_id>`，轮询重试不重复扣费

**失败/取消**（`:198-208`）：全额退预扣 + 写 0 费用 `usage_logs`（审计可见）。

**关键**：`videoTaskCostBreakdown`（`:83-91`）直接调用 `CalculateVideoCost`，**全程没有 `ModelPricingResolver` 参与**。

---

## 四、与普通模型的对比

| 维度 | 普通模型 | 视频生成 |
|------|---------|---------|
| 计费单位 | input/output/cache token | 秒 × 分辨率档 |
| 价格解析 | `ModelPricingResolver`：Group → Channel → LiteLLM → Fallback | 链路 A 走 resolver；**链路 B 完全不走** |
| 扣费通道 | `applyUsageBilling` | 同（链路 B 已对齐，链路 A 本就同） |
| usage_logs | 同表 | 同表，`billing_mode=video` |
| 幂等 | request_id | `video-task:<local_id>` |
| 预扣 | 无（后扣） | 链路 B 有（**仅 Key 配额**），完成时多退少补 |
| 独立倍率 | 分组通用倍率 | 支持 `VideoRateIndependent` 视频独立倍率 |
| 时段定价 | 支持（token 模式） | **不支持**（`channel_service.go:683-703` 限制 TimePricing 仅 token 模式） |

---

## 五、问题清单

### P0-1 异步视频任务完全不走渠道定价（能力缺口）

**证据**：`video_task_billing.go:83-91`

```go
func videoTaskCostBreakdown(...) (*CostBreakdown, float64) {
    ...
    return deps.billingService.CalculateVideoCost(model, resolution, 1, durationSec, groupConfig, videoMultiplier), videoMultiplier
}
```

全链路无 `resolver.Resolve` / `CalculateCostUnified`。管理员在渠道上配的 `BillingModeVideo`、分辨率阶梯价、`per_request_price`，对 `/v1/video` 与 `/v1/media kind=video` **完全无效**——这正是「视频能否像普通模型一样走渠道定价」的答案：**目前不能（对异步链路）**。

**影响**：Seedance / Wan / MiniMax / Hailuo 等主力异步视频模型，只能靠分组 `video_price_*` / `video_model_prices` 定价，渠道维度的成本控制、阶梯、多号差异化全部失效。

---

### P0-2 `768p` / `2k` 档位在运行时永远取不到（确定性 bug，静默错价）

**证据链**：

1. `video_billing.go:54-67` `LookupVideoBillingResolutionAny` **明确支持** `768p` 与 `2k`（注释写明「MiniMax H3 用 768P / 2K…管理员可以配」）
2. `billing_service.go:1966` `CalculateVideoCost` **第一步**就把分辨率折叠成三档：
   ```go
   resolution = NormalizeVideoBillingResolutionOrDefault(resolution)   // 未知 → 480p
   ```
3. `billing_service.go:1969` 再进 `getVideoUnitPrice` → `LookupVideoModelPrice`（`video_billing.go:177`）：
   ```go
   tier := NormalizeVideoBillingResolutionAnyOrDefault(resolution)
   ```
   此时入参**已经是** `480p/720p/1080p` 之一，永远不可能解析出 `768p` / `2k`

**结果**：在 `video_model_prices` 里为 MiniMax H3 配的 `768p` / `2k` 单价**永远命中不了**，实际按 `480p` 档甚至兜底价出账——**配置界面能存、不报错、静默错价**。

同样的折叠也影响渠道侧：`SizeTier` 传入的是三档归一结果，`GetRequestTierPrice`（`model_pricing_resolver.go:383-390`）按 `tier_label` 精确匹配，渠道上同样配不了 `768p` / `2k`。

---

### P1-1 时长上限硬编码 15 秒，不按模型区分

`video_billing_resolution.go:13-17`：

```go
VideoBillingMinDurationSeconds     = 1
VideoBillingMaxDurationSeconds     = 15
VideoBillingDefaultDurationSeconds = 8
```

15 秒来自 xAI 的 1-15s 规格，但这是**全局常量**，对 Seedance / Wan / Veo / 可灵等上限不同的模型同样生效。上游产出超过 15 秒的真实视频会被收敛到 15 秒**少收**；而 6 秒默认模型在上游未回传时长时按 8 秒**多收**。文件注释自己也写明「计费时长必须与上游实际消耗对齐，否则用户可通过拉长 duration 套利」。

> **已修复（见 7.6）**：业务上限 15 秒已移除，改为只保留脏数据防御阈值。

---

### P1-2 兜底单价 = 图片 2K 价，语义错配且无告警

`billing_service.go:2065-2075`：

```go
func (s *BillingService) getDefaultVideoPrice(model string, resolution string) float64 {
    if price, ok := getDefaultGrokImagineVideoPrice(model, resolution); ok { return price }
    // LiteLLM schema 没有 video 价格字段，回退到历史默认值（按每秒解释）
    return s.getDefaultImagePrice(model, ImageBillingSize2K)
}
```

非 Grok 模型在无分组价、无渠道价时，把「**每张图片价**」（默认 `defaultImageGenerationPrice = $0.134`，源自 gemini-3-pro-image）当成「**每秒视频价**」使用 → 8 秒视频 = **$1.072**，且**没有任何 warn 日志**。任何忘记配价的新视频模型都会静默按这个数出账。

---

### P1-3 创建时只预扣 Key 配额，不冻结余额（漏收风险）

`video_task_service.go:214-219` 只做 `UpdateQuotaUsed`（Key 配额维度），余额/订阅在任务完成时才扣。

风险路径：余额接近 0 的用户并发提交 N 个长视频 → 完成时 `applyUsageBilling` 失败 → `video_task_billing.go:177-188` 分支：

```go
// 扣费失败：保留预扣（Key 配额维度近似入账，供对账），日志按 0 费用落库
usageLog.TotalCost = 0
usageLog.ActualCost = 0
```

**余额侧一分钱没收，只扣了 Key 配额，且落一条 0 费用日志。** 这是可被利用的漏收口。

---

### P2 级问题

| # | 问题 | 位置 |
|---|------|------|
| P2-1 | 渠道 `per_request` / `image` 模式用于视频时 `units=数量`（不乘时长），1 秒与 15 秒同价 → 时长套利；代码注释称「有意为之」，但对视频是明显口子 | `openai_gateway_usage.go:817-824` |
| P2-2 | 分组视频价优先于渠道价（与 resolver 的 Group→Channel 顺序一致，但运营在渠道页改价会发现"不生效"） | `openai_gateway_usage.go:806-816` |
| P2-3 | 异步链路 `VideoCount` 硬编码 1，未来支持 n>1 会少计费 | `video_task_billing.go:237` |
| P2-4 | 渠道 video 模式必须配 `PerRequestPrice` 或 `Intervals`，但该字段界面标签是「按次价格」，对视频实为「每秒单价」，易配错 | `channel_service.go:736-746` |
| P2-5 | 视频不支持时段定价（TimePricing 仅 token 模式） | `channel_service.go:683-703` |

---

## 六、改造方案

### 方案 A：补齐链路 B 的渠道定价（最小改动，建议先做）

**目标**：让异步视频任务也能像普通模型一样走渠道定价。

1. `videoTaskBillingDeps` 增加 `resolver *ModelPricingResolver`（当前已持有 `openAIGatewayService`，其 `resolver` 字段在同包可直接复用，**无需改构造器装配**）
2. `videoTaskCostBreakdown` 增加渠道分支：
   ```go
   if resolved := resolver.Resolve(ctx, PricingInput{Model: model, GroupID: apiKey.GroupID, Group: apiKey.Group});
      resolved != nil && (resolved.Source == PricingSourceChannel || resolved.Source == PricingSourceGroup) &&
      resolved.Mode == BillingModeVideo {
       if cost, err := billingService.CalculateCostUnified(CostInput{
           Model: model, GroupID: apiKey.GroupID, Group: apiKey.Group,
           UsageUnits: float64(durationSec), SizeTier: resolution,
           RateMultiplier: videoMultiplier, Resolver: resolver, Resolved: resolved,
       }); err == nil { return cost, videoMultiplier }
   }
   // 回退现有 CalculateVideoCost
   ```
3. 创建时的 `estimateVideoTaskCost` 必须走**同一函数**，否则预扣与实扣口径不一致

**风险**：低。回退路径完整保留，且 `CalculateCostUnified` 已在链路 A 验证过。

---

### 方案 B：统一视频计价入口（彻底治理，建议随后做）

抽 `resolveVideoPricing(ctx, model, resolution, durationSec, apiKey) (*CostBreakdown, string)`，链路 A / B 共用，内部统一：

1. **分辨率用 `NormalizeVideoBillingResolutionAnyOrDefault`**，`CalculateVideoCost` 不再提前折叠 → 修 P0-2，支持 768p / 2k
2. **时长上限改为按价格族/厂商配置**（如 `xai: 1-15`、`minimax: 1-10`、`seedance: 1-12`），缺省沿用 15 → 修 P1-1
3. **无价时显式告警**：新增 `video_pricing_missing` warn 日志，且默认价从"图片 2K 价"改为 0 + 告警，或直接拒绝服务 → 修 P1-2
4. **渠道 video 模式拒绝 `per_request` / `image` 混入**，或强制按 video 语义乘时长 → 修 P2-1

---

### 方案 C：预扣风控（补 P1-3）

1. 创建时对余额/订阅做同等预留（或至少做余额充足性硬校验后拒绝）
2. 扣费失败时**不要静默落 0 费用日志**：写入待补扣队列 / 标记 `billing_failed`，管理端可查
3. 对同一 Key 的并发视频任务数做上限

---

### 优先级建议

| 顺序 | 内容 | 理由 |
|------|------|------|
| 1 | P0-2（分辨率折叠 bug） | 确定性错价，改 1 行 + 1 处调用，零风险 |
| 2 | 方案 A（链路 B 渠道定价） | 回答"能否走渠道定价"，能力缺口，改动可控 |
| 3 | P1-2（默认价告警） | 防止新模型静默按错误价出账 |
| 4 | 方案 C（预扣风控） | 堵漏收口 |
| 5 | 方案 B 其余（时长按模型、per_request 语义） | 结构性重构，可排后续迭代 |

---

## 七、实施记录（2026-09-12）

已按确认方案落地，只改异步链路（`/v1/video` + `/v1/media kind=video`），同步 Grok 链路行为不变。

### 7.1 分辨率解析（修 P0-2 + 支持 WxH）

`service/video_billing_resolution.go`：

- 新增档位常量 `768p` / `2k` / `4k`
- 新增 `ParseVideoBillingResolution`：解析长宽尺寸（`1920x1080`、`1920*1080`、`1920×1080`、`1920:1080`，允许空格）与纯数字，**按短边映射**（`1080x1920` 竖屏同样得 1080p）
- 新增 `VideoBillingResolutionFromPixels`：短边 → 档位（≤480→480p，≤720→720p，≤768→768p，≤1080→1080p，≤1440→2k，其余→4k）
- 新增 `VideoBillingResolutionFallbacks`：降档序列 `4k→2k→1080p→768p→720p→480p`
- `LookupVideoBillingResolutionAny` 现在会兜底调用尺寸解析，所有历史调用点自动受益

`service/billing_service.go`：

- **P0-2 修复**：`CalculateVideoCost` 不再用 `NormalizeVideoBillingResolutionOrDefault` 提前折叠成三档，改为 `NormalizeVideoBillingResolutionAnyOrDefault` → `768p` / `2k` 变为可达
- `getVideoUnitPrice` 沿降档链逐级查价（per-model 表 → 分组 flat 三档），未命中时打 `video_pricing_tier_fallback` 告警
- `VideoPriceConfig` 新增 `flatPrice(tier)` 方法支持降档查询

### 7.2 时长优先级

- 新增 `NormalizeVideoBillingDurationSeconds(actual, requested)`：**上游回传真实时长 > 用户请求时长 > 默认 8 秒**
- `videoTaskBillingInput` 新增 `RequestedDurationSec`
- 创建时预扣用 `(0, req.DurationSec)`；完成时估算用 `(result.DurationSec, record.DurationSec)`；结算用 `(in.DurationSec, in.RequestedDurationSec)`
- 影响面：`video_task_service.go`、`media_task_service.go`（含音频分支同步改为同一优先级）

### 7.3 异步链路接入渠道定价（P0-1）

`service/video_task_billing.go` 的 `videoTaskCostBreakdown` 价格来源顺序，与同步网关链路 `calculateOpenAIVideoCost` 对齐：

| 顺序 | 来源 | 计价 |
|------|------|------|
| 1 | 分组定价卡（`groups.model_pricing`，Mode=video） | 每秒单价 × 时长，SizeTier = 分辨率 |
| 2 | 分组媒体视频价（`video_price_*` / `video_model_prices`） | `CalculateVideoCost` |
| 3 | 渠道定价（Mode ∈ per_request / image / video） | video 模式乘时长，per_request / image 按次 |
| 4 | 兜底 | `CalculateVideoCost` 默认价 |

新增辅助：`resolveVideoTaskPricing`、`calculateVideoTaskUnifiedCost`、`resolveVideoTaskBillingTier`（渠道路径同样支持降档 + 告警）。

**无需改构造器装配**：`videoTaskBillingDeps` 已持有 `OpenAIGatewayService`，同包可直接取 `resolver`。

创建时的 `estimateVideoTaskCost` 与结算共走 `videoTaskCostBreakdown`，预扣与实扣口径一致。

### 7.4 测试

`service/video_billing_test.go` 新增 7 个用例（WxH 解析、档位名解析、降档链、时长优先级、768p/2k 可达、长宽尺寸计费、渠道定价生效、无解析器回退）。

`TestNormalizeVideoModelPricesDropsUnknownResolutions` 中的 `"4k"` 改为 `"8k"`：4k 已从「非法档位」提升为「合法专有档位」，这是有意的语义变更。

### 7.5 未处理（需单独决策）

| 项 | 说明 |
|----|------|
| P1-2 默认价兜底 | 非 Grok **视频**模型无价时仍回退到「图片 2K 价」（$0.134/秒）且无告警 |
| P1-3 预扣风控 | 创建时仍只预扣 Key 配额，不冻结余额 |
| P0-3（新发现） | 见下 |
| 图片：无 per-model 价格表 | 没有 `image_model_prices`，只有分组三档 flat 价（1K/2K/4K）。视频有 `video_model_prices` |
| 图片：异步链路不走渠道定价 | `media_task_service.go` 的 `MediaKindImage` 分支直接调 `CalculateImageCost`，与视频 P0-1 原状一致 |

### 7.6 移除计费时长 15 秒业务上限（P1-1）

各厂商支持的输出时长差异极大（xAI 1-15s，Seedance / 可灵 / Wan / Sora 等支持 30 秒或更长），
统一钳到 15 秒会对长视频**少收一半**。因此业务上限被移除，改为只保留脏数据防御：

```go
const (
	VideoBillingMinDurationSeconds      = 1
	VideoBillingDefaultDurationSeconds  = 8
	// 只拦截上游脏数据（把毫秒当秒返回 / 999999 这类异常值），不是业务上限
	VideoBillingSanityMaxDurationSeconds = 3600
)
```

规则变化：

| 场景 | 旧行为 | 新行为 |
|------|--------|--------|
| 上游回传 30 秒 | 按 15 秒计费（少收） | 按 30 秒计费 |
| 用户请求 60 秒、上游不回传 | 按 15 秒计费 | 按 60 秒计费 |
| 时长 <=0 / 缺失 | 默认 8 秒 | 默认 8 秒（不变） |
| 上游返回 999999（脏数据） | 钳到 15 秒 | 视为无效，回退到用户请求时长 → 默认 8 秒 |

优先级不变：**上游回传真实时长 > 用户请求时长 > 默认 8 秒**，三个来源都只做有效性校验
（`sanitizeVideoBillingDuration`），不再做业务钳制。

改动文件：`video_billing_resolution.go`（常量 + 归一化函数）、`openai_gateway_service.go`（注释）。
测试：`video_billing_test.go` 新增 `TestNormalizeVideoBillingDurationSecondsHasNoBusinessCap`；
`billing_service_test.go` 的 `TestCalculateVideoCostBillsPerSecond` 断言从「999 → 15 秒」改为
「30 秒按 30 秒、999999 回退默认」。

**残留风险**：若用户请求时长超过模型实际能力（例如对 Grok 请求 30 秒，上游只产出 15 秒）
且上游**不回传真实时长**，结算会按用户请求值计费 → 多收。

**不选「按模型能力限制时长」的原因**：限制时长会把「请求 30 秒、实际产出 30 秒」的正常长视频一并砍到
15 秒少收，代价远大于上述偶发多收；而按真实时长计费在绝大多数场景（上游会回传 duration）是准确的。
真正该做的是提高「上游真实时长」的捕获率，见 7.7。后续若仍要兜底，可按模型维度配
`max_duration_seconds`（Grok=15、Seedance=30），当前版本未实现。

### 7.7 时长来源：API 参数，不是 prompt

计费时长有且只有三个来源，按优先级：

1. **上游回传的真实时长**（最可信）——来自 adapter 解析的 `result.DurationSec` / `status.video.duration`
2. **客户端声明的时长**——来自 API 请求参数 `duration` / `duration_sec` / `duration_seconds` / `seconds`
3. **默认 8 秒**——两者都缺失时

**时长绝不能从 prompt 解析**：prompt 里写「生成 10 秒视频」只是文本描述，不代表实际产出。

加固内容（`parseVideoDurationSecondsParam`，`video_task_service.go`）：

| 场景 | 旧行为 | 新行为 |
|------|--------|--------|
| `duration: 10` / `duration_sec: 10` | OK | OK（不变） |
| `duration_seconds: 30`（OpenAI / Sora 风格） | **取不到 → 预扣按默认 8 秒** | 取到 30 |
| `seconds: 20` | **取不到** | 取到 20 |
| `duration: "10s"` / `"10 秒"`（字符串） | **取不到** | 取到 10 |
| `duration: "auto"` / `-1`（由模型自定） | 取不到 | 视为「自动」，走来源 1 → 3 |
| `duration` 缺失 | 默认 8 秒 | 默认 8 秒（不变） |

`duration` 为 `auto` / `-1` / 非正数时**不参与计费**，直接交给上游真实时长兜底，
避免「用户声明 0 秒就免费」和「模型自动时长被算成默认 8 秒」两种错价。

同步加固上游真实时长提取（`video_adapters_vendors.go`）：

- `intAtPath` 原先只认整数字符串，`"10.0"` 这类浮点 duration 会解析失败退化成 0 → 现在支持浮点
- 三个 vendor adapter 的查询路径扩容：`content.duration` / `video.duration` / `data.video.duration` /
  `task.video.duration` / `output.video.duration` / `usage.video_duration` 等

测试：`video_task_service_test.go` 新增 `TestParseVideoDurationSecondsParam`（12 个用例）、
`TestIntAtPathAcceptsFloatStrings`。

### 7.8 P0-4：同步视频链路的分辨率折叠（已修）

`calculateOpenAIVideoCost`（`openai_gateway_usage.go`）入口此前用
`NormalizeVideoBillingResolutionOrDefault` 把分辨率折叠成三档，导致 `video_model_prices`
与渠道定价里配的 `768p` / `2k` / `4k` **在同步 Grok 链路上永远命中不到**。
上一轮 P0-2 只修了 `CalculateVideoCost` 内部与异步链路，漏了这个入口。

改为 `NormalizeVideoBillingResolutionAnyOrDefault`，与异步链路（`video_task_billing.go`）同一口径。

测试：`video_billing_test.go` 新增 `TestCalculateOpenAIVideoCostKeepsVendorResolutionTier`
（分组配 2k = 0.5/秒、请求 2k/10 秒 → 5.0；若被折叠成 480p 会掉到兜底默认价）。

### 7.9 图片计费：降档链与告警（已补）

`getImageUnitPrice` 原先是精确匹配：分组配了 1K 没配 4K 时，请求 4K 直接掉到默认价且**无告警**。

新增 `ImageBillingSizeFallbacks`（`image_billing_size.go`）：**4K → 2K → 1K** 逐级回退，
命中低档时打 `image_pricing_tier_fallback` 告警，与视频口径一致。
分组一张价都没配时仍走默认价（不告警，避免噪音）。

**与视频的语义差异（重要）**：

- 视频兜底价是「图片 2K 价按秒」（$0.134/秒），**语义错配**，所以降档是明确改善
- 图片兜底价是按张的市场价（$0.134 base，2K×1.5、4K×2），**语义本身正确**，
  降档到已配的低价档**可能低于默认价 → 少收**

仍然选择降档的理由：管理员配了低档价说明有明确定价意图，按他的价外推比用硬编码默认价更可控；
且降档会告警，运营可据此补配高档位。**若分组把高档位价格设得远高于默认价，这个改动会让
未配档位的高分辨率请求少收**，需按 `image_pricing_tier_fallback` 告警补配。

测试：`image_billing_size_test.go` 新增 `TestImageBillingSizeFallbacksDescends`、
`TestCalculateImageCost_FallsBackToConfiguredLowerTier`。

## 八、新发现：P0-3 价格族名归一化不一致

`NormalizeVideoModelPrices`（**保存期**归一化，`video_billing.go:109`）只用 `CanonicalGrokImagineVideoPriceFamily`，而 `LookupVideoModelPrice`（**查询期**）用 `CanonicalVideoModelPriceFamily`。

后果：管理员若用**厂商别名**当 key 配价（如 `minimax-hailuo`），保存后 key 仍是 `minimax-hailuo`，但查询期 `minimax-hailuo-02` 会归一化为族名 `minimax-video` → **查不到，价格静默失效**。

以**族名**作 key（`minimax-video` / `seedance` / `wan-video`）时不受影响。

**未修复原因**：改成 `CanonicalVideoModelPriceFamily` 会让**已存库的别名 key 数据**反向失配（老数据不会重新归一化），需要配套数据迁移或查询期双重查找（先查族名再查原始 key）。改动面超出本次范围，需单独决策。

---

## 九、附：关键文件索引

| 文件 | 职责 |
|------|------|
| `service/channel.go:13-18` | `BillingModeVideo` 定义 |
| `service/billing_service.go:1962` | `CalculateVideoCost`（按秒计费核心） |
| `service/billing_service.go:2008-2075` | 视频单价取值优先级 + 兜底默认价 |
| `service/billing_service.go:1508-1541` | `calculatePerRequestCost`（渠道按次/视频共用） |
| `service/video_billing_resolution.go` | 分辨率归一化 + 时长 clamp |
| `service/video_billing.go` | 价格族归一化 + `video_model_prices` 查询 |
| `service/openai_gateway_usage.go:781-845` | 链路 A 五级计价 |
| `service/model_pricing_resolver.go:71-128` | Group → Channel → LiteLLM 解析链 |
| `service/video_task_billing.go` | 链路 B 预扣/结算/落库/幂等 |
| `service/media_quota.go` | 预扣释放与差额结算 |
| `service/channel_service.go:670-746` | 渠道定价校验（video 模式必须配价、TimePricing 限制） |
