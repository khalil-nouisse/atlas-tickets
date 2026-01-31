

-- 2. SEED DATA (Initial Mock Data)
-- Insert a test User (You)
INSERT INTO users (email, full_name) 
VALUES ('demo@atlastickets.ma', 'DevOps Engineer');

-- Insert the Big Match: Morocco vs South Africa
INSERT INTO matches (home_team, away_team, match_date, stadium) 
VALUES ('Morocco', 'South Africa', '2026-01-30 20:00:00', 'Stade Adrar - Agadir');

-- Insert Inventory for that Match (Match ID 1)
-- VIP: 50 seats available
-- CAT1: 100 seats available (Low number to test "Sold Out" logic easily)
INSERT INTO ticket_inventory (match_id, category, price, total_seats, sold_seats) 
VALUES 
(1, 'VIP', 2000.00, 50, 0),
(1, 'CAT1', 500.00, 100, 0);