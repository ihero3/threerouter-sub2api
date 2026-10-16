-- Add request_preview to usage_logs: a short, rune-limited head of the
-- user's input extracted at write time. Only the first 50 runes of the
-- latest user message are kept — never the full prompt — so this column
-- is safe under the ZERO_DATA_RETENTION compliance credential.
-- Nullable (no default) on a large-ish production table so PostgreSQL 11+
-- can add it as a metadata-only operation.
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS request_preview VARCHAR(64);
