package service

import "testing"

func TestResolveThinkingProtocol(t *testing.T) {
	tests := []struct {
		name    string
		modelID string
		want    ThinkingProtocol
	}{
		// Anthropic 官方
		{"claude-sonnet-4-5", "claude-sonnet-4-5", ThinkingProtocolAnthropicStrict},
		{"claude-opus-4-5", "claude-opus-4-5-20251101", ThinkingProtocolAnthropicStrict},
		{"claude-haiku full id", "claude-haiku-4-5-20251001", ThinkingProtocolAnthropicStrict},
		{"opus short", "opus-4-5", ThinkingProtocolAnthropicStrict},
		{"sonnet short", "sonnet-4-5", ThinkingProtocolAnthropicStrict},
		{"haiku short", "haiku-4-5", ThinkingProtocolAnthropicStrict},
		{"upper case Claude", "Claude-Sonnet-4-5", ThinkingProtocolAnthropicStrict},

		// 第三方兼容上游
		{"deepseek-v4-pro", "deepseek-v4-pro", ThinkingProtocolPassbackRequired},
		{"deepseek-r2-thinking", "deepseek-r2-thinking", ThinkingProtocolPassbackRequired},
		{"kimi-coding", "kimi-coding-v2", ThinkingProtocolPassbackRequired},
		{"kimi-k2-thinking", "kimi-k2-thinking", ThinkingProtocolPassbackRequired},
		{"kimi-k3 platform", "kimi-k3", ThinkingProtocolPassbackRequired},
		{"kimi code bare k3", "k3", ThinkingProtocolPassbackRequired},
		{"kimi code bare k3-256k", "k3-256k", ThinkingProtocolPassbackRequired},
		{"moonshot-v1", "moonshot-v1-32k", ThinkingProtocolPassbackRequired},
		{"glm-5.1", "glm-5.1", ThinkingProtocolPassbackRequired},
		{"qwen-2 thinking variant", "qwen-2-72b-thinking", ThinkingProtocolPassbackRequired},
		{"qwen3 thinking (real Alibaba naming)", "qwen3-235b-a22b-thinking-2507", ThinkingProtocolPassbackRequired},
		{"qwen3-next thinking", "qwen3-next-80b-a3b-thinking", ThinkingProtocolPassbackRequired},
		{"upper case Deepseek", "DeepSeek-V4-Pro", ThinkingProtocolPassbackRequired},

		// MiniMax M 系列（Anthropic 兼容端点要求 thinking round-trip）
		{"MiniMax-M2 (case-sensitive original)", "MiniMax-M2", ThinkingProtocolPassbackRequired},
		{"MiniMax-M2.1", "MiniMax-M2.1", ThinkingProtocolPassbackRequired},
		{"MiniMax-M2.5", "MiniMax-M2.5", ThinkingProtocolPassbackRequired},
		{"MiniMax-M2.7", "MiniMax-M2.7", ThinkingProtocolPassbackRequired},
		{"MiniMax-M2.7-highspeed", "MiniMax-M2.7-highspeed", ThinkingProtocolPassbackRequired},
		{"minimax-m2 lowercase", "minimax-m2", ThinkingProtocolPassbackRequired},

		// 未知 / 保守
		{"empty", "", ThinkingProtocolUnknown},
		{"gpt-5", "gpt-5.1", ThinkingProtocolUnknown},
		{"gemini", "gemini-3-pro-preview", ThinkingProtocolUnknown},
		{"qwen3 non-thinking", "qwen3-32b", ThinkingProtocolUnknown},
		{"qwen2 non-thinking", "qwen-2-72b", ThinkingProtocolUnknown},
		{"random vendor", "yi-large", ThinkingProtocolUnknown},
		// 相似但未知的 k3 型号：不得因含 k3 被宽泛匹配为 passback-required
		{"k3-like unknown", "foo-k3-bar", ThinkingProtocolUnknown},
		// MiniMax 非 M 系列（如 abab、speech 等其他产品线）—— unknown
		{"minimax abab non-M", "abab6.5-chat", ThinkingProtocolUnknown},
		// Doubao 走 OpenAI 协议，不属于本网关 Anthropic 路径——归 unknown
		{"doubao goes via openai", "doubao-1-5-thinking-vision-pro-250428", ThinkingProtocolUnknown},
		// Hunyuan T1 未暴露 Anthropic 端点——归 unknown
		{"hunyuan t1 no anthropic endpoint", "hunyuan-t1", ThinkingProtocolUnknown},
		{"hy-t1 short alias", "hy-t1", ThinkingProtocolUnknown},
		// claude-something 但不是 anthropic 官方命名风格——也归 strict（前缀匹配优先）
		{"weird claude prefix", "claude-experimental-fork", ThinkingProtocolAnthropicStrict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveThinkingProtocol(tt.modelID)
			if got != tt.want {
				t.Errorf("ResolveThinkingProtocol(%q) = %v, want %v", tt.modelID, got, tt.want)
			}
		})
	}
}

