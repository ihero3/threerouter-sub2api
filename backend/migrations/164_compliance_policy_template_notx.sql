-- AI 治理与合规模块：compliance_policy_templates 并发索引（非事务执行）
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_compliance_policy_templates_industry ON compliance_policy_templates(industry);
