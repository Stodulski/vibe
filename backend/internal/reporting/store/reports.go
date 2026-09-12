package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stodulski/vibe-server/internal/data"
)

// argentinaTZ is the zone the reporting period is anchored to. Payments are
// stored in UTC, but an owner's "March" is March in Buenos Aires, so the date
// comparison has to happen in local time.
const argentinaTZ = "America/Argentina/Buenos_Aires"

// PaymentMethodSummary is one row of the monthly report: everything taken
// through a single payment method over the period.
type PaymentMethodSummary struct {
	Method     string `json:"method"`
	Count      int    `json:"count"`
	Amount     int    `json:"amount"`
	ServiceFee int    `json:"service_fee"`
	Refunded   int    `json:"refunded"`
}

// PaymentDetail is one payment as it appears in the exported spreadsheet,
// joined to the booking it paid for.
type PaymentDetail struct {
	CreatedAt time.Time
	// StartsAt and EndsAt are the booked hours as absolute instants, read off
	// bookings.span. They were a pair of clock strings until bookings.end_time was dropped
	// dropped bookings.end_time: an owner exporting a month with a 23:00
	// booking in it read "23:00 - 01:00" in the Horario column, with the row
	// sorted by payment date and nothing saying the play ran into the next day.
	StartsAt      time.Time
	EndsAt        time.Time
	CourtName     string
	ClientName    string
	ClientPhone   string
	BookingPrice  int
	Amount        int
	ServiceFee    int
	RefundAmount  int
	Method        string
	PaymentStatus string
	BookingStatus string
}

// ReportReader provides the aggregate reads behind the monthly report and its
// spreadsheet export.
type ReportReader interface {
	PaymentSummaryByMethod(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]PaymentMethodSummary, error)
	PaymentSummaryByCourt(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]PaymentCourtSummary, error)
	PaymentDetails(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]PaymentDetail, error)
}

// ReportStore is the full reporting interface.
type ReportStore interface {
	ReportReader
}

// Store implements ReportStore against PostgreSQL.
//
// These two queries are aggregates spanning payments, bookings, courts and
// clients. They live here rather than in a handler because handlers depend on
// store interfaces, never on the connection pool — the export and the summary
// endpoint previously issued this SQL themselves, and the summary query was
// written out twice.
type Store struct {
	DB *data.DB
}

// countedPaymentStatuses are the payment states that belong in a revenue
// report: money that arrived, and money that was sent back. Pending and failed
// payments are excluded because they never moved.
const countedPaymentStatuses = `('deposit_paid', 'fully_paid', 'refunded', 'refund_pending')`

// PaymentSummaryByMethod totals the period's payments per method.
func (m *Store) PaymentSummaryByMethod(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]PaymentMethodSummary, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.DB.Query(ctx, `
		SELECT p.method::text,
		       COUNT(*)::int,
		       COALESCE(SUM(p.amount), 0)::bigint,
		       COALESCE(SUM(p.service_fee), 0)::bigint,
		       COALESCE(SUM(p.refund_amount), 0)::bigint
		FROM payments p
		WHERE p.complex_id = $1
		  AND p.status IN `+countedPaymentStatuses+`
		  AND (p.created_at AT TIME ZONE '`+argentinaTZ+`')::date
		      BETWEEN $2 AND $3
		GROUP BY p.method`,
		complexID, from.Format("2006-01-02"), to.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []PaymentMethodSummary
	for rows.Next() {
		var s PaymentMethodSummary
		if err := rows.Scan(&s.Method, &s.Count, &s.Amount, &s.ServiceFee, &s.Refunded); err != nil {
			return nil, err
		}
		summaries = append(summaries, s)
	}

	return summaries, rows.Err()
}

