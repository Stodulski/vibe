package store

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestComplex_StructFields(t *testing.T) {
	now := time.Now()
	email := "info@complex.com"
	logoURL := "https://cdn.example.com/logo.webp"
	coverURL := "https://cdn.example.com/cover.webp"
	lat := -34.6037
	lng := -58.3816
	mpToken := "access-token"
	mpRefresh := "refresh-token"
	mpUserID := "mp-user-123"

	c := Complex{
		ID:                uuid.New(),
		OwnerID:           uuid.New(),
		Name:              "Padel Club San Isidro",
		Slug:              "padel-club-san-isidro",
		Address:           "Av. Libertador 1234",
		City:              "San Isidro",
		Province:          "Buenos Aires",
		CountryCode:       "AR",
		Currency:          "ARS",
		Phone:             "+5491155551234",
		Email:             &email,
		LogoURL:           &logoURL,
		CoverURL:          &coverURL,
		DepositPercentage: 50,
		CancellationHours: 24,
		Latitude:          &lat,
		Longitude:         &lng,
		IsActive:          true,
		mpAccessToken:     &mpToken,
		mpRefreshToken:    &mpRefresh,
		MPUserID:          &mpUserID,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if c.Name != "Padel Club San Isidro" {
		t.Errorf("Name = %q, want %q", c.Name, "Padel Club San Isidro")
	}
	if c.Slug != "padel-club-san-isidro" {
		t.Errorf("Slug = %q, want %q", c.Slug, "padel-club-san-isidro")
	}
	if c.DepositPercentage != 50 {
		t.Errorf("DepositPercentage = %d, want %d", c.DepositPercentage, 50)
	}
	if c.CancellationHours != 24 {
		t.Errorf("CancellationHours = %d, want %d", c.CancellationHours, 24)
	}
	if c.CountryCode != "AR" {
		t.Errorf("CountryCode = %q, want %q", c.CountryCode, "AR")
	}
	if c.Currency != "ARS" {
		t.Errorf("Currency = %q, want %q", c.Currency, "ARS")
	}
	if !c.IsActive {
		t.Error("IsActive should be true")
	}
	if c.Latitude == nil || *c.Latitude != -34.6037 {
		t.Error("Latitude unexpected value")
	}
	if c.Longitude == nil || *c.Longitude != -58.3816 {
		t.Error("Longitude unexpected value")
	}
}

func TestComplex_NilOptionalFields(t *testing.T) {
	c := Complex{
		ID:   uuid.New(),
		Name: "Basic Complex",
		Slug: "basic-complex",
	}

	if c.Email != nil {
		t.Error("Email should be nil")
	}
	if c.LogoURL != nil {
		t.Error("LogoURL should be nil")
	}
	if c.CoverURL != nil {
		t.Error("CoverURL should be nil")
	}
	if c.Latitude != nil {
		t.Error("Latitude should be nil")
	}
	if c.Longitude != nil {
		t.Error("Longitude should be nil")
	}
	if c.mpAccessToken != nil {
		t.Error("mpAccessToken should be nil")
	}
	if c.mpRefreshToken != nil {
		t.Error("mpRefreshToken should be nil")
	}
	if c.MPUserID != nil {
		t.Error("MPUserID should be nil")
	}
}

func TestSchedule_StructFields(t *testing.T) {
	s := Schedule{
		ID:        uuid.New(),
		ComplexID: uuid.New(),
		Day:       "monday",
		OpenTime:  "08:00",
		CloseTime: "22:00",
		IsClosed:  false,
	}

	if s.Day != "monday" {
		t.Errorf("Day = %q, want %q", s.Day, "monday")
	}
	if s.OpenTime != "08:00" {
		t.Errorf("OpenTime = %q, want %q", s.OpenTime, "08:00")
	}
	if s.CloseTime != "22:00" {
		t.Errorf("CloseTime = %q, want %q", s.CloseTime, "22:00")
	}
	if s.IsClosed {
		t.Error("IsClosed should be false")
	}
}

func TestSchedule_ClosedDay(t *testing.T) {
	s := Schedule{
		ID:        uuid.New(),
		ComplexID: uuid.New(),
		Day:       "sunday",
		OpenTime:  "00:00",
		CloseTime: "00:00",
		IsClosed:  true,
	}

	if !s.IsClosed {
		t.Error("IsClosed should be true")
	}
}

func TestComplexSlug_StructFields(t *testing.T) {
	cs := ComplexSlug{
		Slug:      "padel-club",
		UpdatedAt: time.Now(),
	}

	if cs.Slug != "padel-club" {
		t.Errorf("Slug = %q, want %q", cs.Slug, "padel-club")
	}
	if cs.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
}

// TestComplexModel_RequiresDB documents that all Store methods
// require a database connection.
func TestComplexModel_RequiresDB(t *testing.T) {
	t.Skip("Store methods all require *pgxpool.Pool and *db.Queries")
}
