package service

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// registrationDeviceFPStub 自包含桩（bare 层，无 //go:build unit）。
// 同包里的 userRepoStub / newAuthService / settingRepoStub 都定义在 unit 标签
// 文件里，bare 测试无法引用，因此这里嵌入 UserRepository 接口占位来满足
// AuthService.userRepo 的静态类型：风控只通过可选接口断言调用下面两个方法，
// 不触达 UserRepository 的其它方法，嵌入值可保持为 nil。
type registrationDeviceFPStub struct {
	UserRepository
	count      int
	countErr   error
	countCalls int
	records    []string
	recordErr  error
}

func (s *registrationDeviceFPStub) CountRegistrationDeviceFingerprints(_ context.Context, _ string, _ time.Time) (int, error) {
	s.countCalls++
	return s.count, s.countErr
}

func (s *registrationDeviceFPStub) RecordRegistrationDeviceFingerprint(_ context.Context, fingerprint string, userID int64, _ string) error {
	if s.recordErr != nil {
		return s.recordErr
	}
	s.records = append(s.records, fingerprint+":"+strconv.FormatInt(userID, 10))
	return nil
}

// registrationDeviceFPSettingRepo 只实现 GetValue；其余方法由嵌入的
// SettingRepository 接口占位，getCachedSettingValue 路径不会触达。
type registrationDeviceFPSettingRepo struct {
	SettingRepository
	values map[string]string
}

func (r *registrationDeviceFPSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	if v, ok := r.values[key]; ok {
		return v, nil
	}
	return "", ErrSettingNotFound
}

func newDeviceFPService(settings map[string]string, stub *registrationDeviceFPStub) *AuthService {
	return &AuthService{
		userRepo: stub,
		settingService: &SettingService{
			settingRepo: &registrationDeviceFPSettingRepo{values: settings},
		},
	}
}

func TestNormalizeClientDeviceFingerprint(t *testing.T) {
	t.Parallel()
	valid := "0123456789abcdef0123456789abcdef"
	require.Equal(t, valid, NormalizeClientDeviceFingerprint(valid))
	require.Equal(t, valid, NormalizeClientDeviceFingerprint("  "+valid+"  "))
	require.Equal(t, valid, NormalizeClientDeviceFingerprint("0123456789ABCDEF0123456789ABCDEF"))
	require.Empty(t, NormalizeClientDeviceFingerprint("abc"))
	require.Empty(t, NormalizeClientDeviceFingerprint(""))
	require.Empty(t, NormalizeClientDeviceFingerprint("zzzzzzzzzzzzzzzzzzzz"))
	// 超长输入先截断到 64 字符再清洗，结果仍是可用的十六进制摘要。
	require.Len(t, NormalizeClientDeviceFingerprint(valid+valid+valid), 64)
}

func TestBuildRegistrationDeviceFingerprint(t *testing.T) {
	t.Parallel()
	client := "0123456789abcdef0123456789abcdef"

	fpA, reportedA := BuildRegistrationDeviceFingerprint(client, "Mozilla/5.0 TestUA")
	fpB, reportedB := BuildRegistrationDeviceFingerprint(client, "Mozilla/5.0 TestUA")
	require.True(t, reportedA)
	require.True(t, reportedB)
	require.Equal(t, fpA, fpB, "same device and UA must produce a stable fingerprint")
	require.Len(t, fpA, 64)
	require.NotEqual(t, client, fpA, "server must not persist the raw client value")

	fpOtherUA, _ := BuildRegistrationDeviceFingerprint(client, "Mozilla/5.0 OtherUA")
	require.NotEqual(t, fpA, fpOtherUA, "server-side UA must be mixed in")

	fpMissing, reportedMissing := BuildRegistrationDeviceFingerprint("", "Mozilla/5.0 TestUA")
	require.False(t, reportedMissing)
	require.NotEqual(t, fpA, fpMissing)
	fpMissing2, reportedMissing2 := BuildRegistrationDeviceFingerprint("short", "Mozilla/5.0 TestUA")
	require.False(t, reportedMissing2)
	require.Equal(t, fpMissing, fpMissing2, "invalid client values must collapse into the same UA-only bucket")
}

func TestClampRegistrationDeviceFingerprintLimit(t *testing.T) {
	t.Parallel()
	require.Equal(t, 0, clampRegistrationDeviceFingerprintLimit(0))
	require.Equal(t, 0, clampRegistrationDeviceFingerprintLimit(-5))
	require.Equal(t, 1, clampRegistrationDeviceFingerprintLimit(1))
	require.Equal(t, 3, clampRegistrationDeviceFingerprintLimit(3))
	require.Equal(t, RegistrationDeviceFingerprintLimitMax, clampRegistrationDeviceFingerprintLimit(999))
}

