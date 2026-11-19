package waitlist

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/booklink"
	"github.com/stodulski/vibe-server/internal/courts"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/notifications"
	"github.com/stodulski/vibe-server/internal/slots"
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

// sweepBatch bounds one sweep run: how many event entries it claims, and how
// many slot grids (complex, day, duration) it reads. The next run, a minute
// later, picks up the rest: event claims skip what was already claimed, and the
// grid walk resumes after the last grid read, so no entry is starved.
const sweepBatch = 500

// MaxWaitingPerEmail caps the places one address can be waiting for at one
// complex at a time, so the join cannot be used to fill someone's inbox.
const MaxWaitingPerEmail = 20

// The join's refusals, each answered as a 422 naming the field it is about.
var (
	// ErrPlaceFree is a join for a place that has a free spot right now: it is
	// to be booked, not waited for, and a wait stored for it would be emailed at
	// once by the next sweep.
	ErrPlaceFree = errors.New("waitlist: the place is free now")
	// ErrNotOnGrid is a slot the court's grid does not sell on that day for that
	// duration, so it can never free.
	ErrNotOnGrid = errors.New("waitlist: the slot is not on the court's grid")
	// ErrPlaceStarted is a slot that has already begun.
	ErrPlaceStarted = errors.New("waitlist: the slot has already started")
	// ErrTooManyWaits is an address already waiting for MaxWaitingPerEmail places
	// at the complex.
	ErrTooManyWaits = errors.New("waitlist: the address is waiting for too many places")
)

// PlaceStore is the persistence the place waitlist needs. See store.PlaceStore.
type PlaceStore interface {
	WaitlistableEvent(ctx context.Context, complexID, eventID uuid.UUID, now time.Time) (int, error)
	WaitlistableCategoryEvent(ctx context.Context, complexID, categoryID uuid.UUID, now time.Time) (uuid.UUID, int, error)
	WaitlistableCourt(ctx context.Context, complexID, courtID uuid.UUID) error
	CountWaiting(ctx context.Context, complexID uuid.UUID, email string) (int, error)
	InsertEvent(ctx context.Context, complexID, eventID uuid.UUID, email string) error
	InsertSlot(ctx context.Context, e store.NewSlotEntry) error
	ClaimFreedEvents(ctx context.Context, now time.Time, limit int) ([]store.FreedEventEntry, error)
	ListWaitingGrids(ctx context.Context, fromDay time.Time, after *store.SlotGrid, limit int) ([]store.SlotGrid, error)
	ClaimFreeSlots(ctx context.Context, grid store.SlotGrid, positions []string) ([]store.FreedSlotEntry, error)
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

	// gridBatch is how many slot grids one sweep reads (sweepBatch; a test
	// lowers it). gridCursor is the last grid the previous run read, nil to
	// start from the first; the walk wraps round when a page comes back short.
	gridBatch  int
	gridMu     sync.Mutex
	gridCursor *store.SlotGrid
}

// NewPlaceService returns a PlaceService. publicURL is the storefront base the
// email links to.
func NewPlaceService(st PlaceStore, complexes ComplexLookup, grid SlotGrid, notify PlaceNotifier, publicURL string, logger *slog.Logger) *PlaceService {
	return &PlaceService{
		store: st, complexes: complexes, grid: grid, notify: notify,
		publicURL: publicURL, logger: logger, now: timezone.Now, gridBatch: sweepBatch,
	}
}

