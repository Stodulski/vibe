package bookings

import (
	"context"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/slots"
)

// The owner's view of the hours a visitor is booking online right now.
//
// A storefront booking that has not been paid is not a booking the owner has
// (owner decision 2026-11-08, slotguard.UnpaidPublicBooking), and it stays out
// of the owner's list. But it does hold its slot for the payment window, and a
// grid that drew that cell as free sent the owner into a refusal they could not
// explain. So the grid gets the hold as what it is: hours being booked online,
// free again at a given time if nobody pays, with nothing about the visitor.

// liveHoldLister lists a complex's online holds for a day. *bookingstore.Store
// implements it; the Store interface stays as cmd/api and its mocks know it,
// and a store without it simply reports no holds.
type liveHoldLister interface {
	LiveHolds(ctx context.Context, complexID uuid.UUID, day time.Time) ([]bookingstore.LiveHold, error)
}

var _ liveHoldLister = (*bookingstore.Store)(nil)

// LiveHolds lists the online holds occupying a complex's local day.
func (s *Service) LiveHolds(ctx context.Context, complexID uuid.UUID, day time.Time) ([]bookingstore.LiveHold, error) {
	lister, ok := s.store.(liveHoldLister)
	if !ok {
		return nil, nil
	}
	return lister.LiveHolds(ctx, complexID, day)
}

// toOnlineHolds is the wire shape of the day's online holds: where and when,
// and when the hours free up. The end is a clock reading like the fixed and
// event blocks the grid already draws, earlier than the start when the hold
// runs past midnight. Always a list, never null.
func toOnlineHolds(holds []bookingstore.LiveHold) []httpx.Envelope {
	out := make([]httpx.Envelope, 0, len(holds))
	for _, h := range holds {
		out = append(out, httpx.Envelope{
			"court_id":         h.CourtID,
			"date":             h.Date.Format(time.DateOnly),
			"start_time":       h.StartTime,
			"end_time":         slots.Add(h.StartTime, h.DurationMinutes),
			"duration_minutes": h.DurationMinutes,
			"frees_at":         h.FreesAt.UTC().Format(time.RFC3339),
		})
	}
	return out
}
