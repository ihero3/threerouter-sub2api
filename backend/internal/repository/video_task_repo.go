package repository

import (
	"context"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbvideotask "github.com/Wei-Shaw/sub2api/ent/videotask"
	"github.com/Wei-Shaw/sub2api/internal/service"

	entsql "entgo.io/ent/dialect/sql"
)

// VideoTaskRepository 视频任务数据访问接口。
// 实现 service.VideoTaskRepo 接口，保持低耦合：不暴露 ent 类型。
type VideoTaskRepository interface {
	Create(ctx context.Context, task *service.VideoTaskRecord) (*service.VideoTaskRecord, error)
	GetByLocalID(ctx context.Context, localID string) (*service.VideoTaskRecord, error)
	GetByID(ctx context.Context, id int64) (*service.VideoTaskRecord, error)
	UpdateStatus(ctx context.Context, id int64, status, errorMsg string) error
	UpdateStatusIfProcessing(ctx context.Context, id int64, status, errorMsg string) (bool, error)
	UpdateResult(ctx context.Context, id int64, status, videoURL, thumbnailURL string, durationSec int, costUSD float64) (bool, error)
	UpdateUpstreamTaskID(ctx context.Context, id int64, upstreamTaskID string) error
	ListByUserID(ctx context.Context, userID int64, limit, offset int) ([]*service.VideoTaskRecord, int, error)
	ListProcessingTasks(ctx context.Context, before time.Time, limit int) ([]*service.VideoTaskRecord, error)
	ListAdmin(ctx context.Context, userID int64, status string, limit, offset int) ([]*service.VideoTaskRecord, int, error)
}

// videoTaskRepository 实现 VideoTaskRepository 接口。
type videoTaskRepository struct {
	client *dbent.Client
}

// NewVideoTaskRepository 创建视频任务仓储实例。
func NewVideoTaskRepository(client *dbent.Client) VideoTaskRepository {
	return &videoTaskRepository{client: client}
}

func (r *videoTaskRepository) Create(ctx context.Context, task *service.VideoTaskRecord) (*service.VideoTaskRecord, error) {
	b := r.client.VideoTask.Create().
		SetLocalID(task.LocalID).
		SetUserID(task.UserID).
		SetPublicModel(task.PublicModel).
		SetUpstreamModel(task.UpstreamModel).
		SetAccountID(task.AccountID).
		SetStatus(task.Status)
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
	if task.VideoURL != "" {
		b.SetVideoURL(task.VideoURL)
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

	vt, err := b.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("video_task_repo: create: %w", err)
	}
	return entToRecord(vt), nil
}

func (r *videoTaskRepository) GetByLocalID(ctx context.Context, localID string) (*service.VideoTaskRecord, error) {
	vt, err := r.client.VideoTask.Query().
		Where(dbvideotask.LocalIDEQ(localID)).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("video_task_repo: get by local_id: %w", err)
	}
	return entToRecord(vt), nil
}

func (r *videoTaskRepository) GetByID(ctx context.Context, id int64) (*service.VideoTaskRecord, error) {
	vt, err := r.client.VideoTask.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("video_task_repo: get by id: %w", err)
	}
	return entToRecord(vt), nil
}

// UpdateStatusIfProcessing 条件更新：仅当任务仍处于 processing 时才改为终态，
// 返回 affected>0（claimed），供调用方保证「同一任务只结算一次」。
//
// 背景：UpdateStatus 用 UpdateOneID 无条件覆盖，GetTask 轮询与 Worker PollTask
// 并发命中同一个 failed/timeout 任务时会各自退一次预扣 → 重复退款（白送配额）。
// 与 media_task_repo.UpdateStatusIfProcessing 保持同一范式。
func (r *videoTaskRepository) UpdateStatusIfProcessing(ctx context.Context, id int64, status, errorMsg string) (bool, error) {
	b := r.client.VideoTask.Update().
		Where(dbvideotask.IDEQ(id), dbvideotask.StatusEQ("processing")).
		SetStatus(status)
	if errorMsg != "" {
		b.SetErrorMessage(errorMsg)
	}
	if status == "succeeded" || status == "failed" || status == "cancelled" {
		b.SetFinishedAt(time.Now())
	}
	affected, err := b.Save(ctx)
	if err != nil {
		return false, fmt.Errorf("video_task_repo: conditional update status: %w", err)
	}
	return affected > 0, nil
}

func (r *videoTaskRepository) UpdateStatus(ctx context.Context, id int64, status, errorMsg string) error {
	b := r.client.VideoTask.UpdateOneID(id).SetStatus(status)
	if errorMsg != "" {
		b.SetErrorMessage(errorMsg)
	}
	if status == "succeeded" || status == "failed" || status == "cancelled" {
		now := time.Now()
		b.SetFinishedAt(now)
	}
	if err := b.Exec(ctx); err != nil {
		return fmt.Errorf("video_task_repo: update status: %w", err)
	}
	return nil
}

