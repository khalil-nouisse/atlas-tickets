package main

import (
	"log"
	"query-service/database/mongo"
	"query-service/database/postgres"
	"query-service/database/redis"
	_ "query-service/docs"
	"query-service/events"
	"query-service/services"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
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

	//INJECT DEPENDENCIES
	bookingService := services.NewBookingService(pgPool, redisClient, mongoClient)

	//Start HTTP Server in BACKGROUND
	r := gin.Default()
	r.GET("/tickets/available", services.GetAvailabilityHandler(redisClient, mongoClient))
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	go func() {
		log.Println(" HTTP Server running on :8081")
		if err := r.Run(":8081"); err != nil {
			log.Fatalf("HTTP Server Failed: %v", err)
		}
	}()

	//Start Consumer in FOREGROUND (Blocking)
	//This function contains the "Wait for CTRL+C" logic, so we let it hold the main thread.
	log.Println("RabbitMQ Consumer Starting...")
	events.StartConsumer(bookingService)
}