func TestShouldPreFilterThinkingBlocks(t *testing.T) {
	tests := []struct {
		modelID string
		want    bool
	}{
		{"claude-sonnet-4-5", true},
		{"deepseek-v4-pro", false},
		{"kimi-coding", false},
		{"glm-5.1", false},
		{"gpt-5.1", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			if got := ShouldPreFilterThinkingBlocks(tt.modelID); got != tt.want {
				t.Errorf("ShouldPreFilterThinkingBlocks(%q) = %v, want %v", tt.modelID, got, tt.want)
			}
		})
	}
}

func TestShouldRectifyThinkingSignatureError(t *testing.T) {
	if !ShouldRectifyThinkingSignatureError("claude-sonnet-4-5") {
		t.Error("anthropic-strict should rectify signature error")
	}
	if ShouldRectifyThinkingSignatureError("deepseek-v4-pro") {
		t.Error("passback-required must NOT rectify (would break protocol contract)")
	}
	if ShouldRectifyThinkingSignatureError("gpt-5.1") {
		t.Error("unknown should NOT rectify (conservative default)")
	}
	if ShouldRectifyThinkingSignatureError("") {
		t.Error("empty model id should NOT rectify")
	}
}

// ShouldApplyRetryFilters 与 ShouldPreFilterThinkingBlocks 必须语义一致：
// 仅 anthropic-strict 走变形，避免预过滤跳过但 retry 路径反而剥离的语义裂缝。
func TestShouldApplyRetryFiltersMirrorsPreFilter(t *testing.T) {
	models := []string{
		"claude-sonnet-4-5", "claude-opus-4-5-20251101", "haiku-4-5",
		"deepseek-v4-pro", "kimi-coding", "glm-5.1",
		"qwen3-235b-a22b-thinking-2507", "qwen3-32b",
		"gpt-5.1", "gemini-3-pro-preview", "yi-large", "",
	}
	for _, m := range models {
		t.Run(m, func(t *testing.T) {
			if got := ShouldApplyRetryFilters(m); got != ShouldPreFilterThinkingBlocks(m) {
				t.Errorf("ShouldApplyRetryFilters(%q)=%v but ShouldPreFilterThinkingBlocks=%v — must match",
					m, got, ShouldPreFilterThinkingBlocks(m))
			}
		})
	}
}

// ShouldReplayToolCallReasoning 是 CC→Responses 桥接的回注门控。它不能收窄成
// `== ThinkingProtocolPassbackRequired`：ResolveThinkingProtocol 靠硬编码厂商
// 前缀匹配，而运营后台添加的模型名不受控（glm4.6、deepseek_flash、自定义别名），
// 匹配不上就会静默退回「把推理塞进 <thinking> 明文」，上游照样 400。
func TestShouldReplayToolCallReasoning(t *testing.T) {
	// 标准厂商前缀：必须回注。
	for _, id := range []string{"deepseek-flash", "deepseek-v4-pro", "kimi-k2", "glm-4.6", "minimax-m2", "qwen3-max-thinking", "k3"} {
		if !ShouldReplayToolCallReasoning(id) {
			t.Fatalf("ShouldReplayToolCallReasoning(%q) = false, want true", id)
		}
	}

	// 运营后台自定义/不规范命名：ResolveThinkingProtocol 判不出来（Unknown），
	// 但同样可能是要求回注的上游，必须放行——这正是门控收窄时漏掉的一类。
	for _, id := range []string{
		"glm4.6",                // 少了连字符，不匹配 glm-
		"deepseek_flash",        // 下划线，不匹配 deepseek-
		"DeepSeek-V4-Pro",       // 大小写不同但 ToLower 后命中，仍须放行
		"ds-r1",                 // 中转别名
		"deepseek-v4-pro-local", // 带后缀
		"gpt-5-thinking",        // 未知厂商但确实在思考
		"",                      // 未知：保守放行而非拒绝
	} {
		if !ShouldReplayToolCallReasoning(id) {
			t.Fatalf("ShouldReplayToolCallReasoning(%q) = false, want true (unknown protocol must still replay)", id)
		}
	}

	// Anthropic 官方语义：thinking block 必须带有效 signature，reasoning item
	// 回注不适用，必须排除。
	for _, id := range []string{"claude-sonnet-4-6", "claude-opus-4-5", "opus-4", "sonnet-4", "haiku-3"} {
		if ShouldReplayToolCallReasoning(id) {
			t.Fatalf("ShouldReplayToolCallReasoning(%q) = true, want false (anthropic-strict)", id)
		}
	}
}
