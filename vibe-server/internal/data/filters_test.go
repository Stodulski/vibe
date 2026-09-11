package data

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestParseCursor(t *testing.T) {
	t.Run("empty cursor", func(t *testing.T) {
		f := Filters{Cursor: ""}
		ts, id, err := f.ParseCursor()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ts.IsZero() {
			t.Error("expected zero time for empty cursor")
		}
		if id != uuid.Nil {
			t.Error("expected nil UUID for empty cursor")
		}
	})

	t.Run("valid date cursor", func(t *testing.T) {
		testID := uuid.New()
		cursor := "2024-03-15:" + testID.String()
		f := Filters{Cursor: cursor}
		ts, id, err := f.ParseCursor()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ts.Year() != 2024 || ts.Month() != 3 || ts.Day() != 15 {
			t.Errorf("unexpected date: %v", ts)
		}
		if id != testID {
			t.Errorf("want ID %v; got %v", testID, id)
		}
	})

	t.Run("valid RFC3339 cursor", func(t *testing.T) {
		testID := uuid.New()
		cursor := "2024-03-15T14:30:00Z:" + testID.String()
		f := Filters{Cursor: cursor}
		ts, id, err := f.ParseCursor()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ts.Hour() != 14 || ts.Minute() != 30 {
			t.Errorf("unexpected time: %v", ts)
		}
		if id != testID {
			t.Errorf("want ID %v; got %v", testID, id)
		}
	})

	t.Run("too short cursor", func(t *testing.T) {
		f := Filters{Cursor: "short"}
		_, _, err := f.ParseCursor()
		if !errors.Is(err, ErrInvalidCursor) {
			t.Errorf("want ErrInvalidCursor; got %v", err)
		}
	})

	t.Run("invalid UUID", func(t *testing.T) {
		f := Filters{Cursor: "2024-03-15:not-a-valid-uuid-at-all-xxxxx"}
		_, _, err := f.ParseCursor()
		if !errors.Is(err, ErrInvalidCursor) {
			t.Errorf("want ErrInvalidCursor; got %v", err)
		}
	})

	t.Run("missing separator", func(t *testing.T) {
		testID := uuid.New()
		f := Filters{Cursor: "2024-03-15" + testID.String()}
		_, _, err := f.ParseCursor()
		if !errors.Is(err, ErrInvalidCursor) {
			t.Errorf("want ErrInvalidCursor; got %v", err)
		}
	})
}

func TestBuildNextCursor(t *testing.T) {
	date := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	id := uuid.New()
	cursor := BuildNextCursor(date, id)

	f := Filters{Cursor: cursor}
	parsedDate, parsedID, err := f.ParseCursor()
	if err != nil {
		t.Fatalf("roundtrip failed: %v", err)
	}
	if parsedDate.Format("2006-01-02") != "2024-03-15" {
		t.Errorf("date mismatch: %v", parsedDate)
	}
	if parsedID != id {
		t.Errorf("ID mismatch: want %v; got %v", id, parsedID)
	}
}

func TestBuildTimestampCursor(t *testing.T) {
	ts := time.Date(2024, 3, 15, 14, 30, 0, 0, time.UTC)
	id := uuid.New()
	cursor := BuildTimestampCursor(ts, id)

	f := Filters{Cursor: cursor}
	parsedTS, parsedID, err := f.ParseCursor()
	if err != nil {
		t.Fatalf("roundtrip failed: %v", err)
	}
	if parsedTS.Hour() != 14 || parsedTS.Minute() != 30 {
		t.Errorf("timestamp mismatch: %v", parsedTS)
	}
	if parsedID != id {
		t.Errorf("ID mismatch: want %v; got %v", id, parsedID)
	}
}
