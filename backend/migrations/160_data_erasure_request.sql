-- AI 治理与合规模块：数据删除请求表（GDPR Art 17 删除权）
-- request_type: FULL_ERASURE / ANONYMIZE / RESTRICT
-- status: pending / approved / rejected / processing / completed

CREATE TABLE IF NOT EXISTS data_erasure_requests (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    request_type VARCHAR(30) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    scope_details TEXT NOT NULL DEFAULT '',
    rejection_reason VARCHAR(500),
    operator VARCHAR(100),
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ
);
