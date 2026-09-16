package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestClassifyMediaUpstreamFailure(t *testing.T) {
	tests := []struct {
		name           string
		status         int
		body           string
		wantClass      MediaUpstreamFailureClass
		wantFailover   bool
		wantCooldown   bool
		wantMinSeconds int
	}{
		{
			name:      "billing body wrapped in 400 fails over",
			status:    400,
			body:      `{"error":{"message":"Insufficient Balance"}}`,
			wantClass: MediaFailureBillingQuota, wantFailover: true, wantCooldown: true, wantMinSeconds: 30 * 60,
		},
		{
			name:      "content policy is not retried",
			status:    400,
			body:      `{"error":{"message":"content policy violation"}}`,
			wantClass: MediaFailureContentPolicy,
		},
		{
			name:      "rate limit fails over",
			status:    429,
			body:      `{"error":{"message":"Too Many Requests"}}`,
			wantClass: MediaFailureRateLimit, wantFailover: true, wantCooldown: true, wantMinSeconds: 60,
		},
		{
			name:      "validation is returned to caller",
			status:    422,
			body:      `{"error":{"message":"invalid parameter"}}`,
			wantClass: MediaFailureValidation,
		},
		{
			name:      "server error fails over",
			status:    503,
			body:      "upstream unavailable",
			wantClass: MediaFailureServer, wantFailover: true, wantCooldown: true, wantMinSeconds: 60,
		},
		{
			// DashScope 免费额度用尽：HTTP 403 + AllocationQuota.FreeTierOnly。
			// 按 auth 归类会把一个完全可用的账号冷却 10 分钟，且错误面板会误导成「密钥失效」。
			name:      "dashscope free tier exhausted 403 is billing quota not auth",
			status:    http.StatusForbidden,
			body:      `{"code":"AllocationQuota.FreeTierOnly","message":"Free quota exhausted. To continue accessing the model on a paid basis, please add funds or disable the \"use free tier only\" mode in the management console.","request_id":"6ef517f6"}`,
			wantClass: MediaFailureBillingQuota, wantFailover: true, wantCooldown: true, wantMinSeconds: 30 * 60,
		},
		{
			// 没有额度文案的裸 403 仍按鉴权失败处理，避免把密钥失效当成额度问题。
			name:      "plain 403 without quota marker stays auth",
			status:    http.StatusForbidden,
			body:      `{"error":{"message":"Forbidden"}}`,
			wantClass: MediaFailureAuth, wantFailover: true, wantCooldown: true, wantMinSeconds: 10 * 60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyMediaUpstreamFailure(tt.status, []byte(tt.body))
			if got.Class != tt.wantClass || got.ShouldFailover != tt.wantFailover || got.ShouldCooldown != tt.wantCooldown {
				t.Fatalf("got %+v, want class=%s failover=%v cooldown=%v", got, tt.wantClass, tt.wantFailover, tt.wantCooldown)
			}
			if got.Cooldown < time.Duration(tt.wantMinSeconds)*time.Second {
				t.Fatalf("cooldown = %v, want at least %vs", got.Cooldown, tt.wantMinSeconds)
			}
		})
	}
}

type mediaTempUnschedRepo struct {
	AccountRepository
	lastUntil time.Time
}

func (r *mediaTempUnschedRepo) SetTempUnschedulable(_ context.Context, _ int64, until time.Time, _ string) error {
	r.lastUntil = until
	return nil
}

func TestRecordMediaUpstreamFailure_CooldownsRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &mediaTempUnschedRepo{}
	svc := &MediaTaskService{
		rateLimitService: NewRateLimitService(repo, nil, &config.Config{}, nil, nil),
	}
	account := &Account{ID: 42, Platform: PlatformOpenAI, Name: "media-account"}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	before := time.Now()
	svc.recordMediaUpstreamFailure(c, context.Background(), account, http.StatusTooManyRequests, []byte("rate limit"), "seedance")

	require.True(t, repo.lastUntil.After(before.Add(50*time.Second)))
	require.True(t, repo.lastUntil.Before(before.Add(70*time.Second)))
}
