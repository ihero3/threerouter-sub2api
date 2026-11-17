package service

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// MiniMax-H3-Image 契约（platform.minimaxi.com 文档，2026-11）与旧版 image-01
// 完全不同：OpenAI 风格 body（model/prompt/size/n）+ data[] 数组响应 + 两个
// 端点（generations / edits）。这里固化请求体构造与响应解析的关键分支。

// 模型归属：H3 图片系列必须被本 adapter 认领，且不能抢走 H3 视频与旧版 image-01。
func TestIsMiniMaxH3ImageModel(t *testing.T) {
	for _, model := range []string{
		"MiniMax-H3-Image",
		"minimax-h3-image",
		"MiniMax-H3-Image-Highspeed",
		"hailuo-h3-image",
	} {
		require.True(t, isMiniMaxH3ImageModel(model), "%q 应属于 MiniMax H3 图片系列", model)
	}
	for _, model := range []string{
		"minimax-h3",        // H3 视频（/v2/video_generation）
		"MiniMax-H3",        // H3 视频官方大小写
		"minimax-hailuo-02", // 旧视频系列
		"minimax-image-01",  // 旧图片系列（走 MiniMaxImageAdapter）
		"image-01",          // 旧图片官方裸名
		"qwen-image-3.0",    // 其他厂商
		"qwen-image-h3",     // 无 minimax/hailuo 字样，不认领
		"",                  // 空
	} {
		require.False(t, isMiniMaxH3ImageModel(model), "%q 不应被 H3 图片 adapter 认领", model)
	}
}

// 适配器注册边界：H3 图片模型必须解析到 H3 adapter 而不是通用兜底。
func TestMiniMaxH3ImageAdapterRegistryRouting(t *testing.T) {
	adapter := NewMiniMaxH3ImageAdapter()
	require.Equal(t, MediaKindImage, adapter.Kind())
	for _, model := range []string{"MiniMax-H3-Image", "minimax-h3-image", "MiniMax-H3-Image-Highspeed"} {
		require.True(t, adapter.Supports(PlatformDeepseek, model), "H3 图片 adapter 应支持 %q", model)
	}
	require.False(t, adapter.Supports(PlatformDeepseek, "minimax-image-01"), "不应抢走旧版 image-01")
	require.False(t, adapter.Supports(PlatformDeepseek, "minimax-h3"), "不应抢走 H3 视频")

	registry := NewMediaAdapterRegistry(
		NewMiniMaxImageAdapter(),
		NewMiniMaxH3ImageAdapter(),
		NewImageMediaAdapter(),
	)
	resolved, err := registry.Resolve(MediaKindImage, PlatformDeepseek, "MiniMax-H3-Image")
	require.NoError(t, err)
	_, ok := resolved.(*MiniMaxH3ImageAdapter)
	require.True(t, ok, "minimax-h3-image 必须解析到 H3 专属 adapter，实际 %T", resolved)

	// 分派口径：图片判定优先于视频前缀（wanx 同款先例）。
	require.Equal(t, ModelCapabilityImage, DispatchModelCapability("minimax-h3-image"))
	require.Equal(t, MediaKindImage, MediaKindFromModel("MiniMax-H3-Image", nil))
	require.Equal(t, ModelCapabilityVideo, DispatchModelCapability("minimax-h3"), "H3 视频归属不能被影响")

	// composite 分组选号：承载平台必须可解析（否则 SelectAccountForModel 503）。
	platform, ok := DetectModelPlatform("minimax-h3-image")
	require.True(t, ok)
	require.Equal(t, PlatformDeepseek, platform)
}

// 文生图请求体：model 归一成官方枚举，size/n 结构化，原生字段（aspect_ratio/
// area/disable_prompt_enhanced/output_format）透传，OpenAI 专有字段拦下。
func TestBuildMiniMaxH3ImageCreateBodyTextToImage(t *testing.T) {
	req := MediaCreateRequest{
		UpstreamModel: "minimax-h3-image",
		Prompt:        "一只橘猫趴在窗台上晒太阳",
		Resolution:    "2720x1536",
		ImageCount:    3,
		Extra: map[string]any{
			"aspect_ratio":            "16:9",
			"area":                    "2K",
			"disable_prompt_enhanced": true,
			"output_format":           "png",
			// OpenAI 专有 / 幂等 / 已结构化字段，不应透传
			"quality":         "hd",
			"negative_prompt": "模糊",
			"style":           "vivid",
			"stream":          true,
			"response_format": "url",
			"request_id":      "idem-123",
		},
	}
	var body map[string]any
	require.NoError(t, json.Unmarshal(buildMiniMaxH3ImageCreateBody(req), &body))

	require.Equal(t, "MiniMax-H3-Image", body["model"])
	require.Equal(t, req.Prompt, body["prompt"])
	require.Equal(t, "2720x1536", body["size"])
	require.Equal(t, float64(3), body["n"])

	// H3 原生字段必须透传
	require.Equal(t, "16:9", body["aspect_ratio"])
	require.Equal(t, "2K", body["area"])
	require.Equal(t, true, body["disable_prompt_enhanced"])
	require.Equal(t, "png", body["output_format"])

	// OpenAI 专有字段 / 幂等键 / stream 必须拦下
	for _, key := range []string{"quality", "negative_prompt", "style", "stream", "response_format", "request_id"} {
		require.NotContains(t, body, key, "%s 不应进上游请求体", key)
	}
	// 纯文生图不能凭空多出 images
	require.NotContains(t, body, "images")
}

