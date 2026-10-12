package admin

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestSanitizeAdminPaymentOrderForResponseAddsCurrency(t *testing.T) {
	now := time.Now()
	order := &dbent.PaymentOrder{
		ID:          1,
		UserID:      2,
		Amount:      100,
		PayAmount:   108,
		FeeRate:     8,
		OutTradeNo:  "sub2_202606250001",
		PaymentType: "stripe",
		OrderType:   "subscription",
		Status:      "COMPLETED",
		ExpiresAt:   now,
		CreatedAt:   now,
		UpdatedAt:   now,
		ProviderSnapshot: map[string]any{
			"schema_version": 2,
			"currency":       "USD",
		},
	}

	got := sanitizeAdminPaymentOrderForResponse(order)
	if got == nil {
		t.Fatal("expected sanitized order")
	}
	if got.Currency != "USD" {
		t.Fatalf("expected currency USD, got %q", got.Currency)
	}

	body, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal sanitized order: %v", err)
	}
	if strings.Contains(string(body), "provider_snapshot") {
		t.Fatalf("expected provider_snapshot to be omitted, got %s", string(body))
	}
}

func TestAdminSubscriptionPlansForResponseIncludesCompositeGroupInfo(t *testing.T) {
	weekly := 25.0
	now := time.Now()
	plans := []*dbent.SubscriptionPlan{
		{
			ID:           11,
			GroupID:      7,
			Name:         "All models",
			Description:  "Composite access",
			Price:        19.99,
			ValidityDays: 30,
			ValidityUnit: "days",
			Features:     "OpenAI\nClaude\nGemini\nGrok",
			ProductName:  "Sub2API",
			ForSale:      true,
			SortOrder:    1,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}
	groupInfo := map[int64]service.PlanGroupInfo{
		7: {
			Platform:       service.PlatformComposite,
			Name:           "Bucket 2 composite",
			RateMultiplier: 1.5,
			WeeklyLimitUSD: &weekly,
			ModelScopes:    []string{"openai", "claude", "gemini", "grok"},
		},
	}

	got := adminSubscriptionPlansForResponse(plans, groupInfo)

	if len(got) != 1 {
		t.Fatalf("expected one plan, got %d", len(got))
	}
	if got[0].GroupPlatform != service.PlatformComposite {
		t.Fatalf("expected composite group platform, got %q", got[0].GroupPlatform)
	}
	if got[0].GroupName != "Bucket 2 composite" {
		t.Fatalf("expected group name to be included, got %q", got[0].GroupName)
	}
	if got[0].WeeklyLimitUSD == nil || *got[0].WeeklyLimitUSD != weekly {
		t.Fatalf("expected weekly limit to be included, got %#v", got[0].WeeklyLimitUSD)
	}
	if strings.Join(got[0].ModelScopes, ",") != "openai,claude,gemini,grok" {
		t.Fatalf("expected model scopes to be preserved, got %#v", got[0].ModelScopes)
	}
	// 套餐货币（Currency）已知缺口：AdminSubscriptionPlanResult 响应 DTO 含 Currency 字段，
	// 但 ent SubscriptionPlan 实体无 currency 列、生产投影也未填充，故响应货币恒为空。
	// 前端 PlanEditDialog 回传货币静默丢失属同一根因。待产品决策：在 subscription_plans
	// 增加 currency 列并打通 Create/Update/投影，或明确取消套餐级货币（统一全局结算币种）。
	// 此处不对此未实现字段做断言，避免伪装成通过的失败用例。
	if !got[0].CreatedAt.Equal(now) || !got[0].UpdatedAt.Equal(now) {
		t.Fatalf("expected created_at/updated_at to be preserved, got %v / %v", got[0].CreatedAt, got[0].UpdatedAt)
	}
}
