package validator

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// EmailRX validates the general shape of an email address (local-part@domain).
var EmailRX = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

// PhoneRX is a loose phone validator kept for backwards compatibility.
//
// Deprecated: use E164RX and NormalizePhone instead.
var PhoneRX = regexp.MustCompile(`^\+?[\d\s\-()]{8,20}$`)

// E164RX validates phone numbers in E.164 format: + prefix followed by 8-15 digits,
// first digit after + must be 1-9.
var E164RX = regexp.MustCompile(`^\+[1-9]\d{7,14}$`)

// NormalizePhone strips formatting characters and validates E.164 format.
// Returns the normalized phone or an error if invalid.
func NormalizePhone(phone string) (string, error) {
	// Strip spaces, dashes, parens — keep digits and leading +.
	cleaned := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' || r == '+' {
			return r
		}
		return -1
	}, phone)

	if !E164RX.MatchString(cleaned) {
		return "", fmt.Errorf("invalid phone number format")
	}
	return cleaned, nil
}

// Validator accumulates named field errors for the fluent v.Check(...) validation pattern.
type Validator struct {
	Errors map[string]string
}

// New returns an empty Validator ready to accumulate field errors.
func New() *Validator {
	return &Validator{Errors: make(map[string]string)}
}

// Valid reports whether no field errors have been recorded.
func (v *Validator) Valid() bool {
	return len(v.Errors) == 0
}

// AddError records message for key, keeping only the first error added per key.
func (v *Validator) AddError(key, message string) {
	if _, exists := v.Errors[key]; !exists {
		v.Errors[key] = message
	}
}

// Check records message for key when ok is false.
func (v *Validator) Check(ok bool, key, message string) {
	if !ok {
		v.AddError(key, message)
	}
}

// Matches reports whether value matches the given regular expression.
func Matches(value string, rx *regexp.Regexp) bool {
	return rx.MatchString(value)
}

// PermittedValue reports whether value is one of permittedValues.
func PermittedValue[T comparable](value T, permittedValues ...T) bool {
	return slices.Contains(permittedValues, value)
}

// Unique reports whether every value in values is distinct.
func Unique[T comparable](values []T) bool {
	seen := make(map[T]bool)
	for _, value := range values {
		if seen[value] {
			return false
		}
		seen[value] = true
	}
	return true
}
