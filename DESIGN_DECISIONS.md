# Architecture & Key Design Decisions

This document explains the technical reasoning behind the architecture of AtlasTickets.

## Why CQRS?

The read and write access patterns for a ticketing system are fundamentally asymmetric. During a sale event:
- **Reads** are frequent and need to be fast (check availability before buying).
- **Writes** are bursty and need to be consistent (no overselling).

Coupling both through the same service and database means optimizing for one degrades the other. CQRS allows the command service (Node.js) to be tuned for high-throughput ingestion, while the query service (Go) is tuned for concurrent processing and fast reads.

## Why Redis Instead of Database Locks?

Traditional approaches:

- **Pessimistic locking** (`SELECT FOR UPDATE`): Serializes concurrent writes. Works at low scale, but creates a queue at the database under high concurrency. One slow transaction blocks all others.
- **Optimistic locking** (version columns): Reduces contention but generates high retry volume under peak load, amplifying database write pressure.

**Redis atomic Lua scripts** execute in a single-threaded interpreter inside Redis, making the check-and-decrement operation inherently atomic without locks. Rejections are handled in memory before any I/O to a disk-backed store. At 34,749 rejected requests, this saved ~34,749 unnecessary PostgreSQL writes.

## Why RabbitMQ Instead of Direct Service Calls?

Direct HTTP calls from the command service to the query service would create tight coupling and back-pressure propagation. If the query service is processing at capacity, direct calls either timeout or back up in the command service. RabbitMQ decouples the ingestion rate from the processing rate: the command service publishes at whatever speed it can, and the query service consumes at whatever speed its worker pool permits — independently scalable and no cascading failures.

## Why Go for the Query Service?

Go's goroutines and the `sync.Semaphore` pattern make it natural to implement a bounded worker pool that is both highly concurrent and memory-efficient. Processing 100 concurrent RabbitMQ messages within a single pod, each making database calls, requires true parallelism — not the single-threaded event loop model of Node.js.

## Why K3s on Oracle Cloud Free Tier?

K3s is a CNCF-certified, production-grade Kubernetes distribution with a smaller footprint than standard K8s — critical for the constrained memory of ARM64 free-tier VMs. Oracle Cloud's Always Free tier provides 4x ARM64 VMs (each with 2 OCPUs and 12GB RAM), enabling a realistic multi-node cluster at zero cost.
