package store

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
)

// HubComplex is one live, switched-on complex as a city hub lists it: the public
// fields a listing shows, and the sports of its switched-on courts.
type HubComplex struct {
	ID        uuid.UUID
	Name      string
	Slug      string
	Address   string
	City      string
	IsActive  bool
	Sports    []string
	UpdatedAt time.Time
}

// ListActiveComplexesForHubs returns every live, switched-on complex, ordered by
// city then name. Callers group the rows by city; see db/queries/hubs.sql for
// why the city match is not done in SQL.
func (m *Store) ListActiveComplexesForHubs(ctx context.Context) ([]HubComplex, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.Q.ListActiveComplexesForHubs(ctx)
	if err != nil {
		return nil, err
	}

	complexes := make([]HubComplex, 0, len(rows))
	for _, row := range rows {
		complexes = append(complexes, HubComplex{
			ID:        data.PgToUUID(row.ID),
			Name:      row.Name,
			Slug:      row.Slug,
			Address:   row.Address,
			City:      row.City,
			IsActive:  row.IsActive,
			Sports:    row.Sports,
			UpdatedAt: data.PgToTime(row.UpdatedAt),
		})
	}
	return complexes, nil
}
