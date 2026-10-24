package handler

import (
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// TeamHandler handles team-related requests
type TeamHandler struct {
	teamService service.TeamService
	userLookup  service.UserLookupRepository
}

// NewTeamHandler creates a new TeamHandler
func NewTeamHandler(teamService service.TeamService, userLookup service.UserLookupRepository) *TeamHandler {
	return &TeamHandler{
		teamService: teamService,
		userLookup:  userLookup,
	}
}

// CreateTeamRequest represents the create team request payload
type CreateTeamRequest struct {
	Name string `json:"name" binding:"required"`
	Slug string `json:"slug" binding:"required"`
}

// UpdateTeamRequest represents the update team request payload
type UpdateTeamRequest struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Timezone    string                 `json:"timezone"`
	Language    string                 `json:"language"`
	Settings    map[string]interface{} `json:"settings"`
	Status      *string                `json:"status"`
}

// List handles listing teams for the current user
// GET /api/v1/teams
func (h *TeamHandler) List(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	teams, err := h.teamService.ListTeams(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Paginated(c, teams, int64(len(teams)), 1, len(teams))
}

// Get handles getting a single team
// GET /api/v1/teams/:id
func (h *TeamHandler) Get(c *gin.Context) {
	teamID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid team ID")
		return
	}

	team, err := h.teamService.GetTeam(c.Request.Context(), teamID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, team)
}

// Create handles creating a new team
// POST /api/v1/teams
func (h *TeamHandler) Create(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req CreateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	team, err := h.teamService.CreateTeam(c.Request.Context(), subject.UserID, req.Name, req.Slug)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, team)
}

// Update handles updating a team
// PUT /api/v1/teams/:id
func (h *TeamHandler) Update(c *gin.Context) {
	teamID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid team ID")
		return
	}

	var req UpdateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	team, err := h.teamService.UpdateTeam(c.Request.Context(), teamID, req.Name, req.Description, req.Timezone, req.Language, req.Settings, req.Status)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, team)
}

// Delete handles deleting a team
// DELETE /api/v1/teams/:id
func (h *TeamHandler) Delete(c *gin.Context) {
	teamID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid team ID")
		return
	}

	if err := h.teamService.DeleteTeam(c.Request.Context(), teamID); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Team deleted successfully"})
}

// TeamMemberResponse represents a team member with user info for API response
type TeamMemberResponse struct {
	ID           int64          `json:"id"`
	TeamID       int64          `json:"team_id"`
	UserID       int64          `json:"user_id"`
	Role         string         `json:"role"`
	Status       string         `json:"status"`
	DisplayName  string         `json:"display_name"`
	DepartmentID *int64         `json:"department_id"`
	ConsumerID   *int64         `json:"consumer_id"`
	JoinedAt     time.Time      `json:"joined_at"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	User         *UserBasicInfo `json:"user,omitempty"`
}

// UserBasicInfo represents basic user info for embedding in responses
type UserBasicInfo struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

func memberToResponse(m *service.TeamMember, user *service.User) TeamMemberResponse {
	resp := TeamMemberResponse{
		ID:           m.ID,
		TeamID:       m.TeamID,
		UserID:       m.UserID,
		Role:         m.Role,
		Status:       m.Status,
		DisplayName:  m.DisplayName,
		DepartmentID: m.DepartmentID,
		ConsumerID:   m.ConsumerID,
		JoinedAt:     m.JoinedAt,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
	if user != nil {
		// 显示名优先级：成员 display_name > 用户 username
		displayName := m.DisplayName
		if displayName == "" {
			displayName = user.Username
		}
		resp.User = &UserBasicInfo{
			ID:        user.ID,
			Name:      displayName,
			Username:  user.Username,
			Email:     user.Email,
			AvatarURL: user.AvatarURL,
		}
	}
	return resp
}

// ListMembers handles listing team members
// GET /api/v1/teams/:id/members
func (h *TeamHandler) ListMembers(c *gin.Context) {
	teamID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid team ID")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 1000 {
		pageSize = 20
	}

	p := pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	}

	members, total, err := h.teamService.ListMembers(c.Request.Context(), teamID, p)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Enrich members with user info
	resp := make([]TeamMemberResponse, 0, len(members))
	for _, m := range members {
		user, _ := h.userLookup.GetByID(c.Request.Context(), m.UserID)
		resp = append(resp, memberToResponse(m, user))
	}

	response.Paginated(c, resp, int64(total), page, pageSize)
}

// InviteMemberRequest represents the invite member request payload
type InviteMemberRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Role        string `json:"role"`
	DisplayName string `json:"display_name"`
}

// InviteMember handles inviting a member to the team
// POST /api/v1/teams/:id/members/invite
func (h *TeamHandler) InviteMember(c *gin.Context) {
	teamID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid team ID")
		return
	}

	var req InviteMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if req.Role == "" {
		req.Role = "member"
	}
	// owner 只能由 owner 授予：admin 邀请/改派 owner 会造成团队内权限提升
	// （新 owner 可删除整个团队）。调用方角色由团队权限中间件写入上下文。
	if req.Role == "owner" && !isCallerTeamOwner(c) {
		response.Forbidden(c, "Only the team owner can grant the owner role")
		return
	}

	member, err := h.teamService.InviteMember(c.Request.Context(), teamID, req.Email, req.Role, req.DisplayName)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, member)
}

// isCallerTeamOwner 判断当前调用方在该团队中的角色是否为 owner。
// 角色由团队权限中间件（RequireAdminOrAbove/RequireOwner 等）写入上下文。
func isCallerTeamOwner(c *gin.Context) bool {
	role, ok := middleware2.GetTeamRoleFromContext(c)
	return ok && role == middleware2.TeamRoleOwner
}

// UpdateMemberRequest represents the update member request payload
type UpdateMemberRequest struct {
	Role   *string `json:"role,omitempty"`
	Status *string `json:"status,omitempty"`
}

// UpdateMember handles updating a team member's role and/or status
// PUT /api/v1/teams/:id/members/:member_id
func (h *TeamHandler) UpdateMember(c *gin.Context) {
	teamID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid team ID")
		return
	}

	memberID, err := strconv.ParseInt(c.Param("member_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid member ID")
		return
	}

	var req UpdateMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if req.Role == nil && req.Status == nil {
		response.BadRequest(c, "No fields to update")
		return
	}

	if req.Role != nil {
		// 与 InviteMember 同口径：owner 角色只能由 owner 授予，防止 admin 自我提权。
		if *req.Role == "owner" && !isCallerTeamOwner(c) {
			response.Forbidden(c, "Only the team owner can grant the owner role")
			return
		}
		if err := h.teamService.UpdateMemberRole(c.Request.Context(), teamID, memberID, *req.Role); err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}
	if req.Status != nil {
		if err := h.teamService.UpdateMemberStatus(c.Request.Context(), teamID, memberID, *req.Status); err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}

	response.Success(c, gin.H{"message": "Member updated successfully"})
}

// RemoveMember handles removing a member from the team
// DELETE /api/v1/teams/:id/members/:member_id
func (h *TeamHandler) RemoveMember(c *gin.Context) {
	teamID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid team ID")
		return
	}

	memberID, err := strconv.ParseInt(c.Param("member_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid member ID")
		return
	}

	if err := h.teamService.RemoveMember(c.Request.Context(), teamID, memberID); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Member removed successfully"})
}
