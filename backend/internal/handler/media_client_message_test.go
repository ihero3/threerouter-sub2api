package handler

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMediaTaskClientErrorNeverLeaksUpstreamText 锁死媒体/视频任务对调用方的错误文案。
//
// media_vendor_image_adapter.go 在「HTTP 200 带业务错误码」等分支会把上游
// message / status_msg 拼进 ErrorMessage 落库（运营排障需要，保留在 UpstreamRaw
// 与库内供排障），但 media_gateway_handler.go / video_gateway_handler.go 原样
// 把 record.ErrorMessage 塞进 response.Error 返回终端用户，与
// media_upstream_failure.go「媒体链路对调用方一律用平台统一话术」冲突。
func TestMediaTaskClientErrorNeverLeaksUpstreamText(t *testing.T) {
	for _, file := range []string{"media_gateway_handler.go", "video_gateway_handler.go"} {
		t.Run(file, func(t *testing.T) {
			src, err := os.ReadFile(file)
			require.NoError(t, err)
			body := string(src)
			require.NotContains(t, body, "response.Error = record.ErrorMessage",
				"客户端可见错误不得直接回显落库的上游原文")
			require.Contains(t, body, "service.MediaUpstreamClientMessage(",
				"必须改用平台统一文案")
		})
	}
}
