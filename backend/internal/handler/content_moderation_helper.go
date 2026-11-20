package handler

import (
	"context"
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

// UsageRequestPreviewEnabled 报告「请求内容预览」独立开关是否开启。
// 供 OpsErrorLoggerMiddleware 门控 ops_error_logs.request_preview 采集；
// 与风控中心（risk_control_enabled）解耦，nil 安全，默认关闭（ZDR）。
func (h *GatewayHandler) UsageRequestPreviewEnabled(ctx context.Context) bool {
	if h == nil || h.settingService == nil || ctx == nil {
		return false
	}
	return h.settingService.IsUsageRequestPreviewEnabled(ctx)
}

// usageRequestPreview 根据独立开关决定是否提取用户输入前 200 字符预览。
// enabled 由各 Handler 注入开关探测函数，保证两套网关（GatewayHandler /
// OpenAIGatewayHandler）写入 usage_logs.request_preview 的口径完全一致。
func usageRequestPreview(c *gin.Context, body []byte, enabled func(context.Context) bool) string {
	if c == nil || c.Request == nil || enabled == nil || !enabled(c.Request.Context()) {
		return ""
	}
	return service.ExtractRequestPreview(body)
}

// usageRequestPreview 返回写入 usage_logs 的用户输入前 200 字符预览（GatewayHandler 路径）。
// 由独立开关 usage_request_preview_enabled 控制，与风控中心（risk_control_enabled）解耦：
// 默认关闭，不落地任何提示词内容（ZERO_DATA_RETENTION）；管理员可单独开启预览采集。
func (h *GatewayHandler) usageRequestPreview(c *gin.Context, body []byte) string {
	if h == nil || h.settingService == nil {
		return ""
	}
	return usageRequestPreview(c, body, h.settingService.IsUsageRequestPreviewEnabled)
}

// usageRequestPreview 返回写入 usage_logs 的用户输入前 200 字符预览（OpenAIGatewayHandler 路径）。
// 该 Handler 未直接持有 SettingService，经 gatewayService 转发读取同一个独立开关。
func (h *OpenAIGatewayHandler) usageRequestPreview(c *gin.Context, body []byte) string {
	if h == nil || h.gatewayService == nil {
		return ""
	}
	return usageRequestPreview(c, body, h.gatewayService.UsageRequestPreviewEnabled)
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
