package service

// minimax_tts_store.go — MiniMax 异步任务 / 上传文件的账号归属存储契约。
//
// MiniMax 的 file_id / task_id 是 int64，客户端（含官方 SDK）会按数字解析。
// 为了保持原生类型不变，网关不能把它们换成字符串 token；改为在 Redis 里保存
// 「上游数字 ID -> 账号/用户/计费上下文」的短期映射。查询与下载只接受数字 ID，
// 通过映射找回归属账号，再校验用户/API Key，避免跨账号或跨用户访问。
//
// 存储由 repository 层用 Redis 实现（service 按 depguard 规则不直接 import Redis）。

import (
	"context"
	"time"
)

// MiniMaxFileBinding 记录一个上游 file_id 的归属与用途。
type MiniMaxFileBinding struct {
	AccountID int64  `json:"account_id"`
	UserID    int64  `json:"user_id"`
	APIKeyID  int64  `json:"api_key_id"`
	Purpose   string `json:"purpose,omitempty"`
}

// MiniMaxTaskBinding 记录一个上游异步 task_id 的账号与计费上下文。
type MiniMaxTaskBinding struct {
	AccountID     int64  `json:"account_id"`
	UserID        int64  `json:"user_id"`
	APIKeyID      int64  `json:"api_key_id"`
	Model         string `json:"model,omitempty"`
	UpstreamModel string `json:"upstream_model,omitempty"`
	BilledChars   int    `json:"billed_chars,omitempty"`
	FileID        string `json:"file_id,omitempty"`
}

// MiniMaxBindingStore 是 file_id / task_id 归属映射的存储抽象。
type MiniMaxBindingStore interface {
	SaveFile(ctx context.Context, fileID string, binding MiniMaxFileBinding, ttl time.Duration) error
	GetFile(ctx context.Context, fileID string) (*MiniMaxFileBinding, error)
	SaveTask(ctx context.Context, taskID string, binding MiniMaxTaskBinding, ttl time.Duration) error
	GetTask(ctx context.Context, taskID string) (*MiniMaxTaskBinding, error)
}
