# Route Upgrade Plan - 智能 API 路由调度升级方案

## 1. 目标与范围

本文档描述账号管理中配置的上游账号在用户 API 调用时的智能路由升级方案。

目标是在现有账号调度能力基础上，引入可配置的路由策略和实时评分机制，使系统能够根据成本、延迟、成功率、限额余量、地域和当前负载等因素，动态选择更合适的上游账号。

本方案覆盖：

- 用户 API 请求进入网关后的账号选择逻辑。
- Anthropic / Claude 兼容网关账号调度。
- OpenAI 兼容网关账号调度。
- Gemini / Antigravity 等平台后续复用同一评分框架。
- 管理后台中的路由策略配置与账号指标展示。

本方案不改变：

- API Key 鉴权模型。
- 用户套餐、余额和计费逻辑。
- 当前模型路由、粘性会话、并发槽位、等待队列的基础语义。


## 2. 当前系统调度现状

当前系统已经具备较完整的账号调度基础，主要逻辑集中在：

| 能力 | 当前状态 | 说明 |
|---|---|---|
| 分组调度 | 已有 | 用户 API Key 绑定 Group，系统只在对应 Group 可用账号内选择 |
| 模型路由 | 已有 | Group 可配置模型到账号的路由规则 |
| 粘性会话 | 已有 | 同一 sessionHash 可绑定到同一账号，默认 TTL 约 1 小时 |
| 并发控制 | 已有 | 基于账号 Concurrency 控制并发槽位 |
| 当前负载 | 已有基础 | `AccountLoadInfo.LoadRate` 可表示账号当前负载 |
| 等待队列 | 已有 | 账号满载时可生成 `AccountWaitPlan` |
| 账号限速状态 | 已有基础 | 账号可根据 rate limit / cooldown 状态暂时不可调度 |
| 模型支持判断 | 已有 | 调度前会检查账号是否支持请求模型 |
| 渠道限制判断 | 已有 | 可根据 channel pricing / restriction 过滤不可用账号 |

当前 Layer 2 的主要排序逻辑偏规则化：

```text
1. 账号优先级 Priority，数值越小越优先
2. 当前负载 LoadRate，越低越优先
3. 未使用账号优先
4. 最久未使用账号优先
```

该策略简单稳定，但存在以下不足：

- 无法表达“低成本优先”“低延迟优先”“高质量优先”等不同业务目标。
- 成功率、TTFT、429 频率、错误率等运行时质量指标未统一参与决策。
- 账号之间的真实成本差异没有系统化进入调度。
- fallback chain 缺少策略化定义，只能依赖当前错误处理和重试逻辑。
- 多区域、多代理、多上游类型下缺少统一评分模型。

---

## 3. 升级原则

### 3.1 保持现有调度语义

智能评分只应增强账号选择，不应破坏现有关键语义：

- 账号必须属于当前用户可用 Group。
- 账号必须处于 Active 且 Schedulable 状态。
- 账号必须支持请求模型和请求端点。
- 账号不能处于明确 cooldown、rate limited、overloaded 或不可用状态。
- 并发槽位仍然是最终是否可立即执行的硬约束。
- 模型路由和粘性会话默认优先于评分调度。

### 3.2 硬过滤优先，评分排序其次

调度流程必须先做硬过滤，再做评分排序。

硬过滤负责排除不能用的账号：

```text
不可用账号直接排除，不参与评分。
```

评分排序只在可用账号之间比较优劣：

```text
可用账号之间，根据策略计算 score，分数高者优先。
```

### 3.3 所有评分统一为正向分数

所有指标最终必须归一化为 `[0, 1]` 的正向分数：

```text
0 = 最差
1 = 最好
```

例如：

- 延迟越低越好，但 `latency_score` 越高越好。
- 成本越低越好，但 `cost_score` 越高越好。
- 负载越低越好，但 `load_score` 越高越好。
- 成功率越高越好，`success_score` 越高越好。

---

## 4. 目标路由策略

系统应支持以下预设策略。

### 4.1 balanced

