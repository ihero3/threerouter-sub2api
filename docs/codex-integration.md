# Codex 接入（推荐写法）

> 适用于 Codex 0.144 及以后（新客户端一律走同一套契约，旧版本顺带兼容）
> 本文所有结论均来自实测：Codex `0.162.0-alpha.17.2`（`app-server` 的 `model/list`，即
> 桌面端/GUI 模型选择器真正读的数据）与 `codex exec` 的真实推理请求。
> 服务端已就绪，用户侧只需改 `~/.codex/config.toml`。

一句话：**只写 `base_url` + 凭证，让 Codex 自己请求 `GET {base_url}/models?client_version=<版本>`，
服务端会返回完整目录 `{"models":[…]}`。** 目录由平台按你的分组实时生成，因此你永远只能看到
自己真正可用的模型（`slug` 与可路由模型名同源）。

---

## 1. 三分钟接通

### 1.1 一键脚本（推荐）

```bash
# 预演：只打印将要写入的差异，不动任何文件
python3 scripts/codex-setup.py --api-key sk-你的平台Key

# 确认无误后落盘
python3 scripts/codex-setup.py --api-key sk-你的平台Key --apply
```

脚本做三件事：拉取并**校验**目录、写入 `~/.codex/threerouter-models.json`、
按选定认证方式补齐 `config.toml`（并自动注释掉会破坏自动发现的那一行）。
`--help` 有全部参数，常用几个：

| 参数 | 作用 |
|---|---|
| `--auth env`（默认） | 平台 API Key 放环境变量 `THREEROUTER_API_KEY`，Key 不落盘 |
| `--auth openai` | 复用 Codex 的 ChatGPT 登录态（`requires_openai_auth = true`） |
| `--auth none` | 凭证交给已有的 provider 段，只补目录相关配置 |
| `--with-catalog-json` / `--no-catalog-json` | 用离线目录 / 纯自动发现（默认跟随是否已有目录文件） |
| `--codex-home` | 指定 Codex home，便于先在临时目录预演 |

### 1.2 手写配置

```toml
# ~/.codex/config.toml
model = "deepseek-v4-pro"          # 你的默认模型，取目录里的 slug
model_provider = "threerouter"
model_reasoning_effort = "high"    # 可选；档位取决于所选模型

[model_providers.threerouter]
name = "threerouter"
base_url = "https://www.threerouter.com/v1"   # 只写到 /v1
wire_api = "responses"
requires_openai_auth = true        # 用 ChatGPT 登录态；平台 Key 改用 env_key
# env_key = "THREEROUTER_API_KEY"  # 二选一：平台 sk- Key 放环境变量
```

如果还想钉死离线目录（最稳、与服务端发现解耦），再加两行：

```toml
model_catalog_json = "~/.codex/threerouter-models.json"   # ← 必须在顶层
```

下载这份目录文件：

```bash
export KEY=sk-你的平台Key
curl -sS -H "Authorization: Bearer $KEY" -H "Version: 0.162.0" \
  "https://www.threerouter.com/v1/models?client_version=0.162.0" \
  -o ~/.codex/threerouter-models.json
python3 -c "import json;d=json.load(open('$HOME/.codex/threerouter-models.json'));print('模型数:',len(d['models']))"
```

> `curl` 时务必带 `?client_version=`：不带这个参数服务端返回的是 OpenAI 兼容形状
> `{"object":"list","data":[…]}`，**那种形状 Codex 无法解析**。

### 1.3 生效与验证

重启 Codex（CLI 直接重开，桌面端退出后重新打开），模型选择器里应出现目录中的全部模型；
随便选一个发一句话能正常出字即可。

```bash
codex debug models --bundled | head   # 先确认 CLI 可用
codex debug models | jq -r '.models[].slug'   # 注意：该命令读的是官方目录，不能用来验证自定义目录
```

---

## 2. 三个必须知道的坑

### 坑 1：`experimental_bearer_token` 会让 Codex 完全跳过目录发现

实测（清空缓存、同一份目录、唯一变量是凭证行）：provider 段里存在
`experimental_bearer_token` 时，Codex 0.162 **一个目录请求都不发**，选择器里只剩内置的官方模型；
去掉它、或改用 `requires_openai_auth = true` / `env_key`，请求立刻恢复。

这是"升级后选不到模型"最可能的直接诱因。`scripts/codex-setup.py` 会自动把它注释掉。

### 坑 2：`model_catalog_json` 只能写在顶层

