package data

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestClient_StructFields(t *testing.T) {
	now := time.Now()
	email := "client@example.com"
	notes := "Regular player"
	c := Client{
		ID:            uuid.New(),
		ComplexID:     uuid.New(),
		FirstName:     "Maria",
		LastName:      "Garcia",
		Phone:         "+5491155551234",
		Email:         &email,
		Notes:         &notes,
		IsBlocked:     false,
		TotalBookings: 15,
		NoShows:       1,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if c.FirstName != "Maria" {
		t.Errorf("FirstName = %q, want %q", c.FirstName, "Maria")
	}
	if c.LastName != "Garcia" {
		t.Errorf("LastName = %q, want %q", c.LastName, "Garcia")
	}
	if c.Phone != "+5491155551234" {
		t.Errorf("Phone = %q, want %q", c.Phone, "+5491155551234")
	}
	if c.Email == nil || *c.Email != "client@example.com" {
		t.Errorf("Email unexpected value")
	}
	if c.TotalBookings != 15 {
		t.Errorf("TotalBookings = %d, want %d", c.TotalBookings, 15)
	}
	if c.NoShows != 1 {
		t.Errorf("NoShows = %d, want %d", c.NoShows, 1)
	}
	if c.IsBlocked {
		t.Error("IsBlocked should be false")
	}
}

func TestClient_NilOptionalFields(t *testing.T) {
	c := Client{
		ID:        uuid.New(),
		FirstName: "John",
		LastName:  "Doe",
		Phone:     "+5491155551234",
	}

	if c.Email != nil {
		t.Error("Email should be nil")
	}
	if c.Notes != nil {
		t.Error("Notes should be nil")
	}
}

func TestTopClient_StructFields(t *testing.T) {
	tc := TopClient{
		ID:           uuid.New(),
		Name:         "Maria Garcia",
		Phone:        "+5491155551234",
		BookingCount: 20,
		TotalSpent:   300000,
	}

	if tc.Name != "Maria Garcia" {
		t.Errorf("Name = %q, want %q", tc.Name, "Maria Garcia")
	}
	if tc.BookingCount != 20 {
		t.Errorf("BookingCount = %d, want %d", tc.BookingCount, 20)
	}
	if tc.TotalSpent != 300000 {
		t.Errorf("TotalSpent = %d, want %d", tc.TotalSpent, 300000)
	}
}

func TestClientInsights_StructFields(t *testing.T) {
	insights := ClientInsights{
		TopClients: []TopClient{
			{Name: "Client A", BookingCount: 10},
		},
		NoShowRate:     5,
		NoShowCount:    2,
		CompletedCount: 40,
		NewClients:     8,
		Recurring:      12,
		TotalActive:    20,
	}

	if insights.NoShowRate != 5 {
		t.Errorf("NoShowRate = %d, want %d", insights.NoShowRate, 5)
	}
	if insights.NewClients != 8 {
		t.Errorf("NewClients = %d, want %d", insights.NewClients, 8)
	}
	if insights.Recurring != 12 {
		t.Errorf("Recurring = %d, want %d", insights.Recurring, 12)
	}
	if insights.TotalActive != 20 {
		t.Errorf("TotalActive = %d, want %d", insights.TotalActive, 20)
	}
	if len(insights.TopClients) != 1 {
		t.Errorf("TopClients length = %d, want %d", len(insights.TopClients), 1)
	}
}

func TestClientInsights_NoShowRateCalculation(t *testing.T) {
	// Mirror the calculation logic from GetInsights:
	// NoShowRate = (NoShowCount * 100) / CompletedCount
	tests := []struct {
		name           string
		noShowCount    int
		completedCount int
		wantRate       int
	}{
		{"no shows", 0, 50, 0},
		{"5 percent", 5, 100, 5},
		{"10 percent", 2, 20, 10},
		{"33 percent truncated", 1, 3, 33},
		{"100 percent", 10, 10, 100},
		{"zero completed", 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rate int
			if tt.completedCount > 0 {
				rate = (tt.noShowCount * 100) / tt.completedCount
			}
			if rate != tt.wantRate {
				t.Errorf("NoShowRate = %d, want %d", rate, tt.wantRate)
			}
		})
	}
}

// TestClientModel_RequiresDB documents that all ClientModel methods
// require a database connection.
func TestClientModel_RequiresDB(t *testing.T) {
	t.Skip("ClientModel methods all require *pgxpool.Pool and *db.Queries")
}
