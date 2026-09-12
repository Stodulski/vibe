package data

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestUuidToPg(t *testing.T) {
	id := uuid.New()
	pg := UUIDToPg(id)

	if !pg.Valid {
		t.Fatal("UUIDToPg returned invalid UUID")
	}
	if pg.Bytes != id {
		t.Errorf("Bytes = %v, want %v", pg.Bytes, id)
	}
}

func TestPgToUUID(t *testing.T) {
	id := uuid.New()
	pg := pgtype.UUID{Bytes: id, Valid: true}

	got := PgToUUID(pg)
	if got != id {
		t.Errorf("PgToUUID = %v, want %v", got, id)
	}
}

func TestPgToUUID_Invalid(t *testing.T) {
	pg := pgtype.UUID{Valid: false}
	got := PgToUUID(pg)
	if got != uuid.Nil {
		t.Errorf("PgToUUID(invalid) = %v, want Nil", got)
	}
}

func TestUuidToPg_RoundTrip(t *testing.T) {
	id := uuid.New()
	got := PgToUUID(UUIDToPg(id))
	if got != id {
		t.Errorf("round-trip UUID = %v, want %v", got, id)
	}
}

func TestUuidPtrToPg_NonNil(t *testing.T) {
	id := uuid.New()
	pg := UUIDPtrToPg(&id)

	if !pg.Valid {
		t.Fatal("UUIDPtrToPg returned invalid UUID for non-nil ptr")
	}
	if pg.Bytes != id {
		t.Errorf("Bytes = %v, want %v", pg.Bytes, id)
	}
}

func TestUuidPtrToPg_Nil(t *testing.T) {
	pg := UUIDPtrToPg(nil)
	if pg.Valid {
		t.Error("UUIDPtrToPg(nil) should return invalid UUID")
	}
}

func TestPgToUUIDPtr_Valid(t *testing.T) {
	id := uuid.New()
	pg := pgtype.UUID{Bytes: id, Valid: true}

	got := PgToUUIDPtr(pg)
	if got == nil {
		t.Fatal("PgToUUIDPtr returned nil for valid UUID")
	}
	if *got != id {
		t.Errorf("PgToUUIDPtr = %v, want %v", *got, id)
	}
}

func TestPgToUUIDPtr_Invalid(t *testing.T) {
	pg := pgtype.UUID{Valid: false}
	got := PgToUUIDPtr(pg)
	if got != nil {
		t.Errorf("PgToUUIDPtr(invalid) = %v, want nil", got)
	}
}

func TestTimeToPg(t *testing.T) {
	now := time.Now().Truncate(time.Microsecond)
	pg := TimeToPg(now)

	if !pg.Valid {
		t.Fatal("TimeToPg returned invalid for non-zero time")
	}
	if !pg.Time.Equal(now) {
		t.Errorf("Time = %v, want %v", pg.Time, now)
	}
}

func TestTimeToPg_Zero(t *testing.T) {
	pg := TimeToPg(time.Time{})
	if pg.Valid {
		t.Error("TimeToPg(zero) should return invalid")
	}
}

func TestPgToTime_Valid(t *testing.T) {
	now := time.Now().Truncate(time.Microsecond)
	pg := pgtype.Timestamptz{Time: now, Valid: true}

	got := PgToTime(pg)
	if !got.Equal(now) {
		t.Errorf("PgToTime = %v, want %v", got, now)
	}
}

func TestPgToTime_Invalid(t *testing.T) {
	pg := pgtype.Timestamptz{Valid: false}
	got := PgToTime(pg)
	if !got.IsZero() {
		t.Errorf("PgToTime(invalid) = %v, want zero", got)
	}
}

func TestTimeToPg_RoundTrip(t *testing.T) {
	now := time.Now().Truncate(time.Microsecond)
	got := PgToTime(TimeToPg(now))
	if !got.Equal(now) {
		t.Errorf("round-trip time = %v, want %v", got, now)
	}
}

func TestDateToPg(t *testing.T) {
	d := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	pg := DateToPg(d)

	if !pg.Valid {
		t.Fatal("DateToPg returned invalid for non-zero date")
	}
	if !pg.Time.Equal(d) {
		t.Errorf("Time = %v, want %v", pg.Time, d)
	}
}

func TestDateToPg_Zero(t *testing.T) {
	pg := DateToPg(time.Time{})
	if pg.Valid {
		t.Error("DateToPg(zero) should return invalid")
	}
}

func TestPgToDate_Valid(t *testing.T) {
	d := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	pg := pgtype.Date{Time: d, Valid: true}

	got := PgToDate(pg)
	if !got.Equal(d) {
		t.Errorf("PgToDate = %v, want %v", got, d)
	}
}

func TestPgToDate_Invalid(t *testing.T) {
	pg := pgtype.Date{Valid: false}
	got := PgToDate(pg)
	if !got.IsZero() {
		t.Errorf("PgToDate(invalid) = %v, want zero", got)
	}
}

func TestDateToPg_RoundTrip(t *testing.T) {
	d := time.Date(2025, 12, 25, 0, 0, 0, 0, time.UTC)
	got := PgToDate(DateToPg(d))
	if !got.Equal(d) {
		t.Errorf("round-trip date = %v, want %v", got, d)
	}
}

