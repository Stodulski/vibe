// Package store persists open-play events ("canchas libres"): an event the owner
// publishes that takes courts off sale for a window of one day, and the people
// who sign up for it one by one.
package store

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// Event statuses.
const (
	EventActive    = "active"
	EventCancelled = "cancelled"
)

// Registration statuses.
const (
	// RegistrationHeld holds one spot until HeldUntil while the person pays.
	RegistrationHeld = "held"
	// RegistrationConfirmed is a paid spot.
	RegistrationConfirmed = "confirmed"
	// RegistrationCancelled is a spot given up by the person or by the owner
	// cancelling the event.
	RegistrationCancelled = "cancelled"
	// RegistrationExpired is a hold nobody paid in time.
	RegistrationExpired = "expired"
)

// Who ended a registration.
const (
	CancelledByParticipant = "participant"
	CancelledByOwner       = "owner"
)

// Sentinel errors returned by this store.
var (
	// ErrEventFull is returned when every spot of an event is taken, by paid
	// registrations or by holds still running.
	ErrEventFull = errors.New("event is full")
	// ErrEventClosed is returned when the event is cancelled or has begun, so no
	// one can sign up or have a late payment confirmed.
	ErrEventClosed = errors.New("event is closed")
	// ErrAlreadyRegistered is returned when the phone already holds a live spot
	// of the event.
	ErrAlreadyRegistered = errors.New("phone already registered in the event")
	// ErrNotCancellable is returned when a registration is already over.
	ErrNotCancellable = errors.New("registration is not cancellable")
	// ErrNotConfirmable is returned when a registration can no longer be
	// confirmed: it was cancelled.
	ErrNotConfirmable = errors.New("registration is not confirmable")
	// ErrEventNotActive is returned by CancelEvent for an event that is already
	// cancelled.
	ErrEventNotActive = errors.New("event is not active")
)

// Reasons an event cannot be created: what already holds the hours.
const (
	ConflictBooking = "booking"
	ConflictBlocked = "blocked"
	ConflictFixed   = "fixed"
	ConflictEvent   = "event"
)

// ConflictError is returned by CreateEvent when something already holds the
// event's hours on one of its courts.
type ConflictError struct {
	// Reason is one of the Conflict* constants.
	Reason string
	// CourtID is the first court found in conflict.
	CourtID uuid.UUID
}

func (e *ConflictError) Error() string {
	return "event hours are held on court " + e.CourtID.String() + " (" + e.Reason + ")"
}

// Court is a court an event holds.
type Court struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// Event is an open-play event.
type Event struct {
	ID              uuid.UUID
	ComplexID       uuid.UUID
	Name            string
	Sport           string
	Date            time.Time
	StartTime       string
	DurationMinutes int
	Capacity        int
	// Price is centavos per person.
	Price        int
	Status       string
	CancelReason *string
	CancelledAt  *time.Time
	CancelledBy  *uuid.UUID
	CreatedBy    uuid.UUID
	CreatedAt    time.Time

	// Courts are the courts the event holds, read with the event.
	Courts []Court
	// ConfirmedCount and HeldCount are the paid spots and the holds still
	// running, read with the event. Their sum is what capacity is checked
	// against.
	ConfirmedCount int
	HeldCount      int
}

// StartsAt is the instant the event begins.
func (e *Event) StartsAt() time.Time {
	return slotsAt(e.Date, e.StartTime)
}

// EndsAt is the instant the event ends: it may fall on the next day.
func (e *Event) EndsAt() time.Time {
	return e.StartsAt().Add(time.Duration(e.DurationMinutes) * time.Minute)
}

// SpotsLeft is how many spots are still free: capacity minus the paid spots and
// the holds still running, never below zero.
func (e *Event) SpotsLeft() int {
	return max(0, e.Capacity-e.ConfirmedCount-e.HeldCount)
}

// slotsAt is the instant of a date's HH:MM on the product's wall-clock.
func slotsAt(date time.Time, hhmm string) time.Time {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		t = time.Time{}
	}
	d := timezone.Day(date)
	return time.Date(d.Year(), d.Month(), d.Day(), t.Hour(), t.Minute(), 0, 0, timezone.Argentina)
}

// Registration is one person's spot in an event.
type Registration struct {
	ID        uuid.UUID
	EventID   uuid.UUID
	ComplexID uuid.UUID
	FirstName string
	LastName  string
	Phone     string
	Email     *string
	Status    string
	// Price and ServiceFee are what the person is asked to pay, in centavos.
	Price       int
	ServiceFee  int
	HeldUntil   time.Time
	ConfirmedAt *time.Time
	CancelledAt *time.Time
	CancelledBy *string
	CreatedAt   time.Time
}

// RegistrationPayment is the payment that carries a registration's money.
type RegistrationPayment struct {
	ID           uuid.UUID
	Status       string
	Amount       int
	ServiceFee   int
	RefundAmount int
	MPPaymentID  *string
}

// RegistrationRow is a registration with its payment, as the owner's
// participant list reads it. Payment is nil while nothing was ever started.
type RegistrationRow struct {
	Registration
	Payment *RegistrationPayment
}

// Store is the sqlc-backed open-play events store.
type Store struct {
	DB *data.DB
	Q  *db.Queries
	// PaymentExpiry is how long a sign-up holds its spot while the person pays,
	// the same configured window a storefront booking holds its slot for.
	PaymentExpiry time.Duration
}
