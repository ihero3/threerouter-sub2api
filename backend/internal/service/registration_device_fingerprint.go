package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// 注册设备指纹风控。
//
// 背景：代理池可以让每次注册都换一个真实出口 IP，单 IP 配额/限流全部失效，
// 但同一台设备连续注册时浏览器侧信号是稳定的。这里把前端上报的
// UA + 时区 + 语言 + 屏幕 + canvas 摘要，与服务端可信的 User-Agent 再混合
// 一次 SHA-256，按「同一指纹 24 小时内成功注册数」做关联拦截。
//
// 边界：
//   - 指纹由前端上报，可被主动随机化；本机制用于抬高对抗成本、串联跨 IP
//     注册，不是强身份认证。需要更强保证时应再叠加行为/支付账号维度。
//   - 前端未上报或上报非法时仍会生成「UA 维度」的降级指纹，并采用更宽松
//     的阈值，避免攻击者通过不传字段直接绕过，同时不误伤零散老客户端。
//   - 计数只统计成功建号；记录写入失败不影响注册（fail-open），但会告警。

const (
	// SettingKeyRegistrationDeviceFingerprintLimit 是后台设置项：0 表示关闭。
	SettingKeyRegistrationDeviceFingerprintLimit = "registration_device_fingerprint_limit"

	RegistrationDeviceFingerprintLimitDefault = 3
	RegistrationDeviceFingerprintLimitMin    = 1
	RegistrationDeviceFingerprintLimitMax    = 50
	RegistrationDeviceFingerprintWindow      = 24 * time.Hour

	registrationDeviceFingerprintClientMinLength = 16
	registrationDeviceFingerprintClientMaxLength = 64
	// 降级指纹的宽松倍数与下限：UA 维度区分度低，不能套用设备级阈值。
	registrationDeviceFingerprintMissingMultiplier = 10
	registrationDeviceFingerprintMissingFloor      = 30
)

var ErrRegistrationDeviceFingerprintLimit = infraerrors.TooManyRequests(
	"REGISTRATION_DEVICE_LIMIT",
	"too many accounts registered from this device, please try again later",
)

// RegistrationDeviceFingerprintRepository 是注册设备指纹记录的持久化能力。
// 与 RegistrationIPQuotaRepository 一样采用可选接口：单测桩不实现时风控自动跳过。
type RegistrationDeviceFingerprintRepository interface {
	CountRegistrationDeviceFingerprints(ctx context.Context, fingerprint string, since time.Time) (int, error)
	RecordRegistrationDeviceFingerprint(ctx context.Context, fingerprint string, userID int64, registerIP string) error
}

type registrationDeviceFingerprintContextKey struct{}

// WithRegistrationDeviceFingerprint 把本次请求解析出的服务端指纹注入上下文，
// 供建号成功后写记录。用上下文传递是为了不改动 RegisterWithVerification 等
// 已有函数签名及其大量调用点/测试桩。
func WithRegistrationDeviceFingerprint(ctx context.Context, fingerprint string) context.Context {
	return context.WithValue(ctx, registrationDeviceFingerprintContextKey{}, fingerprint)
}

// RegistrationDeviceFingerprintFromContext 读取上下文中的服务端指纹。
func RegistrationDeviceFingerprintFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(registrationDeviceFingerprintContextKey{}).(string)
	return strings.TrimSpace(value)
}

// clampRegistrationDeviceFingerprintLimit 把后台配置收敛到 [1, Max]，0 保留为关闭。
func clampRegistrationDeviceFingerprintLimit(limit int) int {
	if limit <= 0 {
		return 0
	}
	if limit < RegistrationDeviceFingerprintLimitMin {
		return RegistrationDeviceFingerprintLimitMin
	}
	if limit > RegistrationDeviceFingerprintLimitMax {
		return RegistrationDeviceFingerprintLimitMax
	}
	return limit
}

// GetRegistrationDeviceFingerprintLimit 读取同一指纹 24 小时内允许的注册数上限。
// 0 表示关闭；设置缺失或非法时回退默认值。
func (s *SettingService) GetRegistrationDeviceFingerprintLimit(ctx context.Context) int {
	raw, found := s.getCachedSettingValue(ctx, SettingKeyRegistrationDeviceFingerprintLimit)
	if !found {
		return RegistrationDeviceFingerprintLimitDefault
	}
	limit, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return RegistrationDeviceFingerprintLimitDefault
	}
	return clampRegistrationDeviceFingerprintLimit(limit)
}

