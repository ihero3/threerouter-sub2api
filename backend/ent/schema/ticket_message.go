package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// TicketMessage holds the schema definition for ticket replies.
type TicketMessage struct {
	ent.Schema
}

func (TicketMessage) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "ticket_messages"},
	}
}

func (TicketMessage) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("ticket_id").
			Comment("工单ID"),
		field.Int64("user_id").
			Optional().
			Nillable().
			Comment("回复用户ID，游客回复为空"),
		field.String("author_type").
			MaxLen(20).
			Default("user").
			Comment("回复人类型: user, admin"),
		field.String("content").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			NotEmpty().
			Comment("回复内容"),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (TicketMessage) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("ticket", Ticket.Type).
			Ref("messages").
			Field("ticket_id").
			Unique().
			Required(),
		edge.From("user", User.Type).
			Ref("ticket_messages").
			Field("user_id").
			Unique(),
	}
}

func (TicketMessage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("ticket_id"),
		index.Fields("user_id"),
		index.Fields("created_at"),
	}
}
