package dto

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type Ticket struct {
	ID        int64           `json:"id"`
	UserID    *int64          `json:"user_id,omitempty"`
	Contact   string          `json:"contact"`
	Title     string          `json:"title"`
	Category  string          `json:"category"`
	Priority  string          `json:"priority"`
	Status    string          `json:"status"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	Messages  []TicketMessage `json:"messages,omitempty"`
}

type TicketMessage struct {
	ID         int64     `json:"id"`
	TicketID   int64     `json:"ticket_id"`
	UserID     *int64    `json:"user_id,omitempty"`
	AuthorType string    `json:"author_type"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

func TicketFromService(t *service.Ticket) *Ticket {
	if t == nil {
		return nil
	}
	out := &Ticket{
		ID:        t.ID,
		UserID:    t.UserID,
		Contact:   t.Contact,
		Title:     t.Title,
		Category:  t.Category,
		Priority:  t.Priority,
		Status:    t.Status,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
		Messages:  make([]TicketMessage, 0, len(t.Messages)),
	}
	for i := range t.Messages {
		out.Messages = append(out.Messages, *TicketMessageFromService(&t.Messages[i]))
	}
	return out
}

func TicketMessageFromService(m *service.TicketMessage) *TicketMessage {
	if m == nil {
		return nil
	}
	return &TicketMessage{
		ID:         m.ID,
		TicketID:   m.TicketID,
		UserID:     m.UserID,
		AuthorType: m.AuthorType,
		Content:    m.Content,
		CreatedAt:  m.CreatedAt,
	}
}
