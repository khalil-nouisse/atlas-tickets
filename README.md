# AtlasTickets: Distributed Event Ticketing System

![Status](https://img.shields.io/badge/Status-Active-success)
![Architecture](https://img.shields.io/badge/Architecture-Event--Driven%20%2F%20Redis%20Atomic-blue)

AtlasTickets is a high-performance distributed microservices system designed to handle high-demand ticket sales without race conditions or overselling. It implements the CQRS (Command Query Responsibility Segregation) pattern and uses Redis for atomic inventory management, ensuring data consistency across multiple data stores.

The core objective of this project is to solve the "Double Booking" problem in high-concurrency environments (e.g., ticket scalping bots, flash sales) by utilizing Redis Lua scripting for atomic operations, replacing traditional database locking mechanisms.

---

## Architecture

The system follows a strict CQRS and Event-Driven Architecture:

![Architecture](images/architecture.png)

* **Command Side (Write):** A Node.js service accepts HTTP requests, validates basic input, and publishes booking events to RabbitMQ. It is designed for high-throughput ingestion.
* **Query Side (Process):** A Go (Golang) service consumes events, manages inventory via Redis, and persists data to PostgreSQL and MongoDB.

### Data Persistence Strategy
* **Redis:** The primary source of truth for real-time inventory. Uses atomic Lua scripts to prevent race conditions during ticket reservation.
* **PostgreSQL:** The persistent record of truth for Users, Matches, and finalized Bookings. It serves as the durable backup for Redis.
* **MongoDB:** The Read Model (Query Side). Optimized for fast read operations by the frontend, containing denormalized booking data.

---

## Tech Stack

* **Services:** Node.js (Express), Go (Golang)
* **Message Broker:** RabbitMQ
* **Databases:** PostgreSQL 15, MongoDB, Redis 7
* **Infrastructure:** Kubernetes (Kind), Docker
* **Testing:** Custom Go Stress Tester

---

## Key Features

### 1. Atomic Inventory Management
To handle thousands of concurrent requests, the system moves away from database-level locking (Optimistic/Pessimistic) and utilizes Redis atomic counters.
* **Mechanism:** A Lua script checks availability and decrements the counter in a single atomic operation.
* **Benefit:** Eliminates database contention and row locking, significantly increasing throughput.
* **Compensation:** If a downstream database error occurs after a successful Redis decrement, a compensation transaction automatically reverts the Redis counter.

### 2. Event-Driven Concurrency
* **Asynchronous Processing:** Booking requests are queued in RabbitMQ, decoupling ingestion from processing.
* **Worker Pool:** The Go service implements a bounded semaphore-based worker pool to process events in parallel without overwhelming the database.

### 3. Data Consistency
* **Lazy Loading:** Redis inventory is lazily initialized from PostgreSQL if a cache miss occurs.
* **Self-Healing:** The system includes mechanisms to resync Redis from the persistent store in case of data divergence.

---

## Performance & Load Testing

A custom stress testing script was developed to validate the architecture under load.

**Scenario:**
* **Supply:** 50 VIP Seats for Match 1.
* **Demand:** 50 concurrent requests.

**Results:**
| Metric | Outcome |
| :--- | :--- |
| **Requests Sent** | 50 |
| **Tickets Sold** | 50 (Exactly) |
| **Oversold** | 0 |
| **Processing Time** | < 2s |

---

## How to Run

### Prerequisites
* Docker & Docker Compose
* Kubernetes (Kind) - Optional for K8s deployment
* Go 1.22+ (for local stress testing)

### 1. Start the Infrastructure
```bash
docker-compose up -d --build
```

### 2. Initialize the Database
The system automatically initializes the database schema and seed data via a Kubernetes Job or the Node.js service startup script.

### 3. Run the Stress Test
This script simulates concurrent users trying to buy tickets simultaneously.

```bash
# Run the stress test from the root directory
go run stress.go
```

### 4. Verify Results
Check the PostgreSQL database to confirm no tickets were oversold.

```bash
kubectl exec -it statefulset/postgres -n atlastickets -- psql -U atlas -d atlastickets -c "SELECT sold_seats FROM ticket_inventory WHERE match_id=1;"
```

---

## Project Structure

```
.
├── command-service/      # Node.js API (Producer)
├── query-service/        # Go Worker (Consumer)
│   ├── events/           # RabbitMQ Consumer
│   ├── database/         # Postgres, Mongo, Redis Adapters
│   ├── services/         # Business Logic & Redis Lua Scripts
│   └── models/           # Go Structs
├── k8s/                  # Kubernetes Manifests
└── stress.go             # Stress Testing Script
```