// NormalizeClientDeviceFingerprint 清洗前端上报值：只保留十六进制字符并转小写。
// 长度不足视为无效，避免攻击者用 "a" 这类固定短值污染计数桶。
func NormalizeClientDeviceFingerprint(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) > registrationDeviceFingerprintClientMaxLength {
		raw = raw[:registrationDeviceFingerprintClientMaxLength]
	}
	var b strings.Builder
	b.Grow(len(raw))
	for _, r := range raw {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
			b.WriteRune(r)
		}
	}
	normalized := strings.ToLower(b.String())
	if len(normalized) < registrationDeviceFingerprintClientMinLength {
		return ""
	}
	return normalized
}

// BuildRegistrationDeviceFingerprint 生成落库用的服务端指纹。
// clientReported 表示前端是否提供了合法指纹；为 false 时调用方需放宽阈值。
func BuildRegistrationDeviceFingerprint(clientFingerprint, userAgent string) (fingerprint string, clientReported bool) {
	normalized := NormalizeClientDeviceFingerprint(clientFingerprint)
	seed := normalized
	if seed == "" {
		seed = "missing-client-fingerprint"
	}
	sum := sha256.Sum256([]byte(seed + "|" + strings.TrimSpace(userAgent)))
	return hex.EncodeToString(sum[:]), normalized != ""
}

func (s *AuthService) registrationDeviceFingerprintRepo() (RegistrationDeviceFingerprintRepository, bool) {
	if s == nil || s.userRepo == nil {
		return nil, false
	}
	repo, ok := s.userRepo.(RegistrationDeviceFingerprintRepository)
	return repo, ok
}

// RegistrationDeviceFingerprintLimit 返回当前生效阈值（0 = 关闭）。
func (s *AuthService) RegistrationDeviceFingerprintLimit(ctx context.Context) int {
	if s == nil || s.settingService == nil {
		return RegistrationDeviceFingerprintLimitDefault
	}
	return s.settingService.GetRegistrationDeviceFingerprintLimit(ctx)
}

// EnforceRegistrationDeviceFingerprint 在发码与注册两个入口做预检。
// 查询失败时 fail-open（记录告警），避免存储故障导致全站无法注册。
func (s *AuthService) EnforceRegistrationDeviceFingerprint(ctx context.Context, fingerprint string, clientReported bool) error {
	limit := s.RegistrationDeviceFingerprintLimit(ctx)
	if limit <= 0 || strings.TrimSpace(fingerprint) == "" {
		return nil
	}
	repo, ok := s.registrationDeviceFingerprintRepo()
	if !ok {
		return nil
	}

	effectiveLimit := limit
	if !clientReported {
		effectiveLimit = limit * registrationDeviceFingerprintMissingMultiplier
		if effectiveLimit < registrationDeviceFingerprintMissingFloor {
			effectiveLimit = registrationDeviceFingerprintMissingFloor
		}
	}

	count, err := repo.CountRegistrationDeviceFingerprints(ctx, fingerprint, time.Now().Add(-RegistrationDeviceFingerprintWindow))
	if err != nil {
		slog.Warn("registration.device_fingerprint_count_failed", "error", err)
		return nil
	}
	if count >= effectiveLimit {
		slog.Warn("registration.device_fingerprint_rejected",
			"count", count, "limit", effectiveLimit, "client_reported", clientReported)
		return ErrRegistrationDeviceFingerprintLimit
	}
	return nil
}

// RecordRegistrationDeviceFingerprint 建号成功后记录指纹，供后续注册关联计数。
// 只记日志不返回错误：偶发写入失败只让一次注册逃过统计，不应回滚已建账号。
func (s *AuthService) RecordRegistrationDeviceFingerprint(ctx context.Context, userID int64, registerIP string) {
	fingerprint := RegistrationDeviceFingerprintFromContext(ctx)
	if fingerprint == "" || s.RegistrationDeviceFingerprintLimit(ctx) <= 0 {
		return
	}
	repo, ok := s.registrationDeviceFingerprintRepo()
	if !ok {
		return
	}
	if err := repo.RecordRegistrationDeviceFingerprint(ctx, fingerprint, userID, registerIP); err != nil {
		slog.Warn("registration.device_fingerprint_record_failed", "user_id", userID, "error", err)
	}
}
