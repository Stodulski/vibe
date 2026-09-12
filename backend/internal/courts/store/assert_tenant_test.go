package store

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
)

func TestTheCourtWritesAssertTheirTenant(t *testing.T) {
	own, other := uuid.New(), uuid.New()
	store := &Store{}

	c := &Court{ComplexID: own}

	if err := store.Insert(data.ContextWithTenant(context.Background(), other), c); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("Insert for another tenant: want ErrRecordNotFound; got %v", err)
	}
	if err := store.Insert(context.Background(), c); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("Insert with no tenant on the context: want ErrRecordNotFound; got %v", err)
	}
	if err := store.Update(data.ContextWithTenant(context.Background(), other), c, nil); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("Update for another tenant: want ErrRecordNotFound; got %v", err)
	}
}
