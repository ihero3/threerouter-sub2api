package repository

import (
	"context"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbmediatask "github.com/Wei-Shaw/sub2api/ent/mediatask"

	"github.com/Wei-Shaw/sub2api/internal/service"

	entsql "entgo.io/ent/dialect/sql"
)

// MediaTaskRepository 媒体任务数据访问接口。
// 实现 service.MediaTaskRepo 接口，保持低耦合：不暴露 ent 类型。
type MediaTaskRepository interface {
	Create(ctx context.Context, task *service.MediaTaskRecord) (*service.MediaTaskRecord, error)
	GetByLocalID(ctx context.Context, localID string) (*service.MediaTaskRecord, error)
	GetByID(ctx context.Context, id int64) (*service.MediaTaskRecord, error)
	UpdateStatusIfProcessing(ctx context.Context, id int64, status, errorMsg string) (bool, error)
	UpdateResult(ctx context.Context, id int64, status, mediaURL, thumbnailURL string, durationSec int, costUSD float64) (bool, error)
	UpdateUpstreamTaskID(ctx context.Context, id int64, upstreamTaskID string) error
	ListByUserID(ctx context.Context, userID int64, limit, offset int) ([]*service.MediaTaskRecord, int, error)
	ListProcessingTasks(ctx context.Context, before time.Time, limit int) ([]*service.MediaTaskRecord, error)
	ListAdmin(ctx context.Context, userID int64, status, mediaKind string, limit, offset int) ([]*service.MediaTaskRecord, int, error)
}

// mediaTaskRepository 实现 MediaTaskRepository 接口。
type mediaTaskRepository struct {
	client *dbent.Client
}

// NewMediaTaskRepository 创建媒体任务仓储实例。
func NewMediaTaskRepository(client *dbent.Client) MediaTaskRepository {
	return &mediaTaskRepository{client: client}
}

func (r *mediaTaskRepository) Create(ctx context.Context, task *service.MediaTaskRecord) (*service.MediaTaskRecord, error) {
	b := r.client.MediaTask.Create().
		SetLocalID(task.LocalID).
		SetUserID(task.UserID).
		SetPublicModel(task.PublicModel).
		SetUpstreamModel(task.UpstreamModel).
		SetAccountID(task.AccountID).
		SetStatus(task.Status)
	if task.MediaKind != "" {
		b.SetMediaKind(string(task.MediaKind))
	}
	if task.APIKeyID > 0 {
		b.SetAPIKeyID(task.APIKeyID)
	}
	if task.UpstreamTaskID != "" {
		b.SetUpstreamTaskID(task.UpstreamTaskID)
	}
	if task.Resolution != "" {
		b.SetResolution(task.Resolution)
	}
	if task.DurationSec > 0 {
		b.SetDurationSec(task.DurationSec)
	}
	if task.MediaURL != "" {
		b.SetMediaURL(task.MediaURL)
	}
	if len(task.MediaURLs) > 0 {
		b.SetMediaUrls(task.MediaURLs)
	}
	if task.ThumbnailURL != "" {
		b.SetThumbnailURL(task.ThumbnailURL)
	}
	if task.RequestBody != nil {
		b.SetRequestBody(task.RequestBody)
	}
	if task.ErrorMessage != "" {
		b.SetErrorMessage(task.ErrorMessage)
	}
	if task.CostUSD > 0 {
		b.SetCostUsd(task.CostUSD)
	}
	if task.ReservedCost != nil {
		b.SetNillableReservedCost(task.ReservedCost)
	}

	mt, err := b.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("media_task_repo: create: %w", err)
	}
	return entToMediaTaskRecord(mt), nil
}

func (r *mediaTaskRepository) GetByLocalID(ctx context.Context, localID string) (*service.MediaTaskRecord, error) {
	mt, err := r.client.MediaTask.Query().
		Where(dbmediatask.LocalIDEQ(localID)).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("media_task_repo: get by local_id: %w", err)
	}
	return entToMediaTaskRecord(mt), nil
}

func (r *mediaTaskRepository) GetByID(ctx context.Context, id int64) (*service.MediaTaskRecord, error) {
	mt, err := r.client.MediaTask.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("media_task_repo: get by id: %w", err)
	}
	return entToMediaTaskRecord(mt), nil
}

func (r *mediaTaskRepository) UpdateStatusIfProcessing(ctx context.Context, id int64, status, errorMsg string) (bool, error) {
	b := r.client.MediaTask.Update().
		Where(dbmediatask.IDEQ(id), dbmediatask.StatusEQ("processing")).
		SetStatus(status)
	if errorMsg != "" {
		b.SetErrorMessage(errorMsg)
	}
	if status == "succeeded" || status == "failed" || status == "cancelled" {
		b.SetFinishedAt(time.Now())
	}
	affected, err := b.Save(ctx)
	if err != nil {
		return false, fmt.Errorf("media_task_repo: conditional update status: %w", err)
	}
	return affected > 0, nil
}

