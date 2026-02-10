package events

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"os/signal"
	"query-service/models"
	service "query-service/services"
	"strconv"
	"sync"
	"syscall"
	"time"

	"query-service/metrics"

	"github.com/joho/godotenv"
	amqp "github.com/rabbitmq/amqp091-go"
)

// EventWrapper is the outer shell of every message
type EventWrapper struct {
	Event     string          `json:"event"`
	RequestID string          `json:"request_id"`
	Data      json.RawMessage `json:"data"`
}

// ProductEventPayload matches the payload sent by producerˇ
type ProductEventPayload struct {
	Description string  `json:"p_desc"`
	Quantity    float64 `json:"qte"`
}

func StartConsumer(bookingService *service.BookingService) {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	url := os.Getenv("RABBITMQ_URL")
	queueName := os.Getenv("RABBITMQ_TICKET_QUEUE")
	if queueName == "" {
		queueName = "ticket_queue"
	}
	if url == "" {
		url = "amqp://guest:guest@localhost:5672/"
	}

	var conn *amqp.Connection
	var err error
	maxRetries := 15
	value := os.Getenv("WORKER_POOL_SIZE")
	maxWorkers, err := strconv.Atoi(value)
	if err != nil {
		maxWorkers = 4 // default
	}

	log.Println("Workers: ", maxWorkers)

	for i := 0; i < maxRetries; i++ {
		log.Printf("Attempting to connect to RabbitMQ (Attempt %d/%d)...", i+1, maxRetries)

		//Connect to RabbitMQ with retry logic
		conn, err = amqp.Dial(url)

		if err == nil {
			log.Println("Connected to RabbitMQ successfully")
			break
		}
		log.Printf("Failed to connect to RabbitMQ: %v. Retrying in 5 seconds...", err)
		time.Sleep(5 * time.Second)
	}

	if err != nil {
		log.Fatalf("Could not connect to RabbitMQ after %d attempts: %v", maxRetries, err)
	}

	//open channel
	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open a channel: %v", err)
	}

	//define the queue and ensures the queue exists
	q, err := ch.QueueDeclare(queueName,
		true,  //durable
		false, //delete when unused
		false, //exclusive
		false, //no-wait
		nil,   //args
	)
	if err != nil {
		log.Fatalf("Failed to declare queue: %v", err)
	}

	err = ch.Qos(
		maxWorkers, // prefetch count
		0,          // prefetch size
		false,      // global
	)
	if err != nil {
		log.Fatalf("Failed to set QoS: %v", err)
	}

	//start consuming.
	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack - by disabling it we garentee that the message will be deleted from the queue only after the consumer successfully processes it
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		log.Fatalf("Failed to register consumer: %v", err)
	}

	// This channel will receive a value when you press CTRL+C or K8s stops the pod
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	//"Done" channel to coordinate shutdown
	// We use this to tell the main thread: "Okay, the consumer loop has actually finished."
	doneChan := make(chan bool)
	sem := make(chan bool, maxWorkers)
	var wg sync.WaitGroup

	go func() {
		for msg := range msgs {
			sem <- true
			wg.Add(1)

			go func(d amqp.Delivery) {

				defer func() {
					metrics.ActiveWorkers.Dec()
					<-sem
					wg.Done()
				}()
				metrics.ActiveWorkers.Inc()

				var wrapper EventWrapper
				if err := json.Unmarshal(d.Body, &wrapper); err != nil {
					log.Printf("Error parsing wrapper: %v", err)
					d.Ack(false)
					return
				}

				switch wrapper.Event {
				case "TICKET_REQUESTED":
					var payload models.TicketRequest
					if err := json.Unmarshal(wrapper.Data, &payload); err != nil {
						log.Printf("Error parsing ticket request payload: %v", err)
						d.Ack(false)
						return
					}

					// Manually inject the RequestID from the wrapper
					payload.RequestID = wrapper.RequestID

					log.Printf("Processing Ticket Request: %s", payload.RequestID)

					const maxRetries = 5
					for attempt := 0; attempt <= maxRetries; attempt++ {
						err = bookingService.ProcessTicketRequest(payload)

						// BOOKING CONFIRMED OR SOLDOUT , NO RETIES
						if err == nil {
							d.Ack(false)
							return
						}

						//version conflict (race condition) , RETRY !
						if errors.Is(err, service.ErrOptimisticLock) {
							log.Printf("Error processing ticket request: %v , Retrying again", err)
							time.Sleep(backoff(attempt))
							continue
						}

						// Business or fatal error → ACK (don't retry)
						d.Ack(false)
						return
					}

					// Retries exhausted
					// TODO → send to DLQ
					// publishToDLQ(payload)
					log.Printf("Retries exhausted for request %s", payload.RequestID)
					d.Ack(false)

				default:
					log.Printf("Unknown Event Type: %s", wrapper.Event)
					d.Ack(false)
				}
			}(msg)
		}
		// If we get here, it means the 'msgs' channel was closed (connection died or app is stopping)
		log.Println("Consumer loop finished")
		wg.Wait()
		doneChan <- true
	}()

	log.Printf("Waiting for events. Press CTRL+C to exit")

	//BLOCK HERE until a signal is received
	<-sigChan

	conn.Close() //closing connection -> close channel

	log.Println("Shutting signal received...")

	// Wait for the goroutine to finish
	<-doneChan

	log.Println("Consumer exited cleanly")
}

func backoff(attempt int) time.Duration {
	return time.Duration(attempt*attempt) * 50 * time.Millisecond
}
