package whatsapp

import "strings"

// This file holds the two rules Meta applies to everything we hand it: who a
// message is addressed to, and what a template parameter is allowed to
// contain. Both used to be nobody's job, and both fail silently — a number
// Meta cannot route is a 4xx the circuit breaker deliberately counts as a
// success, and a parameter with a stray tab is a whole message refused.

// RecipientID renders a stored E.164 phone number the way WhatsApp addresses
// it.
//
// Argentina is the one country this has to do anything about. An Argentine
// mobile is dialled nationally as 0 11 15 5555-1234 and written
// internationally as +54 9 11 5555 1234: the 9 stands in for the 15 that marks
// the line as mobile, and WhatsApp routes on the 9 form. internal/validator's
// NormalizePhone accepts +541155551234 as perfectly good E.164 — it is — and
// nothing between there and Meta ever put the 9 back, so a client who typed
// their number without it got a booking, an email, and no WhatsApp message,
// with a 4xx nobody was watching.
//
// Every other country passes through untouched. Brazil's +55 11 9 9999-9999
// already carries its own 9 as part of the national number and must not be
// read as Argentina's, which is why the rule is keyed on the country code
// rather than on "a leading 9 is missing".
func RecipientID(phone string) string {
	cleaned := cleanPhone(phone)

	const arPrefix = "+54"
	if !strings.HasPrefix(cleaned, arPrefix) {
		return cleaned
	}

	national := cleaned[len(arPrefix):]
	if strings.HasPrefix(national, "9") {
		// Already in the mobile form WhatsApp routes on.
		return cleaned
	}
	// Ten digits is an Argentine mobile without its 9: area code (2 to 4) plus
	// subscriber number, which always sum to ten. A landline is the same
	// length, and sending a template to one fails either way; adding the 9
	// costs nothing there and is the only thing that works for a mobile.
	if len(national) != 10 {
		return cleaned
	}
	return arPrefix + "9" + national
}

// cleanPhone strips the spaces, dashes and parentheses a person types, keeping
// the digits and a leading plus.
func cleanPhone(phone string) string {
	var b strings.Builder
	b.Grow(len(phone))
	for i, r := range phone {
		switch {
		case r == '+' && i == 0:
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		}
	}
	return b.String()
}

// sanitizeParam makes a value safe to substitute into a template parameter.
//
// Meta refuses the whole message when any text parameter carries a newline, a
// tab, or more than four consecutive spaces ("Param text cannot have
// new-line/tab characters or more than 4 consecutive spaces"). None of the
// values this package sends should contain one, but three of them are typed by
// a complex owner — the venue name, the court name, the street address — and
// nothing in the create/edit handlers forbids a stray tab or a double-tapped
// space bar. One of those would silently kill every WhatsApp message that
// venue sends.
//
// Every run of whitespace collapses to a single space rather than only runs of
// four or more: the rule is then one sentence instead of a threshold, and a
// parameter whose value differs from what the owner typed by one space is not
// a thing anybody can see.
func sanitizeParam(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inSpace := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\v' || r == '\f' {
			inSpace = true
			continue
		}
		if inSpace && b.Len() > 0 {
			b.WriteRune(' ')
		}
		inSpace = false
		b.WriteRune(r)
	}
	return b.String()
}
