package service

import (
	"context"
	"fmt"
	"time"
)

// media_task_wait.go — 媒体任务的同步等待能力。
//
// 背景：/v1/images/generations 在 OpenAI 语义下是**同步**端点，客户端发一次请求
// 就要拿到图；而 /v1/videos/generations 是异步任务，客户端必须自己写轮询。
// 统一媒体链路（media_tasks）内部两者共用同一张表与同一套结算，因此只要在
// 任务层补一个"有上限的等待"，就能让同一个实现同时服务两种语义：
//   - 图片：默认等待到终态，直接回图（OpenAI 兼容）
//   - 视频/媒体：默认不等待（保持现有异步行为），客户端带 ?wait=N 时才等待
//
// 等待只影响"响应里带回来的任务状态"，不改变计费与落库：任务一旦创建成功，
// 结算与 Worker 轮询照常进行，客户端提前断开也不会丢任务或丢费用。

const (
	// mediaTaskWaitPollInterval 轮询上游状态的间隔。
	mediaTaskWaitPollInterval = 2 * time.Second
	// mediaTaskWaitMinTimeout 是 ?wait 允许的最小值语义：传了但小于 1 秒视为不等待。
	mediaTaskWaitMinTimeout = time.Second
)

// MaxMediaTaskWait is the upper bound accepted for the "wait" query parameter.
// 上限存在的意义是防止客户端用 ?wait=3600 把连接挂死：媒体生成上游本身可能
// 几分钟不返回，长连接堆积会先拖垮网关，而不是先拖垮上游。
const MaxMediaTaskWait = 180 * time.Second

// AwaitTerminal polls the task until it reaches a terminal state or the timeout
// elapses. It always returns the latest known record, so callers can render a
// partial result instead of failing the request.
//
// ctx 取消（客户端断开）时立即返回当前记录：等待只是"尽量多拿一点结果"，
// 此时上游任务与结算仍然继续，不需要跟着客户端一起放弃。
func (s *MediaTaskService) AwaitTerminal(ctx context.Context, localID string, userID int64, timeout time.Duration) (*MediaTaskRecord, error) {
	if s == nil {
		return nil, fmt.Errorf("media_task_service: await terminal: service is nil")
	}
	// GetTask 内部自带一次状态刷新，所以即使 timeout <= 0 也会刷新一次。
	record, err := s.GetTask(ctx, localID, userID)
	if err != nil {
		return nil, fmt.Errorf("media_task_service: await terminal: %w", err)
	}
	if timeout < mediaTaskWaitMinTimeout {
		return record, nil
	}
	deadline := time.Now().Add(timeout)
	// 用"未到终态"而不是"等于 processing"作为循环条件：状态集合若将来新增
	// （如 pending / queued），这里不会因为字符串比较漏掉而退化成"完全不等待"。
	// 用 ticker 而不是每次 time.After：后者每轮新建一个 timer，长等待下会堆积。
	ticker := time.NewTicker(mediaTaskWaitPollInterval)
	defer ticker.Stop()
	for !IsMediaTaskTerminal(record.Status) && time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return record, nil
		case <-ticker.C:
		}
		refreshed, refreshErr := s.GetTask(ctx, localID, userID)
		if refreshErr != nil || refreshed == nil {
			// 轮询失败（上游抖动/DB 抖动）不该让整单失败：保留上一次已知状态。
			return record, nil
		}
		record = refreshed
	}
	return record, nil
}

// IsMediaTaskTerminal reports whether the status is a final one.
func IsMediaTaskTerminal(status string) bool {
	switch status {
	case "succeeded", "failed", "cancelled":
		return true
	default:
		return false
	}
}
