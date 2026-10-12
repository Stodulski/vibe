package sales

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	salestore "github.com/stodulski/vibe-server/internal/sales/store"
)

// A balance-only checkout's list entry names no cash movement: the wire field is
// present and null, never a zero UUID that a client would read as a real movement.
func TestToGenSaleBalanceOnlyEntryNamesNoCashMovement(t *testing.T) {
	entry := &SaleWithItems{Sale: &salestore.Sale{
		ID:        uuid.New(),
		Method:    "transfer",
		Total:     350_000,
		CreatedAt: time.Date(2026, 10, 12, 18, 0, 0, 0, time.UTC),
	}}

	body, err := json.Marshal(toGenSale(entry))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(body), `"cash_movement_id":null`) {
		t.Errorf("balance-only entry JSON = %s; want cash_movement_id null", body)
	}
}
