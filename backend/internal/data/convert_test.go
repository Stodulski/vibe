package data

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestTimeToPg_Zero(t *testing.T) {
	pg := TimeToPg(time.Time{})
	if pg.Valid {
		t.Error("TimeToPg(zero) should return invalid")
	}
}

func TestDateToPg_Zero(t *testing.T) {
	pg := DateToPg(time.Time{})
	if pg.Valid {
		t.Error("DateToPg(zero) should return invalid")
	}
}

func TestPgToTimeStr(t *testing.T) {
	tests := []struct {
		name         string
		microseconds int64
		valid        bool
		want         string
	}{
		{
			name:         "9:30 AM",
			microseconds: (9*3600 + 30*60) * 1_000_000,
			valid:        true,
			want:         "09:30",
		},
		{
			name:         "midnight",
			microseconds: 0,
			valid:        true,
			want:         "00:00",
		},
		{
			name:         "23:59",
			microseconds: (23*3600 + 59*60) * 1_000_000,
			valid:        true,
			want:         "23:59",
		},
		{
			name:  "invalid returns empty",
			valid: false,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pg := pgtype.Time{Microseconds: tt.microseconds, Valid: tt.valid}
			got := PgToTimeStr(pg)
			if got != tt.want {
				t.Errorf("PgToTimeStr = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTimeStrToPg(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		wantValid        bool
		wantMicroseconds int64
	}{
		{
			name:             "valid 09:30",
			input:            "09:30",
			wantValid:        true,
			wantMicroseconds: (9*3600 + 30*60) * 1_000_000,
		},
		{
			name:             "valid 00:00",
			input:            "00:00",
			wantValid:        true,
			wantMicroseconds: 0,
		},
		{
			name:             "valid 23:59",
			input:            "23:59",
			wantValid:        true,
			wantMicroseconds: (23*3600 + 59*60) * 1_000_000,
		},
		{
			name:      "invalid empty string",
			input:     "",
			wantValid: false,
		},
		{
			name:      "invalid format",
			input:     "abc",
			wantValid: false,
		},
		{
			name:      "invalid single number",
			input:     "14",
			wantValid: false,
		},
		{
			name:      "invalid separator",
			input:     "14-30",
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pg := TimeStrToPg(tt.input)
			if pg.Valid != tt.wantValid {
				t.Errorf("Valid = %v, want %v", pg.Valid, tt.wantValid)
			}
			if tt.wantValid && pg.Microseconds != tt.wantMicroseconds {
				t.Errorf("Microseconds = %d, want %d", pg.Microseconds, tt.wantMicroseconds)
			}
		})
	}
}
