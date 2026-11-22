//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data/datatest"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
)

// lapseRecently makes a hold's window end a minute ago and runs the sweep, so
// it is expired with its first payment check open, as ExpireHolds leaves it.
func lapseRecently(t *testing.T, f *datatest.Fixture, regID uuid.UUID) {
	t.Helper()
	if _, err := f.DB.Exec(context.Background(),
		`UPDATE open_play_registrations SET held_until = NOW() - interval '1 minute' WHERE id = $1`, regID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Stores.Events.ExpireHolds(f.Scoped(context.Background())); err != nil {
		t.Fatal(err)
	}
}

// L19 (7): the spot of a hold that just ran out stays counted while its
// payment check is open, for at most ten minutes after its window ended. The
// first check is due three minutes after the expiry; releasing the spot at
// once let someone else fill the event in that gap, and a payment approved in
// the hold's last seconds was then refunded instead of confirmed.
func TestARecentlyExpiredHoldKeepsItsSpotWhileItsPaymentCheckIsOpen(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	ev := eventWithCapacity(t, f, 1)
	slow, err := signUp(f, ev, "+5491100000031")
	if err != nil {
		t.Fatal(err)
	}
	lapseRecently(t, f, slow.ID)

	if _, err := signUp(f, ev, "+5491100000032"); !errors.Is(err, eventstore.ErrEventFull) {
		t.Fatalf("while the expired hold's payment check is open its spot is taken; got %v", err)
	}
	if got, _ := f.Stores.Events.GetEvent(ctx, ev.ID); got.SpotsLeft() != 0 {
		t.Errorf("the event shows the spot taken too; spots left %d", got.SpotsLeft())
	}
	if err := pay(f, slow, "mp-late-in-grace"); err != nil {
		t.Fatalf("the late payment finds its spot still there: %v", err)
	}
	if got, _ := f.Stores.Events.GetRegistration(ctx, slow.ID); got.Status != eventstore.RegistrationConfirmed {
		t.Fatalf("status = %s, want confirmed", got.Status)
	}
}

// The spot is let go as soon as the check closes (MercadoPago holds no
// payment), or ten minutes after the window ended whatever the check is doing.
func TestAnExpiredHoldFreesItsSpotOnceTheCheckClosesOrTheGraceEnds(t *testing.T) {
	t.Run("the check closed", func(t *testing.T) {
		f := datatest.Isolated(t)
		ev := eventWithCapacity(t, f, 1)
		slow, err := signUp(f, ev, "+5491100000033")
		if err != nil {
			t.Fatal(err)
		}
		lapseRecently(t, f, slow.ID)
		if _, err := f.Stores.Events.ClosePaymentCheck(f.Scoped(context.Background()), slow.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := signUp(f, ev, "+5491100000034"); err != nil {
			t.Fatalf("with the check closed the spot is free: %v", err)
		}
	})
	t.Run("the grace ended", func(t *testing.T) {
		f := datatest.Isolated(t)
		ev := eventWithCapacity(t, f, 1)
		slow, err := signUp(f, ev, "+5491100000035")
		if err != nil {
			t.Fatal(err)
		}
		lapseRecently(t, f, slow.ID)
		if _, err := f.DB.Exec(context.Background(),
			`UPDATE open_play_registrations SET held_until = NOW() - interval '11 minutes' WHERE id = $1`, slow.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := signUp(f, ev, "+5491100000036"); err != nil {
			t.Fatalf("ten minutes after the window the spot is free, check open or not: %v", err)
		}
	})
}

// The person whose hold ran out may sign up again at once: their own expired
// spot is not kept against them (a late payment on it is refused, since they
// hold another spot, and refunded).
func TestThePersonOfARecentlyExpiredHoldCanSignUpAgain(t *testing.T) {
	f := datatest.Isolated(t)
	ev := eventWithCapacity(t, f, 1)
	first, err := signUp(f, ev, "+5491100000037")
	if err != nil {
		t.Fatal(err)
	}
	lapseRecently(t, f, first.ID)

	if _, err := signUp(f, ev, "+5491100000037"); err != nil {
		t.Fatalf("the same person signs up again: %v", err)
	}
}
