//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

// 注册邮箱预检必须挡住重复注册：否则已注册用户会走到邮箱验证页，
// 而发送验证码接口对已注册邮箱静默不发信，用户既等不到验证码也看不到「去登录」提示。

func TestValidateRegistrationEmailForSignup_RejectsExistingEmail(t *testing.T) {
	svc := newAuthService(&userRepoStub{exists: true}, map[string]string{
		SettingKeyRegistrationEnabled: "true",
	}, nil, nil)

	err := svc.ValidateRegistrationEmailForSignup(context.Background(), "someone@gmail.com")
	require.ErrorIs(t, err, ErrEmailExists)
	// 前端靠 reason 驱动「去登录」引导，必须保持 409 EMAIL_EXISTS。
	require.Equal(t, "EMAIL_EXISTS", infraerrors.FromError(err).Reason)
}

func TestValidateRegistrationEmailForSignup_RejectsAliasVariant(t *testing.T) {
	svc := newAuthService(&userRepoStub{aliasExists: true}, map[string]string{
		SettingKeyRegistrationEnabled: "true",
	}, nil, nil)

	// 别名变体用 Gmail 点号形式；带 +tag 的写法会先被格式风险规则拒绝，走不到查重。
	err := svc.ValidateRegistrationEmailForSignup(context.Background(), "some.one@gmail.com")
	require.ErrorIs(t, err, ErrEmailExists)
}

func TestValidateRegistrationEmailForSignup_AllowsNewEmail(t *testing.T) {
	svc := newAuthService(&userRepoStub{}, map[string]string{
		SettingKeyRegistrationEnabled: "true",
	}, nil, nil)

	require.NoError(t, svc.ValidateRegistrationEmailForSignup(context.Background(), "newcomer@gmail.com"))
}

func TestValidateRegistrationEmailForSignup_FailsClosedOnLookupError(t *testing.T) {
	svc := newAuthService(&userRepoStub{existsErr: errors.New("db unavailable")}, map[string]string{
		SettingKeyRegistrationEnabled: "true",
	}, nil, nil)

	err := svc.ValidateRegistrationEmailForSignup(context.Background(), "someone@gmail.com")
	require.ErrorIs(t, err, ErrServiceUnavailable)
}

func TestValidateRegistrationEmailForSignup_RejectsAliasLookupError(t *testing.T) {
	svc := newAuthService(&userRepoStub{aliasErr: errors.New("db unavailable")}, map[string]string{
		SettingKeyRegistrationEnabled: "true",
	}, nil, nil)

	err := svc.ValidateRegistrationEmailForSignup(context.Background(), "someone@gmail.com")
	require.ErrorIs(t, err, ErrServiceUnavailable)
}
