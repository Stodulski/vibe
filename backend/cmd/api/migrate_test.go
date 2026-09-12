package main

import (
	"testing"

	"github.com/stodulski/vibe-server/internal/platform/config"
)

// TestMigratorDSNFallsBackToTheApplicationDSN pins the rule that keeps today's
// single-role deployment working: DB_MIGRATOR_URL is optional, and leaving it
// unset must migrate as the same connection the server uses rather than as
// nothing.
func TestMigratorDSNFallsBackToTheApplicationDSN(t *testing.T) {
	const appDSN = "postgres://app@db/vibe"
	const ownerDSN = "postgres://owner@db/vibe"

	tests := []struct {
		name        string
		dsn         string
		migratorDSN string
		want        string
	}{
		{
			name: "unset falls back to the application DSN",
			dsn:  appDSN,
			want: appDSN,
		},
		{
			name:        "set wins",
			dsn:         appDSN,
			migratorDSN: ownerDSN,
			want:        ownerDSN,
		},
		{
			name:        "set with no application DSN still migrates",
			migratorDSN: ownerDSN,
			want:        ownerDSN,
		},
		{
			name: "neither set is empty, and migrate.Up refuses it",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg config.Config
			cfg.DB.DSN = tt.dsn
			cfg.DB.MigratorDSN = tt.migratorDSN

			if got := migratorDSN(cfg); got != tt.want {
				t.Errorf("migratorDSN() = %q, want %q", got, tt.want)
			}
		})
	}
}