func TestTextToPg_NonNil(t *testing.T) {
	s := "hello"
	pg := TextToPg(&s)

	if !pg.Valid {
		t.Fatal("TextToPg returned invalid for non-nil string")
	}
	if pg.String != "hello" {
		t.Errorf("String = %q, want %q", pg.String, "hello")
	}
}

func TestTextToPg_Nil(t *testing.T) {
	pg := TextToPg(nil)
	if pg.Valid {
		t.Error("TextToPg(nil) should return invalid")
	}
}

func TestPgToTextPtr_Valid(t *testing.T) {
	pg := pgtype.Text{String: "world", Valid: true}
	got := PgToTextPtr(pg)
	if got == nil {
		t.Fatal("PgToTextPtr returned nil for valid text")
	}
	if *got != "world" {
		t.Errorf("PgToTextPtr = %q, want %q", *got, "world")
	}
}

func TestPgToTextPtr_Invalid(t *testing.T) {
	pg := pgtype.Text{Valid: false}
	got := PgToTextPtr(pg)
	if got != nil {
		t.Errorf("PgToTextPtr(invalid) = %v, want nil", got)
	}
}

func TestTextToPg_RoundTrip(t *testing.T) {
	s := "test string"
	got := PgToTextPtr(TextToPg(&s))
	if got == nil || *got != s {
		t.Errorf("round-trip text = %v, want %q", got, s)
	}
}

func TestInt4ToPg(t *testing.T) {
	pg := Int4ToPg(42)
	if !pg.Valid {
		t.Fatal("Int4ToPg returned invalid")
	}
	if pg.Int32 != 42 {
		t.Errorf("Int32 = %d, want %d", pg.Int32, 42)
	}
}

func TestPgToInt_Valid(t *testing.T) {
	pg := pgtype.Int4{Int32: 99, Valid: true}
	got := PgToInt(pg)
	if got != 99 {
		t.Errorf("PgToInt = %d, want %d", got, 99)
	}
}

func TestPgToInt_Invalid(t *testing.T) {
	pg := pgtype.Int4{Valid: false}
	got := PgToInt(pg)
	if got != 0 {
		t.Errorf("PgToInt(invalid) = %d, want 0", got)
	}
}

func TestInt4ToPg_RoundTrip(t *testing.T) {
	got := PgToInt(Int4ToPg(123))
	if got != 123 {
		t.Errorf("round-trip int = %d, want %d", got, 123)
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
			name:         "14:00",
			microseconds: 14 * 3600 * 1_000_000,
			valid:        true,
			want:         "14:00",
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

func TestFloat8ToPg_NonNil(t *testing.T) {
	f := 3.14
	pg := Float8ToPg(&f)

	if !pg.Valid {
		t.Fatal("Float8ToPg returned invalid for non-nil pointer")
	}
	if pg.Float64 != 3.14 {
		t.Errorf("Float64 = %f, want %f", pg.Float64, 3.14)
	}
}

func TestFloat8ToPg_Nil(t *testing.T) {
	pg := Float8ToPg(nil)
	if pg.Valid {
		t.Error("Float8ToPg(nil) should return invalid")
	}
}

func TestPgToFloat8Ptr_Valid(t *testing.T) {
	pg := pgtype.Float8{Float64: 2.718, Valid: true}
	got := PgToFloat8Ptr(pg)
	if got == nil {
		t.Fatal("PgToFloat8Ptr returned nil for valid float")
	}
	if *got != 2.718 {
		t.Errorf("PgToFloat8Ptr = %f, want %f", *got, 2.718)
	}
}

func TestPgToFloat8Ptr_Invalid(t *testing.T) {
	pg := pgtype.Float8{Valid: false}
	got := PgToFloat8Ptr(pg)
	if got != nil {
		t.Errorf("PgToFloat8Ptr(invalid) = %v, want nil", got)
	}
}

func TestFloat8ToPg_RoundTrip(t *testing.T) {
	f := 99.99
	got := PgToFloat8Ptr(Float8ToPg(&f))
	if got == nil || *got != f {
		t.Errorf("round-trip float = %v, want %f", got, f)
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
			name:             "valid 14:00",
			input:            "14:00",
			wantValid:        true,
			wantMicroseconds: 14 * 3600 * 1_000_000,
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

func TestTimeStrToPg_RoundTrip(t *testing.T) {
	input := "15:45"
	got := PgToTimeStr(TimeStrToPg(input))
	if got != input {
		t.Errorf("round-trip timeStr = %q, want %q", got, input)
	}
}

func TestUuidSliceToPg(t *testing.T) {
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	pgs := UUIDSliceToPg(ids)

	if len(pgs) != len(ids) {
		t.Fatalf("len(result) = %d, want %d", len(pgs), len(ids))
	}
	for i, pg := range pgs {
		if !pg.Valid {
			t.Errorf("pgs[%d].Valid = false, want true", i)
		}
		if pg.Bytes != ids[i] {
			t.Errorf("pgs[%d].Bytes = %v, want %v", i, pg.Bytes, ids[i])
		}
	}
}

func TestUuidSliceToPg_Empty(t *testing.T) {
	pgs := UUIDSliceToPg([]uuid.UUID{})
	if len(pgs) != 0 {
		t.Errorf("len(result) = %d, want 0", len(pgs))
	}
}

func TestUuidSliceToPg_Nil(t *testing.T) {
	pgs := UUIDSliceToPg(nil)
	if len(pgs) != 0 {
		t.Errorf("len(result) = %d, want 0", len(pgs))
	}
}