写在 `[model_providers.x]` 里会被判为未知配置并**静默忽略**（实测：选择器里自定义模型 0 命中，
且没有任何报错）。写在顶层才会生效。

### 坑 3：目录里的 `slug` 必须是该账号实际可路由的模型名

选定一个不存在的 `slug`，请求会 404/400。平台生成的目录天然满足这一点；
手工编辑目录时不要在 `slug` 上"造名字"。

补充两条同源约束：

- 配了 `model_catalog_json` 后 Codex 不再请求服务端目录（实测 0 请求），这是设计上的"本地优先"；
- 不要照抄 DeepSeek 官方文档里的 `preferred_auth_method = "apikey"` /
  `forced_login_method = "api"`：当 `auth.json` 是 ChatGPT 登录态时，Codex 会直接报
  `API key login is required, but ChatGPT is currently being used. Logging out.` 并中断。

---

## 3. 目录文件契约（`{"models":[…]}`）

顶层**必须**是 `{"models":[…]}`：裸数组、`{"object":"list","data":[…]}` 都会解析失败。
每个条目由平台按 `configuredCodexModelDescriptor` 生成，硬约束如下（任一被破坏，整份目录失效）：

| 约束 | 说明 |
|---|---|
| 字段齐全 | 每个条目 41–42 个键；`context_window` 等数值字段缺失会让**整份**目录失效 |
| `model_messages.instructions_template` | 必须非空，否则系统提示退化、工具行为异常 |
| 数组字段非 null | `supported_reasoning_levels`、`additional_speed_tiers`、`service_tiers`、`experimental_supported_tools`、`input_modalities` |
| 数值字段 > 0 | `context_window`、`max_context_window`、`effective_context_window_percent`、`truncation_policy.limit` |
| `visibility = "list"` | 否则不进选择器 |
| `default_reasoning_level` | 若存在，必须出现在 `supported_reasoning_levels` 里；无档位时用 `"none"` |

**关于「某些模型没有推理档位」**：当同一个别名背后的多个上游账号上报的推理能力互相冲突
（例如一个说支持 reasoning、另一个说不支持），目录会**故意**把该条目的
`default_reasoning_level` 与 `supported_reasoning_levels` 清空，而不是随便补一个 `"none"`
去"看起来完整"——这是 fail-closed：宁可选择器里不出现档位，也不给用户一个上游可能直接拒绝的档位。
代价是这些模型没有档位可选，你在 `config.toml` 写的 `model_reasoning_effort` 对它们不生效。
这是既有实现的既定语义，不是目录损坏；要看某个别名有没有档位，直接查响应：

```bash
curl -sS -H "Authorization: Bearer $KEY" ".../v1/models?client_version=0.162.0" \
  | jq -r '.models[] | "\(.slug)\t\(.supported_reasoning_levels|length)\t\(.default_reasoning_level)"'
```

未知/多余字段客户端会忽略（实测加了未知顶层键与未知条目字段仍正常解析），
所以以后 Codex 加字段不需要我们改；但**不要缺字段**。

`scripts/codex-setup.py` 会在落盘前逐条校验上述规则，任何一条不满足都会报出具体
位置（例如 `models[3] (kimi-k3) context_window 必须 > 0`），避免把坏目录写进用户 home。

---

## 4. 服务端契约（`GET /v1/models`）

两条形状，由「客户端是否自报版本」决定，与分组平台、与版本号取值都无关：

| 请求 | 响应 |
|---|---|
| `GET /v1/models`（无版本信号） | `{"object":"list","data":[…]}`（OpenAI 兼容，不得改） |
| `GET /v1/models?client_version=<任意非空值>` | `{"models":[完整条目…]}` 目录形状 |

版本信号同时接受这些请求头（取第一个非空值，`?client_version=` 优先）：
`Version`、`X-Client-Version`、`X-Codex-Version`。
空串、垃圾值（`abc`）、重复参数一律 `200`，只做观测与上游透传，**绝不按版本号分桶决定形状**
（未来版本号只会继续变大，阈值判断必然被时间淘汰）。

**目录端点不得以 4xx/5xx 作答。** Codex 对目录没有容错：非 200、无法解析的 body、
缺失字段都会让整份目录静默失效，客户端回退内置列表且界面不报错。因此：

- 任何分组（`openai` / `composite` / `kimi` / `zhipu` / `deepseek` / `grok` / `anthropic` …）
  都返回 `200` + 目录形状；没有可用模型时返回 `200 {"models":[]}`；
