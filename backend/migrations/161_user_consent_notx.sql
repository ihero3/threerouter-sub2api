-- AI 治理与合规模块：user_consents 唯一索引（非事务执行）
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_user_consents_user_type ON user_consents(user_id, consent_type);
