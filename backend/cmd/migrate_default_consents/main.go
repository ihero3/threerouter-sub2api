package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

// Migration script to create default consent records for existing users
// Run this before deploying the new version that enforces consent checks
// Usage: go run cmd/migrate_default_consents/main.go <database_connection_string>
// Example: go run cmd/migrate_default_consents/main.go "host=localhost port=5432 dbname=sub2api user=sub2api password=sub2api sslmode=disable"

var defaultConsentTypes = []string{
	"terms_of_service",
	"gdpr_data_processing",
	"detailed_logging",
	"cross_border_transfer",
	"marketing",
	"model_training",
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run cmd/migrate_default_consents/main.go <database_connection_string>")
		fmt.Println("Example: go run cmd/migrate_default_consents/main.go \"host=localhost port=5432 dbname=sub2api user=sub2api password=sub2api sslmode=disable\"")
		os.Exit(1)
	}

	connStr := os.Args[1]
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// Check if user_consents table exists
	var tableExists bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 
			FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'user_consents'
		)
	`).Scan(&tableExists)
	if err != nil {
		log.Fatalf("Failed to check if user_consents table exists: %v", err)
	}
	if !tableExists {
		log.Fatal("user_consents table does not exist. Please run previous migrations first.")
	}

	// Get total user count
	var totalUsers int
	err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&totalUsers)
	if err != nil {
		log.Fatalf("Failed to count users: %v", err)
	}
	fmt.Printf("Total users in database: %d\n", totalUsers)

	// Get current consent record count
	var currentConsents int
	err = db.QueryRow("SELECT COUNT(*) FROM user_consents").Scan(&currentConsents)
	if err != nil {
		log.Fatalf("Failed to count consents: %v", err)
	}
	fmt.Printf("Current consent records: %d\n", currentConsents)

	// Create default consent records for existing users
	now := time.Now()
	source := "migration_default"
	totalInserted := 0

	for _, consentType := range defaultConsentTypes {
		query := `
INSERT INTO user_consents (user_id, consent_type, granted, granted_at, source, created_at, updated_at)
SELECT id, $1, $2, $3, $4, $5, $6
FROM users
WHERE NOT EXISTS (
    SELECT 1 
    FROM user_consents uc 
    WHERE uc.user_id = users.id 
    AND uc.consent_type = $1
)
`
		result, err := db.Exec(query, consentType, true, now, source, now, now)
		if err != nil {
			log.Printf("Failed to migrate consent %s: %v", consentType, err)
			continue
		}
		rowsAffected, _ := result.RowsAffected()
		totalInserted += int(rowsAffected)
		fmt.Printf("Migrated consent '%s': %d new records created\n", consentType, rowsAffected)
	}

	// Verify migration
	var newConsents int
	err = db.QueryRow("SELECT COUNT(*) FROM user_consents").Scan(&newConsents)
	if err != nil {
		log.Printf("Failed to count consents after migration: %v", err)
	} else {
		fmt.Printf("\nMigration Summary:\n")
		fmt.Printf("- Total users: %d\n", totalUsers)
		fmt.Printf("- Consent records before: %d\n", currentConsents)
		fmt.Printf("- Consent records after: %d\n", newConsents)
		fmt.Printf("- New records created: %d\n", newConsents-currentConsents)
	}

	fmt.Println("\nMigration completed successfully!")
	fmt.Println("Users can now use the API with default consent records.")
}
