package store

import (
	"errors"
	"testing"

	"github.com/stodulski/vibe-server/internal/mpcred"
)

func TestComplex_SellerAccessToken(t *testing.T) {
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
			c := &Complex{mpAccessToken: tt.token}
			got, err := c.SellerAccessToken()

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

func TestComplex_SellerRefreshToken(t *testing.T) {
	value := "seller-refresh-token"
	empty := ""

	tests := []struct {
		name    string
		token   *string
		wantVal string
		wantErr error
	}{
		{name: "nil pointer", token: nil, wantErr: mpcred.ErrMPNotConnected},
		{name: "pointer to empty string", token: &empty, wantErr: mpcred.ErrMPNotConnected},
		{name: "pointer to value", token: &value, wantVal: "seller-refresh-token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Complex{mpRefreshToken: tt.token}
			got, err := c.SellerRefreshToken()

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

func TestComplex_MPConnected(t *testing.T) {
	value := "seller-access-token"
	empty := ""

	tests := []struct {
		name string
		c    *Complex
		want bool
	}{
		{name: "nil access token", c: &Complex{mpAccessToken: nil}, want: false},
		{name: "empty access token", c: &Complex{mpAccessToken: &empty}, want: false},
		{name: "non-empty access token", c: &Complex{mpAccessToken: &value}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.MPConnected(); got != tt.want {
				t.Errorf("MPConnected() = %v; want %v", got, tt.want)
			}
		})
	}
}
