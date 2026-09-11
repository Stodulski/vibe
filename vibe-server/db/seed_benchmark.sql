-- Benchmark seed: realistic data volume
-- ~5 owners, ~10 complexes, ~40 courts, ~500 clients, ~15,000 bookings, ~10,000 payments

BEGIN;

-- 1. Users (5 owners)
INSERT INTO users (id, email, password_hash, first_name, last_name, phone, role, email_verified)
SELECT
    gen_random_uuid(),
    'owner' || i || '@test.com',
    decode(md5(random()::text), 'hex'),
    'Owner' || i,
    'Test',
    '+5411' || lpad((10000000 + i)::text, 8, '0'),
    'owner',
    true
FROM generate_series(1, 5) AS i
ON CONFLICT (email) DO NOTHING;

-- 2. Complexes (2 per owner = 10 total)
INSERT INTO complexes (id, owner_id, name, slug, address, city, province, phone, email, deposit_percentage, cancellation_hours, latitude, longitude)
SELECT
    gen_random_uuid(),
    u.id,
    'Club ' || u.first_name || ' #' || s.n,
    'club-' || lower(u.first_name) || '-' || s.n,
    'Av. Test ' || (100 + s.n),
    CASE s.n % 3 WHEN 0 THEN 'Buenos Aires' WHEN 1 THEN 'Cordoba' ELSE 'Rosario' END,
    CASE s.n % 3 WHEN 0 THEN 'Buenos Aires' WHEN 1 THEN 'Cordoba' ELSE 'Santa Fe' END,
    '+5411' || lpad((20000000 + s.n)::text, 8, '0'),
    'club' || s.n || '@test.com',
    30,
    24,
    -34.6 + (random() * 0.1),
    -58.4 + (random() * 0.1)
FROM users u
CROSS JOIN generate_series(1, 2) AS s(n)
WHERE u.email LIKE 'owner%@test.com';

-- 3. Complex schedules (all days open 08:00-23:00)
INSERT INTO complex_schedules (complex_id, day, open_time, close_time)
SELECT c.id, d.day::day_of_week, '08:00'::TIME, '23:00'::TIME
FROM complexes c
CROSS JOIN (VALUES ('monday'), ('tuesday'), ('wednesday'), ('thursday'), ('friday'), ('saturday'), ('sunday')) AS d(day)
WHERE c.slug LIKE 'club-%'
ON CONFLICT (complex_id, day) DO NOTHING;

-- 4. Courts (4 per complex = 40 total)
INSERT INTO courts (id, complex_id, name, sport, court_type)
SELECT
    gen_random_uuid(),
    c.id,
    'Cancha ' || s.n,
    CASE WHEN s.n <= 3 THEN 'padel' ELSE 'tennis' END :: sport_type,
    CASE s.n % 3 WHEN 0 THEN 'indoor' WHEN 1 THEN 'outdoor' ELSE 'semi_covered' END :: court_type
FROM complexes c
CROSS JOIN generate_series(1, 4) AS s(n)
WHERE c.slug LIKE 'club-%';

-- 5. Court prices (per day of week per court)
INSERT INTO court_prices (court_id, price, day_type, time_from, time_to)
SELECT
    co.id,
    CASE
        WHEN d.day IN ('saturday', 'sunday') THEN 15000 + (random() * 5000)::int
        ELSE 10000 + (random() * 3000)::int
    END,
    d.day::day_of_week,
    '08:00'::TIME,
    '23:00'::TIME
FROM courts co
CROSS JOIN (VALUES ('monday'), ('tuesday'), ('wednesday'), ('thursday'), ('friday'), ('saturday'), ('sunday')) AS d(day)
WHERE co.deleted_at IS NULL;

-- 6. Clients (50 per complex = 500 total)
INSERT INTO clients (id, complex_id, first_name, last_name, phone, email)
SELECT
    gen_random_uuid(),
    c.id,
    'Cliente' || s.n,
    'Apellido' || s.n,
    '+5411' || lpad((30000000 + (row_number() OVER ()))::text, 8, '0'),
    'cliente' || (row_number() OVER ()) || '@test.com'
FROM complexes c
CROSS JOIN generate_series(1, 50) AS s(n)
WHERE c.slug LIKE 'club-%';

