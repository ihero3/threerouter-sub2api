# ThreeRouter / sub2api 安全审计与整改计划

> 审计对象：`D:/li/threerouter-sub2api`（Go 后端 `backend/` + Vue 前端 `frontend/`）
> 审计方式：静态代码审计（只读），所有结论均附 `文件:行号`
> 日期：2026-10-31
> 触发背景：线上出现邀请返利期间大量注册、同一 IP 频繁注册、更换 IP 频繁注册

---

## 一、执行摘要

代码整体工程质量好于同类项目：SQL 注入零发现（全走占位符 / `pq.QuoteIdentifier` / switch 白名单）、文件上传实现规范（类型嗅探 + 随机名 + 0644 + 双层穿越校验）、支付 Webhook 全部做真签名校验、用户面 IDOR 防护正确。

**但反垃圾注册体系存在系统性缺口**，并且有一批默认配置处于"不安全默认值"状态。线上三个现象不是偶发，而是以下根因叠加的必然结果：

| 你观察到的现象 | 技术根因（已核实） |
|---|---|
| **邀请返利时大量注册** | 注册成功即向邀请人发放 `$1.00`（`domain_constants.go:765`），无邀请人数上限、不受冻结期/有效期/单人上限约束（`affiliate_service.go:643`、`affiliate_repo.go:183`）；一次性邮箱仅打标不拦截（`affiliate_risk.go:16-17`） |
| **同一 IP 频繁注册** | 「同 IP 每日注册配额」默认关闭（`setting_parse.go:64`）；IP 风控为精确等值匹配且可被伪造 IP 绕过；唯一有效的 5 次/分限流对慢速攻击无效（1 IP = 300 个号/小时） |
| **更换 IP 频繁注册** | 无任何跨 IP 维度防线：无 CIDR 段聚合、无设备指纹、无支付账号关联、无邀请人总邀请上限、无全局注册速率上限。代理池每换一个出口 IP，`count` 归零 |

**最致命的一条**：注册流程使用的客户端 IP 解析默认走"信任原始转发头"的兼容路径（`config.go:2083` viper 默认 `true`），而限流走的却是可信代理链。结果是**写入 `users.register_ip` 的值可被攻击者通过 `X-Forwarded-For` 完全控制**——这同时摧毁了 IP 风控、事后取证与封禁能力。详见 P0-2。

---

## 二、漏洞分级清单

### P0 — 紧急（建议 24–48 小时内处理）

#### P0-1  JWT 密钥硬编码且弱密钥检测无效化

- **证据**：`backend/config.yaml:26` → `secret` 为一个硬编码的、以 `dev-only-jwt-secret-...` 开头的可预测字符串（具体值见部署机 `config.yaml`，此处不摘录以免扩散）
- **证据**：`internal/config/config.go:3740-3757` `isWeakJWTSecret` 仅对 8 个固定字符串做完全相等匹配（`changeme`/`secret`/`password`/`123456` 等），上述密钥**不在名单内**，连告警都不会触发
- **证据**：`config.go:1977-1979` 即便命中弱密钥也只 `slog.Warn`，**不拒绝启动**
- **影响**：密钥一旦泄露（该文件已被提交习惯、运维拷贝、镜像打包等途径外泄风险高），攻击者可用 HS256 伪造**任意用户乃至管理员**的 JWT，`middleware/admin_auth.go:166` 的 role 校验一并绕过 → 全站接管
- **修复**：
  1. 立即轮换：`openssl rand -base64 48` 生成新密钥，写入环境/密钥管理，**不要留在 config.yaml**
  2. 把弱密钥检测改为：长度 < 32 **或** 熵不足 **或** 命中常见词模式（含 "dev-only" / "please-change" / "secret" 子串）→ **生产模式（`mode: release`）下直接拒绝启动**，debug 模式才降级为告警
  3. 启动自检：生产模式下若检测到默认密钥立即 `os.Exit(1)`

#### P0-2  注册 IP 可被伪造头控制，IP 维度风控全线失效 ★ 核心

