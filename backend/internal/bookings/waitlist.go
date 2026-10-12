package bookings

import (
	"context"

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
