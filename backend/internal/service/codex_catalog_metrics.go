package service

import (
	"strconv"
	"sync/atomic"

	"go.uber.org/zap"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// Codex 目录端点的低基数指标。
//
// 背景：Codex 对目录端点**没有容错**——任何非 2xx、无法解析的 body 或缺失字段都会让
// 整份自定义目录静默失效，客户端回退内置模型列表且界面不报错。因此「目录失效」在
// 用户侧表现为「我的模型全没了」，而服务端若无埋点就只能靠用户报障。
//
// 这里只统计**有界**维度，绝不把客户端自报版本、模型名等无界值当标签（会撑爆基数）：
//   - shape：目录端点交付的响应形状（models / data / none）
//   - source：目录来源（live=上游动态清单 / configured=分组显式配置 / local=本地生成兜底）
//   - platform：分组平台（枚举值，有界）
//   - entries：条目数分桶（zero / one / small / large）
//
// 关键信号是 empty_total（空目录）与 local_total（回退本地生成目录）：前者说明分组
// 没有可用模型，后者说明上游目录不可用、平台正在兜底。
const codexCatalogMetricsLogInterval = 256

var (
	codexCatalogServedTotal atomic.Uint64
	codexCatalogEmptyTotal  atomic.Uint64

	codexCatalogShapeCounts    [3]atomic.Uint64 // 索引见 codexCatalogShapeIndex
	codexCatalogSourceCounts   [5]atomic.Uint64 // 索引见 codexCatalogSourceIndex
	codexCatalogPlatformCounts = newBoundedPlatformCounters()
)

const (
	codexCatalogShapeModels = "models"
	codexCatalogShapeData   = "data"
	codexCatalogShapeNone   = "none"
)

const (
	codexCatalogSourceLive       = "live"
	codexCatalogSourceConfigured = "configured"
	codexCatalogSourceLocal      = "local"
	codexCatalogSourceEmpty      = "empty"
	codexCatalogSourceOther      = "other"
)

// 以下导出常量供 handler 层在真正写响应时上报来源：指标维度必须由「实际交付了
// 哪条分支」决定，集中在写响应的位置调用才不会漏记。
const (
	CodexCatalogShapeModels = codexCatalogShapeModels

	CodexCatalogSourceLive       = codexCatalogSourceLive
	CodexCatalogSourceConfigured = codexCatalogSourceConfigured
	CodexCatalogSourceLocal      = codexCatalogSourceLocal
)

// codexCatalogPlatforms 是已知平台集合：不在集合内的平台一律归到 "other"，
// 避免未来新增平台字符串把指标基数顶上去。
var codexCatalogPlatforms = []string{
	PlatformOpenAI,
	PlatformComposite,
	PlatformAnthropic,
	PlatformGemini,
	PlatformGrok,
	PlatformKimi,
	PlatformZhipu,
	PlatformDeepseek,
	PlatformAntigravity,
}

const codexCatalogPlatformOther = "other"

func codexCatalogShapeIndex(shape string) int {
	switch shape {
	case codexCatalogShapeModels:
		return 0
	case codexCatalogShapeData:
		return 1
	default:
		return 2
	}
}

func codexCatalogSourceIndex(source string) int {
	switch source {
	case codexCatalogSourceLive:
		return 0
	case codexCatalogSourceConfigured:
		return 1
	case codexCatalogSourceLocal:
		return 2
	case codexCatalogSourceEmpty:
		return 3
	default:
		return 4
	}
}

func newBoundedPlatformCounters() map[string]*atomic.Uint64 {
	counters := make(map[string]*atomic.Uint64, len(codexCatalogPlatforms)+1)
	for _, platform := range codexCatalogPlatforms {
		counters[platform] = &atomic.Uint64{}
	}
	counters[codexCatalogPlatformOther] = &atomic.Uint64{}
	return counters
}

func boundedCodexCatalogPlatform(platform string) string {
	for _, known := range codexCatalogPlatforms {
		if known == platform {
			return platform
		}
	}
	return codexCatalogPlatformOther
}

// codexCatalogEntryBucket 把条目数压成有界分桶，避免按条目数打标签。
func codexCatalogEntryBucket(entries int) string {
	switch {
	case entries <= 0:
		return "zero"
	case entries == 1:
		return "one"
	case entries <= 32:
		return "small"
	default:
		return "large"
	}
}

// RecordCodexCatalogServed 记录一次目录端点交付。
//
// entries 为最终交付给客户端的条目数；source 取 codexCatalogSource* 常量。
// 空目录会额外计入 empty_total（客户端会因此退化到内置模型列表）。
func RecordCodexCatalogServed(shape, source, platform string, entries int) {
	if shape == "" {
		shape = codexCatalogShapeNone
	}
	if source == "" {
		source = codexCatalogSourceOther
	}
	codexCatalogServedTotal.Add(1)
	codexCatalogShapeCounts[codexCatalogShapeIndex(shape)].Add(1)
	codexCatalogSourceCounts[codexCatalogSourceIndex(source)].Add(1)
	platform = boundedCodexCatalogPlatform(platform)
	if counter, ok := codexCatalogPlatformCounts[platform]; ok {
		counter.Add(1)
	}
	if entries <= 0 {
		codexCatalogEmptyTotal.Add(1)
	}

	// 节流汇总日志：单次请求不打日志（目录是高频端点），每 N 次输出一份快照。
	if codexCatalogServedTotal.Load()%codexCatalogMetricsLogInterval != 0 {
		return
	}
	snapshot := codexCatalogMetricsSnapshot()
	logger.L().With(
		zap.String("component", "service.codex_catalog_metrics"),
		zap.Uint64("served_total", codexCatalogServedTotal.Load()),
		zap.Uint64("empty_total", codexCatalogEmptyTotal.Load()),
		zap.Uint64("shape_models_total", codexCatalogShapeCounts[0].Load()),
		zap.Uint64("shape_data_total", codexCatalogShapeCounts[1].Load()),
		zap.Uint64("source_live_total", codexCatalogSourceCounts[0].Load()),
		zap.Uint64("source_configured_total", codexCatalogSourceCounts[1].Load()),
		zap.Uint64("source_local_total", codexCatalogSourceCounts[2].Load()),
		zap.String("platform_total", snapshot["platforms"]),
		zap.String("last", snapshot["last"]),
	).Info("codex.catalog_metrics")
}

// codexCatalogMetricsSnapshot 返回便于日志/测试断言的有界字符串快照。
func codexCatalogMetricsSnapshot() map[string]string {
	platforms := ""
	for _, platform := range codexCatalogPlatforms {
		if counter, ok := codexCatalogPlatformCounts[platform]; ok && counter.Load() > 0 {
			if platforms != "" {
				platforms += ","
			}
			platforms += platform + "=" + strconv.FormatUint(counter.Load(), 10)
		}
	}
	return map[string]string{
		"platforms": platforms,
		"last":      strconv.FormatUint(codexCatalogServedTotal.Load(), 10),
	}
}

// CodexCatalogMetricsSnapshotForTest 暴露只读快照，供其它包的集成测试断言埋点是否
// 真的被触发（Go 的包级 var 无法跨包读取，因此这里提供最小只读入口）。
type CodexCatalogMetricsSnapshot struct {
	Served uint64
	Empty  uint64
	Kimi   uint64
}

func CodexCatalogMetricsSnapshotForTest() CodexCatalogMetricsSnapshot {
	kimi := uint64(0)
	if counter, ok := codexCatalogPlatformCounts[PlatformKimi]; ok {
		kimi = counter.Load()
	}
	return CodexCatalogMetricsSnapshot{
		Served: codexCatalogServedTotal.Load(),
		Empty:  codexCatalogEmptyTotal.Load(),
		Kimi:   kimi,
	}
}
