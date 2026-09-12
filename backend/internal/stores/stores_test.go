package stores

import (
	"testing"
)

// TestModels_RequiresDB documents that NewModels and the Stores struct
// require a *pgxpool.Pool database connection. All store interfaces
// (UserStore, ComplexStore, etc.) define methods that take context.Context
// and perform database operations. There is no pure logic on the Stores
// struct itself to unit test without a database.
func TestModels_RequiresDB(t *testing.T) {
	t.Skip("Stores struct requires a *pgxpool.Pool; all store methods are database-dependent")
}

// TestModels_InterfacesAreDefined verifies that the Stores struct has all
// expected store fields by checking the type compiles with all interfaces.
func TestModels_InterfacesAreDefined(t *testing.T) {
	// This test verifies at compile time that the Stores struct
	// contains all the expected store interface fields.
	var m Stores
	var _ = m.Users
	var _ = m.Complexes
	var _ = m.Courts
	var _ = m.Bookings
	var _ = m.BookingLinkTokens
	var _ = m.Tokens
	var _ = m.Clients
	var _ = m.Payments
	var _ = m.EmailVerification
	var _ = m.PasswordReset
	var _ = m.FailedRefunds
	var _ = m.SlotLocks
	var _ = m.Admin
}
