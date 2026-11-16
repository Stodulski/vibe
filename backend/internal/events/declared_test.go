package events

import (
	"errors"
	"testing"
	"time"
)

// setModality gives the fixture event its modality and, when min and max are not
// zero, its level range.
func setModality(f *fixture, modality string, min, max int) {
	f.store.event.Modality = ptrS(modality)
	if min != 0 {
		f.store.event.LevelMin, f.store.event.LevelMax = ptrI(min), ptrI(max)
	}
}

func declare(gender string, level int) SignUpInput {
	in := signUpInput()
	in.Gender, in.Level = ptrS(gender), ptrI(level)
	return in
}

func ptrS(s string) *string { return &s }
func ptrI(n int) *int       { return &n }

// assertNothingHeld fails when a refused sign-up still reached the hold or the
// checkout: the store's hold and the payment preference are the two places money
// starts moving.
func assertNothingHeld(t *testing.T, f *fixture) {
	t.Helper()
	if len(f.store.signedUp) != 0 || len(f.checkout.input) != 0 {
		t.Errorf("holds = %d, preferences = %d; a refused sign-up must hold nothing", len(f.store.signedUp), len(f.checkout.input))
	}
}

func refusedWith(t *testing.T, err error, field, code string) {
	t.Helper()
	var invalid *ValidationError
	if !errors.As(err, &invalid) || invalid.Errors[field] != code {
		t.Fatalf("err = %v; want %s on %s", err, code, field)
	}
}

// A men's event refuses a woman before anything is held.
func TestAMensEventRefusesAWomanBeforeAnyHold(t *testing.T) {
	f := newFixture(t)
	setModality(f, ModalityMen, 0, 0)
	_, err := f.svc.SignUp(t.Context(), visitor(), "vibe", f.store.event.ID, declare(GenderWomen, 5))
	refusedWith(t, err, "gender", CodeModalityMismatch)
	assertNothingHeld(t, f)
}

// A mixed event takes both a man and a woman.
func TestAMixedEventTakesBothGenders(t *testing.T) {
	for _, gender := range []string{GenderMen, GenderWomen} {
		f := newFixture(t)
		setModality(f, ModalityMixed, 0, 0)
		if _, err := f.svc.SignUp(t.Context(), visitor(), "vibe", f.store.event.ID, declare(gender, 5)); err != nil {
			t.Fatalf("%s: %v", gender, err)
		}
		if len(f.store.signedUp) != 1 {
			t.Fatalf("%s: holds = %d; want 1", gender, len(f.store.signedUp))
		}
	}
}

// A level outside the event's range is refused before anything is held.
func TestALevelOutsideTheRangeIsRefusedBeforeAnyHold(t *testing.T) {
	f := newFixture(t)
	setModality(f, ModalityMixed, 4, 4) // solo 4ta
	_, err := f.svc.SignUp(t.Context(), visitor(), "vibe", f.store.event.ID, declare(GenderMen, 5))
	refusedWith(t, err, "level", CodeLevelOutOfRange)
	assertNothingHeld(t, f)
}

// Both ends of the range are inside it: "de 8va a 6ta" is 6 to 8.
func TestALevelAtEitherEndOfTheRangeIsAccepted(t *testing.T) {
	for _, level := range []int{6, 8} {
		f := newFixture(t)
		setModality(f, ModalityMixed, 6, 8)
		if _, err := f.svc.SignUp(t.Context(), visitor(), "vibe", f.store.event.ID, declare(GenderMen, level)); err != nil {
			t.Fatalf("level %d: %v", level, err)
		}
	}
}

// A declaration is required once the event has a modality.
func TestAModalityEventNeedsTheGenderAndTheLevel(t *testing.T) {
	f := newFixture(t)
	setModality(f, ModalityMixed, 0, 0)
	_, err := f.svc.SignUp(t.Context(), visitor(), "vibe", f.store.event.ID, signUpInput())
	refusedWith(t, err, "gender", "must be provided")
	assertNothingHeld(t, f)
}

// An event created before the modality keeps today's behaviour: nothing is asked.
func TestALegacyEventTakesTheSignUpWithoutTheFields(t *testing.T) {
	f := newFixture(t)
	if _, err := f.svc.SignUp(t.Context(), visitor(), "vibe", f.store.event.ID, signUpInput()); err != nil {
		t.Fatal(err)
	}
	if len(f.store.signedUp) != 1 {
		t.Fatalf("holds = %d; want 1", len(f.store.signedUp))
	}
}

// The declaration is stored on the participant's row.
func TestTheDeclaredGenderAndLevelReachTheRegistration(t *testing.T) {
	f := newFixture(t)
	setModality(f, ModalityWomen, 2, 5)
	if _, err := f.svc.SignUp(t.Context(), visitor(), "vibe", f.store.event.ID, declare(GenderWomen, 4)); err != nil {
		t.Fatal(err)
	}
	got := f.store.signedUp[0]
	if got.Gender == nil || *got.Gender != GenderWomen || got.Level == nil || *got.Level != 4 {
		t.Errorf("stored declaration = %v / %v; want women and 4", got.Gender, got.Level)
	}
}

// The owner's create answers modality and range rules field by field.
func TestCreateRefusesAnUnknownModalityAndAPartialLevelRange(t *testing.T) {
	cases := map[string]struct {
		change func(*CreateInput)
		field  string
	}{
		"unknown":       {func(in *CreateInput) { in.Modality = "boys" }, "modality"},
		"min only":      {func(in *CreateInput) { in.LevelMin = 4 }, "level_max"},
		"max only":      {func(in *CreateInput) { in.LevelMax = 4 }, "level_max"},
		"out of 1..9":   {func(in *CreateInput) { in.LevelMin, in.LevelMax = 10, 10 }, "level_min"},
		"min above max": {func(in *CreateInput) { in.LevelMin, in.LevelMax = 8, 6 }, "level_max"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			in := validInput()
			tc.change(&in)
			errs := in.validate(time.Now())
			if _, ok := errs[tc.field]; !ok {
				t.Errorf("want an error on %s; got %v", tc.field, errs)
			}
		})
	}
}

// An event sent without a modality is open to everyone, as every event was before
// modalities existed, so an app that does not send it yet keeps working.
func TestAnEventWithoutModalityIsOpenToEveryone(t *testing.T) {
	in := validInput()
	in.Modality = ""
	if errs := in.validate(time.Now()); len(errs) > 0 {
		t.Fatalf("want no errors; got %v", errs)
	}
}

// An event given no name is named after its sport, and a level range given whole
// is kept.
func TestAnEventGivenNoNameIsNamedAfterItsSport(t *testing.T) {
	in := validInput()
	in.Name, in.Sport = "  ", "padel"
	in.LevelMin, in.LevelMax = 4, 4
	if errs := in.validate(time.Now()); len(errs) > 0 {
		t.Fatalf("errs = %v", errs)
	}
	if in.Name != "Cancha abierta de pádel" {
		t.Errorf("name = %q; want %q", in.Name, "Cancha abierta de pádel")
	}
}
