package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// recordingUsageLogRepo 记录 writeUsageLogBestEffort 落下的每一行，
// 用来断言「整单失败」也会留下 0 费用使用记录。
type recordingUsageLogRepo struct {
	usageBatchLogRepoStub
	logs []*UsageLog
}

func (r *recordingUsageLogRepo) Create(_ context.Context, log *UsageLog) (bool, error) {
	r.logs = append(r.logs, log)
	return true, nil
}

func newMediaFailureUsageTestService(repo UsageLogRepository) *MediaTaskService {
	return &MediaTaskService{
		logger:               zap.NewNop(),
		openAIGatewayService: &OpenAIGatewayService{usageLogRepo: repo},
	}
}

// TestWriteMediaTaskFailureUsageLog_Image 选号/上游全部失败时，图片调用必须留下
// 0 费用使用记录：否则用户在「用量明细」里完全看不到这次调用（只在错误面板可见），
// 会误判成「没落库 / 没计费」。
func TestWriteMediaTaskFailureUsageLog_Image(t *testing.T) {
	repo := &recordingUsageLogRepo{}
	svc := newMediaFailureUsageTestService(repo)

	svc.writeMediaTaskFailureUsageLog(context.Background(), MediaKindImage, 7, 9, "qwen-image-3.0",
		&MediaCreateRequest{Resolution: "1024x1024", ImageCount: 2})

	require.Len(t, repo.logs, 1)
	log := repo.logs[0]
	require.Equal(t, "qwen-image-3.0", log.Model)
	require.Equal(t, "qwen-image-3.0", log.RequestedModel)
	require.Equal(t, int64(7), log.UserID)
	require.Equal(t, int64(9), log.APIKeyID)
	require.Equal(t, 2, log.ImageCount)
	require.NotNil(t, log.BillingMode)
	require.Equal(t, string(BillingModeImage), *log.BillingMode)
	require.Equal(t, 0.0, log.TotalCost)
	require.Equal(t, 0.0, log.ActualCost)
	require.Contains(t, log.RequestID, "media-image:img_")
	require.False(t, log.CreatedAt.IsZero())
}

// TestWriteMediaTaskFailureUsageLog_AudioAndVideo 音频 / 视频走统一口径，
// 三类媒体任务都保证「调用过就有一行」。
func TestWriteMediaTaskFailureUsageLog_AudioAndVideo(t *testing.T) {
	audioRepo := &recordingUsageLogRepo{}
	audioSvc := newMediaFailureUsageTestService(audioRepo)
	audioSvc.writeMediaTaskFailureUsageLog(context.Background(), MediaKindAudio, 1, 2, "qwen3-tts",
		&MediaCreateRequest{DurationSec: 12})
	require.Len(t, audioRepo.logs, 1)
	require.Contains(t, audioRepo.logs[0].RequestID, "media-audio:aud_")
	require.Equal(t, 0.0, audioRepo.logs[0].ActualCost)

	videoRepo := &recordingUsageLogRepo{}
	videoSvc := newMediaFailureUsageTestService(videoRepo)
	videoSvc.writeMediaTaskFailureUsageLog(context.Background(), MediaKindVideo, 1, 2, "wan3.0-video",
		&MediaCreateRequest{Resolution: "720p", DurationSec: 5})
	require.Len(t, videoRepo.logs, 1)
	require.Equal(t, int64(1), videoRepo.logs[0].UserID)
	require.Equal(t, "wan3.0-video", videoRepo.logs[0].Model)
	require.Equal(t, 0.0, videoRepo.logs[0].TotalCost)
	require.Contains(t, videoRepo.logs[0].RequestID, "video-task:vid_")
}

// TestWriteMediaTaskFailureUsageLog_NoRepo 未接线 usageLogRepo 时静默跳过，
// 不能让日志落库失败反过来影响错误返回。
func TestWriteMediaTaskFailureUsageLog_NoRepo(t *testing.T) {
	svc := &MediaTaskService{logger: zap.NewNop(), openAIGatewayService: &OpenAIGatewayService{}}
	require.NotPanics(t, func() {
		svc.writeMediaTaskFailureUsageLog(context.Background(), MediaKindImage, 1, 2, "qwen-image-3.0", nil)
	})

	var nilSvc *MediaTaskService
	require.NotPanics(t, func() {
		nilSvc.writeMediaTaskFailureUsageLog(context.Background(), MediaKindImage, 1, 2, "qwen-image-3.0", nil)
	})
}
