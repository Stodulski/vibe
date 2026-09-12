package data

import "errors"

// Sentinel errors the stores share. A sentinel only one domain raises lives in
// that domain's store package.
var (
	// ErrRecordNotFound is returned when a lookup by ID or unique key matches no row.
	ErrRecordNotFound = errors.New("record not found")
	// ErrInvalidCursor is returned when a pagination cursor cannot be parsed.
	ErrInvalidCursor = errors.New("invalid cursor")
	// ErrCooldownActive is returned when a token resend is attempted before its cooldown expires.
	ErrCooldownActive = errors.New("cooldown active")
	// ErrEditConflict is returned when a row moved out from under a read, so
	// the update that followed would have written over somebody else's work.
	//
	// Every domain that has its own edit conflict wraps this one, because the
	// answer is the same 409 whatever changed underneath and httpx.Responder
	// should not have to learn each domain's name for it.
	ErrEditConflict = errors.New("edit conflict")
)
