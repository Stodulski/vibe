package bookings

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/booklink"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/notifications"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	"github.com/stodulski/vibe-server/internal/pricing"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/timezone"
	"github.com/stodulski/vibe-server/internal/validator"
)

// Service holds this module's rules: which hours may be sold and to whom, what
// a cancellation does about the client's money, and how a booking moves through
// its states. Every store call, every audit entry and every notification of the
// booking domain goes through it.
//
// It is also where the scheduled sweeps live — the four booking cron jobs — so
// that a rule is written once whether a person or a timer triggers it.
type Service struct {
	// Facade carries the reads other domains enter this one through. It is
	// embedded rather than re-proxied so there is one implementation of each:
	// a rule added to a cross-domain read lands there and applies whether the
	// caller is another domain, a handler or a sweep.
	//
	// Service.Update shadows Facade.Update deliberately — the owner's editing
	// use case is this domain's own, and the bare write behind it stays
	// reachable as s.Facade.Update.
	*Facade

	store        Store
	clients      ClientStore
	complexes    ComplexReader
	courts       CourtReader
	payments     PaymentStore
	locks        SlotLocker
	checkout     Checkout
	refunds      Refunder
	linkResolver LinkResolver
	linkTokens   LinkTokenStore
	notify       Notifier
	realtime     Broadcaster
	audit        Recorder
	logger       *slog.Logger
	cfg          Config
	run          func(func())
}

// NewService returns a Service backed by the given dependencies.
func NewService(d Dependencies, cfg Config) *Service {
	return &Service{
		Facade:       d.Facade,
		store:        d.Store,
		clients:      d.Clients,
		complexes:    d.Complexes,
		courts:       d.Courts,
		payments:     d.Payments,
		locks:        d.Locks,
		checkout:     d.Checkout,
		refunds:      d.Refunds,
		linkResolver: d.LinkResolver,
		linkTokens:   d.LinkTokens,
		notify:       d.Notify,
		realtime:     d.Realtime,
		audit:        d.Audit,
		logger:       d.Logger,
		cfg:          cfg,
		run:          d.Run,
	}
}

// ---------------------------------------------------------------------------
// The owner's booking list and detail
// ---------------------------------------------------------------------------

// ListInput is a validated request for one complex's bookings on one day.
type ListInput struct {
	Date    time.Time
	Status  string
	Search  string
	Filters data.Filters
}

// List returns a complex's bookings for one day, narrowed by the dashboard's
// status and free-text filters. The filtering is done here rather than in SQL
// because a single day's volume is small.
func (s *Service) List(ctx context.Context, complexID uuid.UUID, in ListInput) ([]*bookingstore.Booking, data.Metadata, error) {
	bookings, metadata, err := s.store.GetByComplex(ctx, complexID, in.Date, in.Date, in.Filters)
	if err != nil {
		return nil, data.Metadata{}, err
	}

	if in.Status == "" && in.Search == "" {
		return bookings, metadata, nil
	}

	searchLower := strings.ToLower(in.Search)
	filtered := make([]*bookingstore.Booking, 0, len(bookings))
	for _, b := range bookings {
		if in.Status != "" && b.Status != in.Status {
			continue
		}
		if in.Search != "" &&
			!strings.Contains(strings.ToLower(b.ClientName), searchLower) &&
			!strings.Contains(b.ClientPhone, in.Search) {
			continue
		}
		filtered = append(filtered, b)
	}
	return filtered, metadata, nil
}

// Detail is one booking with everything the owner's detail view shows: the
// client it is for, the MercadoPago-preferred payment row, and the whole
// ledger behind it.
type Detail struct {
	Booking *bookingstore.Booking
	// Client is nil when the client row is gone.
	Client *clientstore.Client
	// Payment is nil when the booking carries no payment row yet.
	Payment  *paymentstore.Payment
	Payments []*paymentstore.Payment
}

