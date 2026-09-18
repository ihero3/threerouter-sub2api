package service

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// MiniMax 只有一个参考图入口 subject_reference。OpenAI 风格的 image/image_url 已经
// 由链路转成 subject_reference，若再原样透传，同一个参考图会以两种形态重复出现；
// quality / negative_prompt 是 MiniMax 不支持的字段；style 两边类型还不同
// （OpenAI 字符串 vs MiniMax 对象）。这些都必须在这里拦住。
func TestBuildMiniMaxImageBodyDropsOpenAIOnlyFields(t *testing.T) {
	req := MediaCreateRequest{
		UpstreamModel: "minimax-image-01",
		Prompt:        "把这张图改成水彩风格",
		Resolution:    "1024x1024",
		ImageRefURLs:  []string{"https://cdn.example.com/ref.png"},
		Extra: map[string]any{
			"image":           "https://cdn.example.com/ref.png",
			"image_url":       "https://cdn.example.com/ref.png",
			"quality":         "hd",
			"style":           "vivid", // OpenAI 字符串形态，MiniMax 期望对象
			"negative_prompt": "模糊",
			"response_format": "url", // MiniMax 原生字段，必须保留
		},
	}
	var body map[string]any
	require.NoError(t, json.Unmarshal(buildMiniMaxImageCreateBody(req), &body))

	// 带厂商前缀的别名要收敛成官方枚举值
	require.Equal(t, "image-01", body["model"])
	require.Equal(t, "把这张图改成水彩风格", body["prompt"])
	require.Equal(t, float64(1024), body["width"])
	require.Equal(t, float64(1024), body["height"])

	require.NotContains(t, body, "image")
	require.NotContains(t, body, "image_url")
	require.NotContains(t, body, "image_urls")
	require.NotContains(t, body, "quality")
	require.NotContains(t, body, "style")
	require.NotContains(t, body, "negative_prompt")
	// 原生字段不能被误伤
	require.Equal(t, "url", body["response_format"])

	refs, ok := body["subject_reference"].([]any)
	require.True(t, ok, "参考图应转成 subject_reference")
	require.Len(t, refs, 1)
	ref, ok := refs[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "https://cdn.example.com/ref.png", ref["image_file"])
	require.Equal(t, "character", ref["type"])
}

// 按 MiniMax 官方文档直接调用的用户不能被我们的自动填充改写。
// 这里曾经有 bug：subject_reference 被写进了 Extra 的排除列表，导致下面那句
// 「用户显式传了就不覆盖」的判断永远看不到它，用户自定义被静默丢弃。
func TestBuildMiniMaxImageBodyKeepsNativeSubjectReference(t *testing.T) {
	req := MediaCreateRequest{
		UpstreamModel: "image-01",
		Prompt:        "换个场景",
		ImageRefURLs:  []string{"https://cdn.example.com/from-image-field.png"},
		Extra: map[string]any{
			"subject_reference": []any{
				map[string]any{"type": "character", "image_file": "https://cdn.example.com/user-explicit.png"},
			},
			"style": map[string]any{"style_type": "水彩", "style_weight": 0.8},
		},
	}
	var body map[string]any
	require.NoError(t, json.Unmarshal(buildMiniMaxImageCreateBody(req), &body))

	refs, ok := body["subject_reference"].([]any)
	require.True(t, ok)
	require.Len(t, refs, 1)
	// 用户显式传的必须胜出，不能被 ImageRefURLs 里的那张覆盖
	require.Equal(t, "https://cdn.example.com/user-explicit.png", refs[0].(map[string]any)["image_file"])
	// image 字段本身不落进 body
	require.NotContains(t, body, "image")
	// MiniMax 原生 style 对象要透传，不能被 OpenAI 字符串的过滤逻辑误删
	style, ok := body["style"].(map[string]any)
	require.True(t, ok, "MiniMax 原生 style 对象应透传")
	require.Equal(t, "水彩", style["style_type"])
}

// MiniMax 的 SubjectReference 每次只支持一张，多张时必须取第一张而不是全塞进去，
// 否则上游直接判参数异常。
func TestBuildMiniMaxImageBodyUsesSingleReference(t *testing.T) {
	req := MediaCreateRequest{
		UpstreamModel: "image-01",
		Prompt:        "p",
		ImageRefURLs: []string{
			"https://cdn.example.com/first.png",
			"https://cdn.example.com/second.png",
		},
	}
	var body map[string]any
	require.NoError(t, json.Unmarshal(buildMiniMaxImageCreateBody(req), &body))
	refs := body["subject_reference"].([]any)
	require.Len(t, refs, 1)
	require.Equal(t, "https://cdn.example.com/first.png", refs[0].(map[string]any)["image_file"])
}