- **证据**：`internal/config/config.go:2083` → `viper.SetDefault("security.trust_forwarded_ip_for_api_key_acl", true)`，而 `config.yaml` 未显式覆盖该键 → **实际生效 true**
- **证据**：`internal/server/middleware/session_binding.go:19-27`
  ```go
  ip.SetForwardedIPSettings(c, forwardedIPSettings.TrustForwardedIP, forwardedIPSettings.Headers)
  binding := &service.SessionBinding{
      IP: ip.GetSecurityClientIP(c, forwardedIPSettings.TrustForwardedIP),
  ```
  `trustForwarded=true` → `GetSecurityClientIP` 走 `GetClientIP`（`ip.go:164-172`）
- **证据**：`ip.go:113-148` `resolveLegacyForwardedHeaderIP` **优先信任** `CF-Connecting-IP` → `X-Real-IP` → `X-Forwarded-For`
- **证据**：注册取 IP 走的就是这条路 —— `internal/handler/auth_handler.go:207` `clientIP := middleware2.SecurityClientIP(c)` → `session_binding.go:47-53` 返回 `binding.IP`
- **证据**：`internal/server/http.go:94-105` `configureTrustedProxies`：因 `config.yaml` 未配 `server.trusted_proxies`（`TrustedProxiesConfigured=false`）→ `r.SetTrustedProxies(nil)`，即 Gin **不信任任何代理**
- **同期参照**：限流中间件刻意绕开该开关 `internal/middleware/rate_limiter.go:116-132`，注释写明"必须走 server.trusted_proxies 可信代理链，不信任原始转发头"
- **影响**（三重叠加）：
  1. 攻击者每请求携带一个随机公网 `X-Forwarded-For` → `users.register_ip` 记录垃圾值，**事后取证与 IP 封禁完全失效**
  2. `enforceIPGuard`（邀请人 × 同 IP 关联，`affiliate_service.go:662-697`）判定基于该值 → 直接绕过
  3. 「同 IP 每日注册配额」即便管理员手动开启也形同虚设
- **修复**：
  1. **立即**：`config.yaml` 显式设置 `server.trusted_proxies`（填真实 Nginx/CDN 内网段），并在 `security` 下把 `trust_forwarded_ip_for_api_key_acl: false`
  2. 把 `SecurityClientIP` 的默认立场改为**先取可信代理链**，仅当 `trusted_proxies` 显式配置且开关显式开启时才用转发头
  3. 新增 `register_ip` **写入前的合法性校验**：与 `X-Forwarded-For` 不一致时记录 `register_ip_confidence` 标记；给风控用的 IP 一律取 `GetTrustedClientIP`
  4. 对存量：回填迁移中无法判定的记录标记为低置信度

#### P0-3  OAuth 注册链路硬编码传空 IP，彻底绕过 IP 风控

- **证据**：`internal/service/auth_service.go:1062-1065`
  ```go
  // OAuth 注册链路拿不到客户端 IP，传空串让 IP 关联风控自动跳过（fail-open）。
  if err := s.affiliateService.BindInviterByCode(ctx, userID, code, ""); err != nil {
  ```
- **证据**：`internal/service/affiliate_service.go:663-666` 空 IP 直接放行
  ```go
  registerIP = strings.TrimSpace(registerIP)
  if registerIP == "" { return nil }
  ```
- **调用点共 3 处**：`auth_service.go:911`、`auth_oauth_email_flow.go:319`、`auth_email_oauth_auto.go:207`
- **讽刺之处**：同一条 OAuth 链路 `auth_email_oauth.go:382` 明明已经取到 `middleware2.SecurityClientIP(c)`，只是没往下游传
- **叠加放大**：OAuth `create-account` / `complete-registration` 限流为 **10/min**（`routes/auth.go:90,101,139,151,163,181,199,211,227,239`），是邮箱注册 5/min 的**两倍**
- **影响**：攻击者无需伪造任何请求头，切到任一已启用的 OAuth 入口即可让 IP 风控完整失效，且以双倍速率注册
- **修复**：给 `bindOAuthAffiliate` 增加 `registerIP` 参数并透传；OAuth 入口限流下调至 5/min 与邮箱注册对齐；空 IP 的 fail-open 改为 fail-close（或至少记录 `ip_unknown` 并纳入"同 IP 未知"聚合桶）

