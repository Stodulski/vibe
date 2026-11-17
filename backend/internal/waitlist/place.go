package waitlist

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/booklink"
	"github.com/stodulski/vibe-server/internal/courts"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/notifications"
	"github.com/stodulski/vibe-server/internal/timezone"
	"github.com/stodulski/vibe-server/internal/waitlist/store"
)

// The kinds of place a person can wait for, as the join endpoint names them. A
// tournament category is stored as the open-play event that carries its sign-up.
const (
	KindOpenPlay           = "open_play"
	KindTournamentCategory = "tournament_category"
	KindSlot               = "slot"
)

// sweepBatch bounds how many entries one sweep run claims or inspects per kind;
// the next run, a minute later, picks up the rest.
const sweepBatch = 500

// PlaceStore is the persistence the place waitlist needs. See store.PlaceStore.
type PlaceStore interface {
	WaitlistableEvent(ctx context.Context, complexID, eventID uuid.UUID, now time.Time) error
	WaitlistableCategoryEvent(ctx context.Context, complexID, categoryID uuid.UUID, now time.Time) (uuid.UUID, error)
	WaitlistableCourt(ctx context.Context, complexID, courtID uuid.UUID) error
	InsertEvent(ctx context.Context, complexID, eventID uuid.UUID, email string) error
	InsertSlot(ctx context.Context, e store.NewSlotEntry) error
	ClaimFreedEvents(ctx context.Context, now time.Time, limit int) ([]store.FreedEventEntry, error)
	ListWaitingSlots(ctx context.Context, fromDay time.Time, limit int) ([]store.WaitingSlotEntry, error)
	Claim(ctx context.Context, ids []uuid.UUID) ([]uuid.UUID, error)
	DeletePast(ctx context.Context, today time.Time) (int64, error)
}

// SlotGrid is the storefront's bookable grid for one day, the courts service's
// Availability. Whether a slot is free is its call, so the waitlist can never
// tell someone a slot is free that the booking path would refuse.
type SlotGrid interface {
	Availability(ctx context.Context, slug, dateStr string, date time.Time, duration int) (*courts.AvailabilityResponse, error)
}

// PlaceNotifier queues the "a place was freed" email for one entry.
type PlaceNotifier interface {
	WaitlistPlaceFreed(e notifications.WaitlistPlaceFreedEmail)
}

// PlaceTarget is the place a join waits for. Kind decides which fields count:
// EventID for open play, CategoryID for a tournament category, and CourtID, Day,
// StartTime ("HH:MM") and DurationMinutes for a court slot.
type PlaceTarget struct {
	Kind            string
	EventID         uuid.UUID
	CategoryID      uuid.UUID
	CourtID         uuid.UUID
	Day             time.Time
	StartTime       string
	DurationMinutes int
}

// PlaceService stores place-waitlist entries and, from a periodic sweep, emails
// every entry whose place has a free spot, once each.
type PlaceService struct {
	store     PlaceStore
	complexes ComplexLookup
	grid      SlotGrid
	notify    PlaceNotifier
	publicURL string
	logger    *slog.Logger
	now       func() time.Time
}

// NewPlaceService returns a PlaceService. publicURL is the storefront base the
// email links to.
func NewPlaceService(st PlaceStore, complexes ComplexLookup, grid SlotGrid, notify PlaceNotifier, publicURL string, logger *slog.Logger) *PlaceService {
	return &PlaceService{
		store: st, complexes: complexes, grid: grid, notify: notify,
		publicURL: publicURL, logger: logger, now: timezone.Now,
	}
}

// Join puts email on the waitlist for target at the complex with the given
// slug. Joining the same place twice is not an error. An unknown or inactive
// complex, and an event, category or court that is not that complex's or can no
// longer be waited for, answer data.ErrRecordNotFound.
func (s *PlaceService) Join(ctx context.Context, slug string, target PlaceTarget, email string) error {
	complex, err := s.complexes.GetBySlug(ctx, slug)
	if err != nil {
		return err
	}
	if !complex.IsActive {
		return data.ErrRecordNotFound
	}
	ctx = data.ContextWithTenant(ctx, complex.ID)
	now := s.now()

	switch target.Kind {
	case KindOpenPlay:
		if err := s.store.WaitlistableEvent(ctx, complex.ID, target.EventID, now); err != nil {
			return err
		}
		return s.store.InsertEvent(ctx, complex.ID, target.EventID, email)
	case KindTournamentCategory:
		eventID, err := s.store.WaitlistableCategoryEvent(ctx, complex.ID, target.CategoryID, now)
		if err != nil {
			return err
		}
		return s.store.InsertEvent(ctx, complex.ID, eventID, email)
	case KindSlot:
		if err := s.store.WaitlistableCourt(ctx, complex.ID, target.CourtID); err != nil {
			return err
		}
		return s.store.InsertSlot(ctx, store.NewSlotEntry{
			ComplexID: complex.ID, Email: email, CourtID: target.CourtID, Day: target.Day,
			StartTime: target.StartTime, DurationMinutes: target.DurationMinutes,
		})
	default:
		return fmt.Errorf("waitlist: unknown place kind %q", target.Kind)
	}
}

