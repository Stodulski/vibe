package store

import (
	"errors"
	"testing"

	"github.com/stodulski/vibe-server/internal/mpcred"
)

func TestCronBooking_SellerAccessToken(t *testing.T) {
	value := "seller-access-token"
	empty := ""

	tests := []struct {
		name    string
		token   *string
		wantVal string
		wantErr error
	}{
		{name: "nil pointer", token: nil, wantErr: mpcred.ErrMPNotConnected},
		{name: "pointer to empty string", token: &empty, wantErr: mpcred.ErrMPNotConnected},
		{name: "pointer to value", token: &value, wantVal: "seller-access-token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &CronBooking{mpAccessToken: tt.token}
			got, err := b.SellerAccessToken()

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("want error %v; got %v", tt.wantErr, err)
				}
				if got != "" {
					t.Errorf("want empty value on error; got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantVal {
				t.Errorf("want %q; got %q", tt.wantVal, got)
			}
		})
	}
}