#### P0-4  数据库 / Redis 凭据明文弱口令 + 调试模式

- **证据**：`backend/config.yaml:6-15` → `database.user` 与 `database.password` **取值相同**（用户名即密码，具体值见部署机 `config.yaml`）、`sslmode: "disable"`
- **证据**：`config.yaml:18-23` Redis `password: ""` 且未改默认端口
- **证据**：`config.yaml:4` `mode: "debug"`、`config.yaml:32` `log.level: "debug"`；`internal/server/http.go:47-49` 只有 `release` 才切 `gin.ReleaseMode`
- **证据**：`internal/repository/http_upstream.go:262-268` debug 日志打印**完整代理 URL**（常含 `socks5://user:pass@host` 凭据）；对照 `pkg/tlsfingerprint/dialer.go:144,185` 只记 Host 的正确写法
- **缓释**：已确认 `config.yaml` 未被 git 跟踪（`git ls-files` 无匹配），泄露面仅限本机/部署介质
- **修复**：改强口令并移出配置文件（走环境变量）；Redis 设 `requirepass`；生产切 `release` + `log.level: info`；把 `http_upstream.go:262` 的 `proxyInfo` 改为只输出 `Host`

---

### P1 — 高（建议 1 周内）

#### P1-1  邀请注册奖励无上限、无门槛、结算即到账

- **证据**：`domain_constants.go:762-766` 默认 `$1.00/人`，可配至 `$100`
- **证据**：`affiliate_service.go:640-649` 被邀请人注册成功即 `AccrueRegistrationReward`，失败只记日志不回滚
- **证据**：`affiliate_repo.go:182-184` 直接累加进**可用** `aff_quota`，**不进冻结期**（对比充值返利走 `AccrueQuota` 有 freeze）
- **证据**：`affiliate_repo.go:210` 单人上限统计硬过滤 `action='accrue'`，注册奖励不参与
- **证据**：9 个 `affiliate_*` 设置键（`domain_constants.go:239-251,762-792`）中**无**任何"邀请人数上限"
- **影响**：这是"邀请返利期间大量注册"的直接经济动机。攻击者无需高技巧，慢速低频即可收割：**单出口 IP 5 个/分 → 300 个/小时 → 每小时 $300 返利负债**，跨 IP 则完全无上限
- **修复**（四选三即可大幅收敛）：
  1. 新增 `affiliate_register_reward_max_invitees`（每人总邀请上限）与 `affiliate_register_reward_daily_cap`（单邀请人每日封顶）
  2. 注册奖励**套用冻结期**（复用 `affiliate_rebate_freeze_hours`，建议 24–72h），并纳入 `per_invitee_cap` 统计口径
  3. 改为「**被邀请人首充/实名/活跃达标后**」才解锁注册奖励（最有效，但需产品决策）
  4. 对 `risk_level = abnormal` 的上级暂停发放

#### P1-2  一次性邮箱只标记不拦截

- **证据**：`affiliate_risk.go:14-17` 注释"只标记、不拦截"；`affiliate_risk.go:147-158` `isDisposableEmail` 唯一调用点在 `evaluateAffiliateRisk`
- **证据**：黑名单仅 17 条且**硬编码在 Go 代码**（`domain_constants.go:847-865`），更新需发版
- **影响**：临时邮箱批量注册畅通无阻
- **修复**：把 `isDisposableEmail` 前移到注册预检做**硬拦截**（配置开关控制开关严格度）；黑名单外置为可热更新配置/远程列表

#### P1-3  SSRF：私网访问默认放行

