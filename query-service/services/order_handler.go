package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	mongodb "query-service/database/mongo"
	redisdb "query-service/database/redis"
	"query-service/models"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson"
	// "go.mongodb.org/mongo-driver/bson/primitive"
)

// GetOrder godoc
// @Summary Get order by ID
// @Description Retrieve a specific order by its ID from MongoDB
// @Tags Orders
// @Produce json
// @Param id path string true "Order ID"
// @Success 200 {object} models.Order
// @Failure 404 {object} map[string]string
// @Router /orders/{id} [get]
func GetOrder(c *gin.Context) {
	id := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var order models.Order

	//1 - check Redis Cache
	rdb := redisdb.GetClient()
	orderKey := fmt.Sprintf("order:%s", id)

	val, err := rdb.Get(ctx, orderKey).Result()
	if err == nil {
		// Cache hit
		fmt.Println("Cache hit for order:", id)
		if err := json.Unmarshal([]byte(val), &order); err == nil {
			c.JSON(http.StatusOK, order)
			return
		} else {
			fmt.Println("Failed to unmarshal Redis data, fallback to Mongo:", err)
		}
	} else if err == redis.Nil {
		// Key not found
		fmt.Println("Cache miss for order:", id)
	} else {
		// Redis connection error
		fmt.Println("Redis error:", err)
	}
	// cache miss-> mongoDB

	err = mongodb.OrderCollection.FindOne(ctx, bson.M{"_id": id}).Decode(&order)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	//data found in mongodb
	//cache the result in redis for 10 minutes
	data, _ := json.Marshal(order)

	err = rdb.Set(ctx, orderKey, data, 10*time.Minute).Err()
	if err != nil {
		fmt.Printf("Failed to cache order in Redis: %v\n", err)
	}

	//return response
	c.JSON(http.StatusOK, order)
}

// GetAllOrders godoc
// @Summary Get all orders
// @Description Retrieve all orders from MongoDB
// @Tags Orders
// @Produce json
// @Success 200 {array} models.Order
// @Failure 500 {object} map[string]string
// @Router /orders [get]
func GetAllOrders(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := mongodb.OrderCollection.Find(ctx, bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch orders"})
		return
	}
	defer cursor.Close(ctx)

	var orders []models.Order
	if err = cursor.All(ctx, &orders); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse orders"})
		return
	}

	c.JSON(http.StatusOK, orders)
}
