package handler

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 幂等键来源优先级：Idempotency-Key 头 > 请求体 request_id。
// 视频创建与同步生图共用这条解析，两者必须表现一致，否则同一个客户端
// 在 /v1/images/generations 与 /v1/videos/generations 上行为不同。
func TestResolveMediaIdempotencyKey(t *testing.T) {
	cases := []struct {
		name      string
		headerKey string
		body      map[string]any
		want      string
	}{
		{
			name:      "header wins over body request_id",
			headerKey: " hdr-1 ",
			body:      map[string]any{"request_id": "body-1"},
			want:      "hdr-1",
		},
		{
			name:      "body request_id used when header missing",
			headerKey: "",
			body:      map[string]any{"request_id": " body-2 "},
			want:      "body-2",
		},
		{
			name:      "no key provided stays legacy",
			headerKey: "  ",
			body:      map[string]any{"prompt": "a cat"},
			want:      "",
		},
		{
			name:      "nil body is safe",
			headerKey: "",
			body:      nil,
			want:      "",
		},
		{
			name:      "non-string request_id ignored",
			headerKey: "",
			body:      map[string]any{"request_id": 12345},
			want:      "",
		},
		{
			name:      "blank body request_id ignored",
			headerKey: "",
			body:      map[string]any{"request_id": "   "},
			want:      "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, resolveMediaIdempotencyKey(tc.headerKey, tc.body))
		})
	}
}
