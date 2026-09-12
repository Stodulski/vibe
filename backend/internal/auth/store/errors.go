package store

import "errors"

// ErrDuplicateEmail is returned when an email address is already registered.
var ErrDuplicateEmail = errors.New("duplicate email")
