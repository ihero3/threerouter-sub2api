-- Optimize usage_logs read/write indexes.
--
-- User, API key, and account lists read newest rows first. Adding id as the
-- final sort key lets PostgreSQL scan the index in order without an extra sort.
-- The new composite indexes cover the old single-column prefixes. Dropping the
-- redundant indexes reduces write amplification on every usage log insert.
-- Create new indexes before removing old ones; CONCURRENTLY avoids blocking writes.

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_user_created_id ON usage_logs (user_id, created_at, id);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_api_key_created_id ON usage_logs (api_key_id, created_at, id);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_account_created_id ON usage_logs (account_id, created_at, id);

DROP INDEX CONCURRENTLY IF EXISTS idx_usage_logs_user_id;
DROP INDEX CONCURRENTLY IF EXISTS idx_usage_logs_user_created;
DROP INDEX CONCURRENTLY IF EXISTS idx_usage_logs_api_key_id;
DROP INDEX CONCURRENTLY IF EXISTS idx_usage_logs_api_key_created_at;
DROP INDEX CONCURRENTLY IF EXISTS idx_usage_logs_account_id;
DROP INDEX CONCURRENTLY IF EXISTS idx_usage_logs_account_created_at;
DROP INDEX CONCURRENTLY IF EXISTS idx_usage_logs_group_id;
DROP INDEX CONCURRENTLY IF EXISTS idx_usage_logs_group_created_at_not_null;
DROP INDEX CONCURRENTLY IF EXISTS idx_usage_logs_model;
DROP INDEX CONCURRENTLY IF EXISTS idx_usage_logs_model_created_at;
DROP INDEX CONCURRENTLY IF EXISTS idx_usage_logs_requested_model;
DROP INDEX CONCURRENTLY IF EXISTS idx_usage_logs_request_id;
