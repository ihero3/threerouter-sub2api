package repository

// minimax_binding_store.go — MiniMax file_id / task_id 的账号归属映射（Redis）。
//
// 只保存路由与计费上下文，不保存音频内容；TTL 到期后查询/下载会返回参数错误，
// 与上游「文件过期」的行为一致。key 使用固定前缀，value 为 JSON。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	miniMaxFileKeyPrefix = "mmx:file:"
	miniMaxTaskKeyPrefix = "mmx:task:"
)

type miniMaxBindingStore struct {
	client *redis.Client
}

// NewMiniMaxBindingStore 创建 MiniMax 归属映射存储。
func NewMiniMaxBindingStore(client *redis.Client) service.MiniMaxBindingStore {
	return &miniMaxBindingStore{client: client}
}

func miniMaxBindingKey(prefix, id string) string {
	return prefix + strings.TrimSpace(id)
}

func (s *miniMaxBindingStore) SaveFile(
	ctx context.Context, fileID string, binding service.MiniMaxFileBinding, ttl time.Duration,
) error {
	if s == nil || s.client == nil {
		return errors.New("minimax binding store: redis unavailable")
	}
	if strings.TrimSpace(fileID) == "" {
		return errors.New("minimax binding store: file id is empty")
	}
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	raw, err := json.Marshal(binding)
	if err != nil {
		return fmt.Errorf("minimax binding store: marshal file binding: %w", err)
	}
	return s.client.Set(ctx, miniMaxBindingKey(miniMaxFileKeyPrefix, fileID), raw, ttl).Err()
}

func (s *miniMaxBindingStore) GetFile(
	ctx context.Context, fileID string,
) (*service.MiniMaxFileBinding, error) {
	if s == nil || s.client == nil {
		return nil, errors.New("minimax binding store: redis unavailable")
	}
	raw, err := s.client.Get(ctx, miniMaxBindingKey(miniMaxFileKeyPrefix, fileID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("minimax binding store: get file binding: %w", err)
	}
	var binding service.MiniMaxFileBinding
	if err := json.Unmarshal(raw, &binding); err != nil {
		return nil, fmt.Errorf("minimax binding store: decode file binding: %w", err)
	}
	return &binding, nil
}

func (s *miniMaxBindingStore) SaveTask(
	ctx context.Context, taskID string, binding service.MiniMaxTaskBinding, ttl time.Duration,
) error {
	if s == nil || s.client == nil {
		return errors.New("minimax binding store: redis unavailable")
	}
	if strings.TrimSpace(taskID) == "" {
		return errors.New("minimax binding store: task id is empty")
	}
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	raw, err := json.Marshal(binding)
	if err != nil {
		return fmt.Errorf("minimax binding store: marshal task binding: %w", err)
	}
	return s.client.Set(ctx, miniMaxBindingKey(miniMaxTaskKeyPrefix, taskID), raw, ttl).Err()
}

func (s *miniMaxBindingStore) GetTask(
	ctx context.Context, taskID string,
) (*service.MiniMaxTaskBinding, error) {
	if s == nil || s.client == nil {
		return nil, errors.New("minimax binding store: redis unavailable")
	}
	raw, err := s.client.Get(ctx, miniMaxBindingKey(miniMaxTaskKeyPrefix, taskID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("minimax binding store: get task binding: %w", err)
	}
	var binding service.MiniMaxTaskBinding
	if err := json.Unmarshal(raw, &binding); err != nil {
		return nil, fmt.Errorf("minimax binding store: decode task binding: %w", err)
	}
	return &binding, nil
}
