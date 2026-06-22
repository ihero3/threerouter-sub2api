-- 创建工单表
CREATE TABLE IF NOT EXISTS tickets (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT DEFAULT NULL REFERENCES users(id) ON DELETE SET NULL,
    contact VARCHAR(255) NOT NULL,
    title VARCHAR(200) NOT NULL,
    category VARCHAR(32) NOT NULL DEFAULT 'other',
    priority VARCHAR(20) NOT NULL DEFAULT 'normal',
    status VARCHAR(20) NOT NULL DEFAULT 'open',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 创建工单回复表
CREATE TABLE IF NOT EXISTS ticket_messages (
    id BIGSERIAL PRIMARY KEY,
    ticket_id BIGINT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    user_id BIGINT DEFAULT NULL REFERENCES users(id) ON DELETE SET NULL,
    author_type VARCHAR(20) NOT NULL DEFAULT 'user',
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tickets_user_id ON tickets(user_id);
CREATE INDEX IF NOT EXISTS idx_tickets_status ON tickets(status);
CREATE INDEX IF NOT EXISTS idx_tickets_category ON tickets(category);
CREATE INDEX IF NOT EXISTS idx_tickets_created_at ON tickets(created_at);
CREATE INDEX IF NOT EXISTS idx_ticket_messages_ticket_id ON ticket_messages(ticket_id);
CREATE INDEX IF NOT EXISTS idx_ticket_messages_user_id ON ticket_messages(user_id);
CREATE INDEX IF NOT EXISTS idx_ticket_messages_created_at ON ticket_messages(created_at);

COMMENT ON TABLE tickets IS '支持工单';
COMMENT ON COLUMN tickets.category IS '分类: account, billing, api, model, other';
COMMENT ON COLUMN tickets.priority IS '优先级: low, normal, high, urgent';
COMMENT ON COLUMN tickets.status IS '状态: open, pending, answered, closed';
COMMENT ON TABLE ticket_messages IS '工单回复消息';
COMMENT ON COLUMN ticket_messages.author_type IS '回复人类型: user, admin';
