package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// 发码额度绑定邮箱（一次人机验证 = 一个收件地址）。
//
// 验收标准：
//   - 同一 token 第二次给**另一个邮箱**发码必须被拒（堵住一个 token 无限给任意
//     邮箱发信、刷 SMTP 配额）；
//   - 同一 token 给**同一邮箱**重发不限次（邮件延迟时的正常重试不受影响）；
//   - Claim 不消费 token：注册接口仍能正常做一次性消费；
//   - 发信失败归还绑定后可以换邮箱重试；
//   - 指纹不匹配 / token 过期 / 绑定存储故障一律 fail-close。
//
// 注意：本文件**不加 //go:build unit**（与 registration_click_captcha_consume_test.go
// 同口径），桩复用那里的 captchaConsumeStore。
// ---------------------------------------------------------------------------

// TestRegistrationClickCaptcha_SendQuotaBlocksDifferentEmail 锁死"一次解题一个
// 收件地址"：修复前 SendVerifyCode 用 ValidateToken（只看不记），同一个 token
// 可以反复调用发码接口给任意邮箱发信、刷爆 SMTP 配额。
func TestRegistrationClickCaptcha_SendQuotaBlocksDifferentEmail(t *testing.T) {
	svc, _, token, ipHash := newCaptchaConsumeFixture(t)
	ctx := context.Background()

	require.NoError(t, svc.ClaimTokenSendQuota(ctx, token, "user@example.com", ipHash, ipHash))

	err := svc.ClaimTokenSendQuota(ctx, token, "victim@example.com", ipHash, ipHash)
	require.ErrorIs(t, err, ErrRegistrationClickCaptchaSendLimit)
}

// TestRegistrationClickCaptcha_SendQuotaAllowsResendToSameEmail 同一邮箱重发不限次：
// 邮件延迟是注册流程最常见的正常重试，不能被额度误伤。
func TestRegistrationClickCaptcha_SendQuotaAllowsResendToSameEmail(t *testing.T) {
	svc, _, token, ipHash := newCaptchaConsumeFixture(t)
	ctx := context.Background()

	require.NoError(t, svc.ClaimTokenSendQuota(ctx, token, "user@example.com", ipHash, ipHash))
	require.NoError(t, svc.ClaimTokenSendQuota(ctx, token, "user@example.com", ipHash, ipHash))

	// 大小写与首尾空白归一后视为同一邮箱。
	require.NoError(t, svc.ClaimTokenSendQuota(ctx, token, "  User@Example.COM ", ipHash, ipHash))
}

// TestRegistrationClickCaptcha_SendQuotaDoesNotConsumeToken 占用发码额度不等于消费
// token：正常注册流程是「发码（绑邮箱）→ 输入验证码 → 注册（消费 token）」，
// 若 Claim 顺手把 token 销毁，正常用户会在注册一步被拦。
func TestRegistrationClickCaptcha_SendQuotaDoesNotConsumeToken(t *testing.T) {
	svc, _, token, ipHash := newCaptchaConsumeFixture(t)
	ctx := context.Background()

	require.NoError(t, svc.ClaimTokenSendQuota(ctx, token, "user@example.com", ipHash, ipHash))

	payload, err := svc.ConsumeToken(ctx, token, ipHash, ipHash)
	require.NoError(t, err)
	require.NotNil(t, payload, "Claim 发码额度后 token 必须仍可被注册接口一次性消费")
}

// TestRegistrationClickCaptcha_ReleaseSendQuotaAllowsRetry 发信失败归还绑定：
// SMTP/队列瞬时故障后用户换邮箱（或同邮箱）重试不应被已占用的额度拦截。
func TestRegistrationClickCaptcha_ReleaseSendQuotaAllowsRetry(t *testing.T) {
	svc, _, token, ipHash := newCaptchaConsumeFixture(t)
	ctx := context.Background()

	require.NoError(t, svc.ClaimTokenSendQuota(ctx, token, "first@example.com", ipHash, ipHash))
	require.ErrorIs(t, svc.ClaimTokenSendQuota(ctx, token, "second@example.com", ipHash, ipHash), ErrRegistrationClickCaptchaSendLimit)

	require.NoError(t, svc.ReleaseTokenSendQuota(ctx, token))
	require.NoError(t, svc.ClaimTokenSendQuota(ctx, token, "second@example.com", ipHash, ipHash))
	require.ErrorIs(t, svc.ClaimTokenSendQuota(ctx, token, "third@example.com", ipHash, ipHash), ErrRegistrationClickCaptchaSendLimit)
}

// TestRegistrationClickCaptcha_SendQuotaValidatesTokenFirst 额度校验复用 token 的
// 全部有效性检查：指纹不匹配、过期、不存在的 token 都在绑定之前被拒，
// 垃圾 token 不会在 Redis 里留下绑定键。
func TestRegistrationClickCaptcha_SendQuotaValidatesTokenFirst(t *testing.T) {
	svc, store, token, ipHash := newCaptchaConsumeFixture(t)
	ctx := context.Background()

	t.Run("指纹不匹配", func(t *testing.T) {
		otherHash := HashFingerprint("198.51.100.7", "unit-test-ua")
		require.ErrorIs(t, svc.ClaimTokenSendQuota(ctx, token, "user@example.com", otherHash, otherHash), ErrRegistrationClickCaptchaTokenInvalid)
	})

	t.Run("不存在的 token", func(t *testing.T) {
		require.ErrorIs(t, svc.ClaimTokenSendQuota(ctx, "captcha-token-404", "user@example.com", ipHash, ipHash), ErrRegistrationClickCaptchaTokenInvalid)
	})

	t.Run("过期 token", func(t *testing.T) {
		expired := "captcha-token-expired"
		require.NoError(t, store.SetToken(ctx, expired, &ClickCaptchaTokenPayloadRef{
			IPHash:    ipHash,
			UAHash:    ipHash,
			CreatedAt: time.Now().Add(-2 * time.Minute).Unix(),
			ExpiresAt: time.Now().Add(-time.Minute).Unix(),
		}))
		require.ErrorIs(t, svc.ClaimTokenSendQuota(ctx, expired, "user@example.com", ipHash, ipHash), ErrRegistrationClickCaptchaTokenInvalid)
	})

	// 上面三种失败都不应产生绑定副作用：之后用正确指纹仍能绑定成功。
	require.NoError(t, svc.ClaimTokenSendQuota(ctx, token, "user@example.com", ipHash, ipHash))
}

// TestRegistrationClickCaptcha_SendQuotaFailCloseOnBindingError 绑定存储故障时
// fail-close：宁可让用户重做一次人机验证，也不能让"额度不可知"退化成无限发信。
func TestRegistrationClickCaptcha_SendQuotaFailCloseOnBindingError(t *testing.T) {
	svc, store, token, ipHash := newCaptchaConsumeFixture(t)
	store.sendEmailErr = errors.New("redis down")

	err := svc.ClaimTokenSendQuota(context.Background(), token, "user@example.com", ipHash, ipHash)
	require.ErrorIs(t, err, ErrRegistrationClickCaptchaTokenInvalid)
}