// PaymentCourtSummary is one row of the per-court breakdown: everything taken
// on a single court over the period.
//
// CourtName is what the court was called when the report ran, and it is empty
// for a payment whose court has since been hard-deleted — the join is LEFT so
// the money still shows up rather than vanishing with the court.
type PaymentCourtSummary struct {
	CourtID    string `json:"court_id"`
	CourtName  string `json:"court_name"`
	Count      int    `json:"count"`
	Amount     int    `json:"amount"`
	ServiceFee int    `json:"service_fee"`
	Refunded   int    `json:"refunded"`
}

// PaymentSummaryByCourt totals the period's payments per court.
//
// The same query as PaymentSummaryByMethod with the grouping moved, and
// deliberately so: both answer "what came in", so they must share the status
// filter and the date predicate or the two breakdowns of one month would not
// add up to the same number.
//
// Ranged over payments.created_at, not over the booking's span. This is a
// money question — a court played in July and paid for in August earned that
// money in August, and reading it any other way would put the per-court rows
// at odds with every other figure on the report.
func (m *Store) PaymentSummaryByCourt(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]PaymentCourtSummary, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.DB.Query(ctx, `
		SELECT COALESCE(b.court_id::text, ''),
		       COALESCE(co.name, ''),
		       COUNT(*)::int,
		       COALESCE(SUM(p.amount), 0)::bigint,
		       COALESCE(SUM(p.service_fee), 0)::bigint,
		       COALESCE(SUM(p.refund_amount), 0)::bigint
		FROM payments p
		JOIN bookings b ON b.id = p.booking_id
		LEFT JOIN courts co ON co.id = b.court_id
		WHERE p.complex_id = $1
		  AND p.status IN `+countedPaymentStatuses+`
		  AND (p.created_at AT TIME ZONE '`+argentinaTZ+`')::date
		      BETWEEN $2 AND $3
		GROUP BY b.court_id, co.name
		ORDER BY SUM(p.amount) DESC`,
		complexID, from.Format("2006-01-02"), to.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []PaymentCourtSummary
	for rows.Next() {
		var s PaymentCourtSummary
		if err := rows.Scan(&s.CourtID, &s.CourtName, &s.Count, &s.Amount, &s.ServiceFee, &s.Refunded); err != nil {
			return nil, err
		}
		summaries = append(summaries, s)
	}

	return summaries, rows.Err()
}

// PaymentDetails returns every payment in the period with its booking, court
// and client, ordered oldest first so the spreadsheet reads chronologically.
//
// Unlike the summary it counts every status, because the export is a ledger:
// an owner reconciling their month needs to see the failed attempts too.
func (m *Store) PaymentDetails(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]PaymentDetail, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.DB.Query(ctx, `
		SELECT p.created_at, lower(b.span), upper(b.span),
		       COALESCE(co.name, ''),
		       COALESCE(cl.first_name || ' ' || cl.last_name, ''),
		       COALESCE(cl.phone, ''),
		       b.price, p.amount, p.service_fee, p.refund_amount,
		       p.method::text, p.status::text, b.status::text
		FROM payments p
		JOIN bookings b ON b.id = p.booking_id
		LEFT JOIN courts co ON co.id = b.court_id
		LEFT JOIN clients cl ON cl.id = b.client_id
		WHERE p.complex_id = $1
		  AND (p.created_at AT TIME ZONE '`+argentinaTZ+`')::date
		      BETWEEN $2 AND $3
		ORDER BY p.created_at ASC`,
		complexID, from.Format("2006-01-02"), to.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var details []PaymentDetail
	for rows.Next() {
		var d PaymentDetail
		err := rows.Scan(
			&d.CreatedAt, &d.StartsAt, &d.EndsAt,
			&d.CourtName, &d.ClientName, &d.ClientPhone,
			&d.BookingPrice, &d.Amount, &d.ServiceFee, &d.RefundAmount,
			&d.Method, &d.PaymentStatus, &d.BookingStatus,
		)
		if err != nil {
			return nil, err
		}
		details = append(details, d)
	}

	return details, rows.Err()
}
