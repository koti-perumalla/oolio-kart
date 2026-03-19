package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"time"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func InitPostgres() error {

	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return fmt.Errorf("DATABASE_URL not set")
	}
	// configure retries
	maxAttempts := 10
	if v := os.Getenv("DB_CONNECT_RETRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxAttempts = n
		}
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		db, err := sql.Open("postgres", url)
		if err != nil {
			lastErr = err
			if attempt == maxAttempts {
				break
			}
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}

		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(25)
		db.SetConnMaxIdleTime(5 * time.Minute)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = db.PingContext(ctx)
		cancel()
		if err == nil {
			DB = db
			return nil
		}

		_ = db.Close()
		lastErr = err
		if attempt == maxAttempts {
			break
		}

		// linear retries (1s,2s,3s...)
		time.Sleep(time.Duration(attempt) * time.Second)
	}

	return fmt.Errorf("postgres connect failed after %d attempts: %w", maxAttempts, lastErr)
}
