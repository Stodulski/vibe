package data_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
)

// The store's own authorization check. Every guarantee today comes from the
// HTTP chain — RequireComplexOwner, RequireRole — or from the row-level
// security policies downstream of it, so a caller that does not arrive through
// that chain is authorized by nothing it can see. This is what it sees now.
func TestAssertTenant(t *testing.T) {
	own, other := uuid.New(), uuid.New()

	tests := []struct {
		name    string
		ctx     context.Context
		row     uuid.UUID
		wantErr bool
	}{
		{
			name: "the context is acting for this row's tenant",
			ctx:  data.ContextWithTenant(context.Background(), own),
			row:  own,
		},
		{
			name:    "the context is acting for another tenant",
			ctx:     data.ContextWithTenant(context.Background(), own),
			row:     other,
			wantErr: true,
		},
		{
			// The bypass is not an absence of authorization: it is a declared
			// list of routes with a reason each, and the sweeps and the webhook
			// genuinely span tenants.
			name: "the context carries the cross-tenant bypass",
			ctx:  data.ContextWithTenantBypass(context.Background()),
			row:  other,
		},
		{
			// The case this whole check exists for: nobody scoped the caller,
			// so nobody authorized it.
			name:    "the context carries neither",
			ctx:     context.Background(),
			row:     own,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := data.AssertTenant(tt.ctx, tt.row)
			if tt.wantErr {
				// ErrRecordNotFound rather than a permission error, matching
				// what the handlers already answer for a complex the caller
				// does not own: a 403 would confirm the row exists.
				if !errors.Is(err, data.ErrRecordNotFound) {
					t.Fatalf("want ErrRecordNotFound; got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
