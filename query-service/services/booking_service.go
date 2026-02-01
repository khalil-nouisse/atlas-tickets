package services

import (
	"context"
	"fmt"
	"log"
	"query-service/models"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/mongo"
)

type BookingService struct {
	Postgres *pgxpool.Pool
	Redis    *redis.Client
	Mongo    *mongo.Collection
}

func NewBookingService(pg *pgxpool.Pool, r *redis.Client, m *mongo.Collection) *BookingService {
	return &BookingService{
		Postgres: pg,
		Redis:    r,
		Mongo:    m,
	}
}

// The Public API for Booking
func (s *BookingService) ProcessTicketRequest(req models.TicketRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 1 - Hard Transaction postgres (ACID)
	booking, err := s.executePurchase(ctx, req)
	if err != nil {
		//transaction failed (sold out , DB error)
		return err
	}

	// We do NOT update Redis or Mongo for failed bookings.
	if booking.Status == "SOLD_OUT" {
		log.Printf("Request %s was SOLD OUT. Skipping CQRS updates.", req.RequestID)
		return nil // Return nil so RabbitMQ knows we handled it (don't retry)
	}

	// CQRS Softe Update (Redis , Mongo)
	err = s.updateReadModels(ctx, req, booking)
	if err != nil {
		log.Printf("Soft Update Failed : %v", err)
	}

	return nil

}

// The Private Helper for Postgres (The ACID Logic)
func (s *BookingService) executePurchase(ctx context.Context, req models.TicketRequest) (*models.Booking, error) {
	//start transaction
	tx, err := s.Postgres.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %v", err)
	}

	//rollback automatically if we retrn error before commit
	defer tx.Rollback(ctx)

	//check inventory
	var inventory models.Inventory
	queryCheck := `SELECT inventory_id,total_seats,sold_seats,version  
				   FROM  ticket_inventory 
				   WHERE match_id=$1 AND category=$2`

	//.Scan : 1. executes the SQL query , 2. Fetches the first row , 3. Copies each column into the given Go variables in order
	err = tx.QueryRow(ctx, queryCheck, req.MatchID, req.Category).Scan(
		&inventory.InventoryID,
		&inventory.TotalSeats,
		&inventory.SoldSeats,
		&inventory.Version,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			log.Printf("Inventory not found: Match %d / %s", req.MatchID, req.Category)
			return nil, fmt.Errorf("inventory not found")
		}
		return nil, fmt.Errorf("database read error: %v", err)
	}

	//Quantity check
	if req.Quantity > inventory.TotalSeats-inventory.SoldSeats {
		log.Printf("SOLD OUT: Match %d (Req: %d, Left: %d)", req.MatchID, req.Quantity, inventory.TotalSeats-inventory.SoldSeats)
		//record SOLDOUT "Booking"
		recordFailed := `INSERT INTO bookings (booking_id, user_id, match_id, category, quantity, status, created_at) 
                         VALUES ($1, $2, $3, $4, $5, $6, $7)`

		_, err = tx.Exec(ctx, recordFailed, req.RequestID, req.UserID, req.MatchID, req.Category, req.Quantity, "SOLD_OUT", time.Now())
		if err != nil {
			return nil, fmt.Errorf("database write error: %v", err)
		}

		//commit the failed soldout booking
		tx.Commit(ctx)
		return &models.Booking{
			BookingID: req.RequestID,
			UserID:    req.UserID,
			MatchID:   req.MatchID,
			Category:  req.Category,
			Quantity:  req.Quantity,
			Status:    "SOLD_OUT",
			CreatedAt: time.Now(),
		}, nil
	}

	//Update Inventory (optimistic update)
	queryUpdate := `UPDATE ticket_inventory 
					SET sold_seats=sold_seats+$1 , version=version+1
					WHERE inventory_id=$2 AND version=$3`

	cmdTag, err := tx.Exec(ctx, queryUpdate, req.Quantity, inventory.InventoryID, inventory.Version)
	if err != nil {
		return nil, fmt.Errorf("update failed: %v", err)
	}

	//Detect Race Condition
	//TODO : Upgrade later to " Redis Atomic Counters) "
	if cmdTag.RowsAffected() == 0 {
		// The version changed, so our update was ignored.(someone else bought the ticket)
		log.Printf("RACE CONDITION DETECTED (Version Mismatch): Match %d / %s", req.MatchID, req.Category)
		return nil, fmt.Errorf("race condition detected") //returnin error to rabbitMQ to retry
	}

	//Insert Booking
	queryBook := `INSERT INTO bookings (booking_id, user_id, match_id, category, quantity, status, created_at) 
                  VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err = tx.Exec(ctx, queryBook, req.RequestID, req.UserID, req.MatchID, req.Category, req.Quantity, "CONFIRMED", time.Now())
	if err != nil {
		return nil, fmt.Errorf("booking insert failed: %v", err)
	}

	//commit transaction
	err = tx.Commit(ctx)
	if err != nil {
		return nil, fmt.Errorf("commit failed: %v", err)
	}

	log.Printf("Postgres Transaction Committed for Request %s", req.RequestID)
	log.Printf("Booked %d tickets for Match %d - Category %s", req.Quantity, req.MatchID, req.Category)

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

	//Update Redis Cache
	redisKey := fmt.Sprintf("match:%d:category:%s", req.MatchID, req.Category)
	err := s.Redis.DecrBy(ctx, redisKey, int64(req.Quantity)).Err()
	if err != nil {
		log.Printf("Redis Update Failed: %v", err)
	}

	_, err = s.Mongo.InsertOne(ctx, booking)
	if err != nil {
		log.Printf("Mongo Update Failed (User won't see ticket yet): %v", err)
	}

	log.Printf("CQRS Cycle Complete for %s", req.RequestID)
	return nil
}
