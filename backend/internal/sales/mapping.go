package sales

import (
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/stodulski/vibe-server/internal/openapi/gen"
	salestore "github.com/stodulski/vibe-server/internal/sales/store"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// toGenSale maps a sale together with its items onto the generated wire type.
func toGenSale(s *SaleWithItems) gen.Sale {
	items := make([]gen.SaleItem, len(s.Items))
	for i, item := range s.Items {
		items[i] = toGenSaleItem(item)
	}
	return gen.Sale{
		Id:                 s.ID,
		ComplexId:          s.ComplexID,
		SessionId:          s.SessionID,
		Method:             gen.SaleMethod(s.Method),
		Total:              s.Total,
		CashMovementId:     s.CashMovementID,
		VoidedAt:           s.VoidedAt,
		VoidedBy:           s.VoidedBy,
		VoidCashMovementId: s.VoidCashMovementID,
		Items:              items,
		BookingPayments:    toGenSaleBookingPayments(s.BookingPayments),
		CreatedAt:          s.CreatedAt,
		CreatedBy:          s.CreatedBy,
	}
}

// toGenSaleItem maps a store sale item onto the generated wire type.
func toGenSaleItem(i *salestore.SaleItem) gen.SaleItem {
	return gen.SaleItem{
		Id:          i.ID,
		ComplexId:   i.ComplexID,
		SaleId:      i.SaleID,
		ProductId:   i.ProductID,
		ProductName: i.ProductName,
		UnitPrice:   i.UnitPrice,
		Quantity:    i.Quantity,
		LineTotal:   i.LineTotal,
	}
}

// toGenStockWarning maps a store stock warning onto the generated wire type.
func toGenStockWarning(w salestore.StockWarning) gen.SaleStockWarning {
	return gen.SaleStockWarning{
		ProductId:   w.ProductID,
		ProductName: w.ProductName,
		StockOnHand: w.StockOnHand,
	}
}

// toGenSaleBookingPayments maps the booking balances of a checkout. It is never
// nil: the wire field is a required array, empty when there were none.
func toGenSaleBookingPayments(lines []*salestore.BookingPaymentLine) []gen.SaleBookingPayment {
	out := make([]gen.SaleBookingPayment, len(lines))
	for i, l := range lines {
		out[i] = gen.SaleBookingPayment{
			PaymentId:  l.PaymentID,
			BookingId:  l.BookingID,
			Amount:     l.Amount,
			CourtName:  l.CourtName,
			ClientName: l.ClientName,
			Date:       openapi_types.Date{Time: l.Date},
			StartTime:  clockOf(l.StartsAt),
			EndTime:    clockOf(l.EndsAt),
		}
	}
	return out
}

// toGenBalanceDue maps one booking that still owes money.
func toGenBalanceDue(b *salestore.BalanceDue) gen.BalanceDue {
	return gen.BalanceDue{
		BookingId:  b.BookingID,
		ClientName: b.ClientName,
		CourtName:  b.CourtName,
		Date:       openapi_types.Date{Time: b.Date},
		StartTime:  clockOf(b.StartsAt),
		EndTime:    clockOf(b.EndsAt),
		Status:     gen.BookingStatus(b.Status),
		Total:      b.Total,
		Paid:       b.Paid,
		Remaining:  b.Remaining,
	}
}

// clockOf renders an instant as HH:MM on the Argentina wall clock, the way the
// sell page and a receipt show a booking's times.
func clockOf(t time.Time) string {
	return t.In(timezone.Argentina).Format("15:04")
}
