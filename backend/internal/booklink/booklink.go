// Package booklink builds every client-facing URL a booking's public routes
// are reached through: the absolute cancel link and MercadoPago's
// success/pending/failure checkout redirects.
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
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
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
// notifications.
func Cancel(publicURL, complexSlug, credential string) string {
	return fmt.Sprintf("%s/c/%s/book/cancel?%s=%s", publicURL, complexSlug, QueryParam, url.QueryEscape(credential))
}

// Success builds the MercadoPago checkout preference's success back_url.
func Success(publicURL, complexSlug, credential string) string {
	return fmt.Sprintf("%s/c/%s/book/success?%s=%s", publicURL, complexSlug, QueryParam, url.QueryEscape(credential))
}

// SuccessPending builds the MercadoPago checkout preference's pending
// back_url — the same success page, with an explicit pending status,
// since MercadoPago redirects there for a payment still under review.
func SuccessPending(publicURL, complexSlug, credential string) string {
	return fmt.Sprintf("%s/c/%s/book/success?%s=%s&status=pending", publicURL, complexSlug, QueryParam, url.QueryEscape(credential))
}

// EventPage is the public page of an open-play event ("canchas abiertas"): where
// the sign-up is shared and where a confirmation email points.
func EventPage(publicURL, complexSlug string, eventID uuid.UUID) string {
	return fmt.Sprintf("%s/c/%s/events/%s", publicURL, complexSlug, eventID)
}

// EventSuccess builds the checkout preference's success back_url for an event
// sign-up: the event page with the person's registration credential, which the
// page reads to show the spot and the cancel action.
func EventSuccess(publicURL, complexSlug string, eventID uuid.UUID, credential string) string {
	return fmt.Sprintf("%s?%s=%s&registered=1", EventPage(publicURL, complexSlug, eventID), QueryParam, url.QueryEscape(credential))
}

// EventPending is EventSuccess for a payment still under review.
func EventPending(publicURL, complexSlug string, eventID uuid.UUID, credential string) string {
	return fmt.Sprintf("%s?%s=%s&registered=1&status=pending", EventPage(publicURL, complexSlug, eventID), QueryParam, url.QueryEscape(credential))
}

// EventFailure is the failure back_url of an event sign-up. Like Failure it
// carries no credential.
func EventFailure(publicURL, complexSlug string, eventID uuid.UUID) string {
	return fmt.Sprintf("%s?error=payment_failed", EventPage(publicURL, complexSlug, eventID))
}

// Failure builds the MercadoPago checkout preference's failure back_url. It
// carries the booking's credential, the same one Success carries: a rejected
// payment leaves the booking pending and holding its slot, and the storefront
// uses the credential to let the visitor pay again on the same checkout or let
// the slot go (the book checkout retry and release routes). MercadoPago
// already holds this token in the success URL, so passing it back on failure
// tells no third party anything it did not have; no personal data is in it.
func Failure(publicURL, complexSlug, credential string) string {
	return fmt.Sprintf("%s/c/%s/book?error=payment_failed&%s=%s", publicURL, complexSlug, QueryParam, url.QueryEscape(credential))
}

// MapsQuery builds the dynamic suffix for a Google Maps search button — the
// part appended to https://www.google.com/maps/search/?api=1&query=.
//
// Coordinates when the complex has them, the written address when it does not.
// The fallback is the whole point: a complex with no latitude still gets a
// link. A venue always has a name, so this always returns something.
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

// Book builds the complex's public booking page, absolute, for the
// cancellation and refund emails' "Nueva reserva" button.
func Book(publicURL, complexSlug string) string {
	return fmt.Sprintf("%s/c/%s/book", publicURL, complexSlug)
}

// StorefrontSlot is the complex's public page opened on one slot: the day, the
// sport, the duration and the hour, the four things the storefront reads from
// its query (useComplexPageState), plus the court the notice was about. It lands
// on the courts free at that hour, so a waitlist email takes the person straight
// to the place that freed; when that court was taken first, the court lets the
// page say so.
func StorefrontSlot(publicURL, complexSlug string, day time.Time, sport string, durationMinutes int, startTime string, courtID uuid.UUID) string {
	q := url.Values{}
	q.Set("court", courtID.String())
	q.Set("date", day.Format("2006-01-02"))
	q.Set("sport", sport)
	q.Set("duration", strconv.Itoa(durationMinutes))
	q.Set("time", startTime)
	return fmt.Sprintf("%s/c/%s?%s", publicURL, complexSlug, q.Encode())
}

// StorefrontOnDate is the complex's public page opened on one day. The
// storefront reads the day from ?date=; pages that do not read it yet ignore the
// parameter and show the page as usual.
func StorefrontOnDate(publicURL, complexSlug string, day time.Time) string {
	return fmt.Sprintf("%s/c/%s?date=%s", publicURL, complexSlug, day.Format("2006-01-02"))
}

// mapsURLPrefix is the fixed base a Google Maps "search" deep link is built
// from; MapsQuery returns everything appended after it.
const mapsURLPrefix = "https://www.google.com/maps/search/?api=1&query="

// MapsURL builds the absolute Google Maps link the confirmation and reminder
// emails' "Cómo llegar" link uses, from the same coordinate/address fallback
// as MapsQuery.
//
// It wraps MapsQuery rather than re-deriving the fallback, so the rule for
// which venues have coordinates and which fall back to the written address
// lives in one place. Empty in, empty out: a venue with nothing to show gets no
// link rather than a bare prefix.
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

// TournamentPage is the public page of a tournament: its draw, schedule and the
// pair sign-up, and where a confirmation email points.
func TournamentPage(publicURL, complexSlug string, tournamentID uuid.UUID) string {
	return fmt.Sprintf("%s/c/%s/tournaments/%s", publicURL, complexSlug, tournamentID)
}

// TournamentSuccess builds the checkout preference's success back_url for a pair
// sign-up: the tournament page with the pair's registration credential.
func TournamentSuccess(publicURL, complexSlug string, tournamentID uuid.UUID, credential string) string {
	return fmt.Sprintf("%s?%s=%s&registered=1", TournamentPage(publicURL, complexSlug, tournamentID), QueryParam, url.QueryEscape(credential))
}

// TournamentPending is TournamentSuccess for a payment still under review.
func TournamentPending(publicURL, complexSlug string, tournamentID uuid.UUID, credential string) string {
	return fmt.Sprintf("%s?%s=%s&registered=1&status=pending", TournamentPage(publicURL, complexSlug, tournamentID), QueryParam, url.QueryEscape(credential))
}

// TournamentFailure is the failure back_url of a pair sign-up; like Failure it
// carries no credential.
func TournamentFailure(publicURL, complexSlug string, tournamentID uuid.UUID) string {
	return fmt.Sprintf("%s?error=payment_failed", TournamentPage(publicURL, complexSlug, tournamentID))
}