// 官方写法（照文档直调）的用户不能被改写：显式 images[] 优先于链路转换的
// ImageRefURLs，model 用官方名时不变形，Highspeed 归一到高速版枚举。
func TestBuildMiniMaxH3ImageCreateBodyNativeImagesWin(t *testing.T) {
	req := MediaCreateRequest{
		UpstreamModel: "MiniMax-H3-Image",
		Prompt:        "把图1里的猫换成图2里的柴犬",
		ImageRefURLs:  []string{"https://cdn.example.com/from-openai-field.png"},
		Extra: map[string]any{
			"images": []any{
				map[string]any{"image_url": "https://example.com/cat.jpg"},
				map[string]any{"image_url": "https://example.com/dog.png"},
			},
		},
	}
	var body map[string]any
	require.NoError(t, json.Unmarshal(buildMiniMaxH3ImageCreateBody(req), &body))

	images, ok := body["images"].([]any)
	require.True(t, ok, "参考图应转成官方 images[] 契约")
	require.Len(t, images, 2)
	first, _ := images[0].(map[string]any)
	require.Equal(t, "https://example.com/cat.jpg", first["image_url"])
	// 官方显式 images 必须胜出，不能混入 ImageRefURLs 转换出来的那张
	for _, raw := range images {
		item, _ := raw.(map[string]any)
		require.NotEqual(t, "https://cdn.example.com/from-openai-field.png", item["image_url"])
	}

	// Highspeed 归一
	highspeed := MediaCreateRequest{UpstreamModel: "MiniMax-H3-Image-Highspeed", Prompt: "p"}
	var hsBody map[string]any
	require.NoError(t, json.Unmarshal(buildMiniMaxH3ImageCreateBody(highspeed), &hsBody))
	require.Equal(t, "MiniMax-H3-Image-Highspeed", hsBody["model"])

	// 渠道私有命名原样透传（用账号 model_mapping 自定义的型号）
	private := MediaCreateRequest{UpstreamModel: "my-channel-h3img", Prompt: "p"}
	var pvBody map[string]any
	require.NoError(t, json.Unmarshal(buildMiniMaxH3ImageCreateBody(private), &pvBody))
	require.Equal(t, "my-channel-h3img", pvBody["model"])
}

// OpenAI 风格 image 字段（含 multipart 上传转成的 data URI）要转成 images[]。
func TestBuildMiniMaxH3ImageCreateBodyFromImageRefURLs(t *testing.T) {
	req := MediaCreateRequest{
		UpstreamModel: "minimax-h3-image",
		Prompt:        "改成水彩风格",
		ImageRefURLs: []string{
			"https://cdn.example.com/a.png",
			"data:image/jpeg;base64,AAAA",
		},
	}
	var body map[string]any
	require.NoError(t, json.Unmarshal(buildMiniMaxH3ImageCreateBody(req), &body))
	images, ok := body["images"].([]any)
	require.True(t, ok)
	require.Len(t, images, 2)
	first, _ := images[0].(map[string]any)
	require.Equal(t, "https://cdn.example.com/a.png", first["image_url"])
	second, _ := images[1].(map[string]any)
	require.Equal(t, "data:image/jpeg;base64,AAAA", second["image_url"])
}

