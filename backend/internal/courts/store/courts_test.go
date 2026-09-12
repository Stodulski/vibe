package store

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCourt_StructFields(t *testing.T) {
	now := time.Now()
	c := Court{
		ID:        uuid.New(),
		ComplexID: uuid.New(),
		Name:      "Cancha 1",
		Sport:     "padel",
		CourtType: "indoor",
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if c.Name != "Cancha 1" {
		t.Errorf("Name = %q, want %q", c.Name, "Cancha 1")
	}
	if c.Sport != "padel" {
		t.Errorf("Sport = %q, want %q", c.Sport, "padel")
	}
	if c.CourtType != "indoor" {
		t.Errorf("CourtType = %q, want %q", c.CourtType, "indoor")
	}
	if !c.IsActive {
		t.Error("IsActive should be true")
	}
}

func TestCourtPrice_StructFields(t *testing.T) {
	p := CourtPrice{
		ID:       uuid.New(),
		CourtID:  uuid.New(),
		Price:    12000,
		DayType:  "monday",
		TimeFrom: "08:00",
		TimeTo:   "18:00",
	}

	if p.Price != 12000 {
		t.Errorf("Price = %d, want %d", p.Price, 12000)
	}
	if p.DayType != "monday" {
		t.Errorf("DayType = %q, want %q", p.DayType, "monday")
	}
	if p.TimeFrom != "08:00" {
		t.Errorf("TimeFrom = %q, want %q", p.TimeFrom, "08:00")
	}
	if p.TimeTo != "18:00" {
		t.Errorf("TimeTo = %q, want %q", p.TimeTo, "18:00")
	}
}

func TestBlockedSlot_StructFields(t *testing.T) {
	now := time.Now()
	reason := "Maintenance"
	createdBy := uuid.New()
	s := BlockedSlot{
		ID:        uuid.New(),
		CourtID:   uuid.New(),
		Date:      time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
		StartTime: "10:00",
		EndTime:   "12:00",
		Reason:    &reason,
		CreatedBy: &createdBy,
		CreatedAt: now,
		CourtName: "Court A",
	}

	if s.StartTime != "10:00" {
		t.Errorf("StartTime = %q, want %q", s.StartTime, "10:00")
	}
	if s.EndTime != "12:00" {
		t.Errorf("EndTime = %q, want %q", s.EndTime, "12:00")
	}
	if s.Reason == nil || *s.Reason != "Maintenance" {
		t.Errorf("Reason unexpected value")
	}
	if s.CreatedBy == nil {
		t.Error("CreatedBy should not be nil")
	}
	if s.CourtName != "Court A" {
		t.Errorf("CourtName = %q, want %q", s.CourtName, "Court A")
	}
}

func TestBlockedSlot_NilOptionalFields(t *testing.T) {
	s := BlockedSlot{
		ID:        uuid.New(),
		CourtID:   uuid.New(),
		StartTime: "10:00",
		EndTime:   "12:00",
	}

	if s.Reason != nil {
		t.Error("Reason should be nil")
	}
	if s.CreatedBy != nil {
		t.Error("CreatedBy should be nil")
	}
}

// TestCourtModel_RequiresDB documents that all Store methods
// require a database connection.
func TestCourtModel_RequiresDB(t *testing.T) {
	t.Skip("Store methods all require *pgxpool.Pool and *db.Queries")
}
