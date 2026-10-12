// Package waitlist lets a player ask to be emailed when a slot on a fully booked
// day frees up again ("Avisame si se libera"), and sends those emails when a
// booking is cancelled or expires.
package waitlist

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/booklink"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/notifications"
	"github.com/stodulski/vibe-server/internal/timezone"
	"github.com/stodulski/vibe-server/internal/waitlist/store"
)

// Store is the persistence this service needs. See the store package.
type Store interface {
	Insert(ctx context.Context, complexID uuid.UUID, day time.Time, email string) error
	ClaimUnnotified(ctx context.Context, complexID uuid.UUID, day time.Time) ([]store.Claimed, error)
	DeleteBefore(ctx context.Context, today time.Time) (int64, error)
}

// ComplexLookup resolves a public slug to its complex.
type ComplexLookup interface {
	GetBySlug(ctx context.Context, slug string) (*complexstore.Complex, error)
}

// Notifier queues the "a slot was freed" email for one waitlist entry.
type Notifier interface {
	WaitlistSlotFreed(e notifications.WaitlistSlotFreedEmail)
}

// Freed is one slot that opened up on a complex on a day, because a booking on
// it was cancelled or expired.
type Freed struct {
	ComplexID   uuid.UUID
	ComplexName string
	ComplexSlug string
	// Day is the calendar day of the freed slot. Only its date is used.
	Day time.Time
}

// Service stores waitlist entries and notifies them when a slot frees up.
type Service struct {
	store     Store
	complexes ComplexLookup
	notify    Notifier
	publicURL string
	logger    *slog.Logger
}

// NewService returns a Service. publicURL is the storefront base the email
// links to.
func NewService(st Store, complexes ComplexLookup, notify Notifier, publicURL string, logger *slog.Logger) *Service {
	return &Service{store: st, complexes: complexes, notify: notify, publicURL: publicURL, logger: logger}
}

// Join puts email on the waitlist for day at the complex with the given slug.
// Joining the same day twice is not an error. An unknown or deactivated complex
// answers data.ErrRecordNotFound, the same as the other public reads.
func (s *Service) Join(ctx context.Context, slug string, day time.Time, email string) error {
	complex, err := s.complexes.GetBySlug(ctx, slug)
	if err != nil {
		return err
	}
	if !complex.IsActive {
		return data.ErrRecordNotFound
	}

	ctx = data.ContextWithTenant(ctx, complex.ID)
	return s.store.Insert(ctx, complex.ID, day, email)
}

// SlotFreed emails every entry still waiting for the freed day, once each. It is
// called after the cancelling or expiring write has committed. A day that has
// already passed frees nothing anyone can book, so it is skipped. A failed claim
// is logged and sends nothing: the cancellation it follows has already happened.
func (s *Service) SlotFreed(ctx context.Context, freed Freed) {
	day := timezone.Day(freed.Day)
	if day.Before(timezone.Today()) {
		return
	}

	ctx = data.ContextWithTenant(ctx, freed.ComplexID)
	claimed, err := s.store.ClaimUnnotified(ctx, freed.ComplexID, day)
	if err != nil {
		s.logger.Error("waitlist: could not claim the entries waiting for a freed slot",
			"error", err, "complex_id", freed.ComplexID, "date", day.Format("2006-01-02"))
		return
	}

	bookURL := booklink.Storefront(s.publicURL, freed.ComplexSlug)
	for _, entry := range claimed {
		s.notify.WaitlistSlotFreed(notifications.WaitlistSlotFreedEmail{
			EntryID:     entry.ID.String(),
			Email:       entry.Email,
			ComplexName: freed.ComplexName,
			DateLabel:   day.Format("02/01"),
			BookURL:     bookURL,
		})
	}
}

// Purge deletes every entry for a day before today and reports how many it
// removed. It is cross-tenant, so it must run on the bypass context the cron
// wrapper provides.
func (s *Service) Purge(ctx context.Context) (int64, error) {
	return s.store.DeleteBefore(ctx, timezone.Today())
}
