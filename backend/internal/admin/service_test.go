package admin

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
)

// TestServiceToggleUserActive covers the one write this module allows and the
// rule that guards it: an operator may not change their own account's active
// flag, because an operator who deactivates themselves locks the platform's
// administrative account out of the platform.
func TestServiceToggleUserActive(t *testing.T) {
	operatorID := uuid.New()
	targetID := uuid.New()

	tests := []struct {
		name         string
		target       uuid.UUID
		isActive     bool
		toggleErr    error
		wantErr      error
		wantAudited  bool
		wantInvalid  bool
		wantToggleTo *bool
	}{
		{
			name:        "another account is toggled, audited and uncached",
			target:      targetID,
			isActive:    false,
			wantAudited: true,
			wantInvalid: true,
		},
		{
			name:    "the operator's own account is refused",
			target:  operatorID,
			wantErr: ErrSelfToggle,
		},
		{
			name:      "an account that does not exist is passed through",
			target:    targetID,
			toggleErr: data.ErrRecordNotFound,
			wantErr:   data.ErrRecordNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &stubStore{toggleErr: tt.toggleErr}
			cache := &stubCache{}
			recorder := &stubRecorder{}
			svc := NewService(store, store, cache, recorder)

			actor := Actor{UserID: &operatorID, IP: "1.2.3.4"}
			err := svc.ToggleUserActive(t.Context(), actor, tt.target, tt.isActive)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("got error %v, want %v", err, tt.wantErr)
				}
				if len(recorder.entries) != 0 {
					t.Error("a refused toggle still wrote an audit entry")
				}
				if len(cache.invalidated) != 0 {
					t.Error("a refused toggle still invalidated a cached user")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantAudited {
				if len(recorder.entries) != 1 {
					t.Fatalf("got %d audit entries, want 1", len(recorder.entries))
				}
				e := recorder.entries[0]
				if e.Action != "toggle_active" || e.EntityType != "user" {
					t.Errorf("audit entry is %s/%s, want toggle_active/user", e.Action, e.EntityType)
				}
				if e.UserID == nil || *e.UserID != operatorID || e.IPAddress != "1.2.3.4" {
					t.Error("the audit entry does not name the operator the handler read")
				}
			}
			if tt.wantInvalid && len(cache.invalidated) != 1 {
				t.Errorf("got %d cache invalidations, want 1 — a stale copy keeps a deactivated account working", len(cache.invalidated))
			}
		})
	}
}
