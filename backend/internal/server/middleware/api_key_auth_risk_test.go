package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// riskGateAPIKeyRepo 只实现网关鉴权路径实际会调用的两个方法；其余方法由
// 嵌入的 service.APIKeyRepository 接口（nil）提供，测试中不会被触发。
// 独立命名，避免与同包 unit 标签测试里的其它桩类型重名。
type riskGateAPIKeyRepo struct {
	service.APIKeyRepository
	key *service.APIKey
}

func (r *riskGateAPIKeyRepo) GetByKeyForAuth(_ context.Context, _ string) (*service.APIKey, error) {
	if r.key == nil {
		return nil, service.ErrAPIKeyNotFound
	}
	clone := *r.key
	return &clone, nil
}

// Simple 模式放行正常请求前会尽力更新 last_used_at，给个 no-op 即可。
func (r *riskGateAPIKeyRepo) UpdateLastUsed(_ context.Context, _ int64, _ time.Time) error {
	return nil
}

func newRiskGateService(riskLevel string) *service.APIKeyService {
	user := &service.User{
		ID:          7,
		Status:      service.StatusActive,
		Role:        service.RoleUser,
		Balance:     10,
		Concurrency: 3,
		RiskLevel:   riskLevel,
	}
	repo := &riskGateAPIKeyRepo{key: &service.APIKey{
		ID:     100,
		UserID: 7,
		Key:    "risk-gate-test-key",
		Status: service.StatusActive,
		User:   user,
	}}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	return service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, cfg)
}

// TestAPIKeyAuthRiskGate 验证标准网关在 IsActive() 之后对 suspect / abnormal
// 返回 403 + USER_RISK_REVIEW_REQUIRED，并打上 IngressRejectUserRiskReview；
// 对正常 / 未知等级用户不误拦（放行到 handler）。
func TestAPIKeyAuthRiskGate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{RunMode: config.RunModeSimple}
	cases := []struct {
		name       string
		riskLevel  string
		wantStatus int
	}{
		{name: "suspect is blocked", riskLevel: service.AffiliateRiskLevelSuspect, wantStatus: http.StatusForbidden},
		{name: "abnormal is blocked", riskLevel: service.AffiliateRiskLevelAbnormal, wantStatus: http.StatusForbidden},
		{name: "normal is allowed", riskLevel: "", wantStatus: http.StatusOK},
		{name: "unknown level is allowed", riskLevel: "future_level", wantStatus: http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var captured IngressRejectReason
			svc := newRiskGateService(tc.riskLevel)
			r := gin.New()
			// 注册在鉴权中间件之前，Abort 之后仍能读回 MarkIngressRejected 的 reason。
			r.Use(func(c *gin.Context) {
				c.Next()
				if reason, ok := GetIngressRejectReason(c); ok {
					captured = reason
				}
			})
			r.Use(gin.HandlerFunc(NewAPIKeyAuthMiddleware(svc, nil, cfg)))
			r.GET("/t", func(c *gin.Context) { c.Status(http.StatusOK) })

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/t", nil)
			req.Header.Set("x-api-key", "risk-gate-test-key")
			r.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body=%s)", w.Code, tc.wantStatus, w.Body.String())
			}
			if tc.wantStatus != http.StatusForbidden {
				return
			}
			body := w.Body.String()
			if !strings.Contains(body, service.UserRiskAccessBlockedCode) {
				t.Fatalf("403 body missing code %q: %s", service.UserRiskAccessBlockedCode, body)
			}
			if !strings.Contains(body, service.UserRiskAccessBlockedMessage) {
				t.Fatalf("403 body missing message %q: %s", service.UserRiskAccessBlockedMessage, body)
			}
			if captured != IngressRejectUserRiskReview {
				t.Fatalf("ingress reject reason = %q, want %q", captured, IngressRejectUserRiskReview)
			}
		})
	}
}

// TestAPIKeyAuthGoogleRiskGate 验证 Google 兼容网关（/v1beta）用同一口径
// 拦截待复核账号：403 + 纯英语 message。Google 风格错误体不含业务 code。
func TestAPIKeyAuthGoogleRiskGate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{RunMode: config.RunModeSimple}
	for _, level := range []string{service.AffiliateRiskLevelSuspect, service.AffiliateRiskLevelAbnormal} {
		t.Run(level, func(t *testing.T) {
			svc := newRiskGateService(level)
			r := gin.New()
			r.Use(APIKeyAuthGoogle(svc, cfg))
			r.GET("/v1beta/models", func(c *gin.Context) { c.Status(http.StatusOK) })

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/v1beta/models", nil)
			req.Header.Set("x-goog-api-key", "risk-gate-test-key")
			r.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403 (body=%s)", w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), service.UserRiskAccessBlockedMessage) {
				t.Fatalf("403 body missing message %q: %s", service.UserRiskAccessBlockedMessage, w.Body.String())
			}
		})
	}
}
