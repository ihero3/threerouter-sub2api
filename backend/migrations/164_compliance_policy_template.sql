-- AI 治理与合规模块：行业合规策略模板表
-- industry: ecommerce / finance / healthcare / education 等
-- 并发索引见 164_compliance_policy_template_notx.sql

CREATE TABLE IF NOT EXISTS compliance_policy_templates (
    id BIGSERIAL PRIMARY KEY,
    template_code VARCHAR(30) NOT NULL UNIQUE,
    industry VARCHAR(50) NOT NULL,
    description TEXT,
    rules JSONB,
    risk_tags VARCHAR(500),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
