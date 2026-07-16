-- AI 治理与合规模块：合规证据包表（数字服务出口合规证据包 / EU AI Act 评估 / GDPR / ZDR 认证）
-- issuer_type 默认 SELF_ASSERTION（自证声明），非官方凭证。
-- 并发索引见 162_compliance_credential_notx.sql

CREATE TABLE IF NOT EXISTS compliance_credentials (
    id BIGSERIAL PRIMARY KEY,
    credential_id VARCHAR(64) NOT NULL UNIQUE,
    credential_type VARCHAR(50) NOT NULL,
    issuer VARCHAR(100) NOT NULL,
    issuer_type VARCHAR(50) NOT NULL DEFAULT 'SELF_ASSERTION',
    scope VARCHAR(500),
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    valid_from TIMESTAMPTZ NOT NULL,
    valid_until TIMESTAMPTZ NOT NULL,
    evidence_hashes VARCHAR(1000),
    digital_signature VARCHAR(512),
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
