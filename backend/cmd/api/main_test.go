package main

import (
	"os"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/stodulski/vibe-server/internal/data"
)

// TestMain lowers the bcrypt cost for every test in the package, integration
// files included: see data.PasswordHashCost.
func TestMain(m *testing.M) {
	data.PasswordHashCost = bcrypt.MinCost
	os.Exit(m.Run())
}
