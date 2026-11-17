package service

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestResetPasswordConsumesTokenOnlyAfterValidation 锁死一次性 token 的消费顺序。
//
// 旧实现先 ConsumePasswordResetToken（Lua GET+DEL 原子删除）再查用户、校验状态、
// 改密：DB 瞬时故障、账号停用、Update 失败都会让用户唯一的重置链接永久失效，
// 必须重走「忘记密码 → 收邮件」，而重发还受 30s 冷却与每日发信额度限制。
func TestResetPasswordConsumesTokenOnlyAfterValidation(t *testing.T) {
	src, err := os.ReadFile("auth_service.go")
	require.NoError(t, err)
	body := string(src)

	start := strings.Index(body, "func (s *AuthService) ResetPassword(")
	require.GreaterOrEqual(t, start, 0, "找不到 ResetPassword")
	fn := body[start:]
	if end := strings.Index(fn, "\nfunc "); end > 0 {
		fn = fn[:end]
	}

	verify := strings.Index(fn, "VerifyPasswordResetToken(ctx, email, token)")
	consume := strings.Index(fn, "ConsumePasswordResetToken(ctx, email, token)")
	getUser := strings.Index(fn, "GetByEmail(ctx, email)")
	update := strings.Index(fn, "userRepo.Update(ctx, user")

	require.GreaterOrEqual(t, verify, 0, "必须先用非破坏性的 Verify 校验 token")
	require.GreaterOrEqual(t, consume, 0, "必须在改密前原子消费 token 防重放")
	require.GreaterOrEqual(t, getUser, 0)
	require.GreaterOrEqual(t, update, 0)
	require.Less(t, verify, consume, "Verify 必须在 Consume 之前")
	require.Less(t, getUser, consume, "查用户必须在消费之前，避免 DB 抖动白耗 token")
	require.Less(t, consume, update, "Consume 应紧贴改密，缩小失败窗口")
}
