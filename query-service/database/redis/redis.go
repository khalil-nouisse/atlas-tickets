package redis

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
)

var Client *redis.Client
var once sync.Once

func GetClient() *redis.Client {
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
		err := godotenv.Load("../.env")
		if err != nil {
			log.Fatal("Error loading .env file")
		}

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
