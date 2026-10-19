package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

var (
	ErrTeamNotFound   = infraerrors.NotFound("TEAM_NOT_FOUND", "team not found")
	ErrTeamSlugExists = infraerrors.Conflict("TEAM_SLUG_EXISTS", "team slug already exists")
	ErrTeamNameEmpty  = infraerrors.BadRequest("TEAM_NAME_EMPTY", "team name cannot be empty")
	ErrTeamSlugEmpty  = infraerrors.BadRequest("TEAM_SLUG_EMPTY", "team slug cannot be empty")
	ErrInvalidTeamID  = infraerrors.BadRequest("INVALID_TEAM_ID", "invalid team id")
	ErrInvalidOwnerID = infraerrors.BadRequest("INVALID_OWNER_ID", "invalid owner id")
	ErrNotTeamOwner   = infraerrors.Forbidden("NOT_TEAM_OWNER", "not team owner")
	ErrMemberExists   = infraerrors.Conflict("MEMBER_EXISTS", "user is already a member of this team")
	ErrInvalidRole    = infraerrors.BadRequest("INVALID_ROLE", "invalid team member role")
	// ErrOwnerMemberImmutable 团队必须始终有 owner：owner 成员记录不可被改角色/停用/移除，
	// 否则团队会变成无主状态（当前没有所有权转移 API）。
	ErrOwnerMemberImmutable = infraerrors.Forbidden("OWNER_MEMBER_IMMUTABLE", "cannot modify or remove the team owner member")
)

// TeamRepository 团队数据访问接口（在 service 包内定义以避免循环依赖）
type TeamRepository interface {
	Create(ctx context.Context, t *Team) error
	Update(ctx context.Context, t *Team) error
	Delete(ctx context.Context, id int64) error
	GetByID(ctx context.Context, id int64) (*Team, error)
	GetByOwnerID(ctx context.Context, ownerID int64) ([]*Team, error)
}

// TeamMemberRepository 团队成员数据访问接口（在 service 包内定义以避免循环依赖）
type TeamMemberRepository interface {
	Create(ctx context.Context, m *TeamMember) error
	Update(ctx context.Context, m *TeamMember) error
	Delete(ctx context.Context, id int64) error
	GetByID(ctx context.Context, id int64) (*TeamMember, error)
	GetByTeamAndUser(ctx context.Context, teamID, userID int64) (*TeamMember, error)
	ListByTeam(ctx context.Context, teamID int64, p pagination.PaginationParams) ([]*TeamMember, int, error)
	ListByUser(ctx context.Context, userID int64, p pagination.PaginationParams) ([]*TeamMember, int, error)
	UpdateRole(ctx context.Context, id int64, role string) error
	UpdateStatus(ctx context.Context, id int64, status string) error
}

// TeamService 团队服务接口
type TeamService interface {
	CreateTeam(ctx context.Context, userID int64, name, slug string) (*Team, error)
	UpdateTeam(ctx context.Context, teamID int64, name string, description string, timezone string, language string, settings map[string]interface{}, status *string) (*Team, error)
	DeleteTeam(ctx context.Context, teamID int64) error
	GetTeam(ctx context.Context, teamID int64) (*Team, error)
	GetTeamByOwner(ctx context.Context, ownerID int64) ([]*Team, error)
	ListTeams(ctx context.Context, userID int64) ([]*Team, error)

	// Team member management
	ListMembers(ctx context.Context, teamID int64, p pagination.PaginationParams) ([]*TeamMember, int, error)
	InviteMember(ctx context.Context, teamID int64, email string, role string, displayName string) (*TeamMember, error)
	UpdateMemberRole(ctx context.Context, teamID, memberID int64, role string) error
	UpdateMemberStatus(ctx context.Context, teamID, memberID int64, status string) error
	RemoveMember(ctx context.Context, teamID, memberID int64) error
}

// UserLookupRepository 用户查询接口（仅用于按邮箱/ID查找用户）
type UserLookupRepository interface {
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByID(ctx context.Context, id int64) (*User, error)
}

