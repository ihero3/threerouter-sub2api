package service

import (
	"context"
	"errors"
	"testing"
)

// 这两个 stub 只实现 Create 路径实际会调用的方法；其余方法由嵌入的仓库接口
// （nil）提供，测试中不会被触发。用独立命名，避免与同包 unit 标签测试里的
// 其它桩类型重名。

type riskCreateUserRepo struct {
	UserRepository
	user *User
	err  error
}

func (r *riskCreateUserRepo) GetByID(context.Context, int64) (*User, error) {
	return r.user, r.err
}

type riskCreateAPIKeyRepo struct {
	APIKeyRepository
	created *APIKey
}

func (r *riskCreateAPIKeyRepo) ExistsByKey(context.Context, string) (bool, error) {
	return false, nil
}

func (r *riskCreateAPIKeyRepo) Create(_ context.Context, key *APIKey) error {
	r.created = key
	return nil
}

func newRiskCreateService(user *User) (*APIKeyService, *riskCreateAPIKeyRepo) {
	repo := &riskCreateAPIKeyRepo{}
	svc := &APIKeyService{
		apiKeyRepo: repo,
		userRepo:   &riskCreateUserRepo{user: user},
		// cfg/cache/groupRepo 等保持零值：Create 走自定义 Key 且不触发这些依赖。
	}
	return svc, repo
}

// TestAPIKeyServiceCreateBlocksRiskUsers 验证面板侧建 Key 入口对待复核账号
// 返回 ErrUserRiskAccessBlocked，且不落库；对正常 / 未知 / nil 用户不误拦。
func TestAPIKeyServiceCreateBlocksRiskUsers(t *testing.T) {
	cases := []struct {
		name        string
		user        *User
		wantBlocked bool
	}{
		{name: "suspect is blocked", user: &User{ID: 7, Status: StatusActive, RiskLevel: AffiliateRiskLevelSuspect}, wantBlocked: true},
		{name: "abnormal is blocked", user: &User{ID: 7, Status: StatusActive, RiskLevel: AffiliateRiskLevelAbnormal}, wantBlocked: true},
		{name: "normal empty is allowed", user: &User{ID: 7, Status: StatusActive, RiskLevel: ""}, wantBlocked: false},
		{name: "unknown level is allowed", user: &User{ID: 7, Status: StatusActive, RiskLevel: "future_level"}, wantBlocked: false},
		{name: "uppercase suspect is allowed", user: &User{ID: 7, Status: StatusActive, RiskLevel: "SUSPECT"}, wantBlocked: false},
		{name: "nil user is allowed", user: nil, wantBlocked: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, repo := newRiskCreateService(tc.user)
			custom := "sk-risktest-0123456789"
			key, err := svc.Create(context.Background(), 7, CreateAPIKeyRequest{
				Name:      "k",
				CustomKey: &custom,
			})

			if tc.wantBlocked {
				if !errors.Is(err, ErrUserRiskAccessBlocked) {
					t.Fatalf("Create() error = %v, want ErrUserRiskAccessBlocked", err)
				}
				if key != nil {
					t.Fatal("Create() must not return a key for a blocked user")
				}
				if repo.created != nil {
					t.Fatal("Create() must not persist a key for a blocked user")
				}
				return
			}

			if err != nil {
				t.Fatalf("Create() unexpected error for a non-risk user: %v", err)
			}
			if key == nil {
				t.Fatal("Create() returned a nil key for a non-risk user")
			}
			if repo.created == nil {
				t.Fatal("Create() did not persist the key for a non-risk user")
			}
		})
	}
}

// TestAPIKeyServiceCreateUserLookupErrorIsNotRiskBlock 说明 GetByID 出错是
// 独立的上游错误，不能误报成风险闸门错误。
func TestAPIKeyServiceCreateUserLookupErrorIsNotRiskBlock(t *testing.T) {
	repo := &riskCreateAPIKeyRepo{}
	svc := &APIKeyService{
		apiKeyRepo: repo,
		userRepo:   &riskCreateUserRepo{err: errors.New("db down")},
	}
	custom := "sk-risktest-0123456789"
	_, err := svc.Create(context.Background(), 7, CreateAPIKeyRequest{Name: "k", CustomKey: &custom})
	if err == nil {
		t.Fatal("Create() should surface the user lookup error")
	}
	if errors.Is(err, ErrUserRiskAccessBlocked) {
		t.Fatal("a user lookup error must not be reported as a risk block")
	}
}
