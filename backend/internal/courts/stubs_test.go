package courts

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/timezone"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/slots"
)

type stubStore struct {
	courts       []*courtstore.Court
	court        *courtstore.Court
	prices       []*courtstore.CourtPrice
	blockedSlot  *courtstore.BlockedSlot
	blockedSlots []*courtstore.BlockedSlot

	getErr    error
	insertErr error
	blockErr  error

	inserted       *courtstore.Court
	updated        *courtstore.Court
	softDeleted    *uuid.UUID
	insertedPrices []*courtstore.CourtPrice
	pricesCleared  *uuid.UUID
	// Drive ReplacePrices' failure path: the error the store returns, and the
	// index of the price the database refused (-1 when no single one is at
	// fault). Zero values mean the replacement succeeds.
	replacePricesErr   error
	replaceFailedIndex int
	// Whether SoftDelete refuses because the court still has live bookings.
	// It sits on the store rather than on stubBookings because that is where
	// the real answer moved to — see SoftDelete below.
	hasActiveBookings bool
	// softDeleteErr drives SoftDelete's other two zero-row outcomes — an id
	// nothing owns, or a court already soft-deleted — which hasActiveBookings
	// alone cannot express since the real store no longer conflates either of
	// them with ErrCourtHasActiveBookings. nil means "succeed" (idempotent
	// success covers "already deleted" just as well as "freshly deleted", so
	// tests for that case simply leave this unset).
	softDeleteErr   error
	insertedBlocked *courtstore.BlockedSlot
	deletedBlocked  *uuid.UUID
}

func (s *stubStore) GetByComplex(context.Context, uuid.UUID) ([]*courtstore.Court, error) {
	return s.courts, s.getErr
}

func (s *stubStore) GetByID(context.Context, uuid.UUID) (*courtstore.Court, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.court, nil
}

func (s *stubStore) Insert(_ context.Context, c *courtstore.Court) error {
	if s.insertErr != nil {
		return s.insertErr
	}
	c.ID = uuid.New()
	s.inserted = c
	return nil
}

func (s *stubStore) Update(_ context.Context, c *courtstore.Court, _ *int) error {
	s.updated = c
	return nil
}

// SoftDelete refuses when the court still has live bookings, which is where
// that answer now comes from (H-02): the handler used to ask
// HasActiveBookingsByCourt first and then delete, and a booking committing
// between the two survived on a deleted court. The store asks and acts in one
// statement now, so the stub has to answer the same way — and, as the real one
// does, leave softDeleted unset when it refuses.
func (s *stubStore) SoftDelete(_ context.Context, id uuid.UUID) error {
	if s.hasActiveBookings {
		return courtstore.ErrCourtHasActiveBookings
	}
	if s.softDeleteErr != nil {
		return s.softDeleteErr
	}
	s.softDeleted = &id
	return nil
}

func (s *stubStore) GetPrices(context.Context, uuid.UUID) ([]*courtstore.CourtPrice, error) {
	return s.prices, nil
}

func (s *stubStore) GetPricesByCourtIDs(context.Context, []uuid.UUID) ([]*courtstore.CourtPrice, error) {
	return s.prices, nil
}

func (s *stubStore) InsertPrice(_ context.Context, p *courtstore.CourtPrice) error {
	s.insertedPrices = append(s.insertedPrices, p)
	return nil
}

func (s *stubStore) DeletePricesByCourtID(_ context.Context, courtID uuid.UUID) error {
	s.pricesCleared = &courtID
	return nil
}

// ReplacePrices is the transactional replacement for the delete-then-insert
// pair above (H-07). It records the same two things the old pair recorded
// separately — which court was cleared, and which prices were written — so the
// existing assertions keep meaning what they meant.
//
// replacePricesErr and replaceFailedIndex let a test drive the failure path:
// the real store returns the index of the price the database refused, which is
// what the handler turns into a per-field validation error rather than a 500.
func (s *stubStore) ReplacePrices(_ context.Context, courtID uuid.UUID, prices []*courtstore.CourtPrice, _ *int) (int, error) {
	if s.replacePricesErr != nil {
		return s.replaceFailedIndex, s.replacePricesErr
	}
	s.pricesCleared = &courtID
	s.insertedPrices = append(s.insertedPrices, prices...)
	return -1, nil
}

