package main

import (
	mongo "query-service/database/mongo"
	redisdb "query-service/database/redis"
	_ "query-service/docs"
	"query-service/events"
	"query-service/handlers"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title Query Service API
// @version 1.0
// @description API for querying Products and Orders
// @host localhost:8081
// @BasePath /
func main() {
	//connect to mongoDB
	mongo.ConnectDB()

	//connect to redis
	redisdb.GetClient()

	// Start RabbitMQ Consumer (Writes)
	go events.StartConsumer()

	// Start HTTP Server (Reads)
	r := gin.Default()
	r.GET("/products", handlers.GetAllProducts)
	r.GET("/orders", handlers.GetAllOrders)
	r.GET("/orders/:id", handlers.GetOrder)

	//Swagger
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	r.Run(":8081") // Listen on port 8081
}
