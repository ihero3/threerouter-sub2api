package middleware

import (
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// teamMemberPermissionChecker 基于 team_members 表实现 TeamPermissionChecker。
// 修复前该中间件从未接线，/teams/:id 下所有端点只校验 JWT，
// 任意登录用户可读写他人团队（IDOR）。
type teamMemberPermissionChecker struct {
	memberRepo service.TeamMemberRepository
}

// NewTeamPermissionMiddlewareFromRepo 用成员仓储构建团队权限中间件。
func NewTeamPermissionMiddlewareFromRepo(memberRepo service.TeamMemberRepository) *TeamPermissionMiddleware {
	return NewTeamPermissionMiddleware(&teamMemberPermissionChecker{memberRepo: memberRepo})
}

func (m *teamMemberPermissionChecker) CheckTeamPermission(ctx *gin.Context, userID int64, teamID int64, requiredRole TeamRole) (bool, TeamRole, TeamDataScope, error) {
	if m.memberRepo == nil || userID <= 0 || teamID <= 0 {
		return false, "", "", nil
	}
	member, err := m.memberRepo.GetByTeamAndUser(ctx.Request.Context(), teamID, userID)
	if err != nil {
		return false, "", "", err
	}
	// 仅 active 成员视为有效成员：inactive/removed 不授予任何权限。
	if member == nil || member.Status != "active" {
		return false, "", "", nil
	}
	role := normalizeTeamRole(member.Role)
	if rolePriority(role) < rolePriority(requiredRole) {
		return false, role, "", nil
	}
	return true, role, TeamDataScopeAll, nil
}

// normalizeTeamRole 归一化成员角色。历史数据允许 "manager"（service 层合法值），
// 按 member 级授权（最小权限）；无法识别的角色返回空（优先级 0，拒绝一切）。
func normalizeTeamRole(role string) TeamRole {
	switch TeamRole(role) {
	case TeamRoleOwner, TeamRoleAdmin, TeamRoleMember, TeamRoleViewer:
		return TeamRole(role)
	case "manager":
		return TeamRoleMember
	default:
		return ""
	}
}
