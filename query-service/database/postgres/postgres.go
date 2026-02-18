package postgres

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var Pool *pgxpool.Pool
var once sync.Once

func GetPostgresClient() *pgxpool.Pool {
	once.Do(func() {
		var err error
		Pool, err = connectPostgres()
		if err != nil {
			log.Fatalf("Failed to connect to Postgres: %v", err)
		}
	})
	return Pool
}

func connectPostgres() (*pgxpool.Pool, error) {
	// Example: postgres://user:pass@localhost:5432/dbname

	//load .env file
	pgUrl := os.Getenv("POSTGRES_URL")
	if pgUrl == "" {
		return nil, fmt.Errorf("POSTGRES_URL environment variable is not set")
	}

	config, err := pgxpool.ParseConfig(pgUrl)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database config: %v", err)
	}

	// Performance Tuning: Set max connections
	// This prevents the Go worker from overwhelming Postgres if traffic spikes.
	config.MaxConns = 50 // Max connections per pod
	config.MinConns = 10 // Keep warm connections
	config.MaxConnIdleTime = 30 * time.Minute
	config.MaxConnLifetime = 1 * time.Hour

	// 3. Establish the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %v", err)
	}

	// 4. Verify the connection works (Ping)
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("unable to ping database: %v", err)
	}

	log.Println("Connected to Postgres successfully")
	return pool, nil
}
