package services

import (
	"context"
	"fmt"
	"query-service/models"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/mongo"
)

type AvailabilityCheck struct {
	Redis *redis.Client
	Mongo *mongo.Collection
}

func NewAvailabiltyCheck(redis *redis.Client, mongo *mongo.Collection) *AvailabilityCheck {
	return &AvailabilityCheck{
		Redis: redis,
		Mongo: mongo,
	}
}

func (a *AvailabilityCheck) GetAvailability(req models.Inventory) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Redis key
	redisKey := fmt.Sprintf("match:%d:category:%s", req.MatchID, req.Category)

	// Try Redis cache
	val, err := a.Redis.Get(ctx, redisKey).Result()
	if err == nil {
		return strconv.Atoi(val)
	}

	if err != redis.Nil {
		return 0, err
	}
	return 0, fmt.Errorf("availability not found")

	// TODO : mongo Fallback

	// // Cache miss → MongoDB
	// type Result struct {
	// 	TotalSeats int `bson:"totalSeats"`
	// 	SoldSeats int `bson:"soldSeats"`
	// }

	// filter := bson.M{
	// 	"match_id": req.MatchID,
	// 	"category": req.Category,
	// }

	// opts := options.FindOne().SetProjection(bson.M{
	// 	"totalSeats": 1,
	// 	"_id":        0,
	// })

	// var result Result
	// err = a.Mongo.FindOne(ctx, filter, opts).Decode(&result)
	// if err != nil {
	// 	return 0, err
	// }

	// // Store in Redis
	// _ = a.Redis.Set(ctx, redisKey, result.TotalSeats, 10*time.Minute).Err()

	// return result.TotalSeats, nil
}

func GetAvailabilityHandler(redis *redis.Client, mongo *mongo.Collection) gin.HandlerFunc {
	service := NewAvailabiltyCheck(redis, mongo)

	return func(c *gin.Context) {
		matchIDStr := c.Query("match_id")
		category := c.Query("category")

		if matchIDStr == "" || category == "" {
			c.JSON(400, gin.H{
				"error": "match_id and category are required",
			})
			return
		}

		matchID, err := strconv.Atoi(matchIDStr)
		if err != nil {
			c.JSON(400, gin.H{
				"error": "match_id must be an integer",
			})
			return
		}

		req := models.Inventory{
			MatchID:  matchID,
			Category: category,
		}

		available, err := service.GetAvailability(req)
		if err != nil {
			c.JSON(404, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(200, gin.H{
			"match_id":  matchID,
			"category":  category,
			"available": available,
		})
	}
}