func (r *videoTaskRepository) UpdateResult(ctx context.Context, id int64, status, videoURL, thumbnailURL string, durationSec int, costUSD float64) (bool, error) {
	b := r.client.VideoTask.Update().
		Where(dbvideotask.IDEQ(id), dbvideotask.StatusEQ("processing")).
		SetStatus(status).
		SetVideoURL(videoURL).
		SetThumbnailURL(thumbnailURL).
		SetDurationSec(durationSec).
		SetCostUsd(costUSD)
	if status == "succeeded" || status == "failed" || status == "cancelled" {
		b.SetFinishedAt(time.Now())
	}
	affected, err := b.Save(ctx)
	if err != nil {
		return false, fmt.Errorf("video_task_repo: update result: %w", err)
	}
	return affected > 0, nil
}

func (r *videoTaskRepository) UpdateUpstreamTaskID(ctx context.Context, id int64, upstreamTaskID string) error {
	if err := r.client.VideoTask.UpdateOneID(id).
		SetUpstreamTaskID(upstreamTaskID).
		Exec(ctx); err != nil {
		return fmt.Errorf("video_task_repo: update upstream_task_id: %w", err)
	}
	return nil
}

func (r *videoTaskRepository) ListByUserID(ctx context.Context, userID int64, limit, offset int) ([]*service.VideoTaskRecord, int, error) {
	query := r.client.VideoTask.Query().Where(dbvideotask.UserIDEQ(userID))
	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("video_task_repo: count: %w", err)
	}
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	tasks, err := query.
		Order(dbvideotask.ByCreatedAt(entsql.OrderDesc())).
		Limit(limit).
		Offset(offset).
		All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("video_task_repo: list: %w", err)
	}
	records := make([]*service.VideoTaskRecord, len(tasks))
	for i, vt := range tasks {
		records[i] = entToRecord(vt)
	}
	return records, total, nil
}

func (r *videoTaskRepository) ListAdmin(ctx context.Context, userID int64, status string, limit, offset int) ([]*service.VideoTaskRecord, int, error) {
	query := r.client.VideoTask.Query()
	if userID > 0 {
		query = query.Where(dbvideotask.UserIDEQ(userID))
	}
	if status != "" {
		query = query.Where(dbvideotask.StatusEQ(status))
	}
	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("video_task_repo: count admin: %w", err)
	}
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	tasks, err := query.
		Order(dbvideotask.ByCreatedAt(entsql.OrderDesc())).
		Limit(limit).
		Offset(offset).
		All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("video_task_repo: list admin: %w", err)
	}
	records := make([]*service.VideoTaskRecord, len(tasks))
	for i, vt := range tasks {
		records[i] = entToRecord(vt)
	}
	return records, total, nil
}

func (r *videoTaskRepository) ListProcessingTasks(ctx context.Context, before time.Time, limit int) ([]*service.VideoTaskRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	tasks, err := r.client.VideoTask.Query().
		Where(
			dbvideotask.StatusEQ("processing"),
			dbvideotask.CreatedAtLTE(before),
		).
		Order(dbvideotask.ByCreatedAt(entsql.OrderAsc())).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("video_task_repo: list processing: %w", err)
	}
	records := make([]*service.VideoTaskRecord, len(tasks))
	for i, vt := range tasks {
		records[i] = entToRecord(vt)
	}
	return records, nil
}

// entToRecord 将 ent VideoTask 转换为 service.VideoTaskRecord。
func entToRecord(vt *dbent.VideoTask) *service.VideoTaskRecord {
	rec := &service.VideoTaskRecord{
		ID:             vt.ID,
		LocalID:        vt.LocalID,
		UserID:         vt.UserID,
		PublicModel:    vt.PublicModel,
		UpstreamModel:  vt.UpstreamModel,
		AccountID:      vt.AccountID,
		UpstreamTaskID: vt.UpstreamTaskID,
		Status:         vt.Status,
		Resolution:     vt.Resolution,
		DurationSec:    vt.DurationSec,
		VideoURL:       vt.VideoURL,
		ThumbnailURL:   vt.ThumbnailURL,
		RequestBody:    vt.RequestBody,
		ErrorMessage:   vt.ErrorMessage,
		CostUSD:        vt.CostUsd,
		ReservedCost:   vt.ReservedCost,
		CreatedAt:      vt.CreatedAt,
		UpdatedAt:      vt.UpdatedAt,
		FinishedAt:     vt.FinishedAt,
	}
	rec.APIKeyID = vt.APIKeyID
	return rec
}
