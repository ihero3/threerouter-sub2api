package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUsageLogsReadWriteIndexOptimizationMigration(t *testing.T) {
	content, err := FS.ReadFile("256_optimize_usage_logs_read_write_indexes_notx.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	for _, indexName := range []string{
		"idx_usage_logs_user_created_id",
		"idx_usage_logs_api_key_created_id",
		"idx_usage_logs_account_created_id",
	} {
		require.Contains(t, sql, "CREATE INDEX CONCURRENTLY IF NOT EXISTS "+indexName)
	}
	require.Contains(t, sql, "ON usage_logs (user_id, created_at, id)")
	require.Contains(t, sql, "ON usage_logs (api_key_id, created_at, id)")
	require.Contains(t, sql, "ON usage_logs (account_id, created_at, id)")

	createAccountIndex := strings.Index(sql, "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_account_created_id")
	firstDrop := strings.Index(sql, "DROP INDEX CONCURRENTLY IF EXISTS idx_usage_logs_user_id")
	require.NotEqual(t, -1, createAccountIndex)
	require.NotEqual(t, -1, firstDrop)
	require.Less(t, createAccountIndex, firstDrop, "new covering indexes must exist before old indexes are dropped")
}
