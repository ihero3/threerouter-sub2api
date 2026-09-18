package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// TestSanitizeGrokMediaForwardBodyDropsClientRequestID 覆盖「客户端幂等键不上行」。
//
// request_id 是本网关的幂等键（见 docs/ASYNC_IMAGE_TASKS.md），xAI 的请求 schema
// 里没有这个字段：透传等于把调用方的私有标识交给上游，而未知顶层参数会不会被
// 上游判非法取决于其校验严格程度，网关侧不该让客户端承担这个不确定性。
// 这里锁住「删掉 request_id、其余字段一个不少」。
func TestSanitizeGrokMediaForwardBodyDropsClientRequestID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		endpoint  GrokMediaEndpoint
		body      string
		wantModel string
		wantKept  []string
	}{
		{
			name:      "image generation",
			endpoint:  GrokMediaEndpointImagesGenerations,
			body:      `{"request_id":"client-key-1","model":"grok-imagine","prompt":"a cat","n":2}`,
			wantModel: "grok-imagine",
			wantKept:  []string{"prompt", "n"},
		},
		{
			name:      "image edit",
			endpoint:  GrokMediaEndpointImagesEdits,
			body:      `{"request_id":"client-key-2","model":"grok-imagine","prompt":"restyle","image":{"url":"https://cdn.test/a.png"}}`,
			wantModel: "grok-imagine",
			wantKept:  []string{"prompt", "image.url"},
		},
		{
			name:      "video generation",
			endpoint:  GrokMediaEndpointVideosGenerations,
			body:      `{"request_id":"client-key-3","model":"grok-imagine-video","prompt":"waves","duration":6,"resolution":"720p"}`,
			wantModel: "grok-imagine-video",
			wantKept:  []string{"prompt", "duration", "resolution"},
		},
		{
			name:      "video edit",
			endpoint:  GrokMediaEndpointVideosEdits,
			body:      `{"request_id":"client-key-4","model":"grok-imagine-video","prompt":"extend","video":{"url":"https://cdn.test/v.mp4"}}`,
			wantModel: "grok-imagine-video",
			wantKept:  []string{"prompt", "video.url"},
		},
		{
			name:      "video extension",
			endpoint:  GrokMediaEndpointVideosExtensions,
			body:      `{"request_id":"client-key-5","model":"grok-imagine-video","video":{"url":"https://cdn.test/v.mp4"}}`,
			wantModel: "grok-imagine-video",
			wantKept:  []string{"video.url"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			out, contentType, err := sanitizeGrokMediaForwardBody(tc.endpoint, []byte(tc.body), "application/json")
			require.NoError(t, err)
			require.Equal(t, "application/json", contentType)

			require.False(t, gjson.GetBytes(out, "request_id").Exists(),
				"客户端幂等键不得进入上行 body：%s", string(out))
			require.Equal(t, tc.wantModel, gjson.GetBytes(out, "model").String())
			for _, path := range tc.wantKept {
				require.True(t, gjson.GetBytes(out, path).Exists(),
					"字段 %s 被误删：%s", path, string(out))
			}
		})
	}
}

// TestSanitizeGrokMediaForwardBodyKeepsNestedRequestID 保证只删顶层幂等键，
// 嵌在上游对象里的 request_id（属于上游任务引用语义）保持原样。
func TestSanitizeGrokMediaForwardBodyKeepsNestedRequestID(t *testing.T) {
	t.Parallel()

	out, _, err := sanitizeGrokMediaForwardBody(
		GrokMediaEndpointVideosGenerations,
		[]byte(`{"request_id":"client-key","model":"grok-imagine-video","video":{"request_id":"upstream-ref"}}`),
		"application/json",
	)
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(out, "request_id").Exists())
	require.Equal(t, "upstream-ref", gjson.GetBytes(out, "video.request_id").String())
}

// TestForwardGrokMediaNeverSendsClientRequestIDUpstream 是端到端兜底：
// 走完 ForwardGrokMedia 的完整剔除链后，上游实际收到的 body 里没有 request_id。
// 只覆盖函数级剔除不够 —— 中间任何一步把字段写回来都会被这条测试抓住。
func TestForwardGrokMediaNeverSendsClientRequestIDUpstream(t *testing.T) {
	t.Setenv(xai.EnvAllowUnsafeURLOverrides, "true")
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name     string
		endpoint GrokMediaEndpoint
		path     string
		wantURL  string
		// respBody 必须让该端点认得出产物：图片端点没有图像输出会被判 502。
		respBody string
	}{
		{"image generation", GrokMediaEndpointImagesGenerations, "/v1/images/generations", "https://xai.test/v1/images/generations", `{"data":[{"url":"https://cdn.test/out.png"}]}`},
		{"image edit", GrokMediaEndpointImagesEdits, "/v1/images/edits", "https://xai.test/v1/images/edits", `{"data":[{"url":"https://cdn.test/out.png"}]}`},
		{"video generation", GrokMediaEndpointVideosGenerations, "/v1/videos/generations", "https://xai.test/v1/videos/generations", `{"request_id":"upstream-request"}`},
		{"video edit", GrokMediaEndpointVideosEdits, "/v1/videos/edits", "https://xai.test/v1/videos/edits", `{"request_id":"upstream-request"}`},
		{"video extension", GrokMediaEndpointVideosExtensions, "/v1/videos/extensions", "https://xai.test/v1/videos/extensions", `{"request_id":"upstream-request"}`},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			body := []byte(`{"request_id":"client-key-e2e","model":"grok-imagine-video","prompt":"waves"}`)
			c.Request = httptest.NewRequest(http.MethodPost, tc.path, bytes.NewReader(body))
			c.Request.Header.Set("Content-Type", "application/json")

			account := &Account{
				ID:          63,
				Name:        "grok",
				Platform:    PlatformGrok,
				Type:        AccountTypeAPIKey,
				Concurrency: 1,
				Credentials: map[string]any{
					"api_key":  "api-key",
					"base_url": "https://xai.test/v1",
				},
			}
			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(tc.respBody)),
			}}
			svc := &OpenAIGatewayService{httpUpstream: upstream}

			_, err := svc.ForwardGrokMedia(context.Background(), c, account, tc.endpoint, "", body, "application/json")
			require.NoError(t, err)
			require.Equal(t, tc.wantURL, upstream.lastReq.URL.String())
			require.False(t, gjson.GetBytes(upstream.lastBody, "request_id").Exists(),
				"上游收到了客户端幂等键：%s", string(upstream.lastBody))
			require.Equal(t, "client-key-e2e", gjson.GetBytes(body, "request_id").String(),
				"原始请求体不能被就地修改")
		})
	}
}
