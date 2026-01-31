package postgres

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

var Pool *pgxpool.Pool

func GetPostgresClient() *pgxpool.Pool {
	if Pool == nil {
		connectPostgres()
	}
	return Pool
}

func connectPostgres() (*pgxpool.Pool, error) {
	// Example: postgres://user:pass@localhost:5432/dbname

	//load .env file
	err := godotenv.Load("../.env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database config: %v", err)
	}

	// Performance Tuning: Set max connections
	// This prevents the Go worker from overwhelming Postgres if traffic spikes.
	config.MaxConns = 10
	config.MinConns = 1

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
