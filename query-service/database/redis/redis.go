package redis

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

var Client *redis.Client
var once sync.Once

func GetRedisClient() *redis.Client {
	if Client == nil {
		return connectRedis()
	}
	return Client
}

func connectRedis() *redis.Client {
	/*
			Using sync.Once ensures you only initialize the client once (singleton pattern).
		•	You can now reuse Client anywhere by calling redis.connectRedis().
	*/
	once.Do(func() {

		//load .env file
		_ = godotenv.Load()

		// Get environment variable
		reddisAddr := os.Getenv("REDIS_ADDRESS")
		reddisPass := os.Getenv("REDIS_PASSWORD")
		//reddisDB := os.Getenv("REDIS_DB")

		// Fallback to HOST:PORT if ADDRESS is not set
		if reddisAddr == "" {
			host := os.Getenv("REDIS_HOST")
			port := os.Getenv("REDIS_PORT")
			if host != "" && port != "" {
				reddisAddr = fmt.Sprintf("%s:%s", host, port)
			} else {
				// Default to localhost if nothing is set (for local dev without .env)
				reddisAddr = "localhost:6379"
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		//connect to the client
		Client = redis.NewClient(&redis.Options{
			Addr:     reddisAddr,
			Password: reddisPass,
			DB:       0,
		})

		//ping the client
		ping, err := Client.Ping(ctx).Result()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Connected succesfully to redis , PING:%s ", ping)

	})

	return Client
}

// warm cach : SYNC the cache available ticket count to match Inventory total_seats - sold_seats  (to be decremented when purshased)
func WarmCach(ctx context.Context, db *pgxpool.Pool, rdb *redis.Client) error {
	// Fetch current inventory from Postgres
	rows, err := db.Query(ctx, `
        SELECT ti.match_id, ti.category, ti.total_seats, ti.sold_seats, m.match_date
        FROM ticket_inventory ti
        JOIN matches m ON m.match_id = ti.match_id
        WHERE m.match_date > NOW()  -- Only sync future events
    `)

	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var matchID int
		var category string
		var total, sold int
		var matchDate time.Time
		if err := rows.Scan(&matchID, &category, &total, &sold, &matchDate); err != nil {
			return err
		}

		// Calculate remaining seats
		remaining := total - sold
		key := fmt.Sprintf("ticket_inventory:%d:%s", matchID, category)

		// 3. SET the value in Redis
		ttl := calculateTTL(matchDate)
		err = rdb.Set(ctx, key, remaining, ttl).Err()
		if err != nil {
			log.Printf("Failed to warm cache for %s: %v", key, err)
		}
	}
	log.Println("Redis Cache Warmed Successfully")
	return nil
}

func calculateTTL(matchDate time.Time) time.Duration {
	now := time.Now()
	timeUntilMatch := matchDate.Sub(now)
	// Event Soon - keep data fresh
	if timeUntilMatch < 24*time.Hour {
		return 5 * time.Minute
	}
	//event is 1-7 days away
	if timeUntilMatch < 7*24*time.Hour {
		return 1 * time.Hour
	}
	//event is far in the future
	return 24 * time.Hour
}
