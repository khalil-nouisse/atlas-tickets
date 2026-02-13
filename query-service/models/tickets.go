package models

import "time"

type User struct {
	UserID    int       `bson:"user_id" json:"user_id"`
	Email     string    `bson:"email" json:"email"`
	Fullname  string    `bson:"fullname" json:"fullname"`
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
}

type Booking struct {
	BookingID string    `bson:"booking_id" json:"booking_id"`
	UserID    int       `bson:"user_id" json:"user_id"`
	MatchID   int       `bson:"match_id" json:"match_id"`
	Category  string    `bson:"category" json:"category"`
	Quantity  int       `bson:"quantity" json:"quantity"`
	Status    string    `bson:"status" json:"status"`
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
}

type Match struct {
	MatchID   int       `bson:"match_id" json:"match_id"`
	HomeTeam  string    `bson:"home_team" json:"home_team"`
	AwayTeam  string    `bson:"away_team" json:"away_team"`
	MatchDate time.Time `bson:"match_date" json:"match_date"`
	Stadium   string    `bson:"stadium" json:"stadium"`
	IsActive  bool      `bson:"is_active" json:"is_active"`
}

type Inventory struct {
	InventoryID int       `bson:"inventory_id" json:"inventory_id"`
	MatchID     int       `bson:"match_id" json:"match_id"`
	Category    string    `bson:"category" json:"category"`
	Price       float64   `bson:"price" json:"price"`
	TotalSeats  int       `bson:"total_seats" json:"total_seats"`
	SoldSeats   int       `bson:"sold_seats" json:"sold_seats"`
	CreatedAt   time.Time `bson:"created_at" json:"created_at"`
}

// TicketRequest represents the incoming JSON message from RabbitMQ
type TicketRequest struct {
	RequestID string    `json:"request_id"`
	MatchID   int       `json:"match_id"`
	Category  string    `json:"category"`
	Quantity  int       `json:"quantity"`
	UserID    int       `json:"user_id"`
	Timestamp time.Time `json:"timestamp"`
}