- **证据**：`config.go:2075-2076` → `allow_private_hosts: true`、`allow_insecure_http: true`（viper 默认），`config.yaml:38-57` 未覆盖 → **当前生效 true**
- **证据**：`internal/repository/http_upstream.go:580-605` `shouldValidateResolvedIP()` 返回 `Enabled && !AllowPrivateHosts`；`allow_private_hosts=true` 时**不安装 CheckRedirect 校验器**，私网与 localhost 字面量放行
- **证据**：触发面广 —— `routes/admin.go:161` `ProbeEndpoint`、`:464` Upstream Billing Probe、`:617` Proxy Test、`:697` Web Search Test
- **修复**：`config.yaml` 显式 `allow_private_hosts: false`、`allow_insecure_http: false`；代码中把这两个的默认值改为 `false`

#### P1-4  JWT 无法吊销（登出 / 踢下线对 access token 无效）

- **证据**：`internal/service/auth_service.go:2105-2121` 代码注释自认 —— `users` 表**没有 `token_version` 列**，自增只改内存副本
- **证据**：`auth_service.go:2129-2141` `resolvedTokenVersion` 实为 `TokenVersion XOR sha256(email + password_hash)`，只能靠**改密码**改变
- **证据**：`handler/auth_handler.go:776-794` Logout 仅在请求体传了 refresh token 时才撤销它，对 stateless JWT 无任何处置；`:817` 却返回"All sessions have been revoked"
- **影响**：账号被盗后改密前，攻击者手里的 access token 在 `config.yaml:27` 的 **24 小时**窗口内持续可用
- **修复**：新增 `users.token_version` 列 + 迁移；签发时写入 claim，校验时比对；`RevokeAllUserTokens` 自增该列使之真正生效

#### P1-5  自定义 API Key 明文存储且允许弱值

- **证据**：`internal/service/api_key_service.go:414-433` `ValidateCustomKey` 仅校验长度 ≥16 与字符集 → `1234567890123456` 合法
- **证据**：`api_key_service.go:550-560` Key **明文落库**（对比 `content_moderation.go:2764` 的哈希与 `auth_service.go:2124` refresh token 哈希，项目具备哈希能力但未用）
- **影响**：数据库只读泄露 = 全部用户 API Key 直接可用
- **修复**：存储 `sha256(key)`，验证时比对（保留前缀做展示）；自定义 Key 增加熵/常见弱值黑名单校验

---

### P2 — 中（建议 2–4 周）

