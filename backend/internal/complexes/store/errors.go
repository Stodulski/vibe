package store

import "errors"

// Sentinel errors returned by this store.
var (
	// ErrDuplicateSlug is returned when a complex's public slug is already
	// taken.
	//
	// complexes.slug is UNIQUE across the whole table (complexes_slug_key), not
	// only across live rows: a soft-deleted venue keeps its slug. That is
	// deliberate — the slug is the public URL clients hold in links, messages
	// and search results, so handing it to a different venue would silently
	// redirect one business's inbound traffic to another. Reusing a deleted
	// complex's slug is therefore this error, not an ordinary insert. Contrast
	// ErrDuplicateCourtName, whose constraint is partial.
	ErrDuplicateSlug = errors.New("duplicate slug")
	// ErrDuplicateOwner is returned when an account already owns a live
	// complex.
	//
	// complexes_owner_id_key is a PARTIAL unique index (WHERE deleted_at IS
	// NULL), unlike complexes_slug_key: a soft-deleted complex frees the
	// owner's slot, so the owner can open a new one. Contrast ErrDuplicateSlug,
	// whose constraint holds across deleted rows too.
	ErrDuplicateOwner = errors.New("duplicate owner")
)
