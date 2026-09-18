package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// 本文件锁住视频轮询结果与音频同步结果的交付口径，与图片侧
// media_image_result_test.go 同源：上游报"成功"却没给产物时，
// 服务层会立刻真实扣费，而调用方什么都拿不到。

func TestVideoQueryWithoutURLStaysProcessing(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		call func([]byte, int) (*VideoTaskResult, error)
		body string
	}{
		{
			name: "seedance",
			call: parseSeedanceVideoQueryResult,
			body: `{"status":"succeeded"}`,
		},
		{
			name: "minimax",
			call: parseMiniMaxVideoQueryResult,
			body: `{"status":"SUCCESS"}`,
		},
		{
			name: "wan",
			call: parseWanVideoQueryResult,
			body: `{"output":{"task_status":"SUCCEEDED"}}`,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			res, err := tc.call([]byte(tc.body), http.StatusOK)
			require.NoError(t, err)
			// 上游报成功但没给 URL 时不能算完成：一旦算完成就会立即真实扣费，
			// 而调用方拿不到视频。降级为 processing，由超时兜底判失败退预扣。
			require.Equal(t, "processing", res.Status)
			require.Empty(t, res.VideoURL)
		})
	}
}

func TestVideoQueryWithURLKeepsSucceeded(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		call func([]byte, int) (*VideoTaskResult, error)
		body string
	}{
		{
			name: "seedance",
			call: parseSeedanceVideoQueryResult,
			body: `{"status":"succeeded","content":{"video_url":"https://cdn.test/v.mp4"}}`,
		},
		{
			name: "minimax",
			call: parseMiniMaxVideoQueryResult,
			body: `{"status":"SUCCESS","task":{"content":{"url":"https://cdn.test/v.mp4"}}}`,
		},
		{
			name: "wan",
			call: parseWanVideoQueryResult,
			body: `{"output":{"task_status":"SUCCEEDED","video_url":"https://cdn.test/v.mp4"}}`,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			res, err := tc.call([]byte(tc.body), http.StatusOK)
			require.NoError(t, err)
			require.Equal(t, "succeeded", res.Status)
			require.Equal(t, "https://cdn.test/v.mp4", res.VideoURL)
		})
	}
}

func TestAudioResultRejectsEmptyOutput(t *testing.T) {
	t.Parallel()

	// 三家 TTS 都是同步终态：服务层看到 succeeded 会立刻真实扣费，
	// 没有产物就必须判失败，否则"扣了钱没有声音"。
	cases := []struct {
		name string
		call func([]byte, int) (*MediaCreateResult, error)
		body string
	}{
		{name: "minimax", call: parseMiniMaxTTSResult, body: `{"data":{}}`},
		{name: "volcano", call: parseVolcanoTTSResult, body: `{"code":3000}`},
		{name: "aliyun", call: parseAliyunTTSResult, body: `{"output":{}}`},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			res, err := tc.call([]byte(tc.body), http.StatusOK)
			require.NoError(t, err)
			require.Equal(t, "failed", res.Status)
			require.Equal(t, MediaCompletionFailed, res.Mode)
			require.Empty(t, res.InlineURL)
		})
	}
}

func TestAudioResultKeepsUsableOutput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		call func([]byte, int) (*MediaCreateResult, error)
		body string
		want string
	}{
		{name: "minimax", call: parseMiniMaxTTSResult,
			body: `{"data":{"audio":"https://cdn.test/a.mp3"}}`, want: "https://cdn.test/a.mp3"},
		{name: "volcano", call: parseVolcanoTTSResult,
			body: `{"data":"https://cdn.test/a.mp3"}`, want: "https://cdn.test/a.mp3"},
		{name: "aliyun", call: parseAliyunTTSResult,
			body: `{"output":{"audio_url":"https://cdn.test/a.mp3"}}`, want: "https://cdn.test/a.mp3"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			res, err := tc.call([]byte(tc.body), http.StatusOK)
			require.NoError(t, err)
			require.Equal(t, "succeeded", res.Status)
			require.Equal(t, tc.want, res.InlineURL)
		})
	}
}
