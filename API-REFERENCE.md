Here's the updated and integrated API Documentation:

---

# AtlasTickets API Documentation & Testing Guide

## Table of Contents
1. [System Architecture Overview](#architecture)
2. [API Endpoints Reference](#endpoints)
3. [Monitoring & Observability](#monitoring)
4. [Ports & Services](#ports)
5. [Database Management](#database)
6. [Testing Procedures](#testing)
7. [Troubleshooting](#troubleshooting)

---

## <a name="architecture"></a>1. System Architecture Overview

### **Traffic Flow**

```
User/k6 Load Test
       ↓
Traefik Ingress (Port 80)
       ↓
       ├─→ api.<MASTER_IP>.nip.io → Command Service (Port 3000)
       │                              ↓
       │                           RabbitMQ (Port 5672)
       │                              ↓
       │                           Query Service (Port 4000)
       │                              ↓
       │                           ├─→ PostgreSQL (Port 5432)
       │                           ├─→ MongoDB (Port 27017)
       │                           └─→ Redis (Port 6379)
       │
       └─→ query.<MASTER_IP>.nip.io → Query Service (Port 4000)
                                        ↓
                                     Read from MongoDB/Redis
```

---

## <a name="endpoints"></a>2. API Endpoints Reference

### **Base URLs**

**Local (kind):**
```
Command API: http://api.localhost
Query API:   http://query.localhost
```

**Cloud (K3s):**
```
Command API: http://api.<MASTER_PUBLIC_IP>.nip.io
Query API:   http://query.<MASTER_PUBLIC_IP>.nip.io
```

---

### **Command Service Endpoints (Write Operations)**

#### **POST /api/tickets - Purchase Tickets**

**Purpose:** Submit a ticket purchase request

**URL:** `http://api.<MASTER_IP>.nip.io/api/tickets`

**Method:** `POST`

**Headers:**
```
Content-Type: application/json
```

**Request Body:**
```json
{
  "match_id": 3,
  "category": "VIP",
  "user_id": 1000,
  "quantity": 2
}
```

**Field Validation:**
- `match_id`: Integer (1, 2, or 3)
- `category`: String ("VIP" or "CAT1")
- `user_id`: Integer (1000-100999 for test users)
- `quantity`: Integer (1-10)

**Response (Success):**
```json
{
  "status": "QUEUED",
  "message": "Ticket request queued for processing",
  "request_id": "uuid-here"
}
```
**Status Code:** `202 Accepted`

**Response (Error):**
```json
{
  "error": "Invalid request",
  "details": "user_id is required"
}
```
**Status Code:** `400 Bad Request`

**Example cURL:**
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

---

### **Query Service Endpoints (Read Operations)**

#### **GET /api/tickets/available - Check Ticket Availability**

**Purpose:** Check available tickets for a match/category in real-time

**URL:** `http://query.<MASTER_IP>.nip.io/api/tickets/available`

**Method:** `GET`

**Query Parameters:**
- `match_id` (required): Integer
- `category` (required): String ("VIP" or "CAT1")

**Full URL Example:**
```
http://query.<MASTER_IP>.nip.io/api/tickets/available?match_id=3&category=VIP
```

**Response (Success):**
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
**Status Code:** `200 OK`

**Response (Sold Out):**
```json
{
  "match_id": 3,
  "category": "VIP",
  "total_seats": 5000,
  "sold_seats": 5000,
  "available": 0,
  "status": "SOLD_OUT"
}
```
**Status Code:** `200 OK`

**Example cURL:**
```bash
curl "http://query.<MASTER_IP>.nip.io/api/tickets/available?match_id=3&category=VIP"
```

---

#### **GET /metrics - Prometheus Metrics**

**Purpose:** Exposes Prometheus metrics for monitoring

**URL:** Internal only (accessed via Prometheus scraping)

**Access via Port-Forward:**
```bash
kubectl port-forward -n atlastickets svc/query-service 4000:4000
curl http://localhost:4000/metrics
```

**Exposed Metrics:**
```
atlastickets_bookings_confirmed_total    # Total confirmed bookings
atlastickets_bookings_sold_out_total      # Total sold out responses
atlastickets_bookings_race_condition_total # Race condition events
atlastickets_active_workers               # Current active workers
```

---

#### **GET /health - Health Check**

**Purpose:** Check if services are running

**Command Service:**
```bash
curl http://api.<MASTER_IP>.nip.io/health
```

**Query Service:**
```bash
curl http://query.<MASTER_IP>.nip.io/health
```

**Response:**
```json
{
  "status": "ok"
}
```

---

## <a name="monitoring"></a>3. Monitoring & Observability

### **Grafana Access**

**Namespace:** `monitoring`

**Port-Forward:**
```bash
kubectl port-forward -n monitoring svc/prometheus-grafana 3000:80
```

**URL:** http://localhost:3000

**Credentials:**
- Username: `admin`
- Password: `admin123`

---

### **Prometheus Access**

**Port-Forward:**
```bash
kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090
```

**URL:** http://localhost:9090

---

### **Prometheus ServiceMonitor**

**Location:** `k8s/monitoring/servicemonitor.yaml`

**Key Configuration:**
```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: query-service-monitor
  namespace: monitoring
  labels:
    release: prometheus  # Required for Prometheus to discover
spec:
  selector:
    matchLabels:
      app: query-service
  namespaceSelector:
    matchNames:
    - atlastickets
  endpoints:
  - port: http
    path: /metrics
    interval: 15s
```

**Apply:**
```bash
kubectl apply -f k8s/monitoring/servicemonitor.yaml
```

**Verify Prometheus is Scraping:**
1. Open Prometheus: http://localhost:9090
2. Go to: Status → Targets
3. Search for: `query-service`
4. Status should be: **UP** ✅

---

### **Custom Metrics in Grafana**

**Create Dashboard:**

1. Open Grafana: http://localhost:3000
2. Click **+** → **Dashboard** → **Add new panel**
3. Add these queries:

**Panel 1: Total Confirmed Bookings**
```promql
atlastickets_bookings_confirmed_total
```

**Panel 2: Sold Out Events**
```promql
atlastickets_bookings_sold_out_total
```

**Panel 3: Active Workers**
```promql
atlastickets_active_workers
```

**Panel 4: Request Rate**
```promql
rate(atlastickets_bookings_confirmed_total[1m])
```

**Panel 5: Success Rate (%)**
```promql
rate(atlastickets_bookings_confirmed_total[1m]) / 
(rate(atlastickets_bookings_confirmed_total[1m]) + rate(atlastickets_bookings_sold_out_total[1m])) * 100
```

4. Save dashboard as: `AtlasTickets System Metrics`

---

## <a name="ports"></a>4. Ports & Services Reference

### **External Access (Through Ingress)**

| Service | URL | Port | Purpose |
|---------|-----|------|---------|
| Command API | `http://api.<IP>.nip.io` | 80 | Submit ticket purchases |
| Query API | `http://query.<IP>.nip.io` | 80 | Check availability, read data |
| Grafana | `http://grafana.<IP>.nip.io` | 80 | Monitoring dashboard |

---

### **Internal Services (ClusterIP)**

| Service | Internal DNS | Port | Purpose |
|---------|--------------|------|---------|
| PostgreSQL | `postgres-service.atlastickets.svc.cluster.local` | 5432 | Primary database |
| MongoDB | `mongo-service.atlastickets.svc.cluster.local` | 27017 | Read model (CQRS) |
| Redis | `redis-service.atlastickets.svc.cluster.local` | 6379 | Cache + token bucket |
| RabbitMQ (AMQP) | `rabbitmq-service.atlastickets.svc.cluster.local` | 5672 | Message queue |
| RabbitMQ (Management) | `rabbitmq-service.atlastickets.svc.cluster.local` | 15672 | Admin UI |
| Command Service | `command-service.atlastickets.svc.cluster.local` | 3000 | Write API |
| Query Service | `query-service.atlastickets.svc.cluster.local` | 4000 | Read API + Metrics |

---

### **Access Internal Services (Port-Forward)**

```bash
# PostgreSQL
kubectl port-forward -n atlastickets postgres-0 5432:5432

# MongoDB
kubectl port-forward -n atlastickets mongo-0 27017:27017

# Redis
kubectl port-forward -n atlastickets svc/redis-service 6379:6379

# RabbitMQ Management UI
kubectl port-forward -n atlastickets svc/rabbitmq-service 15672:15672
# Access: http://localhost:15672 (guest/guest)

# Query Service (Direct)
kubectl port-forward -n atlastickets svc/query-service 4000:4000
```

---

## <a name="database"></a>5. Database Management

### ⚠️ **Critical Schema Requirement**

**Before running any tests, ensure these columns exist:**

```bash
kubectl exec -it postgres-0 -n atlastickets -- psql -U atlas -d atlastickets
```

```sql
-- Add version columns (required for optimistic locking)
ALTER TABLE ticket_inventory ADD COLUMN IF NOT EXISTS version INTEGER DEFAULT 0;
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS version INTEGER DEFAULT 0;

-- Verify
\d ticket_inventory
\d bookings

\q
```

**Without these columns, bookings will fail silently!**

---

### **A. Reset Database (Clean Slate)**

#### **Method 1: Quick Script (Recommended)**

```bash
#!/bin/bash
set -e

echo "🔄 Resetting AtlasTickets Database..."

# PostgreSQL
kubectl exec -i postgres-0 -n atlastickets -- psql -U atlas -d atlastickets <<EOF
TRUNCATE TABLE bookings;
UPDATE ticket_inventory SET sold_seats = 0;
SELECT match_id, category, total_seats, sold_seats FROM ticket_inventory;
EOF

# MongoDB
kubectl exec -i mongo-0 -n atlastickets -- mongosh tickets_read_db --quiet --eval "
db.bookings.deleteMany({});
print('Bookings cleared. Count:', db.bookings.find().count());
"

# Redis
kubectl exec -i $(kubectl get pod -n atlastickets -l app=redis -o jsonpath='{.items[0].metadata.name}') -n atlastickets -- redis-cli FLUSHALL

# Restart query service
kubectl rollout restart deployment query-service -n atlastickets

echo "Database reset complete!"
```

**Save as `scripts/reset-database.sh` and run:**
```bash
chmod +x scripts/reset-database.sh
./scripts/reset-database.sh
```

---

### **B. Verify Database State**

```bash
# Check PostgreSQL
kubectl exec -it postgres-0 -n atlastickets -- psql -U atlas -d atlastickets -c "
SELECT match_id, category, total_seats, sold_seats, total_seats - sold_seats AS available
FROM ticket_inventory;"

# Check MongoDB
kubectl exec -it mongo-0 -n atlastickets -- mongosh tickets_read_db --quiet --eval "
db.bookings.find().count()"

# Check Redis
kubectl exec -it $(kubectl get pod -n atlastickets -l app=redis -o jsonpath='{.items[0].metadata.name}') -n atlastickets -- redis-cli GET "ticket_inventory:3:VIP"
```

---

## <a name="testing"></a>6. Testing Procedures

### **A. Manual Testing**

#### **Test 1: Single Purchase**

```bash
# Purchase ticket
curl -X POST http://api.<MASTER_IP>.nip.io/api/tickets \
  -H "Content-Type: application/json" \
  -d '{
    "match_id": 3,
    "category": "VIP",
    "user_id": 1000,
    "quantity": 1
  }'

# Wait 5 seconds for processing

# Check availability
curl "http://query.<MASTER_IP>.nip.io/api/tickets/available?match_id=3&category=VIP"
```

---

### **B. Load Testing with k6**

```bash
# Warmup (100 VUs, 30s)
k6 run load-tests/01-warmup.js

# Moderate (10K VUs, 4 min)
k6 run load-tests/02-moderate.js

# Peak (50K VUs, 9 min)
k6 run load-tests/03-afcon-peak.js --out json=results/peak.json
```

---

### **C. Post-Test Verification**

```bash
# Check for overselling
kubectl exec -it postgres-0 -n atlastickets -- psql -U atlas -d atlastickets -c "
SELECT 
    match_id,
    category,
    total_seats,
    sold_seats,
    CASE WHEN sold_seats > total_seats THEN '❌ OVERSOLD' ELSE 'OK' END as status
FROM ticket_inventory;"

# Verify data consistency
kubectl exec -it postgres-0 -n atlastickets -- psql -U atlas -d atlastickets -c "
SELECT COUNT(*) as postgres_bookings FROM bookings;"

kubectl exec -it mongo-0 -n atlastickets -- mongosh tickets_read_db --quiet --eval "
print('MongoDB bookings:', db.bookings.find().count());"
```

---

## <a name="troubleshooting"></a>7. Troubleshooting

### **Issue: 404 on /metrics Endpoint**

**Symptoms:**
- Prometheus shows: `Error scraping target: 404 Not Found`
- `curl http://localhost:4000/metrics` returns 404

**Cause:** Pod is running an old image version without the `/metrics` route

**Fix:**

```bash
# On master node
cd ~/atlas-tickets/query-service

# Rebuild with new version tag
docker build -t khalilaitnouisse/query-service:v9 .

# Push to Docker Hub (if using)
docker push khalilaitnouisse/query-service:v9

# Import to K3s
docker save khalilaitnouisse/query-service:v9 -o query-v9.tar
sudo k3s ctr images import query-v9.tar

# Update deployment YAML
nano ~/atlas-tickets/k8s/services/query-service/deployment.yaml
# Change: image: atlastickets/query-service:v9

# Apply and restart
kubectl apply -f ~/atlas-tickets/k8s/services/query-service/deployment.yaml
kubectl rollout restart deployment query-service -n atlastickets
```

---

### **Issue: Ingress 404**

**Symptoms:**
- `curl http://api.localhost/api/tickets` returns 404
- Works on local kind but not on K3s

**Cause:** Host header mismatch

**Fix:** Use the exact hostname from ingress:
```bash
# Wrong:
curl http://api.localhost/api/tickets

# Correct:
curl http://api.<MASTER_IP>.nip.io/api/tickets
```

---

### **Issue: Zero Bookings After Test**

**Symptoms:**
- k6 shows 100% success
- PostgreSQL shows 0 bookings

**Diagnosis:**
```bash
# Check RabbitMQ queue
kubectl port-forward -n atlastickets svc/rabbitmq-service 15672:15672
# Open: http://localhost:15672 (guest/guest)

# Check command-service logs
kubectl logs -l component=command-service -n atlastickets --tail=50

# Check query-service logs
kubectl logs -l app=query-service -n atlastickets --tail=50
```

---

### **Issue: High Latency**

**Solutions:**
```bash
# Scale up query service
kubectl scale deployment query-service -n atlastickets --replicas=10

# Increase worker pool
kubectl edit configmap atlastickets-config -n atlastickets
# Change WORKER_POOL_SIZE to "200"

# Restart
kubectl rollout restart deployment query-service -n atlastickets
```

---

## Image Deployment Workflow

**Updating Go Services (v9+ Rollout):**

```bash
# 1. Build and tag new image
docker build -t khalilaitnouisse/query-service:v9 .

# 2. Push to Docker Hub
docker push khalilaitnouisse/query-service:v9

# 3. Update deployment YAML
image: khalilaitnouisse/query-service:v9

# 4. Apply changes
kubectl apply -f k8s/services/query-service/deployment.yaml

# 5. Force restart
kubectl rollout restart deployment query-service -n atlastickets
```

---

## Quick Reference Commands

```bash
# Check all resources
kubectl get all -n atlastickets

# Reset database
./scripts/reset-database.sh

# Test single purchase
curl -X POST http://api.<MASTER_IP>.nip.io/api/tickets \
  -H "Content-Type: application/json" \
  -d '{"match_id":3,"category":"VIP","user_id":1000,"quantity":1}'

# Check availability
curl "http://query.<MASTER_IP>.nip.io/api/tickets/available?match_id=3&category=VIP"

# Access Grafana
kubectl port-forward -n monitoring svc/prometheus-grafana 3000:80

# View metrics
curl http://localhost:4000/metrics | grep atlastickets
```

---

**Save this as `docs/API-REFERENCE.md` in your project!** 📚