# AtlasTickets

**A production-grade distributed ticketing system engineered to handle extreme concurrency without race conditions or overselling.**

AtlasTickets simulates the real-world problem of a major event ticket sale — thousands of users simultaneously competing for a limited inventory (e.g., an AFCON final or a Taylor Swift concert). The core engineering challenge: guarantee that exactly the number of available seats are sold, no more, at any scale.

![Architecture](https://img.shields.io/badge/Pattern-CQRS%20%2F%20Event--Driven-blue)
![Platform](https://img.shields.io/badge/Platform-Kubernetes%20%28K3s%29-326CE5)
![Cloud](https://img.shields.io/badge/Cloud-Oracle%20Cloud%20ARM64-F80000)
![Status](https://img.shields.io/badge/Status-Production-success)

---

## Architecture Overview

AtlasTickets is built on a strict **CQRS (Command Query Responsibility Segregation)** pattern backed by an event-driven **RabbitMQ** message bus.

![System Architecture](Assets/SystemArchLight.png)

### The Primary Innovation: Zero Database Contention

The "double-booking" problem is classically solved with database row locks (pessimistic locking) or version columns (optimistic locking). Both approaches struggle to scale under burst traffic because they amplify database contention.

This system achieves **Zero Race Conditions** by deploying a multi-layer defense strategy centered around **Atomic Redis Lua scripts**.

![Concurrency Control Flow](Assets/ConcurrencyV2Light.png)

By handling inventory decrements entirely in memory via Lua (Layer 1), buffering valid requests in RabbitMQ (Layer 2), and controlling writes via a bounded Go worker pool (Layer 3), the system achieves massive throughput and guarantees flawless data consistency.

---

## 🚀 Quick Start (Local Docker)

The fastest way to test the system locally without a full Kubernetes cluster is using Docker Compose.

```bash
# 1. Clone the repository
git clone https://github.com/khalilaitnouisse/atlas-tickets.git
cd atlas-tickets

# 2. Start all services and databases
docker-compose up -d --build

# 3. Services are now available
# Command API: http://localhost:3000
# Query API: http://localhost:4000
# RabbitMQ UI: http://localhost:15672 (guest/guest)
```

---

## 📚 Project Documentation

To keep this README clean, the full documentation is split into specialized guides. Please refer to these documents for deep-dives into the architecture, deployment, and testing details:

| Document | Description |
|---|---|
| 📐 [**Architecture & Design Decisions**](DESIGN_DECISIONS.md) | WhyCQRS, Redis over DB locks, and Go concurrency. |
| 📊 [**Load Testing & Performance**](LOAD_TESTING.md) | How the system achieved 48,000+ requests with 0 rejections latency. |
| ⚓ [**Kubernetes Deployment Manual**](KUBERNETES_MANUAL.md) | Step-by-step instructions for Kind (local) and K3s (cloud) clusters. |
| 🔄 [**CI/CD Pipeline**](CICD_PIPELINE.md) | Automated multi-arch builder and deployment workflow. |
| 📡 [**API Reference**](API-REFERENCE.md) | Endpoints, cURL examples, and response schemas. |
| 📈 [**Grafana Manual**](GRAFANA_MANUAL.md) | Prometheus metrics and dashboard configuration. |
| 🛠️ [**Troubleshooting Guide**](TROUBLESHOOTING.md) | Solutions for common deployment or testing issues. |

---

## Technical Stack Summary

- **Backend Logic:** Node.js (Command), Go (Query Service)
- **Message Broker:** RabbitMQ
- **Databases:** PostgreSQL 15, MongoDB 7.x, Redis 7.x
- **Infrastructure:** Kubernetes (K3s & Kind), Docker, Traefik
- **Observability:** Prometheus, Grafana
- **Testing & CI/CD:** k6, GitHub Actions

---

## License

This project is licensed under the MIT License.