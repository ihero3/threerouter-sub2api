package service

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/locallimit"
)

// 注册设备指纹风控。
//
// 背景：代理池可以让每次注册都换一个真实出口 IP，单 IP 配额/限流全部失效，
// 但同一台设备连续注册时浏览器侧信号是稳定的。这里把前端上报的
// UA + 时区 + 语言 + 屏幕 + canvas 摘要，与服务端可信的 User-Agent 再混合
// 一次 SHA-256，按「同一指纹 1 小时内成功注册数」做关联拦截。
//
// 边界：
//   - 指纹由前端上报，可被主动随机化；本机制用于抬高对抗成本、串联跨 IP
//     注册，不是强身份认证。需要更强保证时应再叠加行为/支付账号维度。
//   - 前端未上报或上报非法时仍会生成「UA 维度」的降级指纹参与计数，但不再
//     放宽阈值：缺失指纹与正常指纹使用同一个 limit，避免「不传字段」成为绕过
//     口子；UA 派生降级指纹只是聚合维度不同，阈值一致。
//   - 计数只统计成功建号；记录写入失败不影响注册（fail-open），但会告警。

const (
	// SettingKeyRegistrationDeviceFingerprintLimit 是后台设置项：0 表示关闭。
	SettingKeyRegistrationDeviceFingerprintLimit = "registration_device_fingerprint_limit"

	RegistrationDeviceFingerprintLimitDefault = 2
	RegistrationDeviceFingerprintLimitMin     = 1
	RegistrationDeviceFingerprintLimitMax     = 50
	RegistrationDeviceFingerprintWindow       = 1 * time.Hour

	registrationDeviceFingerprintClientMinLength = 16
	registrationDeviceFingerprintClientMaxLength = 64
)

var ErrRegistrationDeviceFingerprintLimit = infraerrors.TooManyRequests(
	"REGISTRATION_DEVICE_LIMIT",
	"too many accounts registered from this device, please try again later",
)

var (
	// ErrRegistrationDeviceSignalMismatch UA 分字段哈希与请求头不一致。
	ErrRegistrationDeviceSignalMismatch = infraerrors.TooManyRequests(
		"REGISTRATION_DEVICE_SIGNAL_MISMATCH",
		"device verification does not match this browser; reload the page and try again",
	)
	// ErrRegistrationDeviceSignalEntropy 同一 IP 短窗口内出现过多不同设备信号，判定为随机化。
	ErrRegistrationDeviceSignalEntropy = infraerrors.TooManyRequests(
		"REGISTRATION_DEVICE_SIGNAL_ENTROPY",
		"too many device variations from this network; please try again later",
	)
)

// DeviceFingerprintSignals 前端分字段上报的信号摘要（均为 32 位 hex）。
// 服务端只校验 UA 与请求头一致，并把整组摘要当作关联/熵检测信号，不还原原始值。
type DeviceFingerprintSignals struct {
	UA     string `json:"ua"`
	TZ     string `json:"tz"`
	Lang   string `json:"lang"`
	Screen string `json:"screen"`
	Canvas string `json:"canvas"`
}

const (
	deviceSignalEntropyWindow    = 10 * time.Minute
	deviceSignalEntropyThreshold = 8
	deviceSignalEntropyMaxKeys   = 4096
)

type deviceSignalWindow struct {
	seen      map[string]struct{}
	expiresAt time.Time
}

type deviceSignalEntropyTracker struct {
	mu      sync.Mutex
	windows map[string]*deviceSignalWindow
}

func (t *deviceSignalEntropyTracker) observe(ip, signalKey string) bool {
	if t == nil || strings.TrimSpace(ip) == "" {
		return true
	}
	now := time.Now()
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.windows == nil {
		t.windows = make(map[string]*deviceSignalWindow)
	}
	if len(t.windows) >= deviceSignalEntropyMaxKeys {
		for key, window := range t.windows {
			if now.After(window.expiresAt) {
				delete(t.windows, key)
			}
		}
		if len(t.windows) >= deviceSignalEntropyMaxKeys {
			return true
		}
	}
	key := hashRegistrationProofPart(ip)
	window := t.windows[key]
	if window == nil || now.After(window.expiresAt) {
		window = &deviceSignalWindow{seen: make(map[string]struct{}), expiresAt: now.Add(deviceSignalEntropyWindow)}
		t.windows[key] = window
	}
	window.seen[signalKey] = struct{}{}
	return len(window.seen) < deviceSignalEntropyThreshold
}

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

