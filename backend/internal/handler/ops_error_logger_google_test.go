package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// recordGoogleFormatOpsErrorWithOps 让 handle 以 Google 错误格式回写错误，
// 返回落库条目与入队数量，便于验证「被过滤时不落库」的场景。
func recordGoogleFormatOpsErrorWithOps(t *testing.T, handle gin.HandlerFunc, ops *service.OpsService) (*service.OpsInsertErrorLogInput, int64) {
	t.Helper()
	setupOpsErrorLogTestQueue(t, 1)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(OpsErrorLoggerMiddleware(ops))
	router.POST("/v1beta/cachedContents", handle)

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/v1beta/cachedContents", nil))

	queued := OpsErrorLogQueueLength()
	if queued == 0 {
		return nil, queued
	}
	return (<-opsErrorLogQueue).entry, queued
}

// recordGoogleFormatOpsError 让 handle 以 Google 错误格式回写错误，返回 ops 中间件记录的错误日志。
func recordGoogleFormatOpsError(t *testing.T, handle gin.HandlerFunc) *service.OpsInsertErrorLogInput {
	t.Helper()
	ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	entry, queued := recordGoogleFormatOpsErrorWithOps(t, handle, ops)
	require.Equal(t, int64(1), queued)
	return entry
}

func TestOpsErrorLoggerMiddleware_GoogleClientErrorsClassifiedAsClient(t *testing.T) {
	for _, tc := range []struct {
		status  int
		errType string
		phase   string
	}{
		{status: http.StatusBadRequest, errType: "invalid_request_error", phase: "request"},
		{status: http.StatusRequestEntityTooLarge, errType: "invalid_request_error", phase: "request"},
		{status: http.StatusUnauthorized, errType: "authentication_error", phase: "auth"},
		{status: http.StatusForbidden, errType: "permission_error", phase: "request"},
		{status: http.StatusNotFound, errType: "not_found_error", phase: "request"},
	} {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			entry := recordGoogleFormatOpsError(t, func(c *gin.Context) {
				googleError(c, tc.status, "client request problem")
			})
			require.Equal(t, tc.status, entry.StatusCode)
			require.Equal(t, tc.errType, entry.ErrorType)
			require.Equal(t, tc.phase, entry.ErrorPhase)
			require.Equal(t, "client", entry.ErrorOwner)
			require.Equal(t, "client_request", entry.ErrorSource)
			require.Equal(t, "P3", entry.Severity)
		})
	}
}

func TestOpsErrorLoggerMiddleware_GooglePlatformErrorStaysPlatform(t *testing.T) {
	entry := recordGoogleFormatOpsError(t, func(c *gin.Context) {
		googlePlatformError(c, http.StatusBadRequest, "Explicit context caching is not available for model m: storage pricing is not configured")
	})
	require.Equal(t, "api_error", entry.ErrorType)
	require.Equal(t, "internal", entry.ErrorPhase)
	require.Equal(t, "platform", entry.ErrorOwner)
	require.Equal(t, "gateway", entry.ErrorSource)
}

func TestOpsErrorLoggerMiddleware_GoogleServerErrorsNotTypedByStatus(t *testing.T) {
	entry := recordGoogleFormatOpsError(t, func(c *gin.Context) {
		googleError(c, http.StatusInternalServerError, "Failed to persist cached content")
	})
	require.Equal(t, "api_error", entry.ErrorType)
	require.Equal(t, "platform", entry.ErrorOwner)
}

