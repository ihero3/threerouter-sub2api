# ThreeRouter 博客发布 API 使用指南

> 端点定位：面向**自动化脚本 / 第三方系统**的博客发布通道，路径沿用 OpenAI 兼容风格的 `/v1` 前缀。
> 与后台管理 API（`/api/v1/admin/blogs`）不同，本接口使用**用户级 `sk-` API Key** 认证，且**不参与计费**（不受余额 / 订阅 / 额度限制）。

## 快速开始

**Base URL**：`https://www.threerouter.com/v1`（本地开发 `http://localhost:8080/v1`，或你的部署地址 `/v1`）

**认证**：在后台「API 密钥」页创建密钥，请求头携带：

```bash
Authorization: Bearer sk-xxxxxxxx
```

（也支持 `x-api-key: sk-xxx`）

> **权限**：仅 **管理员（admin）用户** 的 API Key 允许创建 / 更新博客；所有已鉴权调用方均可读取。普通用户的 Key 调用发布接口会返回 `403`。

## 端点一览

| 方法 | 路径 | 权限 | 用途 |
|---|---|---|---|
| POST | `/v1/blog/posts` | 管理员 | 创建博客（**默认直接发布**，可传 `status=draft` 存草稿） |
| GET  | `/v1/blog/posts` | 已鉴权 | 列出已发布博客（按发布时间倒序） |
| GET  | `/v1/blog/posts/:id` | 已鉴权 | 获取单篇博客详情 |
| POST | `/v1/blog/uploads/image` | 管理员 | 上传博客配图（multipart），返回公开图片 URL |

> 注：读取接口位于受密钥保护的 `/v1/blog` 路径下（如需完全公开读取，仍可使用既有的 `/api/v1/public/blogs`）。

## 正文格式（重要）

`content` 需为 **HTML**。前端详情页（`BlogDetailView.vue`）仅当内容包含 HTML 标签
（`<p> / <div> / <h1..6> / <ul> / <ol> / <img> / <blockquote> / <pre> / <table>`）时才按富文本渲染；
否则会作为**纯文本原样输出**（Markdown 语法会直接暴露在页面上）。

- 因此请勿直接提交 Markdown 源码；应先转换为 HTML（脚本 `scripts/publish_blog.py` 的 `md_to_html()` 可完成转换）。
- 推荐正文结构（与前端模板「标题 → 元信息 → 封面图 → 正文」对齐）：
  1. 第一段 `<p>`：图片图注（纯文本，紧贴封面图下方显示）
  2. 若干 `<p>`：正文段落
  3. 末段 `<p>Sources: ...</p>`
- 配图放在 `cover_image`（列表缩略图 / 详情页 hero / SEO og:image 均使用它）；如需正文内嵌图，用
  `<figure><img src="..." alt="..." /><figcaption>图注</figcaption></figure>`。
- 前端渲染前会用 DOMPurify 净化（允许 `img` 及 `src/alt/style/width/height` 等属性）。

## 创建博客（POST /v1/blog/posts）

**请求体**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `title` | string | 是 | 标题（≤500 字符） |
| `content` | string | 是 | 正文，须为 **HTML**（详见上文「正文格式」） |
| `summary` | string | 否 | 摘要（≤1000 字符） |
| `cover_image` | string | 否 | 封面图 URL（≤1000 字符） |
| `tags` | string | 否 | 标签，逗号分隔，如 `"ai,news,daily"` |
| `status` | string | 否 | `published`（默认，直接发布）/ `draft`（草稿） |

**示例**

```bash
curl https://www.threerouter.com/v1/blog/posts \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "AI 日报：Anthropic 冲刺最大 IPO",
    "content": "# 今日要点\n\n- Anthropic Q3 营收 168 亿美元……",
    "summary": "AI 行业每日热点速览",
    "tags": "ai,news,daily",
    "status": "published"
  }'
```

