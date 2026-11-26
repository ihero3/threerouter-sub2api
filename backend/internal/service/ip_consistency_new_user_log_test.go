package service

import (
	"bytes"
	"context"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuffer 并发安全的日志缓冲区：处罚走 goroutine，日志写入与断言可能交错。
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// captureDebugLogs 临时把默认 slog handler 换成写入缓冲区的 debug handler，
// 返回缓冲区与还原函数。这些放行日志是 debug 级别，默认 handler 会直接丢弃。
func captureDebugLogs(t *testing.T) *syncBuffer {
	t.Helper()
	buf := &syncBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return buf
}

// TestNewUserFirstUseIP_SkipReasonsLogged 锁死「每条放行路径都留下可查的 skip_reason」。
// 2026-11-26 加日志的唯一目的：新号注册后立刻换 IP 却没被拦时，能从日志里直接读出卡在哪一步。
// 判定与处罚行为本身不允许因加日志而变化（各分支返回值仍为 nil，无扣减、无打标）。
//
// 级别分流是核心契约（线上 log.level=info）：「本该拦却没拦成」的原因必须 info 级可见，
// 「规则正常放行」的原因只能是 debug，否则新号请求量会把 info 日志刷满。
func TestNewUserFirstUseIP_SkipReasonsLogged(t *testing.T) {
	inWindow := time.Now().Add(-time.Minute)

	cases := []struct {
		name      string
		userID    int64
		role      string
		clientIP  string
		register  string
		created   time.Time
		mutate    func(*IPConsistencyService, *fakePenaltyLocker)
		want      string
		wantLevel string
	}{
		{
			name: "outside window", userID: 42, role: "user",
			clientIP: "2.2.2.2", register: "1.1.1.1", created: time.Now().Add(-4 * time.Minute),
			want: ipConsistencySkipOutsideWindow, wantLevel: "DEBUG",
		},
		{
			name: "missing register ip", userID: 42, role: "user",
			clientIP: "2.2.2.2", register: "", created: inWindow,
			want: ipConsistencySkipMissingFields, wantLevel: "INFO",
		},
		{
			name: "missing client ip", userID: 42, role: "user",
			clientIP: "", register: "1.1.1.1", created: inWindow,
			want: ipConsistencySkipMissingFields, wantLevel: "INFO",
		},
		{
			name: "client ip matches register ip", userID: 42, role: "user",
			clientIP: "1.1.1.1", register: "1.1.1.1", created: inWindow,
			want: ipConsistencySkipIPMatched, wantLevel: "DEBUG",
		},
		{
			name: "admin", userID: 42, role: RoleAdmin,
			clientIP: "2.2.2.2", register: "1.1.1.1", created: inWindow,
			want: ipConsistencySkipAdmin, wantLevel: "DEBUG",
		},
		{
			name: "zero created at", userID: 42, role: "user",
			clientIP: "2.2.2.2", register: "1.1.1.1",
			want: ipConsistencySkipMissingCreatedAt, wantLevel: "INFO",
		},
		{
			name: "feature disabled", userID: 42, role: "user",
			clientIP: "2.2.2.2", register: "1.1.1.1", created: inWindow,
			mutate: func(svc *IPConsistencyService, _ *fakePenaltyLocker) {
				settings := penaltySettings()
				settings.settings.Enabled = false
				settings.settings.BalancePenaltyEnabled = false
				svc.settingsProvider = settings
			},
			want: ipConsistencySkipFeatureOff, wantLevel: "INFO",
		},
		{
			name: "whitelisted user", userID: 42, role: "user",
			clientIP: "2.2.2.2", register: "1.1.1.1", created: inWindow,
			mutate: func(svc *IPConsistencyService, _ *fakePenaltyLocker) {
				settings := penaltySettings()
				settings.settings.WhitelistUserIDs = []int64{42}
				svc.settingsProvider = settings
			},
			want: ipConsistencySkipWhitelist, wantLevel: "INFO",
		},
		{
			name: "risk exemption", userID: 42, role: "user",
			clientIP: "2.2.2.2", register: "1.1.1.1", created: inWindow,
			mutate: func(svc *IPConsistencyService, _ *fakePenaltyLocker) {
				svc.SetRiskExemptionChecker(&fakeExemptionCheck{exempt: map[int64]bool{42: true}})
			},
			want: ipConsistencySkipExemption, wantLevel: "INFO",
		},
		{
			name: "penalty dependencies not wired", userID: 42, role: "user",
			clientIP: "2.2.2.2", register: "1.1.1.1", created: inWindow,
			mutate: func(svc *IPConsistencyService, _ *fakePenaltyLocker) {
				svc.penaltyLocker = nil
			},
			want: ipConsistencySkipNoPenaltyDeps, wantLevel: "INFO",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := captureDebugLogs(t)
			svc, locker, _, marker := newNewUserFixture()
			if tc.mutate != nil {
				tc.mutate(svc, locker)
			}

			if got := svc.CheckNewUserFirstUseIP(context.Background(), tc.userID, tc.role, tc.clientIP, tc.register, tc.created); got != nil {
				t.Fatalf("pass-through branch must return nil, got %+v", got)
			}
			// 行为不变：放行分支不扣钱、不打标。
			if calls, _, _, _ := locker.snapshot(); calls != 0 {
				t.Fatalf("pass-through must not claim penalty key, got %d", calls)
			}
			if len(marker.calls) != 0 {
				t.Fatalf("pass-through must not mark risk, got %+v", marker.calls)
			}

			logs := buf.String()
			if !strings.Contains(logs, "skip_reason="+tc.want) {
				t.Fatalf("expected skip_reason=%s in logs, got:\n%s", tc.want, logs)
			}
			if !strings.Contains(logs, "level="+tc.wantLevel) {
				t.Fatalf("expected level=%s for skip_reason=%s, got:\n%s", tc.wantLevel, tc.want, logs)
			}
			if !strings.Contains(logs, "user_id="+itoa(tc.userID)) {
				t.Fatalf("skip log must carry user_id=%d, got:\n%s", tc.userID, logs)
			}
		})
	}
}

