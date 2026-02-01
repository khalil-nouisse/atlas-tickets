

-- 2. SEED DATA (Initial Mock Data)
-- Insert a test User (You)
DELETE FROM users WHERE email = 'demo@atlastickets.ma' AND user_id != 1;
INSERT INTO users (user_id, email, full_name) 
VALUES (1, 'demo@atlastickets.ma', 'DevOps Engineer')
ON CONFLICT (user_id) DO NOTHING;

DELETE FROM users WHERE email = 'test@atlastickets.ma' AND user_id != 2;
INSERT INTO users (user_id, email, full_name) 
VALUES (2, 'test@atlastickets.ma', 'Test User')
ON CONFLICT (user_id) DO NOTHING;

-- Insert the Big Match: Morocco vs South Africa
INSERT INTO matches (match_id, home_team, away_team, match_date, stadium) 
VALUES (1, 'Morocco', 'South Africa', '2026-01-30 20:00:00', 'Stade Adrar - Agadir')
ON CONFLICT (match_id) DO NOTHING;

-- Insert Inventory for that Match (Match ID 1)
-- VIP: 50 seats available
-- CAT1: 100 seats available (Low number to test "Sold Out" logic easily)
INSERT INTO ticket_inventory (match_id, category, price, total_seats, sold_seats) 
VALUES 
(1, 'VIP', 2000.00, 50, 0),
(1, 'CAT1', 500.00, 100, 0)
ON CONFLICT (match_id, category) DO NOTHING;

-- Match 2: Real Madrid vs FC Barcelona
INSERT INTO matches (match_id, home_team, away_team, match_date, stadium)
VALUES (2, 'Real Madrid', 'FC Barcelona', '2026-02-15 21:00:00', 'Santiago Bernabéu')
ON CONFLICT (match_id) DO NOTHING;

-- Inventory for Match 2
INSERT INTO ticket_inventory (match_id, category, price, total_seats, sold_seats)
VALUES
(2, 'VIP', 3000.00, 20, 0),
(2, 'CAT1', 800.00, 150, 0)
ON CONFLICT (match_id, category) DO NOTHING;