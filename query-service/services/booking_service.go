package services

import (
	"context"
	"fmt"
	"log"
	"query-service/models"
	"time"

	"query-service/metrics"

	"github.com/go-redis/redis/v8"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/mongo"
)

type BookingService struct {
	Postgres *pgxpool.Pool
	Redis    *redis.Client
	Mongo    *mongo.Collection
}

// For dependency enjecction
func NewBookingService(pg *pgxpool.Pool, r *redis.Client, m *mongo.Collection) *BookingService {
	return &BookingService{
		Postgres: pg,
		Redis:    r,
		Mongo:    m,
	}
}

// Redis First: The Scalable Gatekeeper
func (s *BookingService) ProcessTicketRequest(req models.TicketRequest) error {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Ensure Redis has the inventory (Lazy Load)
	redisKey := fmt.Sprintf("ticket_inventory:%d:%s", req.MatchID, req.Category)
	if s.Redis.Exists(ctx, redisKey).Val() == 0 {
		if err := s.initializeRedisInventory(ctx, req.MatchID, req.Category, redisKey); err != nil {
			return err
		}
	}

	// 2. Atomic Decrement (The "Token Bucket")
	// Lua Script: Check if available > 0, then DECR. Else return -1.
	script := `
		local available = tonumber(redis.call("GET", KEYS[1]) or "0")
		local qty = tonumber(ARGV[1])

		if qty <= 0 then
			return -2
		end

		if available >= qty then
			return redis.call("DECRBY", KEYS[1], qty)
		else
			return -1
		end
	`
	val, err := s.Redis.Eval(
		ctx,
		script,
		[]string{redisKey},
		req.Quantity,
	).Result()

	if err != nil {
		log.Printf("Redis Eval Failed: %v", err)
		return err // Retry in consumer
	}

	//redis query result
	result := val.(int64)

	if result == -1 {
		log.Printf("SOLD OUT (Redis): Match %d", req.MatchID)
		metrics.SoldOutEvents.Inc()
		return nil // Stop processing (Ack message)
	}
	if result == -2 {
		return fmt.Errorf("invalid ticket quantity")
	}

	//We have a token! Proceed to Postgres
	booking, err := s.executePurchase(ctx, req)
	if err != nil {
		// COMPENSATION: We took a token but failed to write to DB. Give it back!
		log.Printf("DB Error: %v. Rolling back Redis token...", err)
		s.Redis.IncrBy(ctx, redisKey, int64(req.Quantity))
		return err // Retry in consumer
	}

	//CQRS Updates
	err = s.updateReadModels(ctx, req, booking)
	if err != nil {
		log.Printf("Soft Update Failed: %v", err)
	}

	return nil
}

// Helper to load inventory from Postgres to Redis if missing
func (s *BookingService) initializeRedisInventory(ctx context.Context, matchID int, category string, key string) error {
	var total, sold int
	var matchDate time.Time
	err := s.Postgres.QueryRow(ctx, "SELECT ti.total_seats, ti.sold_seats, m.match_date FROM ticket_inventory ti JOIN matches m ON ti.match_id = m.match_id WHERE match_id=$1 AND category=$2", matchID, category).Scan(&total, &sold, &matchDate)
	if err != nil {
		log.Printf("Failed to load inventory for Redis init: %v", err)
		return err
	}
	available := total - sold

	ttl := calculateTTL(matchDate)

	// Set NX (Only if not exists, to avoid race conditions resetting it)
	err = s.Redis.SetNX(ctx, key, available, ttl).Err()
	if err != nil {
		log.Printf("Failed to set Redis inventory: %v", err)
		return err
	}

	log.Printf("Initialized Redis Inventory for %s: %d seats", key, available)
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

// The Private Helper for Postgres (The ACID Logic)
func (s *BookingService) executePurchase(ctx context.Context, req models.TicketRequest) (*models.Booking, error) {
	//start transaction
	tx, err := s.Postgres.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	// Update Inventory (Just for record keeping, Redis is the guard)
	// We increment sold_seats just to keep Postgres consistent eventually
	queryUpdate := `UPDATE ticket_inventory SET sold_seats=sold_seats+$1 WHERE match_id=$2 AND category=$3`
	_, err = tx.Exec(ctx, queryUpdate, req.Quantity, req.MatchID, req.Category)
	if err != nil {
		return nil, fmt.Errorf("update inventory failed: %v", err)
	}

	//Insert Booking
	queryBook := `INSERT INTO bookings (booking_id, user_id, match_id, category, quantity, status, created_at) 
                  VALUES ($1, $2, $3, $4, $5, $6, $7)
				  ON CONFLICT (booking_id) DO NOTHING;`

	_, err = tx.Exec(ctx, queryBook, req.RequestID, req.UserID, req.MatchID, req.Category, req.Quantity, "CONFIRMED", time.Now())
	if err != nil {
		return nil, fmt.Errorf("booking insert failed: %v", err)
	}

	//commit transaction
	err = tx.Commit(ctx)
	if err != nil {
		return nil, fmt.Errorf("commit failed: %v", err)
	}

	log.Printf("Booked %d tickets for Match %d", req.Quantity, req.MatchID)
	metrics.ConfirmedBookings.Inc()

	// Return the booking object
	return &models.Booking{
		BookingID: req.RequestID,
		UserID:    req.UserID,
		MatchID:   req.MatchID,
		Category:  req.Category,
		Quantity:  req.Quantity,
		Status:    "CONFIRMED",
		CreatedAt: time.Now(),
	}, nil
}

func (s *BookingService) updateReadModels(ctx context.Context, req models.TicketRequest, booking *models.Booking) error {

	// Update Redis Cache (Safe Decrement)
	// Lua script: Check if key exists. If yes, DECRBY. If no, do nothing (Read-Through will fix it next time).
	redisKey := fmt.Sprintf("availability:%d:%s", req.MatchID, req.Category)

	available, err := s.Redis.Get(ctx,
		fmt.Sprintf("ticket_inventory:%d:%s", req.MatchID, req.Category),
	).Int()
	if err == nil {
		s.Redis.Set(ctx, redisKey, available, 10*time.Second)
	}

	// script := `
	// 	if redis.call("EXISTS", KEYS[1]) == 1 then
	// 		return redis.call("DECRBY", KEYS[1], ARGV[1])
	// 	else
	// 		return 0
	// 	end
	// `
	// err := s.Redis.Eval(ctx, script, []string{redisKey}, req.Quantity).Err()
	// if err != nil {
	// 	log.Printf("Redis Update Failed: %v", err)
	// }

	_, err = s.Mongo.InsertOne(ctx, booking)
	if err != nil {
		log.Printf("Mongo Update Failed (User won't see ticket yet): %v", err)
	}

	log.Printf("CQRS Cycle Complete for %s", req.RequestID)
	return nil
}