默认策略，兼顾质量、成本、延迟和负载。

适用场景：

- 普通用户默认调用。
- 需要稳定、成本和性能平衡的生产流量。

建议权重：

```text
latency_score     20%
cost_score        20%
success_score     25%
rate_limit_score  15%
load_score        15%
region_score       5%
```

### 4.2 cheapest

低成本优先策略。

适用场景：

- 大批量非实时任务。
- 对延迟不敏感的后台任务。
- 成本敏感型套餐或低价分组。

建议权重：

```text
cost_score        50%
success_score     20%
rate_limit_score  10%
load_score        10%
latency_score      5%
region_score       5%
```

硬约束：

```text
success_score >= 0.80
load_score    >= 0.10
rate_limit_score > 0
```

### 4.3 fastest

低延迟优先策略。

适用场景：

- 交互式聊天。
- IDE / CLI 实时补全。
- 对 TTFT 敏感的请求。

建议权重：

```text
latency_score     45%
success_score     25%
rate_limit_score  15%
load_score        10%
cost_score         3%
region_score       2%
```

硬约束：

```text
success_score >= 0.85
rate_limit_score > 0
load_score >= 0.05
```

### 4.4 best_quality

高质量优先策略。

适用场景：

- 企业用户。
- 高价值请求。
- 对失败率敏感的任务。

建议权重：

```text
success_score     45%
rate_limit_score  20%
latency_score     15%
load_score        10%
region_score       5%
cost_score         5%
```

硬约束：

```text
success_score >= 0.90
rate_limit_score > 0
```

### 4.5 fallback_chain

按评分结果生成候选账号链路，请求失败时按链路顺序尝试下一个账号。

适用场景：

- 高可用请求。
- 企业级 SLA。
- 上游容易间歇性失败的模型。

fallback_chain 不是简单重试，需要严格区分可重试和不可重试错误。

可重试错误：

| 错误类型 | 示例 |
|---|---|
| 网络错误 | timeout、connection reset、temporary DNS error |
| 上游 5xx | 500、502、503、504 |
| 上游限速 | 429、rate limit exceeded |
| 临时容量不足 | overloaded、server busy |
| 空响应或协议异常 | 上游返回非预期结构且请求未开始流式输出 |

不可重试错误：

| 错误类型 | 示例 |
|---|---|
| 用户请求错误 | 400、请求体非法、上下文超限 |
| 鉴权错误 | 用户 API Key 无效、余额不足、套餐无权限 |
| 模型不支持 | 当前 Group 或账号不支持该模型 |
| 已输出的流式请求 | SSE 已向客户端输出 token 后不得切换账号 |
| 明确业务拒绝 | 内容安全拦截、权限策略拒绝 |

fallback_chain 限制：

```text
默认最大尝试次数：2
企业高可用策略最大尝试次数：3
流式响应一旦开始输出，不再 fallback
每次 fallback 必须记录 audit log 和 excluded account
```

---

## 5. 评分模型

### 5.1 总分公式

```text
score =
    w_latency    * latency_score
  + w_cost       * cost_score
  + w_success    * success_score
  + w_headroom   * rate_limit_score
  + w_load       * load_score
  + w_region     * region_score
  + w_priority   * priority_score
```

约束：

```text
0 <= 每个子分数 <= 1
0 <= 每个权重 <= 1
所有权重之和建议为 1
score 越高越优先
```

`priority_score` 用于保留现有账号 Priority 的影响，避免智能评分完全覆盖管理员手动配置。

### 5.2 latency_score

延迟分数应优先使用账号级 TTFT，而不是仅使用代理节点延迟。

推荐数据来源优先级：

```text
1. account + model + endpoint 的滑动窗口 TTFT
2. account + model 的滑动窗口 TTFT
3. account 的滑动窗口平均延迟
4. proxy latency
5. 平台默认中位数
```

计算方式：

```text
latency_score = clamp(1 - (p50_ttft_ms / latency_baseline_ms), 0, 1)
```

建议使用 P50 / P95 组合：

