-- A. Users Table
CREATE TABLE users (
    user_id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    full_name VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- B. Matches Table (AFCON Context)
CREATE TABLE matches (
    match_id SERIAL PRIMARY KEY,
    home_team VARCHAR(50) NOT NULL,
    away_team VARCHAR(50) NOT NULL,
    match_date TIMESTAMP NOT NULL,
    stadium VARCHAR(100) NOT NULL,
    is_active BOOLEAN DEFAULT TRUE
);

-- C. Inventory Table (Category Based + Optimistic Locking)
CREATE TABLE ticket_inventory (
    inventory_id SERIAL PRIMARY KEY,
    match_id INT REFERENCES matches(id),
    category VARCHAR(20) NOT NULL, -- e.g., 'VIP', 'CAT1'
    price DECIMAL(10, 2) NOT NULL,
    total_seats INT NOT NULL,
    sold_seats INT DEFAULT 0,
    version INT DEFAULT 1, -- Optimistic Locking for Concurrency
    
    UNIQUE(match_id, category),
    CONSTRAINT check_capacity CHECK (sold_seats <= total_seats)
);

-- D. Bookings Table (The Transaction Log)
CREATE TABLE bookings (
    bid UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id INT REFERENCES users(id),
    match_id INT REFERENCES matches(id),
    category VARCHAR(20) NOT NULL,
    quantity INT NOT NULL,
    status VARCHAR(20) DEFAULT 'PENDING', -- 'PENDING', 'CONFIRMED', 'FAILED', 'SOLD_OUT'
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

