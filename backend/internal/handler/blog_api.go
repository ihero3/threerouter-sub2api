package handler

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// BlogAPIHandler 提供 /v1/blog/*（OpenAI 兼容网关路径）下的博客发布接口，
// 面向自动化脚本与第三方系统：
//   - 调用方使用用户级 sk- API Key 认证（需通过现有 API Key 中间件鉴权）；
//   - 仅管理员用户的密钥允许创建/更新（发布），普通用户仅可读；
//   - 该端点不参与计费（由 API Key 中间件的 skipBilling 规则豁免）。
type BlogAPIHandler struct {
	blogService *service.BlogService
}

// NewBlogAPIHandler 创建博客 API Handler 实例。
func NewBlogAPIHandler(blogService *service.BlogService) *BlogAPIHandler {
	return &BlogAPIHandler{blogService: blogService}
}

// BlogPostRequest 是 POST /v1/blog/posts 创建博客的请求体（OpenAI 兼容风格）。
type BlogPostRequest struct {
	Title      string `json:"title" binding:"required"`
	Content    string `json:"content" binding:"required"`
	Summary    string `json:"summary"`
	CoverImage string `json:"cover_image"`
	Tags       string `json:"tags"`
	// Status 可选：published（默认，直接发布）/ draft（存为草稿）。
	Status string `json:"status" binding:"omitempty,oneof=draft published"`
}

// Create 通过 API 创建博客。默认直接发布（status=published），可传 status=draft 存为草稿。
// POST /v1/blog/posts
func (h *BlogAPIHandler) Create(c *gin.Context) {
	role, ok := middleware.GetUserRoleFromContext(c)
	if !ok || role != service.RoleAdmin {
		response.Forbidden(c, "Only admin users can publish blogs via the API")
		return
	}

	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not found in context")
		return
	}

	var req BlogPostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	status := req.Status
	if status == "" {
		status = service.BlogStatusPublished
	}

	actorID := subject.UserID
	created, err := h.blogService.Create(c.Request.Context(), &service.CreateBlogInput{
		Title:      req.Title,
		Content:    req.Content,
		Summary:    req.Summary,
		CoverImage: req.CoverImage,
		Tags:       req.Tags,
		Status:     status,
		ActorID:    &actorID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.BlogFromService(created))
}

// List 列出已发布的博客（按发布时间倒序），无需管理员权限。
// GET /v1/blog/posts
func (h *BlogAPIHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	items, paginationResult, err := h.blogService.ListPublished(
		c.Request.Context(),
		pagination.PaginationParams{
			Page:      page,
			PageSize:  pageSize,
			SortBy:    "published_at",
			SortOrder: "desc",
		},
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.UserBlog, 0, len(items))
	for i := range items {
		out = append(out, *dto.UserBlogFromService(&items[i]))
	}
	response.Paginated(c, out, paginationResult.Total, page, pageSize)
}

// GetByID 获取单篇博客详情。
// GET /v1/blog/posts/:id
func (h *BlogAPIHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid blog ID")
		return
	}
	item, err := h.blogService.GetByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.UserBlogFromService(item))
}
