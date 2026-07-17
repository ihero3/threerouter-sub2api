package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

func main() {
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		dsn = "host=localhost port=5432 user=postgres password=postgres dbname=sub2api sslmode=disable"
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	rows, err := db.QueryContext(ctx, `SELECT id, channel_id, models, input_price, output_price, cache_write_price, cache_read_price FROM channel_model_pricing`)
	if err != nil {
		log.Fatalf("Failed to query channel_model_pricing: %v", err)
	}
	defer rows.Close()

	found := false
	for rows.Next() {
		var id int64
		var channelID int64
		var models string
		var inputPrice, outputPrice, cacheWritePrice, cacheReadPrice float64
		if err := rows.Scan(&id, &channelID, &models, &inputPrice, &outputPrice, &cacheWritePrice, &cacheReadPrice); err != nil {
			log.Printf("Failed to scan row: %v", err)
			continue
		}
		if contains(models, "deepseek") {
			fmt.Printf("ID: %d, ChannelID: %d\n", id, channelID)
			fmt.Printf("Models: %s\n", models)
			fmt.Printf("InputPrice: %.2e, OutputPrice: %.2e, CacheWrite: %.2e, CacheRead: %.2e\n", inputPrice, outputPrice, cacheWritePrice, cacheReadPrice)
			fmt.Println("---")
			found = true
		}
	}

	if !found {
		fmt.Println("No channel found with deepseek model pricing")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
