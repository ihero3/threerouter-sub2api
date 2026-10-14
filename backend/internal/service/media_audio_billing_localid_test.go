package service

import "testing"

// TestDeriveAudioSpeechBillingLocalID_StableAndDistinct 锁死 #1 修复：
// 同步音频合成之前用随机 localID 作计费幂等键，客户端重试会生成新键导致双扣。
// 改为按 (用户 + 端点 + 请求体) 派生稳定键后，本测试验证：同请求稳定、跨用户不串键、
// 不同输入不误合并、且保留 aud_ 前缀。
func TestDeriveAudioSpeechBillingLocalID_StableAndDistinct(t *testing.T) {
	userID := int64(42)
	body := map[string]any{"input": "hello world", "model": "tts-1", "voice": "alloy"}

	id1 := deriveAudioSpeechBillingLocalID(userID, body)
	id2 := deriveAudioSpeechBillingLocalID(userID, body)
	if id1 != id2 {
		t.Fatalf("same request must derive stable localID: %q != %q", id1, id2)
	}
	if id1 == "" {
		t.Fatal("localID must not be empty")
	}
	if len(id1) < 4 || id1[:4] != "aud_" {
		t.Fatalf("localID must keep aud_ prefix, got %q", id1)
	}

	// 不同用户 → 不同键（跨用户不串键）
	if other := deriveAudioSpeechBillingLocalID(43, body); other == id1 {
		t.Fatal("different user must derive different localID")
	}

	// 不同输入 → 不同键（不误合并）
	otherBody := map[string]any{"input": "different text", "model": "tts-1", "voice": "alloy"}
	if deriveAudioSpeechBillingLocalID(userID, otherBody) == id1 {
		t.Fatal("different input must derive different localID")
	}
}