func (s *stubStore) InsertBlockedSlot(_ context.Context, slot *courtstore.BlockedSlot) error {
	if s.blockErr != nil {
		return s.blockErr
	}
	slot.ID = uuid.New()
	s.insertedBlocked = slot
	return nil
}

func (s *stubStore) GetBlockedSlotByID(context.Context, uuid.UUID) (*courtstore.BlockedSlot, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.blockedSlot, nil
}

func (s *stubStore) GetBlockedSlotsByComplex(context.Context, uuid.UUID, time.Time, time.Time) ([]*courtstore.BlockedSlot, error) {
	return s.blockedSlots, s.getErr
}

func (s *stubStore) GetBlockedSlotsByCourtIDs(context.Context, []uuid.UUID, time.Time) ([]*courtstore.BlockedSlot, error) {
	return s.blockedSlots, nil
}

func (s *stubStore) GetBlockedSlots(context.Context, uuid.UUID, time.Time) ([]*courtstore.BlockedSlot, error) {
	return s.blockedSlots, nil
}

func (s *stubStore) DeleteBlockedSlot(_ context.Context, id uuid.UUID) error {
	s.deletedBlocked = &id
	return nil
}

type stubBookings struct {
	booked    []bookingstore.BookedSpan
	hasActive bool
	err       error
}

func (b *stubBookings) GetBookedSlotsByCourtIDs(context.Context, []uuid.UUID, time.Time) ([]bookingstore.BookedSpan, error) {
	return b.booked, b.err
}

func (b *stubBookings) HasActiveBookingsByCourt(context.Context, uuid.UUID) (bool, error) {
	return b.hasActive, b.err
}

type stubComplexes struct {
	complex   *complexstore.Complex
	schedules []*complexstore.Schedule
	err       error
}

func (c *stubComplexes) GetBySlug(context.Context, string) (*complexstore.Complex, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.complex, nil
}

func (c *stubComplexes) GetSchedules(context.Context, uuid.UUID) ([]*complexstore.Schedule, error) {
	return c.schedules, nil
}

type stubRecorder struct{ entries []audit.Entry }

func (r *stubRecorder) Record(e audit.Entry) { r.entries = append(r.entries, e) }

// auditRow is one persisted audit entry, as bytes.
type auditRow struct {
	action, entityType string
	oldJSON, newJSON   []byte
}

// stubAuditStore stands in for the audit table.
type stubAuditStore struct{ rows []auditRow }

func (s *stubAuditStore) InsertAuditLog(_ context.Context, _, _ *uuid.UUID, action, entityType string, _ *uuid.UUID, oldJSON, newJSON []byte, _ string) error {
	s.rows = append(s.rows, auditRow{action: action, entityType: entityType, oldJSON: oldJSON, newJSON: newJSON})
	return nil
}

// newTestHandlerWithAuditTrail wires the handler to the real audit.Recorder over
// a stub table, and returns that table plus a func that runs the writes the
// recorder scheduled.
//
// stubRecorder cannot answer questions about entry *content*: it keeps the
// caller's live pointer, so by the time a test reads it the handler has finished
// and every later mutation is already visible. Anything asking what the audit
// row actually says has to go through the real recorder, which encodes when
// Record is called. The writes are deferred rather than inline for the same
// reason — running them inline would encode at Record time no matter where the
// encoding lives, and hide the very ordering under test.
func newTestHandlerWithAuditTrail(store *stubStore, bookings *stubBookings, complexes *stubComplexes) (*Handler, *stubAuditStore, func()) {
	table := &stubAuditStore{}

	var scheduled []func()
	deferRun := func(fn func()) { scheduled = append(scheduled, fn) }

	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	rec := audit.NewRecorder(table, logger, deferRun)

	h := NewHandler(NewService(store, bookings, complexes, rec), httpx.NewResponder(logger), false)
	return h, table, func() {
		for _, fn := range scheduled {
			fn()
		}
	}
}

