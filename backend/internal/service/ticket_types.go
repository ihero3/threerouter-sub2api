package service

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	TicketStatusOpen     = "open"
	TicketStatusPending  = "pending"
	TicketStatusAnswered = "answered"
	TicketStatusClosed   = "closed"

	TicketPriorityLow    = "low"
	TicketPriorityNormal = "normal"
	TicketPriorityHigh   = "high"
	TicketPriorityUrgent = "urgent"

	TicketCategoryAccount = "account"
	TicketCategoryBilling = "billing"
	TicketCategoryAPI     = "api"
	TicketCategoryModel   = "model"
	TicketCategoryOther   = "other"

	TicketAuthorUser  = "user"
	TicketAuthorAdmin = "admin"
)

var (
	ErrTicketNotFound        = infraerrors.NotFound("TICKET_NOT_FOUND", "ticket not found")
	ErrTicketInvalidTitle    = infraerrors.BadRequest("TICKET_INVALID_TITLE", "ticket title must be 1-200 characters")
	ErrTicketInvalidContact  = infraerrors.BadRequest("TICKET_INVALID_CONTACT", "contact must be 1-255 characters")
	ErrTicketContentRequired = infraerrors.BadRequest("TICKET_CONTENT_REQUIRED", "message content is required")
	ErrTicketInvalidCategory = infraerrors.BadRequest("TICKET_INVALID_CATEGORY", "invalid ticket category")
	ErrTicketInvalidPriority = infraerrors.BadRequest("TICKET_INVALID_PRIORITY", "invalid ticket priority")
	ErrTicketInvalidStatus   = infraerrors.BadRequest("TICKET_INVALID_STATUS", "invalid ticket status")
	ErrTicketClosed          = infraerrors.BadRequest("TICKET_CLOSED", "ticket is closed")
	ErrTicketForbidden       = infraerrors.Forbidden("TICKET_FORBIDDEN", "ticket access denied")
)

type Ticket struct {
	ID        int64
	UserID    *int64
	Contact   string
	Title     string
	Category  string
	Priority  string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
	Messages  []TicketMessage
}

type TicketMessage struct {
	ID         int64
	TicketID   int64
	UserID     *int64
	AuthorType string
	Content    string
	CreatedAt  time.Time
}

type CreateTicketInput struct {
	UserID   *int64
	Contact  string
	Title    string
	Category string
	Priority string
	Content  string
}

type AddTicketMessageInput struct {
	UserID     *int64
	AuthorType string
	Content    string
}

type TicketListFilters struct {
	UserID   *int64
	Status   string
	Category string
	Search   string
}

type TicketRepository interface {
	Create(ctx context.Context, ticket *Ticket, firstMessage string) error
	GetByID(ctx context.Context, id int64) (*Ticket, error)
	List(ctx context.Context, params pagination.PaginationParams, filters TicketListFilters) ([]Ticket, *pagination.PaginationResult, error)
	UpdateStatus(ctx context.Context, id int64, status string) (*Ticket, error)
	AddMessage(ctx context.Context, ticketID int64, message *TicketMessage) error
}
