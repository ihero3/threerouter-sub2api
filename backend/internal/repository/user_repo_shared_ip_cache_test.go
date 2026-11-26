package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

// TestGetSharedUsageIPsCache verifies the short-TTL cache in front of the
// expensive 10-day full-table aggregation: repeated reads must not hit the DB,
// TTL expiry must refresh, and callers must not be able to mutate the cache.
func TestGetSharedUsageIPsCache(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := newUserRepositoryWithSQL(nil, db)
	ctx := context.Background()

	// First call hits the DB.
	mock.ExpectQuery("GROUP BY ip_address").
		WillReturnRows(sqlmock.NewRows([]string{"ip_address"}).AddRow("1.2.3.4").AddRow("5.6.7.8"))
	first, err := repo.GetSharedUsageIPs(ctx)
	require.NoError(t, err)
	require.Equal(t, map[string]bool{"1.2.3.4": true, "5.6.7.8": true}, first)

	// Caller mutation must not corrupt the cached set.
	first["1.2.3.4"] = false
	delete(first, "5.6.7.8")

	// Second call within TTL is served from cache; no DB query expected.
	second, err := repo.GetSharedUsageIPs(ctx)
	require.NoError(t, err)
	require.Equal(t, map[string]bool{"1.2.3.4": true, "5.6.7.8": true}, second)

	require.NoError(t, mock.ExpectationsWereMet())

	// Simulate TTL expiry and confirm a fresh DB query repopulates the cache.
	repo.sharedIPsMu.Lock()
	repo.sharedIPsCachedAt = time.Now().Add(-time.Hour)
	repo.sharedIPsMu.Unlock()
	mock.ExpectQuery("GROUP BY ip_address").
		WillReturnRows(sqlmock.NewRows([]string{"ip_address"}).AddRow("9.9.9.9"))
	third, err := repo.GetSharedUsageIPs(ctx)
	require.NoError(t, err)
	require.Equal(t, map[string]bool{"9.9.9.9": true}, third)
	require.NoError(t, mock.ExpectationsWereMet())
}
