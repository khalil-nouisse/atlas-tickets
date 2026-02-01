-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_inventory_match ON ticket_inventory(match_id);
CREATE INDEX IF NOT EXISTS idx_bookings_user ON bookings(user_id);