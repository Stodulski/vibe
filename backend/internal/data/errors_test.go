package data

import (
	"errors"
	"fmt"
	"testing"
)

func TestSentinelErrors_NotNil(t *testing.T) {
	sentinels := []struct {
		name string
		err  error
	}{
		{"ErrRecordNotFound", ErrRecordNotFound},
		{"ErrInvalidCursor", ErrInvalidCursor},
		{"ErrCooldownActive", ErrCooldownActive},
	}

	for _, tt := range sentinels {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Errorf("%s is nil", tt.name)
			}
		})
	}
}

func TestSentinelErrors_Distinct(t *testing.T) {
	all := []error{
		ErrRecordNotFound,
		ErrInvalidCursor,
		ErrCooldownActive,
	}

	for i := 0; i < len(all); i++ {
		for j := i + 1; j < len(all); j++ {
			if errors.Is(all[i], all[j]) {
				t.Errorf("sentinel errors %d and %d should be distinct", i, j)
			}
		}
	}
}

func TestSentinelErrors_MatchWithErrorsIs(t *testing.T) {
	sentinels := []error{
		ErrRecordNotFound,
		ErrInvalidCursor,
		ErrCooldownActive,
	}

	for _, sentinel := range sentinels {
		t.Run(sentinel.Error(), func(t *testing.T) {
			if !errors.Is(sentinel, sentinel) {
				t.Errorf("errors.Is(%v, %v) = false, want true", sentinel, sentinel)
			}
		})
	}
}

func TestSentinelErrors_WrappedMatch(t *testing.T) {
	wrapped := fmt.Errorf("operation failed: %w", ErrRecordNotFound)
	if !errors.Is(wrapped, ErrRecordNotFound) {
		t.Error("wrapped ErrRecordNotFound should be matched by errors.Is")
	}
}

func TestSentinelErrors_NonEmptyMessage(t *testing.T) {
	sentinels := []error{
		ErrRecordNotFound,
		ErrInvalidCursor,
		ErrCooldownActive,
	}

	for _, sentinel := range sentinels {
		t.Run(sentinel.Error(), func(t *testing.T) {
			if sentinel.Error() == "" {
				t.Error("error message should not be empty")
			}
		})
	}
}
