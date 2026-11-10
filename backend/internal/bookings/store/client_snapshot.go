package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
)

// ClientSnapshot is who a storefront booking is for while it has no client
// yet. POST /api/v1/book stores it instead of creating a clients row (owner
// rule 2026-11-10: a client exists only once a booking succeeded), and the
// transaction that records the booking's payment turns it into a client with
// AttachClientFromSnapshot, clearing it. db/migrations/024 holds the columns
// and the rule that a booking always has one or the other.
type ClientSnapshot struct {
	FirstName string
	LastName  string
	Phone     string
	Email     string
}

// HasClient reports whether the booking names its client. A storefront
// booking that has not been paid does not; it carries a ClientSnapshot.
func (b *Booking) HasClient() bool { return b.ClientID != uuid.Nil }

// ClientIDOrNil is the booking's client id for the wire, nil while the booking
// has none, so the zero UUID is never handed out as somebody's id.
func (b *Booking) ClientIDOrNil() *uuid.UUID {
	if !b.HasClient() {
		return nil
	}
	id := b.ClientID
	return &id
}

// snapshotFromDB reads the snapshot columns of a bookings row; nil when the row
// names its client (the columns are cleared when one is attached).
func snapshotFromDB(b db.Booking) *ClientSnapshot {
	if b.ClientID.Valid || !b.ClientPhone.Valid {
		return nil
	}
	return &ClientSnapshot{
		FirstName: b.ClientFirstName.String,
		LastName:  b.ClientLastName.String,
		Phone:     b.ClientPhone.String,
		Email:     b.ClientEmail.String,
	}
}

// snapshotParams is the four snapshot columns of an insert; all NULL when the
// booking names its client.
func snapshotParams(b *Booking) (first, last, phone, email *string) {
	if b.HasClient() || b.ClientSnapshot == nil {
		return nil, nil, nil, nil
	}
	s := b.ClientSnapshot
	first, last, phone = &s.FirstName, &s.LastName, &s.Phone
	if s.Email != "" {
		email = &s.Email
	}
	return first, last, phone, email
}

// clientIDParam is client_id for an insert: NULL for a booking held under its
// snapshot.
func clientIDParam(b *Booking) pgtype.UUID {
	return data.UUIDPtrToPg(b.ClientIDOrNil())
}

// AttachClientFromSnapshot gives a storefront booking its client, inside the
// transaction that records the booking's payment, so a booking that is paid
// always names a client and one that never is leaves no clients row behind.
//
// The client is found or created by (complex_id, phone) through
// clientstore.UpsertByPhone — GetOrCreate's own statement, run on tx — with
// allowNameUpdate false: the snapshot came from an unauthenticated request, so
// it never renames an existing client (see clientstore.Store.GetOrCreate). It
// never touches is_blocked either.
//
// A booking that already names its client is left as it is, which makes this
// safe on every payment write and on a redelivered webhook. On success b
// carries the client id and no snapshot.
func AttachClientFromSnapshot(ctx context.Context, tx pgx.Tx, qtx *db.Queries, b *Booking) error {
	snap, err := qtx.GetUnattachedBookingClientSnapshot(ctx, data.UUIDToPg(b.ID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return loadAttachedClient(ctx, tx, b)
		}
		return fmt.Errorf("bookings: read client snapshot: %w", err)
	}

	clientID, err := clientstore.UpsertByPhone(ctx, tx, data.PgToUUID(snap.ComplexID),
		snap.ClientFirstName.String, snap.ClientLastName.String, snap.ClientPhone.String, snap.ClientEmail.String, false)
	if err != nil {
		return err
	}

	if _, err := qtx.AttachBookingClient(ctx, db.AttachBookingClientParams{
		ID:       data.UUIDToPg(b.ID),
		ClientID: data.UUIDToPg(clientID),
	}); err != nil {
		return fmt.Errorf("bookings: attach client: %w", err)
	}
	b.ClientID = clientID
	b.ClientSnapshot = nil
	return nil
}

// loadAttachedClient fills b's client id when the snapshot is already gone
// but b was read before that happened: a concurrent payment write (a
// redelivered webhook, the counter confirming at the same moment) attached the
// client and committed while this transaction waited on the booking's row
// lock. Without it the caller would carry no client and skip its notice and
// audit id. The row is already locked by this transaction, so the value read
// is the committed one.
func loadAttachedClient(ctx context.Context, tx pgx.Tx, b *Booking) error {
	if b.ClientID != uuid.Nil {
		return nil
	}
	var clientID pgtype.UUID
	if err := tx.QueryRow(ctx, `SELECT client_id FROM bookings WHERE id = $1`, data.UUIDToPg(b.ID)).Scan(&clientID); err != nil {
		return fmt.Errorf("bookings: read attached client: %w", err)
	}
	if clientID.Valid {
		b.ClientID = data.PgToUUID(clientID)
		b.ClientSnapshot = nil
	}
	return nil
}

// Contact is who to tell about a booking: its client once attached, its
// snapshot until then.
type Contact struct {
	FirstName string
	LastName  string
	Phone     string
	Email     string
}

// FullName is the name the notices print.
func (c Contact) FullName() string { return c.FirstName + " " + c.LastName }

// ClientGetter reads a client by id; every domain's client store has it.
type ClientGetter interface {
	GetByID(ctx context.Context, id uuid.UUID) (*clientstore.Client, error)
}

// ContactOf resolves who a booking is for. A booking with no client and no
// snapshot loaded answers data.ErrRecordNotFound, as a missing client did.
func ContactOf(ctx context.Context, clients ClientGetter, b *Booking) (Contact, error) {
	if !b.HasClient() {
		if b.ClientSnapshot == nil {
			return Contact{}, data.ErrRecordNotFound
		}
		s := b.ClientSnapshot
		return Contact{FirstName: s.FirstName, LastName: s.LastName, Phone: s.Phone, Email: s.Email}, nil
	}
	client, err := clients.GetByID(ctx, b.ClientID)
	if err != nil {
		return Contact{}, err
	}
	c := Contact{FirstName: client.FirstName, LastName: client.LastName, Phone: client.Phone}
	if client.Email != nil {
		c.Email = *client.Email
	}
	return c, nil
}