func (r *mediaTaskRepository) UpdateResult(ctx context.Context, id int64, status, mediaURL, thumbnailURL string, durationSec int, costUSD float64) (bool, error) {
	b := r.client.MediaTask.Update().
		Where(dbmediatask.IDEQ(id), dbmediatask.StatusEQ("processing")).
		SetStatus(status).
		SetMediaURL(mediaURL).
		SetThumbnailURL(thumbnailURL).
		SetDurationSec(durationSec).
		SetCostUsd(costUSD)
	if status == "succeeded" || status == "failed" || status == "cancelled" {
		b.SetFinishedAt(time.Now())
	}
	affected, err := b.Save(ctx)
	if err != nil {
		return false, fmt.Errorf("media_task_repo: update result: %w", err)
	}
	return affected > 0, nil
}

func (r *mediaTaskRepository) UpdateUpstreamTaskID(ctx context.Context, id int64, upstreamTaskID string) error {
	if err := r.client.MediaTask.UpdateOneID(id).
		SetUpstreamTaskID(upstreamTaskID).
		Exec(ctx); err != nil {
		return fmt.Errorf("media_task_repo: update upstream_task_id: %w", err)
	}
	return nil
}

func (r *mediaTaskRepository) ListByUserID(ctx context.Context, userID int64, limit, offset int) ([]*service.MediaTaskRecord, int, error) {
	query := r.client.MediaTask.Query().Where(dbmediatask.UserIDEQ(userID))
	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("media_task_repo: count: %w", err)
	}
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	tasks, err := query.
		Order(dbmediatask.ByCreatedAt(entsql.OrderDesc())).
		Limit(limit).
		Offset(offset).
		All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("media_task_repo: list: %w", err)
	}
	records := make([]*service.MediaTaskRecord, len(tasks))
	for i, mt := range tasks {
		records[i] = entToMediaTaskRecord(mt)
	}
	return records, total, nil
}

func (r *mediaTaskRepository) ListAdmin(ctx context.Context, userID int64, status, mediaKind string, limit, offset int) ([]*service.MediaTaskRecord, int, error) {
	query := r.client.MediaTask.Query()
	if userID > 0 {
		query = query.Where(dbmediatask.UserIDEQ(userID))
	}
	if status != "" {
		query = query.Where(dbmediatask.StatusEQ(status))
	}
	if mediaKind != "" {
		query = query.Where(dbmediatask.MediaKindEQ(mediaKind))
	}
	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("media_task_repo: count admin: %w", err)
	}
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	tasks, err := query.
		Order(dbmediatask.ByCreatedAt(entsql.OrderDesc())).
		Limit(limit).
		Offset(offset).
		All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("media_task_repo: list admin: %w", err)
	}
	records := make([]*service.MediaTaskRecord, len(tasks))
	for i, mt := range tasks {
		records[i] = entToMediaTaskRecord(mt)
	}
	return records, total, nil
}

func (r *mediaTaskRepository) ListProcessingTasks(ctx context.Context, before time.Time, limit int) ([]*service.MediaTaskRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	tasks, err := r.client.MediaTask.Query().
		Where(
			dbmediatask.StatusEQ("processing"),
			dbmediatask.CreatedAtLTE(before),
		).
		Order(dbmediatask.ByCreatedAt(entsql.OrderAsc())).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("media_task_repo: list processing: %w", err)
	}
	records := make([]*service.MediaTaskRecord, len(tasks))
	for i, mt := range tasks {
		records[i] = entToMediaTaskRecord(mt)
	}
	return records, nil
}

// entToMediaTaskRecord 将 ent MediaTask 转换为 service.MediaTaskRecord。
func entToMediaTaskRecord(mt *dbent.MediaTask) *service.MediaTaskRecord {
	rec := &service.MediaTaskRecord{
		ID:             mt.ID,
		LocalID:        mt.LocalID,
		MediaKind:      service.MediaKind(mt.MediaKind),
		UserID:         mt.UserID,
		PublicModel:    mt.PublicModel,
		UpstreamModel:  mt.UpstreamModel,
		AccountID:      mt.AccountID,
		UpstreamTaskID: mt.UpstreamTaskID,
		Status:         mt.Status,
		Resolution:     mt.Resolution,
		DurationSec:    mt.DurationSec,
		MediaURL:       mt.MediaURL,
		MediaURLs:      mt.MediaUrls,
		ThumbnailURL:   mt.ThumbnailURL,
		RequestBody:    mt.RequestBody,
		ErrorMessage:   mt.ErrorMessage,
		CostUSD:        mt.CostUsd,
		ReservedCost:   mt.ReservedCost,
		CreatedAt:      mt.CreatedAt,
		UpdatedAt:      mt.UpdatedAt,
		FinishedAt:     mt.FinishedAt,
	}
	rec.APIKeyID = mt.APIKeyID
	return rec
}
