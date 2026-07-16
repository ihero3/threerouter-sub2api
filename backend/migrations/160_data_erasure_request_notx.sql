-- AI 治理与合规模块：data_erasure_requests 并发索引（非事务执行）
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_data_erasure_requests_user_id ON data_erasure_requests(user_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_data_erasure_requests_status ON data_erasure_requests(status);
