package clients

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	"github.com/stodulski/vibe-server/internal/data"
)

// ErrEditConflict reports that a client row moved out from under a read: the
// row the update aimed at was gone by the time it ran. It is distinct from
// data.ErrRecordNotFound, which means the client was never this complex's to
// begin with, because the two answer the caller differently.
var ErrEditConflict = errors.New("client changed before the update")

// Service holds this module's rules. The important one is the tenant check: a
// client record belongs to exactly one complex, and a record under another
// complex is reported as missing rather than forbidden, so the endpoints cannot
// be used to probe which client ids exist elsewhere.
type Service struct {
	store    Store
	bookings BookingReader
}

// NewService returns a Service backed by the given stores.
func NewService(store Store, bookings BookingReader) *Service {
	return &Service{store: store, bookings: bookings}
}

// owned resolves a client and confirms it belongs to the given complex.
//
// A client under another complex answers the same data.ErrRecordNotFound one
// that does not exist answers: telling the caller it exists but is not theirs
// would confirm the id.
func (s *Service) owned(ctx context.Context, complexID, clientID uuid.UUID) (*clientstore.Client, error) {
	client, err := s.store.GetByID(ctx, clientID)
	if err != nil {
		return nil, err
	}
	if client.ComplexID != complexID {
		return nil, data.ErrRecordNotFound
	}
	return client, nil
}

// Get returns one client of this complex together with their recent bookings.
func (s *Service) Get(ctx context.Context, complexID, clientID uuid.UUID) (*clientstore.Client, []*bookingstore.Booking, error) {
	client, err := s.owned(ctx, complexID, clientID)
	if err != nil {
		return nil, nil, err
	}

	recentBookings, err := s.bookings.GetByClient(ctx, complexID, client.ID, recentBookingLimit)
	if err != nil {
		return nil, nil, err
	}

	return client, recentBookings, nil
}

// UpdateInput is a validated partial update. Both fields are pointers so that
// omitting one leaves it untouched rather than clearing it.
type UpdateInput struct {
	Notes     *string
	IsBlocked *bool
}

// Update edits the owner's own annotations on a client of this complex. The
// client's identity fields come from their bookings and are not writable here.
//
// It reports ErrEditConflict when the row moved out from under the read, which
// is the one outcome the caller has to be told apart from a client that was
// never theirs.
func (s *Service) Update(ctx context.Context, complexID, clientID uuid.UUID, in UpdateInput) (*clientstore.Client, error) {
	client, err := s.owned(ctx, complexID, clientID)
	if err != nil {
		return nil, err
	}

	if in.Notes != nil {
		client.Notes = in.Notes
	}
	if in.IsBlocked != nil {
		client.IsBlocked = *in.IsBlocked
	}

	if err := s.store.Update(ctx, client); err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			return nil, ErrEditConflict
		}
		return nil, err
	}

	return client, nil
}

// List returns a page of the complex's clients, optionally filtered by a search
// term.
func (s *Service) List(ctx context.Context, complexID uuid.UUID, search string, filters data.Filters) ([]*clientstore.Client, data.Metadata, error) {
	return s.store.GetByComplex(ctx, complexID, search, filters)
}

// ---------------------------------------------------------------------------
// Cross-domain reads
//
// These proxy the store with its exact signature. They exist so another
// domain's entry point into clients is this service rather than the client
// store, which is what lets a rule added later (authorization, caching) land in
// one place.
// ---------------------------------------------------------------------------

// GetByID returns one client. Exported for bookings and payments.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*clientstore.Client, error) {
	return s.store.GetByID(ctx, id)
}

// GetOrCreate resolves the person a booking is for, creating the record when a
// public booking arrives with no account behind it. Exported for bookings.
//
// allowNameUpdate must be true only from the authenticated owner path and false
// from the public one — see clientstore.Store.GetOrCreate's comment for why an
// unauthenticated caller must never be able to overwrite an existing client's
// name.
func (s *Service) GetOrCreate(ctx context.Context, complexID uuid.UUID, firstName, lastName, phone, email string, allowNameUpdate bool) (*clientstore.Client, error) {
	return s.store.GetOrCreate(ctx, complexID, firstName, lastName, phone, email, allowNameUpdate)
}

// IncrementNoShows records that a client did not turn up. Exported for
// bookings.
func (s *Service) IncrementNoShows(ctx context.Context, clientID uuid.UUID) error {
	return s.store.IncrementNoShows(ctx, clientID)
}

// CountByComplex returns how many clients a complex has. Exported for
// reporting's dashboard.
func (s *Service) CountByComplex(ctx context.Context, complexID uuid.UUID) (int, error) {
	return s.store.CountByComplex(ctx, complexID)
}

// GetInsights returns the aggregates behind the client insights panel. Exported
// for reporting.
func (s *Service) GetInsights(ctx context.Context, complexID uuid.UUID, today time.Time) (*clientstore.ClientInsights, error) {
	return s.store.GetInsights(ctx, complexID, today)
}