```text
latency_score = 0.7 * p50_score + 0.3 * p95_score
```

这样可以避免只看平均值导致长尾延迟被忽略。

### 5.3 cost_score

成本分数不能简单使用模型单价，需要按请求端点和预估 token 量计算。

推荐成本估算维度：

| 维度 | 说明 |
|---|---|
| requested_model | 用户请求模型 |
| upstream_model | 实际上游模型 |
| endpoint_type | messages、chat_completions、responses、embeddings、images |
| input_token_price | 输入 token 单价 |
| output_token_price | 输出 token 单价 |
| cache_price | cache read/write 成本 |
| image_price | 图片生成或编辑成本 |
| proxy_cost | 代理额外成本，若有 |
| account_type | API Key、OAuth、订阅账号可能成本不同 |

推荐公式：

```text
estimated_cost =
    estimated_input_tokens  * input_token_price
  + estimated_output_tokens * output_token_price
  + estimated_cache_tokens  * cache_token_price
  + endpoint_extra_cost
  + proxy_extra_cost
```

归一化：

```text
cost_score = 1 - normalize(estimated_cost, min_cost, max_cost)
```

当无法估算成本时：

```text
cost_score = 0.5
```

### 5.4 success_score

成功率必须只统计可归因于账号或上游质量的问题，不应把用户错误计入账号失败率。

计入失败：

| 类型 | 示例 |
|---|---|
| 上游 5xx | 500、502、503、504 |
| 上游 429 | rate limit、quota exceeded |
| 网络错误 | timeout、connection reset |
| 鉴权失效 | 上游账号 token 失效、API Key 被拒绝 |
| 协议错误 | 上游返回无法解析的数据 |

不计入失败：

| 类型 | 示例 |
|---|---|
| 用户请求错误 | 400、参数错误、上下文过长 |
| 用户权限错误 | 用户余额不足、套餐无权限 |
| 本地策略拒绝 | 内容安全、本地模型限制 |
| 客户端中断 | 用户主动断开连接 |

推荐窗口：

```text
最近 5 分钟 + 最近 100 次请求
```

计算方式：

```text
success_score = successful_upstream_requests / attributed_total_requests
```

样本不足时：

```text
样本数 < 20 时使用贝叶斯平滑：
success_score = (success + prior_success) / (total + prior_total)
prior_success = 18
prior_total   = 20
```

### 5.5 rate_limit_score

限额余量不能只看 RPM，需要综合多类限制。

可能维度：

| 维度 | 说明 |
|---|---|
| RPM | 每分钟请求数 |
| TPM | 每分钟 token 数 |
| RPD | 每日请求数 |
| TPD | 每日 token 数 |
| provider quota | 上游平台额度 |
| cooldown | 账号临时冷却状态 |
| hidden limit | 无法准确知道的上游隐性限制 |

推荐计算：

```text
rate_limit_score = min(
  rpm_headroom_score,
  tpm_headroom_score,
  daily_quota_score,
  cooldown_score
)
```

其中：

```text
headroom_score = clamp((limit - used) / limit, 0, 1)
cooldown_score = 0 if now < cooldown_until else 1
```

如果缺少精确限额数据：

```text
无 cooldown 且近期无 429：rate_limit_score = 0.7
近期出现 429：rate_limit_score = 0.2
当前 cooldown 中：rate_limit_score = 0
```

### 5.6 load_score

负载分数基于当前并发槽位。

```text
load_score = clamp(1 - load_rate, 0, 1)
```

其中：

```text
load_rate = current_concurrency / effective_concurrency
```

注意：

- `effective_concurrency` 应考虑账号配置的 `Concurrency` 和 `EffectiveLoadFactor()`。
- 满载账号不应只给低分，而应在硬过滤或等待队列中处理。

### 5.7 region_score

地域分数用于就近路由，但不能作为强约束。

推荐等级：

