package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/data"
	slotguard "github.com/stodulski/vibe-server/internal/data/slotguard"
	"github.com/stodulski/vibe-server/internal/db"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// Checkout refusals. Each one leaves nothing recorded: the checkout is one
// transaction, so a refused balance also rolls back the products beside it.
var (
	// ErrNothingToCheckOut reports a checkout with neither product lines nor
	// booking payments.
	ErrNothingToCheckOut = errors.New("sale: checkout has no product lines and no booking payments")
	// ErrBookingBalanceMismatch reports a booking payment whose amount is not
	// the booking's current remaining balance: the cart is stale.
	ErrBookingBalanceMismatch = errors.New("sale: booking amount differs from its remaining balance")
	// ErrBookingAlreadyPaid reports a booking with nothing left to collect.
	ErrBookingAlreadyPaid = errors.New("sale: booking is already fully paid")
	// ErrBookingNotPayable reports a booking that cannot take a counter
	// payment: cancelled, no-show, or still pending (confirm it from the
	// booking panel first, which re-checks its slot).
	ErrBookingNotPayable = errors.New("sale: booking cannot take a payment")
	// ErrSaleHasBookingPayments reports a void of a sale that collected booking
	// balances. A booking payment has no reversal, so the sale stays as it is.
	ErrSaleHasBookingPayments = errors.New("sale: this sale collected booking payments and cannot be voided")
)

// ErrUnknownBooking names a booking_payments entry whose booking does not exist
// in this complex. The handler maps it to a field error on that entry.
type ErrUnknownBooking struct {
	BookingID uuid.UUID
}

// Error implements error.
func (e *ErrUnknownBooking) Error() string {
	return fmt.Sprintf("sale: booking %s does not exist in this complex", e.BookingID)
}

// BookingPaymentInput is one balance a checkout collects: the booking and the
// amount, which must equal the booking's remaining balance.
type BookingPaymentInput struct {
	BookingID uuid.UUID
	Amount    int
}

// BookingPaymentLine is one balance a checkout collected, with the booking
// summary a receipt names it by.
type BookingPaymentLine struct {
	PaymentID  uuid.UUID
	BookingID  uuid.UUID
	Amount     int
	Date       time.Time
	StartsAt   time.Time
	EndsAt     time.Time
	CourtName  string
	ClientName string
}

// CheckoutResult is what one checkout recorded. Sale is nil when the checkout
// had no product lines: a booking-only checkout has no sale to hang on.
type CheckoutResult struct {
	Sale         *Sale
	Items        []*SaleItem
	Warnings     []StockWarning
	BookingLines []*BookingPaymentLine
}

// BalanceDue is one booking of a day that still owes money, for the sell page.
type BalanceDue struct {
	BookingID  uuid.UUID
	Date       time.Time
	StartsAt   time.Time
	EndsAt     time.Time
	CourtName  string
	ClientName string
	Status     string
	Total      int
	Paid       int
	Remaining  int
}

// Create records a product sale: Checkout with no booking payments.
func (m *Store) Create(ctx context.Context, complexID, actorID uuid.UUID, items []ItemInput, method string, note *string) (*Sale, []*SaleItem, []StockWarning, error) {
	res, err := m.Checkout(ctx, complexID, actorID, items, nil, method, note)
	if err != nil {
		return nil, nil, nil, err
	}
	return res.Sale, res.Items, res.Warnings, nil
}

