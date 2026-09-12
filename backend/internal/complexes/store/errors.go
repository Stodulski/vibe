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
)
