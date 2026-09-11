-- Heavy benchmark seed: 100k+ rows per major table
-- Simulates a platform with 50 complexes, 200 courts, 2+ years of bookings

BEGIN;

-- 1. Users (50 owners)
INSERT INTO users (id, email, password_hash, first_name, last_name, phone, role, email_verified)
SELECT
    gen_random_uuid(),
    'bench_owner' || i || '@test.com',
    decode(md5(random()::text), 'hex'),
    'BenchOwner' || i,
    'Test',
    '+5411' || lpad((50000000 + i)::text, 8, '0'),
    'owner',
    true
FROM generate_series(1, 50) AS i
ON CONFLICT (email) DO NOTHING;

-- 2. Complexes (50 total, 1 per owner)
INSERT INTO complexes (id, owner_id, name, slug, description, address, city, province, phone, email, deposit_percentage, cancellation_hours, latitude, longitude)
SELECT
    gen_random_uuid(),
    u.id,
    'BenchClub ' || row_number() OVER (),
    'bench-club-' || row_number() OVER (),
    'Complejo benchmark de padel y tenis en zona ' || CASE (row_number() OVER ()) % 5
        WHEN 0 THEN 'norte' WHEN 1 THEN 'sur' WHEN 2 THEN 'oeste' WHEN 3 THEN 'centro' ELSE 'este' END,
    'Av. Benchmark ' || (row_number() OVER ()) * 100,
    (ARRAY['Buenos Aires','Cordoba','Rosario','Mendoza','La Plata','Mar del Plata','Tucuman','Salta','Santa Fe','Neuquen'])[1 + ((row_number() OVER ()) % 10)],
    (ARRAY['Buenos Aires','Cordoba','Santa Fe','Mendoza','Buenos Aires','Buenos Aires','Tucuman','Salta','Santa Fe','Neuquen'])[1 + ((row_number() OVER ()) % 10)],
    '+5411' || lpad((60000000 + (row_number() OVER ()))::text, 8, '0'),
    'bench' || (row_number() OVER ()) || '@test.com',
    20 + ((row_number() OVER ()) % 4) * 10,
    CASE WHEN (row_number() OVER ()) % 3 = 0 THEN 12 ELSE 24 END,
    -34.6 + (random() * 2),
    -58.4 + (random() * 2)
FROM users u
WHERE u.email LIKE 'bench_owner%@test.com';

-- 3. Complex schedules
INSERT INTO complex_schedules (complex_id, day, open_time, close_time)
SELECT c.id, d.day::day_of_week,
    CASE WHEN d.day IN ('saturday', 'sunday') THEN '09:00'::TIME ELSE '08:00'::TIME END,
    '23:00'::TIME
FROM complexes c
CROSS JOIN (VALUES ('monday'),('tuesday'),('wednesday'),('thursday'),('friday'),('saturday'),('sunday')) AS d(day)
WHERE c.slug LIKE 'bench-club-%'
ON CONFLICT (complex_id, day) DO NOTHING;

-- 4. Courts (4 per complex = 200 total)
INSERT INTO courts (id, complex_id, name, sport, court_type)
SELECT
    gen_random_uuid(),
    c.id,
    'Cancha B' || s.n,
    CASE WHEN s.n <= 3 THEN 'padel' ELSE 'tennis' END :: sport_type,
    CASE s.n % 3 WHEN 0 THEN 'indoor' WHEN 1 THEN 'outdoor' ELSE 'semi_covered' END :: court_type
FROM complexes c
CROSS JOIN generate_series(1, 4) AS s(n)
WHERE c.slug LIKE 'bench-club-%';

-- 5. Court prices
INSERT INTO court_prices (court_id, price, day_type, time_from, time_to)
SELECT
    co.id,
    CASE
        WHEN d.day IN ('saturday', 'sunday') THEN 12000 + (random() * 8000)::int
        ELSE 8000 + (random() * 5000)::int
    END,
    d.day,
    '08:00'::TIME,
    '23:00'::TIME
FROM courts co
CROSS JOIN (VALUES ('monday'),('tuesday'),('wednesday'),('thursday'),('friday'),('saturday'),('sunday')) AS d(day)
WHERE co.name LIKE 'Cancha B%';

-- 6. Clients (2000 per complex = 100,000 total)
-- Using batches to avoid memory issues
DO $$
DECLARE
    c_id UUID;
    c_slug TEXT;
    batch_offset INT;
BEGIN
    FOR c_id, c_slug IN SELECT id, slug FROM complexes WHERE slug LIKE 'bench-club-%'
    LOOP
        FOR batch_offset IN 0..3 LOOP
            INSERT INTO clients (id, complex_id, first_name, last_name, phone, email)
            SELECT
                gen_random_uuid(),
                c_id,
                (ARRAY['Juan','Maria','Carlos','Ana','Pedro','Laura','Diego','Sofia','Martin','Lucia',
                       'Pablo','Valentina','Nicolas','Camila','Matias','Florencia','Santiago','Julieta',
                       'Tomas','Agustina'])[1 + (s.n % 20)],
                (ARRAY['Garcia','Rodriguez','Martinez','Lopez','Gonzalez','Perez','Sanchez','Ramirez',
                       'Torres','Flores','Rivera','Gomez','Diaz','Ruiz','Hernandez','Moreno',
                       'Alvarez','Romero','Fernandez','Castro'])[1 + ((s.n / 20) % 20)],
                '+5411' || lpad((70000000 + hashtext(c_slug || s.n::text) % 9000000 + s.n)::text, 8, '0'),
                NULL
            FROM generate_series(batch_offset * 500 + 1, (batch_offset + 1) * 500) AS s(n)
            ON CONFLICT (complex_id, phone) DO NOTHING;
        END LOOP;
    END LOOP;
