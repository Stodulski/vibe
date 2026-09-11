package validator

import (
	"testing"
)

func TestNormalizePhone(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "valid Argentine phone",
			input:   "+5491112345678",
			want:    "+5491112345678",
			wantErr: false,
		},
		{
			name:    "valid with spaces trimmed",
			input:   "  +5491112345678  ",
			want:    "+5491112345678",
			wantErr: false,
		},
		{
			name:    "valid with dashes and spaces stripped",
			input:   "+54 9 11-1234-5678",
			want:    "+5491112345678",
			wantErr: false,
		},
		{
			name:    "valid with parens stripped",
			input:   "+54(911)12345678",
			want:    "+5491112345678",
			wantErr: false,
		},
		{
			name:    "invalid letters",
			input:   "abc",
			want:    "",
			wantErr: true,
		},
		{
			name:    "too short",
			input:   "+123",
			want:    "",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			want:    "",
			wantErr: true,
		},
		{
			name:    "missing plus prefix",
			input:   "5491112345678",
			want:    "",
			wantErr: true,
		},
		{
			name:    "starts with zero after plus",
			input:   "+0491112345678",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizePhone(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NormalizePhone(%q) expected error, got nil (result: %q)", tt.input, got)
				}
				return
			}

			if err != nil {
				t.Errorf("NormalizePhone(%q) unexpected error: %v", tt.input, err)
				return
			}

			if got != tt.want {
				t.Errorf("NormalizePhone(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestE164RX(t *testing.T) {
	tests := []struct {
		name  string
		phone string
		want  bool
	}{
		{"valid E.164", "+5491112345678", true},
		{"valid short E.164", "+12345678", true},
		{"valid max length", "+123456789012345", true},
		{"missing plus", "5491112345678", false},
		{"too short after plus", "+1234567", false},
		{"too long", "+1234567890123456", false},
		{"starts with zero", "+0123456789", false},
		{"has letters", "+54abc12345678", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := E164RX.MatchString(tt.phone)
			if got != tt.want {
				t.Errorf("E164RX.MatchString(%q) = %v, want %v", tt.phone, got, tt.want)
			}
		})
	}
}
