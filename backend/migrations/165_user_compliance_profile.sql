-- AI 治理与合规模块：用户级合规档案表
-- 每个 User Account（企业客户）拥有独立的合规配置。
-- 并发索引见 165_user_compliance_profile_notx.sql

CREATE TABLE IF NOT EXISTS user_compliance_profiles (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    active_template_code VARCHAR(30),
    zdr_mode VARCHAR(20) NOT NULL DEFAULT 'aggregate_only',
    detail_retention_days INT NOT NULL DEFAULT 7,
    compliance_frameworks JSONB NOT NULL DEFAULT '[]',
    moderation_policy JSONB NOT NULL DEFAULT '{"enabled_rule_ids":[]}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