// TestNewUserFirstUseIP_AnomalousSkipsVisibleAtInfoHandler 锁死生产可见性：
// 默认 log.level=info 时，「本该拦却没拦成」的原因必须真的打出来（debug handler 之外的验证）。
func TestNewUserFirstUseIP_AnomalousSkipsVisibleAtInfoHandler(t *testing.T) {
	buf := &syncBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	svc, _, _, _ := newNewUserFixture()
	// 注册 IP 为空 = media/proxy 场景下最容易出现的「规则静默失效」原因。
	if got := svc.CheckNewUserFirstUseIP(context.Background(), 42, "user", "2.2.2.2", "", time.Now().Add(-time.Minute)); got != nil {
		t.Fatalf("missing register ip must pass through, got %+v", got)
	}
	logs := buf.String()
	if !strings.Contains(logs, ipConsistencySkipMissingFields) {
		t.Fatalf("anomalous skip must be visible at info level, got:\n%s", logs)
	}

	// IP 一致属于正常放行，info handler 下不许出现（否则按请求量刷屏）。
	buf2 := &syncBuffer{}
	slog.SetDefault(slog.New(slog.NewTextHandler(buf2, &slog.HandlerOptions{Level: slog.LevelInfo})))
	if got := svc.CheckNewUserFirstUseIP(context.Background(), 42, "user", "1.1.1.1", "1.1.1.1", time.Now().Add(-time.Minute)); got != nil {
		t.Fatalf("matching ip must pass, got %+v", got)
	}
	if strings.Contains(buf2.String(), ipConsistencySkipIPMatched) {
		t.Fatalf("normal pass must stay at debug level, got:\n%s", buf2.String())
	}
}

// TestNewUserFirstUseIP_HitLogsNoSkipReason 命中路径不应出现 skip 日志，避免排查时误读。
func TestNewUserFirstUseIP_HitLogsNoSkipReason(t *testing.T) {
	buf := captureDebugLogs(t)
	svc, _, _, _ := newNewUserFixture()

	got := svc.CheckNewUserFirstUseIP(context.Background(), 42, "user", "2.2.2.2", "1.1.1.1", time.Now().Add(-time.Minute))
	if got == nil {
		t.Fatal("mismatch within window must still hit")
	}
	if logs := buf.String(); strings.Contains(logs, "skip_reason=") {
		t.Fatalf("hit path must not emit skip_reason, got:\n%s", logs)
	}
}

func itoa(v int64) string { return strconv.FormatInt(v, 10) }
