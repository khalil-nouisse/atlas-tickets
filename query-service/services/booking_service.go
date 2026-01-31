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

func (s *BookingService) ProcessTicketRequest(req models.TicketRequest) error {

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	//start transaction
	tx, err := s.Postgres.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
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
			return fmt.Errorf("inventory not found")
		}
		return fmt.Errorf("database read error: %v", err)
	}

	//Quantity check
	if req.Quantity > inventory.TotalSeats-inventory.SoldSeats {
		log.Printf("SOLD OUT: Match %d (Req: %d, Left: %d)", req.MatchID, req.Quantity, inventory.TotalSeats-inventory.SoldSeats)
		//record SOLDOUT "Booking"
		recordFailed := `INSERT INTO bookings (booking_id, user_id, match_id, category, quantity, status, created_at) 
                         VALUES ($1, $2, $3, $4, $5, $6, $7)`

		_, err = tx.Exec(ctx, recordFailed, req.RequestID, req.UserID, req.MatchID, req.Category, req.Quantity, "SOLD_OUT", time.Now())
		if err != nil {
			return fmt.Errorf("database write error: %v", err)
		}

		//commit the failed soldout booking
		tx.Commit(ctx)
		return nil
	}

	//Update Inventory (optimistic update)
	queryUpdate := `UPDATE ticket_inventory 
					SET sold_seats=sold_seats+$1 , version=version+1
					WHERE inventory_id=$2 AND version=$3`

	cmdTag, err := tx.Exec(ctx, queryUpdate, req.Quantity, inventory.InventoryID, inventory.Version)
	if err != nil {
		return fmt.Errorf("update failed: %v", err)
	}

	//Detect Race Condition
	//Upgrading later to " Redis Atomic Counters) "
	if cmdTag.RowsAffected() == 0 {
		// The version changed, so our update was ignored.(someone else bought the ticket)
		log.Printf("RACE CONDITION DETECTED (Version Mismatch): Match %d / %s", req.MatchID, req.Category)
		return fmt.Errorf("race condition detected") //returnin error to rabbitMQ to retry
	}

	//Record Booking
	queryBook := `INSERT INTO bookings (booking_id, user_id, match_id, category, quantity, status, created_at) 
                  VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err = tx.Exec(ctx, queryBook, req.RequestID, req.UserID, req.MatchID, req.Category, req.Quantity, "CONFIRMED", time.Now())
	if err != nil {
		return fmt.Errorf("database write error: %v", err)
	}

	//Commit PostgresTransaction
	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("commit failed: %v", err)
	}

	log.Printf("Postgres Transaction Committed for Request %s", req.RequestID)
	log.Printf("SUCCESS: Booked %d tickets for Match %d - Category %s", req.Quantity, req.MatchID, req.Category)

	//Update Redis Cache
	redisKey := fmt.Sprintf("inventory:%d:%s", req.MatchID, req.Category)
	s.Redis.DecrBy(ctx, redisKey, int64(req.Quantity))

	//Update MongoDB
	bookingDoc := models.Booking{
		BookingID: req.RequestID,
		UserID:    req.UserID,
		MatchID:   req.MatchID,
		Category:  req.Category,
		Quantity:  req.Quantity,
		Status:    "CONFIRMED",
		CreatedAt: time.Now(),
	}

	_, err = s.Mongo.InsertOne(ctx, bookingDoc)
	if err != nil {
		log.Printf("Mongo Update Failed (User won't see ticket yet): %v", err)
	}

	log.Printf("CQRS Cycle Complete for %s", req.RequestID)
	return nil
}
