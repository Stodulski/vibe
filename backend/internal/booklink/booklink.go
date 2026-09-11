// Package booklink builds every client-facing URL a booking's public routes
// are reached through: the absolute cancel link and its relative WhatsApp-button
// counterpart, and MercadoPago's success/pending/failure checkout redirects.
//
// Before this package existed, three sites built the same four URL shapes with
// duplicated fmt.Sprintf pairs — internal/bookings/create.go,
// internal/payments/process.go, and internal/mp/mp.go (the one MercadoPago
// hands to a third party). Collapsing them here means "exactly one function
// constructs a booking link" is true by construction, not by convention: a
// future rename or credential swap touches one file, not three.
package booklink

import (
	"fmt"
	"net/url"
	"strings"
)

// QueryParam is the name the three public routes read and every link writes.
//
// It must stay exactly "token": cmd/api/sentry.go's named-secret rule ends
// its alternation with a bare `token`, and the \b before it cannot match
// inside `booking_token` because an underscore is a word character
// (sentry.go's "named secret" rule comment). Renaming this constant silently
// loses that redaction with no visible failure at the call site — pinned by
// TestBookingLinkURLIsScrubbed (cmd/api/sentry_test.go), not by this comment.
const QueryParam = "token"

// Cancel builds the absolute cancel link sent in booking confirmation
// notifications (email, WhatsApp).
func Cancel(frontendURL, complexSlug, credential string) string {
	return fmt.Sprintf("%s/%s/book/cancel?%s=%s", frontendURL, complexSlug, QueryParam, url.QueryEscape(credential))
}

// CancelPath builds the same link as Cancel, relative to the frontend origin
// rather than absolute, for the WhatsApp confirmation message's button.
func CancelPath(complexSlug, credential string) string {
	return fmt.Sprintf("%s/book/cancel?%s=%s", complexSlug, QueryParam, url.QueryEscape(credential))
}

// Success builds the MercadoPago checkout preference's success back_url.
func Success(frontendURL, complexSlug, credential string) string {
	return fmt.Sprintf("%s/%s/book/success?%s=%s", frontendURL, complexSlug, QueryParam, url.QueryEscape(credential))
}

// SuccessPending builds the MercadoPago checkout preference's pending
// back_url — the same success page, with an explicit pending status,
// since MercadoPago redirects there for a payment still under review.
func SuccessPending(frontendURL, complexSlug, credential string) string {
	return fmt.Sprintf("%s/%s/book/success?%s=%s&status=pending", frontendURL, complexSlug, QueryParam, url.QueryEscape(credential))
}

// Failure builds the MercadoPago checkout preference's failure back_url. It
// carries no credential: nothing on the far side of a failed payment can be
// looked up by one.
func Failure(frontendURL, complexSlug string) string {
	return fmt.Sprintf("%s/%s/book?error=payment_failed", frontendURL, complexSlug)
}

// BookPath builds the complex's public booking page, relative to the frontend
// origin, for the "Nueva reserva" button on the cancellation and refund
// WhatsApp templates.
//
// It is the one client-facing link that carries no credential: the booking
// page is public, and a client whose booking was just cancelled has nothing
// left to authorize against.
func BookPath(complexSlug string) string {
	return fmt.Sprintf("%s/book", complexSlug)
}

// MapsQuery builds the dynamic suffix for a Google Maps search button — the
// part appended to https://www.google.com/maps/search/?api=1&query=.
//
// Coordinates when the complex has them, the written address when it does not.
// The fallback is the whole point: this used to return "" for a complex with
// no latitude, and internal/notifications refused to enqueue a WhatsApp
// confirmation whose button parameter was empty, so every venue that had never
// filled in its coordinates lost the entire WhatsApp channel — silently, and
// with the email still going out to hide it. A venue always has a name, so
// this always returns something.
func MapsQuery(name, address, city string, latitude, longitude *float64) string {
	if latitude != nil && longitude != nil {
		return fmt.Sprintf("%f,%f", *latitude, *longitude)
	}

	parts := make([]string, 0, 3)
	for _, part := range []string{name, address, city} {
		if strings.TrimSpace(part) != "" {
			parts = append(parts, strings.TrimSpace(part))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	// Escaped here rather than by the caller: Meta appends the parameter to
	// the button's base URL verbatim, so whatever this returns is what lands
	// in the query string.
	return url.QueryEscape(strings.Join(parts, ", "))
}

// Book builds the same public booking page as BookPath, absolute, for the
// cancellation and refund emails' "Nueva reserva" button.
func Book(frontendURL, complexSlug string) string {
	return fmt.Sprintf("%s/%s/book", frontendURL, complexSlug)
}

// mapsURLPrefix is the fixed base a Google Maps "search" deep link is built
// from; MapsQuery returns everything appended after it.
const mapsURLPrefix = "https://www.google.com/maps/search/?api=1&query="

// MapsURL builds the absolute Google Maps link the confirmation and reminder
// emails' "Cómo llegar" link uses, from the same coordinate/address fallback
// as MapsQuery.
//
// It wraps MapsQuery rather than re-deriving the fallback so the two
// call sites — the WhatsApp templates' button suffix and the emails' full
// link — can never disagree about which venues have coordinates and which
// fall back to the written address. Empty in, empty out: a venue with
// nothing to show gets no link rather than a bare prefix.
func MapsURL(name, address, city string, latitude, longitude *float64) string {
	q := MapsQuery(name, address, city, latitude, longitude)
	if q == "" {
		return ""
	}
	return mapsURLPrefix + q
}

// Address joins a complex's street address and city into the single line the
// confirmation and reminder emails print before their "Cómo llegar" link.
func Address(address, city string) string {
	switch {
	case address == "":
		return city
	case city == "":
		return address
	default:
		return address + ", " + city
	}
}