| # | 问题 | 证据 | 修复建议 |
|---|---|---|---|
| P2-1 | `POST /api/v1/public/tickets` 匿名写库，**无限流无验证码**（对比 auth 全家都挂了 fail-close 限流，此处是缺口） | `routes/user.go:24`；`handler/ticket_handler.go:38-63` | 补限流 + 验证码 |
| P2-2 | `GET /api/v1/settings/public` 过度暴露部署配置：API base URL、已启用 OAuth/支付通道、站点身份、注册域名配额策略、验证码站点 key | `routes/auth.go:251`；`handler/setting_handler.go:39-99+` | 收敛为前端渲染必需的最小字段白名单 |
| P2-3 | `GET /models` 未鉴权且返回**实际部署**的真实模型清单 | `routes/gateway.go:486`；`handler/gateway_handler.go:1167-1180` | 加鉴权或返回静态兜底清单 |
| P2-4 | `metadata.user_id` 可由客户端透传上游，污染计费归因与风控 | `service/gateway_request.go:223`；`gateway_forward.go:232-238`；开关 `domain_constants.go:673` 默认 false | 默认改为忽略客户端值，始终用服务端account UUID |
| P2-5 | `/plugin-ui/:token/*path` 脱离 adminAuth 与审计中间件（靠 AES-GCM 短时能力令牌兜底，非裸奔，但无审计留痕） | `routes/admin.go:24`（group 在 `:26` 才创建）；`plugin_manager.go:823-876` | 移入 admin group 或补审计日志 |
| P2-6 | 管理员手动加余额产生返利且**无任何幂等**，重复点击重复计提 | `admin_user.go:748-764`；`affiliate_service.go:699-701` | 走 `payment_audit_logs` 占位或直接不产生返利（已改为默认关闭 `affiliate_admin_recharge_enabled: false`，但代码路径仍在） |
| P2-7 | 充值**退款不追回已发返利** —— 存在"充值→返利→退款"套现闭环 | `payment_refund.go:157-181` 全文无 `affiliate` 字样 | 退款时按比例冲销对应已发返利，透支则扣负至 `aff_quota` |
| P2-8 | 邀请码可被管理员改成 **4 字符**（可暴力枚举撞库）、无保留词黑名单 | `affiliate_service.go:34,41-66,897-906` | 自定义码最短提高到 8 位 + 保留词黑名单 |
| P2-9 | `register_reward` 账本**无唯一约束**，绑关系 CAS 是唯一防线 | `migrations/131:13-24`、`migrations/239`（均为普通索引） | 增加 `(user_id, action, source_user_id)` 唯一索引做幂等兜底 |
| P2-10 | `register_ip` **缺索引**，`CountUsersByRegisterIP` 将退化为全表扫描 | `migrations/240` 只加列未建索引 | 补 `CREATE INDEX CONCURRENTLY ON users(register_ip)` |
| P2-11 | 密码强度仅 `min=6` 且**只在 handler 层**，`RegisterWithVerification` 完全不校验 | `auth_handler.go:59`；`auth_service.go:326` | 下沉到 service，加复杂度与 72 字节上限（bcrypt 截断） |
| P2-12 | 风险标记 `risk_level` **不参与任何鉴权/限流/结算**（迁移注释自述） | `migrations/245:8`；`affiliate_risk.go:16-17` | 至少让 `abnormal` 拦截注册奖励发放与 quota 转余额 |
| P2-13 | 无 CSRF 框架，仅靠 Bearer-only + `SameSite=Lax` 架构性缓解 | 全仓无 csrf 实现 | 维持现状并写入规范：禁止引入 cookie-only 会话认证 |
| P2-14 | 错误信息少量回显 internals（`err.Error()` 直出） | `api_key_handler.go:201,296`；`plugin_handler.go:77,100,167,180` | 统一错误码，internal 细节仅进日志 |

### P3 — 低 / 加固项

- bcrypt cost 10 → 建议 12（`auth_service.go:1674`、`service/user.go:113`）
- 返利金额全程 `float64` 运算，仅 `roundTo(_,8)`（`affiliate_service.go:738`）→ 建议金额改 decimal / 分为单位
- `BatchSetUserRebateRate` 无行数校验，一次可给全站设 100%（`affiliate_repo.go:1703-1726`）；建议加行数上限与二次确认
- 风险信号 SQL 把 `password_hash` 拉进 Go 内存做分组统计（`affiliate_repo.go:1887-1892`），建议改为 SQL 侧聚合，避免密码哈希离开数据库
- 无 `.gitignore` 对 `backend/config.yaml` 的显式保护（当前未被 git 跟踪，属部署纪律而非配置保障），建议补 ignore 规则防止误提交

---

## 三、分阶段整改路线图

### 阶段 0：止血（不改代码，今天就能做完）

1. 轮换 JWT 密钥（强随机 48 字节），移出 `config.yaml`
2. 改 DB / Redis 强口令；Redis 设 `requirepass`；生产切 `mode: release`、`log.level: info`
3. `config.yaml` 显式配置：
   ```yaml
   server:
     trusted_proxies: ["<你的 Nginx/CDN 内网段>"]
   security:
     trust_forwarded_ip_for_api_key_acl: false
     url_allowlist:
       allow_private_hosts: false
       allow_insecure_http: false
   ```
4. 后台临时把 `affiliate_register_reward` 调为 0（先停损，再谈策略）
5. 临时关闭当前未使用的 OAuth 注册渠道

### 阶段 1：堵住核心缺口（1 周）