// Checkout records one counter checkout in ONE transaction. It locks the
// complex's open cash session (ErrNoOpenCashSession when none), then:
//
//   - for product lines: locks every named product (ascending id, one row at a
//     time, see writeProductSale), validates each exists and is active, prices
//     the total from the LOCKED products, and writes the 'sale' cash income
//     (amount = products total), the sale, its items and one 'sale' stock
//     movement per tracked item;
//   - for booking payments: locks each booking's court day, then the booking
//     row (FOR UPDATE), re-reads its balance under that lock, refuses an amount
//     that is not the remaining balance, and records the payment through the
//     booking payment logic (payments.Payments.ConfirmInTx) with the same method.
//     When the checkout has a sale, each balance is linked to it.
//
// The sale's cash income carries the products only: a balance is counted once,
// through its payments row. Refusals are the sentinels above, plus
// *ErrInvalidItems and *ErrUnknownBooking; a refused checkout records nothing.
//
//nolint:funlen // one cohesive transaction: session, products, balances and the links between them.
func (m *Store) Checkout(ctx context.Context, complexID, actorID uuid.UUID, items []ItemInput, bookings []BookingPaymentInput, method string, note *string) (*CheckoutResult, error) {
	if len(items) == 0 && len(bookings) == 0 {
		return nil, ErrNothingToCheckOut
	}

	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	if err := data.AssertTenant(ctx, complexID); err != nil {
		return nil, err
	}

	result := &CheckoutResult{}
	err := m.DB.WithTx(ctx, func(tx pgx.Tx, qtx *db.Queries) error {
		session, err := qtx.GetOpenCashSessionForUpdate(ctx, data.UUIDToPg(complexID))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNoOpenCashSession
			}
			return fmt.Errorf("sale: lock open cash session: %w", err)
		}

		var sale *Sale
		if len(items) > 0 {
			sale, result.Items, result.Warnings, err = writeProductSale(ctx, qtx, complexID, actorID, data.PgToUUID(session.ID), items, method, note)
			if err != nil {
				return err
			}
			result.Sale = sale
		}

		if len(bookings) > 0 {
			lines, err := m.collectBalances(ctx, tx, qtx, complexID, bookings, method, sale)
			if err != nil {
				return err
			}
			result.BookingLines = lines
		}
		return nil
	})
	if err != nil {
		var invalid *ErrInvalidItems
		if errors.As(err, &invalid) {
			return nil, invalid
		}
		var unknown *ErrUnknownBooking
		if errors.As(err, &unknown) {
			return nil, unknown
		}
		return nil, err
	}
	return result, nil
}

