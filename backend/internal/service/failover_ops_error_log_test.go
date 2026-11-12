package service

import (
	"context"
	"sync"
	"testing"
)

// failoverOpsErrorRecorderStub 是 opsErrorRecorder 的测试替身，记录调用次数与最近一次入参。
type failoverOpsErrorRecorderStub struct {
	mu        sync.Mutex
	calls     int
	lastEntry *OpsInsertErrorLogInput
}

func (s *failoverOpsErrorRecorderStub) RecordError(_ context.Context, entry *OpsInsertErrorLogInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.lastEntry = entry
	return nil
}

// withFailoverOpsRecorder 把 stub 注入进程级单例 failoverOpsErrorRecorder，返回清空函数。
// 必须在测试结束/其他用例前恢复，避免跨用例污染（recordFailoverOpsError 仅对 5xx 生效，
// 但单例是包级共享状态，必须妥善复位）。
func withFailoverOpsRecorder(stub *failoverOpsErrorRecorderStub) func() {
	failoverOpsErrorRecorder.Store(stub)
	return func() {
		// 复位为 typed-nil，保证后续 Load 返回非 panic 的空接口（值为 nil）。
		failoverOpsErrorRecorder.Store((*failoverOpsErrorRecorderStub)(nil))
	}
}

func TestMaybeDisableAccountOnFailover_Upstream503_RecordsOpsErrorLog(t *testing.T) {
	stub := &failoverOpsErrorRecorderStub{}
	defer withFailoverOpsRecorder(stub)()

	repo := &failoverDisableRepoStub{acct: &Account{ID: 42, Status: StatusActive, Schedulable: true, Platform: "openai"}}
	maybeDisableAccountOnFailover(context.Background(), newFailoverDisableSettingService(true), repo,
		repo.acct, 503, nil, alwaysFailover, "Service Unavailable", []byte(`{"error":"Service Unavailable"}`))

	if stub.calls != 1 {
		t.Fatalf("expected exactly 1 Ops error log on upstream 503, got %d", stub.calls)
	}
	entry := stub.lastEntry
	if entry == nil {
		t.Fatal("expected non-nil Ops error log entry")
	}
	if entry.AccountID == nil || *entry.AccountID != 42 {
		t.Errorf("expected AccountID=42, got %v", entry.AccountID)
	}
	if entry.Platform != "openai" {
		t.Errorf("expected Platform=openai, got %q", entry.Platform)
	}
	if entry.StatusCode != 503 {
		t.Errorf("expected StatusCode=503, got %d", entry.StatusCode)
	}
	if entry.UpstreamStatusCode == nil || *entry.UpstreamStatusCode != 503 {
		t.Errorf("expected UpstreamStatusCode=503, got %v", entry.UpstreamStatusCode)
	}
	if entry.ErrorPhase != "upstream" || entry.ErrorType != "upstream_error" {
		t.Errorf("expected ErrorPhase=upstream/ErrorType=upstream_error, got %q/%q", entry.ErrorPhase, entry.ErrorType)
	}
	if entry.UpstreamErrorMessage == nil || *entry.UpstreamErrorMessage == "" {
		t.Errorf("expected non-empty UpstreamErrorMessage, got %v", entry.UpstreamErrorMessage)
	}
	// 同时确认账号被自动停用（原有副作用未受影响）。
	if repo.updated == nil || repo.updated.Status != StatusError {
		t.Errorf("expected account to be auto-disabled on 503, got updated=%v", repo.updated)
	}
}

// TestMaybeDisableAccountOnFailover_Upstream503_DisableOffStillLogs 验证需求 B 的核心：
// 即使 "Disable Failed Account on Failover" 关闭、账号未被停用，上游 503 仍要写一条 Ops 错误日志
// （此前换号成功/开关关闭时后台错误日志完全无记录）。
func TestMaybeDisableAccountOnFailover_Upstream503_DisableOffStillLogs(t *testing.T) {
	stub := &failoverOpsErrorRecorderStub{}
	defer withFailoverOpsRecorder(stub)()

	repo := &failoverDisableRepoStub{acct: &Account{ID: 7, Status: StatusActive, Schedulable: true, Platform: "anthropic"}}
	maybeDisableAccountOnFailover(context.Background(), newFailoverDisableSettingService(false), repo,
		repo.acct, 503, nil, alwaysFailover, "upstream 503", []byte(`{"error":"upstream 503"}`))

	if stub.calls != 1 {
		t.Fatalf("expected exactly 1 Ops error log even when disable is off, got %d", stub.calls)
	}
	if stub.lastEntry == nil || stub.lastEntry.AccountID == nil || *stub.lastEntry.AccountID != 7 {
		t.Errorf("expected Ops error log for account 7, got %+v", stub.lastEntry)
	}
	// 关闭开关时不应触发账号停用。
	if repo.updated != nil {
		t.Errorf("expected no account update when disable is off, got %+v", repo.updated)
	}
}

// TestMaybeDisableAccountOnFailover_RelayTempCooldown_RecordsOpsErrorLog 验证 relay 中转站账号
// 命中上游 503 走临时冷却分支时，同样要写一条 Ops 错误日志（用户明确要求临时冷却分支也记）。
func TestMaybeDisableAccountOnFailover_RelayTempCooldown_RecordsOpsErrorLog(t *testing.T) {
	stub := &failoverOpsErrorRecorderStub{}
	defer withFailoverOpsRecorder(stub)()

	relayAcct := &Account{
		ID:         99,
		Status:     StatusActive,
		Schedulable: true,
		Platform:   "openai",
		Extra:      map[string]interface{}{AccountExtraUpstreamRelay: true},
	}
	repo := &failoverDisableRepoStub{acct: relayAcct}
	maybeDisableAccountOnFailover(context.Background(), newFailoverDisableSettingService(true), repo,
		relayAcct, 503, nil, alwaysFailover, "bad gateway", []byte(`{"error":"502"}`))

	if stub.calls != 1 {
		t.Fatalf("expected exactly 1 Ops error log for relay temp-cooldown path, got %d", stub.calls)
	}
	if !repo.tempCalled {
		t.Errorf("expected relay account to be temp-cooldowned, but SetTempUnschedulable was not called")
	}
	if repo.tempID != 99 {
		t.Errorf("expected temp cooldown for account 99, got id=%d", repo.tempID)
	}
}

// TestMaybeDisableAccountOnFailover_Non5xx_NotLogged 反向验证：非 5xx（如 429 限流）不应写
// Ops 错误日志，避免把客户端错误误记为上游服务器故障。
func TestMaybeDisableAccountOnFailover_Non5xx_NotLogged(t *testing.T) {
	stub := &failoverOpsErrorRecorderStub{}
	defer withFailoverOpsRecorder(stub)()

	repo := &failoverDisableRepoStub{acct: &Account{ID: 3, Status: StatusActive, Schedulable: true, Platform: "openai"}}
	maybeDisableAccountOnFailover(context.Background(), newFailoverDisableSettingService(true), repo,
		repo.acct, 429, nil, alwaysFailover, "rate limited", []byte(`{"error":"rate limited"}`))

	if stub.calls != 0 {
		t.Fatalf("expected no Ops error log for 429 (non-5xx), got %d", stub.calls)
	}
}
