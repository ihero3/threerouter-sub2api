package repository

import (
	"context"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/ticket"
	"github.com/Wei-Shaw/sub2api/ent/ticketmessage"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"

	entsql "entgo.io/ent/dialect/sql"
)

type ticketRepository struct {
	client *dbent.Client
}

func NewTicketRepository(client *dbent.Client) service.TicketRepository {
	return &ticketRepository{client: client}
}

func (r *ticketRepository) Create(ctx context.Context, t *service.Ticket, firstMessage string) error {
	client := clientFromContext(ctx, r.client)
	created, err := client.Ticket.Create().
		SetNillableUserID(t.UserID).
		SetContact(t.Contact).
		SetTitle(t.Title).
		SetCategory(t.Category).
		SetPriority(t.Priority).
		SetStatus(t.Status).
		Save(ctx)
	if err != nil {
		return err
	}

	msgBuilder := client.TicketMessage.Create().
		SetTicketID(created.ID).
		SetAuthorType(service.TicketAuthorUser).
		SetContent(firstMessage)
	if t.UserID != nil {
		msgBuilder.SetUserID(*t.UserID)
	}
	msg, err := msgBuilder.Save(ctx)
	if err != nil {
		return err
	}

	applyTicketEntityToService(t, created)
	t.Messages = []service.TicketMessage{*ticketMessageEntityToService(msg)}
	return nil
}

func (r *ticketRepository) GetByID(ctx context.Context, id int64) (*service.Ticket, error) {
	m, err := r.client.Ticket.Query().
		Where(ticket.IDEQ(id)).
		WithMessages(func(q *dbent.TicketMessageQuery) {
			q.Order(dbent.Asc(ticketmessage.FieldCreatedAt), dbent.Asc(ticketmessage.FieldID))
		}).
		Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrTicketNotFound, nil)
	}
	return ticketEntityToService(m), nil
}

func (r *ticketRepository) List(ctx context.Context, params pagination.PaginationParams, filters service.TicketListFilters) ([]service.Ticket, *pagination.PaginationResult, error) {
	q := r.client.Ticket.Query()
	if filters.UserID != nil {
		q = q.Where(ticket.UserIDEQ(*filters.UserID))
	}
	if filters.Status != "" {
		q = q.Where(ticket.StatusEQ(filters.Status))
	}
	if filters.Category != "" {
		q = q.Where(ticket.CategoryEQ(filters.Category))
	}
	if filters.Search != "" {
		q = q.Where(ticket.Or(
			ticket.TitleContainsFold(filters.Search),
			ticket.ContactContainsFold(filters.Search),
		))
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}

	itemsQuery := q.Offset(params.Offset()).Limit(params.Limit())
	for _, order := range ticketListOrders(params) {
		itemsQuery = itemsQuery.Order(order)
	}
	items, err := itemsQuery.All(ctx)
	if err != nil {
		return nil, nil, err
	}
	return ticketEntitiesToService(items), paginationResultFromTotal(int64(total), params), nil
}

func (r *ticketRepository) UpdateStatus(ctx context.Context, id int64, status string) (*service.Ticket, error) {
	client := clientFromContext(ctx, r.client)
	m, err := client.Ticket.UpdateOneID(id).SetStatus(status).Save(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrTicketNotFound, nil)
	}
	return ticketEntityToService(m), nil
}

func (r *ticketRepository) AddMessage(ctx context.Context, ticketID int64, message *service.TicketMessage) error {
	client := clientFromContext(ctx, r.client)
	builder := client.TicketMessage.Create().
		SetTicketID(ticketID).
		SetAuthorType(message.AuthorType).
		SetContent(message.Content)
	if message.UserID != nil {
		builder.SetUserID(*message.UserID)
	}
	created, err := builder.Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrTicketNotFound, nil)
	}

	status := service.TicketStatusPending
	if message.AuthorType == service.TicketAuthorAdmin {
		status = service.TicketStatusAnswered
	}
	if _, err := client.Ticket.UpdateOneID(ticketID).SetStatus(status).Save(ctx); err != nil {
		return translatePersistenceError(err, service.ErrTicketNotFound, nil)
	}

	applyTicketMessageEntityToService(message, created)
	return nil
}

func ticketListOrders(params pagination.PaginationParams) []func(*entsql.Selector) {
	field := ticket.FieldCreatedAt
	sortBy := strings.ToLower(strings.TrimSpace(params.SortBy))
	sortOrder := params.NormalizedSortOrder(pagination.SortOrderDesc)
	switch sortBy {
	case "id":
		field = ticket.FieldID
	case "title":
		field = ticket.FieldTitle
	case "status":
		field = ticket.FieldStatus
	case "priority":
		field = ticket.FieldPriority
	case "updated_at":
		field = ticket.FieldUpdatedAt
	}
	if sortOrder == pagination.SortOrderAsc {
		return []func(*entsql.Selector){dbent.Asc(field), dbent.Asc(ticket.FieldID)}
	}
	return []func(*entsql.Selector){dbent.Desc(field), dbent.Desc(ticket.FieldID)}
}

func applyTicketEntityToService(dst *service.Ticket, src *dbent.Ticket) {
	if dst == nil || src == nil {
		return
	}
	dst.ID = src.ID
	dst.UserID = src.UserID
	dst.Contact = src.Contact
	dst.Title = src.Title
	dst.Category = src.Category
	dst.Priority = src.Priority
	dst.Status = src.Status
	dst.CreatedAt = src.CreatedAt
	dst.UpdatedAt = src.UpdatedAt
}

func ticketEntityToService(m *dbent.Ticket) *service.Ticket {
	if m == nil {
		return nil
	}
	out := &service.Ticket{}
	applyTicketEntityToService(out, m)
	out.Messages = make([]service.TicketMessage, 0, len(m.Edges.Messages))
	for _, msg := range m.Edges.Messages {
		if item := ticketMessageEntityToService(msg); item != nil {
			out.Messages = append(out.Messages, *item)
		}
	}
	return out
}

func ticketEntitiesToService(models []*dbent.Ticket) []service.Ticket {
	out := make([]service.Ticket, 0, len(models))
	for _, m := range models {
		if item := ticketEntityToService(m); item != nil {
			out = append(out, *item)
		}
	}
	return out
}

func applyTicketMessageEntityToService(dst *service.TicketMessage, src *dbent.TicketMessage) {
	if dst == nil || src == nil {
		return
	}
	dst.ID = src.ID
	dst.TicketID = src.TicketID
	dst.UserID = src.UserID
	dst.AuthorType = src.AuthorType
	dst.Content = src.Content
	dst.CreatedAt = src.CreatedAt
}

func ticketMessageEntityToService(m *dbent.TicketMessage) *service.TicketMessage {
	if m == nil {
		return nil
	}
	out := &service.TicketMessage{}
	applyTicketMessageEntityToService(out, m)
	return out
}