func newTestHandler(store *stubStore, bookings *stubBookings, complexes *stubComplexes) (*Handler, *stubRecorder) {
	rec := &stubRecorder{}
	responder := httpx.NewResponder(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	return NewHandler(NewService(store, bookings, complexes, rec), responder, false), rec
}

// ownerRequest builds a request from the complex's authenticated owner, with
// the given path parameters bound.
func ownerRequest(t *testing.T, method, target string, complexID uuid.UUID, params map[string]string, body string) *http.Request {
	t.Helper()

	var r *http.Request
	if body == "" {
		r = httptest.NewRequestWithContext(t.Context(), method, target, nil)
	} else {
		r = httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}

	r = httpx.ContextSetUser(r, &authstore.User{ID: uuid.New(), Role: "owner"})
	r = httpx.ContextSetComplex(r, &complexstore.Complex{ID: complexID})

	for k, v := range params {
		r.SetPathValue(k, v)
	}
	return r
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not valid JSON: %v\n%s", err, w.Body.String())
	}
	return body
}

// futureDate returns a date the block-slot handler will accept.
func futureDate() string {
	return time.Now().AddDate(0, 1, 0).Format("2006-01-02")
}

// withSlug binds the {slug} path parameter, as the router does for the public
// availability route.
func withSlug(r *http.Request, slug string) *http.Request {
	r.SetPathValue("slug", slug)
	return r
}

// openEveryDay returns a schedule with the complex open 08:00-22:00 all week,
// so a test does not have to care which weekday its date lands on.
func openEveryDay() []*complexstore.Schedule {
	days := []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}
	out := make([]*complexstore.Schedule, 0, len(days))
	for _, d := range days {
		out = append(out, &complexstore.Schedule{Day: d, OpenTime: "08:00", CloseTime: "22:00"})
	}
	return out
}

// pricedEveryDay returns a full-week price band for one court, for the same
// reason.
func pricedEveryDay(courtID uuid.UUID) []*courtstore.CourtPrice {
	return everyDayBand(courtID, "08:00", "22:00", 500000)
}

// weekdays is the day vocabulary the schedule and price rows are keyed by.
var weekdays = []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}

// everyDaySchedule opens the complex between the same two times all week, so a
// test does not have to care which weekday its date lands on.
func everyDaySchedule(open, closes string) []*complexstore.Schedule {
	out := make([]*complexstore.Schedule, 0, len(weekdays))
	for _, d := range weekdays {
		out = append(out, &complexstore.Schedule{Day: d, OpenTime: open, CloseTime: closes})
	}
	return out
}

// everyDayBand prices one court identically all week, so that what a test is
// actually about — which slots are covered — is the only thing that varies.
func everyDayBand(courtID uuid.UUID, from, to string, price int) []*courtstore.CourtPrice {
	out := make([]*courtstore.CourtPrice, 0, len(weekdays))
	for _, d := range weekdays {
		out = append(out, courtstore.NewCourtPriceForTest(courtID, d, from, to, price))
	}
	return out
}

// nextWeekday returns the next future date falling on the named weekday.
func nextWeekday(t *testing.T, day string) string {
	t.Helper()
	d := time.Now().AddDate(0, 0, 1)
	for range 8 {
		if slots.DayName(d.Weekday()) == day {
			return d.Format("2006-01-02")
		}
		d = d.AddDate(0, 0, 1)
	}
	t.Fatalf("no %s within a week", day)
	return ""
}

// mustParseDate parses a date a test itself produced.
func mustParseDate(t *testing.T, s string) time.Time {
	t.Helper()
	// Anchored the way the handler anchors it (timezone.ParseDay), so a test
	// building an instant from this date compares against the same wall-clock
	// the code under test used. A UTC-parsed date is three hours off, which
	// shows up as a slot that is free in the test and taken in production.
	d, err := timezone.ParseDay(s)
	if err != nil {
		t.Fatalf("test produced an unparseable date %q: %v", s, err)
	}
	return d
}