// Join puts email on the waitlist for target at the complex with the given
// slug. Joining the same place twice while waiting is not an error. An unknown
// or inactive complex, and an event, category or court that is not that
// complex's or can no longer be waited for, answer data.ErrRecordNotFound. A
// place with a free spot now answers ErrPlaceFree, a slot the grid does not sell
// ErrNotOnGrid, a slot already begun ErrPlaceStarted, and an address already
// waiting for MaxWaitingPerEmail places there ErrTooManyWaits.
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

	waiting, err := s.store.CountWaiting(ctx, complex.ID, email)
	if err != nil {
		return err
	}
	if waiting >= MaxWaitingPerEmail {
		return ErrTooManyWaits
	}

	switch target.Kind {
	case KindOpenPlay:
		spots, err := s.store.WaitlistableEvent(ctx, complex.ID, target.EventID, now)
		if err != nil {
			return err
		}
		if spots > 0 {
			return ErrPlaceFree
		}
		return s.store.InsertEvent(ctx, complex.ID, target.EventID, email)
	case KindTournamentCategory:
		eventID, spots, err := s.store.WaitlistableCategoryEvent(ctx, complex.ID, target.CategoryID, now)
		if err != nil {
			return err
		}
		if spots > 0 {
			return ErrPlaceFree
		}
		return s.store.InsertEvent(ctx, complex.ID, eventID, email)
	case KindSlot:
		if err := s.store.WaitlistableCourt(ctx, complex.ID, target.CourtID); err != nil {
			return err
		}
		if err := s.slotIsTaken(ctx, slug, target, now); err != nil {
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

// slotIsTaken reads the court's grid for the slot's day and duration, the one
// the storefront shows and the sweep reads, and answers nil only for a slot the
// grid sells, that has not begun, and that is taken now.
func (s *PlaceService) slotIsTaken(ctx context.Context, slug string, target PlaceTarget, now time.Time) error {
	dateStr := target.Day.Format(time.DateOnly)
	grid, err := s.grid.Availability(ctx, slug, dateStr, target.Day, target.DurationMinutes)
	if err != nil {
		return err
	}
	for _, c := range grid.Courts {
		if c.CourtID != target.CourtID.String() {
			continue
		}
		for _, sl := range c.Slots {
			if sl.StartTime != target.StartTime {
				continue
			}
			// Placed on the calendar day it falls on: the 00:30 of a window that
			// trades past midnight is the next day's.
			start := slots.At(target.Day.AddDate(0, 0, slots.CalendarDays(sl.StartMin)), sl.StartTime)
			switch {
			case !start.After(now):
				return ErrPlaceStarted
			case sl.Available:
				return ErrPlaceFree
			default:
				return nil
			}
		}
	}
	return ErrNotOnGrid
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

// sweepSlots reads the next page of the grids slot entries wait on, one
// Availability read each, and claims the entries whose position the grid sells
// now. The page starts after the grid the previous run read last and wraps to
// the first when it comes back short, so every waiting entry is inspected in
// turn however many older ones are still taken.
func (s *PlaceService) sweepSlots(ctx context.Context, now time.Time) {
	s.gridMu.Lock()
	defer s.gridMu.Unlock()

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, timezone.Argentina)
	// From yesterday: a venue trading past midnight files tonight's 00:30 under
	// yesterday's grid.
	grids, err := s.store.ListWaitingGrids(ctx, today.AddDate(0, 0, -1), s.gridCursor, s.gridBatch)
	if err != nil {
		s.logger.Error("waitlist: could not list the grids slot entries wait on", "error", err)
		return
	}
	if len(grids) < s.gridBatch {
		s.gridCursor = nil
	} else {
		last := grids[len(grids)-1]
		s.gridCursor = &last
	}

	for _, g := range grids {
		positions := s.freePositions(ctx, g)
		if len(positions) == 0 {
			continue
		}
		claimed, err := s.store.ClaimFreeSlots(ctx, g, positions)
		if err != nil {
			s.logger.Error("waitlist: could not claim the slot entries of a freed grid", "error", err,
				"complex_id", g.ComplexID, "date", g.Day.Format(time.DateOnly))
			continue
		}
		for _, e := range claimed {
			day := timezone.Day(e.Day)
			s.notify.WaitlistPlaceFreed(notifications.WaitlistPlaceFreedEmail{
				EntryID: e.ID.String(), Email: e.Email, ComplexName: e.ComplexName,
				PlaceLabel: fmt.Sprintf("%s el %s %s a las %s", courtLabel(e.CourtName),
					notifications.SpanishWeekday(day.Weekday()), day.Format("02/01"), e.StartTime),
				BookURL: booklink.StorefrontSlot(s.publicURL, e.ComplexSlug, day, e.CourtSport,
					g.DurationMinutes, e.StartTime),
			})
		}
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

// freePositions reads one grid and returns the court positions on sale now,
// each "<court id> HH:MM". The grid already leaves out what is booked, held,
// blocked or begun. A grid that cannot be read frees nothing this run.
func (s *PlaceService) freePositions(ctx context.Context, g store.SlotGrid) []string {
	dateStr := g.Day.Format(time.DateOnly)
	day, err := timezone.ParseDay(dateStr)
	if err != nil {
		return nil
	}
	grid, err := s.grid.Availability(ctx, g.ComplexSlug, dateStr, day, g.DurationMinutes)
	if err != nil {
		s.logger.Warn("waitlist: could not read the grid for waiting slot entries",
			"error", err, "slug", g.ComplexSlug, "date", dateStr, "duration", g.DurationMinutes)
		return nil
	}
	var out []string
	for _, c := range grid.Courts {
		for _, sl := range c.Slots {
			if sl.Available {
				out = append(out, c.CourtID+" "+sl.StartTime)
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
