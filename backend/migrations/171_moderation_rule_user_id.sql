ALTER TABLE moderation_rules ADD COLUMN IF NOT EXISTS user_id BIGINT;
CREATE INDEX IF NOT EXISTS idx_moderation_rules_user_id ON moderation_rules(user_id);