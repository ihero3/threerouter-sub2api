package admin

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

type UpdateTicketStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

type AddTicketMessageRequest struct {
	Content string `json:"content" binding:"required"`
}

// List handles listing all tickets.
// GET /api/v1/admin/tickets
func (h *TicketHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	params := pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.DefaultQuery("sort_by", "updated_at"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}
	search := strings.TrimSpace(c.Query("search"))
	if len(search) > 200 {
		search = search[:200]
	}
	items, result, err := h.ticketService.List(c.Request.Context(), params, service.TicketListFilters{
		Status:   strings.TrimSpace(c.Query("status")),
		Category: strings.TrimSpace(c.Query("category")),
		Search:   search,
	})
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

// GetByID handles getting a ticket detail.
// GET /api/v1/admin/tickets/:id
func (h *TicketHandler) GetByID(c *gin.Context) {
	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || ticketID <= 0 {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}
	item, err := h.ticketService.GetByID(c.Request.Context(), ticketID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.TicketFromService(item))
}

// UpdateStatus handles updating ticket status.
// PUT /api/v1/admin/tickets/:id/status
func (h *TicketHandler) UpdateStatus(c *gin.Context) {
	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || ticketID <= 0 {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}
	var req UpdateTicketStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.ticketService.UpdateStatus(c.Request.Context(), ticketID, req.Status)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.TicketFromService(item))
}

// AddMessage handles adding an admin reply.
// POST /api/v1/admin/tickets/:id/messages
func (h *TicketHandler) AddMessage(c *gin.Context) {
	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || ticketID <= 0 {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	var req AddTicketMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	msg, err := h.ticketService.AddAdminMessage(c.Request.Context(), ticketID, subject.UserID, strings.TrimSpace(req.Content))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, dto.TicketMessageFromService(msg))
}
