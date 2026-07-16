-- AI 治理与合规模块：扩展 usage_logs 增加合规维度字段
-- 说明：usage_logs 为分区表，以下均为可选字段（NULL 允许），对高吞吐批量写入影响极低。
-- 并发索引见 158_compliance_fields_notx.sql。

ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS model_tags VARCHAR(500);
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS risk_tags VARCHAR(500);
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS eu_ai_act_role VARCHAR(50);
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS eu_ai_act_risk_tier VARCHAR(20);
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS compliance_checkpoint_hash VARCHAR(64);
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS moderation_action VARCHAR(20);
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS user_jurisdiction VARCHAR(50);
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS aggregate_only BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS retention_expires_at TIMESTAMPTZ;
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS model_provider VARCHAR(50);
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS model_version VARCHAR(50);
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS prompt_hash VARCHAR(64);
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS response_hash VARCHAR(64);
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS policy_decision VARCHAR(20);
