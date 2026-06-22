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

// Ticket holds the schema definition for support tickets.
type Ticket struct {
	ent.Schema
}

func (Ticket) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "tickets"},
	}
}

func (Ticket) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id").
			Optional().
			Nillable().
			Comment("关联用户ID，游客工单为空"),
		field.String("contact").
			MaxLen(255).
			NotEmpty().
			Comment("联系方式"),
		field.String("title").
			MaxLen(200).
			NotEmpty().
			Comment("工单标题"),
		field.String("category").
			MaxLen(32).
			Default("other").
			Comment("分类: account, billing, api, model, other"),
		field.String("priority").
			MaxLen(20).
			Default("normal").
			Comment("优先级: low, normal, high, urgent"),
		field.String("status").
			MaxLen(20).
			Default("open").
			Comment("状态: open, pending, answered, closed"),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (Ticket) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("tickets").
			Field("user_id").
			Unique(),
		edge.To("messages", TicketMessage.Type),
	}
}

func (Ticket) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("status"),
		index.Fields("category"),
		index.Fields("created_at"),
	}
}
