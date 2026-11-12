package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// 以下为 bare 层测试（无 //go:build unit 标签），不依赖真实 mmdb 文件：
// 通过可替换 lookupFn + 最小设置仓储 stub 覆盖「命中机房 IP → 难度升级」的完整链路，
// 难度升级本身另由纯函数 elevateCaptchaDifficulty 直接覆盖。

// asnCaptchaStore 是 bare 层最小 clickCaptchaCacheStore 实现，仅捕获 CreateChallenge 写入的载荷。
type asnCaptchaStore struct {
	challenges map[string]*ClickCaptchaChallengePayloadRef
}

func (s *asnCaptchaStore) SetChallenge(ctx context.Context, id string, p *ClickCaptchaChallengePayloadRef) error {
	if s.challenges == nil {
		s.challenges = map[string]*ClickCaptchaChallengePayloadRef{}
	}
	s.challenges[id] = p
	return nil
}
func (s *asnCaptchaStore) GetChallenge(ctx context.Context, id string) (*ClickCaptchaChallengePayloadRef, error) {
	return s.challenges[id], nil
}
func (s *asnCaptchaStore) DeleteChallenge(ctx context.Context, id string) error { return nil }
func (s *asnCaptchaStore) SetToken(ctx context.Context, token string, p *ClickCaptchaTokenPayloadRef) error {
	return nil
}
func (s *asnCaptchaStore) TakeToken(ctx context.Context, token string) (*ClickCaptchaTokenPayloadRef, error) {
	return nil, nil
}
func (s *asnCaptchaStore) PeekToken(ctx context.Context, token string) (*ClickCaptchaTokenPayloadRef, error) {
	return nil, nil
}
func (s *asnCaptchaStore) IncrAttempts(ctx context.Context, id string) (int64, error) { return 0, nil }
func (s *asnCaptchaStore) ClaimTokenSendEmail(ctx context.Context, token, email string) (string, error) {
	return email, nil
}

// asnSettingRepoStub 只实现 GetValue：命中给定 key 返回值，其余返回 ErrSettingNotFound，
// 模拟「设置项缺失 / DB 不可用」的 fail-open 场景。
type asnSettingRepoStub struct {
	values map[string]string
}

