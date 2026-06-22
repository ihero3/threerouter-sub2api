package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

type TicketService struct {
	ticketRepo TicketRepository
}

func NewTicketService(ticketRepo TicketRepository) *TicketService {
	return &TicketService{ticketRepo: ticketRepo}
}

func (s *TicketService) Create(ctx context.Context, input *CreateTicketInput) (*Ticket, error) {
	contact := strings.TrimSpace(input.Contact)
	title := strings.TrimSpace(input.Title)
	content := strings.TrimSpace(input.Content)
	if title == "" || len(title) > 200 {
		return nil, ErrTicketInvalidTitle
	}
	if contact == "" || len(contact) > 255 {
		return nil, ErrTicketInvalidContact
	}
	if content == "" {
		return nil, ErrTicketContentRequired
	}
	category := normalizeTicketCategory(input.Category)
	if !isValidTicketCategory(category) {
		return nil, ErrTicketInvalidCategory
	}
	priority := normalizeTicketPriority(input.Priority)
	if !isValidTicketPriority(priority) {
		return nil, ErrTicketInvalidPriority
	}

	ticket := &Ticket{
		UserID:   positiveInt64Ptr(input.UserID),
		Contact:  contact,
		Title:    title,
		Category: category,
		Priority: priority,
		Status:   TicketStatusOpen,
	}
	if err := s.ticketRepo.Create(ctx, ticket, content); err != nil {
		return nil, fmt.Errorf("create ticket: %w", err)
	}
	return ticket, nil
}

func (s *TicketService) List(ctx context.Context, params pagination.PaginationParams, filters TicketListFilters) ([]Ticket, *pagination.PaginationResult, error) {
	filters.Status = strings.TrimSpace(filters.Status)
	filters.Category = strings.TrimSpace(filters.Category)
	filters.Search = strings.TrimSpace(filters.Search)
	if filters.Status != "" && !isValidTicketStatus(filters.Status) {
		return nil, nil, ErrTicketInvalidStatus
	}
	if filters.Category != "" && !isValidTicketCategory(filters.Category) {
		return nil, nil, ErrTicketInvalidCategory
	}
	return s.ticketRepo.List(ctx, params, filters)
}

func (s *TicketService) GetForUser(ctx context.Context, id int64, userID int64) (*Ticket, error) {
	ticket, err := s.ticketRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if ticket.UserID == nil || *ticket.UserID != userID {
		return nil, ErrTicketForbidden
	}
	return ticket, nil
}

func (s *TicketService) GetByID(ctx context.Context, id int64) (*Ticket, error) {
	return s.ticketRepo.GetByID(ctx, id)
}

func (s *TicketService) AddUserMessage(ctx context.Context, ticketID int64, userID int64, content string) (*TicketMessage, error) {
	ticket, err := s.GetForUser(ctx, ticketID, userID)
	if err != nil {
		return nil, err
	}
	if ticket.Status == TicketStatusClosed {
		return nil, ErrTicketClosed
	}
	return s.addMessage(ctx, ticketID, &AddTicketMessageInput{
		UserID:     &userID,
		AuthorType: TicketAuthorUser,
		Content:    content,
	})
}

func (s *TicketService) AddAdminMessage(ctx context.Context, ticketID int64, adminID int64, content string) (*TicketMessage, error) {
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if ticket.Status == TicketStatusClosed {
		return nil, ErrTicketClosed
	}
	return s.addMessage(ctx, ticketID, &AddTicketMessageInput{
		UserID:     &adminID,
		AuthorType: TicketAuthorAdmin,
		Content:    content,
	})
}

func (s *TicketService) UpdateStatus(ctx context.Context, ticketID int64, status string) (*Ticket, error) {
	status = strings.TrimSpace(status)
	if !isValidTicketStatus(status) {
		return nil, ErrTicketInvalidStatus
	}
	return s.ticketRepo.UpdateStatus(ctx, ticketID, status)
}

func (s *TicketService) addMessage(ctx context.Context, ticketID int64, input *AddTicketMessageInput) (*TicketMessage, error) {
	content := strings.TrimSpace(input.Content)
	if content == "" {
		return nil, ErrTicketContentRequired
	}
	msg := &TicketMessage{
		TicketID:   ticketID,
		UserID:     positiveInt64Ptr(input.UserID),
		AuthorType: input.AuthorType,
		Content:    content,
	}
	if err := s.ticketRepo.AddMessage(ctx, ticketID, msg); err != nil {
		return nil, fmt.Errorf("add ticket message: %w", err)
	}
	return msg, nil
}

func normalizeTicketCategory(category string) string {
	category = strings.TrimSpace(category)
	if category == "" {
		return TicketCategoryOther
	}
	return category
}

func normalizeTicketPriority(priority string) string {
	priority = strings.TrimSpace(priority)
	if priority == "" {
		return TicketPriorityNormal
	}
	return priority
}

func isValidTicketCategory(category string) bool {
	switch category {
	case TicketCategoryAccount, TicketCategoryBilling, TicketCategoryAPI, TicketCategoryModel, TicketCategoryOther:
		return true
	default:
		return false
	}
}

func isValidTicketPriority(priority string) bool {
	switch priority {
	case TicketPriorityLow, TicketPriorityNormal, TicketPriorityHigh, TicketPriorityUrgent:
		return true
	default:
		return false
	}
}

func isValidTicketStatus(status string) bool {
	switch status {
	case TicketStatusOpen, TicketStatusPending, TicketStatusAnswered, TicketStatusClosed:
		return true
	default:
		return false
	}
}

func positiveInt64Ptr(v *int64) *int64 {
	if v == nil || *v <= 0 {
		return nil
	}
	return v
}