| 情况 | region_score |
|---|---|
| 用户区域与账号出口区域完全匹配 | 1.0 |
| 同大区，例如同在亚洲、欧洲、北美 | 0.7 |
| 跨区但可接受 | 0.4 |
| 明确不推荐区域 | 0.1 |
| 无地域数据 | 0.5 |

注意：

- 如果当前只有 Proxy Region，没有账号 Region，应将其视为代理出口区域。
- 如果没有用户侧地域，不应启用强地域权重。

### 5.8 priority_score

保留管理员手动配置的账号 Priority。

当前 Priority 语义是数值越小越优先。

推荐转换：

```text
priority_score = 1 - normalize(priority, min_priority, max_priority)
```

如果所有候选账号 Priority 相同：

```text
priority_score = 1
```

---

## 6. 调度流程设计

### 6.1 总体流程

```text
用户请求
  ↓
API Key 鉴权
  ↓
解析 Group / Platform / Model / Endpoint
  ↓
硬过滤候选账号
  ↓
Layer 1: 模型路由
  ↓ 未命中或允许降级
Layer 1.5: 粘性会话
  ↓ 未命中或账号不可用
Layer 2: 策略评分排序
  ↓ 无可立即执行账号
Layer 3: 等待队列或 fallback chain
  ↓
选中账号并转发上游
  ↓
采集指标并更新滑动窗口
```

### 6.2 硬过滤条件

候选账号必须满足：

```text
account.status == Active
account.schedulable == true
account.group_id 匹配当前 Group
account.platform 匹配当前请求平台
account 支持 requested_model
account 支持 endpoint capability
account 未被 excludedIDs 排除
account 未处于 cooldown
account 未被渠道限制禁止
account 未超过硬性配额限制
```

不满足硬过滤条件的账号不进入评分阶段。

### 6.3 模型路由与评分策略关系

默认行为：

```text
模型路由优先级高于评分策略。
```

即如果 Group 的模型路由规则命中账号集合，则优先在该集合内评分。

建议新增配置：

```text
model_routing_fallback_enabled: true/false
```

行为：

| 配置 | 行为 |
|---|---|
| false | 模型路由命中后只在指定账号内选择，指定账号不可用则失败 |
| true | 指定账号不可用时，允许回退到 Group 内其他支持模型的账号 |

### 6.4 粘性会话与评分策略关系

默认行为：

```text
粘性会话优先级高于评分策略。
```

但粘性账号必须通过硬过滤。以下情况应清除粘性绑定：

```text
账号不存在
账号 disabled
账号不支持当前模型
账号处于 cooldown
账号被 channel restriction 禁止
账号连续失败超过阈值
```

建议新增配置：

```text
sticky_quality_break_enabled: true/false
sticky_min_success_score: 0.70
sticky_max_wait_ms: 3000
```

当粘性账号质量过低或等待过久时，可进入 Layer 2 重新评分选择。

### 6.5 Layer 2 评分选择

评分选择步骤：

```text
1. 获取候选账号列表
2. 批量获取实时指标
3. 对每个账号计算子分数
4. 根据策略权重计算总分
5. 按 score 降序排序
6. 分数相同则按 Priority、LoadRate、LastUsedAt 排序
7. 尝试获取并发槽位
8. 成功则选中账号
9. 失败则尝试下一个账号或进入等待队列
```

排序规则：

```text
score DESC
priority ASC
load_rate ASC
last_used_at ASC NULLS FIRST
account_id ASC
```

### 6.6 Layer 3 等待与 fallback

当所有候选账号没有可用并发槽位时：

1. 如果请求属于普通策略，生成等待计划。
2. 如果请求属于 fallback_chain，优先尝试下一个可用账号。
3. 如果所有账号都不可用，进入等待队列或返回无可用账号错误。

等待队列必须遵守：

```text
最大等待数限制
等待超时限制
请求上下文取消时释放等待
账号槽位释放后重新校验账号状态
```

---

## 7. 指标采集与存储

### 7.1 指标采集时机

每次上游请求完成后记录：

