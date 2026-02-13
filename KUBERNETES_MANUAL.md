# Atlas Tickets - Kubernetes (KIND) Integration Manual

## What Was Done (Infrastructure & Fixes)

We have successfully migrated the **Atlas Tickets** microservices architecture to a local Kubernetes cluster using **KIND**.

### 1. Infrastructure Services (Stateful)
-   **Postgres**: Managing relational data (Users, Matches, Inventory, Bookings).
    -   *Enhancement*: Added `db-init-job` to automatically Seed data (Schema + Data + Indexes) on startup.
-   **MongoDB**: Managing the Read Model (Projections / Archives).
    -   *Fix*: Configured missing `DB_NAME` and `COLLECTION_NAME` to ensure CQRS updates persist.
-   **Redis**: High-performance Caching for Availability.
    -   *Fix*: Implemented **Read-Through Caching** (Fallback to Postgres on miss).
    -   *Fix*: Implemented **Safe Decrement** (Lua Script) to prevent negative inventory values.
    -   *Fix*: Enabled **Cache Warming** on service startup.
-   **RabbitMQ**: Event Bus for Asynchronous Communication.
    -   *Fix*: Hardened `query-service` consumer configuration to use the correct `ticket_queue`.

### 2. Microservices
-   **Command Service** (Node.js): Handles "Buy Ticket" requests (Write).
    -   *Fix*: Corrected port mapping (3000) and Readiness Probes.
-   **Query Service** (Go): Handles "Check Availability" (Read).
    -   *Fix*: Corrected port mapping (4000).
    -   *Fix*: Integrated Postgres connection for cache rebuilding.

### 3. Networking (Ingress)
-   **Nginx Ingress Controller**: Configured to route traffic from `localhost` to internal services.
    -   `api.localhost` -> `command-service`
    -   `query.localhost` -> `query-service`

---

##  User Manual: How to Use the App

### 1. Prerequisites
Ensure your KIND cluster is running and all pods are ready:
```bash
kubectl get pods -n atlastickets
# Status should be 'Running' or 'Completed' (for db-init-job)
```

### 2. Testing Endpoints (API)

#### A. Check Ticket Availability (Query Service)
Check how many tickets are left for "Morocco vs South Africa" (Match 1, VIP).
```bash
curl "http://query.localhost/tickets/available?match_id=1&category=VIP"
# Expected Output: {"available":50,"category":"VIP","match_id":1}
```

#### B. Buy a Ticket (Command Service)
Purchase a ticket. This triggers the CQRS flow: Command -> Postgres -> RabbitMQ -> Query Service -> Redis & Mongo.
```bash
curl -X POST http://api.localhost/api/tickets \
  -H "Content-Type: application/json" \
  -d '{
    "match_id": 1,
    "category": "VIP",
    "quantity": 2,
    "user_id": 1
  }'
# Expected Output: {"message":"Request received. Processing..."}
```

#### C. Verify Update
Check availability again. It should have decreased by 2.
```bash
curl "http://query.localhost/tickets/available?match_id=1&category=VIP"
# Expected Output: {"available":48,"category":"VIP","match_id":1}
```

### 3. Inspecting Databases (Deep Dive)

Use these commands to access the databases directly for debugging or verification.

#### Postgres (The Source of Truth)
Check the `bookings` table to see confirmed transactions.
```bash
kubectl exec -it -n atlastickets statefulset/postgres -- psql -U atlas -d atlastickets
```
**SQL Commands:**
```sql
SELECT * FROM bookings;
SELECT * FROM ticket_inventory;
\q  -- to exit
```

#### MongoDB (The Read Model)
Check the `bookings` collection to see the CQRS projection.
```bash
kubectl exec -it -n atlastickets statefulset/mongo -- mongosh tickets_read_db
```
**Mongo Commands:**
```javascript
db.bookings.find()
db.bookings.countDocuments()
exit  // to exit
```

#### Redis (The Cache)
Check the real-time partial inventory count.
```bash
kubectl exec -it -n atlastickets deployment/redis -- redis-cli
```
**Redis Commands:**
```bash
KEYS *
GET ticket_inventory:1:category:VIP
exit
```
