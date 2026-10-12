package store

import "errors"

// Sentinel errors returned by this store.
var (
	// ErrDuplicateSlug is returned when a live complex already holds the public
	// slug.
	//
	// complexes_slug_live_key is a PARTIAL unique index (WHERE deleted_at IS
	// NULL): a soft-deleted venue releases its slug, so a new venue may take it,
	// and its old public page answers 410 Gone until then. Two live venues still
	// cannot share a slug, because the slug is the public URL clients hold in
	// links, messages and search results.
	ErrDuplicateSlug = errors.New("duplicate slug")
	// ErrDuplicateOwner is returned when an account already owns a live
	// complex.
	//
	// complexes_owner_id_key is a PARTIAL unique index (WHERE deleted_at IS
	// NULL): a soft-deleted complex frees the owner's slot, so the owner can
	// open a new one. ErrDuplicateSlug is the same kind of partial index.
	ErrDuplicateOwner = errors.New("duplicate owner")
	// ErrActiveBookings is returned by the MercadoPago connect and disconnect
	// writes when the complex still owes someone their hours and the write
	// would strand them: disconnecting, or connecting a different seller
	// account than the one those bookings were priced and collected under.
	//
	// It is the database's answer, asked inside the transaction that holds the
	// complex row, not a pre-check the caller made earlier. The complexes
	// package re-exports it as complexes.ErrActiveBookings.
	ErrActiveBookings = errors.New("complex still has active bookings")
)
