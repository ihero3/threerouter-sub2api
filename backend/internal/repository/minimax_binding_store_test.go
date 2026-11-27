package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestMiniMaxBindingStoreFileAndTask(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = client.Close() }()
	store := NewMiniMaxBindingStore(client)
	ctx := context.Background()

	require.NoError(t, store.SaveFile(ctx, "123", service.MiniMaxFileBinding{
		AccountID: 7, UserID: 42, APIKeyID: 100, Purpose: "voice_clone",
	}, time.Hour))
	file, err := store.GetFile(ctx, "123")
	require.NoError(t, err)
	require.NotNil(t, file)
	require.Equal(t, int64(7), file.AccountID)
	require.Equal(t, int64(42), file.UserID)
	require.Equal(t, "voice_clone", file.Purpose)

	missing, err := store.GetFile(ctx, "404")
	require.NoError(t, err)
	require.Nil(t, missing)

	require.NoError(t, store.SaveTask(ctx, "951", service.MiniMaxTaskBinding{
		AccountID: 7, UserID: 42, APIKeyID: 100,
		Model: "speech-2.8-hd", UpstreamModel: "speech-2.8-hd",
		BilledChars: 11, FileID: "952",
	}, time.Hour))
	task, err := store.GetTask(ctx, "951")
	require.NoError(t, err)
	require.NotNil(t, task)
	require.Equal(t, 11, task.BilledChars)
	require.Equal(t, "952", task.FileID)

	// TTL 到期后映射消失，查询按「已过期」处理。
	mr.FastForward(2 * time.Hour)
	file, err = store.GetFile(ctx, "123")
	require.NoError(t, err)
	require.Nil(t, file)
	task, err = store.GetTask(ctx, "951")
	require.NoError(t, err)
	require.Nil(t, task)
}

func TestMiniMaxBindingStoreWithoutRedis(t *testing.T) {
	store := NewMiniMaxBindingStore(nil)
	_, err := store.GetFile(context.Background(), "123")
	require.Error(t, err)
	require.Error(t, store.SaveTask(context.Background(), "951", service.MiniMaxTaskBinding{}, time.Hour))
}
