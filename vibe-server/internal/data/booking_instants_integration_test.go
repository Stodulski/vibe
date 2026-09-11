//go:build integration

package data

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/timezone"
)

// The owner's booking list is serialized straight from *Booking — the handler
// puts the slice in an envelope and nothing reshapes it — so the JSON tags on
// that struct are the API. Since bookings.end_time was dropped the hours reach the client as
// starts_at and ends_at, and this is the test that they are the span the
// database generated rather than anything Go recomputed.
//
// The distinction is the whole point of the change. A booking's end used to be
// stored as a clock reading produced by modular arithmetic, so a 23:00 booking
// of two hours was published as "01:00" — a time of day smaller than its own
// start, on a day nobody named. lower(span) and upper(span) are the values the
// exclusion constraint and the availability grid already agree on; reading them
// back off the row and comparing is what says the wire carries those and not a
// second derivation that has to stay in step by hand.
func TestTheOwnerListCarriesTheSpansOwnInstants(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()

	// Two bookings on one court: one ordinary, one crossing midnight. The
	// second is the row every clock-reading representation was wrong about,
	// and it must be on the same page as the first.
	overnight := f.createBooking(t, bookingOptions{StartTime: "23:00", EndTime: "01:00"})
	ordinary := f.createBooking(t, bookingOptions{StartTime: "09:00", EndTime: "10:30"})

	from := timezone.Day(overnight.Date).AddDate(0, 0, -1)
	listed, _, err := f.Models.Bookings.GetByComplex(ctx, f.ComplexID, from, from.AddDate(0, 0, 3), Filters{Limit: 50})
	if err != nil {
		t.Fatalf("listing the complex's bookings: %v", err)
	}

	byID := map[string]*Booking{}
	for _, b := range listed {
		byID[b.ID.String()] = b
	}

	for _, want := range []*Booking{overnight, ordinary} {
		got, ok := byID[want.ID.String()]
		if !ok {
			t.Fatalf("booking %s starting at %s is missing from the owner list", want.ID, want.StartTime)
		}

		// The row's own answer, asked of the generated column rather than of
		// any Go code. A test that compared the DTO against another Go
		// derivation of the same arithmetic would agree with itself.
		var lower, upper time.Time
		if err := f.Pool.QueryRow(ctx,
			`SELECT lower(span), upper(span) FROM bookings WHERE id = $1`, got.ID,
		).Scan(&lower, &upper); err != nil {
			t.Fatalf("reading the span of %s: %v", got.ID, err)
		}

		if !got.StartsAt.Equal(lower) {
			t.Errorf("booking at %s: starts_at = %s, but lower(span) = %s",
				got.StartTime, got.StartsAt.Format(time.RFC3339), lower.Format(time.RFC3339))
		}
		if !got.EndsAt.Equal(upper) {
			t.Errorf("booking at %s: ends_at = %s, but upper(span) = %s. These have to be the same value: "+
				"the span is what the exclusion constraint and the availability grid read, and a second "+
				"derivation on the wire is a second thing to keep in step.",
				got.StartTime, got.EndsAt.Format(time.RFC3339), upper.Format(time.RFC3339))
		}
	}

	// And the shape on the wire. json.Marshal here is exactly what the handler
	// does with the same struct.
	payload, err := json.Marshal(byID[overnight.ID.String()])
	if err != nil {
		t.Fatalf("marshalling the booking: %v", err)
	}
	var wire map[string]any
	if err := json.Unmarshal(payload, &wire); err != nil {
		t.Fatalf("decoding the marshalled booking: %v", err)
	}

	if _, present := wire["end_time"]; present {
		t.Errorf("end_time is still on the owner payload; the column it came from no longer exists")
	}

	endsAt, err := time.Parse(time.RFC3339, wire["ends_at"].(string))
	if err != nil {
		t.Fatalf("ends_at must be an RFC3339 instant; got %v (%v)", wire["ends_at"], err)
	}
	wantDay := timezone.Day(overnight.Date).AddDate(0, 0, 1).Format("2006-01-02")
	if got := endsAt.In(timezone.Argentina).Format("2006-01-02"); got != wantDay {
		t.Errorf("a 23:00 booking of two hours ends on %s, not %s", wantDay, got)
	}

	// The offset the client reads is the venue's. Marshalled in UTC the same
	// instant prints "04:00" on the next day, which is correct and unreadable:
	// the person looking at it is standing at the court.
	if _, offset := endsAt.Zone(); offset != argentinaOffset(endsAt) {
		t.Errorf("ends_at is serialized on the wrong clock: offset %d, want %d", offset, argentinaOffset(endsAt))
	}
}

// argentinaOffset reads the venue clock's offset at an instant rather than
// writing -10800 into the test: Argentina has had no DST since 2009, but a
// literal is a claim about the future too.
func argentinaOffset(at time.Time) int {
	_, offset := at.In(timezone.Argentina).Zone()
	return offset
}
