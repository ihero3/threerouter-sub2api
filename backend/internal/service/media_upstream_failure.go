package service

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// media_upstream_failure.go — 统一媒体上游错误分类。
// 目标是创建媒体任务时先解析 HTTP 状态和响应体，再决定是否切换账号；
// 参数类错误不切换，配额/认证/服务故障自动切换并写入账号冷却状态。

type MediaUpstreamFailureClass string

const (
	MediaFailureNone          MediaUpstreamFailureClass = ""
	MediaFailureRateLimit     MediaUpstreamFailureClass = "rate_limit"
	MediaFailureBillingQuota  MediaUpstreamFailureClass = "billing_quota"
	MediaFailureAuth          MediaUpstreamFailureClass = "auth_error"
	MediaFailureServer        MediaUpstreamFailureClass = "server_error"
	MediaFailureValidation    MediaUpstreamFailureClass = "validation_error"
	MediaFailureContentPolicy MediaUpstreamFailureClass = "content_policy"
	MediaFailureTransport     MediaUpstreamFailureClass = "transport_error"
	MediaFailureUnknown       MediaUpstreamFailureClass = "unknown"
)

type MediaUpstreamFailureDecision struct {
	Class          MediaUpstreamFailureClass
	ShouldFailover bool
	ShouldCooldown bool
	Cooldown       time.Duration
}

var mediaBillingFailureMarkers = []string{
	"insufficient balance",
	"insufficient_quota",
	"quota exceeded",
	"exceeded your current quota",
	"billing",
	"payment required",
	// DashScope 免费额度用尽：HTTP 403 + {"code":"AllocationQuota.FreeTierOnly"}。
	// 这是额度问题而非鉴权问题——按 auth 归类会把一个完全可用的账号冷却 10 分钟，
	// 且错误面板会把「该充值/关开关」误导成「密钥失效」。
	"free quota exhausted",
	"quota exhausted",
	"exceeded your free quota",
	"free tier only",
	"freetieronly",
	"allocationquota",
	"欠费",
	"余额不足",
	"额度不足",
	"配额不足",
	"免费额度",
}

var mediaContentPolicyMarkers = []string{
	"content policy",
	"content_policy",
	"policy violation",
	"safety system",
	"violates our",
	"敏感内容",
	"违规内容",
	"内容审核",
}

var mediaAuthFailureMarkers = []string{
	"invalid api key",
	"invalid_api_key",
	"api key expired",
	"unauthorized",
	"forbidden",
	"authentication",
	"api key 无效",
	"鉴权失败",
	"无权限",
}

var mediaRateLimitMarkers = []string{
	"rate limit",
	"rate_limit",
	"too many requests",
	"throttl",
	"请求过于频繁",
	"触发限流",
}

var mediaValidationMarkers = []string{
	"invalid request",
	"invalid_request",
	"invalid parameter",
	"validation",
	"unsupported",
	"参数错误",
	"请求参数",
	"不支持",
}

// classifyMediaUpstreamFailure 使用 body-first 分类，兼容各厂商用 400/403
// 包装余额、限流或内容审核错误的场景。
func classifyMediaUpstreamFailure(statusCode int, responseBody []byte) MediaUpstreamFailureDecision {
	body := strings.ToLower(strings.TrimSpace(string(responseBody)))
	body = strings.ReplaceAll(body, "_", " ")

	if containsAnyMarker(body, mediaContentPolicyMarkers) {
		return MediaUpstreamFailureDecision{Class: MediaFailureContentPolicy}
	}
	if containsAnyMarker(body, mediaBillingFailureMarkers) {
		return MediaUpstreamFailureDecision{
			Class:          MediaFailureBillingQuota,
			ShouldFailover: true,
			ShouldCooldown: true,
			Cooldown:       30 * time.Minute,
		}
	}
	if containsAnyMarker(body, mediaAuthFailureMarkers) {
		return MediaUpstreamFailureDecision{
			Class:          MediaFailureAuth,
			ShouldFailover: true,
			ShouldCooldown: true,
			Cooldown:       10 * time.Minute,
		}
	}
	if containsAnyMarker(body, mediaRateLimitMarkers) || statusCode == http.StatusTooManyRequests || statusCode == 529 {
		return MediaUpstreamFailureDecision{
			Class:          MediaFailureRateLimit,
			ShouldFailover: true,
			ShouldCooldown: true,
			Cooldown:       time.Minute,
		}
	}
	if containsAnyMarker(body, mediaValidationMarkers) || statusCode == http.StatusBadRequest || statusCode == http.StatusUnprocessableEntity || statusCode == http.StatusConflict {
		return MediaUpstreamFailureDecision{Class: MediaFailureValidation}
	}

	switch {
	case statusCode == http.StatusUnauthorized, statusCode == http.StatusForbidden, statusCode == http.StatusPaymentRequired:
		return MediaUpstreamFailureDecision{
			Class:          MediaFailureAuth,
			ShouldFailover: true,
			ShouldCooldown: true,
			Cooldown:       10 * time.Minute,
		}
	case statusCode == http.StatusNotFound, statusCode == http.StatusRequestTimeout, statusCode == http.StatusServiceUnavailable, statusCode >= 500:
		return MediaUpstreamFailureDecision{
			Class:          MediaFailureServer,
			ShouldFailover: true,
			ShouldCooldown: statusCode >= 500,
			Cooldown:       time.Minute,
		}
	case statusCode == 0:
		return MediaUpstreamFailureDecision{
			Class:          MediaFailureUnknown,
			ShouldFailover: true,
		}
	default:
		return MediaUpstreamFailureDecision{Class: MediaFailureUnknown}
	}
}