// ownedBooking loads a booking and hides one that belongs to another complex:
// it answers the same ErrRecordNotFound a booking that does not exist answers,
// so a probe cannot tell the two apart.
func (s *Service) ownedBooking(ctx context.Context, complexID, bookingID uuid.UUID) (*bookingstore.Booking, error) {
	booking, err := s.store.GetByID(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	if booking.ComplexID != complexID {
		return nil, data.ErrRecordNotFound
	}
	return booking, nil
}

// Get returns one booking of a complex with its client and its payments.
//
// The whole ledger is read, not just the single MercadoPago-preferred row: a
// booking can carry more than one payment (a deposit paid online plus a balance
// confirmed in cash), and the detail view has to show every one of them.
func (s *Service) Get(ctx context.Context, complexID, bookingID uuid.UUID) (Detail, error) {
	booking, err := s.ownedBooking(ctx, complexID, bookingID)
	if err != nil {
		return Detail{}, err
	}

	client, err := s.clients.GetByID(ctx, booking.ClientID)
	if err != nil && !errors.Is(err, data.ErrRecordNotFound) {
		return Detail{}, err
	}

	payment, err := s.payments.GetByBookingID(ctx, booking.ID)
	if err != nil && !errors.Is(err, data.ErrRecordNotFound) {
		return Detail{}, err
	}

	payments, err := s.payments.ListByBookingID(ctx, booking.ID)
	if err != nil {
		return Detail{}, err
	}
	if payments == nil {
		payments = []*paymentstore.Payment{}
	}

	return Detail{Booking: booking, Client: client, Payment: payment, Payments: payments}, nil
}

// UpdateInput is a request to edit a booking. Every field is optional; an
// omitted one keeps its current value.
type UpdateInput struct {
	Status           *string
	CollectionStatus *string
	RefundStatus     *string
	Notes            *string
}

// cancelPaidViaUpdateMessage refuses cancelling a paid booking through the
// generic edit: money was collected and nothing is being given back yet, which
// is the pair the refund endpoint exists to handle.
const cancelPaidViaUpdateMessage = "cannot cancel a paid booking via update, use the /cancel endpoint"

// Update applies an owner's edit to a booking, enforcing the three transition
// matrices — status, collection and refund — that decide which moves are legal
// from the state the booking is in.
//
// The order of the checks is the behaviour: a refusal that is decided on the
// spot (a paid booking being cancelled here, a money-axis transition) answers
// immediately, while a status refusal joins the field errors the request is
// answered with at the end. That is why the whole sequence lives in one method
// rather than being split between a shape check and a rule check.
//
// It is one cohesive state transition, and splitting it per axis would relocate
// sequential steps into helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
func (s *Service) Update(ctx context.Context, complexID uuid.UUID, actor Actor, bookingID uuid.UUID, in UpdateInput) (*bookingstore.Booking, error) {
	booking, err := s.ownedBooking(ctx, complexID, bookingID)
	if err != nil {
		return nil, err
	}

	// No version field: the request carries no optimistic-concurrency token
	// because the column behind it was removed with the version counter. What refuses a
	// write that races a bulk cancel or a payment confirmation is the
	// bookings_forbid_status_reversal trigger, in the database, for every
	// writer — the version check provably never fired on that path.
	v := validator.New()

	if in.Status != nil {
		validStatuses := map[string]bool{"pending": true, "confirmed": true, "cancelled": true, "completed": true, "no_show": true}
		v.Check(validStatuses[*in.Status], "status", "must be pending, confirmed, cancelled, completed, or no_show")

		// Validate state transitions.
		allowedTransitions := map[string]map[string]bool{
			"pending":   {"confirmed": true, "cancelled": true},
			"confirmed": {"cancelled": true, "completed": true, "no_show": true},
			"cancelled": {},
			"completed": {"no_show": true},
			"no_show":   {},
		}
		statusLabels := map[string]string{
			"pending":   "pendiente",
			"confirmed": "confirmada",
			"cancelled": "cancelada",
			"completed": "completada",
			"no_show":   "ausente",
		}
		if allowed, ok := allowedTransitions[booking.Status]; ok {
			if !allowed[*in.Status] && *in.Status != booking.Status {
				fromLabel := statusLabels[booking.Status]
				toLabel := statusLabels[*in.Status]
				v.AddError("status", fmt.Sprintf("cannot change from '%s' to '%s'", fromLabel, toLabel))
			}
		}

		// Prevent cancelling a paid booking via generic update — use the cancel or refund endpoint instead.
		//
		// Money was collected AND nothing is being given back yet, which is the
		// pair the single payment_status enum spelled 'deposit_paid' or
		// 'fully_paid'. Both halves are load-bearing: a booking already mid- or
		// post-refund was cancellable here before the payment_status split and still is,
		// because the refund pipeline has already decided what happens to that
		// money.
		if *in.Status == "cancelled" &&
			booking.CollectionStatus != bookingstore.CollectionStatusUnpaid &&
			booking.RefundStatus == bookingstore.RefundStatusNone {
			return nil, &StateError{Message: cancelPaidViaUpdateMessage}
		}

		booking.Status = *in.Status
	}
	if in.CollectionStatus != nil {
		validCollectionStatuses := map[string]bool{
			bookingstore.CollectionStatusUnpaid:      true,
			bookingstore.CollectionStatusDepositPaid: true,
			bookingstore.CollectionStatusFullyPaid:   true,
		}
		v.Check(validCollectionStatuses[*in.CollectionStatus], "collection_status",
			"must be unpaid, deposit_paid or fully_paid")

		// Collection only moves upward. This is the half of the old
		// validPaymentTransitions matrix that lived on the money-in axis
		// (unpaid -> deposit_paid/fully_paid, deposit_paid -> fully_paid); the
		// refund half is the matrix below. The database refuses the return to
		// 'unpaid' as well, in bookings_forbid_status_reversal — this is the
		// policy above that floor, and it also refuses fully_paid ->
		// deposit_paid, which the floor deliberately does not (see the bookings
		// section of db/migrations/001_init.sql for why the floor stops short).
		validCollectionTransitions := map[string]map[string]bool{
			bookingstore.CollectionStatusUnpaid:      {bookingstore.CollectionStatusDepositPaid: true, bookingstore.CollectionStatusFullyPaid: true},
			bookingstore.CollectionStatusDepositPaid: {bookingstore.CollectionStatusFullyPaid: true},
			bookingstore.CollectionStatusFullyPaid:   {},
		}
		collectionLabels := map[string]string{
			bookingstore.CollectionStatusUnpaid:      "sin pago",
			bookingstore.CollectionStatusDepositPaid: "seña pagada",
			bookingstore.CollectionStatusFullyPaid:   "pago completo",
		}
		if allowed, ok := validCollectionTransitions[booking.CollectionStatus]; ok {
			if !allowed[*in.CollectionStatus] && *in.CollectionStatus != booking.CollectionStatus {
				return nil, &ConflictError{Message: fmt.Sprintf(
					"cannot change payment status from '%s' to '%s'",
					collectionLabels[booking.CollectionStatus], collectionLabels[*in.CollectionStatus])}
			}
		}

		booking.CollectionStatus = *in.CollectionStatus
	}
	if in.RefundStatus != nil {
		// 'partial' is deliberately absent from validRefundStatuses: it names a
		// booking whose MercadoPago-backed rows were refunded automatically and
		// whose cash/transfer rows are still owed by hand, and only the refund
		// pipeline that computed that split may set it
		// (internal/payments/refund.go). It IS a key of the transition matrix
		// below, though, and that is not an oversight — before the equivalent
		// entry existed on the merged enum, a booking sitting in that state
		// matched no key at all, `allowed, ok := ...` left ok false, the whole
		// guard fell through, and a generic edit could move it anywhere,
		// including back to fully collected with the manual balance still
		// unpaid. The only transition a staff edit may make from 'partial' is
		// the one the manual-refund endpoint makes, to 'full', once the owner
		// confirms the cash portion was actually returned.
		validRefundStatuses := map[string]bool{
			bookingstore.RefundStatusNone:    true,
			bookingstore.RefundStatusPending: true,
			bookingstore.RefundStatusFull:    true,
		}
		v.Check(validRefundStatuses[*in.RefundStatus], "refund_status",
			"must be none, pending or full")

		validRefundTransitions := map[string]map[string]bool{
			bookingstore.RefundStatusNone:    {bookingstore.RefundStatusPending: true},
			bookingstore.RefundStatusPending: {bookingstore.RefundStatusFull: true},
			bookingstore.RefundStatusPartial: {bookingstore.RefundStatusFull: true},
			bookingstore.RefundStatusFull:    {},
		}
		refundLabels := map[string]string{
			bookingstore.RefundStatusNone:    "sin reembolso",
			bookingstore.RefundStatusPending: "reembolso pendiente",
			bookingstore.RefundStatusPartial: "reembolso parcial",
			bookingstore.RefundStatusFull:    "reembolsado",
		}
		if allowed, ok := validRefundTransitions[booking.RefundStatus]; ok {
			if !allowed[*in.RefundStatus] && *in.RefundStatus != booking.RefundStatus {
				return nil, &ConflictError{Message: fmt.Sprintf(
					"cannot change payment status from '%s' to '%s'",
					refundLabels[booking.RefundStatus], refundLabels[*in.RefundStatus])}
			}
		}

		booking.RefundStatus = *in.RefundStatus
	}
	if in.Notes != nil {
		booking.Notes = in.Notes
	}

	if !v.Valid() {
		return nil, &ValidationError{Errors: v.Errors}
	}

	if err := s.store.Update(ctx, booking); err != nil {
		// The row was deleted between the read above and this write. Same 409
		// the other Update handlers give for it.
		if errors.Is(err, data.ErrRecordNotFound) {
			return nil, ErrEditConflict
		}
		return nil, err
	}

	s.record(actor, complexID, "update", &booking.ID, booking)

	// Increment no_shows counter when owner marks a booking as no_show.
	if in.Status != nil && *in.Status == "no_show" {
		if err := s.clients.IncrementNoShows(ctx, booking.ClientID); err != nil {
			s.logger.Error("update booking: failed to increment no_shows", "error", err, "client_id", booking.ClientID)
		}
	}

	s.realtime.PublishBookingChanged(complexID)

	return booking, nil
}

// unpricedSlotMessage is what both write paths tell a client who asked for a
// slot no price rule covers. It is a business answer, not a fault: the court is
// configured and the complex is open, and this particular position is simply
// not for sale.
const unpricedSlotMessage = "the selected time has no price configured and cannot be booked"

// findPrice answers what one slot costs, and refuses when nothing prices it.
//
// It used to answer that question with a fallback ladder of its own — the band
// covering the slot, then any band for that weekday, then whichever price row
// the query happened to return first — while the availability grid answered the
// same question with pricing.SlotPrice, which has no fallback at all. Two
// answers to one question, and they disagreed exactly where it costs money: a
// slot no band covers is omitted from the storefront and was sold here at the
// first band for that weekday, which under GetCourtPrices' `ORDER BY day_type,
// time_from` is the earliest — the base band. A court whose peak window stops
// short of closing time therefore charged off-peak for peak hours, and a court
// with no bands for that weekday at all charged another weekday's price
// outright. The number the client was shown was never the number they paid.
//
// The overlap half of that defect is already closed in the schema: the
// court_prices_no_overlapping_rule exclusion constraint makes at most one
// band cover any given minute, which is what makes a first-match loop correct
// rather than merely repeatable. What was left was the fallback, and the fix is
// to stop having a second implementation to fall back in.
//
// pricing.ErrNoPriceRule travels out unwrapped so both callers can answer 409
// rather than 500.
//
// It prices the whole booking span, not one slot: court_prices.price is now an
// hourly rate, and a booking's chosen duration can cross a band boundary the
// same way a run of consecutive slots used to.
func (s *Service) findPrice(ctx context.Context, complexID, courtID uuid.UUID, date time.Time, startTime string, durationMinutes int) (int, error) {
	prices, err := s.courts.GetPrices(ctx, courtID)
	if err != nil {
		return 0, err
	}

	// Which window sold this hour decides which rate card prices it. For a
	// venue trading past midnight that is not the calendar day: an hour at
	// 00:30 came out of the previous night's window while that window was
	// still open, and pays that night's rate. See priceWindow.
	schedules, err := s.complexes.GetSchedules(ctx, complexID)
	if err != nil {
		return 0, err
	}
	window := priceWindow(schedules, date, startTime)
	return pricing.BookingPrice(prices, window.Day, window.StartMin, durationMinutes)
}

// ---------------------------------------------------------------------------
// Creating a booking from the owner's dashboard
// ---------------------------------------------------------------------------

// CreateInput is a validated request to book from the dashboard.
type CreateInput struct {
	CourtID         uuid.UUID
	Date            time.Time
	StartTime       string
	DurationMinutes int
	ClientFirstName string
	ClientLastName  string
	ClientPhone     string
	ClientEmail     string
	Notes           string
	PaymentMethod   string
	PaymentOption   string
	DepositAmount   *int
	Price           *int
}

// Create books a court from the owner's dashboard. Unlike the public flow there
// is no payment step: the owner is trusted, so the booking is confirmed
// immediately.
//
// It is one cohesive write — check the court, price it, resolve the client,
// insert, tell everyone — and splitting it would spread a single
// transaction-like sequence across helpers.
//
//nolint:funlen // see the cohesion note above
func (s *Service) Create(ctx context.Context, complex *complexstore.Complex, actor Actor, in CreateInput) (*bookingstore.Booking, error) {
	// Verify court belongs to complex.
	court, err := s.courts.GetByID(ctx, in.CourtID)
	if err != nil {
		return nil, err
	}
	if court.ComplexID != complex.ID {
		return nil, data.ErrRecordNotFound
	}

	// Staff may book any hour of the day, whether or not the complex is open
	// then, or on the grid the storefront renders — that check belongs to the
	// public path alone (PublicBook), which still validates against courtGrid.
	// One thing remains non-negotiable here: the start must fall on the
	// package's 30-minute grid step, or it takes a position no slot lock and no
	// other request can name.
	//
	// A midnight refusal used to sit beside it, because a booking was one row
	// with a single date, start time and stored end time, and the start had
	// to be the
	// smaller of the two. A booking now carries a range that crosses midnight
	// without noticing (the generated span), the check that forbade it is gone
	// (a booking may cross midnight), and a 23:00 booking on a court open until 02:00 is
	// an ordinary sale rather than a shape the database cannot hold. What stops
	// two of them landing on one court is the exclusion constraint, which does
	// not care which day either end falls on.
	if !slots.OnGridStep(in.StartTime) {
		return nil, &FieldError{Field: "start_time", Message: offBoundaryMessage}
	}

	// Hours the owner has taken off sale are off sale to the owner's own
	// dashboard too. The blocked-slot write already refuses to block over a
	// live booking; this is the other direction, which nothing enforced.
	// The end comes from adding the duration to the start instant, never from
	// parsing the end string: slots.Add wraps at midnight, so a 23:00 booking of
	// sixty minutes would read as ending before it began.
	// The candidate's instants are built on the product's wall-clock, the same
	// one blockedDays anchors the block side to. date is parsed as midnight UTC
	// here, and slots.At carries a date's location through, so leaving it
	// unanchored put both sides three hours out — which happened to cancel out
	// on this path and did not on the confirmation path (see grid.go).
	startAt := slots.At(timezone.Day(in.Date), in.StartTime)
	endAt := startAt.Add(time.Duration(in.DurationMinutes) * time.Minute)

	blocked, err := s.slotIsBlocked(ctx, court.ID, in.Date, startAt, endAt)
	if err != nil {
		return nil, err
	}
	if blocked {
		return nil, &ConflictError{Message: blockedSlotMessage}
	}

	// An explicit price overrides the computed one outright — the owner's
	// dashboard may book hours no price rule covers, and this is the only way
	// such a booking can be priced at all. When price is absent, computing it
	// is unchanged from before.
	var totalPrice int
	if in.Price != nil {
		totalPrice = *in.Price
	} else {
		computedPrice, priceErr := s.findPrice(ctx, court.ComplexID, court.ID, in.Date, in.StartTime, in.DurationMinutes)
		if priceErr != nil {
			// A booking no rule fully covers is not for sale on its own — but
			// unlike the public path, the owner can still make it one by
			// supplying price explicitly. The field-level error is what lets
			// the dashboard show the manual price input instead of a dead end.
			if errors.Is(priceErr, pricing.ErrNoPriceRule) {
				return nil, &FieldError{Field: "price", Message: httpx.CodePriceRequired}
			}
			return nil, priceErr
		}
		totalPrice = computedPrice
	}

	if complex.DepositPercentage > 100 {
		return nil, &FieldError{Field: "deposit_percentage", Message: httpx.CodeDepositOver100}
	}

	depositAmount := totalPrice * complex.DepositPercentage / 100

	// Determine collection status based on payment_option. A booking created
	// here has nothing to give back yet, so the refund axis stays at its column
	// default of 'none' and is not named.
	collectionStatus := bookingstore.CollectionStatusUnpaid
	switch in.PaymentOption {
	case "deposit":
		collectionStatus = bookingstore.CollectionStatusDepositPaid
		if in.DepositAmount != nil && *in.DepositAmount > 0 {
			if *in.DepositAmount > totalPrice {
				return nil, &FieldError{Field: "deposit_amount", Message: httpx.CodeDepositExceedsPrice}
			}
			depositAmount = *in.DepositAmount
		}
	case "full":
		collectionStatus = bookingstore.CollectionStatusFullyPaid
	}

	// Get or create client. This is the authenticated owner path
	// (requireComplexOwner), so a name correction on an existing phone match
	// is trusted the way the public path's is not — see
	// clientstore.Store.GetOrCreate's comment on allowNameUpdate.
	client, err := s.clients.GetOrCreate(ctx, complex.ID, in.ClientFirstName, in.ClientLastName, in.ClientPhone, in.ClientEmail, true)
	if err != nil {
		return nil, err
	}

	if client.IsBlocked {
		return nil, &ConflictError{Message: "client is blocked, cannot create booking"}
	}

	if actor.UserID == nil {
		return nil, ErrNoActor
	}
	var notes *string
	if in.Notes != "" {
		notes = &in.Notes
	}

	booking := &bookingstore.Booking{
		ComplexID:        complex.ID,
		CourtID:          in.CourtID,
		ClientID:         client.ID,
		Date:             in.Date,
		StartTime:        in.StartTime,
		DurationMinutes:  in.DurationMinutes,
		Price:            totalPrice,
		DepositAmount:    depositAmount,
		Status:           "confirmed",
		CollectionStatus: collectionStatus,
		RefundStatus:     bookingstore.RefundStatusNone,
		Notes:            notes,
		CreatedBy:        actor.UserID,
	}

	if err := s.store.InsertSafe(ctx, booking); err != nil {
		return nil, err
	}

	s.record(actor, complex.ID, "create", &booking.ID, booking)

	// Record payment when staff marks deposit or full payment at creation.
	if in.PaymentMethod != "" && collectionStatus != bookingstore.CollectionStatusUnpaid {
		paymentAmount := totalPrice
		if collectionStatus == bookingstore.CollectionStatusDepositPaid {
			paymentAmount = depositAmount
		}
		payment := &paymentstore.Payment{
			BookingID: booking.ID,
			ComplexID: booking.ComplexID,
			Amount:    paymentAmount,
			Method:    in.PaymentMethod,
			// payments.status still carries the shared payment_status enum
			// (the payment_status split took only bookings off it), and these two values
			// are legal in it.
			Status: collectionStatus,
		}
		if err := s.payments.Insert(ctx, payment); err != nil {
			s.logger.Error("create booking: failed to insert payment", "error", err, "booking_id", booking.ID)
		}
	}

	// Send booking confirmation notification.
	clientEmail := ""
	if client.Email != nil {
		clientEmail = *client.Email
	}
	// InsertSafe already minted this booking's access token
	// (bookingstore.Booking.LinkToken); it is the credential every public route
	// authorizes on, not the booking's primary key.
	cancelURL := booklink.Cancel(s.cfg.FrontendURL, complex.Slug, booking.LinkToken)
	cancelPath := booklink.CancelPath(complex.Slug, booking.LinkToken)
	mapsQuery := booklink.MapsQuery(complex.Name, complex.Address, complex.City, complex.Latitude, complex.Longitude)
	mapsURL := booklink.MapsURL(complex.Name, complex.Address, complex.City, complex.Latitude, complex.Longitude)
	address := booklink.Address(complex.Address, complex.City)
	clientFullName := client.FirstName + " " + client.LastName
	// Read off the booking that was just written, so a staff booking entered
	// with nothing collected reports a $0 deposit rather than quoting one
	// nobody paid.
	confirmDepositAmount, confirmBalanceAmount := notifications.PaymentAmounts(booking.Price, booking.DepositAmount, booking.CollectionStatus)
	s.notify.BookingConfirmed(notifications.BookingConfirmation{
		// SourceStaffCreate, and not a free-text label: this constant is what
		// suppresses the owner's "Nueva reserva" email, since the owner is the
		// person who just made this booking.
		Source:        notifications.SourceStaffCreate,
		Email:         clientEmail,
		Phone:         client.Phone,
		ComplexName:   complex.Name,
		CourtName:     court.Name,
		ClientName:    clientFullName,
		Date:          booking.Date.Format("02/01"),
		StartTime:     timezone.HoursLabel(booking.StartsAt, booking.EndsAt),
		CancelURL:     cancelURL,
		CancelPath:    cancelPath,
		MapsQuery:     mapsQuery,
		Address:       address,
		MapsURL:       mapsURL,
		DepositAmount: confirmDepositAmount,
		BalanceAmount: confirmBalanceAmount,
		CancellationLine: notifications.CancellationLine(complex.CancellationHours, s.cfg.GracePeriod,
			pricing.WithinStandardWindow(booking, complex.CancellationHours)),
		OwnerID:   complex.OwnerID.String(),
		BookingID: booking.ID.String(),
	})

	s.realtime.PublishBookingChanged(complex.ID)

	return booking, nil
}

// ---------------------------------------------------------------------------
// Cancelling, confirming payment, closing out a manual refund
// ---------------------------------------------------------------------------

// CancelResult is what a staff cancellation produced: the booking as it now
// stands, and what became of the client's money.
type CancelResult struct {
	Booking *bookingstore.Booking
	Outcome paymentstore.RefundOutcome
	// AlreadyCancelled reports that the booking was cancelled before this
	// request arrived, so nothing was done and no refund was attempted.
	AlreadyCancelled bool
}

// Cancel cancels a booking from the owner's dashboard and returns whatever the
// client had paid.
//
// Cancelling an already-cancelled booking does nothing, so a retried request
// cannot refund twice. Completed and no-show bookings are terminal and refused.
//
// Staff cancellations refund regardless of the complex's cancellation window;
// only the public path applies it. That asymmetry is the product's, not an
// oversight — the owner cancelling is the venue's own decision, and it should
// not charge the client a penalty for it.
//
// It is one cohesive state transition — cancel, refund, notify, release — and
// splitting it would relocate sequential steps into helpers without reducing
// what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
func (s *Service) Cancel(ctx context.Context, complex *complexstore.Complex, actor Actor, bookingID uuid.UUID, reason string) (CancelResult, error) {
	booking, err := s.ownedBooking(ctx, complex.ID, bookingID)
	if err != nil {
		return CancelResult{}, err
	}

	// Only pending/confirmed bookings can be cancelled. Terminal states are immutable.
	if booking.Status == "cancelled" || booking.Status == "completed" || booking.Status == "no_show" {
		if booking.Status == "cancelled" {
			// A retried cancellation answers from the row: the booking already
			// carries the payment status the refund path wrote, so nothing here
			// has to guess at an outcome this request did not produce.
			return CancelResult{Booking: booking, AlreadyCancelled: true}, nil
		}
		return CancelResult{}, &StateError{Message: fmt.Sprintf("cannot cancel a booking with status '%s'", booking.Status)}
	}

	booking.Status = "cancelled"
	if reason != "" {
		notes := reason
		if booking.Notes != nil {
			notes = *booking.Notes + " | Cancelación: " + reason
		}
		booking.Notes = &notes
	}

	// owesRefund feeds both the marker write below and the refund dispatch
	// after Update — one expression, not two, so an edit that marks a
	// cancellation the money must not follow is, by construction, the same
	// edit that stops refunding it (refund-intent-durability spec's
	// load-bearing property). The staff path ignores the complex's refund
	// window entirely (see the comment above), so this reads only the
	// collection axis: money was taken, so money is owed back.
	owesRefund := booking.CollectionStatus != bookingstore.CollectionStatusUnpaid
	if owesRefund {
		now := time.Now()
		booking.RefundIntentAt = &now
	}

	if err := s.store.Update(ctx, booking); err != nil {
		return CancelResult{}, err
	}

	s.record(actor, complex.ID, "cancel", &booking.ID, booking)

	// Handle MP payment: expire the preference if unpaid, auto-refund if paid.
	var outcome paymentstore.RefundOutcome
	if !owesRefund {
		s.expireCheckoutPreference(ctx, booking, complex)
		outcome = paymentstore.RefundOutcome{Result: paymentstore.RefundNone, Reason: "the booking was never paid"}
	} else {
		outcome = s.refunds.AutoRefundIfPaid(ctx, booking)
	}
	if outcome.NeedsAHuman() {
		// The owner is the person who has to hand the money back, and this
		// response is the only moment they are looking.
		s.logger.Error("cancel booking: a refund is owed that must be returned by hand",
			"booking_id", booking.ID, "complex_id", complex.ID,
			"amount", outcome.AmountCentavos, "reason", outcome.Reason)
	}

	s.notifyCancelledAndRelease(ctx, booking, complex, outcome)

	s.realtime.PublishBookingChanged(complex.ID)

	booking.RefundStatus = refundStatusAfter(booking, outcome)
	// ClaimRefund clears the marker in the database but cannot reach this
	// in-memory *Booking, so it is cleared here unconditionally: the field was
	// either never set, cleared inside a committed ClaimRefund, or cleared by
	// AutoRefundIfPaid's other exits — nil is correct in every case.
	booking.RefundIntentAt = nil

	return CancelResult{Booking: booking, Outcome: outcome}, nil
}

// ConfirmPaymentInput is a validated record of money the owner took in cash or
// by transfer.
type ConfirmPaymentInput struct {
	Method string
	Amount int
}

// ConfirmPayment records a payment the owner took at the counter, and confirms
// the booking it pays for.
//
// It is one cohesive state transition — check, price, commit — and splitting it
// would relocate sequential steps into helpers without reducing what a reader
// holds at once.
//
//nolint:funlen // see the cohesion note above
func (s *Service) ConfirmPayment(ctx context.Context, complexID uuid.UUID, actor Actor, bookingID uuid.UUID, in ConfirmPaymentInput) (*bookingstore.Booking, *paymentstore.Payment, error) {
	booking, err := s.ownedBooking(ctx, complexID, bookingID)
	if err != nil {
		return nil, nil, err
	}

	// Prevent confirming payment on cancelled or already-paid bookings.
	if booking.Status == "cancelled" {
		return nil, nil, &StateError{Message: "cannot confirm payment for a cancelled booking"}
	}
	if booking.CollectionStatus == bookingstore.CollectionStatusFullyPaid {
		return nil, nil, &StateError{Message: "esta reserva ya tiene pago confirmado"}
	}

	// Confirming a pending booking is what makes it hold its slot: until this
	// point the row does not stop anyone else, and afterwards it does. So it is
	// the second place that has to ask whether those hours are still on sale.
	//
	// InsertAndConfirmBooking re-checks the bookings table under the court-day
	// advisory lock, which is why a genuine race reaches here as
	// ErrSlotUnavailable. It does not look at blocked_slots at all, and neither
	// did this path — so an owner who blocked a court for maintenance after
	// a client started checkout could still be handed a confirmed booking on
	// it, taking the hours out of circulation for the maintenance and selling
	// them at the same time.
	blocked, err := s.slotIsBlocked(ctx, booking.CourtID, booking.Date, booking.StartsAt, booking.EndsAt)
	if err != nil {
		return nil, nil, err
	}
	if blocked {
		return nil, nil, &ConflictError{Message: blockedSlotMessage}
	}

	payment := &paymentstore.Payment{
		BookingID: booking.ID,
		ComplexID: booking.ComplexID,
		Amount:    in.Amount,
		Method:    in.Method,
		Status:    "fully_paid",
	}

	// Confirming payment also confirms the booking (prevents cron from
	// expiring a paid booking that was still in "pending" status).
	if booking.Status == "pending" {
		booking.Status = "confirmed"
	}
	totalPaid := in.Amount
	if booking.CollectionStatus == bookingstore.CollectionStatusDepositPaid {
		totalPaid += booking.DepositAmount
	}
	if totalPaid >= booking.Price {
		booking.CollectionStatus = bookingstore.CollectionStatusFullyPaid
	} else {
		booking.CollectionStatus = bookingstore.CollectionStatusDepositPaid
		booking.DepositAmount = totalPaid
	}
	if err := s.payments.InsertAndConfirmBooking(ctx, payment, booking); err != nil {
		return nil, nil, err
	}

	s.record(actor, complexID, "confirm_payment", &booking.ID, booking)
	s.realtime.PublishBookingChanged(complexID)

	return booking, payment, nil
}

// ManualRefundResult is what closing out a partial refund produced.
type ManualRefundResult struct {
	Booking  *bookingstore.Booking
	Payments []*paymentstore.Payment
	// Returned is the cash/transfer balance the owner has just confirmed they
	// handed back.
	Returned int
}

// ManualRefund closes out a partially refunded booking once the owner confirms
// they returned the cash/transfer balance to the client by hand.
//
// A booking reaches refund_status 'partial' when the refund pipeline
// auto-refunds its MercadoPago-backed payment rows but a sibling cash or
// transfer row is still owed — nothing automatic can return that money, and it
// stays owed until this records that a person did. It is the only path allowed
// to move a booking off 'partial' (see validRefundStatuses in Update); a
// generic edit may not.
func (s *Service) ManualRefund(ctx context.Context, complexID uuid.UUID, actor Actor, bookingID uuid.UUID) (ManualRefundResult, error) {
	booking, err := s.ownedBooking(ctx, complexID, bookingID)
	if err != nil {
		return ManualRefundResult{}, err
	}

	if booking.Status != "cancelled" || booking.RefundStatus != bookingstore.RefundStatusPartial {
		return ManualRefundResult{}, &StateError{Message: noManualRefundOwedMessage}
	}

	returned, err := s.payments.RecordManualRefund(ctx, booking.ID)
	if err != nil {
		if errors.Is(err, paymentstore.ErrNoManualRefundOwed) {
			// Locked under its own transaction, the booking no longer read
			// refund_status 'partial' — another request closed it out between
			// the read above and the write.
			return ManualRefundResult{}, &StateError{Message: noManualRefundOwedMessage}
		}
		return ManualRefundResult{}, err
	}

	booking.RefundStatus = bookingstore.RefundStatusFull

	payments, err := s.payments.ListByBookingID(ctx, booking.ID)
	if err != nil {
		return ManualRefundResult{}, err
	}

	s.record(actor, complexID, "manual_refund", &booking.ID, booking)
	s.realtime.PublishBookingChanged(complexID)

	return ManualRefundResult{Booking: booking, Payments: payments, Returned: returned}, nil
}

// noManualRefundOwedMessage is the one sentence both refusals answer with: the
// pre-check against the booking as it was read, and the store's own re-check
// under the lock.
const noManualRefundOwedMessage = "esta reserva no tiene una devolución manual pendiente"