// GetRegistrationDeviceFingerprintLimit 读取同一指纹 1 小时内允许的注册数上限。
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
// clientReported 表示前端是否提供了合法指纹；调用方不再据此放宽阈值。
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

// fnv1aDeviceSignal 与前端 utils/deviceFingerprint.ts 的 FNV-1a 实现逐字节对齐，
// 使用 UTF-16 code unit 的低 8 位（与 JS charCodeAt(i) & 0xff 一致）。
func fnv1aDeviceSignal(input string, seed uint32) uint32 {
	hash := seed
	for _, unit := range utf16.Encode([]rune(input)) {
		hash ^= uint32(unit & 0xff)
		hash *= 0x01000193
	}
	return hash
}

// HashDeviceFingerprintSignal 生成单个前端信号摘要（32 位 hex）。
func HashDeviceFingerprintSignal(value string) string {
	seeds := [...]uint32{0x811c9dc5, 0x9e3779b9, 0x85ebca6b, 0xc2b2ae35}
	var b strings.Builder
	for _, seed := range seeds {
		part := strconv.FormatUint(uint64(fnv1aDeviceSignal(value, seed)), 16)
		for len(part) < 8 {
			part = "0" + part
		}
		b.WriteString(part)
	}
	return b.String()
}

// DeviceFingerprintSignalKey 把一组分字段摘要压成熵检测用的关联键。
func DeviceFingerprintSignalKey(signals *DeviceFingerprintSignals) string {
	if signals == nil {
		return ""
	}
	raw := strings.ToLower(strings.Join([]string{
		signals.UA, signals.TZ, signals.Lang, signals.Screen, signals.Canvas,
	}, "|"))
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// ValidateRegistrationDeviceSignals 校验 UA 摘要与请求头一致，并对同一 IP
// 短窗口内的随机化做熵检测。signals 为空时保持旧客户端兼容，不新增拦截。
func (s *AuthService) ValidateRegistrationDeviceSignals(ctx context.Context, signals *DeviceFingerprintSignals, requestUA, clientIP string) error {
	_ = ctx
	if signals == nil {
		return nil
	}
	expectedUA := HashDeviceFingerprintSignal(requestUA)
	if len(signals.UA) != len(expectedUA) || subtle.ConstantTimeCompare([]byte(signals.UA), []byte(expectedUA)) != 1 {
		return ErrRegistrationDeviceSignalMismatch
	}
	if s == nil {
		return nil
	}
	s.deviceSignalEntropyOnce.Do(func() {
		s.deviceSignalEntropy = &deviceSignalEntropyTracker{windows: make(map[string]*deviceSignalWindow)}
	})
	if !s.deviceSignalEntropy.observe(clientIP, DeviceFingerprintSignalKey(signals)) {
		return ErrRegistrationDeviceSignalEntropy
	}
	return nil
}

// RegistrationDeviceFingerprintLimit 返回当前生效阈值（0 = 关闭）。
func (s *AuthService) RegistrationDeviceFingerprintLimit(ctx context.Context) int {
	if s == nil || s.settingService == nil {
		return RegistrationDeviceFingerprintLimitDefault
	}
	return s.settingService.GetRegistrationDeviceFingerprintLimit(ctx)
}

// EnforceRegistrationDeviceFingerprint 在发码与注册两个入口做预检。
//
// 缺失指纹不再放宽阈值：clientReported 仅为兼容既有调用点而保留，不参与判定，
// 只在日志中作为观测维度区分真实上报与 UA 降级桶。UA 派生降级指纹仍照常参与
// 计数，只是与正常指纹使用同一个 limit，避免「不传字段」成为绕过口子。
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

	count, err := repo.CountRegistrationDeviceFingerprints(ctx, fingerprint, time.Now().Add(-RegistrationDeviceFingerprintWindow))
	if err != nil {
		// 计数存储故障时不全放行：退化为进程内窗口，避免攻击诱导 DB 异常后批量绕过。
		slog.Warn("registration.device_fingerprint_count_failed, using local fallback", "error", err)
		s.deviceFingerprintOnce.Do(func() {
			if s.deviceFingerprintFallback == nil {
				s.deviceFingerprintFallback = locallimit.New(8192)
			}
		})
		if !s.deviceFingerprintFallback.Allow("registration-device:"+fingerprint, limit, RegistrationDeviceFingerprintWindow, time.Now()) {
			return ErrRegistrationDeviceFingerprintLimit
		}
		return nil
	}
	if count >= limit {
		slog.Warn("registration.device_fingerprint_rejected",
			"count", count, "limit", limit, "client_reported", clientReported)
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
