package data

// The SQLSTATEs a write path translates instead of surfacing.
//
// Both mean the same thing to a caller — this court is already sold for these
// hours — and both have to be listed. Before the generated span the refusal came
// from idx_bookings_no_double, a unique index, as 23505; that index was dropped
// once bookings_no_overlapping_span subsumed it, which moved every
// double-booking refusal to 23P01. Matching only the first would have turned a
// 409 the client can act on into a 500 nobody can.
const (
	SQLStateUniqueViolation    = "23505"
	SQLStateExclusionViolation = "23P01"
)