// teamService 团队服务实现
type teamService struct {
	teamRepo       TeamRepository
	memberRepo     TeamMemberRepository
	userRepo       UserLookupRepository
	emailService   *EmailService
	settingService *SettingService
}

// NewTeamService 创建团队服务实例
func NewTeamService(teamRepo TeamRepository, memberRepo TeamMemberRepository, userRepo UserLookupRepository, emailService *EmailService, settingService *SettingService) TeamService {
	return &teamService{
		teamRepo:       teamRepo,
		memberRepo:     memberRepo,
		userRepo:       userRepo,
		emailService:   emailService,
		settingService: settingService,
	}
}

// CreateTeam 创建团队并添加创建者为 owner
func (s *teamService) CreateTeam(ctx context.Context, userID int64, name, slug string) (*Team, error) {
	if userID <= 0 {
		return nil, ErrInvalidOwnerID
	}
	if name == "" {
		return nil, ErrTeamNameEmpty
	}
	if slug == "" {
		return nil, ErrTeamSlugEmpty
	}

	team := &Team{
		Name:    name,
		Slug:    slug,
		OwnerID: userID,
		Status:  "active",
	}

	if err := s.teamRepo.Create(ctx, team); err != nil {
		return nil, fmt.Errorf("create team: %w", err)
	}

	// 添加创建者为团队 owner 成员
	member := &TeamMember{
		TeamID:   team.ID,
		UserID:   userID,
		Role:     "owner",
		JoinedAt: time.Now(),
	}
	if err := s.memberRepo.Create(ctx, member); err != nil {
		return nil, fmt.Errorf("add team owner member: %w", err)
	}

	return team, nil
}

// UpdateTeam 更新团队信息
func (s *teamService) UpdateTeam(ctx context.Context, teamID int64, name string, description string, timezone string, language string, settings map[string]interface{}, status *string) (*Team, error) {
	if teamID <= 0 {
		return nil, ErrInvalidTeamID
	}

	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("get team: %w", err)
	}
	if team == nil {
		return nil, ErrTeamNotFound
	}

	if name != "" {
		team.Name = name
	}
	if description != "" {
		team.Description = description
	}
	if timezone != "" {
		team.Timezone = timezone
	}
	if language != "" {
		team.Language = language
	}
	if settings != nil {
		team.Settings = settings
	}
	if status != nil && *status != "" {
		team.Status = *status
	}

	if err := s.teamRepo.Update(ctx, team); err != nil {
		return nil, fmt.Errorf("update team: %w", err)
	}

	return team, nil
}

// DeleteTeam 删除团队（软删除）
func (s *teamService) DeleteTeam(ctx context.Context, teamID int64) error {
	if teamID <= 0 {
		return ErrInvalidTeamID
	}

	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		return fmt.Errorf("get team: %w", err)
	}
	if team == nil {
		return ErrTeamNotFound
	}

	if err := s.teamRepo.Delete(ctx, teamID); err != nil {
		return fmt.Errorf("delete team: %w", err)
	}

	return nil
}

// GetTeam 获取团队详情
func (s *teamService) GetTeam(ctx context.Context, teamID int64) (*Team, error) {
	if teamID <= 0 {
		return nil, ErrInvalidTeamID
	}

	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("get team: %w", err)
	}
	if team == nil {
		return nil, ErrTeamNotFound
	}

	return team, nil
}

// GetTeamByOwner 获取指定用户作为 owner 的团队列表
func (s *teamService) GetTeamByOwner(ctx context.Context, ownerID int64) ([]*Team, error) {
	if ownerID <= 0 {
		return nil, ErrInvalidOwnerID
	}

	teams, err := s.teamRepo.GetByOwnerID(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("get teams by owner: %w", err)
	}

	return teams, nil
}

