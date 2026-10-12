package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// CodexModels serves the Codex models manifest for Codex clients.
//
// Codex CLI and the Codex desktop app refresh their model picker from
// GET {base_url}/models?client_version=... (custom provider mode) or
// GET /backend-api/codex/models (chatgpt_base_url mode). Both routes land
// here when the group platform is OpenAI; other platforms are served by
// GatewayHandler.CodexModels with the same {"models":[…]} envelope.
//
// OpenAI groups prefer the live upstream ChatGPT manifest so real upstream
// metadata is preserved; groups with explicit account model mappings are
// generated locally. Anything this handler cannot serve — an unsupported
// platform, a configured catalog that fails to build, a transient upstream
// failure with no account left to switch to — falls back to the locally
// generated catalog instead of an error status. Codex parses the catalog
// without tolerance, so a 4xx/5xx silently removes every custom model from
// the picker while the UI reports no error at all: a catalog must never be
// answered with a non-2xx status.
func (h *OpenAIGatewayHandler) CodexModels(c *gin.Context) {
	if c.Request.Context().Err() != nil {
		return
	}
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey.Group == nil {
		h.errorResponse(c, http.StatusUnauthorized, "invalid_request_error", "API key group is required")
		return
	}
	if apiKey.Group.Platform != service.PlatformOpenAI && apiKey.Group.Platform != service.PlatformComposite {
		// 该分组不由 OpenAI 分支提供目录：交回本地生成目录（同形状、同 200）。
		// 目录端点不得以 4xx/5xx 作答——Codex 对目录没有容错，会静默丢弃整份
		// 目录并回退内置模型列表，用户看到的是「自定义模型整体消失」而界面
		// 不报错。
		signalCodexModelsLocalFallback(c)
		return
	}

	ifNoneMatch := c.GetHeader("If-None-Match")
	// 版本信号必须与路由层的判定同源：路由层用「client_version 查询参数或 Version 等
	// 请求头」决定是否按目录形状响应，这里却只读查询参数，就会出现「客户端只发
	// Version 头 → 路由按目录形状响应、但上游请求丢了版本」的不一致（上游可能因此
	// 返回陈旧/不匹配的清单）。取一次，全流程复用。
	clientVersion := service.CodexClientVersionFromRequest(c.Request.URL.Query(), c.Request.Header)

	configuredManifest, configured, err := h.gatewayService.BuildGroupConfiguredCodexModelsManifest(
		c.Request.Context(),
		apiKey.Group,
		ifNoneMatch,
	)
	if err != nil {
		if c.Request.Context().Err() != nil {
			return
		}
		// 分组已显式配置模型但本地构建失败：仍要给客户端一份可用目录，
		// 交由路由层的本地生成目录兜底。
		signalCodexModelsLocalFallback(c)
		return
	}
	if configured {
		writeCodexModelsManifestResponse(c, configuredManifest)
		recordCodexCatalogDelivery(c, service.CodexCatalogSourceConfigured, configuredManifest)
		return
	}

	maxAccountSwitches := h.maxAccountSwitches
	if maxAccountSwitches <= 0 {
		maxAccountSwitches = 3
	}
	failedAccountIDs := make(map[int64]struct{})
	switchCount := 0
	var lastUpstreamErr error

	for {
		account, err := h.gatewayService.SelectAccountForModelWithExclusions(c.Request.Context(), apiKey.GroupID, "", "", failedAccountIDs)
		if err != nil {
			if c.Request.Context().Err() != nil {
				return
			}
			if lastUpstreamErr != nil {
				// 所有可切换账号都试过了：回退本地生成目录，避免 5xx 让整份
				// 目录在客户端静默失效。
				signalCodexModelsLocalFallback(c)
				return
			}
			// 从来没有可用账号：这在 Codex 眼里不是"上游报错"，而是"你的目录没了"。
			// 目录端点对客户端没有容错，503 会让整份自定义目录被静默丢弃并回退内置
			// 模型列表，界面不报错。因此这里同样 fail-open 到本地生成目录（分组配置的
			// 模型 + 默认集合），把"账号不可用"留在 ops 日志里可见。
			markCodexCatalogUpstreamUnavailable(c, apiKey.GroupID)
			signalCodexModelsLocalFallback(c)
			return
		}
		// 让 ops 错误日志携带实际选中的上游账号，便于定位失效账号（#4544）。
		setOpsSelectedAccount(c, account.ID, account.Name, account.Platform)

		// The client ETag represents the final group-specific body, so fetch the
		// source manifest before applying local filtering and alias metadata.
		manifest, err := h.gatewayService.FetchCodexModelsManifest(c.Request.Context(), account, clientVersion, "")
		if err != nil {
			if c.Request.Context().Err() != nil {
				return
			}
			if service.IsRetryableCodexModelsManifestError(err) && switchCount < maxAccountSwitches {
				failedAccountIDs[account.ID] = struct{}{}
				switchCount++
				lastUpstreamErr = err
				continue
			}
			// 上游目录不可用且无法再换号：同样 fail-open 到本地生成目录。
			signalCodexModelsLocalFallback(c)
			return
		}
		if err := h.gatewayService.CompleteAPIKeyCodexModelsManifestForClient(manifest, account); err != nil {
			signalCodexModelsLocalFallback(c)
			return
		}
		if err := h.gatewayService.MergeGroupConfiguredCodexModels(c.Request.Context(), apiKey.Group, manifest, ifNoneMatch); err != nil {
			signalCodexModelsLocalFallback(c)
			return
		}
		if c.Request.Context().Err() != nil {
			return
		}

		writeCodexModelsManifestResponse(c, manifest)
		recordCodexCatalogDelivery(c, service.CodexCatalogSourceLive, manifest)
		return
	}
}

