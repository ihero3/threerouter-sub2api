package middleware

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// fakeTeamMemberRepo 实现 service.TeamMemberRepository，仅支持 GetByTeamAndUser。
type fakeTeamMemberRepo struct {
	member *service.TeamMember
	err    error
}

func (f *fakeTeamMemberRepo) Create(context.Context, *service.TeamMember) error { return nil }
func (f *fakeTeamMemberRepo) Update(context.Context, *service.TeamMember) error { return nil }
func (f *fakeTeamMemberRepo) Delete(context.Context, int64) error               { return nil }
func (f *fakeTeamMemberRepo) GetByID(context.Context, int64) (*service.TeamMember, error) {
	return nil, nil
}
func (f *fakeTeamMemberRepo) GetByTeamAndUser(context.Context, int64, int64) (*service.TeamMember, error) {
	return f.member, f.err
}
func (f *fakeTeamMemberRepo) ListByTeam(context.Context, int64, pagination.PaginationParams) ([]*service.TeamMember, int, error) {
	return nil, 0, nil
}
func (f *fakeTeamMemberRepo) ListByUser(context.Context, int64, pagination.PaginationParams) ([]*service.TeamMember, int, error) {
	return nil, 0, nil
}
func (f *fakeTeamMemberRepo) UpdateRole(context.Context, int64, string) error   { return nil }
func (f *fakeTeamMemberRepo) UpdateStatus(context.Context, int64, string) error { return nil }

func newTeamPermTestContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/api/v1/teams/1", nil)
	return c
}

func TestTeamMemberPermissionChecker(t *testing.T) {
	active := func(role string) *service.TeamMember {
		return &service.TeamMember{TeamID: 1, UserID: 2, Role: role, Status: "active"}
	}
	tests := []struct {
		name         string
		member       *service.TeamMember
		repoErr      error
		requiredRole TeamRole
		wantAllowed  bool
		wantRole     TeamRole
	}{
		{name: "非成员拒绝", member: nil, requiredRole: TeamRoleViewer, wantAllowed: false},
		{name: "inactive 成员拒绝", member: &service.TeamMember{Role: "owner", Status: "inactive"}, requiredRole: TeamRoleViewer, wantAllowed: false},
		{name: "removed 成员拒绝", member: &service.TeamMember{Role: "owner", Status: "removed"}, requiredRole: TeamRoleViewer, wantAllowed: false},
		{name: "仓储错误向上传递", repoErr: errors.New("db down"), requiredRole: TeamRoleViewer, wantAllowed: false},
		{name: "viewer 可读", member: active("viewer"), requiredRole: TeamRoleViewer, wantAllowed: true, wantRole: TeamRoleViewer},
		{name: "viewer 不可管理", member: active("viewer"), requiredRole: TeamRoleAdmin, wantAllowed: false, wantRole: TeamRoleViewer},
		{name: "member 不可管理", member: active("member"), requiredRole: TeamRoleAdmin, wantAllowed: false, wantRole: TeamRoleMember},
		{name: "admin 可管理但非 owner", member: active("admin"), requiredRole: TeamRoleAdmin, wantAllowed: true, wantRole: TeamRoleAdmin},
		{name: "admin 不可执行 owner 操作", member: active("admin"), requiredRole: TeamRoleOwner, wantAllowed: false, wantRole: TeamRoleAdmin},
		{name: "owner 全权限", member: active("owner"), requiredRole: TeamRoleOwner, wantAllowed: true, wantRole: TeamRoleOwner},
		{name: "manager 按 member 级授权", member: active("manager"), requiredRole: TeamRoleMember, wantAllowed: true, wantRole: TeamRoleMember},
		{name: "manager 不可管理", member: active("manager"), requiredRole: TeamRoleAdmin, wantAllowed: false, wantRole: TeamRoleMember},
		{name: "未知角色拒绝", member: active("superuser"), requiredRole: TeamRoleViewer, wantAllowed: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := &teamMemberPermissionChecker{memberRepo: &fakeTeamMemberRepo{member: tt.member, err: tt.repoErr}}
			allowed, role, _, err := checker.CheckTeamPermission(newTeamPermTestContext(), 2, 1, tt.requiredRole)
			if tt.repoErr != nil {
				if err == nil {
					t.Fatalf("expected repo error to propagate")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if allowed != tt.wantAllowed {
				t.Fatalf("allowed = %v, want %v", allowed, tt.wantAllowed)
			}
			if tt.wantAllowed && role != tt.wantRole {
				t.Fatalf("role = %q, want %q", role, tt.wantRole)
			}
		})
	}

	// nil 仓储必须拒绝（防止配置遗漏导致放行）
	t.Run("nil 仓储拒绝", func(t *testing.T) {
		checker := &teamMemberPermissionChecker{}
		allowed, _, _, err := checker.CheckTeamPermission(newTeamPermTestContext(), 2, 1, TeamRoleViewer)
		if err != nil || allowed {
			t.Fatalf("nil repo must deny, got allowed=%v err=%v", allowed, err)
		}
	})
}
