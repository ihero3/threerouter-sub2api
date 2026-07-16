-- 跨法域合规映射用户偏好表
-- 用户选择的法域、行业、服务类型映射配置，以及自动应用的合规规则

CREATE TABLE IF NOT EXISTS user_jurisdiction_mappings (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    company_region VARCHAR(50) NOT NULL,
    industry VARCHAR(100),
    service_type VARCHAR(50),
    risk_level VARCHAR(20) NOT NULL,
    applicable_regulations TEXT[],
    required_measures TEXT[],
    recommended_actions TEXT[],
    applied_rules JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);