| 指标 | 说明 |
|---|---|
| account_id | 上游账号 ID |
| group_id | 当前用户组 |
| platform | Anthropic / OpenAI / Gemini / Antigravity |
| model | 请求模型 |
| endpoint | messages / responses / chat / embeddings / images |
| started_at | 请求开始时间 |
| ttft_ms | 首 token 延迟，非流式可为空 |
| total_latency_ms | 总耗时 |
| status_code | 上游状态码 |
| error_type | 标准化错误类型 |
| retryable | 是否可重试 |
| input_tokens | 输入 token 数 |
| output_tokens | 输出 token 数 |
| estimated_cost | 预估成本 |
| proxy_id | 使用的代理 ID |
| region | 出口地域 |

### 7.2 Redis 滑动窗口

建议 Redis Key：

```text
routing:metrics:{platform}:{account_id}:{endpoint}:{model}:5m
routing:metrics:{platform}:{account_id}:{endpoint}:5m
routing:metrics:{platform}:{account_id}:5m
```

记录内容：

```json
{
  "success": 92,
  "attributed_failure": 8,
  "retryable_failure": 5,
  "p50_ttft_ms": 850,
  "p95_ttft_ms": 2400,
  "avg_latency_ms": 1800,
  "rate_limit_count": 2,
  "total_requests": 100,
  "updated_at": 1760000000
}
```

### 7.3 指标缺失默认值

| 指标 | 默认值 |
|---|---|
| latency_score | 0.5 |
| cost_score | 0.5 |
| success_score | 使用贝叶斯先验，约 0.9 |
| rate_limit_score | 0.7 |
| load_score | 根据实时并发计算 |
| region_score | 0.5 |
| priority_score | 根据现有 Priority 计算 |

---

## 8. 配置设计

### 8.1 系统级配置

建议新增系统设置：

```json
{
  "routing_strategy_enabled": true,
  "default_routing_strategy": "balanced",
  "metrics_window_seconds": 300,
  "min_sample_size": 20,
  "fallback_chain_max_attempts": 2,
  "sticky_quality_break_enabled": false,
  "model_routing_fallback_enabled": false
}
```

### 8.2 Group 级配置

建议在 Group 配置中增加：

```json
{
  "routing_strategy": "balanced",
  "routing_weights": {
    "latency": 0.20,
    "cost": 0.20,
    "success": 0.25,
    "rate_limit": 0.15,
    "load": 0.15,
    "region": 0.05,
    "priority": 0.00
  },
  "routing_constraints": {
    "min_success_score": 0.80,
    "max_load_rate": 0.95,
    "allow_fallback_outside_model_routing": false
  }
}
```

### 8.3 Account 级配置

建议在账号配置中增加可选字段：

```json
{
  "routing_enabled": true,
  "routing_weight_override": null,
  "region": "us-east",
  "cost_multiplier": 1.0,
  "quality_tier": "standard",
  "max_fallback_attempts_per_request": 1
}
```

---

## 9. 管理后台设计

### 9.1 Group 管理页面

需要展示和配置：

- 当前路由策略。
- 是否启用自定义权重。
- 各评分权重。
- 最低成功率门槛。
- 是否允许模型路由失败后 fallback。
- 是否允许粘性会话质量破坏。

### 9.2 账号管理页面

需要展示：

| 字段 | 说明 |
|---|---|
| 当前 score | 当前策略下账号总分 |
| success_score | 成功率分数 |
| latency_score | 延迟分数 |
| cost_score | 成本分数 |
| rate_limit_score | 限额余量分数 |
| load_score | 当前负载分数 |
| region_score | 地域分数 |
| 最近失败原因 | 最近一次可归因失败 |
| 是否参与调度 | 硬过滤结果 |

### 9.3 调度解释能力

建议在调试模式或管理后台提供“为什么选中这个账号”的解释：

```json
{
  "selected_account_id": 123,
  "strategy": "balanced",
  "score": 0.86,
  "scores": {
    "latency": 0.82,
    "cost": 0.74,
    "success": 0.96,
    "rate_limit": 0.90,
    "load": 0.88,
    "region": 0.50
  },
  "rejected_accounts": [
    {
      "account_id": 456,
      "reason": "cooldown"
    },
    {
      "account_id": 789,
      "reason": "model_not_supported"
    }
  ]
}
```