-- 7. Bookings (~15,000: spread across 2026-03-01 to 2026-09-30)
-- Each court gets ~2-4 bookings per day across the date range
INSERT INTO bookings (complex_id, court_id, client_id, date, start_time, duration_minutes, price, deposit_amount, status, collection_status, created_by, created_at)
SELECT
    co.complex_id,
    co.id,
    cl.id,
    d.date,
    (('08:00'::TIME) + (s.slot * INTERVAL '90 minutes'))::TIME,
    90,
    12000,
    3600,
    CASE
        WHEN d.date < '2026-03-12' THEN 'completed'
        WHEN d.date = '2026-03-12' THEN
            CASE WHEN random() < 0.7 THEN 'confirmed' ELSE 'pending' END
        WHEN d.date <= '2026-03-20' THEN
            CASE WHEN random() < 0.8 THEN 'confirmed' ELSE 'pending' END
        ELSE
            CASE WHEN random() < 0.6 THEN 'confirmed' WHEN random() < 0.8 THEN 'pending' ELSE 'cancelled' END
    END :: booking_status,
    CASE
        WHEN random() < 0.5 THEN 'fully_paid'
        WHEN random() < 0.8 THEN 'deposit_paid'
        ELSE 'unpaid'
    END,
    (SELECT id FROM users WHERE email LIKE 'owner%@test.com' LIMIT 1),
    d.date::TIMESTAMPTZ + (('08:00'::TIME) + (s.slot * INTERVAL '90 minutes'))::INTERVAL - INTERVAL '2 days'
FROM courts co
CROSS JOIN generate_series('2026-03-01'::date, '2026-09-30'::date, '1 day'::interval) AS d(date)
CROSS JOIN generate_series(0, 5) AS s(slot)  -- 6 slots per day (08:00 to 17:00)
JOIN LATERAL (
    SELECT id FROM clients
    WHERE complex_id = co.complex_id
    ORDER BY random()
    LIMIT 1
) cl ON true
WHERE co.deleted_at IS NULL
  AND random() < 0.35  -- ~35% fill rate = realistic occupancy
ON CONFLICT DO NOTHING;

-- 8. Payments (one per non-cancelled booking)
INSERT INTO payments (booking_id, complex_id, amount, service_fee, method, status, created_at)
SELECT
    b.id,
    b.complex_id,
    b.price,
    CASE WHEN random() < 0.3 THEN (b.price * 0.05)::int ELSE 0 END,
    CASE
        WHEN random() < 0.4 THEN 'mercadopago'
        WHEN random() < 0.7 THEN 'cash'
        ELSE 'transfer'
    END :: payment_method,
    b.collection_status::payment_status,
    b.created_at + INTERVAL '5 minutes'
FROM bookings b
WHERE b.status != 'cancelled'
ON CONFLICT DO NOTHING;

-- 9. Refresh tokens (a few per user)
INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
SELECT
    u.id,
    decode(md5(random()::text || s.n::text), 'hex'),
    NOW() + (s.n || ' days')::INTERVAL
FROM users u
CROSS JOIN generate_series(1, 3) AS s(n)
WHERE u.email LIKE 'owner%@test.com';

-- 10. Update client booking counts
UPDATE clients c
SET total_bookings = sub.cnt
FROM (
    SELECT client_id, COUNT(*) AS cnt
    FROM bookings
    WHERE status != 'cancelled'
    GROUP BY client_id
) sub
WHERE c.id = sub.client_id;

-- 11. Some audit log entries
INSERT INTO audit_log (user_id, complex_id, action, entity_type, entity_id, created_at)
SELECT
    (SELECT id FROM users WHERE email LIKE 'owner%@test.com' ORDER BY random() LIMIT 1),
    c.id,
    CASE (random() * 3)::int WHEN 0 THEN 'create' WHEN 1 THEN 'update' ELSE 'delete' END,
    CASE (random() * 2)::int WHEN 0 THEN 'booking' WHEN 1 THEN 'court' ELSE 'client' END,
    gen_random_uuid(),
    NOW() - (random() * 90 || ' days')::INTERVAL
FROM complexes c
CROSS JOIN generate_series(1, 50) AS s(n)
WHERE c.slug LIKE 'club-%';

COMMIT;
