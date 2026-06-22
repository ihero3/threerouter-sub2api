package handler

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type TicketHandler struct {
	ticketService *service.TicketService
}

func NewTicketHandler(ticketService *service.TicketService) *TicketHandler {
	return &TicketHandler{ticketService: ticketService}
}

type CreateTicketRequest struct {
	Contact  string `json:"contact" binding:"required"`
	Title    string `json:"title" binding:"required"`
	Category string `json:"category"`
	Priority string `json:"priority"`
	Content  string `json:"content" binding:"required"`
}

type AddTicketMessageRequest struct {
	Content string `json:"content" binding:"required"`
}

// Create handles creating a ticket for authenticated users.
// POST /api/v1/tickets
func (h *TicketHandler) Create(c *gin.Context) {
	var req CreateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	var userID *int64
	if subject, ok := middleware2.GetAuthSubjectFromContext(c); ok && subject.UserID > 0 {
		userID = &subject.UserID
	}

	created, err := h.ticketService.Create(c.Request.Context(), &service.CreateTicketInput{
		UserID:   userID,
		Contact:  req.Contact,
		Title:    req.Title,
		Category: req.Category,
		Priority: req.Priority,
		Content:  req.Content,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, dto.TicketFromService(created))
}

// ListMine handles listing current user's tickets.
// GET /api/v1/tickets/my
func (h *TicketHandler) ListMine(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	page, pageSize := response.ParsePagination(c)
	params := pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.DefaultQuery("sort_by", "updated_at"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}
	items, result, err := h.ticketService.List(c.Request.Context(), params, service.TicketListFilters{UserID: &subject.UserID})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.Ticket, 0, len(items))
	for i := range items {
		out = append(out, *dto.TicketFromService(&items[i]))
	}
	response.Paginated(c, out, result.Total, page, pageSize)
}

// GetMine handles getting a current user's ticket.
// GET /api/v1/tickets/:id
func (h *TicketHandler) GetMine(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || ticketID <= 0 {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}
	item, err := h.ticketService.GetForUser(c.Request.Context(), ticketID, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.TicketFromService(item))
}

// AddMessageMine handles adding a user reply.
// POST /api/v1/tickets/:id/messages
func (h *TicketHandler) AddMessageMine(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || ticketID <= 0 {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}
	var req AddTicketMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	msg, err := h.ticketService.AddUserMessage(c.Request.Context(), ticketID, subject.UserID, strings.TrimSpace(req.Content))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, dto.TicketMessageFromService(msg))
}
