package main

import (
	"bytes"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const (
	totalRequests = 50
	url           = "http://api.localhost/api/tickets"
	payload       = `{"match_id": 7, "category": "VIP", "quantity": 1, "user_id": 1}`
)

func main() {
	var wg sync.WaitGroup
	successCount := 0
	failCount := 0
	start := time.Now()

	fmt.Printf("🚀 Starting Stress Test: %d concurrent requests...\n", totalRequests)

	// Mutex for safe counter updates
	var mu sync.Mutex

	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			resp, err := http.Post(url, "application/json", bytes.NewBuffer([]byte(payload)))
			if err != nil {
				fmt.Printf("Request %d failed: %v\n", id, err)
				return
			}
			defer resp.Body.Close()

			mu.Lock()
			if resp.StatusCode == 200 || resp.StatusCode == 201 || resp.StatusCode == 202 {
				successCount++
			} else {
				failCount++
			}
			mu.Unlock()
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	fmt.Printf("\n Stress Test Completed in %v\n", duration)
	fmt.Printf("Success (200/202): %d\n", successCount)
	fmt.Printf("Failed (Sold Out/Error): %d\n", failCount)
}
