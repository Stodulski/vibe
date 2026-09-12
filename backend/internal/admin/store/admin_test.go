package store

import (
	"testing"
	"time"

	"github.com/google/uuid"
	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	"github.com/stodulski/vibe-server/internal/data"
)

func TestPlatformStats_StructFields(t *testing.T) {
	stats := PlatformStats{
		TotalUsers:        100,
		ActiveUsers:       85,
		NewUsersMonth:     12,
		TotalComplexes:    25,
		NewComplexesMonth: 3,
		TotalCourts:       75,
		TotalBookings:     5000,
		TotalRevenue:      15000000,
	}

	if stats.TotalUsers != 100 {
		t.Errorf("TotalUsers = %d, want %d", stats.TotalUsers, 100)
	}
	if stats.ActiveUsers != 85 {
		t.Errorf("ActiveUsers = %d, want %d", stats.ActiveUsers, 85)
	}
	if stats.NewUsersMonth != 12 {
		t.Errorf("NewUsersMonth = %d, want %d", stats.NewUsersMonth, 12)
	}
	if stats.TotalComplexes != 25 {
		t.Errorf("TotalComplexes = %d, want %d", stats.TotalComplexes, 25)
	}
	if stats.TotalCourts != 75 {
		t.Errorf("TotalCourts = %d, want %d", stats.TotalCourts, 75)
	}
	if stats.TotalBookings != 5000 {
		t.Errorf("TotalBookings = %d, want %d", stats.TotalBookings, 5000)
	}
	if stats.TotalRevenue != 15000000 {
		t.Errorf("TotalRevenue = %d, want %d", stats.TotalRevenue, 15000000)
	}
}

func TestAdminUserRow_StructFields(t *testing.T) {
	now := time.Now()
	u := AdminUserRow{
		ID:            uuid.New(),
		Email:         "admin@example.com",
		FirstName:     "Admin",
		LastName:      "User",
		Phone:         "+5491155551234",
		Role:          "admin",
		IsActive:      true,
		EmailVerified: true,
		CreatedAt:     now,
		ComplexCount:  3,
	}

	if u.Email != "admin@example.com" {
		t.Errorf("Email = %q, want %q", u.Email, "admin@example.com")
	}
	if u.Role != "admin" {
		t.Errorf("Role = %q, want %q", u.Role, "admin")
	}
	if u.ComplexCount != 3 {
		t.Errorf("ComplexCount = %d, want %d", u.ComplexCount, 3)
	}
}

func TestAdminUserDetail_StructFields(t *testing.T) {
	user := &authstore.User{
		ID:    uuid.New(),
		Email: "owner@example.com",
		Role:  "owner",
	}
	complexes := []*data.Complex{
		{ID: uuid.New(), Name: "Complex A"},
		{ID: uuid.New(), Name: "Complex B"},
	}

	detail := AdminUserDetail{
		User:      user,
		Complexes: complexes,
	}

	if detail.User.Email != "owner@example.com" {
		t.Errorf("User.Email = %q, want %q", detail.User.Email, "owner@example.com")
	}
	if len(detail.Complexes) != 2 {
		t.Errorf("Complexes length = %d, want %d", len(detail.Complexes), 2)
	}
}

func TestAdminComplexRow_StructFields(t *testing.T) {
	row := AdminComplexRow{
		ID:          uuid.New(),
		OwnerID:     uuid.New(),
		OwnerName:   "John Doe",
		OwnerEmail:  "john@example.com",
		Name:        "Padel Club",
		Slug:        "padel-club",
		City:        "Buenos Aires",
		IsActive:    true,
		CourtsCount: 4,
		MPConnected: true,
		CreatedAt:   time.Now(),
	}

	if row.OwnerName != "John Doe" {
		t.Errorf("OwnerName = %q, want %q", row.OwnerName, "John Doe")
	}
	if row.CourtsCount != 4 {
		t.Errorf("CourtsCount = %d, want %d", row.CourtsCount, 4)
	}
	if !row.MPConnected {
		t.Error("MPConnected should be true")
	}
}

func TestAdminComplexDetail_StructFields(t *testing.T) {
	detail := AdminComplexDetail{
		Complex:       &data.Complex{ID: uuid.New(), Name: "Test Complex"},
		OwnerName:     "Jane Smith",
		OwnerEmail:    "jane@example.com",
		CourtsCount:   6,
		ClientsCount:  150,
		BookingsCount: 3000,
		TotalRevenue:  9000000,
	}

	if detail.OwnerName != "Jane Smith" {
		t.Errorf("OwnerName = %q, want %q", detail.OwnerName, "Jane Smith")
	}
	if detail.CourtsCount != 6 {
		t.Errorf("CourtsCount = %d, want %d", detail.CourtsCount, 6)
	}
	if detail.ClientsCount != 150 {
		t.Errorf("ClientsCount = %d, want %d", detail.ClientsCount, 150)
	}
	if detail.BookingsCount != 3000 {
		t.Errorf("BookingsCount = %d, want %d", detail.BookingsCount, 3000)
	}
	if detail.TotalRevenue != 9000000 {
		t.Errorf("TotalRevenue = %d, want %d", detail.TotalRevenue, 9000000)
	}
}

// TestAdminModel_RequiresDB documents that all Store methods
// require a database connection.
func TestAdminModel_RequiresDB(t *testing.T) {
	t.Skip("Store methods all require *pgxpool.Pool")
}
