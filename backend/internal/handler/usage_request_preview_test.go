package handler

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// usagePreviewSettingRepoStub 只实现热路径读取用到的 GetValue / GetMultiple / GetAll。
type usagePreviewSettingRepoStub struct {
	values map[string]string
}

func (r *usagePreviewSettingRepoStub) Get(context.Context, string) (*service.Setting, error) {
	return nil, fmt.Errorf("unused")
}
func (r *usagePreviewSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if v, ok := r.values[key]; ok {
		return v, nil
	}
	return "", fmt.Errorf("setting %s not found", key)
}
func (r *usagePreviewSettingRepoStub) Set(context.Context, string, string) error { return nil }
func (r *usagePreviewSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	return r.values, nil
}
func (r *usagePreviewSettingRepoStub) SetMultiple(context.Context, map[string]string) error { return nil }
func (r *usagePreviewSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return r.values, nil
}
func (r *usagePreviewSettingRepoStub) Delete(context.Context, string) error { return nil }

var usagePreviewTestBody = []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hello preview world"}]}`)

func newUsagePreviewSettingService(values map[string]string) *service.SettingService {
	return service.NewSettingService(&usagePreviewSettingRepoStub{values: values}, &config.Config{})
}

func newUsagePreviewGinContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(usagePreviewTestBody))
	return c
}

// TestGatewayHandlerUsageRequestPreviewIsDecoupledFromRiskControl 锁住核心改动：
// usage_logs.request_preview 只看 usage_request_preview_enabled；风控中心开关开/关
// 都不再影响预览采集。若把实现改回读 RiskControlEnabled，本测试必然变红。
func TestGatewayHandlerUsageRequestPreviewIsDecoupledFromRiskControl(t *testing.T) {
	cases := []struct {
		name        string
		values      map[string]string
		wantPreview bool
	}{
		{
			name:        "both switches off stays empty",
			values:      map[string]string{},
			wantPreview: false,
		},
		{
			name:        "risk control on alone does NOT capture preview",
			values:      map[string]string{service.SettingKeyRiskControlEnabled: "true"},
			wantPreview: false,
		},
		{
			name: "preview on while risk control off captures preview",
			values: map[string]string{
				service.SettingKeyUsageRequestPreviewEnabled: "true",
				service.SettingKeyRiskControlEnabled:         "false",
			},
			wantPreview: true,
		},
		{
			name: "preview off while risk control on stays empty",
			values: map[string]string{
				service.SettingKeyUsageRequestPreviewEnabled: "false",
				service.SettingKeyRiskControlEnabled:         "true",
			},
			wantPreview: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &GatewayHandler{settingService: newUsagePreviewSettingService(tc.values)}
			got := h.usageRequestPreview(newUsagePreviewGinContext(), usagePreviewTestBody)
			if (got != "") != tc.wantPreview {
				t.Fatalf("usageRequestPreview = %q, wantPreview=%v (values=%v)", got, tc.wantPreview, tc.values)
			}
		})
	}

	// nil 安全：依赖缺失时保持不采集。
	if got := (&GatewayHandler{}).usageRequestPreview(newUsagePreviewGinContext(), usagePreviewTestBody); got != "" {
		t.Fatalf("missing settingService should not capture preview, got %q", got)
	}
	var nilHandler *GatewayHandler
	if got := nilHandler.usageRequestPreview(newUsagePreviewGinContext(), usagePreviewTestBody); got != "" {
		t.Fatalf("nil handler should not capture preview, got %q", got)
	}
}

// TestGatewayHandlerUsageRequestPreviewEnabledIsDecoupled 锁住 OpsErrorLoggerMiddleware
// 使用的回调：错误请求日志的 request_preview 同样由独立开关门控，与风控中心解耦。
func TestGatewayHandlerUsageRequestPreviewEnabledIsDecoupled(t *testing.T) {
	cases := []struct {
		name   string
		values map[string]string
		want   bool
	}{
		{
			name:   "risk control on alone keeps callback false",
			values: map[string]string{service.SettingKeyRiskControlEnabled: "true"},
			want:   false,
		},
		{
			name: "preview on while risk control off enables callback",
			values: map[string]string{
				service.SettingKeyUsageRequestPreviewEnabled: "true",
				service.SettingKeyRiskControlEnabled:         "false",
			},
			want: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &GatewayHandler{settingService: newUsagePreviewSettingService(tc.values)}
			if got := h.UsageRequestPreviewEnabled(context.Background()); got != tc.want {
				t.Fatalf("UsageRequestPreviewEnabled = %v, want %v", got, tc.want)
			}
		})
	}
	var nilHandler *GatewayHandler
	if nilHandler.UsageRequestPreviewEnabled(context.Background()) {
		t.Fatal("nil handler callback must stay false")
	}
	if (&GatewayHandler{}).UsageRequestPreviewEnabled(context.Background()) {
		t.Fatal("missing settingService callback must stay false")
	}
}

// TestUsageRequestPreviewHelper 锁住共享包级函数的门控与 nil 安全（两套 Handler 共用）。
func TestUsageRequestPreviewHelper(t *testing.T) {
	alwaysOn := func(context.Context) bool { return true }
	alwaysOff := func(context.Context) bool { return false }

	if got := usageRequestPreview(newUsagePreviewGinContext(), usagePreviewTestBody, alwaysOff); got != "" {
		t.Fatalf("disabled gate should return empty, got %q", got)
	}
	if got := usageRequestPreview(newUsagePreviewGinContext(), usagePreviewTestBody, alwaysOn); got == "" {
		t.Fatal("enabled gate should extract preview")
	}
	if got := usageRequestPreview(newUsagePreviewGinContext(), usagePreviewTestBody, nil); got != "" {
		t.Fatalf("nil gate should return empty, got %q", got)
	}
	if got := usageRequestPreview(nil, usagePreviewTestBody, alwaysOn); got != "" {
		t.Fatalf("nil gin context should return empty, got %q", got)
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder()) // Request 为 nil
	if got := usageRequestPreview(c, usagePreviewTestBody, alwaysOn); got != "" {
		t.Fatalf("nil request should return empty, got %q", got)
	}
}

// TestOpenAIGatewayHandlerUsageRequestPreviewNilSafe 覆盖第二条网关 Handler 的失败路径：
// 依赖缺失时不得 panic、不得采集（其开启路径由 OpenAIGatewayService 转发方法在
// service 层测试覆盖，两者共用上面的包级门控函数）。
func TestOpenAIGatewayHandlerUsageRequestPreviewNilSafe(t *testing.T) {
	var nilHandler *OpenAIGatewayHandler
	if got := nilHandler.usageRequestPreview(newUsagePreviewGinContext(), usagePreviewTestBody); got != "" {
		t.Fatalf("nil handler should not capture preview, got %q", got)
	}
	if got := (&OpenAIGatewayHandler{}).usageRequestPreview(newUsagePreviewGinContext(), usagePreviewTestBody); got != "" {
		t.Fatalf("missing gatewayService should not capture preview, got %q", got)
	}
	if got := (&OpenAIGatewayHandler{gatewayService: &service.OpenAIGatewayService{}}).usageRequestPreview(newUsagePreviewGinContext(), usagePreviewTestBody); got != "" {
		t.Fatalf("zero-value gatewayService should not capture preview, got %q", got)
	}
}
