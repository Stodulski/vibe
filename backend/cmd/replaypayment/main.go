// Command replaypayment has the running API process a MercadoPago payment
// again, by its id, through the same path a delivered webhook takes.
//
// It exists for the payments the automatic paths refuse or give up on: a
// collector the platform's read could not name, a venue whose credential had
// to be reconnected, an expired booking the reconciliation stopped asking
// about after its failure budget (2026-11-24: jota-f5's payment 191310158388
// sat captured with nothing left to retry it). Once the cause is fixed, the
// payment only needs to be processed again, and before this command that took
// hand-written SQL against payments and bookings.
//
// It writes one row and nothing else: a webhook_events row for the payment,
// the row MercadoPago's own notification leaves (store.ReplayEvent). The API's
// webhook sweep picks it up within minutes and does the rest — reads the
// payment from MercadoPago, checks its collector and amount, and confirms,
// reopens or refunds what it pays for. Replaying a payment already processed
// changes nothing: the processing is idempotent per payment.
//
// Usage, from backend/:
//
//	railway run --service Backend go run ./cmd/replaypayment <mp-payment-id>
//	go run ./cmd/replaypayment [-db-dsn=<dsn>] <mp-payment-id>
//
// -db-dsn defaults to DATABASE_URL. The command needs only the database; it
// never calls MercadoPago itself.
//
// Precedent: cmd/anonymize and cmd/mpcredkey, the other standalone operator
// tools that talk to the database with pgxpool rather than through cmd/api.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stodulski/vibe-server/internal/data"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// runTimeout bounds the whole run: one connection and one insert.
const runTimeout = 30 * time.Second

// paymentIDPattern is what a MercadoPago payment id looks like. Anything else
// (a booking's uuid pasted by mistake, a preference id) would sit in the inbox
// failing every retry, so it is refused before the database is touched.
var paymentIDPattern = regexp.MustCompile(`^[0-9]+$`)

// inbox is the one store call the command makes.
type inbox interface {
	Insert(ctx context.Context, e *paymentstore.WebhookEvent) error
}

func main() {
	fs := flag.NewFlagSet("replaypayment", flag.ExitOnError)
	dsn := fs.String("db-dsn", os.Getenv("DATABASE_URL"), "PostgreSQL DSN (defaults to DATABASE_URL)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: replaypayment [-db-dsn=<dsn>] <mp-payment-id>")
		fs.PrintDefaults()
	}
	if err := fs.Parse(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
	if fs.NArg() != 1 {
		fs.Usage()
		os.Exit(2)
	}
	if *dsn == "" {
		log.Fatal("replaypayment: -db-dsn or DATABASE_URL must be set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()

	pool, err := pgxpool.New(ctx, *dsn)
	if err != nil {
		cancel()
		//nolint:gocritic // exitAfterDefer: cancel() is called explicitly on the line above.
		log.Fatalf("replaypayment: connect: %v", err)
	}
	defer pool.Close()

	event, err := replay(ctx, &paymentstore.WebhookEvents{DB: data.NewDB(pool)}, fs.Arg(0))
	if err != nil {
		pool.Close()
		cancel()
		log.Fatalf("replaypayment: %v", err)
	}
	fmt.Printf("queued MercadoPago payment %s for processing as webhook event %s\n", event.ExternalID, event.ID)
	fmt.Println("the API's webhook sweep works it within a few minutes; its outcome is the event's status and last_error:")
	fmt.Printf("  SELECT status, retry_count, last_error FROM webhook_events WHERE id = '%s';\n", event.ID)
}

// replay records the inbox row that has the payment worker process
// mpPaymentID again.
func replay(ctx context.Context, store inbox, mpPaymentID string) (*paymentstore.WebhookEvent, error) {
	if !paymentIDPattern.MatchString(mpPaymentID) {
		return nil, fmt.Errorf("%q is not a MercadoPago payment id (digits only)", mpPaymentID)
	}
	event, err := paymentstore.ReplayEvent(mpPaymentID, "payment.replayed")
	if err != nil {
		return nil, err
	}
	if err := store.Insert(ctx, event); err != nil {
		return nil, errors.Join(fmt.Errorf("record the replay of payment %s", mpPaymentID), err)
	}
	return event, nil
}
