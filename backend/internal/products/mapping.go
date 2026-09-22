package products

import (
	"github.com/stodulski/vibe-server/internal/openapi/gen"
	productstore "github.com/stodulski/vibe-server/internal/products/store"
)

// toGenProduct maps a store product onto the generated wire type (rule
// HTTP-08), including the two computed flags (LowStock, NeedsStockReview)
// productstore.Product itself derives from its own fields.
func toGenProduct(p *productstore.Product) gen.Product {
	version := p.Version
	return gen.Product{
		Id:                p.ID,
		ComplexId:         p.ComplexID,
		Name:              p.Name,
		Category:          p.Category,
		Price:             p.Price,
		TracksStock:       p.TracksStock,
		StockOnHand:       p.StockOnHand,
		LowStockThreshold: p.LowStockThreshold,
		Active:            p.Active,
		LowStock:          p.LowStock(),
		NeedsStockReview:  p.NeedsStockReview(),
		CreatedAt:         p.CreatedAt,
		UpdatedAt:         p.UpdatedAt,
		// gen.Product.Version is *int with omitempty, but the store's Version
		// is always set — same reasoning as courts' own toGenCourt.
		Version: &version,
	}
}

// toGenStockMovement maps a store stock movement onto the generated wire type.
func toGenStockMovement(m *productstore.StockMovement) gen.StockMovement {
	var reason *gen.StockMovementReason
	if m.Reason != nil {
		r := gen.StockMovementReason(*m.Reason)
		reason = &r
	}
	return gen.StockMovement{
		Id:             m.ID,
		ComplexId:      m.ComplexID,
		ProductId:      m.ProductID,
		Kind:           gen.StockMovementKind(m.Kind),
		Quantity:       m.Quantity,
		Reason:         reason,
		Note:           m.Note,
		CashMovementId: m.CashMovementID,
		SaleId:         m.SaleID,
		CreatedAt:      m.CreatedAt,
		CreatedBy:      m.CreatedBy,
	}
}
