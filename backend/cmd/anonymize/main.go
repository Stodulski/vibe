// Command anonymize erases a final client's identity from the database at
// their own request, keeping the money and the audit trail.
//
// It exists because the owner can delete their account — DELETE
// /api/v1/auth/me, which cascades complexes, courts and clients — and the
// person who booked a court without ever having an account cannot. Their name,
// phone and email sit in clients and in the free text of their bookings, put
// there by a venue they have no login for, and until this command there was no
// path at all for the request "remove me". The runbook beside it
// (backend/docs/runbook-data-deletion.md) is the process; this is the one step
// of it that touches the database.
//
// Usage:
//
//	go run ./cmd/anonymize -client=<uuid> [-db-dsn=<dsn>] [-apply]
//	go run ./cmd/anonymize -complex=<uuid> -phone=<phone> [-db-dsn=<dsn>] [-apply]
//
// The second form is for someone who booked on the storefront and never paid:
// since migration 024 such a person has no clients row, only the snapshot on
// their bookings, so the venue and the phone they gave are the only handle.
// When the phone does belong to a client, it erases that client, whole.
//
// Without -apply it is a dry run: the transaction is opened, the work is done,
// the summary is printed, and the transaction is rolled back. That is the
// default on purpose — an operator answering a request should be able to see
// exactly what would change, for the right person, before anything does.
//
// -db-dsn defaults to DATABASE_URL. The connection needs to be able to read and
// write clients and bookings across tenants; the command declares the
// cross-tenant scope itself (see anonymizeClient).
//
// Precedent: cmd/mpcredkey and cmd/benchmark, the other standalone operator
// tools that talk to the database with pgxpool rather than through cmd/api.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// runTimeout bounds the whole run. It is generous because the operator is
// watching it, and bounded because a wedged database must not leave a
// transaction holding a client row open indefinitely.
const runTimeout = 2 * time.Minute

func main() {
	var (
		clientSpec  string
		complexSpec string
		phone       string
		dsn         string
		apply       bool
	)
	fs := flag.NewFlagSet("anonymize", flag.ExitOnError)
	fs.StringVar(&clientSpec, "client", "", "id of the client to anonymize")
	fs.StringVar(&complexSpec, "complex", "", "id of the venue, with -phone, for someone who has no client row")
	fs.StringVar(&phone, "phone", "", "phone the person booked with, exactly as stored, with -complex")
	fs.StringVar(&dsn, "db-dsn", os.Getenv("DATABASE_URL"), "PostgreSQL DSN (defaults to DATABASE_URL)")
	fs.BoolVar(&apply, "apply", false, "write the changes; without it the run is rolled back and only reported")
	if err := fs.Parse(os.Args[1:]); err != nil {
		log.Fatal(err)
	}

	byClient := clientSpec != ""
	byPhone := complexSpec != "" || phone != ""
	if byClient == byPhone {
		fs.Usage()
		log.Fatal("anonymize: give either -client, or -complex with -phone")
	}
	if byPhone && (complexSpec == "" || phone == "") {
		fs.Usage()
		log.Fatal("anonymize: -complex and -phone go together")
	}
	if dsn == "" {
		log.Fatal("anonymize: -db-dsn is empty and DATABASE_URL is not set")
	}

	var who target
	if byClient {
		clientID, err := uuid.Parse(clientSpec)
		if err != nil {
			log.Fatalf("anonymize: -client is not a uuid: %v", err)
		}
		who = target{clientID: clientID}
	} else {
		complexID, err := uuid.Parse(complexSpec)
		if err != nil {
			log.Fatalf("anonymize: -complex is not a uuid: %v", err)
		}
		who = target{complexID: complexID, phone: phone}
	}

	if err := run(who, dsn, apply); err != nil {
		log.Fatalf("anonymize: %v", err)
	}
}

// target is who to erase: a client id, or a venue and a phone.
type target struct {
	clientID  uuid.UUID
	complexID uuid.UUID
	phone     string
}

func (t target) String() string {
	if t.phone != "" {
		return fmt.Sprintf("phone %s at complex %s", t.phone, t.complexID)
	}
	return "client " + t.clientID.String()
}

func run(who target, dsn string, apply bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connecting: %w", err)
	}
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("opening the transaction: %w", err)
	}
	// Rollback after a successful Commit is a no-op, and on the dry-run path it
	// is the whole mechanism: the work runs, the summary is printed from what
	// it found, and nothing is kept.
	defer func() { _ = tx.Rollback(ctx) }()

	var result Result
	if who.phone != "" {
		result, err = anonymizeByPhone(ctx, tx, who.complexID, who.phone)
	} else {
		result, err = anonymizeClient(ctx, tx, who.clientID)
	}
	if err != nil {
		return err
	}
	// Nobody found is reported, not passed off as a successful no-op.
	if !result.ClientFound && result.SnapshotsCleared == 0 {
		return fmt.Errorf("nobody found for %s", who)
	}

	if apply {
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("committing: %w", err)
		}
	}

	report(os.Stdout, who, result, apply)
	return nil
}

// report prints what happened, in the shape an operator can paste into the
// record of the request.
func report(w *os.File, who target, result Result, apply bool) {
	verb := "would be anonymized (dry run; re-run with -apply to write)"
	if apply {
		verb = "anonymized"
	}
	fmt.Fprintf(w, "%s of complex %s %s\n", who, result.Complex, verb)
	if result.ClientFound {
		fmt.Fprintf(w, "  erased:  name, phone, email, client notes\n")
	} else {
		fmt.Fprintf(w, "  no client row: the person never paid a booking here\n")
	}
	fmt.Fprintf(w, "  erased:  notes on %d booking(s)\n", result.BookingNotesCleared)
	fmt.Fprintf(w, "  erased:  the name and contact on %d unpaid booking(s)\n", result.SnapshotsCleared)
	fmt.Fprintf(w, "  phone is now %q\n", result.Phone)
	fmt.Fprintf(w, "  kept:    %d booking(s), %d payment(s) — the venue's books and the refund state\n",
		result.Bookings, result.Payments)
	fmt.Fprintf(w, "  kept:    %d audit entry/entries for this complex — the record of who did what\n",
		result.AuditEntries)
}
