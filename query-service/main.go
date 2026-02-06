package main

import (
	"context"
	"log"
	"query-service/database/mongo"
	"query-service/database/postgres"
	"query-service/database/redis"
	_ "query-service/docs"
	"query-service/events"
	"query-service/services"

	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title Query Service API
// @version 1.0
// @description API for querying Products and Orders
// @host localhost:8081
// @BasePath /
func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on system environment variables")
	}

	//CREATE DEPENDENCIES
	pgPool := postgres.GetPostgresClient()
	redisClient := redis.GetRedisClient()
	mongoClient := mongo.GetMongoCollection()

	// Warm the Cache
	go func() {
		// Run in background so we don't block startup if expensive
		if err := redis.WarmCach(context.Background(), pgPool, redisClient); err != nil {
			log.Printf("Failed to warm cache: %v", err)
		}
	}()

	//INJECT DEPENDENCIES
	bookingService := services.NewBookingService(pgPool, redisClient, mongoClient)

	//Start HTTP Server in BACKGROUND
	r := gin.Default()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})
	r.GET("/tickets/available", services.GetAvailabilityHandler(redisClient, mongoClient, pgPool))
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	go func() {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8081"
		}
		addr := ":" + port
		log.Println(" HTTP Server running on " + addr)
		if err := r.Run(addr); err != nil {
			log.Fatalf("HTTP Server Failed: %v", err)
		}
	}()

	//Start Consumer in FOREGROUND (Blocking)
	//This function contains the "Wait for CTRL+C" logic, so we let it hold the main thread.
	log.Println("RabbitMQ Consumer Starting...")
	events.StartConsumer(bookingService)
}
