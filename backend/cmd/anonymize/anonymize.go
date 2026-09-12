package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Result is what one run touched, so the operator can record it against the
// request they answered.
type Result struct {
	// ClientFound is false when the id matches no row. The command reports
	// that rather than a successful no-op: "we anonymized nobody" and "we
	// anonymized the person who asked" must not look the same in a log.
	ClientFound bool
	// Complex is the venue the client belongs to, for the record.
	Complex uuid.UUID
	// BookingNotesCleared is how many of the client's bookings carried
	// free-text notes that were erased.
	BookingNotesCleared int64
	// Bookings is how many booking rows the client has, all of which are kept.
	Bookings int64
	// Payments is how many payment rows are kept untouched.
	Payments int64
	// AuditEntries is how many audit rows are kept untouched.
	AuditEntries int64
	// Phone is the placeholder written into the phone column.
	Phone string
}

// anonymizedNames are what replaces the person's own. They are not blanked:
// first_name and last_name are NOT NULL, and an empty name renders as a gap in
// every list the venue reads, which reads as a bug rather than as a decision.
const (
	anonymizedFirstName = "Cliente"
	anonymizedLastName  = "Anonimizado"
)

// anonymizedPhone is the placeholder for a client's phone.
//
// It is derived from the id rather than blanked or shared, for two reasons that
// both have to hold: clients.phone is NOT NULL and UNIQUE per complex, so a
// blank or a constant would refuse the second anonymization in a venue; and it
// must not look like a phone number, because a value that does can be dialled,
// messaged on WhatsApp, or matched by GetClientByPhone against a real person
// who later books with it.
func anonymizedPhone(clientID uuid.UUID) string {
	return "anonymized-" + clientID.String()
}

// anonymizeClient erases the person from a client row and from the free text
// their bookings carry, inside one transaction, and leaves the money and the
// audit trail alone.
//
// WHAT IS ERASED and why that is the whole list:
//
//   - clients.first_name, last_name, phone, email — the identity itself.
//   - clients.notes — staff free text about the person ("prefers court 2,
//     husband is Juan"), which is about them even when it does not name them.
//   - bookings.notes — the same free text on the booking, and the one place a
//     name or a phone number reaches a table that is not clients. Nothing else
//     in bookings is about the person: the row is a court, a date and a price.
//
// WHAT IS KEPT, deliberately:
//
//   - Every booking and payment row, with its amounts and its client_id. The
//     venue's books have to still add up, the refund state machine has to still
//     be able to answer for money that moved, and tax records are not the
//     requester's to erase. The client_id now points at a row that names
//     nobody, which is what makes keeping them acceptable.
//   - Every audit_log entry. It is the record of who did what, including this
//     operation; erasing it would remove the evidence that the request was
//     honoured.
//
// tx rather than a pool: the caller owns the transaction, so a run that fails
// halfway leaves a client either fully anonymized or untouched, never named in
// one table and erased in another.
func anonymizeClient(ctx context.Context, tx pgx.Tx, clientID uuid.UUID) (Result, error) {
	var result Result

	// Row-level security is FORCEd on clients and bookings and applies to the
	// schema owner too, so without this the statements below match nothing and
	// the command reports a client that does not exist. This tool is the
	// operator acting outside any one tenant — it is answering a person, not
	// serving a venue — which is exactly what the bypass is for.
	if _, err := tx.Exec(ctx, `SET LOCAL app.bypass_tenant = 'on'`); err != nil {
		return result, fmt.Errorf("declaring the cross-tenant scope: %w", err)
	}

	// FOR UPDATE: a booking being created for this client while the run is in
	// flight would otherwise land after the notes were cleared and carry the
	// person's name past the erasure.
	err := tx.QueryRow(ctx,
		`SELECT complex_id FROM clients WHERE id = $1 FOR UPDATE`, clientID).Scan(&result.Complex)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return result, nil
		}
		return result, fmt.Errorf("locking the client: %w", err)
	}
	result.ClientFound = true
	result.Phone = anonymizedPhone(clientID)

	tag, err := tx.Exec(ctx, `
		UPDATE clients
		SET first_name = $2,
		    last_name  = $3,
		    phone      = $4,
		    email      = NULL,
		    notes      = NULL
		WHERE id = $1`,
		clientID, anonymizedFirstName, anonymizedLastName, result.Phone)
	if err != nil {
		return result, fmt.Errorf("anonymizing the client: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return result, fmt.Errorf("anonymizing the client: %d rows changed, want 1", tag.RowsAffected())
	}

	tag, err = tx.Exec(ctx,
		`UPDATE bookings SET notes = NULL WHERE client_id = $1 AND notes IS NOT NULL`, clientID)
	if err != nil {
		return result, fmt.Errorf("clearing the booking notes: %w", err)
	}
	result.BookingNotesCleared = tag.RowsAffected()

	// The counts of what is kept are part of the answer, not decoration: the
	// operator writing back to the requester has to be able to say "your name
	// and number are gone, these N bookings and M payments remain as
	// anonymous rows, and here is why".
	for _, kept := range []struct {
		query string
		into  *int64
	}{
		{`SELECT COUNT(*) FROM bookings WHERE client_id = $1`, &result.Bookings},
		{`SELECT COUNT(*) FROM payments p JOIN bookings b ON b.id = p.booking_id WHERE b.client_id = $1`, &result.Payments},
		{`SELECT COUNT(*) FROM audit_log WHERE complex_id = $1`, &result.AuditEntries},
	} {
		arg := clientID
		if kept.into == &result.AuditEntries {
			arg = result.Complex
		}
		if err := tx.QueryRow(ctx, kept.query, arg).Scan(kept.into); err != nil {
			return result, fmt.Errorf("counting what is kept: %w", err)
		}
	}

	return result, nil
}