// 前置校验：prompt 32000 字符上限（按 rune 计）、参考图 16 张上限。
func TestValidateMiniMaxH3ImageCreate(t *testing.T) {
	a := NewMiniMaxH3ImageAdapter()
	require.NotNil(t, a.validateCreate)

	// 恰好 32000 字符（中文同计）放行
	require.NoError(t, a.validateCreate(MediaCreateRequest{Prompt: strings.Repeat("画", miniMaxH3ImageMaxPromptChars)}))
	// 32001 拒绝，错误信息带上限数值
	err := a.validateCreate(MediaCreateRequest{Prompt: strings.Repeat("画", miniMaxH3ImageMaxPromptChars+1)})
	require.Error(t, err)
	require.Contains(t, err.Error(), strconv.Itoa(miniMaxH3ImageMaxPromptChars))
	// 英文按 rune 计
	require.NoError(t, a.validateCreate(MediaCreateRequest{Prompt: strings.Repeat("a", miniMaxH3ImageMaxPromptChars)}))
	require.Error(t, a.validateCreate(MediaCreateRequest{Prompt: strings.Repeat("a", miniMaxH3ImageMaxPromptChars+1)}))

	// 参考图 16 张放行、17 张拒绝
	refs16 := make([]string, miniMaxH3ImageMaxRefs)
	for i := range refs16 {
		refs16[i] = "https://cdn.example.com/ref.png"
	}
	require.NoError(t, a.validateCreate(MediaCreateRequest{Prompt: "p", ImageRefURLs: refs16}))
	require.Error(t, a.validateCreate(MediaCreateRequest{Prompt: "p", ImageRefURLs: append(refs16, "one-more.png")}))
}

// 正常成功：data[] 全收（不是只取第一张）、b64_json 转 data URI（MIME 按
// output_format）、UpstreamSize 取第一张真实尺寸供计费分档。
func TestParseMiniMaxH3ImageCreateResultSuccess(t *testing.T) {
	raw := []byte(`{
		"data": [
			{"url": "https://cdn.example.com/out-0.jpg", "size": "2720x1536", "revised_prompt": "..."},
			{"b64_json": "QUJD", "size": "2720x1536"}
		],
		"output_format": "png",
		"usage": {"generated_images": 2}
	}`)
	result, err := parseMiniMaxH3ImageCreateResult(raw, 200)
	require.NoError(t, err)
	require.Equal(t, "succeeded", result.Status)
	require.Equal(t, MediaCompletionSync, result.Mode)
	require.Len(t, result.InlineURLs, 2)
	require.Equal(t, "https://cdn.example.com/out-0.jpg", result.InlineURL)
	require.Equal(t, "data:image/png;base64,QUJD", result.InlineURLs[1])
	require.Equal(t, "2720x1536", result.UpstreamSize)
	require.Empty(t, result.ErrorMessage)
}

// 部分失败：只要有一张成功就是 200 + 成功；失败项的 error 进不了产物列表。
func TestParseMiniMaxH3ImageCreateResultPartialFailure(t *testing.T) {
	raw := []byte(`{
		"data": [
			{"url": "https://cdn.example.com/output-0.jpg", "size": "2048x2048"},
			{"error": {"code": "1027", "message": "output image failed content safety check"}}
		],
		"output_format": "jpeg",
		"usage": {"generated_images": 1}
	}`)
	result, err := parseMiniMaxH3ImageCreateResult(raw, 200)
	require.NoError(t, err)
	require.Equal(t, "succeeded", result.Status)
	require.Len(t, result.InlineURLs, 1)
}

// 全部失败（422 内容安全 / 402 余额不足）：base_resp 语义要进错误消息，
// 让调用方能自助排障，而不是只看到一句状态码文本。
func TestParseMiniMaxH3ImageCreateResultAllFailed(t *testing.T) {
	cases := []struct {
		status int
		raw    string
		want   []string
	}{
		{
			status: 422,
			raw:    `{"base_resp":{"status_code":1027,"status_msg":"content safety"},"data":[],"usage":{"generated_images":0}}`,
			want:   []string{"1027", "content safety", "content safety check"},
		},
		{
			status: 402,
			raw:    `{"base_resp":{"status_code":1008,"status_msg":"insufficient balance"}}`,
			want:   []string{"1008", "insufficient balance"},
		},
	}
	for _, c := range cases {
		result, err := parseMiniMaxH3ImageCreateResult([]byte(c.raw), c.status)
		require.NoError(t, err)
		require.Equal(t, "failed", result.Status)
		require.Empty(t, result.InlineURL)
		require.Equal(t, c.status, result.UpstreamStatusCode)
		for _, want := range c.want {
			require.Contains(t, result.ErrorMessage, want)
		}
	}
}

// 200 但没有任何产物（含全部项都是 error 的防御分支）：必须判失败并把
// 每项的失败原因拼进消息 —— 判成功会立刻真实扣费而调用方拿 502。
func TestParseMiniMaxH3ImageCreateResultNoPayloadIsFailure(t *testing.T) {
	raw := []byte(`{
		"data": [
			{"error": {"code": "1027", "message": "safety"}},
			{"error": {"code": "1028", "message": "safety again"}}
		],
		"usage": {"generated_images": 0}
	}`)
	result, err := parseMiniMaxH3ImageCreateResult(raw, 200)
	require.NoError(t, err)
	require.Equal(t, "failed", result.Status)
	require.Contains(t, result.ErrorMessage, "no image url")
	require.Contains(t, result.ErrorMessage, "1027")
	require.Contains(t, result.ErrorMessage, "1028")
}

