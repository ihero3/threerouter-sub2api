-- Widen request_preview on usage_logs and ops_error_logs from VARCHAR(64) to
-- VARCHAR(255) so the 200-rune preview (service.MaxRequestPreviewRunes) fits
-- without triggering a "value too long for type character varying(64)" error.
-- Widening a varchar column is a metadata-only change in PostgreSQL.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';
ALTER TABLE usage_logs ALTER COLUMN request_preview TYPE VARCHAR(255);
ALTER TABLE ops_error_logs ALTER COLUMN request_preview TYPE VARCHAR(255);
