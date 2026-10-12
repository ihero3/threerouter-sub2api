package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// codexCatalogContractKeysExcludingConditional 是目录条目**必须始终存在**的键。
//
// 唯一例外是 default_reasoning_level：它带 omitempty，且在"上游推理能力冲突"时被
// 故意清空（见 applyUpstreamModelMetadataToCodexDescriptor）——其语义是"该别名不宣告
// 任何档位"，客户端因此不显示推理档位选择器，这是 fail-closed 的既定取舍，不是缺字段。
// 把这个事实写成断言，避免以后有人无意中改掉条目形状而没人发现。
var codexCatalogContractKeysExcludingConditional = []string{
	"slug", "display_name", "description", "supported_reasoning_levels", "shell_type",
	"visibility", "supported_in_api", "priority", "additional_speed_tiers", "service_tiers",
	"default_service_tier", "availability_nux", "upgrade", "model_messages",
	"include_skills_usage_instructions", "include_plugin_usage_instructions",
	"include_apps_usage_instructions", "supports_reasoning_summary_parameter",
	"default_reasoning_summary", "support_verbosity", "default_verbosity",
	"apply_patch_tool_type", "web_search_tool_type", "truncation_policy",
	"supports_image_detail_original", "supports_parallel_tool_calls", "context_window",
	"max_context_window", "auto_compact_token_limit", "comp_hash",
	"effective_context_window_percent", "experimental_supported_tools", "input_modalities",
	"supports_search_tool", "use_responses_lite", "node_repl_auto_review_required",
	"node_repl_disabled", "auto_review_model_override", "model_specialty", "tool_mode",
	"multi_agent_version",
}

func decodeSingleCodexCatalogEntry(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var envelope struct {
		Models []map[string]any `json:"models"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope), "body=%s", string(body))
	require.Len(t, envelope.Models, 1)
	return envelope.Models[0]
}

// 条目形状在两条路径上都必须满足 §3 硬约束：数组非 null、数值 > 0、
// instructions_template 非空、visibility=list。任一条被破坏都会让**整份**目录在
// Codex 侧失效（客户端零容错、且不报错），所以用测试钉住。
func TestCodexCatalogEntrySatisfiesHardConstraints(t *testing.T) {
	body, err := BuildCodexModelsManifest([]string{"gpt-5.6-sol", "deepseek-v4-pro"})
	require.NoError(t, err)
	require.Contains(t, string(body), `"models"`, "顶层必须是 {\"models\":[...]}")

	var envelope struct {
		Models []map[string]any `json:"models"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))
	require.Len(t, envelope.Models, 2)

	for _, entry := range envelope.Models {
		slug, _ := entry["slug"].(string)
		require.Equal(t, "list", entry["visibility"], "slug=%s", slug)

		// 条目形状只有两种，且都以 41 个「恒定键」为底：
		//   42 键 = 41 恒定键 + default_reasoning_level（该别名有可宣告的推理档位）
		//   41 键 = 仅恒定键（上游推理能力冲突，故意不宣告档位——fail-closed 取舍）
		// 出现第三种键数说明条目形状被改动，必须显式确认后才能放行。
		contractBaseKeys := len(codexCatalogContractKeysExcludingConditional)
		hasDefaultLevel := entry["default_reasoning_level"] != nil
		wantKeys := contractBaseKeys
		shapeNote := "（仅 41 恒定键：不宣告档位）"
		if hasDefaultLevel {
			wantKeys++
			shapeNote = "（41 恒定键 + default_reasoning_level）"
		}
		require.Len(t, entry, wantKeys, "slug=%s 条目键数为 %d，期望 %d %s", slug, len(entry), wantKeys, shapeNote)
		for _, key := range codexCatalogContractKeysExcludingConditional {
			_, exists := entry[key]
			require.True(t, exists, "slug=%s 缺少契约字段 %s", slug, key)
		}

		messages, ok := entry["model_messages"].(map[string]any)
		require.True(t, ok, "slug=%s model_messages 必须是对象", slug)
		require.NotEmpty(t, messages["instructions_template"], "slug=%s instructions_template 不得为空", slug)

		for _, key := range []string{
			"supported_reasoning_levels", "additional_speed_tiers", "service_tiers",
			"experimental_supported_tools", "input_modalities",
		} {
			value, exists := entry[key]
			require.True(t, exists, "slug=%s 缺少数组字段 %s", slug, key)
			require.NotNil(t, value, "slug=%s 数组字段 %s 不得为 null", slug, key)
			_, isSlice := value.([]any)
			require.True(t, isSlice, "slug=%s 字段 %s 必须是数组", slug, key)
		}

		for _, key := range []string{"context_window", "max_context_window", "effective_context_window_percent"} {
			number, ok := entry[key].(float64)
			require.True(t, ok, "slug=%s %s 必须是数值", slug, key)
			require.Greater(t, number, float64(0), "slug=%s %s 必须 > 0", slug, key)
		}

		policy, ok := entry["truncation_policy"].(map[string]any)
		require.True(t, ok, "slug=%s truncation_policy 必须是对象", slug)
		limit, ok := policy["limit"].(float64)
		require.True(t, ok, "slug=%s truncation_policy.limit 必须是数值", slug)
		require.Greater(t, limit, float64(0), "slug=%s truncation_policy.limit 必须 > 0", slug)

		// default_reasoning_level 若存在，必须落在 supported_reasoning_levels 内。
		if defaultLevel, present := entry["default_reasoning_level"]; present && defaultLevel != nil {
			levels, _ := entry["supported_reasoning_levels"].([]any)
			require.NotEmpty(t, levels, "slug=%s 声明了默认档位却没有档位列表", slug)
			found := false
			for _, level := range levels {
				if model, ok := level.(map[string]any); ok && model["effort"] == defaultLevel {
					found = true
					break
				}
			}
			require.True(t, found, "slug=%s default_reasoning_level=%v 不在档位列表里", slug, defaultLevel)
		}
	}
}

// 未知字段必须被容忍（前向兼容）：Codex 以后新增字段时我们不需要改任何东西。
// 这条是本设计的关键前提（§3.7），因此用测试固定下来。
func TestCodexCatalogEntrySurvivesUnknownFields(t *testing.T) {
	body, err := BuildCodexModelsManifest([]string{"gpt-5.6-sol"})
	require.NoError(t, err)

	// 模拟"未来的 Codex 字段"：在条目与顶层各塞一个未知键，再解析一次。
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &raw))
	var models []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw["models"], &models))
	models[0]["future_model_field"] = json.RawMessage(`{"anything":true}`)
	patchedModels, err := json.Marshal(models)
	require.NoError(t, err)
	raw["models"] = patchedModels
	raw["future_top_level"] = json.RawMessage(`"ignored"`)
	patched, err := json.Marshal(raw)
	require.NoError(t, err)

	entry := decodeSingleCodexCatalogEntry(t, patched)
	require.Contains(t, entry, "future_model_field")
	require.NotEmpty(t, entry["model_messages"].(map[string]any)["instructions_template"])
	require.Greater(t, entry["context_window"].(float64), float64(0))
}
