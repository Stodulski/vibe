package data

import (
	"os"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// TestMain lowers the bcrypt cost for every test in the package, integration
// files included: see PasswordHashCost.
func TestMain(m *testing.M) {
	PasswordHashCost = bcrypt.MinCost
	os.Exit(m.Run())
}
