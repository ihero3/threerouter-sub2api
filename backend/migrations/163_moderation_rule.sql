-- AI 治理与合规模块：内容审核自定义规则表
-- rule_type: KEYWORD / REGEX / PATTERN / ML_MODEL / THIRD_PARTY
-- action: ALLOW / REVIEW / BLOCK
-- 并发索引见 163_moderation_rule_notx.sql

CREATE TABLE IF NOT EXISTS moderation_rules (
    id BIGSERIAL PRIMARY KEY,
    rule_id VARCHAR(64) NOT NULL UNIQUE,
    rule_name VARCHAR(100) NOT NULL,
    rule_type VARCHAR(30) NOT NULL,
    rule_pattern TEXT,
    threshold DOUBLE PRECISION NOT NULL DEFAULT 0.8,
    action VARCHAR(20) NOT NULL DEFAULT 'ALLOW',
    risk_category VARCHAR(50),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    priority INT NOT NULL DEFAULT 100,
    created_by VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
