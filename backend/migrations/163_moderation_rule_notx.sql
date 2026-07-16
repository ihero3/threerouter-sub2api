-- AI 治理与合规模块：moderation_rules 并发索引（非事务执行）
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_moderation_rules_rule_type ON moderation_rules(rule_type);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_moderation_rules_enabled ON moderation_rules(enabled);
