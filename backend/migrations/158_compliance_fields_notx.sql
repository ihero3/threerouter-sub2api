-- AI 治理与合规模块：usage_logs 合规字段并发索引（非事务执行）
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_risk_tags ON usage_logs(risk_tags);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_model_tags ON usage_logs(model_tags);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_user_jurisdiction ON usage_logs(user_jurisdiction);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_aggregate_only ON usage_logs(aggregate_only);
