-- Indexes for performance
CREATE INDEX idx_inventory_match ON ticket_inventory(match_id);
CREATE INDEX idx_bookings_user ON bookings(user_id);