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
//	go run ./cmd/replaypayment [-db-dsn=<dsn>] -stop <event-id>
//
// The event ending 'processed' means the processing ran, not that the booking
// was confirmed: a collector or amount refusal ends 'processed' too, with its
// own Sentry alert. The command says so, and the operator checks Sentry and the
// booking or registration afterwards.
//
// A payment id MercadoPago does not know (a typo of the right number of digits)
// fails every attempt, and an exhausted event is picked up again on the tail of
// its backoff, so it would be retried for ever. -stop closes such a replay; it
// only touches events this command wrote. Asking MercadoPago first would catch
// the typo up front, but it would need the venue's credential, and this command
// needs only the database by design.
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
	"io"
	"log"
	"os"
	"regexp"
	"time"

	"github.com/google/uuid"
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

// inbox is the store calls the command makes.
type inbox interface {
	Insert(ctx context.Context, e *paymentstore.WebhookEvent) error
	StopReplay(ctx context.Context, id uuid.UUID) (bool, error)
}

func main() {
	fs := flag.NewFlagSet("replaypayment", flag.ExitOnError)
	dsn := fs.String("db-dsn", os.Getenv("DATABASE_URL"), "PostgreSQL DSN (defaults to DATABASE_URL)")
	stopping := fs.Bool("stop", false, "close a replay that keeps failing; the argument is its event id")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: replaypayment [-db-dsn=<dsn>] <mp-payment-id>")
		fmt.Fprintln(fs.Output(), "       replaypayment [-db-dsn=<dsn>] -stop <event-id>")
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

	store := &paymentstore.WebhookEvents{DB: data.NewDB(pool)}
	if *stopping {
		if err := stop(ctx, store, fs.Arg(0)); err != nil {
			pool.Close()
			cancel()
			log.Fatalf("replaypayment: %v", err)
		}
		fmt.Printf("stopped replay %s; it will not be retried again\n", fs.Arg(0))
		return
	}
	event, err := replay(ctx, store, fs.Arg(0))
	if err != nil {
		pool.Close()
		cancel()
		log.Fatalf("replaypayment: %v", err)
	}
	report(os.Stdout, event)
}

// report tells the operator what was queued and how to read what happens to
// it. Its one non-obvious line is the most important: 'processed' is not
// 'confirmed'.
func report(w io.Writer, event *paymentstore.WebhookEvent) {
	fmt.Fprintf(w, "queued MercadoPago payment %s for processing as webhook event %s\n", event.ExternalID, event.ID)
	fmt.Fprintln(w, "the API's webhook sweep works it within a few minutes; its state is:")
	fmt.Fprintf(w, "  SELECT status, retry_count, last_error FROM webhook_events WHERE id = '%s';\n", event.ID)
	fmt.Fprintln(w, "status 'processed' means the processing ran; it does not mean the payment was confirmed.")
	fmt.Fprintln(w, "a collector or amount refusal also ends 'processed': check Sentry for this payment id")
	fmt.Fprintln(w, "(COLLECTOR UNVERIFIED, FRAUD ALERT) and look at the booking or registration it pays for.")
	fmt.Fprintln(w, "if it keeps failing (last_error 404: MercadoPago does not know this payment id), stop it with:")
	fmt.Fprintf(w, "  go run ./cmd/replaypayment -stop %s\n", event.ID)
}

// stop closes the replay event eventID names, refusing anything but an event
// id and saying so when there was no open replay to close.
func stop(ctx context.Context, store inbox, eventID string) error {
	id, err := uuid.Parse(eventID)
	if err != nil {
		return fmt.Errorf("%q is not a webhook event id (the id the replay printed)", eventID)
	}
	stopped, err := store.StopReplay(ctx, id)
	if err != nil {
		return err
	}
	if !stopped {
		return fmt.Errorf("event %s is not an open replay: already processed or stopped, or not written by this command", id)
	}
	return nil
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
