package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Config
const (
	TotalRequests = 200 // 200 users
	API_URL       = "http://localhost:5000/api/tickets"
)

func main() {
	var wg sync.WaitGroup
	startTime := time.Now()

	fmt.Printf("STARTING STRESS TEST: %d Users targeting 50 Seats...\n", TotalRequests)

	successCount := 0
	failCount := 0
	var mu sync.Mutex // To safely count results

	for i := 0; i < TotalRequests; i++ {
		wg.Add(1)

		// Simulate User ID (1000 + i) to be unique
		userID := 1000 + i

		go func(uid int) {
			defer wg.Done()

			// JSON Body
			payload := map[string]interface{}{
				"match_id": 1,
				"category": "VIP",
				"quantity": 1, // Everyone buys 1 ticket
				"user_id":  uid,
			}
			jsonValue, _ := json.Marshal(payload)

			// Send Request
			resp, err := http.Post(API_URL, "application/json", bytes.NewBuffer(jsonValue))
			if err != nil {
				fmt.Printf("Request Failed: %v\n", err)
				return
			}
			defer resp.Body.Close()

			// Read Body (Drain it)
			io.Copy(io.Discard, resp.Body)

			mu.Lock()
			if resp.StatusCode == 200 || resp.StatusCode == 202 {
				successCount++
			} else {
				failCount++
			}
			mu.Unlock()
		}(userID)
	}

	wg.Wait()
	duration := time.Since(startTime)

	fmt.Println("------------------------------------------------")
	fmt.Printf("Test Complete in %v\n", duration)
	fmt.Printf("Requests Sent: %d\n", TotalRequests)
	fmt.Printf("HTTP Accepted (202): %d\n", successCount)
	fmt.Printf("HTTP Failed: %d\n", failCount)
	fmt.Println("------------------------------------------------")
	fmt.Println("NOTE: 'Accepted' just means RabbitMQ received it.")
	fmt.Println("Now check the Database for the REAL results.")
}
