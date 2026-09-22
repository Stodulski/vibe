package store

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestTranslateProductWriteMapsTheActiveNameUniqueConstraint(t *testing.T) {
	err := translateProductWrite(&pgconn.PgError{ConstraintName: productsActiveNameUnique})
	if !errors.Is(err, ErrDuplicateProductName) {
		t.Errorf("want ErrDuplicateProductName; got %v", err)
	}
}

func TestTranslateProductWriteLeavesAnUnknownConstraintUnchanged(t *testing.T) {
	original := &pgconn.PgError{ConstraintName: "some_other_constraint"}
	if got := translateProductWrite(original); got != error(original) { //nolint:errorlint // exact identity is the point: an unrecognised error must pass through unwrapped.
		t.Errorf("want the original error returned unchanged; got %v", got)
	}
}

func TestTranslateProductWritePassesThroughANonPgError(t *testing.T) {
	plain := errors.New("boom")
	if got := translateProductWrite(plain); !errors.Is(got, plain) {
		t.Errorf("want the original error; got %v", got)
	}
}

func TestProductLowStock(t *testing.T) {
	threshold := 5

	tests := []struct {
		name string
		p    Product
		want bool
	}{
		{"tracks stock, at threshold", Product{TracksStock: true, StockOnHand: 5, LowStockThreshold: &threshold}, true},
		{"tracks stock, below threshold", Product{TracksStock: true, StockOnHand: 2, LowStockThreshold: &threshold}, true},
		{"tracks stock, above threshold", Product{TracksStock: true, StockOnHand: 10, LowStockThreshold: &threshold}, false},
		{"tracks stock, no threshold set", Product{TracksStock: true, StockOnHand: 0, LowStockThreshold: nil}, false},
		{"does not track stock, would otherwise be low", Product{TracksStock: false, StockOnHand: 0, LowStockThreshold: &threshold}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.LowStock(); got != tt.want {
				t.Errorf("LowStock() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProductNeedsStockReview(t *testing.T) {
	if (&Product{StockOnHand: 0}).NeedsStockReview() {
		t.Error("zero stock must not need review")
	}
	if (&Product{StockOnHand: 5}).NeedsStockReview() {
		t.Error("positive stock must not need review")
	}
	if !(&Product{StockOnHand: -1}).NeedsStockReview() {
		t.Error("negative stock must need review")
	}
}
