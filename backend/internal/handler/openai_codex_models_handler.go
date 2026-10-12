package handler

import (
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
			// 从来没有可用账号属于分组配置问题，保持既有 503 语义（运维可见）。
			h.errorResponse(c, http.StatusServiceUnavailable, "upstream_error", "No available OpenAI accounts")
			return
		}
		// 让 ops 错误日志携带实际选中的上游账号，便于定位失效账号（#4544）。
		setOpsSelectedAccount(c, account.ID, account.Name, account.Platform)

		// The client ETag represents the final group-specific body, so fetch the
		// source manifest before applying local filtering and alias metadata.
		manifest, err := h.gatewayService.FetchCodexModelsManifest(c.Request.Context(), account, c.Query("client_version"), "")
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
		return
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