**响应**（与后台统一结构，`code=0` 表示成功）

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 3,
    "title": "AI 日报：Anthropic 冲刺最大 IPO",
    "content": "# 今日要点……",
    "summary": "AI 行业每日热点速览",
    "cover_image": "",
    "status": "published",
    "tags": "ai,news,daily",
    "published_at": "2026-10-15T09:17:50+08:00",
    "created_by": 1,
    "created_at": "2026-10-15T09:17:50+08:00",
    "updated_at": "2026-10-15T09:17:50+08:00"
  }
}
```

## 上传配图（POST /v1/blog/uploads/image）

使用与发布相同的 `sk-` API Key（需**管理员**角色），`multipart/form-data` 上传，字段名 `file`。

```bash
curl -X POST https://www.threerouter.com/v1/blog/uploads/image \
  -H "Authorization: Bearer sk-xxx" \
  -F "file=@./infographic.png"
```

**响应**

```json
{ "code": 0, "message": "success", "data": { "url": "/api/v1/public/uploads/blogs/6fc26413b336074b61b6defbf93b36dd.png" } }
```

- 返回的 `url` 为**相对路径**，可直接用于 `cover_image` 或正文 `<img src="...">`；该地址**公开、免认证**，浏览器可直接加载。
- 限制：仅 `jpeg/png/gif/webp`，≤10MB（按文件内容嗅探类型，不信任扩展名）。
- 文件落盘于 `{data_dir}/uploads/blogs/`（`pricing.data_dir` 默认 `./data`），随机文件名、不可变。

> 另有后台管理端点 `POST /api/v1/admin/uploads/image`（要求 `settings.admin_api_key` 或管理员 JWT）；
> 本 `/v1/blog/uploads/image` 端点面向自动化脚本，复用用户级 `sk-` Key 即可，无需额外配置 Admin API Key。

## 读取博客

```bash
# 列表（分页：page、page_size）
curl https://www.threerouter.com/v1/blog/posts?page=1&page_size=10 \
  -H "Authorization: Bearer sk-xxx"

# 详情
curl https://www.threerouter.com/v1/blog/posts/3 \
  -H "Authorization: Bearer sk-xxx"
```

## 常见错误

| 状态码 | 含义 |
|---|---|
| 401 | 未传 / 无效 API Key（`API_KEY_REQUIRED` / `INVALID_API_KEY`） |
| 403 | 当前 Key 所属用户非管理员（`Only admin users can publish blogs via the API`） |
| 400 | 参数错误（标题为空、JSON 非法等） |
| 429 | 触发密钥级限流（发布接口本身不计费，但仍受全局 RPM 限制） |

## 与后台管理 API 的区别

| 维度 | `/v1/blog/posts`（本接口） | `/api/v1/admin/blogs` |
|---|---|---|
| 认证 | 用户级 `sk-` Key | 管理员会话 / Admin API Key |
| 计费 | 豁免（不计费、不限额度） | 不适用（管理面） |
| 权限 | 仅管理员用户 Key | 管理员 |
| 路径风格 | OpenAI 兼容 `/v1` | 后台 REST `/api/v1/admin` |
| 适用场景 | 自动化 / 外部系统推送 | 人工后台管理 |

## 自动化发布示例

参见仓库 `scripts/publish_blog.py`：读取本地 Markdown / HTML 简报，**自动转 HTML + 上传配图 + 发布**，可直接接入每日 AI 新闻推送流程。

```bash
python scripts/publish_blog.py \
  --base-url https://www.threerouter.com \
  --api-key  sk-xxxx \
  --file     ./news1.md \
  --tags     "AI,Funding" \
  --images   "./infographic.png:DeepSeek funding round infographic" \
  --summary  'DeepSeek latest round nears $12B led by Tencent and CATL.'
```

`--images` 第一张图会作为封面（`cover_image`），其 `caption` 会自动作为正文首段（即图片下方图注）。
