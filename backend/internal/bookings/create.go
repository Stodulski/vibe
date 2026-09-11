package bookings

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/stodulski/vibe-server/internal/booklink"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/notifications"
	"github.com/stodulski/vibe-server/internal/pricing"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/timezone"
	"github.com/stodulski/vibe-server/internal/validator"
)

// Create handles POST /api/v1/complexes/:id/bookings, the owner booking from
// the dashboard. Unlike the public flow there is no payment step: the owner
// is trusted, so the booking is confirmed immediately.
//
// It is one cohesive request lifecycle for a single resource operation, per
// this codebase's handler conventions (CLAUDE.md); splitting it would relocate
// sequential steps into helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	var input struct {
		CourtID         string `json:"court_id"`
		Date            string `json:"date"`
		StartTime       string `json:"start_time"`
		ClientPhone     string `json:"client_phone"`
		ClientEmail     string `json:"client_email"`
		ClientFirstName string `json:"client_first_name"`
		ClientLastName  string `json:"client_last_name"`
		PaymentMethod   string `json:"payment_method"`
		Notes           string `json:"notes"`
		DurationMinutes int    `json:"duration_minutes"`
		PaymentOption   string `json:"payment_option"`
		DepositAmount   *int   `json:"deposit_amount"`
		Price           *int   `json:"price"`
	}

	err := httpx.ReadJSON(w, r, &input)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	v.Check(input.CourtID != "", "court_id", "must be provided")
	v.Check(input.Date != "", "date", "must be provided")
	v.Check(input.StartTime != "", "start_time", "must be provided")
	v.Check(input.ClientPhone != "", "client_phone", "must be provided")
	if input.ClientPhone != "" {
		normalized, nerr := validator.NormalizePhone(input.ClientPhone)
		if nerr != nil {
			v.AddError("client_phone", "must be a valid phone number (E.164 format, e.g. +5491112345678)")
		} else {
			input.ClientPhone = normalized
		}
	}
	v.Check(input.ClientFirstName != "", "client_first_name", "must be provided")
	v.Check(input.ClientLastName != "", "client_last_name", "must be provided")
	if input.ClientEmail != "" {
		v.Check(validator.Matches(input.ClientEmail, validator.EmailRX), "client_email", "must be a valid email address")
	}
	v.Check(input.PaymentMethod == "" || input.PaymentMethod == "cash" || input.PaymentMethod == "transfer",
		"payment_method", "must be cash or transfer")
	v.Check(input.PaymentOption == "" || input.PaymentOption == "unpaid" || input.PaymentOption == "deposit" || input.PaymentOption == "full",
		"payment_option", "must be unpaid, deposit, or full")
	v.Check(validator.PermittedValue(input.DurationMinutes, slots.PermittedDurations()...), "duration_minutes", durationMessage)
	if input.DepositAmount != nil {
		v.Check(*input.DepositAmount > 0, "deposit_amount", "must be greater than 0")
	}
	if input.Price != nil {
		v.Check(*input.Price >= 0, "price", "must not be negative")
	}
	if input.StartTime != "" {
		v.Check(slots.ValidFormat(input.StartTime), "start_time", "must be in HH:MM format (00:00-23:59)")
	}
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	courtID, err := uuid.Parse(input.CourtID)
	if err != nil {
		v.AddError("court_id", "must be a valid UUID")
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	date, err := time.Parse("2006-01-02", input.Date)
	if err != nil {
		v.AddError("date", "must be in YYYY-MM-DD format")
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	// Verify court belongs to complex.
	court, err := h.courts.GetByID(r.Context(), courtID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			h.respond.NotFound(w, r)
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}
	if court.ComplexID != complex.ID {
		h.respond.NotFound(w, r)
		return
	}

	// Staff may book any hour of the day, whether or not the complex is open
	// then, or on the grid the storefront renders — that check belongs to the
	// public path alone (public.go), which still validates against h.courtGrid.
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
	if !slots.OnGridStep(input.StartTime) {
		v.AddError("start_time", offBoundaryMessage)
		h.respond.FailedValidation(w, r, v.Errors)
		return
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
	startAt := slots.At(timezone.Day(date), input.StartTime)
	endAt := startAt.Add(time.Duration(input.DurationMinutes) * time.Minute)

	blocked, err := h.slotIsBlocked(r.Context(), court.ID, date, startAt, endAt)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}
	if blocked {
		h.respond.Error(w, r, http.StatusConflict, blockedSlotMessage)
		return
	}

	// An explicit price overrides the computed one outright — the owner's
	// dashboard may book hours no price rule covers, and this is the only way
	// such a booking can be priced at all. When price is absent, computing it
	// is unchanged from before.
	var totalPrice int
	if input.Price != nil {
		totalPrice = *input.Price
	} else {
		computedPrice, priceErr := h.findPrice(r, court.ComplexID, court.ID, date, input.StartTime, input.DurationMinutes)
		if priceErr != nil {
			// A booking no rule fully covers is not for sale on its own — but
			// unlike the public path, the owner can still make it one by
			// supplying price explicitly. The field-level error is what lets
			// the dashboard show the manual price input instead of a dead end.
			if errors.Is(priceErr, pricing.ErrNoPriceRule) {
				v.AddError("price", httpx.CodePriceRequired)
				h.respond.FailedValidation(w, r, v.Errors)
				return
			}
			h.respond.ServerError(w, r, priceErr)
			return
		}
		totalPrice = computedPrice
	}

	if complex.DepositPercentage > 100 {
		v.AddError("deposit_percentage", httpx.CodeDepositOver100)
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	depositAmount := totalPrice * complex.DepositPercentage / 100

	// Determine collection status based on payment_option. A booking created
	// here has nothing to give back yet, so the refund axis stays at its column
	// default of 'none' and is not named.
	collectionStatus := data.CollectionStatusUnpaid
	switch input.PaymentOption {
	case "deposit":
		collectionStatus = data.CollectionStatusDepositPaid
		if input.DepositAmount != nil && *input.DepositAmount > 0 {
			if *input.DepositAmount > totalPrice {
				v.AddError("deposit_amount", httpx.CodeDepositExceedsPrice)
				h.respond.FailedValidation(w, r, v.Errors)
				return
			}
			depositAmount = *input.DepositAmount
		}
	case "full":
		collectionStatus = data.CollectionStatusFullyPaid
	}

	// Get or create client. This is the authenticated owner path
	// (requireComplexOwner), so a name correction on an existing phone match
	// is trusted the way public.go's is not — see ClientModel.GetOrCreate's
	// comment on allowNameUpdate.
	client, err := h.clients.GetOrCreate(r.Context(), complex.ID, input.ClientFirstName, input.ClientLastName, input.ClientPhone, input.ClientEmail, true)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	if client.IsBlocked {
		h.respond.Error(w, r, http.StatusConflict, "client is blocked, cannot create booking")
		return
	}

	user, ok := httpx.ContextGetAuthenticatedUser(r)
	if !ok {
		h.respond.InvalidAuthenticationToken(w, r)
		return
	}
	var notes *string
	if input.Notes != "" {
		notes = &input.Notes
	}

	booking := &data.Booking{
		ComplexID:        complex.ID,
		CourtID:          courtID,
		ClientID:         client.ID,
		Date:             date,
		StartTime:        input.StartTime,
		DurationMinutes:  input.DurationMinutes,
		Price:            totalPrice,
		DepositAmount:    depositAmount,
		Status:           "confirmed",
		CollectionStatus: collectionStatus,
		RefundStatus:     data.RefundStatusNone,
		Notes:            notes,
		CreatedBy:        &user.ID,
	}

	err = h.store.InsertSafe(r.Context(), booking)
	if err != nil {
		if errors.Is(err, data.ErrDuplicateBooking) || errors.Is(err, data.ErrSlotUnavailable) {
			h.respond.EditConflict(w, r)
			return
		}
		// H-22: the court was deleted between the GetByID above and this
		// commit. Answered the same way as a court that was already gone when
		// that check ran — it is the same condition, found a moment later, and
		// a caller has no way to tell the two apart or any reason to.
		if errors.Is(err, data.ErrRecordNotFound) {
			h.respond.NotFound(w, r)
			return
		}
		h.respond.ServerError(w, r, err)
		return
	}

	h.record(r, complex.ID, "create", &booking.ID, booking)

	// Record payment when staff marks deposit or full payment at creation.
	if input.PaymentMethod != "" && collectionStatus != data.CollectionStatusUnpaid {
		paymentAmount := totalPrice
		if collectionStatus == data.CollectionStatusDepositPaid {
			paymentAmount = depositAmount
		}
		payment := &data.Payment{
			BookingID: booking.ID,
			ComplexID: booking.ComplexID,
			Amount:    paymentAmount,
			Method:    input.PaymentMethod,
			// payments.status still carries the shared payment_status enum
			// (the payment_status split took only bookings off it), and these two values
			// are legal in it.
			Status: collectionStatus,
		}
		if err := h.payments.Insert(r.Context(), payment); err != nil {
			h.logger.Error("create booking: failed to insert payment", "error", err, "booking_id", booking.ID)
		}
	}

	// Send booking confirmation notification.
	clientEmail := ""
	if client.Email != nil {
		clientEmail = *client.Email
	}
	// InsertSafe already minted this booking's access token
	// (data.Booking.LinkToken); it is the credential every public route
	// authorizes on, not the booking's primary key.
	cancelURL := booklink.Cancel(h.cfg.FrontendURL, complex.Slug, booking.LinkToken)
	cancelPath := booklink.CancelPath(complex.Slug, booking.LinkToken)
	mapsQuery := booklink.MapsQuery(complex.Name, complex.Address, complex.City, complex.Latitude, complex.Longitude)
	mapsURL := booklink.MapsURL(complex.Name, complex.Address, complex.City, complex.Latitude, complex.Longitude)
	address := booklink.Address(complex.Address, complex.City)
	clientFullName := client.FirstName + " " + client.LastName
	// Read off the booking that was just written, so a staff booking entered
	// with nothing collected reports a $0 deposit rather than quoting one
	// nobody paid.
	confirmDepositAmount, confirmBalanceAmount := notifications.PaymentAmounts(booking.Price, booking.DepositAmount, booking.CollectionStatus)
	h.notify.BookingConfirmed(notifications.BookingConfirmation{
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
		CancellationLine: notifications.CancellationLine(complex.CancellationHours, h.cfg.GracePeriod,
			pricing.WithinStandardWindow(booking, complex.CancellationHours)),
		OwnerID: complex.OwnerID.String(),
	})

	h.realtime.PublishBookingChanged(complex.ID)

	h.respond.JSON(w, r, http.StatusCreated, httpx.Envelope{"booking": booking})
}
