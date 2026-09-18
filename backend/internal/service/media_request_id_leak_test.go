package service

import (
	"strings"
	"testing"
)

// request_id 是本网关的幂等标识（客户端可用它反查任务），不是任何上游厂商的参数。
// 它由 handler 从请求体里读出来后会留在 MediaCreateRequest.Extra 中一路往下传，
// 一旦漏进上游请求体：官方 API 会因未知参数整单拒绝，宽松的厂商则会把它当成
// 未知字段静默忽略并可能出现告警噪声。这里逐个适配器兜底，防止以后新增字段时漏掉。
func TestMediaAdaptersNeverForwardRequestID(t *testing.T) {
	const marker = "gw-request-id-should-not-leak"
	extra := func() map[string]any {
		return map[string]any{
			"request_id": marker,
			// 顺带确认排除逻辑没有把整个 Extra 都丢掉
			"custom_field": "keep-me",
		}
	}

	mediaReq := func() MediaCreateRequest {
		return MediaCreateRequest{
			PublicModel:   "x-image",
			UpstreamModel: "image-01",
			Prompt:        "画一只猫",
			ImageRefURLs:  []string{"https://cdn.example.com/ref.png"},
			Extra:         extra(),
		}
	}
	videoReq := func() VideoCreateRequest {
		return VideoCreateRequest{
			PublicModel:   "x-video",
			UpstreamModel: "video-01",
			Prompt:        "镜头推进",
			ImageRefURLs:  []string{"https://cdn.example.com/ref.png"},
			Extra:         extra(),
		}
	}

	cases := []struct {
		name  string
		build func() []byte
	}{
		{"seedance-image", func() []byte { return buildSeedanceImageCreateBody(mediaReq()) }},
		{"wan-image", func() []byte { return buildWanImageCreateBody(mediaReq()) }},
		{"minimax-image", func() []byte { return buildMiniMaxImageCreateBody(mediaReq()) }},
		{"generic-video", func() []byte { return buildVideoCreateBody(videoReq()) }},
		{"seedance-video", func() []byte { return buildSeedanceVideoCreateBody(videoReq()) }},
		{"minimax-video", func() []byte { return buildMiniMaxVideoCreateBody(videoReq()) }},
		{"wan-video", func() []byte { return buildWanVideoCreateBody(videoReq()) }},
		{"minimax-tts", func() []byte { return buildMiniMaxTTSBody(mediaReq()) }},
		{"volcano-tts", func() []byte { return buildVolcanoTTSBody(mediaReq()) }},
		{"aliyun-tts", func() []byte { return buildAliyunTTSBody(mediaReq()) }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := tc.build()
			if len(body) == 0 {
				t.Fatalf("构建结果为空")
			}
			if strings.Contains(string(body), marker) {
				t.Fatalf("request_id 泄漏到上游请求体: %s", body)
			}
			if !strings.Contains(string(body), "keep-me") {
				t.Fatalf("未识别字段不应被丢弃（排除逻辑收窄过头了）: %s", body)
			}
		})
	}
}