// writeProductSale is the product half of a checkout: products locked one at a
// time in ascending id order (a single batched FOR UPDATE does not guarantee
// the lock order two concurrent sales need, see Checkout), validated, priced
// from the locked rows, and written as one sale with its cash income and
// stock movements. The caller holds the transaction and the open session.
//
//nolint:funlen // one cohesive write, the same shape and justification as Checkout.
func writeProductSale(ctx context.Context, qtx *db.Queries, complexID, actorID, sessionID uuid.UUID, items []ItemInput, method string, note *string) (*Sale, []*SaleItem, []StockWarning, error) {
	ids := make([]uuid.UUID, len(items))
	for i, it := range items {
		ids[i] = it.ProductID
	}
	sort.Slice(ids, func(i, j int) bool { return bytes.Compare(ids[i][:], ids[j][:]) < 0 })

	locked := make(map[uuid.UUID]db.Product, len(ids))
	problems := map[uuid.UUID]ItemProblem{}
	for _, id := range ids {
		row, err := qtx.GetProductByIDForUpdate(ctx, db.GetProductByIDForUpdateParams{
			ID:        data.UUIDToPg(id),
			ComplexID: data.UUIDToPg(complexID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				problems[id] = ItemNotFound
				continue
			}
			return nil, nil, nil, fmt.Errorf("sale: lock product %s: %w", id, err)
		}
		if !row.Active {
			problems[id] = ItemInactive
			continue
		}
		locked[id] = row
	}
	if len(problems) > 0 {
		return nil, nil, nil, &ErrInvalidItems{Problems: problems}
	}

	total := 0
	for _, it := range items {
		total += int(locked[it.ProductID].Price) * it.Quantity
	}
	if total <= 0 {
		return nil, nil, nil, ErrZeroTotal
	}
	if total > maxSaleTotal {
		return nil, nil, nil, ErrTotalExceedsCap
	}

	description := fmt.Sprintf("Sale: %d item(s)", len(items))
	if note != nil && *note != "" {
		description = description + " — " + *note
	}

	//nolint:gosec // G115: total is checked positive and <= maxSaleTotal above, well within int32 range.
	cashRow, err := qtx.InsertCashMovement(ctx, db.InsertCashMovementParams{
		ComplexID: data.UUIDToPg(complexID),
		SessionID: data.UUIDToPg(sessionID),
		Kind:      "income",
		Category:  "sale",
		Method:    db.PaymentMethod(method),
		Amount:    int32(total),
		Note:      data.TextToPg(&description),
		CreatedBy: data.UUIDToPg(actorID),
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("sale: insert cash movement: %w", err)
	}

	saleRow, err := qtx.InsertSale(ctx, db.InsertSaleParams{
		ComplexID:      data.UUIDToPg(complexID),
		SessionID:      data.UUIDToPg(sessionID),
		Method:         db.PaymentMethod(method),
		Total:          int32(total), //nolint:gosec // G115: same bound as Amount above.
		CashMovementID: cashRow.ID,
		CreatedBy:      data.UUIDToPg(actorID),
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("sale: insert sale: %w", err)
	}
	sale := saleFromDB(saleRow)

	var saleItems []*SaleItem
	var warnings []StockWarning
	for _, it := range items {
		product := locked[it.ProductID]
		lineTotal := int(product.Price) * it.Quantity

		itemRow, err := qtx.InsertSaleItem(ctx, db.InsertSaleItemParams{
			ComplexID:   data.UUIDToPg(complexID),
			SaleID:      saleRow.ID,
			ProductID:   product.ID,
			ProductName: product.Name,
			UnitPrice:   product.Price,
			//nolint:gosec // G115: quantity is validated 1..1000 by the handler.
			Quantity: int32(it.Quantity),
			//nolint:gosec // G115: bounded by unit_price and quantity above, both well within range, and by the sale total cap.
			LineTotal: int32(lineTotal),
		})
		if err != nil {
			return nil, nil, nil, fmt.Errorf("sale: insert sale item for product %s: %w", data.PgToUUID(product.ID), err)
		}
		saleItems = append(saleItems, saleItemFromDB(itemRow))

		if !product.TracksStock {
			continue
		}

		if _, err := qtx.InsertStockMovement(ctx, db.InsertStockMovementParams{
			ComplexID: data.UUIDToPg(complexID),
			ProductID: product.ID,
			Kind:      "sale",
			//nolint:gosec // G115: quantity is validated 1..1000 by the handler, negated here, still far below int32 range.
			Quantity:  int32(-it.Quantity),
			SaleID:    saleRow.ID,
			CreatedBy: data.UUIDToPg(actorID),
		}); err != nil {
			return nil, nil, nil, fmt.Errorf("sale: insert stock movement for product %s: %w", data.PgToUUID(product.ID), err)
		}

		updatedProduct, err := qtx.UpdateProductStock(ctx, db.UpdateProductStockParams{
			ID:        product.ID,
			ComplexID: data.UUIDToPg(complexID),
			//nolint:gosec // G115: same bound as Quantity above.
			Delta: int32(-it.Quantity),
		})
		if err != nil {
			return nil, nil, nil, fmt.Errorf("sale: apply stock delta for product %s: %w", data.PgToUUID(product.ID), err)
		}
		if updatedProduct.StockOnHand <= 0 {
			warnings = append(warnings, StockWarning{
				ProductID:   data.PgToUUID(product.ID),
				ProductName: product.Name,
				StockOnHand: int(updatedProduct.StockOnHand),
			})
		}
	}
	return sale, saleItems, warnings, nil
}

// collectBalances is the booking half of a checkout. It runs inside the
// checkout's transaction and, when sale is non-nil, links each payment to it.
//
// Lock order is the one payments.Payments.ConfirmInTx documents: every court
// day this checkout touches is locked BEFORE any booking row, because the
// booking confirmation path and the slot writer take the same two locks in
// that order. Rows are then locked in ascending id order, so two checkouts
// sharing bookings cannot deadlock each other.
//
//nolint:funlen // one cohesive pass: read, lock, validate and record each balance, then link them.
func (m *Store) collectBalances(ctx context.Context, tx pgx.Tx, qtx *db.Queries, complexID uuid.UUID, requested []BookingPaymentInput, method string, sale *Sale) ([]*BookingPaymentLine, error) {
	amounts := make(map[uuid.UUID]int, len(requested))
	ids := make([]uuid.UUID, 0, len(requested))
	for _, r := range requested {
		if _, dup := amounts[r.BookingID]; dup {
			return nil, fmt.Errorf("sale: booking %s appears twice in one checkout", r.BookingID)
		}
		amounts[r.BookingID] = r.Amount
		ids = append(ids, r.BookingID)
	}
	sort.Slice(ids, func(i, j int) bool { return bytes.Compare(ids[i][:], ids[j][:]) < 0 })

	// Unlocked read to learn each booking's court and day, so the court days
	// can be locked first.
	type courtDay struct {
		court uuid.UUID
		date  time.Time
	}
	var days []courtDay
	seen := make(map[courtDay]bool)
	for _, id := range ids {
		row, err := qtx.GetBookingByID(ctx, db.GetBookingByIDParams{
			ID:        data.UUIDToPg(id),
			ComplexID: data.UUIDToPg(complexID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, &ErrUnknownBooking{BookingID: id}
			}
			return nil, fmt.Errorf("sale: read booking %s: %w", id, err)
		}
		day := courtDay{court: data.PgToUUID(row.CourtID), date: data.PgToDate(row.Date)}
		if !seen[day] {
			seen[day] = true
			days = append(days, day)
		}
	}
	sort.Slice(days, func(i, j int) bool {
		if c := bytes.Compare(days[i].court[:], days[j].court[:]); c != 0 {
			return c < 0
		}
		return days[i].date.Before(days[j].date)
	})
	for _, day := range days {
		if err := slotguard.LockCourtDay(ctx, tx, day.court, day.date); err != nil {
			return nil, err
		}
	}

	paymentIDs := make(map[uuid.UUID]uuid.UUID, len(ids))
	for _, id := range ids {
		row, err := qtx.GetBookingByIDForUpdate(ctx, db.GetBookingByIDForUpdateParams{
			ID:        data.UUIDToPg(id),
			ComplexID: data.UUIDToPg(complexID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, &ErrUnknownBooking{BookingID: id}
			}
			return nil, fmt.Errorf("sale: lock booking %s: %w", id, err)
		}
		booking := bookingstore.BookingFromDB(row)

		if booking.Status != "confirmed" && booking.Status != "completed" {
			return nil, ErrBookingNotPayable
		}
		if booking.CollectionStatus == bookingstore.CollectionStatusFullyPaid {
			return nil, ErrBookingAlreadyPaid
		}
		remaining := booking.BalanceDue()
		if remaining <= 0 {
			return nil, ErrBookingAlreadyPaid
		}
		if amounts[id] != remaining {
			return nil, ErrBookingBalanceMismatch
		}

		booking.RecordCounterPayment(amounts[id])
		payment := &paymentstore.Payment{
			BookingID: booking.ID,
			ComplexID: booking.ComplexID,
			Amount:    amounts[id],
			Method:    method,
			Status:    "fully_paid",
		}
		if err := m.Payments.ConfirmInTx(ctx, tx, qtx, payment, booking); err != nil {
			return nil, err
		}
		paymentIDs[id] = payment.ID
	}

	pids := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		pids = append(pids, paymentIDs[id])
	}
	rows, err := qtx.ListBookingPaymentLinesByPaymentIDs(ctx, db.ListBookingPaymentLinesByPaymentIDsParams{
		PaymentIds: data.UUIDSliceToPg(pids),
		ComplexID:  data.UUIDToPg(complexID),
	})
	if err != nil {
		return nil, fmt.Errorf("sale: read booking payment lines: %w", err)
	}
	byPayment := make(map[uuid.UUID]*BookingPaymentLine, len(rows))
	for _, r := range rows {
		line := bookingLineFromDB(r.PaymentID, r.BookingID, r.Amount, r.BookingDate, r.StartsAt, r.EndsAt, r.CourtName, r.ClientFirstName, r.ClientLastName)
		byPayment[line.PaymentID] = line
	}

	// Lines come back in request order, not lock order.
	lines := make([]*BookingPaymentLine, 0, len(requested))
	for _, r := range requested {
		line := byPayment[paymentIDs[r.BookingID]]
		if line == nil {
			return nil, fmt.Errorf("sale: booking payment line missing for booking %s", r.BookingID)
		}
		lines = append(lines, line)
		if sale != nil {
			if _, err := qtx.InsertSaleBookingPayment(ctx, db.InsertSaleBookingPaymentParams{
				ComplexID: data.UUIDToPg(complexID),
				SaleID:    data.UUIDToPg(sale.ID),
				BookingID: data.UUIDToPg(r.BookingID),
				PaymentID: data.UUIDToPg(line.PaymentID),
				//nolint:gosec // G115: amount is bounded by the remaining balance, far below int32 range.
				Amount: int32(line.Amount),
			}); err != nil {
				return nil, fmt.Errorf("sale: link booking payment: %w", err)
			}
		}
	}
	return lines, nil
}

// ListBookingPaymentsBySaleIDs returns, per sale, the booking balances it
// collected, in the order they were recorded. A sale with none has no key.
func (m *Store) ListBookingPaymentsBySaleIDs(ctx context.Context, complexID uuid.UUID, saleIDs []uuid.UUID) (map[uuid.UUID][]*BookingPaymentLine, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	out := make(map[uuid.UUID][]*BookingPaymentLine)
	if len(saleIDs) == 0 {
		return out, nil
	}
	rows, err := m.Q.ListSaleBookingPaymentsBySaleIDs(ctx, db.ListSaleBookingPaymentsBySaleIDsParams{
		SaleIds:   data.UUIDSliceToPg(saleIDs),
		ComplexID: data.UUIDToPg(complexID),
	})
	if err != nil {
		return nil, fmt.Errorf("sale: list booking payments by sale ids: %w", err)
	}
	for _, r := range rows {
		line := bookingLineFromDB(r.PaymentID, r.BookingID, r.Amount, r.BookingDate, r.StartsAt, r.EndsAt, r.CourtName, r.ClientFirstName, r.ClientLastName)
		saleID := data.PgToUUID(r.SaleID)
		out[saleID] = append(out[saleID], line)
	}
	return out, nil
}

// ListBalancesDue returns the complex's bookings of one day (an Argentina
// calendar day, as a DATE) that still owe money, sorted by start time. See the
// ListBookingBalancesDue query for the status rules.
func (m *Store) ListBalancesDue(ctx context.Context, complexID uuid.UUID, day time.Time) ([]*BalanceDue, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.Q.ListBookingBalancesDue(ctx, db.ListBookingBalancesDueParams{
		ComplexID:   data.UUIDToPg(complexID),
		BookingDate: data.DateToPg(day),
	})
	if err != nil {
		return nil, fmt.Errorf("sale: list bookings with a balance due: %w", err)
	}
	out := make([]*BalanceDue, 0, len(rows))
	for _, r := range rows {
		paid := 0
		switch r.CollectionStatus {
		case bookingstore.CollectionStatusFullyPaid:
			paid = int(r.Price)
		case bookingstore.CollectionStatusDepositPaid:
			paid = int(r.DepositAmount)
		case bookingstore.CollectionStatusUnpaid:
		}
		total := int(r.Price)
		out = append(out, &BalanceDue{
			BookingID:  data.PgToUUID(r.ID),
			Date:       data.PgToDate(r.BookingDate),
			StartsAt:   data.PgToTime(r.StartsAt),
			EndsAt:     data.PgToTime(r.EndsAt),
			CourtName:  r.CourtName,
			ClientName: clientName(r.ClientFirstName, r.ClientLastName),
			Status:     string(r.Status),
			Total:      total,
			Paid:       paid,
			Remaining:  total - paid,
		})
	}
	return out, nil
}

// bookingLineFromDB maps one booking-payment row of any of the readers above.
func bookingLineFromDB(paymentID, bookingID pgtype.UUID, amount int32, date pgtype.Date, startsAt, endsAt pgtype.Timestamptz, court, first, last string) *BookingPaymentLine {
	return &BookingPaymentLine{
		PaymentID:  data.PgToUUID(paymentID),
		BookingID:  data.PgToUUID(bookingID),
		Amount:     int(amount),
		Date:       data.PgToDate(date),
		StartsAt:   data.PgToTime(startsAt),
		EndsAt:     data.PgToTime(endsAt),
		CourtName:  court,
		ClientName: clientName(first, last),
	}
}

func clientName(first, last string) string {
	if last == "" {
		return first
	}
	return first + " " + last
}
