package handler

// media_gateway_create_outcome_test.go — 媒体创建结果收敛与错误分级的回归测试。
//
// 背景（线上事故）：客户端 POST /v1/media/generations 收到 502
// "Media generation request failed"，但任务其实已经落库、已扣费并 succeeded，
// 客户端因为没有 local_id 而永远取不回产物。原因是任务创建成功之后的幂等
// 落库/回读失败被当成创建失败上报。这里锁死两条不变量：
//  1. record 非空（任务已落库）→ 一律按成功返回，不再冒泡错误；
//  2. 真正没创建出任务时才按错误语义分级（400 / 409 / 503 / 502）。

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 回归：任务已落库时，幂等协调器的失败不能把创建结果吞掉。
func TestResolveMediaCreateOutcomeKeepsPersistedRecord(t *testing.T) {
	record := &service.MediaTaskRecord{LocalID: "img_deadbeef", Status: "succeeded", PublicModel: "qwen-image-3.0"}

	got, replayed, err := resolveMediaCreateOutcome(record, false,
		infraerrors.ServiceUnavailable("IDEMPOTENCY_STORE_UNAVAILABLE", "idempotency store unavailable"))
	require.NoError(t, err, "任务已落库就不能再报创建失败：客户端要靠 local_id 取回已付费产物")
	require.Same(t, record, got)
	require.False(t, replayed)

	// 重放路径同样以本地记录为准（回读失败不应推翻已有任务）。
	got, replayed, err = resolveMediaCreateOutcome(record, true, errors.New("decode stored response"))
	require.NoError(t, err)
	require.Same(t, record, got)
	require.True(t, replayed)

	// 没有记录才是真正的创建失败：错误必须原样冒泡（由 handler 分级映射）。
	upstreamErr := errors.New("media_task_service: upstream create: dial tcp: timeout")
	got, replayed, err = resolveMediaCreateOutcome(nil, false, upstreamErr)
	require.ErrorIs(t, err, upstreamErr)
	require.Nil(t, got)
	require.False(t, replayed)

	// 协调器声称成功却没给出记录：内部不一致必须显式报错，
	// 否则调用方会在渲染响应时解引用 nil 而 panic。
	got, _, err = resolveMediaCreateOutcome(nil, false, nil)
	require.ErrorIs(t, err, errMediaCreateRecordMissing)
	require.Nil(t, got)
}

func TestMediaCreateErrorDetail(t *testing.T) {
	cases := []struct {
		name            string
		err             error
		model           string
		wantStatus      int
		wantType        string
		wantRetryAfter  int
		wantMessagePart string
	}{
		{
			name:            "参数契约错误映射 400",
			err:             &service.MediaInvalidRequestError{Reason: "resolution 档位非法"},
			model:           "wan2.2-t2v-plus",
			wantStatus:      http.StatusBadRequest,
			wantType:        "invalid_request_error",
			wantMessagePart: "档位",
		},
		{
			name:            "选号失败映射 503 容量提示",
			err:             fmt.Errorf("%w for model %s", service.ErrNoAvailableMediaAccount, "wan2.1-image"),
			model:           "wan2.1-image",
			wantStatus:      http.StatusServiceUnavailable,
			wantType:        "capacity_error",
			wantMessagePart: "wan2.1-image",
		},
		{
			name:            "旧版裸字符串选号错误仍被识别",
			err:             errors.New("media_task_service: no available account for model wan2.1-image"),
			model:           "wan2.1-image",
			wantStatus:      http.StatusServiceUnavailable,
			wantType:        "capacity_error",
			wantMessagePart: "wan2.1-image",
		},
		{
			name:            "幂等冲突保留 409 与 Retry-After",
			err:             infraerrors.Conflict("IDEMPOTENCY_IN_PROGRESS", "idempotent request is still processing").WithMetadata(map[string]string{"retry_after": "7"}),
			wantStatus:      http.StatusConflict,
			wantType:        "invalid_request_error",
			wantRetryAfter:  7,
			wantMessagePart: "still processing",
		},
		{
			name:            "幂等存储不可用保留 503 而非 502",
			err:             infraerrors.ServiceUnavailable("IDEMPOTENCY_STORE_UNAVAILABLE", "idempotency store unavailable"),
			wantStatus:      http.StatusServiceUnavailable,
			wantType:        "api_error",
			wantMessagePart: "idempotency store unavailable",
		},
		{
			name:            "真正的上游故障才回落 502",
			err:             errors.New("media_task_service: upstream create: dial tcp: timeout"),
			wantStatus:      http.StatusBadGateway,
			wantType:        "api_error",
			wantMessagePart: "Media generation request failed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, errType, message, retryAfter := mediaCreateErrorDetail(tc.err, tc.model)
			require.Equal(t, tc.wantStatus, status)
			require.Equal(t, tc.wantType, errType)
			require.Contains(t, message, tc.wantMessagePart)
			require.Equal(t, tc.wantRetryAfter, retryAfter)
		})
	}
}
