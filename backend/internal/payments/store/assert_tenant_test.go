package store

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/data"
)

// Money is the write path where "the HTTP chain already checked" is least
// worth relying on: the confirmation flows are reached from a webhook and from
// a cron as well as from a request, and a caller that declared no tenant is
// authorized by nothing.
func TestThePaymentWritesAssertTheirTenant(t *testing.T) {
	own, other := uuid.New(), uuid.New()
	store := &Payments{}

	p := &Payment{ComplexID: own}
	b := &bookingstore.Booking{ComplexID: own}

	asOther := data.ContextWithTenant(context.Background(), other)
	unscoped := context.Background()

	cases := map[string]error{
		"Insert for another tenant":                     store.Insert(asOther, p),
		"Insert with no tenant":                         store.Insert(unscoped, p),
		"Update for another tenant":                     store.Update(asOther, p),
		"InsertAndConfirmBooking for another tenant":    store.InsertAndConfirmBooking(asOther, p, b),
		"ConfirmWebhookPayment for another tenant":      store.ConfirmWebhookPayment(asOther, p, b),
		"InsertAndConfirmBooking with no tenant at all": store.InsertAndConfirmBooking(unscoped, p, b),
	}

	for name, err := range cases {
		if !errors.Is(err, data.ErrRecordNotFound) {
			t.Errorf("%s: want ErrRecordNotFound; got %v", name, err)
		}
	}
}

// A confirmation whose payment and booking disagree about their tenant is
// refused even when the context matches one of them: both are written, so both
// have to belong to the caller.
func TestAConfirmationRefusesAMismatchedBooking(t *testing.T) {
	own, other := uuid.New(), uuid.New()
	store := &Payments{}

	ctx := data.ContextWithTenant(context.Background(), own)
	p := &Payment{ComplexID: own}
	b := &bookingstore.Booking{ComplexID: other}

	if err := store.InsertAndConfirmBooking(ctx, p, b); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("want ErrRecordNotFound; got %v", err)
	}
}