// ListTeams 获取用户所属的所有团队列表
func (s *teamService) ListTeams(ctx context.Context, userID int64) ([]*Team, error) {
	if userID <= 0 {
		return nil, ErrInvalidOwnerID
	}

	// 获取用户作为成员的所有团队关系
	members, _, err := s.memberRepo.ListByUser(ctx, userID, pagination.PaginationParams{
		PageSize: 1000,
	})
	if err != nil {
		return nil, fmt.Errorf("list team memberships: %w", err)
	}

	if len(members) == 0 {
		// 用户没有团队，自动创建默认团队
		team, err := s.ensureDefaultTeam(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("auto create team: %w", err)
		}
		return []*Team{team}, nil
	}

	// 去重 teamID 并查询团队详情
	teamIDSet := make(map[int64]struct{}, len(members))
	for _, m := range members {
		teamIDSet[m.TeamID] = struct{}{}
	}

	teams := make([]*Team, 0, len(teamIDSet))
	for teamID := range teamIDSet {
		team, err := s.teamRepo.GetByID(ctx, teamID)
		if err != nil {
			continue // 跳过已删除或不可用的团队
		}
		if team != nil {
			teams = append(teams, team)
		}
	}

	return teams, nil
}

// ensureDefaultTeam 为没有团队的用户自动创建默认团队
func (s *teamService) ensureDefaultTeam(ctx context.Context, userID int64) (*Team, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return nil, fmt.Errorf("get user %d: %w", userID, err)
	}
	teamName := user.Username
	if teamName == "" {
		teamName = user.Email
	}
	if teamName == "" {
		teamName = fmt.Sprintf("team_%d", userID)
	}
	slug := fmt.Sprintf("team_%d", userID)
	team, err := s.CreateTeam(ctx, userID, teamName, slug)
	if err != nil {
		// slug 可能冲突，追加时间戳重试
		team, err = s.CreateTeam(ctx, userID, teamName, fmt.Sprintf("team_%d_%d", userID, time.Now().Unix()))
		if err != nil {
			return nil, err
		}
	}
	return team, nil
}

// ListMembers 获取团队成员列表
func (s *teamService) ListMembers(ctx context.Context, teamID int64, p pagination.PaginationParams) ([]*TeamMember, int, error) {
	if teamID <= 0 {
		return nil, 0, ErrInvalidTeamID
	}

	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		return nil, 0, fmt.Errorf("get team: %w", err)
	}
	if team == nil {
		return nil, 0, ErrTeamNotFound
	}

	members, total, err := s.memberRepo.ListByTeam(ctx, teamID, p)
	if err != nil {
		return nil, 0, fmt.Errorf("list team members: %w", err)
	}

	return members, total, nil
}

// InviteMember 通过邮箱邀请用户加入团队
func (s *teamService) InviteMember(ctx context.Context, teamID int64, email string, role string, displayName string) (*TeamMember, error) {
	if teamID <= 0 {
		return nil, ErrInvalidTeamID
	}
	if email == "" {
		return nil, ErrUserNotFound
	}

	validRoles := map[string]bool{"owner": true, "admin": true, "manager": true, "member": true, "viewer": true}
	if !validRoles[role] {
		role = "member"
	}

	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("get team: %w", err)
	}
	if team == nil {
		return nil, ErrTeamNotFound
	}

	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, ErrUserNotFound
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	existing, err := s.memberRepo.GetByTeamAndUser(ctx, teamID, user.ID)
	if err != nil {
		return nil, fmt.Errorf("check existing membership: %w", err)
	}
	if existing != nil {
		return nil, ErrMemberExists
	}

	member := &TeamMember{
		TeamID:      teamID,
		UserID:      user.ID,
		Role:        role,
		DisplayName: displayName,
		JoinedAt:    time.Now(),
	}
	if err := s.memberRepo.Create(ctx, member); err != nil {
		return nil, fmt.Errorf("create team member: %w", err)
	}

	// 发送通知邮件（best-effort，失败不影响邀请流程）
	s.notifyTeamInvite(ctx, user, team, role)

	return member, nil
}