- OpenAI 分组优先用上游 live 目录（保留上游元数据）；上游不可用时改由本地生成目录兜底，
  而不是把上游 5xx 透给客户端。

响应带 `ETag`；带上相同的 `If-None-Match` 会得到 `304`（无 body）。

---

## 5. 故障自查：选择器里没有我的模型

三种成因的现象完全一样（界面都不报错），按顺序排查：

```bash
export KEY=sk-你的平台Key
# 1) 形状：期望 ["models"]；拿到 ["data","object"] 说明少了 ?client_version=
curl -sS -H "Authorization: Bearer $KEY" "https://www.threerouter.com/v1/models?client_version=0.162.0" | jq 'keys'

# 2) 模型清单与条目完整性
curl -sS -H "Authorization: Bearer $KEY" "https://www.threerouter.com/v1/models?client_version=0.162.0" \
  | jq '{count: (.models|length), slugs: [.models[].slug]}'

# 3) 本地目录文件（若用了 model_catalog_json）
python3 - <<'PY'
import json, os, pathlib
p = pathlib.Path(os.environ.get("CODEX_HOME", pathlib.Path.home()/".codex")) / "threerouter-models.json"
d = json.load(p.open())
bad = [m["slug"] for m in d["models"] if not m.get("model_messages", {}).get("instructions_template")]
print("模型数:", len(d["models"]), "缺 instructions_template:", bad)
PY
```

| 现象 | 成因 | 处理 |
|---|---|---|
| `keys` 是 `["data","object"]` | 请求没带 `?client_version=`，或 `model_catalog_url` 手写了查询串导致参数错位 | 只写干净路径，让客户端自己追加 `?client_version=` |
| 端点 404/500/返回 HTML | 服务端/网关异常 | 服务端问题；目录端点已改为 fail-open，若仍复现请附上 curl 输出 |
| `model_catalog_url` 指向了干净路径 | 见下方注意事项 | 只写 `https://www.threerouter.com/v1/models`（带 `/v1`），不要写平台根路径的 `/models` |
| 条目字段缺失 | 目录被手工改坏 | 用 `scripts/codex-setup.py` 重新生成（会逐条校验） |
| 请求根本没发出（服务端日志 0 请求） | provider 段里有 `experimental_bearer_token` | 删掉/注释该行，或改用 `requires_openai_auth` / `env_key` |
| 只有官方内置模型 | 同上 | 同上 |
| 配了 `model_catalog_json` 仍不生效 | 写在了 provider 段内（被静默忽略） | 移到文件顶层 |

> **路径别写错**：平台的根路径 `/models` 是**公开的模型名单**（无需鉴权、返回裸 ID 数组，
> 供外部展示用），不是 Codex 目录端点。Codex 目录端点是 `/v1/models`，因此
> `base_url` 必须写到 `/v1`（`https://www.threerouter.com/v1`），
> `model_catalog_url` 也必须写 `/v1/models`。指向根路径 `/models` 时你拿到的是
> 裸 ID 数组，Codex 会判定为无效目录。

---

## 6. 两条路线怎么选

| | A. 服务端自动发现（默认） | B. 本地目录文件 |
|---|---|---|
| 用户配置 | `base_url` + 凭证 | 顶层 `model_catalog_json` + 目录文件 |
| 模型来源 | 服务端按分组实时生成 | 本地文件，客户端自己读 |
| 新增上游模型 | 客户端自动更新 | 需要重新下载目录 |
| 与 ChatGPT 登录态共存 | ✅ | ✅（实测 bearer token 不影响本地目录读取） |
| 适用场景 | 日常使用，永远最新 | 离线/内网、想钉死版本、想按分组裁剪 |

两条路互不冲突，且**本地目录优先**：实测配了顶层 `model_catalog_json` 后，Codex 不再请求服务端目录。

---

## 7. 验收清单

接通后按顺序跑一遍：

1. `curl .../v1/models | jq 'keys'` → `["models"]`；
2. `curl .../v1/models | jq '.models|length'` → 等于你在平台可见的模型数；
3. 每个 `slug` 都能在选择器里选到，选定后请求体里的 `model` 与 `slug` 一致；
4. `codex exec "说一句话"` 能出字（证明 `/v1/responses` 与目录无关地正常工作）；
5. 若用了 `model_catalog_json`：目录条目数、`instructions_template` 非空、数组非 null、数值 > 0；
6. `ETag`：同内容两次一致，`If-None-Match` 命中 → `304` 无 body。
