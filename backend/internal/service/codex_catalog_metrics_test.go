package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 目录端点指标必须只带**有界**维度：客户端自报版本、模型名等无界值绝不能成为标签，
// 否则基数会随流量无限增长。这里锁住分桶与平台收敛行为。
func TestRecordCodexCatalogServedKeepsDimensionsBounded(t *testing.T) {
	beforeServed := codexCatalogServedTotal.Load()
	beforeEmpty := codexCatalogEmptyTotal.Load()

	// 未知平台（含看起来像版本号/模型名的值）必须收敛到 other 桶。
	RecordCodexCatalogServed(CodexCatalogShapeModels, CodexCatalogSourceLive, "0.162.0-alpha.17.2", 7)
	RecordCodexCatalogServed(CodexCatalogShapeModels, CodexCatalogSourceLive, "some-new-platform-2026", 7)

	require.Equal(t, beforeServed+2, codexCatalogServedTotal.Load())
	require.Equal(t, beforeEmpty, codexCatalogEmptyTotal.Load(), "非空目录不应计入 empty_total")
	require.Equal(t, uint64(2), codexCatalogPlatformCounts[codexCatalogPlatformOther].Load())
	require.Equal(t, uint64(0), codexCatalogPlatformCounts[PlatformOpenAI].Load())

	// 空目录必须计入 empty_total：客户端会因此退化到内置模型列表，这是要报警的信号。
	RecordCodexCatalogServed(CodexCatalogShapeModels, CodexCatalogSourceLocal, PlatformDeepseek, 0)
	require.Equal(t, beforeEmpty+1, codexCatalogEmptyTotal.Load())
	require.Equal(t, uint64(1), codexCatalogPlatformCounts[PlatformDeepseek].Load())

	// 未知形状与来源不得 panic，且仍计入总量。
	RecordCodexCatalogServed("", "", PlatformOpenAI, -1)
	require.Equal(t, beforeServed+4, codexCatalogServedTotal.Load())
	require.Equal(t, uint64(1), codexCatalogShapeCounts[codexCatalogShapeIndex(codexCatalogShapeNone)].Load())
	require.Equal(t, uint64(1), codexCatalogSourceCounts[codexCatalogSourceIndex(codexCatalogSourceOther)].Load())
	require.Equal(t, beforeEmpty+2, codexCatalogEmptyTotal.Load(), "条目数未知(-1)按空处理更安全")

	// 每个维度桶都必须是固定的有界集合（长度断言不能走 require.Len：那会复制
	// 含 sync/atomic.noCopy 的值，go vet 会报 copylocks）。
	if n := len(codexCatalogShapeCounts); n != 3 {
		t.Fatalf("shape buckets: got %d, want 3", n)
	}
	if n := len(codexCatalogSourceCounts); n != 5 {
		t.Fatalf("source buckets: got %d, want 5", n)
	}
	if n := len(codexCatalogPlatformCounts); n != len(codexCatalogPlatforms)+1 {
		t.Fatalf("platform buckets: got %d, want %d", n, len(codexCatalogPlatforms)+1)
	}
}

func TestCodexCatalogEntryBucket(t *testing.T) {
	require.Equal(t, "zero", codexCatalogEntryBucket(0))
	require.Equal(t, "zero", codexCatalogEntryBucket(-1))
	require.Equal(t, "one", codexCatalogEntryBucket(1))
	require.Equal(t, "small", codexCatalogEntryBucket(32))
	require.Equal(t, "large", codexCatalogEntryBucket(33))
}
