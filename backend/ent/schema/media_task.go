package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
)

// MediaTask holds the schema definition for the MediaTask entity.
// 媒体生成异步任务表：统一承载图片 / 视频 / 音频生成任务。
// 与 Channel/Account 通过 account_id 弱关联（不做 ent edge，降低耦合）。
// 同一模型可配置多个上游账号，调度时自动轮转。
type MediaTask struct {
	ent.Schema
}

func (MediaTask) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "media_tasks"},
	}
}

func (MediaTask) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (MediaTask) Fields() []ent.Field {
	return []ent.Field{
		field.String("local_id").
			MaxLen(128).
			Unique().
			NotEmpty().
			Comment("对用户暴露的唯一任务 ID，如 med_xxx / img_xxx / vid_xxx"),
		field.String("media_kind").
			MaxLen(20).
			Default("video").
			Comment("image / video / audio"),
		field.Int64("user_id").
			NonNegative(),
		field.Int64("api_key_id").
			Optional().
			NonNegative(),
		field.String("public_model").
			MaxLen(100).
			NotEmpty().
			Comment("用户请求的统一模型名，如 seedance-2.5"),
		field.String("upstream_model").
			MaxLen(100).
			NotEmpty().
			Comment("model_mapping 翻译后上游实际模型名"),
		field.Int64("account_id").
			NonNegative().
			Comment("实际使用的上游 channel (Account) ID"),
		field.String("upstream_task_id").
			MaxLen(255).
			Optional().
			Comment("上游返回的任务 ID"),
		field.String("status").
			MaxLen(20).
			Default("processing").
			Comment("processing / succeeded / failed / cancelled"),
		field.String("resolution").
			MaxLen(20).
			Optional(),
		field.Int("duration_sec").
			Optional().
			NonNegative(),
		field.Text("media_url").
			Optional().
			Comment("产物 URL（视频 / 图片 / 音频）"),
		field.JSON("media_urls", []string{}).
			Optional().
			Comment("多张产物 URL（图片 n>1 时全量落库，轮询接口据此返回 urls）"),
		field.Text("thumbnail_url").
			Optional(),
		field.JSON("request_body", map[string]any{}).
			Optional().
			Comment("原始请求快照，便于排障"),
		field.Text("error_message").
			Optional(),
		field.Float("cost_usd").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Float("reserved_cost").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Comment("创建时预扣的估算费用，完成时用于多退少补"),
		field.Time("finished_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (MediaTask) Edges() []ent.Edge {
	return nil
}

func (MediaTask) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("media_kind", "status", "created_at"),
		index.Fields("user_id", "created_at"),
		index.Fields("account_id"),
	}
}
