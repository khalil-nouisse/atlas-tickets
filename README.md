# AtlasTickets

**A production-grade distributed ticketing system engineered to handle high-concurrency scenarios without race conditions or overselling.**

AtlasTickets simulates the real-world problem of a major event ticket sale — thousands of users simultaneously competing for a limited inventory, as in an AFCON final or a sold-out concert. The core engineering challenge: guarantee that exactly the number of available seats are sold, no more, at any scale.

The system was validated under a controlled peak load of **48,077 concurrent requests**, resulting in **13,328 tickets sold** (the exact seeded capacity), **0 oversold**, and **0 race conditions**.

![Architecture](https://img.shields.io/badge/Pattern-CQRS%20%2F%20Event--Driven-blue)
![Platform](https://img.shields.io/badge/Platform-Kubernetes%20%28K3s%29-326CE5)
![Cloud](https://img.shields.io/badge/Cloud-Oracle%20Cloud%20ARM64-F80000)
![Status](https://img.shields.io/badge/Status-Production-success)

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Key Features](#key-features)
3. [Technical Stack](#technical-stack)
4. [Project Structure](#project-structure)
5. [Key Design Decisions](#key-design-decisions)
6. [Getting Started](#getting-started)
7. [Deployment](#deployment)
8. [Load Testing](#load-testing)
9. [Monitoring](#monitoring)
10. [Performance Results](#performance-results)
11. [CI/CD Pipeline](#cicd-pipeline)
12. [API Reference](#api-reference)
13. [Troubleshooting](#troubleshooting)
14. [Future Improvements](#future-improvements)

---

## Architecture Overview

AtlasTickets is built on a strict **CQRS (Command Query Responsibility Segregation)** pattern backed by an **event-driven message bus**. The write path and read path are fully decoupled and independently scalable.

### System Architecture Diagram

![System Architecture](Assets/SystemArchLight.png)

### Traffic Flow

```
Client / k6 Load Test
        |
        v
Traefik Ingress (Port 80)
        |
        +---> api.<MASTER_IP>.nip.io  --> Command Service (Node.js, Port 3000)
        |                                         |
        |                                   RabbitMQ (AMQP, Port 5672)
        |                                         |
        |                                   Query Service (Go, Port 4000)
        |                                         |
        |                              +----------+----------+
        |                              |          |          |
        |                         PostgreSQL   MongoDB    Redis
        |
        +---> query.<MASTER_IP>.nip.io --> Query Service (Port 4000)
                                               |
                                        MongoDB / Redis (Read)
```

### Write Path (Command Side)
A **Node.js** service accepts HTTP `POST /api/tickets` requests. It validates the payload and immediately publishes an event to **RabbitMQ** (`ticket_queue`), returning `202 Accepted` to the client. This decouples ingestion latency from processing latency and allows the command service to absorb burst traffic without back-pressure.

### Process Path (Query Side)
A **Go** service consumes events from RabbitMQ through a bounded worker pool (semaphore-based, max 100 concurrent workers per pod). For each event, it:

1. Attempts an atomic Redis decrement via Lua script (Layer 1 defense).
2. On success, persists the booking to PostgreSQL (source of truth).
3. Publishes the confirmed booking to MongoDB (CQRS read model).
4. On Redis rejection (inventory at zero), the request is discarded instantly — no database write.

### Read Path
The Query Service exposes `GET /tickets/available`, which reads from **Redis** first (sub-millisecond) and falls back to **MongoDB** if Redis is cold, or **PostgreSQL** if MongoDB is stale.

---

## Key Features

### 1. Race Condition Prevention (Primary Innovation)

The double-booking problem in concurrent systems is typically solved with database row locks (pessimistic locking) or compare-and-swap loops (optimistic locking). Both approaches collapse under extreme concurrency because they create contention on the database itself.

AtlasTickets uses a **multi-layer defense** that eliminates database-level contention entirely:

#### Concurrency Control Flow

![Concurrency Control Flow](Assets/ConcurrencyV2Light.png)

**Layer 1 — Redis Atomic Lua Script**

All inventory checks and decrements happen in a single Redis transaction. The Lua script is guaranteed to execute atomically with no interleaving from other clients:

```lua
local current = tonumber(redis.call('GET', KEYS[1]))
if current == nil then
    return -2  -- Cache miss: fallback to Postgres
end
if current <= 0 then
    return -1  -- Sold out: reject instantly
end
return redis.call('DECR', KEYS[1])  -- Reserve seat atomically
```

This means rejections are handled at the cache layer with **sub-millisecond latency** — no query ever reaches the database for a sold-out event.

**Layer 2 — RabbitMQ Message Queue**

The command service does not call the query service directly. All requests are serialized through RabbitMQ. Even if 50,000 HTTP connections arrive simultaneously, they are buffered in the queue and processed at a controlled rate.

**Layer 3 — Go Worker Pool (Buffered Semaphore)**

The query service processes messages through a bounded semaphore pool. The worker count is configurable via `WORKER_POOL_SIZE` and set to 100 per pod. This prevents goroutine explosion under burst load while maintaining high throughput.

```
Incoming Request --> Redis Layer (atomic Lua) --> [Reject ~72%]
                                              --> [Accept] --> RabbitMQ
                                                                  |
                                                          Go Worker Pool (100 workers)
                                                                  |
                                                       PostgreSQL + MongoDB write
```

**Result:** 48,077 concurrent requests — 0 race conditions, 0 overselling.

### 2. CQRS with Multi-Database Strategy

| Store | Role | Why |
|---|---|---|
| Redis | Real-time inventory counter | Atomic ops, sub-ms latency |
| PostgreSQL | Source of truth for bookings and users | ACID compliance, durable |
| MongoDB | CQRS read model (denormalized projections) | Fast reads, flexible schema |

The read model in MongoDB is updated asynchronously after each confirmed booking. Reads never touch PostgreSQL, eliminating read/write contention on the primary store.

### 3. Kubernetes Auto-Scaling

Both services are configured with **Horizontal Pod Autoscalers** that scale on CPU utilization:

| Service | Min Replicas | Max Replicas | CPU Trigger |
|---|---|---|---|
| Command Service | 2 | 3 | 70% |
| Query Service | 2 | 3 | 70% |

StatefulSets with persistent volumes back all three databases. The command service is intentionally configured to scale up aggressively since it is the ingestion layer exposed directly to the internet.

### 4. Cache Warming and Self-Healing

On startup, the query service asynchronously warms the Redis cache by reading current inventory from PostgreSQL. This prevents cold-start cache misses during the first surge of traffic. If Redis encounters a cache miss at runtime, it falls back to PostgreSQL and repopulates the cache transparently.

### 5. Full Observability

Custom Prometheus metrics are exposed at `/metrics` on the query service:

```
atlastickets_bookings_confirmed_total    # Confirmed bookings (counter)
atlastickets_bookings_sold_out_total     # Sold-out rejections (counter)
atlastickets_bookings_race_condition_total  # Race condition events (counter)
atlastickets_active_workers              # Worker pool utilization (gauge)
```

A `ServiceMonitor` resource enables automatic discovery by the Prometheus Operator. Grafana dashboards display real-time system state during load tests.

---

## Technical Stack

| Component | Technology | Version |
|---|---|---|
| Command Service | Node.js (Express) | 20.x |
| Query Service | Go (Gin, prometheus/client_golang) | 1.22+ |
| Message Broker | RabbitMQ | 3.x |
| Primary Database | PostgreSQL | 15 |
| Read Model | MongoDB | 7.x |
| Cache / Inventory | Redis | 7.x |
| Container Orchestration | Kubernetes (K3s) | v1.34.4 |
| Local Dev Cluster | Kind (Kubernetes in Docker) | Latest |
| Ingress (Cloud) | Traefik | Built-in K3s |
| Ingress (Local) | NGINX Ingress Controller | Latest |
| Monitoring | Prometheus + Grafana | kube-prometheus-stack |
| CI/CD | GitHub Actions | — |
| Load Testing | k6 | Latest |
| Cloud Provider | Oracle Cloud Free Tier | ARM64 (VM.Standard.A1.Flex) |

---

## Project Structure

```
atlas-tickets/
|
+-- command-service/          # Node.js write service (event producer)
|   +-- src/
|   |   +-- routes/           # Express route handlers
|   |   +-- services/         # RabbitMQ publisher logic
|   +-- dockerfile
|
+-- query-service/            # Go read/process service (event consumer)
|   +-- main.go               # Entry point: HTTP server + RabbitMQ consumer
|   +-- events/               # RabbitMQ consumer and worker pool
|   +-- services/             # Booking logic, Redis Lua scripts, availability handler
|   +-- database/
|   |   +-- postgres/         # PostgreSQL client and queries
|   |   +-- mongo/            # MongoDB client and projections
|   |   +-- redis/            # Redis client, Lua scripts, cache warming
|   +-- metrics/              # Prometheus counter and gauge definitions
|   +-- models/               # Shared Go structs
|   +-- docs/                 # Swagger/OpenAPI generated documentation
|
+-- k8s/
|   +-- base/                 # Namespace, Secrets, ConfigMaps
|   +-- infrastructure/
|   |   +-- postgres/         # StatefulSet, PVC, Service
|   |   +-- mongodb/          # StatefulSet, PVC, Service
|   |   +-- redis/            # Deployment, Service
|   |   +-- rabbitmq/         # StatefulSet, PVC, Service
|   |   +-- ingress/          # Traefik IngressRoute rules
|   +-- services/
|   |   +-- command-service/  # Deployment, Service
|   |   +-- query-service/    # Deployment, Service
|   +-- jobs/
|   |   +-- db-init-job.yaml  # One-time DB schema + seed job (100K users)
|   +-- monitoring/
|   |   +-- servicemonitor.yaml # Prometheus ServiceMonitor for query-service
|   +-- kind/                 # Kind-specific ingress patch for local dev
|   +-- hpa.yaml              # HorizontalPodAutoscaler for both services
|
+-- load-tests/               # k6 load test scripts
|   +-- 01-warmup.js          # 100 VUs warmup scenario
|   +-- 02-moderate.js        # 1,200 VUs moderate scenario
|   +-- 03-afcon-peak.js      # 50,000 VUs peak scenario
|   +-- 04-moderatev2.js      # Revised moderate scenario
|
+-- scripts/
|   +-- create-cluster.sh     # Kind cluster creation with NGINX-ready config
|   +-- reset-database.sh     # Reset PostgreSQL, MongoDB, Redis to clean state
|
+-- .github/
|   +-- workflows/
|       +-- cicd-command-service.yml  # CI/CD pipeline for Node.js service
|       +-- cicd-query-service.yml    # CI/CD pipeline for Go service
|
+-- docker-compose.yml        # Local development without Kubernetes
+-- Assets/                   # Architecture diagrams, Grafana screenshots
+-- API-REFERENCE.md          # Full API documentation with examples
+-- KUBERNETES_MANUAL.md      # Kubernetes deployment manual
+-- GRAFANA_MANUAL.md         # Grafana dashboard setup guide
```

---

## Key Design Decisions

### Why CQRS?

The read and write access patterns for a ticketing system are fundamentally asymmetric. During a sale event:
- **Reads** are frequent and need to be fast (check availability before buying).
- **Writes** are bursty and need to be consistent (no overselling).

Coupling both through the same service and database means optimizing for one degrades the other. CQRS allows the command service (Node.js) to be tuned for high-throughput ingestion, while the query service (Go) is tuned for concurrent processing and fast reads.

### Why Redis Instead of Database Locks?

Traditional approaches:

- **Pessimistic locking** (`SELECT FOR UPDATE`): Serializes concurrent writes. Works at low scale, but creates a queue at the database under high concurrency. One slow transaction blocks all others.
- **Optimistic locking** (version columns): Reduces contention but generates high retry volume under peak load, amplifying database write pressure.

**Redis atomic Lua scripts** execute in a single-threaded interpreter inside Redis, making the check-and-decrement operation inherently atomic without locks. Rejections are handled in memory before any I/O to a disk-backed store. At 34,749 rejected requests, this saved ~34,749 unnecessary PostgreSQL writes.

### Why RabbitMQ Instead of Direct Service Calls?

Direct HTTP calls from the command service to the query service would create tight coupling and back-pressure propagation. If the query service is processing at capacity, direct calls either timeout or back up in the command service. RabbitMQ decouples the ingestion rate from the processing rate: the command service publishes at whatever speed it can, and the query service consumes at whatever speed its worker pool permits — independently scalable and no cascading failures.

### Why Go for the Query Service?

Go's goroutines and the `sync.Semaphore` pattern make it natural to implement a bounded worker pool that is both highly concurrent and memory-efficient. Processing 100 concurrent RabbitMQ messages within a single pod, each making database calls, requires true parallelism — not the single-threaded event loop model of Node.js.

### Why K3s on Oracle Cloud Free Tier?

K3s is a CNCF-certified, production-grade Kubernetes distribution with a smaller footprint than standard K8s — critical for the constrained memory of ARM64 free-tier VMs. Oracle Cloud's Always Free tier provides 4x ARM64 VMs (each with 2 OCPUs and 12GB RAM), enabling a realistic multi-node cluster at zero cost.

---

## Getting Started

### Prerequisites

| Tool | Purpose | Install |
|---|---|---|
| Docker | Container runtime | [docs.docker.com](https://docs.docker.com/get-docker/) |
| kubectl | Kubernetes CLI | `brew install kubectl` |
| Kind | Local Kubernetes cluster | `brew install kind` |
| k6 | Load testing | `brew install k6` |
| Go 1.22+ | (Optional) Run stress.go locally | [go.dev](https://go.dev/dl/) |

### Option 1: Docker Compose (Local Development)

The fastest way to run the full stack locally without Kubernetes.

```bash
git clone https://github.com/khalilaitnouisse/atlas-tickets.git
cd atlas-tickets

# Start all services (PostgreSQL, MongoDB, Redis, RabbitMQ, command-service, query-service)
docker-compose up -d --build

# Verify all containers are running
docker-compose ps
```

Services will be available at:
- Command Service: `http://localhost:3000`
- Query Service: `http://localhost:4000`
- RabbitMQ Management: `http://localhost:15672` (guest / guest)

---

## Deployment

### Local Deployment (Kind)

Kind runs a full Kubernetes cluster inside Docker containers, suitable for development and integration testing.

**Step 1 — Create the cluster**

```bash
./scripts/create-cluster.sh
```

**Step 2 — Install the NGINX Ingress Controller**

```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml

kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=90s
```

**Step 3 — Deploy in dependency order**

```bash
# Base: namespace, secrets, configmaps
kubectl apply -f k8s/base/namespace.yaml
kubectl apply -f k8s/base/secrets.yaml
kubectl apply -f k8s/base/configmap.yaml

# Infrastructure: databases and broker
kubectl apply -f k8s/infrastructure/postgres
kubectl apply -f k8s/infrastructure/mongodb
kubectl apply -f k8s/infrastructure/redis
kubectl apply -f k8s/infrastructure/rabbitmq

# Wait for infrastructure pods to be Running
kubectl get pods -n atlastickets -w

# Database initialization: schema creation + 100K user seed
kubectl apply -f k8s/jobs/db-init-job.yaml
kubectl logs -n atlastickets -l job-name=db-init-job -f

# Application services
kubectl apply -f k8s/services/command-service
kubectl apply -f k8s/services/query-service

# Ingress routing
kubectl apply -f k8s/infrastructure/ingress/command-ingress.yaml
kubectl apply -f k8s/infrastructure/ingress/query-ingress.yaml
```

**Step 4 — Verify**

```bash
kubectl get pods -n atlastickets
# Expected: command-service (Running), query-service (Running),
#           postgres-0 (Running), mongo-0 (Running),
#           redis-* (Running), rabbitmq-* (Running),
#           db-init-job-* (Completed)
```

**Endpoints (local):**
- `http://api.localhost/api/tickets`
- `http://query.localhost/tickets/available`

---

### Cloud Deployment (K3s on Oracle Cloud)

The production environment runs on a 3-node K3s cluster on Oracle Cloud ARM64 VMs. The CI/CD pipeline handles automated deployment on every push to `main`.

**Infrastructure:**

| Component | Spec |
|---|---|
| Master Node | VM.Standard.A1.Flex (2 OCPU, 12GB RAM) |
| Worker Nodes (x3) | VM.Standard.A1.Flex (2 OCPU, 12GB RAM) |
| Architecture | linux/arm64 |
| Kubernetes | K3s v1.34.4 |
| Ingress | Traefik (built into K3s) |
| DNS | nip.io wildcard (no DNS registration required) |

**Endpoints (cloud):**
- `http://api.<MASTER_PUBLIC_IP>.nip.io/api/tickets`
- `http://query.<MASTER_PUBLIC_IP>.nip.io/tickets/available`
- `http://grafana.<MASTER_PUBLIC_IP>.nip.io`

**Manual deployment to K3s:**

```bash
# Apply all manifests on the master node
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

kubectl apply -f k8s/base/
kubectl apply -f k8s/infrastructure/
kubectl apply -f k8s/jobs/db-init-job.yaml
kubectl apply -f k8s/services/
kubectl apply -f k8s/hpa.yaml
kubectl apply -f k8s/monitoring/servicemonitor.yaml
```

---

## Load Testing

All load tests use [k6](https://k6.io/). The test scripts target the cloud endpoint but can be modified to point at `http://api.localhost` for local testing.

### Test Scenarios

| Script | VUs | Duration | Purpose |
|---|---|---|---|
| `01-warmup.js` | 100 | 30s | Verify baseline connectivity |
| `02-moderate.js` | 1,200 | ~3 min | Validate worker pool under moderate load |
| `03-afcon-peak.js` | 50,000 | ~9 min | Peak stress test simulating a major event sale |
| `04-moderatev2.js` | Staged | ~4 min | Ramp-up + sustain + ramp-down profile |

### Running Tests

**Prerequisites:**

```bash
# Install k6
brew install k6

# (Optional) Reset database state before each test run
./scripts/reset-database.sh
```

**Warmup test:**

```bash
k6 run load-tests/01-warmup.js
```

**Moderate load test:**

```bash
k6 run load-tests/02-moderate.js
```

**Peak stress test (results saved to JSON):**

```bash
k6 run load-tests/03-afcon-peak.js --out json=results/peak.json
```

### Post-Test Verification

After each test, verify that no overselling occurred and that all three databases are consistent:

```bash
# Check PostgreSQL: sold seats vs total seats
kubectl exec -it postgres-0 -n atlastickets -- psql -U atlas -d atlastickets -c \
  "SELECT match_id, category, total_seats, sold_seats,
          CASE WHEN sold_seats > total_seats THEN 'OVERSOLD' ELSE 'OK' END AS status
   FROM ticket_inventory;"

# Check MongoDB: booking count
kubectl exec -it mongo-0 -n atlastickets -- mongosh tickets_read_db --quiet --eval \
  "print('MongoDB bookings:', db.bookings.countDocuments());"

# Cross-verify: PostgreSQL count must equal MongoDB count
kubectl exec -it postgres-0 -n atlastickets -- psql -U atlas -d atlastickets -c \
  "SELECT COUNT(*) AS postgres_bookings FROM bookings;"
```

#### Database Consistency Verification

![Database Verification](Assets/DatabaseVerification.png)

---

## Monitoring

### Grafana Dashboard

The kube-prometheus-stack is deployed in the `monitoring` namespace. A `ServiceMonitor` resource configures Prometheus to automatically scrape the query service's `/metrics` endpoint every 15 seconds.

![Grafana Dashboard](Assets/GrafanaDashboard.png)

**Access Grafana:**

```bash
kubectl port-forward -n monitoring svc/prometheus-grafana 3000:80
# Open: http://localhost:3000
# Username: admin  |  Password: admin123
```

**Access Prometheus:**

```bash
kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090
# Open: http://localhost:9090
```

### Custom PromQL Queries

| Panel | Query |
|---|---|
| Total Confirmed Bookings | `atlastickets_bookings_confirmed_total` |
| Sold Out Rejections | `atlastickets_bookings_sold_out_total` |
| Active Workers | `atlastickets_active_workers` |
| Booking Rate (per minute) | `rate(atlastickets_bookings_confirmed_total[1m])` |
| Success Rate (%) | `rate(atlastickets_bookings_confirmed_total[1m]) / (rate(atlastickets_bookings_confirmed_total[1m]) + rate(atlastickets_bookings_sold_out_total[1m])) * 100` |

### Verify Prometheus Is Scraping

1. Open Prometheus at `http://localhost:9090`
2. Navigate to **Status > Targets**
3. Search for `query-service` — status must be **UP**

---

## Performance Results

The following results are from the peak load test (48,077 concurrent requests against a seeded inventory of 13,328 tickets):

| Metric | Value |
|---|---|
| Total Requests | 48,077 |
| Tickets Sold | 13,328 (exact capacity) |
| Requests Rejected (Redis layer) | 34,749 |
| Oversold Tickets | 0 |
| Race Conditions | 0 |
| Error Rate | 0.00% |
| Average Rejection Latency | < 1ms (Redis Lua) |
| Peak Throughput | 31 requests/second |
| PostgreSQL vs MongoDB Consistency | Perfect (counts match) |
| Total CI/CD Pipeline Time | ~3 minutes |

### What the Numbers Prove

- **13,328 sold / 13,328 capacity = 100.00% sell-through.** Not 13,327. Not 13,329. Atomicity was maintained under 48,077 competing requests.
- **34,749 rejections at < 1ms** means the Redis layer absorbed ~72% of all load before it touched a database. The database only handled confirmed sales.
- **0.00% error rate** means no request received an incorrect response — every acceptance was a valid booking and every rejection was a deliberate sold-out signal.

---

## CI/CD Pipeline

AtlasTickets uses separate GitHub Actions workflows for each service, triggered only when the corresponding service directory changes. This avoids unnecessary builds when unrelated code is modified.

### CI/CD Pipeline Diagram

![CI/CD Pipeline](Assets/cicdDiagram.png)

### Pipeline Steps

**Trigger:** Push to `main` or `cicd` branch, with path filter on `command-service/**` or `query-service/**`.

```
1. Checkout code
2. Set up QEMU (ARM64 emulation on GitHub's x86 runners)
3. Set up Docker Buildx (multi-architecture build support)
4. Login to Docker Hub
5. Extract short Git SHA (used as image tag for traceability)
6. Build and push Docker image (linux/arm64, tagged :latest and :<sha>)
7. SSH into Oracle Cloud master node
8. kubectl set image (rolling update to new SHA tag)
9. kubectl rollout status (wait for successful rollout before pipeline exits)
```

**Command Service workflow:**

```yaml
- name: Build and Push Docker Image
  uses: docker/build-push-action@v5
  with:
    context: ./command-service
    platforms: linux/arm64
    push: true
    tags: |
      khalilaitnouisse/command-service:latest
      khalilaitnouisse/command-service:${{ steps.vars.outputs.sha_short }}

- name: Deploy to OCI Master Node
  uses: appleboy/ssh-action@v1.0.3
  with:
    script: |
      kubectl set image deployment/command-service \
        command-service=khalilaitnouisse/command-service:${{ steps.vars.outputs.sha_short }} \
        -n atlastickets
      kubectl rollout status deployment/command-service -n atlastickets
```

### Required GitHub Secrets

| Secret | Description |
|---|---|
| `DOCKERHUB_USERNAME` | Docker Hub username |
| `DOCKERHUB_TOKEN` | Docker Hub access token |
| `OCI_MASTER_IP` | Public IP of K3s master node |
| `OCI_USERNAME` | SSH username on master node |
| `OCI_SSH_KEY` | Private SSH key for master node access |

---

## API Reference

### Command Service

**Base URL (local):** `http://api.localhost`
**Base URL (cloud):** `http://api.<MASTER_PUBLIC_IP>.nip.io`

#### POST /api/tickets — Purchase a Ticket

```bash
curl -X POST http://api.<MASTER_IP>.nip.io/api/tickets \
  -H "Content-Type: application/json" \
  -d '{
    "match_id": 3,
    "category": "VIP",
    "user_id": 5000,
    "quantity": 1
  }'
```

**Response (202 Accepted):**

```json
{
  "status": "QUEUED",
  "message": "Ticket request queued for processing",
  "request_id": "uuid-here"
}
```

Field constraints: `match_id` (1-3), `category` ("VIP" or "CAT1"), `user_id` (1000-100999), `quantity` (1-10).

---

### Query Service

**Base URL (local):** `http://query.localhost`
**Base URL (cloud):** `http://query.<MASTER_PUBLIC_IP>.nip.io`

#### GET /tickets/available — Check Availability

```bash
curl "http://query.<MASTER_IP>.nip.io/tickets/available?match_id=3&category=VIP"
```

**Response (200 OK):**

```json
{
  "match_id": 3,
  "category": "VIP",
  "total_seats": 5000,
  "sold_seats": 1234,
  "available": 3766,
  "status": "AVAILABLE"
}
```

#### GET /health — Health Check

```bash
curl http://api.<MASTER_IP>.nip.io/health
# {"status": "ok"}
```

#### GET /metrics — Prometheus Metrics (Internal)

```bash
kubectl port-forward -n atlastickets svc/query-service 4000:4000
curl http://localhost:4000/metrics | grep atlastickets
```

---

## Troubleshooting

### Bookings Not Appearing After a Successful k6 Run

**Symptom:** k6 reports 100% success rate but PostgreSQL shows 0 bookings.

**Diagnosis:**

```bash
# Check RabbitMQ queue depth
kubectl port-forward -n atlastickets svc/rabbitmq-service 15672:15672
# Open http://localhost:15672 (guest / guest) -> Queues -> ticket_queue

# Check query-service consumer logs for errors
kubectl logs -l app=query-service -n atlastickets --tail=100

# Check command-service logs for publish failures
kubectl logs -l component=command-service -n atlastickets --tail=100
```

**Common cause:** The `ticket_queue` exists but the consumer is not connected. Restart the query service:

```bash
kubectl rollout restart deployment query-service -n atlastickets
```

---

### Prometheus Shows query-service as DOWN

**Symptom:** Status > Targets in Prometheus shows `query-service` with state `DOWN` or `Error scraping target: 404`.

**Cause:** Pod is running an image that predates the `/metrics` route implementation.

**Fix:**

```bash
# Force a new rollout to pull the latest image
kubectl rollout restart deployment query-service -n atlastickets
kubectl rollout status deployment query-service -n atlastickets

# Verify the metrics endpoint is accessible
kubectl port-forward -n atlastickets svc/query-service 4000:4000
curl http://localhost:4000/metrics | grep atlastickets
```

---

### Ingress Returns 404 on Cloud (K3s)

**Symptom:** `curl http://api.localhost/api/tickets` returns 404 when the cluster is K3s.

**Cause:** The `kind/` ingress is NGINX-based and uses `api.localhost` as the host rule. K3s uses Traefik with nip.io hostnames.

**Fix:** Use the cloud hostname:

```bash
# Correct (K3s / cloud)
curl http://api.<MASTER_PUBLIC_IP>.nip.io/api/tickets

# Correct (Kind / local)
curl http://api.localhost/api/tickets
```

---

### postgres-0 Pod Stuck in Pending

**Symptom:** `kubectl get pods -n atlastickets` shows `postgres-0` as `Pending`.

**Diagnosis:**

```bash
kubectl describe pod postgres-0 -n atlastickets
# Look for: "no persistent volume available for this claim"
```

**Fix:** Ensure StorageClass is available on the node and apply the PVC:

```bash
kubectl get storageclass
kubectl get pvc -n atlastickets
```

On K3s, the default `local-path` StorageClass is pre-installed. On Kind, ensure the PVC spec does not request a StorageClass that does not exist in the cluster.

---

### Resetting the Database Between Test Runs

```bash
# Reset PostgreSQL bookings, MongoDB bookings, and Redis cache
./scripts/reset-database.sh

# Verify clean state
kubectl exec -it postgres-0 -n atlastickets -- psql -U atlas -d atlastickets -c \
  "SELECT match_id, category, total_seats, sold_seats FROM ticket_inventory;"

kubectl exec -it mongo-0 -n atlastickets -- mongosh tickets_read_db --quiet --eval \
  "print('MongoDB bookings:', db.bookings.countDocuments());"
```

---

## Future Improvements

| Area | Description |
|---|---|
| Outbox Pattern | Implement transactional outbox to guarantee at-least-once delivery from PostgreSQL to RabbitMQ, eliminating the slim possibility of a committed booking not reaching the queue. |
| Dead Letter Queue | Route failed or expired messages to a DLQ with alerting, enabling manual inspection and replay of failed bookings. |
| Distributed Tracing | Integrate OpenTelemetry with Jaeger or Tempo to trace individual requests across command-service, RabbitMQ, and query-service. |
| Authentication | Add JWT-based authentication to the command service to prevent anonymous abuse of the booking endpoint. |
| Rate Limiting | Implement per-user rate limiting at the ingress layer (Traefik middleware) to throttle bot traffic before it enters the system. |
| gRPC Internal Communication | Replace the RabbitMQ queue with gRPC streaming between services for scenarios requiring synchronous confirmation with lower latency. |
| Multi-Region Deployment | Extend to a multi-region active-active setup with Redis Cluster and PostgreSQL logical replication for geographic fault tolerance. |

---

## References

- [API Reference](API-REFERENCE.md) — Full endpoint documentation with curl examples
- [Kubernetes Manual](KUBERNETES_MANUAL.md) — Step-by-step Kind deployment guide
- [Grafana Manual](GRAFANA_MANUAL.md) — Dashboard setup and PromQL reference