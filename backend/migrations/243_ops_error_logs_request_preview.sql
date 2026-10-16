-- Add request_preview to ops_error_logs: a short, rune-limited head of the
-- user's input captured at error time, mirroring usage_logs.request_preview.
-- Only the first 50 runes of the latest user message are kept — never the
-- full prompt — and it is only populated when the risk-control center is
-- enabled (risk_control_enabled = true), so this stays safe under the
-- ZERO_DATA_RETENTION compliance credential.
-- Nullable (no default) so PostgreSQL can add it as a metadata-only operation.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

ALTER TABLE ops_error_logs
    ADD COLUMN IF NOT EXISTS request_preview VARCHAR(64);