// notifyTeamInvite 发送团队邀请通知邮件（失败仅记录日志，不影响邀请流程）
func (s *teamService) notifyTeamInvite(ctx context.Context, user *User, team *Team, role string) {
	if s.emailService == nil {
		return
	}
	siteName := "Sub2API"
	if s.settingService != nil {
		siteName = s.settingService.GetSiteName(ctx)
	}
	subject := fmt.Sprintf("[%s] You have been added to team \"%s\"", siteName, team.Name)
	body := buildTeamInviteEmailBody(siteName, team.Name, role, user.Username)
	if err := s.emailService.SendEmail(ctx, user.Email, subject, body); err != nil {
		slog.Warn("failed to send team invite notification email",
			"team_id", team.ID,
			"user_id", user.ID,
			"email", user.Email,
			"error", err,
		)
	}
}

// buildTeamInviteEmailBody 构建团队邀请通知邮件 HTML 正文
func buildTeamInviteEmailBody(siteName, teamName, role, userName string) string {
	return fmt.Sprintf(`<html><body style="font-family: Arial, Helvetica, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px; color: #333;">
<h2 style="color: #1a1a1a;">Hello %s,</h2>
<p>You have been added to the team <strong>&quot;%s&quot;</strong> on %s with the role of <strong>%s</strong>.</p>
<p>You can now access the team's resources and collaborate with other team members.</p>
<p>If you were not expecting this invitation, please contact the team administrator.</p>
<hr style="border: none; border-top: 1px solid #eee; margin: 20px 0;">
<p style="color: #888; font-size: 12px;">This is an automated notification from %s. Please do not reply directly.</p>
</body></html>`, userName, teamName, siteName, role, siteName)
}

// UpdateMemberRole 更新成员角色
func (s *teamService) UpdateMemberRole(ctx context.Context, teamID, memberID int64, role string) error {
	if teamID <= 0 {
		return ErrInvalidTeamID
	}
	if memberID <= 0 {
		return ErrInvalidTeamID
	}

	validRoles := map[string]bool{"owner": true, "admin": true, "manager": true, "member": true, "viewer": true}
	if !validRoles[role] {
		return ErrInvalidRole
	}

	member, err := s.memberRepo.GetByID(ctx, memberID)
	if err != nil {
		return fmt.Errorf("get member: %w", err)
	}
	if member == nil || member.TeamID != teamID {
		return ErrTeamNotFound
	}
	if member.Role == "owner" {
		return ErrOwnerMemberImmutable
	}

	if err := s.memberRepo.UpdateRole(ctx, memberID, role); err != nil {
		return fmt.Errorf("update member role: %w", err)
	}

	return nil
}

// UpdateMemberStatus 更新成员状态（active/inactive）
func (s *teamService) UpdateMemberStatus(ctx context.Context, teamID, memberID int64, status string) error {
	if teamID <= 0 || memberID <= 0 {
		return ErrInvalidTeamID
	}

	validStatuses := map[string]bool{"active": true, "inactive": true}
	if !validStatuses[status] {
		return fmt.Errorf("invalid status: %s", status)
	}

	member, err := s.memberRepo.GetByID(ctx, memberID)
	if err != nil {
		return fmt.Errorf("get member: %w", err)
	}
	if member == nil || member.TeamID != teamID {
		return ErrTeamNotFound
	}
	if member.Role == "owner" {
		return ErrOwnerMemberImmutable
	}

	if err := s.memberRepo.UpdateStatus(ctx, memberID, status); err != nil {
		return fmt.Errorf("update member status: %w", err)
	}

	return nil
}

// RemoveMember 移除团队成员
func (s *teamService) RemoveMember(ctx context.Context, teamID, memberID int64) error {
	if teamID <= 0 {
		return ErrInvalidTeamID
	}
	if memberID <= 0 {
		return ErrInvalidTeamID
	}

	member, err := s.memberRepo.GetByID(ctx, memberID)
	if err != nil {
		return fmt.Errorf("get member: %w", err)
	}
	if member == nil || member.TeamID != teamID {
		return ErrTeamNotFound
	}
	if member.Role == "owner" {
		return ErrOwnerMemberImmutable
	}

	if err := s.memberRepo.Delete(ctx, memberID); err != nil {
		return fmt.Errorf("remove member: %w", err)
	}

	return nil
}
