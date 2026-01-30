# Polyglot CQRS & Event-Driven Architecture Demo

![Status](https://img.shields.io/badge/Status-Active-success)
![Docker](https://img.shields.io/badge/Docker-Enabled-blue)
![Architecture](https://img.shields.io/badge/Architecture-CQRS%20%2F%20Event--Driven-orange)

## 📖 Overview

This project is a proof-of-concept implementation of the **CQRS (Command Query Responsibility Segregation)** pattern combined with an **Event-Driven Architecture**. 

It demonstrates a "Polyglot Persistence" approach, utilizing **Node.js** and **PostgreSQL** for high-integrity write operations, and **Go** with **MongoDB** for high-performance read operations. The two systems are decoupled via **RabbitMQ**, ensuring asynchronous consistency and scalability.

## 🏗️ Architecture

![Architecture](images/architecture.png)

The system is divided into two distinct parts:

1.  **Command Side (Write Optimized):**
    * **Stack:** Node.js (Express) + PostgreSQL.
    * **Role:** Handles ACID-compliant transactions (Orders, Inventory). It accepts user requests, validates business logic, updates the relational database, and publishes an event to the message broker.

2.  **Message Broker:**
    * **Stack:** RabbitMQ.
    * **Role:** Acts as an asynchronous buffer, decoupling the write service from the read service to ensure system resilience.

3.  **Query Side (Read Optimized):**
    * **Stack:** Go + MongoDB.
    * **Role:** Consumes events from the broker. The Go worker transforms relational data into a document-oriented structure and inserts it into MongoDB for fast, denormalized querying.

## 🛠️ Tech Stack

* **Infrastructure:** Docker & Docker Compose
* **Command Service:** Node.js, PostgreSQL (SQL)
* **Message Broker:** RabbitMQ (AMQP)
* **Query Worker:** Go, MongoDB (NoSQL)
* **Caching (Optional):** Redis

## 🚀 Getting Started

This project is fully containerized. You can spin up the entire infrastructure with a single command.

### Prerequisites
* Docker & Docker Compose

### Installation

1.  **Clone the repository**
    ```bash
    git clone [https://github.com/your-username/cqrs-event-driven-demo.git](https://github.com/your-username/cqrs-event-driven-demo.git)
    cd cqrs-event-driven-demo
    ```

2.  **Start the services**
    ```bash
    docker-compose up --build
    ```

3.  **Access the services**
    * **Node.js API:** `http://localhost:3000`
    * **RabbitMQ Dashboard:** `http://localhost:15672`
    * **MongoDB:** `mongodb://localhost:27017`

## 💡 Key Concepts Demonstrated
* **Microservices Communication:** Asynchronous messaging vs. synchronous HTTP.
* **Data Consistency:** Handling eventual consistency between SQL and NoSQL databases.
* **Polyglot Programming:** Interfacing between JavaScript and Go ecosystems.
* **Containerization:** Orchestrating multi-service environments.

## 👤 Author
**[Your Name]** *Software Engineering Student at ENSAM Meknes*
