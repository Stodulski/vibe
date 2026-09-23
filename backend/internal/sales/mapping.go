package sales

import (
	"github.com/stodulski/vibe-server/internal/openapi/gen"
	salestore "github.com/stodulski/vibe-server/internal/sales/store"
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
