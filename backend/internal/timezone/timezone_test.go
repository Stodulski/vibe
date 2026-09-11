package timezone

import (
	"testing"
	"time"
)

func TestArgentinaIsLoaded(t *testing.T) {
	if Argentina == time.UTC {
		t.Skip("system tzdata is missing; the fallback is in use")
	}

	// Argentina has been at UTC-3 year round since 2009.
	_, offset := time.Date(2026, 1, 15, 12, 0, 0, 0, Argentina).Zone()
	if offset != -3*3600 {
		t.Errorf("want a -03:00 offset in January; got %d seconds", offset)
	}
	_, offset = time.Date(2026, 7, 15, 12, 0, 0, 0, Argentina).Zone()
	if offset != -3*3600 {
		t.Errorf("want a -03:00 offset in July too, there is no DST; got %d seconds", offset)
	}
}

func TestTodayZeroesTheTimeOfDay(t *testing.T) {
	today := Today()

	if h, m, s := today.Clock(); h != 0 || m != 0 || s != 0 {
		t.Errorf("want midnight; got %02d:%02d:%02d", h, m, s)
	}
	if today.Location() != Argentina {
		t.Errorf("want the Argentine location; got %v", today.Location())
	}
}

// Today must agree with Now about which day it is, which is the whole reason
// the zone matters: near midnight UTC the two disagree by a calendar day.
func TestTodayMatchesNowsCalendarDay(t *testing.T) {
	now, today := Now(), Today()

	if y, m, d := now.Date(); today.Year() != y || today.Month() != m || today.Day() != d {
		t.Errorf("Today is %v but Now is %v", today.Format("2006-01-02"), now.Format("2006-01-02"))
	}
}

// The whole point of ParseDay: the same calendar date, anchored here rather
// than in UTC, is a different instant — and the difference is a day's worth of
// wrong for the first three hours of every morning.
func TestParseDayAnchorsMidnightLocally(t *testing.T) {
	got, err := ParseDay("2026-08-28")
	if err != nil {
		t.Fatalf("ParseDay: %v", err)
	}

	if y, m, d := got.Date(); y != 2026 || m != time.August || d != 28 {
		t.Errorf("calendar date = %04d-%02d-%02d, want 2026-08-28", y, int(m), d)
	}
	if h, min, s := got.Clock(); h != 0 || min != 0 || s != 0 {
		t.Errorf("clock = %02d:%02d:%02d, want midnight", h, min, s)
	}
	if got.Location() != Argentina {
		t.Errorf("location = %v, want %v", got.Location(), Argentina)
	}

	// The instant it names is not the UTC-parsed one. Stated as an inequality
	// rather than a fixed offset so the test survives a zone whose offset
	// changes; only that they differ is the contract.
	utc, err := time.Parse("2006-01-02", "2026-08-28")
	if err != nil {
		t.Fatalf("time.Parse: %v", err)
	}
	if Argentina != time.UTC && got.Equal(utc) {
		t.Error("ParseDay must not resolve to the same instant as a UTC parse")
	}
}

func TestParseDayRejectsMalformedInput(t *testing.T) {
	for _, s := range []string{"", "28-08-2026", "2026-8-28", "2026-08-28T00:00:00Z", "not a date"} {
		if _, err := ParseDay(s); err == nil {
			t.Errorf("ParseDay(%q) must fail", s)
		}
	}
}
