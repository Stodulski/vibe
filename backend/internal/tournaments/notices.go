package tournaments

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/booklink"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/events/notice"
	"github.com/stodulski/vibe-server/internal/mailer"
	"github.com/stodulski/vibe-server/internal/notifications"
	tournamentstore "github.com/stodulski/vibe-server/internal/tournaments/store"
)

// The players of a tournament hear of the draw (published or remade) and of
// their own matches (scheduled, moved, about to start). Results and other pairs'
// changes send nothing (A4): a tournament day would bury the players in email.

// recipient is one person of a pair who can be written to.
type recipient struct {
	email, firstName string
}

// playersOf are the people of a pair who left an email: the payer and the
// partner, each under their own name.
func playersOf(p *tournamentstore.PairRow) []recipient {
	var out []recipient
	for _, pl := range []tournamentstore.Player{p.Payer, p.Partner} {
		if pl.Email != nil && *pl.Email != "" {
			out = append(out, recipient{email: *pl.Email, firstName: pl.FirstName})
		}
	}
	return out
}

// playing reports whether a pair is in the category's play: paid and not
// withdrawn.
func playing(p *tournamentstore.PairRow) bool {
	return p.Status == tournamentstore.PairActive && p.RegistrationStatus == eventsConfirmed
}

// pairLabel is how a pair is named to the players across the net.
func pairLabel(p *tournamentstore.PairRow) string {
	return PlayerName(p.Payer) + " / " + PlayerName(p.Partner)
}

// noticeBase is what every notice of a tournament carries: the venue, the
// tournament and the category, and the page where the draw is.
func (s *Service) noticeBase(complex *complexstore.Complex, d *tournamentstore.Detail, cat *tournamentstore.Category, kind string) notifications.TournamentNotice {
	return notifications.TournamentNotice{
		TournamentNotice: mailer.TournamentNotice{
			Kind: kind, ComplexName: complex.Name, TournamentName: d.Tournament.Name, CategoryName: cat.Name,
			PageURL: booklink.TournamentPage(s.publicURL, complex.Slug, d.Tournament.ID),
		},
		ComplexPhone: complex.Phone,
		OwnerID:      complex.OwnerID.String(),
	}
}

// categoryOf is the category of the detail with the id.
func categoryOf(d *tournamentstore.Detail, id uuid.UUID) *tournamentstore.Category {
	for _, c := range d.Categories {
		if c.ID == id {
			return c
		}
	}
	return nil
}

// drawStamp names one draw of a category: the smallest id of its matches. A
// remade draw deletes the matches and writes new ones, so each draw has its own
// stamp, and the same draw told twice has the same.
func drawStamp(d *tournamentstore.Detail, categoryID uuid.UUID) string {
	stamp := ""
	for _, m := range d.Matches {
		if id := m.ID.String(); m.CategoryID == categoryID && (stamp == "" || id < stamp) {
			stamp = id
		}
	}
	return stamp
}

// notifyDraw tells every player of the category's paid pairs that the draw is
// out, or that it was remade. It never fails the draw: a notice the queue
// refused is logged.
func (s *Service) notifyDraw(complex *complexstore.Complex, d *tournamentstore.Detail, categoryID uuid.UUID, remade bool) {
	if s.notify == nil {
		return
	}
	cat := categoryOf(d, categoryID)
	if cat == nil {
		return
	}
	kind := mailer.TournamentDrawPublished
	if remade {
		kind = mailer.TournamentDrawRemade
	}
	subject := "draw:" + categoryID.String() + ":" + drawStamp(d, categoryID)
	for _, p := range d.Pairs {
		if p.CategoryID != categoryID || !playing(p) {
			continue
		}
		for _, pl := range playersOf(p) {
			n := s.noticeBase(complex, d, cat, kind)
			n.Email, n.FirstName, n.Subject = pl.email, pl.firstName, subject
			if err := s.notify.TournamentNoticeDue(n); err != nil {
				s.logger.Error("tournament draw: failed to queue a player's notice", "error", err, "category_id", categoryID)
			}
		}
	}
}