// recordCodexCatalogDelivery 上报 OpenAI 分支的目录交付（低基数指标）。
//
// 只统计真正写出的响应：304 复用既有 ETag 不是新交付，空 body 也不是。
// 平台取自分组，条目数直接数 body，因此与客户端实际拿到的内容一致。
func recordCodexCatalogDelivery(c *gin.Context, source string, manifest *service.CodexModelsManifest) {
	if manifest == nil || manifest.NotModified || len(manifest.Body) == 0 {
		return
	}
	platform := ""
	if apiKey, ok := middleware2.GetAPIKeyFromContext(c); ok && apiKey != nil && apiKey.Group != nil {
		platform = apiKey.Group.Platform
	}
	var envelope struct {
		Models []json.RawMessage `json:"models"`
	}
	entries := -1
	if err := json.Unmarshal(manifest.Body, &envelope); err == nil {
		entries = len(envelope.Models)
	}
	service.RecordCodexCatalogServed(service.CodexCatalogShapeModels, source, platform, entries)
}

// markCodexCatalogUpstreamUnavailable 把「上游目录不可用」标记进 ops 上下文。
//
// 目录端点现在一律 fail-open，因此"拿不到上游清单"不再以状态码表达，必须靠这层标记
// 才能在 ops 侧看见。用既有的分组业务受限原因，避免为了埋点新增枚举。
func markCodexCatalogUpstreamUnavailable(c *gin.Context, groupID *int64) {
	if c == nil {
		return
	}
	service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalModelConfiguration)
	if groupID != nil {
		c.Set("sub2api.codex_models.group_id", *groupID)
	}
}

// signalCodexModelsLocalFallback 请求路由层改由本地生成目录提供本次响应。
//
// 只置位、不写响应体：这样 OpenAI 分支可以保留「先试配置目录、再试上游」的既有
// 顺序，而由路由层决定是否继续，handler 无需反向依赖 routes 包。
func signalCodexModelsLocalFallback(c *gin.Context) {
	if c == nil {
		return
	}
	c.Set(service.CodexModelsLocalFallbackContextKey, true)
}

func writeCodexModelsManifestResponse(c *gin.Context, manifest *service.CodexModelsManifest) {
	if manifest.ETag != "" {
		c.Header("ETag", manifest.ETag)
	}
	if manifest.NotModified {
		c.Status(http.StatusNotModified)
		c.Writer.WriteHeaderNow()
		return
	}
	c.Data(http.StatusOK, "application/json", manifest.Body)
}
