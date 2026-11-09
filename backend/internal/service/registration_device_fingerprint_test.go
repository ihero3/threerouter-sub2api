//go:build unit

package service

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type registrationDeviceFPStub struct {
	*userRepoStub
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

func newDeviceFPService(settings map[string]string, stub *registrationDeviceFPStub) *AuthService {
	svc := newAuthService(stub.userRepoStub, settings, nil, nil)
	svc.userRepo = stub
	return svc
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

	t.Run("disabled by zero skips storage entirely", func(t *testing.T) {
		stub := &registrationDeviceFPStub{userRepoStub: &userRepoStub{}, count: 999}
		svc := newDeviceFPService(map[string]string{
			SettingKeyRegistrationDeviceFingerprintLimit: "0",
		}, stub)
		require.NoError(t, svc.EnforceRegistrationDeviceFingerprint(ctx, fp, true))
		require.Zero(t, stub.countCalls)
	})

	t.Run("rejects at limit", func(t *testing.T) {
		stub := &registrationDeviceFPStub{userRepoStub: &userRepoStub{}, count: 3}
		svc := newDeviceFPService(map[string]string{
			SettingKeyRegistrationDeviceFingerprintLimit: "3",
		}, stub)
		require.ErrorIs(t, svc.EnforceRegistrationDeviceFingerprint(ctx, fp, true), ErrRegistrationDeviceFingerprintLimit)
	})

	t.Run("allows below limit", func(t *testing.T) {
		stub := &registrationDeviceFPStub{userRepoStub: &userRepoStub{}, count: 2}
		svc := newDeviceFPService(map[string]string{
			SettingKeyRegistrationDeviceFingerprintLimit: "3",
		}, stub)
		require.NoError(t, svc.EnforceRegistrationDeviceFingerprint(ctx, fp, true))
	})

	t.Run("missing client fingerprint uses relaxed limit", func(t *testing.T) {
		stub := &registrationDeviceFPStub{userRepoStub: &userRepoStub{}, count: 3}
		svc := newDeviceFPService(map[string]string{
			SettingKeyRegistrationDeviceFingerprintLimit: "3",
		}, stub)
		// 降级指纹阈值 = max(3*10, 30) = 30，避免只按 UA 聚合误伤。
		require.NoError(t, svc.EnforceRegistrationDeviceFingerprint(ctx, fp, false))
		stub.count = 30
		require.ErrorIs(t, svc.EnforceRegistrationDeviceFingerprint(ctx, fp, false), ErrRegistrationDeviceFingerprintLimit)
	})

	t.Run("count error fails open", func(t *testing.T) {
		stub := &registrationDeviceFPStub{userRepoStub: &userRepoStub{}, countErr: errors.New("db down")}
		svc := newDeviceFPService(map[string]string{}, stub)
		require.NoError(t, svc.EnforceRegistrationDeviceFingerprint(ctx, fp, true))
	})

	t.Run("empty fingerprint is not queried", func(t *testing.T) {
		stub := &registrationDeviceFPStub{userRepoStub: &userRepoStub{}, count: 999}
		svc := newDeviceFPService(map[string]string{}, stub)
		require.NoError(t, svc.EnforceRegistrationDeviceFingerprint(ctx, "", true))
		require.Zero(t, stub.countCalls)
	})
}

func TestRecordRegistrationDeviceFingerprint(t *testing.T) {
	t.Parallel()

	t.Run("records when enabled and fingerprint present", func(t *testing.T) {
		stub := &registrationDeviceFPStub{userRepoStub: &userRepoStub{}}
		svc := newDeviceFPService(map[string]string{}, stub)
		ctx := WithRegistrationDeviceFingerprint(context.Background(), "aabbccddeeff00112233445566778899")
		svc.RecordRegistrationDeviceFingerprint(ctx, 42, "203.0.113.9")
		require.Len(t, stub.records, 1)
	})

	t.Run("skips when no fingerprint in context", func(t *testing.T) {
		stub := &registrationDeviceFPStub{userRepoStub: &userRepoStub{}}
		svc := newDeviceFPService(map[string]string{}, stub)
		svc.RecordRegistrationDeviceFingerprint(context.Background(), 42, "203.0.113.9")
		require.Empty(t, stub.records)
	})

	t.Run("skips when disabled", func(t *testing.T) {
		stub := &registrationDeviceFPStub{userRepoStub: &userRepoStub{}}
		svc := newDeviceFPService(map[string]string{
			SettingKeyRegistrationDeviceFingerprintLimit: "0",
		}, stub)
		ctx := WithRegistrationDeviceFingerprint(context.Background(), "aabbccddeeff00112233445566778899")
		svc.RecordRegistrationDeviceFingerprint(ctx, 42, "203.0.113.9")
		require.Empty(t, stub.records)
	})
}
