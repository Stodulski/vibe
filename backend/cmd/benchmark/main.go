package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// queries, print results); splitting would fragment a single linear script into arbitrarily-named
// helpers without clarifying it. Standalone dev tool, not on any production request path.
//
//nolint:funlen // flat sequential dev/benchmarking-script wiring (connect, seed, run timed
func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		// log.Fatalf exits immediately and would skip the deferred cancel();
		// cancel explicitly first to release the timeout context's resources.
		cancel()
		//nolint:gocritic // exitAfterDefer: the deferred cancel() is already replicated
		// explicitly on the line above, so this log.Fatalf does not actually skip it.
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	// Check if heavy seed data already exists
	var count int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM payments").Scan(&count); err != nil {
		log.Fatalf("count payments: %v", err)
	}

	if count < 1000 {
		fmt.Println("=== Seeding database (100k+ per table) ===")
		seedSQL, err := os.ReadFile("db/seed_100k.sql")
		if err != nil {
			log.Fatalf("read seed: %v", err)
		}

		// Split by semicolons and execute each statement separately
		// to avoid Supabase statement timeout on large transactions
		statements := splitSQL(string(seedSQL))
		for i, stmt := range statements {
			if strings.TrimSpace(stmt) == "" {
				continue
			}
			seedCtx, seedCancel := context.WithTimeout(context.Background(), 10*time.Minute)
			if _, err := pool.Exec(seedCtx, stmt); err != nil {
				seedCancel()
				log.Fatalf("seed statement %d: %v\nSQL: %.200s...", i+1, err, stmt)
			}
			seedCancel()
			if i%5 == 0 {
				fmt.Printf("  executed %d/%d statements\n", i+1, len(statements))
			}
		}
		fmt.Println("Seed completed")
	} else {
		fmt.Println("=== Heavy seed data already exists, skipping ===")
	}

	// Print table counts
	fmt.Println("\n=== Table counts ===")
	tables := []string{"users", "complexes", "courts", "court_prices", "clients", "bookings", "payments", "refresh_tokens", "audit_log"}
	for _, t := range tables {
		var c int
		if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM "+t).Scan(&c); err != nil {
			log.Fatalf("count %s: %v", t, err)
		}
		fmt.Printf("  %-20s %d\n", t, c)
	}

	// Get a real complex_id and court_id for queries
	var complexID, courtID, clientID string
	if err := pool.QueryRow(ctx, "SELECT c.id FROM complexes c JOIN payments p ON p.complex_id = c.id GROUP BY c.id ORDER BY COUNT(*) DESC LIMIT 1").Scan(&complexID); err != nil {
		log.Fatalf("select sample complex_id: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT id FROM courts WHERE complex_id = $1 LIMIT 1", complexID).Scan(&courtID); err != nil {
		log.Fatalf("select sample court_id: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT id FROM clients WHERE complex_id = $1 LIMIT 1", complexID).Scan(&clientID); err != nil {
		log.Fatalf("select sample client_id: %v", err)
	}

	fmt.Printf("\n  Using complex: %s\n", complexID)
	fmt.Printf("  Using court:   %s\n", courtID)

	// Queries to benchmark
	queries := []struct {
		name  string
		query string
		args  []any
	}{
		{
			"GetBookingsByComplex (date range + pagination)",
			`EXPLAIN ANALYZE SELECT * FROM bookings
			 WHERE complex_id = $1 AND date BETWEEN '2026-03-01' AND '2026-03-31'
			   AND (date, id) > ('1970-01-01', '00000000-0000-0000-0000-000000000000')
			 ORDER BY date ASC, id ASC LIMIT 20`,
			[]any{complexID},
		},
		{
			"GetBookingsByCourt (single day)",
			`EXPLAIN ANALYZE SELECT * FROM bookings
			 WHERE court_id = $1 AND date BETWEEN '2026-03-12' AND '2026-03-12'
			 ORDER BY date ASC, start_time ASC`,
			[]any{courtID},
		},
		{
			"GetBookedSlots (court + date)",
			`EXPLAIN ANALYZE SELECT court_id, lower(span), upper(span) FROM bookings
			 WHERE court_id = $1 AND date = '2026-03-15' AND status NOT IN ('cancelled', 'no_show')
			 ORDER BY court_id, start_time`,
			[]any{courtID},
		},
		{
			"Expired-pending sweep (partial index)",
			`EXPLAIN ANALYZE SELECT * FROM bookings
			 WHERE status = 'pending' AND collection_status = 'unpaid'
			   AND created_by IS NULL AND created_at < NOW() - INTERVAL '15 minutes'`,
			nil,
		},
		{
			"CompletePastBookings (partial index)",
			`EXPLAIN ANALYZE SELECT count(*) FROM bookings
			 WHERE status = 'confirmed' AND date < CURRENT_DATE`,
			nil,
		},
		{
			"GetRefreshTokenByHash (new index)",
			`EXPLAIN ANALYZE SELECT * FROM refresh_tokens
			 WHERE token_hash = decode(md5('test'), 'hex')
			   AND expires_at > NOW() AND used_at IS NULL`,
			nil,
		},
		{
			"Dashboard: today revenue (denormalized complex_id)",
			`EXPLAIN ANALYZE SELECT COALESCE(SUM(p.amount), 0)
			 FROM payments p
			 WHERE p.complex_id = $1 AND p.status != 'refunded'
			   AND (p.created_at AT TIME ZONE 'America/Argentina/Buenos_Aires')::date = '2026-03-12'`,
			[]any{complexID},
		},
		{
			"Dashboard: monthly revenue (denormalized complex_id)",
			`EXPLAIN ANALYZE SELECT COALESCE(SUM(p.amount), 0)
			 FROM payments p
			 WHERE p.complex_id = $1 AND p.status != 'refunded'
			   AND (p.created_at AT TIME ZONE 'America/Argentina/Buenos_Aires')::date BETWEEN '2026-02-12' AND '2026-03-12'`,
			[]any{complexID},
		},
		{
			"GetBookingsByClient (client_date index)",
			`EXPLAIN ANALYZE SELECT * FROM bookings
			 WHERE complex_id = $1 AND client_id = $2
			   AND date BETWEEN CURRENT_DATE - INTERVAL '365 days' AND CURRENT_DATE + INTERVAL '30 days'
			 ORDER BY date DESC, start_time DESC LIMIT 20`,
			[]any{complexID, clientID},
		},
		{
			// The predicate is the span overlap the exclusion constraint
			// indexes, not the pair of clock comparisons this used to measure.
			// Those read bookings.end_time, a column that no longer exists —
			// and they were the shape of comparison that could not see a
			// booking crossing midnight in the first place.
			"Booking collision check (InsertSafe)",
			`EXPLAIN ANALYZE SELECT EXISTS(
				SELECT 1 FROM bookings
				WHERE court_id = $1 AND status NOT IN ('cancelled', 'no_show')
				  AND span && tstzrange(
				        booking_starts_at('2026-03-15'::date, '09:30'::time),
				        booking_starts_at('2026-03-15'::date, '09:30'::time) + make_interval(mins => 90),
				        '[)')
			)`,
			[]any{courtID},
		},
		{
			"AuditLog by complex (descending)",
			`EXPLAIN ANALYZE SELECT * FROM audit_log
			 WHERE complex_id = $1
			   AND (created_at, id) < (NOW(), '00000000-0000-0000-0000-000000000000')
			 ORDER BY created_at DESC, id DESC LIMIT 20`,
			[]any{complexID},
		},
		// There is no full-text search benchmark. complexes.search_vector, its
		// GIN index and its trigger are gone (nothing ever searched them): this was the
		// only `@@` in the repository, measuring an index that existed because
		// the column existed. The search that actually runs against complexes
		// is the admin ILIKE below.
		{
			"HasActiveBookings",
			`EXPLAIN ANALYZE SELECT EXISTS(
				SELECT 1 FROM bookings
				WHERE complex_id = $1 AND date >= CURRENT_DATE AND status NOT IN ('cancelled')
			)`,
			[]any{complexID},
		},
	}

	fmt.Println("\n=== EXPLAIN ANALYZE Results ===")

	for _, q := range queries {
		fmt.Printf("\n--- %s ---\n", q.name)
		rows, err := pool.Query(ctx, q.query, q.args...)
		if err != nil {
			fmt.Printf("  ERROR: %v\n", err)
			continue
		}
		var lines []string
		for rows.Next() {
			var line string
			if err := rows.Scan(&line); err != nil {
				break
			}
			lines = append(lines, line)
		}
		rows.Close()

		// Print plan (last few lines have the summary)
		for _, line := range lines {
			fmt.Printf("  %s\n", line)
		}

		// Extract execution time
		for _, line := range lines {
			if strings.Contains(line, "Execution Time") || strings.Contains(line, "Planning Time") {
				fmt.Printf("  >>> %s\n", strings.TrimSpace(line))
			}
		}
	}

	fmt.Println("\n=== Benchmark complete ===")
}

// splitSQL splits a SQL script into individual statements.
// It handles DO $$ ... $$ blocks correctly.
func splitSQL(sql string) []string {
	var statements []string
	var current strings.Builder
	inDollarBlock := false
	lines := strings.Split(sql, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip pure comments
		if strings.HasPrefix(trimmed, "--") && !inDollarBlock {
			continue
		}

		// Track DO $$ blocks
		if !inDollarBlock && (strings.Contains(trimmed, "DO $$") || strings.Contains(trimmed, "AS $$")) {
			inDollarBlock = true
		}
		if inDollarBlock && strings.HasSuffix(trimmed, "$$;") {
			inDollarBlock = false
			current.WriteString(line)
			current.WriteString("\n")
			statements = append(statements, current.String())
			current.Reset()
			continue
		}

		current.WriteString(line)
		current.WriteString("\n")

		// Statement ends with ; (not inside $$ block)
		if !inDollarBlock && strings.HasSuffix(trimmed, ";") {
			stmt := strings.TrimSpace(current.String())
			if stmt != "" && stmt != "BEGIN;" && stmt != "COMMIT;" {
				statements = append(statements, stmt)
			}
			current.Reset()
		}
	}

	if s := strings.TrimSpace(current.String()); s != "" {
		statements = append(statements, s)
	}

	return statements
}
