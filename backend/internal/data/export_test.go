package data

// Exports for the package's external test binary (package data_test).
//
// These are implementation details a test asserts against — here, the
// retention and grace constants a lease is timed against. They are
// deliberately not part of the package's API: a caller has no use for them,
// and a test that needs them says so here rather than by forcing them
// exported.
const (
	// LeaseGraceForTest is the slack added to a lease's TTL before it is
	// considered abandoned.
	LeaseGraceForTest = leaseGrace
	// AbandonedLeaseRetentionForTest is how long an abandoned lease row is kept.
	AbandonedLeaseRetentionForTest = abandonedLeaseRetention
)
