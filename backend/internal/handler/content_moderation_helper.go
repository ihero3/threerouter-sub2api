package handler

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func contentModerationStatus(decision *service.ContentModerationDecision) int {
	if decision == nil || decision.StatusCode < 400 || decision.StatusCode > 599 {
		return http.StatusForbidden
	}
	return decision.StatusCode
}

func contentModerationErrorCode(decision *service.ContentModerationDecision) string {
	return "content_policy_violation"
}

func clientRequestedModel(c *gin.Context, fallback string) string {
	fallback = strings.TrimSpace(fallback)
	if c == nil || c.Request == nil {
		return fallback
	}
	if model, ok := service.RequestedPublicModelFromContext(c.Request.Context()); ok {
		return model
	}
	return fallback
}

func clientRequestedUsageFields(c *gin.Context, mapping service.ChannelMappingResult, fallbackModel, upstreamModel string) service.ChannelUsageFields {
	return mapping.ToUsageFields(clientRequestedModel(c, fallbackModel), upstreamModel)
}

// postModerateOutput 对模型输出（含流式聚合结果）触发异步事后审核（合规方案 0.4）。
// 该调用永不阻断响应，仅记录/计入违规；因此可在响应结束后安全调用。
func (h *GatewayHandler) postModerateOutput(c *gin.Context, apiKey *service.APIKey, subject middleware2.AuthSubject, protocol string, model string, requestBody []byte, output string) {
	if h == nil || h.contentModerationService == nil {
		return
	}
	postModerateOutput(c, h.contentModerationService, apiKey, subject, protocol, model, requestBody, output)
}

func (h *OpenAIGatewayHandler) postModerateOutput(c *gin.Context, apiKey *service.APIKey, subject middleware2.AuthSubject, protocol string, model string, requestBody []byte, output string) {
	if h == nil || h.contentModerationService == nil {
		return
	}
	postModerateOutput(c, h.contentModerationService, apiKey, subject, protocol, model, requestBody, output)
}

// RiskControlEnabled 报告全局风控中心是否启用，供网关中间件用于门控请求内容预览采集。
// nil 安全：handler 或依赖缺失时返回 false（保持原行为，不采集）。
func (h *GatewayHandler) RiskControlEnabled(ctx context.Context) bool {
	if h == nil || h.contentModerationService == nil || ctx == nil {
		return false
	}
	return h.contentModerationService.RiskControlEnabled(ctx)
}

// usageRequestPreview 返回写入 usage_logs 的用户输入前 200 字符预览。
// 仅当全局风控中心启用（risk_control_enabled=true）时才提取；未启用（默认）时返回空串，
// 保持 usage_logs 原行为、不落地任何提示词内容（符合 ZERO_DATA_RETENTION 合规声明）。
func (h *GatewayHandler) usageRequestPreview(c *gin.Context, body []byte) string {
	if h == nil || h.contentModerationService == nil || c == nil || c.Request == nil {
		log.Printf("[DEBUG usage_preview] guard nil: h_nil=%v svc_nil=%v c_nil=%v",
			h == nil, h != nil && h.contentModerationService == nil, c == nil)
		return ""
	}
	enabled := h.contentModerationService.RiskControlEnabled(c.Request.Context())
	if !enabled {
		log.Printf("[DEBUG usage_preview] risk_control_enabled=false (gateway handler)")
		return ""
	}
	preview := service.ExtractRequestPreview(body)
	log.Printf("[DEBUG usage_preview] body=%dB preview=%dB head=%q",
		len(body), len(preview), truncateForLog(preview, 80))
	return preview
}

func truncateForLog(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func postModerateOutput(c *gin.Context, svc *service.ContentModerationService, apiKey *service.APIKey, subject middleware2.AuthSubject, protocol string, model string, requestBody []byte, output string) {
	if svc == nil || c == nil || c.Request == nil || strings.TrimSpace(output) == "" {
		return
	}
	input := buildContentModerationInput(c, apiKey, subject, protocol, model, requestBody)
	svc.CheckOutput(c.Request.Context(), input, output)
}

func runContentModeration(c *gin.Context, reqLog *zap.Logger, svc *service.ContentModerationService, apiKey *service.APIKey, subject middleware2.AuthSubject, protocol string, model string, body []byte) *service.ContentModerationDecision {
	if svc == nil || c == nil || c.Request == nil {
		return nil
	}
	input := buildContentModerationInput(c, apiKey, subject, protocol, model, body)
	if reqLog != nil {
		reqLog.Info("content_moderation.gateway_check_start",
			zap.String("request_id", input.RequestID),
			zap.Int64("user_id", input.UserID),
			zap.Int64("api_key_id", input.APIKeyID),
			zap.String("api_key_name", input.APIKeyName),
			zap.Int64p("group_id", input.GroupID),
			zap.String("group_name", input.GroupName),
			zap.String("endpoint", input.Endpoint),
			zap.String("provider", input.Provider),
			zap.String("protocol", input.Protocol),
			zap.String("model", input.Model),
			zap.Int("body_bytes", len(body)),
		)
	}
	decision, err := svc.Check(c.Request.Context(), input)
	if err != nil {
		if reqLog != nil {
			reqLog.Warn("content_moderation.check_failed", zap.Error(err))
		}
		return nil
	}
	if reqLog != nil && decision != nil {
		reqLog.Info("content_moderation.gateway_check_done",
			zap.String("request_id", input.RequestID),
			zap.Bool("allowed", decision.Allowed),
			zap.Bool("blocked", decision.Blocked),
			zap.Bool("flagged", decision.Flagged),
			zap.String("action", decision.Action),
			zap.Int("status_code", decision.StatusCode),
			zap.String("highest_category", decision.HighestCategory),
			zap.Float64("highest_score", decision.HighestScore),
		)
	}
	return decision
}

func buildContentModerationInput(c *gin.Context, apiKey *service.APIKey, subject middleware2.AuthSubject, protocol string, model string, body []byte) service.ContentModerationCheckInput {
	input := service.ContentModerationCheckInput{
		RequestID: contentModerationRequestID(c.Request.Context()),
		UserID:    subject.UserID,
		Endpoint:  GetInboundEndpoint(c),
		Provider:  contentModerationProvider(apiKey),
		Model:     clientRequestedModel(c, model),
		Protocol:  protocol,
		Body:      body,
	}
	if resolvedPlatform, ok := service.ResolvedTargetPlatformFromContext(c.Request.Context()); ok {
		input.Provider = resolvedPlatform
	}
	if forcedPlatform, ok := middleware2.GetForcePlatformFromContext(c); ok {
		input.Provider = strings.TrimSpace(forcedPlatform)
	}
	if apiKey != nil {
		input.APIKeyID = apiKey.ID
		input.APIKeyName = apiKey.Name
		if apiKey.User != nil {
			input.UserEmail = apiKey.User.Email
		}
		if apiKey.GroupID != nil {
			groupID := *apiKey.GroupID
			input.GroupID = &groupID
		}
		if apiKey.Group != nil {
			input.GroupName = apiKey.Group.Name
		}
	}
	if input.Endpoint == "" && c.Request != nil && c.Request.URL != nil {
		input.Endpoint = c.Request.URL.Path
	}
	return input
}

func contentModerationProvider(apiKey *service.APIKey) string {
	if apiKey == nil || apiKey.Group == nil {
		return ""
	}
	return strings.TrimSpace(apiKey.Group.Platform)
}

func contentModerationRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if requestID, ok := ctx.Value(ctxkey.RequestID).(string); ok {
		return strings.TrimSpace(requestID)
	}
	return ""
}
