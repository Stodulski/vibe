package db

import (
	"testing"
	"time"
)

const testDSN = "postgres://vibe:vibe@localhost:5432/vibe?sslmode=disable"

func TestBuildPoolConfigSetsTheServerSideTimeouts(t *testing.T) {
	poolConfig, err := BuildPoolConfig(Config{
		DSN:                      testDSN,
		MaxOpenConns:             25,
		MaxIdleConns:             10,
		MaxIdleTime:              15 * time.Minute,
		StatementTimeout:         15 * time.Second,
		IdleInTransactionTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	params := poolConfig.ConnConfig.RuntimeParams
	// Milliseconds, unitless: that is how PostgreSQL reads a bare number for
	// both settings, and a value with a unit suffix here is a connection that
	// fails to open rather than a timeout that is quietly wrong.
	for name, want := range map[string]string{
		"statement_timeout":                   "15000",
		"idle_in_transaction_session_timeout": "30000",
	} {
		if got := params[name]; got != want {
			t.Errorf("want %s=%q; got %q", name, want, got)
		}
	}
}

// A non-positive duration must leave the server's own setting alone. Writing
// "0" would mean "no limit", which is not what an operator who left the knob
// unset asked for — and for idle_in_transaction_session_timeout that is the
// difference between a role-level default and a transaction that can sit open
// forever.
func TestBuildPoolConfigLeavesUnsetTimeoutsToTheServer(t *testing.T) {
	poolConfig, err := BuildPoolConfig(Config{DSN: testDSN, MaxOpenConns: 1, MaxIdleConns: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, name := range []string{"statement_timeout", "idle_in_transaction_session_timeout"} {
		if got, ok := poolConfig.ConnConfig.RuntimeParams[name]; ok {
			t.Errorf("want %s unset; got %q", name, got)
		}
	}
}

func TestBuildPoolConfigCarriesThePoolTuning(t *testing.T) {
	poolConfig, err := BuildPoolConfig(Config{
		DSN:          testDSN,
		MaxOpenConns: 25,
		MaxIdleConns: 10,
		MaxIdleTime:  15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if poolConfig.MaxConns != 25 {
		t.Errorf("want MaxConns 25; got %d", poolConfig.MaxConns)
	}
	if poolConfig.MinConns != 10 {
		t.Errorf("want MinConns 10; got %d", poolConfig.MinConns)
	}
	if poolConfig.MaxConnIdleTime != 15*time.Minute {
		t.Errorf("want MaxConnIdleTime 15m; got %v", poolConfig.MaxConnIdleTime)
	}
	if poolConfig.MaxConnLifetime != maxConnLifetime {
		t.Errorf("want MaxConnLifetime %v; got %v", maxConnLifetime, poolConfig.MaxConnLifetime)
	}
}

func TestBuildPoolConfigRefusesAnUnparseableDSN(t *testing.T) {
	if _, err := BuildPoolConfig(Config{DSN: "not://a valid dsn at all"}); err == nil {
		t.Fatal("expected an unparseable DSN to be refused")
	}
}
