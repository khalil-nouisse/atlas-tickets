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

## 🚀 Quick Start (Deployment Guide)

Follow these steps to deploy the entire stack on a local Kubernetes cluster using KIND.

### 1. Prerequisites
- **Docker** desktop installed and running.
- **Kind** (Kubernetes in Docker) installed: `brew install kind`
- **Kubectl** installed: `brew install kubectl`

### 2. Create the Cluster
Use the provided script to create a cluster with NGINX-ready configuration.
```bash
# From the root directory
./scripts/create-cluster.sh
```

### 3. Install NGINX Ingress Controller
We need an Ingress Controller to route traffic from `localhost` to our services.
```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml

# Wait for the controller to be ready (can take a minute)
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=90s
```

### 4. Deploy the Application
Apply the manifests in the following order to respect dependencies.

#### Step A: Base Configurations (Namespace, Secrets, ConfigMaps)
```bash
kubectl apply -f k8s/base/namespace.yaml
kubectl apply -f k8s/base/secrets.yaml
kubectl apply -f k8s/base/configmap.yaml
```

#### Step B: Infrastructure (Databases & Broker)
```bash
kubectl apply -f k8s/infrastructure/postgres
kubectl apply -f k8s/infrastructure/mongodb
kubectl apply -f k8s/infrastructure/redis
kubectl apply -f k8s/infrastructure/rabbitmq
```
*Wait for pods to be ready:* `kubectl get pods -n atlastickets`

#### Step C: Database Initialization (Schema & Seeds)
This job waits for Postgres to be ready, then populates it with tables and data.
```bash
kubectl apply -f k8s/jobs/db-init-job.yaml
```
*Check logs:* `kubectl logs -n atlastickets -l job-name=db-init-job`

#### Step D: Microservices (Apps)
```bash
kubectl apply -f k8s/services/command-service
kubectl apply -f k8s/services/query-service
```

#### Step E: Ingress Rules (Routing)
```bash
kubectl apply -f k8s/infrastructure/ingress/command-ingress.yaml
kubectl apply -f k8s/infrastructure/ingress/query-ingress.yaml
```

### 5. Verification
Check if all pods are running:
```bash
kubectl get pods -n atlastickets
```
You should see:
- `command-service-...` (Running)
- `query-service-...` (Running)
- `postgres-0` (Running)
- `mongo-0` (Running)
- `redis-...` (Running)
- `rabbitmq-...` (Running)
- `db-init-job-...` (Completed)
```

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