---

## 10. 实施计划

### Phase 1：评分框架与 balanced 策略

目标：引入最小可用评分框架，不改变现有调度稳定性。

任务：

1. 新增 `RoutingStrategy` 类型和预设权重。
2. 新增 `AccountRoutingMetrics` 结构。
3. 新增 `ScoreEngine`，输入账号、负载、策略，输出分数。
4. Layer 2 中将硬编码排序替换为评分排序。
5. 保留 Priority、LoadRate、LastUsedAt 作为 tie-breaker。
6. 增加单元测试覆盖 balanced 排序。

验收标准：

```text
在未启用智能路由时，调度结果与现有逻辑保持一致。
启用 balanced 后，可根据 score 选择账号。
所有硬过滤条件仍然生效。
```

### Phase 2：实时指标采集

目标：补齐账号级成功率、延迟、限额信号。

任务：

1. 在上游请求完成后记录 routing metrics。
2. Redis 中维护 5 分钟滑动窗口。
3. 成功率只统计可归因错误。
4. TTFT 和总延迟分开记录。
5. 429 / cooldown 影响 rate_limit_score。
6. 管理后台展示账号实时指标。

验收标准：

```text
账号列表能看到最近 5 分钟成功率、TTFT、负载和限速状态。
样本不足时使用默认分数，不导致调度异常。
```

### Phase 3：cheapest / fastest / best_quality 策略

目标：上线多策略选择。

任务：

1. 增加三种预设策略。
2. Group 支持选择策略。
3. 支持策略级硬约束。
4. 增加成本估算逻辑。
5. 增加策略切换测试。

验收标准：

```text
cheapest 倾向选择低成本账号，但不会选择低成功率账号。
fastest 倾向选择低 TTFT 账号，但不会选择限速账号。
best_quality 倾向选择高成功率账号。
```

### Phase 4：fallback_chain

目标：为高可用场景提供安全 fallback。

任务：

1. 根据 score 生成候选账号链。
2. 定义 retryable error 分类。
3. 非流式请求支持 fallback。
4. 流式请求在首 token 输出前可 fallback，输出后不可 fallback。
5. fallback 记录 audit log。
6. fallback 后将失败账号加入 excludedIDs，避免同一请求重复选择。

验收标准：

```text
timeout / 429 / 5xx 可触发 fallback。
400 / 权限错误 / 已开始输出的流式请求不会 fallback。
每次 fallback 都有日志可追踪。
```

### Phase 5：地域感知与自定义权重

目标：支持更细粒度企业配置。

任务：

1. 补充账号或代理出口地域字段。
2. 根据用户请求来源计算 region_score。
3. Group 支持自定义权重。
4. 增加权重校验，确保总和合理。
5. 管理后台增加评分预览。

验收标准：

```text
开启地域权重后，同区域账号得分更高。
自定义权重非法时不保存。
评分预览能解释每个账号的分数来源。
```

---

## 11. 测试计划

### 11.1 单元测试

必须覆盖：

- 分数归一化。
- 不同策略的权重计算。
- 成本、延迟、成功率缺失时的默认值。
- Priority tie-breaker。
- load 满载账号不被直接选中。
- success_score 低于硬约束时被过滤。
- cooldown 账号 rate_limit_score 为 0。

### 11.2 集成测试

必须覆盖：

- Group 策略配置影响账号选择。
- 模型路由命中后只在指定账号内评分。
- 模型路由 fallback 开启后可回退到其他账号。
- 粘性账号可用时继续使用粘性账号。
- 粘性账号不可用时清除 sticky 并重新评分。
- fallback_chain 对 429 / 5xx 生效。
- fallback_chain 不对 400 / 鉴权错误生效。

### 11.3 回归测试

必须验证：

