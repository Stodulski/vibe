package bookings

import (
	"context"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/waitlist"
)

// SlotFreedNotifier hears about a slot that a cancellation or an expiry put back
// on sale. waitlist.Service is the production implementation.
type SlotFreedNotifier interface {
	SlotFreed(ctx context.Context, freed waitlist.Freed)
}

// freeWaitlistSlot hands a freed slot to the waitlist on its own goroutine, so
// the request or sweep that freed it does not wait on the emails. The context
// is detached from the request, which ends when the response is written. It is
// a no-op when no waitlist is wired.
func (s *Service) freeWaitlistSlot(ctx context.Context, freed waitlist.Freed) {
	if s.waitlist == nil {
		return
	}
	ctx = context.WithoutCancel(ctx)
	s.run(func() { s.waitlist.SlotFreed(ctx, freed) })
}

// FreedSlotListener hears about a booked slot a cancellation put back on sale.
// complexes.InstagramStories is the production implementation: it queues the
// complex's freed-slot story. It must not fail or block: it runs after the
// cancellation is committed, on its own goroutine, and logs its own errors.
type FreedSlotListener interface {
	SlotCancelled(ctx context.Context, complexID, courtID uuid.UUID, slug string, startsAt, endsAt time.Time)
}

// announceFreedSlot hands a cancelled booking's slot to the freed-slot
// listener on its own goroutine, detached from the request. It is a no-op when
// no listener is wired, and when the booking was not confirmed before the
// cancellation: a pending booking is a storefront hold awaiting payment, so
// its slot never went off sale and nothing was freed. previousStatus is the
// booking's status before the cancellation, which the caller reads before it
// overwrites b.Status: by the time the slot is announced the booking reads
// cancelled whatever it was.
func (s *Service) announceFreedSlot(ctx context.Context, complex *complexstore.Complex, b *bookingstore.Booking, previousStatus bookingstore.BookingStatus) {
	if s.freedSlots == nil || previousStatus != bookingstore.BookingStatusConfirmed {
		return
	}
	ctx = context.WithoutCancel(ctx)
	complexID, courtID, slug, startsAt, endsAt := complex.ID, b.CourtID, complex.Slug, b.StartsAt, b.EndsAt
	s.run(func() { s.freedSlots.SlotCancelled(ctx, complexID, courtID, slug, startsAt, endsAt) })
}
