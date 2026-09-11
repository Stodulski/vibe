package validator

import (
	"testing"
)

func TestValidatorCheck(t *testing.T) {
	t.Run("ok true does not add error", func(t *testing.T) {
		v := New()
		v.Check(true, "field", "should not appear")
		if !v.Valid() {
			t.Error("expected validator to be valid")
		}
	})

	t.Run("ok false adds error", func(t *testing.T) {
		v := New()
		v.Check(false, "field", "must be provided")
		if v.Valid() {
			t.Error("expected validator to be invalid")
		}
		if v.Errors["field"] != "must be provided" {
			t.Errorf("want error %q; got %q", "must be provided", v.Errors["field"])
		}
	})
}

func TestValidatorAddError(t *testing.T) {
	t.Run("first error wins", func(t *testing.T) {
		v := New()
		v.AddError("email", "first error")
		v.AddError("email", "second error")
		if v.Errors["email"] != "first error" {
			t.Errorf("want %q; got %q", "first error", v.Errors["email"])
		}
	})

	t.Run("different keys are independent", func(t *testing.T) {
		v := New()
		v.AddError("email", "email error")
		v.AddError("name", "name error")
		if len(v.Errors) != 2 {
			t.Errorf("want 2 errors; got %d", len(v.Errors))
		}
	})
}

func TestMatches(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  bool
	}{
		{"valid email", "test@example.com", true},
		{"valid with dots", "user.name@domain.co", true},
		{"missing @", "testexample.com", false},
		{"missing domain", "test@", false},
		{"empty", "", false},
		{"spaces", "test @example.com", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Matches(tt.email, EmailRX)
			if got != tt.want {
				t.Errorf("Matches(%q, EmailRX) = %v, want %v", tt.email, got, tt.want)
			}
		})
	}
}

func TestPermittedValue(t *testing.T) {
	t.Run("value in list", func(t *testing.T) {
		if !PermittedValue("a", "a", "b", "c") {
			t.Error("expected true for value in permitted list")
		}
	})

	t.Run("value not in list", func(t *testing.T) {
		if PermittedValue("d", "a", "b", "c") {
			t.Error("expected false for value not in permitted list")
		}
	})

	t.Run("int values", func(t *testing.T) {
		if !PermittedValue(1, 1, 2, 3) {
			t.Error("expected true for int 1 in [1,2,3]")
		}
	})
}

func TestUnique(t *testing.T) {
	t.Run("unique values", func(t *testing.T) {
		if !Unique([]string{"a", "b", "c"}) {
			t.Error("expected true for unique values")
		}
	})

	t.Run("duplicate values", func(t *testing.T) {
		if Unique([]string{"a", "b", "a"}) {
			t.Error("expected false for duplicate values")
		}
	})

	t.Run("empty slice", func(t *testing.T) {
		if !Unique([]string{}) {
			t.Error("expected true for empty slice")
		}
	})

	t.Run("single element", func(t *testing.T) {
		if !Unique([]int{42}) {
			t.Error("expected true for single element")
		}
	})
}