- P0-2 注册 IP 取可信代理链 + 置信度标记
- P0-3 OAuth 透传真实 IP + 空 IP 改 fail-close + 限流对齐 5/min
- P1-1 注册奖励：加人数/日额上限 + 套用冻结期 + `abnormal` 上级停发
- P1-2 一次性邮箱硬拦截 + 黑名单外置热更新
- P1-3 SSRF 默认值翻转
- P1-4 新增 `users.token_version` 使吊销真正生效

### 阶段 2：纵深防御（2–4 周）

- **注册前置人机验证**：把 `registration_click_captcha_enabled` 与 `email_verify_enabled` 默认打开，或接入 Turnstile / 云厂商无感验证
- **IP 段聚合风控**：IPv4 按 /24、IPv6 按 /64 对注册与邀请返利计数（这是解决"更换 IP"最关键的一条）
- **设备/浏览器指纹**：复用已有 `HashFingerprint(ip, ua)`（`registration_click_captcha.go:372`），扩展到注册布隆统计
- **全局注册速率熔断**：全站注册数在 5 分钟窗口内超阈值 → 自动提升验证强度或暂停注册并告警
- **邮箱维度**：`<local>+N@` / Gmail 点号已做；补随机本地部（熵判据）与一次性域名拦截
- P1-5 API Key 哈希存储；P2-1/2-2/2-3 收敛暴露面；P2-7 退款追回返利

### 阶段 3：运营与可观测（持续）

- **异常注册看板**：近 1/24h 注册数、Top register_ip / IP 段 / 邮箱域名、返利发放额趋势
- **自动告警**：注册速率突变、单邀请人日邀请数、返利总额超预算 → Webhook / 邮件
- **返利预算熔断**：日/周返利支出超阈值自动降级发放策略
- **把 `risk_level` 接入实际业务决策**（当前标记即等于没标记）
- **复盘机制**：活动前压风控阈值，活动中监控，事后审计 `register_reward` 账本

---

## 四、验证清单（整改后回归）

- [ ] 携带随机 `X-Forwarded-For` 注册，`users.register_ip` **不等于**伪造值
- [ ] 同一 IP 1 分钟内第 6 次注册返回 429
- [ ] 同 IP /24 段内第 N+1 个账号绑定同一邀请人被拦截
- [ ] OAuth 注册链路传入真实 IP，`enforceIPGuard` 生效
- [ ] 一次性邮箱域名注册被拒绝
- [ ] 邀请人邀请数达上限后不再发放注册奖励
- [ ] 管理员改密 / 踢下线后，旧 access token 立即失效（不再依赖 24h 过期）
- [ ] 伪造 `admin` role 的 JWT（用旧密钥签）被拒绝
- [ ] 通过 Channel `base_url` 指向 `169.254.169.254` 的 Probe 请求被拦截
- [ ] `SELECT * FROM users WHERE register_ip = '<ip>'` 走索引而非全表扫描

---

## 五、附注：本项目的既有优点（不必返工）

- SQL 注入零发现；`ORDER BY` 走 switch 白名单兜底
- 文件上传实现规范（类型嗅探不信客户端 + 随机名 + 0644 不可执行 + 双层穿越校验）
- 支付 Webhook 全部真签名校验（Stripe/Alipay/Wxpay/Airwallex/EasyPay），body 限 1MB
- JWT 算法白名单（禁 `alg=none`）、过期强校验、长度上限、refresh token SHA-256 落库
- Admin Key 用 `ConstantTimeCompare`；忘记密码静默成功防枚举
- 用户面 IDOR 防护正确；团队 RBAC 已修复历史越权
- 邀请码强度足够（crypto/rand 12 字符 / 32 字符集，无取模偏差，DB UNIQUE）
- 充值返利有基于 `payment_audit_logs` 唯一索引的真幂等；转余额有 `FOR UPDATE`
- 邮箱 `+别名` / Gmail 点号归一化去重（registration_email_alias.go）
- CORS 未放行通配符反射；`InsecureSkipVerify` 硬编码禁止；代理失败不回退直连
