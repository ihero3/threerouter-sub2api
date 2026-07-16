-- AI 治理与合规模块：合规审计日志表（仿 payment_audit_logs 追加式设计）
-- 记录合规决策层面的事件（风险评估、EU AI Act 评估报告生成、数据删除请求、合规审查等）。
-- 与 content_moderation_logs（内容审核执行层，请求级、短保留）边界分离。

CREATE TABLE IF NOT EXISTS compliance_audit_logs (
    id BIGSERIAL PRIMARY KEY,
    compliance_type VARCHAR(50) NOT NULL,
    subject_type VARCHAR(20) NOT NULL,
    subject_id BIGINT,
    details TEXT NOT NULL DEFAULT '',
    operator VARCHAR(100) NOT NULL DEFAULT 'system',
    evidence_hash VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