- 智能路由关闭时，现有调度行为不变。
- API Key 鉴权不受影响。
- 用户余额和计费不受影响。
- 并发槽位释放正常。
- 请求取消时不会泄漏等待队列或并发槽位。

---

## 12. 风险与控制措施

| 风险 | 影响 | 控制措施 |
|---|---|---|
| 指标冷启动 | 新账号得分不准确 | 使用贝叶斯先验 + 中位默认分 + 逐步放量 |
| 指标抖动 | 调度频繁变化 | 使用滑动窗口、P50/P95、最小样本数 |
| 权重配置错误 | 账号选择异常 | 提供预设策略，限制自定义权重范围 |
| fallback 重复扣费 | 成本增加或重复请求 | 仅 retryable 错误 fallback，流式输出后禁止 fallback |
| 粘性会话破坏 | 多轮对话上下文不一致 | 默认保持 sticky，质量破坏需要显式开启 |
| 成本估算不准 | cheapest 策略偏差 | 初期用模型定价估算，后续用真实 usage 校准 |
| Redis 指标丢失 | 分数回退默认值 | 所有指标缺失都有安全默认分 |
| 打分增加延迟 | 网关性能下降 | 批量读取指标，O(n) 计算，缓存预计算结果 |

---

## 13. 兼容性设计

### 13.1 默认关闭智能路由

初始上线建议：

```text
routing_strategy_enabled = false
```

开启后默认策略为：

```text
balanced
```

### 13.2 保留旧排序作为 fallback

当出现以下情况时使用旧排序：

```text
智能路由关闭
评分配置非法
指标服务不可用且不允许默认分
策略名称未知
```

旧排序：

```text
Priority ASC
LoadRate ASC
LastUsedAt ASC NULLS FIRST
```

### 13.3 灰度上线

建议灰度顺序：

```text
1. 管理员测试 Group
2. 内部低流量 Group
3. 普通用户 balanced
4. 企业用户 best_quality
5. fallback_chain 单独灰度
```

---

## 14. 最终目标架构

```text
                    ┌──────────────────────┐
                    │      用户 API 请求     │
                    └──────────┬───────────┘
                               ↓
                    ┌──────────────────────┐
                    │ API Key / Group 校验  │
                    └──────────┬───────────┘
                               ↓
                    ┌──────────────────────┐
                    │ 候选账号硬过滤        │
                    └──────────┬───────────┘
                               ↓
        ┌──────────────────────┼──────────────────────┐
        ↓                      ↓                      ↓
┌───────────────┐      ┌────────────────┐      ┌────────────────┐
│ 模型路由       │      │ 粘性会话         │      │ Score Engine    │
└───────┬───────┘      └───────┬────────┘      └───────┬────────┘
        ↓                      ↓                       ↓
        └──────────────┬───────┴───────────────┬───────┘
                       ↓                       ↓
              ┌────────────────┐       ┌────────────────┐
              │ 并发槽位获取     │       │ fallback chain  │
              └───────┬────────┘       └───────┬────────┘
                      ↓                        ↓
              ┌────────────────┐       ┌────────────────┐
              │ 转发上游账号     │       │ 等待/重试/失败   │
              └───────┬────────┘       └────────────────┘
                      ↓
              ┌────────────────┐
              │ 指标采集与更新   │
              └────────────────┘
```

---

## 15. 结论

智能路由升级方向可行，但不应简单地将现有排序替换为一个公式。正确做法是：

1. 保留现有硬过滤、模型路由、粘性会话、并发槽位和等待队列机制。
2. 在 Layer 2 引入统一 Score Engine。
3. 使用策略权重表达 cheapest、fastest、best_quality、balanced 等业务目标。
4. 使用 Redis 滑动窗口补齐账号级实时指标。
5. 对 fallback_chain 做严格错误分类，避免重复扣费和流式响应异常。
6. 通过默认关闭、灰度启用和旧排序 fallback 保证上线安全。

推荐优先落地顺序：

```text
balanced 评分框架 → 实时指标采集 → 多策略 → fallback_chain → 地域与自定义权重
```
