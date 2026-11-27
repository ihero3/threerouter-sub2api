package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
)

func TestNormalizeOpenAIMessagesDispatchModelConfig(t *testing.T) {
	t.Parallel()

	cfg := normalizeOpenAIMessagesDispatchModelConfig(OpenAIMessagesDispatchModelConfig{
		OpusMappedModel:   " gpt-5.4-high ",
		SonnetMappedModel: "gpt-5.3-codex",
		HaikuMappedModel:  " gpt-5.4-mini-medium ",
		ExactModelMappings: map[string]string{
			" claude-sonnet-4-5-20250929 ": " gpt-5.2-high ",
			"":                             "gpt-5.4",
			"claude-opus-4-6":              " ",
		},
	})

	require.Equal(t, "gpt-5.4", cfg.OpusMappedModel)
	require.Equal(t, "gpt-5.3-codex", cfg.SonnetMappedModel)
	require.Equal(t, "gpt-5.4-mini", cfg.HaikuMappedModel)
	require.Equal(t, map[string]string{
		"claude-sonnet-4-5-20250929": "gpt-5.2",
	}, cfg.ExactModelMappings)
}

func TestGroupResolveMessagesDispatchModel_GrokRequiresCrossClientMapping(t *testing.T) {
	original := xai.RuntimeModelMappingOptions()
	t.Cleanup(func() { xai.SetRuntimeModelMappingOptions(original) })
	group := &Group{Platform: PlatformGrok}

	xai.SetRuntimeModelMappingOptions(xai.ModelMappingOptions{})
	require.Empty(t, group.ResolveMessagesDispatchModel("claude-sonnet-4-5"))

	xai.SetRuntimeModelMappingOptions(xai.ModelMappingOptions{
		DefaultText:          "grok-build-0.1",
		EnableCrossClientMap: true,
	})
	require.Equal(t, "grok-build-0.1", group.ResolveMessagesDispatchModel("claude-sonnet-4-5"))
	require.Equal(t, "grok-build-0.1", group.ResolveMessagesDispatchModel("claude-opus-4-6"))
	require.Equal(t, "grok-build-0.1", group.ResolveMessagesDispatchModel("claude-haiku-4-5"))
	require.Empty(t, group.ResolveMessagesDispatchModel("grok"))
	require.Empty(t, group.ResolveMessagesDispatchModel("gpt-5.3-codex"))
}

func TestSanitizeGroupMessagesDispatchFields_ClearsNonOpenAIPlatform(t *testing.T) {
	t.Parallel()

	group := &Group{
		Platform:              PlatformAnthropic,
		AllowMessagesDispatch: true,
		DefaultMappedModel:    "gpt-5.6-sol",
		MessagesDispatchModelConfig: OpenAIMessagesDispatchModelConfig{
			SonnetMappedModel: "gpt-5.3-codex",
			ExactModelMappings: map[string]string{
				"claude-fable-5": "gpt-5.6-sol",
			},
		},
	}

	sanitizeGroupMessagesDispatchFields(group)

	require.False(t, group.AllowMessagesDispatch)
	require.Empty(t, group.DefaultMappedModel)
	require.Equal(t, OpenAIMessagesDispatchModelConfig{}, group.MessagesDispatchModelConfig)
}

func TestSanitizeGroupMessagesDispatchFields_PreservesCompositeDispatchToggle(t *testing.T) {
	t.Parallel()

	group := &Group{
		Platform:              PlatformComposite,
		AllowMessagesDispatch: true,
		DefaultMappedModel:    "gpt-5.6-sol",
		MessagesDispatchModelConfig: OpenAIMessagesDispatchModelConfig{
			SonnetMappedModel: "gpt-5.3-codex",
			ExactModelMappings: map[string]string{
				"claude-fable-5": "gpt-5.6-sol",
			},
		},
	}

	sanitizeGroupMessagesDispatchFields(group)

	require.True(t, group.AllowMessagesDispatch)
	require.Empty(t, group.DefaultMappedModel)
	require.Equal(t, OpenAIMessagesDispatchModelConfig{}, group.MessagesDispatchModelConfig)
}

// 线上 vip 分组的真实配置：分组级 Messages 调度映射产出公开别名，账号级
// model_mapping 再把它翻译成各上游的真实模型 ID。锁死这条两级映射链，
// 避免分组映射绕过账号映射把 deepseek-v4-pro / claude-opus-5 直接发给
// xiaomi / neutoken / deepseek 官方上游。
func TestProductionVipDispatchChainsThroughAccountMapping(t *testing.T) {
	group := &Group{
		Platform:              PlatformOpenAI,
		AllowMessagesDispatch: true,
		MessagesDispatchModelConfig: OpenAIMessagesDispatchModelConfig{
			OpusMappedModel:   "claude-opus-4.8",
			SonnetMappedModel: "claude-opus-5",
			HaikuMappedModel:  "deepseek-v4-pro",
			ExactModelMappings: map[string]string{
				"claude-opus-5":   "deepseek-v4-pro",
				"claude-opus-4.8": "deepseek-v4-pro",
			},
		},
	}

	xiaomi := &Account{Credentials: map[string]any{"model_mapping": map[string]any{
		"deepseek-v4-pro": "mimo-v2.6-pro",
		"claude-opus-5":   "mimo-v2.6-pro",
		"claude-opus-4.8": "mimo-v2.6-pro",
	}}}
	deepseekOfficial := &Account{Credentials: map[string]any{"model_mapping": map[string]any{
		"deepseek-v4-pro": "deepseek-flash",
		"claude-opus-5":   "deepseek-flash",
		"claude-opus-4.8": "deepseek-flash",
	}}}
	neutoken := &Account{Credentials: map[string]any{"model_mapping": map[string]any{
		"deepseek-v4-pro": "deepseek-v4.1-flash-1017",
		"claude-opus-5":   "deepseek-v4.1-flash-1017",
		"claude-opus-4.8": "deepseek-v4.1-flash-1017",
	}}}

	tests := []struct {
		clientModel          string
		wantDispatchModel    string
		wantXiaomiUpstream   string
		wantDeepseekUpstream string
		wantNeutokenUpstream string
	}{
		{"claude-3-5-haiku-20241022", "deepseek-v4-pro", "mimo-v2.6-pro", "deepseek-flash", "deepseek-v4.1-flash-1017"},
		{"claude-sonnet-5", "claude-opus-5", "mimo-v2.6-pro", "deepseek-flash", "deepseek-v4.1-flash-1017"},
		{"claude-opus-4-1-20250805", "claude-opus-4.8", "mimo-v2.6-pro", "deepseek-flash", "deepseek-v4.1-flash-1017"},
	}
	for _, tt := range tests {
		t.Run(tt.clientModel, func(t *testing.T) {
			dispatch := group.ResolveMessagesDispatchModel(tt.clientModel)
			require.Equal(t, tt.wantDispatchModel, dispatch)
			require.Equal(t, tt.wantXiaomiUpstream, resolveOpenAIForwardModel(xiaomi, tt.clientModel, dispatch))
			require.Equal(t, tt.wantDeepseekUpstream, resolveOpenAIForwardModel(deepseekOfficial, tt.clientModel, dispatch))
			require.Equal(t, tt.wantNeutokenUpstream, resolveOpenAIForwardModel(neutoken, tt.clientModel, dispatch))
		})
	}
}