func (s *asnSettingRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if v, ok := s.values[key]; ok {
		return v, nil
	}
	return "", ErrSettingNotFound
}
func (s *asnSettingRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	return nil, ErrSettingNotFound
}
func (s *asnSettingRepoStub) Set(ctx context.Context, key, value string) error { return nil }
func (s *asnSettingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	return map[string]string{}, nil
}
func (s *asnSettingRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error { return nil }
func (s *asnSettingRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	return s.values, nil
}
func (s *asnSettingRepoStub) Delete(ctx context.Context, key string) error { return nil }

// newASNCaptchaServiceForTest 构造固定 image_click 模式、给定基础难度与清单、可模拟命中/未命中的服务。
func newASNCaptchaServiceForTest(baseDifficulty, list string, hitASN uint, hitOrg string, lookupOK bool) (*RegistrationClickCaptchaService, *asnCaptchaStore) {
	store := &asnCaptchaStore{}
	values := map[string]string{
		SettingKeyRegistrationCaptchaMode: RegistrationCaptchaModeImageClick,
	}
	if baseDifficulty != "" {
		values[SettingKeyRegistrationCaptchaDifficulty] = baseDifficulty
	}
	if list != "" {
		values[SettingKeyRegistrationDatacenterASNs] = list
	}
	settings := &SettingService{settingRepo: &asnSettingRepoStub{values: values}}
	svc := NewRegistrationClickCaptchaService(store, settings)
	svc.SetASNRiskService(&ASNRiskService{
		lookupFn: func(ip string) (ASNRecord, bool) {
			if !lookupOK {
				return ASNRecord{}, false
			}
			return ASNRecord{AutonomousSystemNumber: hitASN, AutonomousSystemOrganization: hitOrg}, true
		},
	})
	return svc, store
}

func TestElevateCaptchaDifficulty(t *testing.T) {
	// 命中机房 IP：只升不降
	require.Equal(t, RegistrationCaptchaDifficultyHigh, elevateCaptchaDifficulty(RegistrationCaptchaDifficultyLow, true))
	require.Equal(t, RegistrationCaptchaDifficultyHigh, elevateCaptchaDifficulty(RegistrationCaptchaDifficultyMedium, true))
	require.Equal(t, RegistrationCaptchaDifficultyHigh, elevateCaptchaDifficulty(RegistrationCaptchaDifficultyHigh, true))
	// 未命中：保持后台原难度
	require.Equal(t, RegistrationCaptchaDifficultyLow, elevateCaptchaDifficulty(RegistrationCaptchaDifficultyLow, false))
	require.Equal(t, RegistrationCaptchaDifficultyMedium, elevateCaptchaDifficulty(RegistrationCaptchaDifficultyMedium, false))
	require.Equal(t, RegistrationCaptchaDifficultyHigh, elevateCaptchaDifficulty(RegistrationCaptchaDifficultyHigh, false))
}

func TestWithClientIPForChallenge(t *testing.T) {
	ctx := WithClientIPForChallenge(context.Background(), "  203.0.113.9  ")
	require.Equal(t, "203.0.113.9", clientIPFromChallengeContext(ctx))
	// 未附加 IP 的 ctx 与 nil ctx 一律返回空
	require.Equal(t, "", clientIPFromChallengeContext(context.Background()))
	require.Equal(t, "", clientIPFromChallengeContext(nil))
	// nil ctx 也可安全附加
	require.Equal(t, "1.2.3.4", clientIPFromChallengeContext(WithClientIPForChallenge(nil, "1.2.3.4")))
}

func TestIsDatacenterIPForChallenge_FailOpen(t *testing.T) {
	clientCtx := WithClientIPForChallenge(context.Background(), "203.0.113.9")

	// 未注入 ASN 服务
	svc0, _ := newASNCaptchaServiceForTest(RegistrationCaptchaDifficultyLow, "16509", 16509, "AMAZON-AES", true)
	svc0.SetASNRiskService(nil)
	require.False(t, svc0.isDatacenterIPForChallenge(clientCtx))

	// ctx 未携带真实客户端 IP
	svc1, _ := newASNCaptchaServiceForTest(RegistrationCaptchaDifficultyLow, "16509", 16509, "AMAZON-AES", true)
	require.False(t, svc1.isDatacenterIPForChallenge(context.Background()))

	// 设置项缺失 / 读取失败（found=false）→ 按未命中
	svc2, _ := newASNCaptchaServiceForTest(RegistrationCaptchaDifficultyLow, "", 16509, "AMAZON-AES", true)
	require.False(t, svc2.isDatacenterIPForChallenge(clientCtx))

	// settingSvc 为 nil
	svc3 := NewRegistrationClickCaptchaService(&asnCaptchaStore{}, nil)
	svc3.SetASNRiskService(&ASNRiskService{lookupFn: func(ip string) (ASNRecord, bool) {
		return ASNRecord{AutonomousSystemNumber: 16509}, true
	}})
	require.False(t, svc3.isDatacenterIPForChallenge(clientCtx))

	// 私有 / 回环 / 非法 IP 一律未命中（即使 lookupFn 会返回命中记录）
	svc4, _ := newASNCaptchaServiceForTest(RegistrationCaptchaDifficultyLow, "16509", 16509, "AMAZON-AES", true)
	require.False(t, svc4.isDatacenterIPForChallenge(WithClientIPForChallenge(context.Background(), "10.0.0.1")))
	require.False(t, svc4.isDatacenterIPForChallenge(WithClientIPForChallenge(context.Background(), "not-an-ip")))
}

func TestIsDatacenterIPForChallenge_Hit(t *testing.T) {
	clientCtx := WithClientIPForChallenge(context.Background(), "203.0.113.9")

	// ASN 号命中
	svc, _ := newASNCaptchaServiceForTest(RegistrationCaptchaDifficultyLow, "16509,14061", 16509, "AMAZON-AES", true)
	require.True(t, svc.isDatacenterIPForChallenge(clientCtx))
	// 组织名片段命中
	svc2, _ := newASNCaptchaServiceForTest(RegistrationCaptchaDifficultyLow, "amazon", 99999, "AMAZON-AES", true)
	require.True(t, svc2.isDatacenterIPForChallenge(clientCtx))
	// 清单未命中该 ASN
	svc3, _ := newASNCaptchaServiceForTest(RegistrationCaptchaDifficultyLow, "digitalocean", 16509, "AMAZON-AES", true)
	require.False(t, svc3.isDatacenterIPForChallenge(clientCtx))
	// 空清单（管理员显式清空）→ 未命中
	svc4, _ := newASNCaptchaServiceForTest(RegistrationCaptchaDifficultyLow, " , , ", 16509, "AMAZON-AES", true)
	require.False(t, svc4.isDatacenterIPForChallenge(clientCtx))
	// 查询失败 → 未命中
	svc5, _ := newASNCaptchaServiceForTest(RegistrationCaptchaDifficultyLow, "16509", 0, "", false)
	require.False(t, svc5.isDatacenterIPForChallenge(clientCtx))
}

func TestCreateChallenge_DatacenterElevatesDifficulty(t *testing.T) {
	clientCtx := WithClientIPForChallenge(context.Background(), "203.0.113.9")
	cases := []struct {
		name     string
		base     string
		list     string
		hitASN   uint
		hitOrg   string
		lookupOK bool
		want     string
	}{
		{"low upgraded to high", RegistrationCaptchaDifficultyLow, "16509", 16509, "AMAZON-AES", true, RegistrationCaptchaDifficultyHigh},
		{"medium upgraded to high", RegistrationCaptchaDifficultyMedium, "amazon", 99999, "AMAZON-AES", true, RegistrationCaptchaDifficultyHigh},
		{"high stays high", RegistrationCaptchaDifficultyHigh, "16509", 16509, "AMAZON-AES", true, RegistrationCaptchaDifficultyHigh},
		{"miss keeps low", RegistrationCaptchaDifficultyLow, "digitalocean", 16509, "AMAZON-AES", true, RegistrationCaptchaDifficultyLow},
		{"lookup failure keeps medium", RegistrationCaptchaDifficultyMedium, "16509", 0, "", false, RegistrationCaptchaDifficultyMedium},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, store := newASNCaptchaServiceForTest(tc.base, tc.list, tc.hitASN, tc.hitOrg, tc.lookupOK)
			ch, err := svc.CreateChallenge(clientCtx, "ip-a", "ua-a")
			require.NoError(t, err)
			require.NotNil(t, ch)
			payload := store.challenges[ch.ChallengeID]
			require.NotNil(t, payload)
			require.Equal(t, tc.want, payload.Difficulty)
			// high 难度同时收紧单题失败次数上限，确认升级贯穿校验参数
			if tc.want == RegistrationCaptchaDifficultyHigh {
				require.Equal(t, int64(2), clickCaptchaMaxAttempts(payload.Difficulty))
			}
		})
	}
}

func TestCreateChallenge_NoASNServiceKeepsConfiguredDifficulty(t *testing.T) {
	// 未注入 ASN 服务（例如未放置 mmdb）：既有难度完全不变，行为向后兼容。
	store := &asnCaptchaStore{}
	svc := NewRegistrationClickCaptchaService(store, &SettingService{
		settingRepo: &asnSettingRepoStub{values: map[string]string{
			SettingKeyRegistrationCaptchaMode:        RegistrationCaptchaModeImageClick,
			SettingKeyRegistrationCaptchaDifficulty: RegistrationCaptchaDifficultyLow,
		}},
	})
	ch, err := svc.CreateChallenge(WithClientIPForChallenge(context.Background(), "203.0.113.9"), "ip-a", "ua-a")
	require.NoError(t, err)
	require.NotNil(t, ch)
	payload := store.challenges[ch.ChallengeID]
	require.NotNil(t, payload)
	require.Equal(t, RegistrationCaptchaDifficultyLow, payload.Difficulty)
}
