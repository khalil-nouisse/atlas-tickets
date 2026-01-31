package events

import (
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"query-service/models"
	service "query-service/services"
	"syscall"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// EventWrapper is the outer shell of every message
type EventWrapper struct {
	Event     string          `json:"event"`
	RequestID string          `json:"request_id"`
	Payload   json.RawMessage `json:"payload"`
}

// ProductEventPayload matches the payload sent by producer
type ProductEventPayload struct {
	Description string  `json:"p_desc"`
	Quantity    float64 `json:"qte"`
}

func StartConsumer(bookingService *service.BookingService) {

	url := os.Getenv("RABBITMQ_URL")
	queueName := os.Getenv("RABBITMQ_TICKET_QUEUE")
	if url == "" {
		url = "amqp://guest:guest@localhost:5672/"
	}

	var conn *amqp.Connection
	var err error
	maxRetries := 15

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

	//define the queue
	q, err := ch.QueueDeclare(queueName, true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Failed to declare queue: %v", err)
	}

	//start consuming.                   | false for disabling auto ack that garentee presistance  , Only after success, manually run: d.Ack(false) to delete the data. from the mthe message broker
	msgs, err := ch.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		log.Fatalf("Failed to register consumer: %v", err)
	}

	// This channel will receive a value when you press CTRL+C or K8s stops the pod
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 4. Create a "Done" channel to coordinate shutdown
	// We use this to tell the main thread: "Okay, the consumer loop has actually finished."
	doneChan := make(chan bool)

	go func() {
		for d := range msgs {
			var wrapper EventWrapper
			if err := json.Unmarshal(d.Body, &wrapper); err != nil {
				log.Printf("Error parsing wrapper: %v", err)
				d.Ack(false)
				continue
			}

			switch wrapper.Event {
			case "TICKET_REQUESTED":
				var payload models.TicketRequest
				if err := json.Unmarshal(wrapper.Payload, &payload); err != nil {
					log.Printf("Error parsing ticket request payload: %v", err)
					d.Ack(false)
					continue
				}

				log.Printf("Processing Ticket Request: %s", payload.RequestID)

				err = bookingService.ProcessTicketRequest(payload)
				if err != nil {
					log.Printf("Error processing ticket request: %v", err)
					d.Ack(false)
					continue
				}

				//tell rabbitMQ : DONE , dont resend the data "delete it"
				d.Ack(false)

			default:
				log.Printf("Unknown Event Type: %s", wrapper.Event)
			}
		}
		// If we get here, it means the 'msgs' channel was closed (connection died or app is stopping)
		log.Println("Consumer loop finished")
		doneChan <- true
	}()

	log.Printf("Waiting for events. Press CTRL+C to exit")

	// 5. BLOCK HERE until a signal is received
	<-sigChan

	conn.Close() //closing connection -> close channel

	log.Println("Shutting signal received...")

	// 7. Wait for the goroutine to finish
	<-doneChan

	log.Println("Consumer exited cleanly")
}