// notifyMatch tells the players of a scheduled match about it: its court, its
// time and the pair across the net, or that the other side is still to be
// defined. It returns the first notice the queue refused, so the reminder sweep
// can leave the match unmarked.
func (s *Service) notifyMatch(ctx context.Context, complex *complexstore.Complex, d *tournamentstore.Detail, m *tournamentstore.Match, kind string, now time.Time) error {
	if s.notify == nil || !m.Scheduled() || m.ScheduledOn == nil || m.StartTime == nil {
		return nil
	}
	cat := categoryOf(d, m.CategoryID)
	if cat == nil {
		return nil
	}
	pairs := map[uuid.UUID]*tournamentstore.PairRow{}
	for _, p := range d.Pairs {
		pairs[p.ID] = p
	}
	side := func(id *uuid.UUID) *tournamentstore.PairRow {
		if id == nil {
			return nil
		}
		if p, ok := pairs[*id]; ok && playing(p) {
			return p
		}
		return nil
	}
	one, two := side(m.Pair1ID), side(m.Pair2ID)

	start := m.StartsAt()
	prefix := "match:"
	if kind == mailer.TournamentMatchReminder {
		prefix = "reminder:"
	}
	subject := prefix + m.ID.String() + ":" + start.UTC().Format(time.RFC3339) + ":" + m.CourtID.String()
	court := s.courtName(ctx, complex.ID, *m.CourtID)

	var errs []error
	for _, sides := range [][2]*tournamentstore.PairRow{{one, two}, {two, one}} {
		mine, theirs := sides[0], sides[1]
		if mine == nil {
			continue
		}
		opponent := mailer.TournamentOpponentUnknown
		if theirs != nil {
			opponent = pairLabel(theirs)
		}
		for _, pl := range playersOf(mine) {
			n := s.noticeBase(complex, d, cat, kind)
			n.Email, n.FirstName, n.Subject = pl.email, pl.firstName, subject
			n.When, n.Court, n.Opponent = notice.When(start, true, now), court, opponent
			n.Address = booklink.Address(complex.Address, complex.City)
			n.MapsURL = booklink.MapsURL(complex.Name, complex.Address, complex.City, complex.Latitude, complex.Longitude)
			if err := s.notify.TournamentNoticeDue(n); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// courtName is a court's name as the players read it, or nothing when the
// courts cannot be read: the time and the opponent still go out.
func (s *Service) courtName(ctx context.Context, complexID, courtID uuid.UUID) string {
	courts, err := s.courts.GetByComplex(ctx, complexID)
	if err != nil {
		s.logger.Warn("tournament notice: cannot read the courts, sending without the court's name", "error", err)
		return ""
	}
	for _, c := range courts {
		if c.ID == courtID {
			return c.Name
		}
	}
	return ""
}

// sameSlot reports whether a match was already scheduled on the court and at
// the time it has now, so scheduling it again tells nobody anything new.
func sameSlot(before, after *tournamentstore.Match) bool {
	if !before.Scheduled() || !after.Scheduled() || before.StartTime == nil || after.StartTime == nil ||
		before.ScheduledOn == nil || after.ScheduledOn == nil {
		return false
	}
	return *before.CourtID == *after.CourtID && before.StartsAt().Equal(after.StartsAt())
}

// RemindMatches queues the two-hour reminder of every scheduled match about to
// start, to its players, as the booking reminder does: queued first and marked
// second, so a refused enqueue leaves the match for the next tick and a repeat
// after a failed mark is absorbed by the dedup key. Called by the scheduler,
// under the tenant bypass.
func (s *Service) RemindMatches(ctx context.Context) {
	now := time.Now()
	due, err := s.store.DueMatchReminders(ctx, now)
	if err != nil {
		s.logger.Error("match reminders: failed to read the due matches", "error", err)
		return
	}
	sent := 0
	for _, dm := range due {
		if s.remindMatch(ctx, dm, now) {
			sent++
		}
	}
	s.logger.Info("match reminders: completed", "candidates", len(due), "sent", sent)
}

// remindMatch queues one match's reminders and marks it, reporting whether it did.
func (s *Service) remindMatch(ctx context.Context, dm tournamentstore.DueMatch, now time.Time) bool {
	complex, err := s.complexes.GetByID(ctx, dm.ComplexID)
	if err != nil {
		s.logger.Error("match reminders: cannot read the complex", "error", err, "match_id", dm.MatchID)
		return false
	}
	d, err := s.store.Detail(ctx, dm.TournamentID)
	if err != nil {
		s.logger.Error("match reminders: cannot read the tournament", "error", err, "match_id", dm.MatchID)
		return false
	}
	var m *tournamentstore.Match
	for _, x := range d.Matches {
		if x.ID == dm.MatchID {
			m = x
		}
	}
	if m == nil {
		return false
	}
	if err := s.notifyMatch(ctx, complex, d, m, mailer.TournamentMatchReminder, now); err != nil {
		s.logger.Error("match reminders: failed to queue, not marked; the next tick retries", "error", err, "match_id", dm.MatchID)
		return false
	}
	if err := s.store.MarkMatchReminded(ctx, dm.MatchID); err != nil {
		s.logger.Error("match reminders: failed to mark sent; the dedup key absorbs the repeat", "error", err, "match_id", dm.MatchID)
	}
	return true
}
