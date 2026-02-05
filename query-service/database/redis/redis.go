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

// warm cach : Initialize the cache available ticket count to Inventory total_seats - sold_seats  (to be decremented when purshased)
func WarmCach(ctx context.Context, db *pgxpool.Pool, rdb *redis.Client) error {
	// Fetch current inventory from Postgres
	rows, err := db.Query(ctx, "SELECT match_id, category, total_seats, sold_seats FROM ticket_inventory")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var matchID int
		var category string
		var total, sold int
		if err := rows.Scan(&matchID, &category, &total, &sold); err != nil {
			return err
		}

		// Calculate remaining seats
		remaining := total - sold
		key := fmt.Sprintf("match:%d:category:%s", matchID, category)

		// 3. SET the value in Redis
		err = rdb.Set(ctx, key, remaining, 0).Err()
		if err != nil {
			log.Printf("Failed to warm cache for %s: %v", key, err)
		}
	}
	log.Println("✅ Redis Cache Warmed Successfully")
	return nil
}