func containsAnyMarker(text string, markers []string) bool {
	if text == "" {
		return false
	}
	for _, marker := range markers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

// recordMediaUpstreamFailure 将分类结果写入账号调度状态。认证和余额错误优先走
// RateLimitService 的平台规则；其余媒体错误使用轻量临时冷却，避免污染通用平台逻辑。
func (s *MediaTaskService) recordMediaUpstreamFailure(c *gin.Context, ctx context.Context, account *Account, statusCode int, responseBody []byte, requestedModel string) {
	decision := classifyMediaUpstreamFailure(statusCode, responseBody)
	if account != nil {
		message := strings.TrimSpace(sanitizeUpstreamErrorMessage(string(responseBody)))
		if message == "" && statusCode != 0 {
			message = strings.TrimSpace(sanitizeUpstreamErrorMessage(string(decision.Class)))
		}
		safeBody := string(truncateForLog([]byte(message), 1024))
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:             account.Platform,
			AccountID:            account.ID,
			AccountName:          account.Name,
			UpstreamStatusCode:   statusCode,
			UpstreamResponseBody: safeBody,
			Kind:                 "http_error",
			Stage:                "media_upstream",
			Reason:               string(decision.Class),
			Message:              message,
		})
	}
	if !decision.ShouldCooldown || account == nil {
		return
	}
	if s.rateLimitService != nil && (decision.Class == MediaFailureAuth || decision.Class == MediaFailureBillingQuota) {
		_ = s.rateLimitService.HandleUpstreamError(ctx, account, statusCode, nil, responseBody, requestedModel)
		return
	}
	if s.rateLimitService != nil {
		until := time.Now().Add(decision.Cooldown)
		if s.rateLimitService.SetMediaTempUnschedulable(ctx, account.ID, until, "media upstream failure: "+string(decision.Class)) {
			return
		}
	}
	if s.accountService == nil {
		return
	}
	until := time.Now().Add(decision.Cooldown)
	_ = s.accountService.SetTempUnschedulable(ctx, account.ID, until, "media upstream failure: "+string(decision.Class))
}

// recordMediaTransportFailure 持久网络故障停调账号；瞬时网络故障只切换账号。
func (s *MediaTaskService) recordMediaTransportFailure(c *gin.Context, ctx context.Context, account *Account, createErr error) {
	if account == nil || createErr == nil {
		return
	}
	safeErr := strings.TrimSpace(sanitizeUpstreamErrorMessage(createErr.Error()))
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		Platform:           account.Platform,
		AccountID:          account.ID,
		AccountName:        account.Name,
		UpstreamStatusCode: 0,
		Kind:               "request_error",
		Stage:              "media_upstream",
		Reason:             "transport_error",
		Message:            safeErr,
	})
	if !classifyUpstreamTransportError(createErr).Persistent {
		return
	}
	until := time.Now().Add(10 * time.Minute)
	if s.rateLimitService == nil || !s.rateLimitService.SetMediaTempUnschedulable(ctx, account.ID, until, "media upstream transport error: "+createErr.Error()) {
		if s.accountService != nil {
			_ = s.accountService.SetTempUnschedulable(ctx, account.ID, until, "media upstream transport error: "+createErr.Error())
		}
	}
}
