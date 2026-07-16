-- AI 治理与合规模块：user_compliance_profiles 并发索引（非事务执行）
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_user_compliance_profiles_user_id ON user_compliance_profiles(user_id);
