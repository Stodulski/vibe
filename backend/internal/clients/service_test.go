package clients

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	"github.com/stodulski/vibe-server/internal/data"
)

// TestServiceGet covers the tenant rule this module exists to enforce: a client
// record belongs to exactly one complex, and one under another complex is
// reported as missing rather than as forbidden, so the endpoint cannot be used
// to probe which client ids exist elsewhere.
func TestServiceGet(t *testing.T) {
	complexID := uuid.New()
	clientID := uuid.New()
	boom := errors.New("store is down")

	tests := []struct {
		name         string
		client       *clientstore.Client
		getErr       error
		bookings     []*bookingstore.Booking
		bookingsErr  error
		wantErr      error
		wantBookings int
	}{
		{
			name:         "a client of this complex is returned with their recent bookings",
			client:       &clientstore.Client{ID: clientID, ComplexID: complexID},
			bookings:     []*bookingstore.Booking{{ID: uuid.New()}, {ID: uuid.New()}},
			wantBookings: 2,
		},
		{
			name:    "a client of another complex is reported as missing",
			client:  &clientstore.Client{ID: clientID, ComplexID: uuid.New()},
			wantErr: data.ErrRecordNotFound,
		},
		{
			name:    "a client that does not exist is missing too",
			getErr:  data.ErrRecordNotFound,
			wantErr: data.ErrRecordNotFound,
		},
		{
			name:    "a store failure is passed through, not swallowed",
			getErr:  boom,
			wantErr: boom,
		},
		{
			name:        "a booking-side failure is passed through",
			client:      &clientstore.Client{ID: clientID, ComplexID: complexID},
			bookingsErr: boom,
			wantErr:     boom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(
				&stubStore{client: tt.client, getErr: tt.getErr},
				&stubBookings{bookings: tt.bookings, err: tt.bookingsErr},
			)

			client, bookings, err := svc.Get(t.Context(), complexID, clientID)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("got error %v, want %v", err, tt.wantErr)
				}
				if client != nil {
					t.Error("a refused read still handed back a client")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if client == nil || client.ID != clientID {
				t.Fatalf("got client %v, want %v", client, clientID)
			}
			if len(bookings) != tt.wantBookings {
				t.Errorf("got %d recent bookings, want %d", len(bookings), tt.wantBookings)
			}
		})
	}
}