END $$;

-- 7. Bookings: ~150,000+ rows spread across 2026-01-01 to 2027-03-31
-- Each court: ~10 slots/day, ~60% fill rate across 15 months
-- Process in monthly batches per complex to manage memory
DO $$
DECLARE
    c_rec RECORD;
    month_start DATE;
    month_end DATE;
BEGIN
    FOR c_rec IN
        SELECT c.id AS complex_id, co.id AS court_id
        FROM complexes c
        JOIN courts co ON co.complex_id = c.id
        WHERE c.slug LIKE 'bench-club-%'
        ORDER BY c.id, co.id
    LOOP
        month_start := '2026-01-01';
        WHILE month_start < '2027-04-01' LOOP
            month_end := (month_start + INTERVAL '1 month')::date;

            INSERT INTO bookings (
                complex_id, court_id, client_id, date,
                start_time, duration_minutes,
                price, deposit_amount, status, collection_status,
                created_by, created_at
            )
            SELECT
                c_rec.complex_id,
                c_rec.court_id,
                cl.id,
                d.date,
                (('08:00'::TIME) + (s.slot * INTERVAL '90 minutes'))::TIME,
                90,
                8000 + (random() * 12000)::int,
                (8000 + (random() * 12000)::int) * 30 / 100,
                CASE
                    WHEN d.date < '2026-03-12' THEN 'completed'
                    WHEN d.date = '2026-03-12' THEN
                        CASE WHEN random() < 0.7 THEN 'confirmed' ELSE 'pending' END
                    WHEN d.date <= CURRENT_DATE + 7 THEN
                        CASE WHEN random() < 0.85 THEN 'confirmed' ELSE 'pending' END
                    ELSE
                        CASE
                            WHEN random() < 0.55 THEN 'confirmed'
                            WHEN random() < 0.80 THEN 'pending'
                            ELSE 'cancelled'
                        END
                END :: booking_status,
                CASE
                    WHEN random() < 0.45 THEN 'fully_paid'
                    WHEN random() < 0.75 THEN 'deposit_paid'
                    ELSE 'unpaid'
                END,
                (SELECT id FROM users WHERE email LIKE 'bench_owner%@test.com' LIMIT 1),
                d.date::TIMESTAMPTZ - INTERVAL '1 day' + (random() * INTERVAL '12 hours')
            FROM generate_series(month_start, month_end - 1, '1 day'::interval) AS d(date)
            CROSS JOIN generate_series(0, 9) AS s(slot)
            JOIN LATERAL (
                SELECT id FROM clients
                WHERE complex_id = c_rec.complex_id
                ORDER BY md5(c_rec.court_id::text || d.date::text || s.slot::text)
                LIMIT 1
            ) cl ON true
            WHERE random() < 0.55
            ON CONFLICT DO NOTHING;

            month_start := month_end;
        END LOOP;
    END LOOP;
END $$;

-- 8. Payments (~100k+, one per non-cancelled booking that doesn't have one yet)
INSERT INTO payments (booking_id, amount, service_fee, method, status, created_at)
SELECT
    b.id,
    b.price,
    CASE WHEN random() < 0.3 THEN (b.price * 0.05)::int ELSE 0 END,
    CASE
        WHEN random() < 0.4 THEN 'mercadopago'
        WHEN random() < 0.7 THEN 'cash'
        ELSE 'transfer'
    END :: payment_method,
    b.collection_status::payment_status,
    b.created_at + (random() * INTERVAL '30 minutes')
FROM bookings b
LEFT JOIN payments p ON p.booking_id = b.id
WHERE b.status != 'cancelled'
  AND p.id IS NULL;

-- 9. Refresh tokens (100k+ via many users with many tokens)
INSERT INTO refresh_tokens (user_id, token_hash, expires_at, created_at)
SELECT
    u.id,
    decode(md5(random()::text || s.n::text || u.id::text), 'hex'),
    NOW() + ((s.n % 30) || ' days')::INTERVAL,
    NOW() - ((s.n % 90) || ' days')::INTERVAL
FROM users u
CROSS JOIN generate_series(1, 2000) AS s(n)
WHERE u.email LIKE 'bench_owner%@test.com';

-- 10. Audit log (100k+ entries)
INSERT INTO audit_log (user_id, complex_id, action, entity_type, entity_id, ip_address, created_at)
SELECT
    (SELECT id FROM users WHERE email LIKE 'bench_owner%@test.com' ORDER BY md5(s.n::text) LIMIT 1),
    c.id,
    (ARRAY['create','update','delete','view'])[1 + (s.n % 4)],
    (ARRAY['booking','court','client','payment','complex'])[1 + (s.n % 5)],
    gen_random_uuid(),
    ('192.168.' || (s.n % 255) || '.' || ((s.n * 7) % 255))::INET,
    NOW() - (random() * 365 || ' days')::INTERVAL
FROM complexes c
CROSS JOIN generate_series(1, 2000) AS s(n)
WHERE c.slug LIKE 'bench-club-%';

-- 11. Update client booking counts
UPDATE clients c
SET total_bookings = COALESCE(sub.cnt, 0),
    no_shows = COALESCE(sub.ns, 0)
FROM (
    SELECT client_id,
           COUNT(*) FILTER (WHERE status != 'cancelled') AS cnt,
           COUNT(*) FILTER (WHERE status = 'no_show') AS ns
    FROM bookings
    GROUP BY client_id
) sub
WHERE c.id = sub.client_id;

COMMIT;