// Sweep emails every entry whose place has a free spot now and has not begun,
// and marks it notified in the same claim, so each entry is emailed once. It is
// cross-tenant, so it runs on the bypass context the cron wrapper provides. A
// failure is logged; the next run tries again.
func (s *PlaceService) Sweep(ctx context.Context) {
	now := s.now()
	s.sweepEvents(ctx, now)
	s.sweepSlots(ctx, now)
}

func (s *PlaceService) sweepEvents(ctx context.Context, now time.Time) {
	claimed, err := s.store.ClaimFreedEvents(ctx, now, sweepBatch)
	if err != nil {
		s.logger.Error("waitlist: could not claim the event entries with a freed spot", "error", err)
		return
	}
	for _, e := range claimed {
		var bookURL, label string
		if e.TournamentID != nil {
			bookURL = booklink.TournamentPage(s.publicURL, e.ComplexSlug, *e.TournamentID)
			label = e.EventName
		} else {
			bookURL = booklink.EventPage(s.publicURL, e.ComplexSlug, e.EventID)
			day := timezone.Day(e.EventDay)
			label = fmt.Sprintf("%s, %s %s %s", e.EventName,
				notifications.SpanishWeekday(day.Weekday()), day.Format("02/01"), e.EventStart)
		}
		s.notify.WaitlistPlaceFreed(notifications.WaitlistPlaceFreedEmail{
			EntryID: e.ID.String(), Email: e.Email, ComplexName: e.ComplexName, PlaceLabel: label, BookURL: bookURL,
		})
	}
}

// gridKey names one day's grid of one venue for one duration: the slot entries
// that share it are checked against a single Availability read.
type gridKey struct {
	slug     string
	day      string
	duration int
}

func (s *PlaceService) sweepSlots(ctx context.Context, now time.Time) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, timezone.Argentina)
	waiting, err := s.store.ListWaitingSlots(ctx, today.AddDate(0, 0, -1), sweepBatch)
	if err != nil {
		s.logger.Error("waitlist: could not list the slot entries still waiting", "error", err)
		return
	}

	free := make(map[gridKey]map[string]bool) // per grid: "courtID start" -> free now
	var freedIDs []uuid.UUID
	byID := make(map[uuid.UUID]store.WaitingSlotEntry, len(waiting))
	for _, e := range waiting {
		key := gridKey{slug: e.ComplexSlug, day: e.Day.Format(time.DateOnly), duration: e.DurationMinutes}
		slotsFree, ok := free[key]
		if !ok {
			slotsFree = s.freeSlots(ctx, key)
			free[key] = slotsFree
		}
		if slotsFree[e.CourtID.String()+" "+e.StartTime] {
			freedIDs = append(freedIDs, e.ID)
			byID[e.ID] = e
		}
	}
	if len(freedIDs) == 0 {
		return
	}

	claimed, err := s.store.Claim(ctx, freedIDs)
	if err != nil {
		s.logger.Error("waitlist: could not claim the slot entries with a freed slot", "error", err)
		return
	}
	for _, id := range claimed {
		e := byID[id]
		day := timezone.Day(e.Day)
		s.notify.WaitlistPlaceFreed(notifications.WaitlistPlaceFreedEmail{
			EntryID: e.ID.String(), Email: e.Email, ComplexName: e.ComplexName,
			PlaceLabel: fmt.Sprintf("%s el %s %s a las %s", courtLabel(e.CourtName),
				notifications.SpanishWeekday(day.Weekday()), day.Format("02/01"), e.StartTime),
			BookURL: booklink.StorefrontOnDate(s.publicURL, e.ComplexSlug, day),
		})
	}
}

// courtLabel names a court the way the email says it: "Cancha 2", without
// doubling the word when the owner already named it "Cancha 2".
func courtLabel(name string) string {
	if strings.HasPrefix(strings.ToLower(name), "cancha") {
		return name
	}
	return "Cancha " + name
}

// freeSlots reads one day's grid and returns the court positions that are on
// sale now, keyed "courtID start". The grid already leaves out what is booked,
// held, blocked or begun. A grid that cannot be read frees nothing this run.
func (s *PlaceService) freeSlots(ctx context.Context, key gridKey) map[string]bool {
	out := map[string]bool{}
	day, err := timezone.ParseDay(key.day)
	if err != nil {
		return out
	}
	grid, err := s.grid.Availability(ctx, key.slug, key.day, day, key.duration)
	if err != nil {
		s.logger.Warn("waitlist: could not read the grid for waiting slot entries",
			"error", err, "slug", key.slug, "date", key.day, "duration", key.duration)
		return out
	}
	for _, c := range grid.Courts {
		for _, sl := range c.Slots {
			if sl.Available {
				out[c.CourtID+" "+sl.StartTime] = true
			}
		}
	}
	return out
}

// Purge deletes the entries whose place has gone (a passed day, a cancelled
// event) and reports how many it removed. It is cross-tenant, so it must run on
// the bypass context the cron wrapper provides.
func (s *PlaceService) Purge(ctx context.Context) (int64, error) {
	return s.store.DeletePast(ctx, timezone.Today())
}