func TestEnforceRegistrationDeviceFingerprint(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fp := "aabbccddeeff00112233445566778899"

	t.Run("disabled by zero allows even a missing client fingerprint", func(t *testing.T) {
		stub := &registrationDeviceFPStub{count: 999}
		svc := newDeviceFPService(map[string]string{
			SettingKeyRegistrationDeviceFingerprintLimit: "0",
		}, stub)
		require.NoError(t, svc.EnforceRegistrationDeviceFingerprint(ctx, fp, false))
		require.Zero(t, stub.countCalls, "关闭时不应查询存储")
	})

	t.Run("client-reported rejects at the 3rd registration", func(t *testing.T) {
		stub := &registrationDeviceFPStub{count: 3}
		svc := newDeviceFPService(map[string]string{
			SettingKeyRegistrationDeviceFingerprintLimit: "3",
		}, stub)
		require.ErrorIs(t, svc.EnforceRegistrationDeviceFingerprint(ctx, fp, true), ErrRegistrationDeviceFingerprintLimit)
	})

	t.Run("allows below limit", func(t *testing.T) {
		stub := &registrationDeviceFPStub{count: 2}
		svc := newDeviceFPService(map[string]string{
			SettingKeyRegistrationDeviceFingerprintLimit: "3",
		}, stub)
		require.NoError(t, svc.EnforceRegistrationDeviceFingerprint(ctx, fp, true))
	})

	// 收紧回归哨兵：缺失指纹不再享受放宽阈值，必须与正常指纹一样在第 3 次被拦截。
	// 若把旧的 max(limit*10, 30) 下限放回去，count=3 会被放行，此用例应变红。
	t.Run("missing client fingerprint uses the same limit", func(t *testing.T) {
		stub := &registrationDeviceFPStub{count: 3}
		svc := newDeviceFPService(map[string]string{
			SettingKeyRegistrationDeviceFingerprintLimit: "3",
		}, stub)
		require.ErrorIs(t, svc.EnforceRegistrationDeviceFingerprint(ctx, fp, false), ErrRegistrationDeviceFingerprintLimit)

		stub.count = 2
		require.NoError(t, svc.EnforceRegistrationDeviceFingerprint(ctx, fp, false), "低于阈值时降级指纹同样放行")
	})

	// 设置缺失时回退默认 limit=3；计数查询失败必须 fail-open，即便无指纹也放行。
	t.Run("count error fails open", func(t *testing.T) {
		stub := &registrationDeviceFPStub{countErr: errors.New("db down")}
		svc := newDeviceFPService(map[string]string{}, stub)
		require.NoError(t, svc.EnforceRegistrationDeviceFingerprint(ctx, fp, false))
	})

	t.Run("empty fingerprint is not queried", func(t *testing.T) {
		stub := &registrationDeviceFPStub{count: 999}
		svc := newDeviceFPService(map[string]string{}, stub)
		require.NoError(t, svc.EnforceRegistrationDeviceFingerprint(ctx, "", true))
		require.Zero(t, stub.countCalls)
	})
}

func TestRecordRegistrationDeviceFingerprint(t *testing.T) {
	t.Parallel()

	t.Run("records when enabled and fingerprint present", func(t *testing.T) {
		stub := &registrationDeviceFPStub{}
		svc := newDeviceFPService(map[string]string{}, stub)
		ctx := WithRegistrationDeviceFingerprint(context.Background(), "aabbccddeeff00112233445566778899")
		svc.RecordRegistrationDeviceFingerprint(ctx, 42, "203.0.113.9")
		require.Len(t, stub.records, 1)
	})

	t.Run("skips when no fingerprint in context", func(t *testing.T) {
		stub := &registrationDeviceFPStub{}
		svc := newDeviceFPService(map[string]string{}, stub)
		svc.RecordRegistrationDeviceFingerprint(context.Background(), 42, "203.0.113.9")
		require.Empty(t, stub.records)
	})

	t.Run("skips when disabled", func(t *testing.T) {
		stub := &registrationDeviceFPStub{}
		svc := newDeviceFPService(map[string]string{
			SettingKeyRegistrationDeviceFingerprintLimit: "0",
		}, stub)
		ctx := WithRegistrationDeviceFingerprint(context.Background(), "aabbccddeeff00112233445566778899")
		svc.RecordRegistrationDeviceFingerprint(ctx, 42, "203.0.113.9")
		require.Empty(t, stub.records)
	})
}
