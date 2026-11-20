package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// usagePreviewSettingRepo 只实现热路径读取用到的 GetValue / GetMultiple。
type usagePreviewSettingRepo struct {
	values map[string]string
}

func (r *usagePreviewSettingRepo) Get(context.Context, string) (*Setting, error) {
	return nil, fmt.Errorf("unused")
}
func (r *usagePreviewSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	if v, ok := r.values[key]; ok {
		return v, nil
	}
	return "", fmt.Errorf("setting %s not found", key)
}
func (r *usagePreviewSettingRepo) Set(context.Context, string, string) error { return nil }
func (r *usagePreviewSettingRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	return r.values, nil
}
func (r *usagePreviewSettingRepo) SetMultiple(context.Context, map[string]string) error { return nil }
func (r *usagePreviewSettingRepo) GetAll(context.Context) (map[string]string, error) { return r.values, nil }
func (r *usagePreviewSettingRepo) Delete(context.Context, string) error                { return nil }

// TestUsageRequestPreviewEnabledIsDecoupledFromRiskControl 锁住核心改动：
// 预览采集只看 usage_request_preview_enabled，风控中心开关再怎么变都不影响它。
func TestUsageRequestPreviewEnabledIsDecoupledFromRiskControl(t *testing.T) {
	cases := []struct {
		name       string
		values     map[string]string
		wantEnable bool
	}{
		{
			name:       "missing setting defaults off",
			values:     map[string]string{},
			wantEnable: false,
		},
		{
			name:       "risk control on alone does not enable preview",
			values:     map[string]string{SettingKeyRiskControlEnabled: "true"},
			wantEnable: false,
		},
		{
			name: "explicit preview switch on",
			values: map[string]string{
				SettingKeyUsageRequestPreviewEnabled: "true",
			},
			wantEnable: true,
		},
		{
			name: "preview on while risk control off",
			values: map[string]string{
				SettingKeyUsageRequestPreviewEnabled: "true",
				SettingKeyRiskControlEnabled:         "false",
			},
			wantEnable: true,
		},
		{
			name: "non true value stays off",
			values: map[string]string{
				SettingKeyUsageRequestPreviewEnabled: "TRUE",
				SettingKeyRiskControlEnabled:         "true",
			},
			wantEnable: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewSettingService(&usagePreviewSettingRepo{values: tc.values}, nil)
			got := svc.IsUsageRequestPreviewEnabled(context.Background())
			if got != tc.wantEnable {
				t.Fatalf("IsUsageRequestPreviewEnabled = %v, want %v (values=%v)", got, tc.wantEnable, tc.values)
			}
		})
	}
}

// TestUsageRequestPreviewSettingRoundTrip 验证设置项经 ParseSettings → 视图 → 写入
// 不丢失：管理端开/关后能通过缓存读到。
func TestUsageRequestPreviewSettingRoundTrip(t *testing.T) {
	repo := &usagePreviewSettingRepo{values: map[string]string{
		SettingKeyUsageRequestPreviewEnabled: "true",
	}}
	svc := NewSettingService(repo, &config.Config{})

	if !svc.IsUsageRequestPreviewEnabled(context.Background()) {
		t.Fatal("preview switch should be on after parse")
	}
	view, err := svc.GetAllSettings(context.Background())
	if err != nil {
		t.Fatalf("GetAllSettings: %v", err)
	}
	if !view.UsageRequestPreviewEnabled {
		t.Fatal("GetAllSettings lost usage_request_preview_enabled")
	}
}

// TestOpenAIGatewayServiceUsageRequestPreviewEnabled 锁住 OpenAIGatewayHandler 路径：
// 预览开关经 OpenAIGatewayService 转发读取（该 Handler 不直接持有 SettingService），
// 且不受风控中心开关影响、nil 安全。
func TestOpenAIGatewayServiceUsageRequestPreviewEnabled(t *testing.T) {
	cases := []struct {
		name   string
		values map[string]string
		want   bool
	}{
		{"missing setting defaults off", map[string]string{}, false},
		{"risk control on alone stays off", map[string]string{SettingKeyRiskControlEnabled: "true"}, false},
		{"preview switch on", map[string]string{SettingKeyUsageRequestPreviewEnabled: "true"}, true},
		{
			name: "preview on while risk control off",
			values: map[string]string{
				SettingKeyUsageRequestPreviewEnabled: "true",
				SettingKeyRiskControlEnabled:         "false",
			},
			want: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &OpenAIGatewayService{
				settingService: NewSettingService(&usagePreviewSettingRepo{values: tc.values}, &config.Config{}),
			}
			if got := svc.UsageRequestPreviewEnabled(context.Background()); got != tc.want {
				t.Fatalf("UsageRequestPreviewEnabled = %v, want %v (values=%v)", got, tc.want, tc.values)
			}
		})
	}

	var nilSvc *OpenAIGatewayService
	if nilSvc.UsageRequestPreviewEnabled(context.Background()) {
		t.Fatal("nil service must stay off")
	}
	if (&OpenAIGatewayService{}).UsageRequestPreviewEnabled(context.Background()) {
		t.Fatal("missing settingService must stay off")
	}
}