// 端点选择：有参考图走 /v1/images/edits，纯文生图走 /v1/images/generations，
// 账号凭证 video_create_path 只覆写文生图路径（渠道自定义网关场景）。
func TestMiniMaxH3ImageAdapterEndpointSelection(t *testing.T) {
	buildAccount := func(pathOverride string) *Account {
		account := &Account{ID: 1, Platform: PlatformDeepseek}
		credentials := map[string]any{
			"base_url": "https://api.minimaxi.com",
			"api_key":  "test-key",
		}
		if pathOverride != "" {
			credentials["video_create_path"] = pathOverride
		}
		account.Credentials = credentials
		return account
	}

	t.Run("text-to-image uses generations endpoint", func(t *testing.T) {
		a := NewMiniMaxH3ImageAdapter()
		served := captureH3UpstreamRequest(t, a, buildAccount(""), MediaCreateRequest{
			UpstreamModel: "minimax-h3-image",
			Prompt:        "一只猫",
		}, `{"data":[{"url":"https://cdn.example.com/a.jpg"}],"output_format":"jpeg"}`)
		require.Equal(t, "/v1/images/generations", served.path)
	})

	t.Run("image edit uses edits endpoint", func(t *testing.T) {
		a := NewMiniMaxH3ImageAdapter()
		served := captureH3UpstreamRequest(t, a, buildAccount(""), MediaCreateRequest{
			UpstreamModel: "minimax-h3-image",
			Prompt:        "改成水彩风格",
			ImageRefURLs:  []string{"https://cdn.example.com/ref.png"},
		}, `{"data":[{"url":"https://cdn.example.com/b.jpg"}],"output_format":"jpeg"}`)
		require.Equal(t, "/v1/images/edits", served.path)
	})

	t.Run("credential override rewrites text-to-image path only", func(t *testing.T) {
		a := NewMiniMaxH3ImageAdapter()
		served := captureH3UpstreamRequest(t, a, buildAccount("/proxy/v1/images/generations"), MediaCreateRequest{
			UpstreamModel: "minimax-h3-image",
			Prompt:        "一只猫",
		}, `{"data":[{"url":"https://cdn.example.com/a.jpg"}],"output_format":"jpeg"}`)
		require.Equal(t, "/proxy/v1/images/generations", served.path)
	})

	t.Run("extra override beats credential override", func(t *testing.T) {
		a := NewMiniMaxH3ImageAdapter()
		account := buildAccount("/proxy/v1/images/generations")
		served := captureH3UpstreamRequest(t, a, account, MediaCreateRequest{
			UpstreamModel: "minimax-h3-image",
			Prompt:        "一只猫",
			Extra:         map[string]any{"video_create_path": "/client/v1/images/generations"},
		}, `{"data":[{"url":"https://cdn.example.com/a.jpg"}],"output_format":"jpeg"}`)
		require.Equal(t, "/client/v1/images/generations", served.path)
	})

	t.Run("edit request body carries official images contract", func(t *testing.T) {
		a := NewMiniMaxH3ImageAdapter()
		served := captureH3UpstreamRequest(t, a, buildAccount(""), MediaCreateRequest{
			UpstreamModel: "minimax-h3-image",
			Prompt:        "把猫换成柴犬",
			ImageRefURLs:  []string{"https://cdn.example.com/cat.jpg"},
		}, `{"data":[{"url":"https://cdn.example.com/b.jpg"}],"output_format":"jpeg"}`)
		require.Equal(t, "/v1/images/edits", served.path)
		images, ok := served.body["images"].([]any)
		require.True(t, ok, "编辑请求体必须带官方 images[] 契约")
		require.Len(t, images, 1)
		first, _ := images[0].(map[string]any)
		require.Equal(t, "https://cdn.example.com/cat.jpg", first["image_url"])
		require.Equal(t, "MiniMax-H3-Image", served.body["model"])
	})
}

// h3CapturedRequest 记录捕获到的上游请求。
type h3CapturedRequest struct {
	path string
	body map[string]any
}

// captureH3UpstreamRequest 起一个本地 httptest 服务捕获 H3 adapter 实际请求的
// 路径与请求体（统一返回 200 + responseBody），返回捕获结果供断言。
func captureH3UpstreamRequest(t *testing.T, a *MiniMaxH3ImageAdapter, account *Account, req MediaCreateRequest, responseBody string) h3CapturedRequest {
	t.Helper()
	var served h3CapturedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served.path = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &served.body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(responseBody))
	}))
	t.Cleanup(server.Close)

	account.Credentials["base_url"] = server.URL
	_, err := a.Create(t.Context(), account, req)
	require.NoError(t, err)
	return served
}
