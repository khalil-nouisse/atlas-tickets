package services

import (
	"context"
	"fmt"
	"query-service/models"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/mongo"
)

type AvailabilityCheck struct {
	Redis    *redis.Client
	Mongo    *mongo.Collection
	Postgres *pgxpool.Pool
}

func NewAvailabiltyCheck(redis *redis.Client, mongo *mongo.Collection, pg *pgxpool.Pool) *AvailabilityCheck {
	return &AvailabilityCheck{
		Redis:    redis,
		Mongo:    mongo,
		Postgres: pg,
	}
}

func (a *AvailabilityCheck) GetAvailability(req models.Inventory) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Redis key
	redisKey := fmt.Sprintf("availability:%d:category:%s", req.MatchID, req.Category)

	// Try Redis cache
	val, err := a.Redis.Get(ctx, redisKey).Result()
	if err == nil {
		return strconv.Atoi(val)
	}

	if err != redis.Nil {
		return 0, err
	}

	//
	// TODO : cache miss -> MongoDB Fallback , instead if postgress Fallback !

	// Cache miss → Postgres Fallback
	var total, sold int
	query := `SELECT total_seats, sold_seats FROM ticket_inventory WHERE match_id=$1 AND category=$2`
	err = a.Postgres.QueryRow(ctx, query, req.MatchID, req.Category).Scan(&total, &sold)
	if err != nil {
		return 0, fmt.Errorf("availability not found (db)")
	}

	available := total - sold

	// Populate Redis (Read Repair)
	// Cache for 10 minutes
	_ = a.Redis.Set(ctx, redisKey, available, 10*time.Minute).Err()

	return available, nil
}

func GetAvailabilityHandler(redis *redis.Client, mongo *mongo.Collection, pg *pgxpool.Pool) gin.HandlerFunc {
	service := NewAvailabiltyCheck(redis, mongo, pg)

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
