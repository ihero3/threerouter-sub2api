package service

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

// TestAPIKeyAuthSnapshotVersionIsBumpedForRiskLevel 保证快照结构变更时
// 版本号同步递增，否则旧缓存（无 risk_level）会被当成新快照直接命中，
// 闸门静默失效。
func TestAPIKeyAuthSnapshotVersionIsBumpedForRiskLevel(t *testing.T) {
	if apiKeyAuthSnapshotVersion != 26 {
		t.Fatalf("apiKeyAuthSnapshotVersion = %d, want 26 (user risk_level gate)", apiKeyAuthSnapshotVersion)
	}
}

// TestAPIKeyAuthSnapshotRoundTripsUserRiskLevel 验证 User.RiskLevel 在
// snapshotFromAPIKey -> snapshotToAPIKey 的往返中不丢失。丢字段会导致
// 网关拿到的 apiKey.User.RiskAccessBlocked() 恒 false。
func TestAPIKeyAuthSnapshotRoundTripsUserRiskLevel(t *testing.T) {
	// 两个快照函数只依赖纯字段拷贝 / compileAPIKeyIPRules，零值服务即可驱动。
	svc := &APIKeyService{}
	cases := []struct {
		name      string
		riskLevel string
	}{
		{name: "suspect", riskLevel: AffiliateRiskLevelSuspect},
		{name: "abnormal", riskLevel: AffiliateRiskLevelAbnormal},
		{name: "empty", riskLevel: ""},
		{name: "unknown", riskLevel: "some_future_level"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := &APIKey{
				ID:     1,
				UserID: 7,
				Status: StatusActive,
				User: &User{
					ID:        7,
					Status:    StatusActive,
					RiskLevel: tc.riskLevel,
				},
			}

			snapshot := svc.snapshotFromAPIKey(context.Background(), in)
			if snapshot == nil {
				t.Fatal("snapshotFromAPIKey returned nil")
			}
			if snapshot.User.RiskLevel != tc.riskLevel {
				t.Fatalf("snapshot.User.RiskLevel = %q, want %q", snapshot.User.RiskLevel, tc.riskLevel)
			}
			if snapshot.Version != apiKeyAuthSnapshotVersion {
				t.Fatalf("snapshot.Version = %d, want %d", snapshot.Version, apiKeyAuthSnapshotVersion)
			}

			out := svc.snapshotToAPIKey("test-key", snapshot)
			if out == nil || out.User == nil {
				t.Fatal("snapshotToAPIKey returned nil key or user")
			}
			if out.User.RiskLevel != tc.riskLevel {
				t.Fatalf("round-tripped User.RiskLevel = %q, want %q", out.User.RiskLevel, tc.riskLevel)
			}
			if out.User.RiskAccessBlocked() != in.User.RiskAccessBlocked() {
				t.Fatalf("RiskAccessBlocked changed after round trip: in=%v out=%v",
					in.User.RiskAccessBlocked(), out.User.RiskAccessBlocked())
			}
		})
	}
}

// TestAPIKeyAuthSnapshotRiskLevelJSONTag 验证 risk_level 会随快照序列化进
// 缓存（L2/Redis），反序列化后仍能读回；json tag 写错同样会让闸门失效。
func TestAPIKeyAuthSnapshotRiskLevelJSONTag(t *testing.T) {
	snapshot := &APIKeyAuthSnapshot{
		Version: apiKeyAuthSnapshotVersion,
		User: APIKeyAuthUserSnapshot{
			ID:        7,
			Status:    StatusActive,
			RiskLevel: AffiliateRiskLevelSuspect,
		},
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if !bytes.Contains(raw, []byte(`"risk_level":"suspect"`)) {
		t.Fatalf("snapshot JSON is missing risk_level: %s", raw)
	}
	var decoded APIKeyAuthSnapshot
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
	if decoded.User.RiskLevel != AffiliateRiskLevelSuspect {
		t.Fatalf("decoded User.RiskLevel = %q, want %q", decoded.User.RiskLevel, AffiliateRiskLevelSuspect)
	}
}