func TestOpsErrorLoggerMiddleware_GoogleBillingErrorMatchesClaudeFormat(t *testing.T) {
	// 余额不足错误默认被「忽略 Insufficient 错误」开关过滤，不写错误日志；
	// 分类口径改在开关关闭时验证，保证 Google 格式仍与 Claude 格式一致。
	handle := func(c *gin.Context) {
		status, code, message, _ := billingErrorDetails(service.ErrInsufficientBalance)
		googleErrorWithType(c, status, code, "", message)
	}

	t.Run("filtered by default", func(t *testing.T) {
		ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		_, queued := recordGoogleFormatOpsErrorWithOps(t, handle, ops)
		require.Zero(t, queued, "默认开启过滤时余额不足错误不写错误日志")
	})

	t.Run("classification when filter disabled", func(t *testing.T) {
		// 必须带 schema_version=2，否则读取路径会把存量配置迁移成默认开启。
		ops := service.NewOpsService(nil, &opsAdvancedSettingsRepoStub{
			advanced: `{"ignore_insufficient_balance_errors":false,"schema_version":2}`,
		}, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		require.False(t, ops.OpsAdvancedSettingsSnapshot().IgnoreInsufficientBalanceErrors)
		entry, queued := recordGoogleFormatOpsErrorWithOps(t, handle, ops)
		require.Equal(t, int64(1), queued)
		require.Equal(t, "billing_error", entry.ErrorType)
		require.Equal(t, "request", entry.ErrorPhase)
		require.Equal(t, "client", entry.ErrorOwner)
		require.True(t, entry.IsBusinessLimited)
	})
}

func TestOpsErrorLoggerMiddleware_GoogleConcurrencyErrorClassifiedAsRequest(t *testing.T) {
	entry := recordGoogleFormatOpsError(t, func(c *gin.Context) {
		googleConcurrencyError(c, &ConcurrencyError{SlotType: "user"}, "user")
	})
	require.Equal(t, http.StatusTooManyRequests, entry.StatusCode)
	require.Equal(t, "rate_limit_error", entry.ErrorType)
	require.Equal(t, "request", entry.ErrorPhase)
	require.Equal(t, "client", entry.ErrorOwner)
}

func TestOpsErrorLoggerMiddleware_GoogleErrorAfterUpstreamFailureKeepsUpstreamAttribution(t *testing.T) {
	entry := recordGoogleFormatOpsError(t, func(c *gin.Context) {
		service.SetOpsUpstreamError(c, http.StatusBadRequest, "upstream rejected", "")
		googleError(c, http.StatusBadRequest, "upstream rejected")
	})
	require.Equal(t, "upstream", entry.ErrorPhase)
	require.Equal(t, "provider", entry.ErrorOwner)
	require.Equal(t, "upstream_http", entry.ErrorSource)
}

func TestOpsErrorLoggerMiddleware_RequestScopedUpstreamErrorClassifiedAsClient(t *testing.T) {
	entry := recordGoogleFormatOpsError(t, func(c *gin.Context) {
		service.SetOpsUpstreamError(c, http.StatusBadRequest, "Invalid resource state for cache content 8079767076522164224.", "")
		c.Set(service.OpsUpstreamErrorsKey, []*service.OpsUpstreamErrorEvent{{
			Platform: service.PlatformGemini, AccountID: 173, UpstreamStatusCode: http.StatusBadRequest, Kind: "http_error",
			Message: "Invalid resource state for cache content 8079767076522164224.",
		}})
		service.MarkOpsRequestScopedError(c, "permission_error")
		c.Data(http.StatusForbidden, "application/json", []byte(service.GeminiCachedContentNotFoundResponse))
	})
	require.Equal(t, http.StatusForbidden, entry.StatusCode)
	require.Equal(t, "permission_error", entry.ErrorType)
	require.Equal(t, "request", entry.ErrorPhase)
	require.Equal(t, "client", entry.ErrorOwner)
	require.Equal(t, "client_request", entry.ErrorSource)
	require.False(t, entry.IsBusinessLimited)
	require.NotNil(t, entry.UpstreamStatusCode, "上游原始错误保留为明细")
	require.Equal(t, http.StatusBadRequest, *entry.UpstreamStatusCode)
}

func TestSetOpsLocalErrorTypeClearsRequestScopedMark(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	service.MarkOpsRequestScopedError(c, "permission_error")
	require.True(t, service.IsOpsRequestScopedError(c))

	service.SetOpsLocalErrorType(c, "", "")
	require.False(t, service.IsOpsRequestScopedError(c))
	errType, code := service.OpsLocalErrorType(c)
	require.Empty(t, errType)
	require.Empty(t, code)
}
