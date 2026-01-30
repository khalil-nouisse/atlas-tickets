package mongo

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var ProductCollection *mongo.Collection
var OrderCollection *mongo.Collection

// ConnectDB initializes the database connection
func ConnectDB() {
	// 1. Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, relying on system environment variables")
	}

	// 2. Get the URI
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		log.Fatal("MONGO_URI is not set in environment")
	}

	// 3. Create a Context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 4. Connect to the Client
	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatal(err)
	}

	// 5. Ping the database
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal("Failed to ping MongoDB:", err)
	}

	fmt.Println("✅ Connected to MongoDB Successfully!")

	// 6. Assign the collection
	dbName := os.Getenv("DB_NAME")
	colName := os.Getenv("COLLECTION_NAME")
	ProductCollection = client.Database(dbName).Collection(colName)
	OrderCollection = client.Database(dbName).Collection("orders")
}