// 纯文生图不能凭空多出 subject_reference。
func TestBuildMiniMaxImageBodyWithoutReference(t *testing.T) {
	req := MediaCreateRequest{UpstreamModel: "image-01", Prompt: "一只猫"}
	var body map[string]any
	require.NoError(t, json.Unmarshal(buildMiniMaxImageCreateBody(req), &body))
	require.NotContains(t, body, "subject_reference")
	require.Equal(t, "image-01", body["model"])
}

// HTTP 200 + status_code != 0 是 MiniMax 表达业务失败的方式，只看状态码会当成成功。
func TestParseMiniMaxImageCreateResultTreatsNonZeroStatusAsFailure(t *testing.T) {
	resp := []byte(`{"id":"x","base_resp":{"status_code":1026,"status_msg":"illegal"}}`)
	result, err := parseMiniMaxImageCreateResult(resp, 200)
	require.NoError(t, err)
	require.Equal(t, "failed", result.Status)
	require.Empty(t, result.InlineURL)
}

// 1026 只回数字，调用方无法自助排障，必须带一句可执行的指引。
func TestParseMiniMaxImageCreateResultExplainsStatusCodes(t *testing.T) {
	cases := []struct {
		code int
		want string
	}{
		{1002, "限流"},
		{1004, "API Key"},
		{1008, "余额不足"},
		{1026, "单人正面人像"},
		{2013, "size"},
		{2049, "API Key"},
	}
	for _, c := range cases {
		raw := []byte(`{"base_resp":{"status_code":` + strconv.Itoa(c.code) + `,"status_msg":"boom"}}`)
		result, err := parseMiniMaxImageCreateResult(raw, 200)
		require.NoError(t, err)
		require.Equal(t, "failed", result.Status)
		require.Contains(t, result.ErrorMessage, strconv.Itoa(c.code), "错误码必须保留")
		require.Contains(t, result.ErrorMessage, c.want, "缺少状态码 %d 的排查指引", c.code)
	}
}

// MiniMax 官方限制 prompt 最长 1500 字符（中文同计）。超长必须前置拒绝，
// 否则打一次必然失败的上游，用户只拿到一句 2013 invalid params。
func TestMiniMaxImageValidateCreateRejectsOverlongPrompt(t *testing.T) {
	a := NewMiniMaxImageAdapter()
	require.NotNil(t, a.validateCreate)

	// 恰好 1500 字符（含中文）应放行
	require.NoError(t, a.validateCreate(MediaCreateRequest{Prompt: strings.Repeat("画", 1500)}))
	// 空与短 prompt 放行
	require.NoError(t, a.validateCreate(MediaCreateRequest{Prompt: ""}))
	require.NoError(t, a.validateCreate(MediaCreateRequest{Prompt: "一只猫"}))
	// 1501 字符拒绝，错误信息要带上限数值
	err := a.validateCreate(MediaCreateRequest{Prompt: strings.Repeat("画", 1501)})
	require.Error(t, err)
	require.Contains(t, err.Error(), "1500")
	// 英文按 rune 计（不是字节）：1500 个 ASCII 字符放行，1501 拒绝
	require.NoError(t, a.validateCreate(MediaCreateRequest{Prompt: strings.Repeat("a", 1500)}))
	require.Error(t, a.validateCreate(MediaCreateRequest{Prompt: strings.Repeat("a", 1501)}))
}

// 正常响应走 success 分支，不应带 hint。
func TestParseMiniMaxImageCreateResultSuccess(t *testing.T) {
	raw := []byte(`{"data":{"image_urls":["https://cdn.example.com/a.png","https://cdn.example.com/b.png"]},"base_resp":{"status_code":0,"status_msg":"success"}}`)
	result, err := parseMiniMaxImageCreateResult(raw, 200)
	require.NoError(t, err)
	require.Equal(t, "succeeded", result.Status)
	require.Equal(t, MediaCompletionSync, result.Mode)
	require.Len(t, result.InlineURLs, 2)
	require.Equal(t, "https://cdn.example.com/a.png", result.InlineURL)
	require.Empty(t, result.ErrorMessage)
}
