package store

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/stodulski/vibe-server/internal/data"
)

// The refusal a client can act on depends on this mapping, and the code that
// carries it moved: the exclusion constraint retired idx_bookings_no_double, so a
// double booking is now refused by an exclusion constraint (23P01) rather than
// a unique index (23505). Both are checked, against the SQLSTATE PostgreSQL
// actually emits for each.
func TestIsSlotAlreadySold(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"exclusion violation — what bookings_no_overlapping_span raises today", &pgconn.PgError{Code: "23P01"}, true},
		{"unique violation — what the dropped index used to raise", &pgconn.PgError{Code: "23505"}, true},
		{"wrapped, because callers see it through fmt.Errorf", fmt.Errorf("insert: %w", &pgconn.PgError{Code: "23P01"}), true},
		{"a check constraint is a different refusal and must surface", &pgconn.PgError{Code: "23514"}, false},
		{"a foreign key violation is not a taken slot", &pgconn.PgError{Code: "23503"}, false},
		{"a plain error is not a database refusal at all", errors.New("connection reset"), false},
		{"no error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSlotAlreadySold(tt.err); got != tt.want {
				t.Errorf("isSlotAlreadySold = %v, want %v", got, tt.want)
			}
		})
	}
}

// The store refuses a write whose row belongs to another tenant, and one whose
// caller declared no tenant at all. Neither refusal depends on the HTTP chain
// having run, which is the point: the chain is where every other check lives.
func TestTheBookingWritesAssertTheirTenant(t *testing.T) {
	own, other := uuid.New(), uuid.New()
	store := &Store{}

	b := &Booking{ComplexID: own}

	if err := store.Insert(data.ContextWithTenant(context.Background(), other), b); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("Insert for another tenant: want ErrRecordNotFound; got %v", err)
	}
	if err := store.Insert(context.Background(), b); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("Insert with no tenant on the context: want ErrRecordNotFound; got %v", err)
	}
	if err := store.Update(data.ContextWithTenant(context.Background(), other), b); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("Update for another tenant: want ErrRecordNotFound; got %v", err)
	}
	if err := store.InsertSafe(data.ContextWithTenant(context.Background(), other), b); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("InsertSafe for another tenant: want ErrRecordNotFound; got %v", err)
	}
	if err := store.CancelFutureByComplex(data.ContextWithTenant(context.Background(), other), own); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("CancelFutureByComplex for another tenant: want ErrRecordNotFound; got %v", err)
	}
}

// Valid answers "could the column hold this", which is what separates a value
// the database would refuse from one only a particular caller may not set.
func TestTheStatusTypesKnowTheirOwnVocabulary(t *testing.T) {
	for _, s := range []BookingStatus{
		BookingStatusPending, BookingStatusConfirmed, BookingStatusCancelled,
		BookingStatusCompleted, BookingStatusNoShow,
	} {
		if !s.Valid() {
			t.Errorf("BookingStatus %q is one of the five the column accepts", s)
		}
	}
	for _, s := range []BookingStatus{"", "Confirmed", "confirmed ", "unpaid", "none"} {
		if s.Valid() {
			t.Errorf("BookingStatus %q is not a booking status", s)
		}
	}

	for _, s := range []CollectionStatus{
		CollectionStatusUnpaid, CollectionStatusDepositPaid, CollectionStatusFullyPaid,
	} {
		if !s.Valid() {
			t.Errorf("CollectionStatus %q is one of the three the column accepts", s)
		}
	}
	// The cross-vocabulary case the types exist for: 'confirmed' is a real
	// value of a different column, and a bare string field accepted it.
	for _, s := range []CollectionStatus{"", "confirmed", "none", "partial"} {
		if s.Valid() {
			t.Errorf("CollectionStatus %q is not a collection status", s)
		}
	}

	for _, s := range []RefundStatus{
		RefundStatusNone, RefundStatusPending, RefundStatusPartial, RefundStatusFull,
	} {
		if !s.Valid() {
			t.Errorf("RefundStatus %q is one of the four the column accepts", s)
		}
	}
	for _, s := range []RefundStatus{"", "unpaid", "confirmed", "refunded"} {
		if s.Valid() {
			t.Errorf("RefundStatus %q is not a refund status", s)
		}
	}
}
