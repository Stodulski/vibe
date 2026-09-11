package booklink

import (
	"net/url"
	"strings"
	"testing"
)

// TestEveryFunctionEmitsTheConfiguredQueryParam pins the one property this
// package exists to guarantee: every URL-building function names its
// credential with exactly QueryParam, and no other identifying query
// parameter appears anywhere in the built URL.
//
// Mutation, run and recorded: change QueryParam to a literal "token" inline
// in one function only (e.g. Cancel) — re-run, and this test must fail
// because that one function's URL shape becomes inconsistent with the rest of
// the package.
func TestEveryFunctionEmitsTheConfiguredQueryParam(t *testing.T) {
	const credential = "cred-abc123"
	urls := map[string]string{
		"Cancel":         Cancel("https://vibe.test", "acme", credential),
		"CancelPath":     CancelPath("acme", credential),
		"Success":        Success("https://vibe.test", "acme", credential),
		"SuccessPending": SuccessPending("https://vibe.test", "acme", credential),
	}

	for name, raw := range urls {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("%s built an unparsable URL %q: %v", name, raw, err)
		}
		qs := u.Query()
		if got := qs.Get(QueryParam); got != credential {
			t.Errorf("%s: query param %q = %q, want %q (raw=%q)", name, QueryParam, got, credential, raw)
		}
		// No other identifying query parameter — status=pending is the one
		// documented exception on SuccessPending.
		for key := range qs {
			if key == QueryParam || (name == "SuccessPending" && key == "status") {
				continue
			}
			t.Errorf("%s: unexpected query parameter %q in %q", name, key, raw)
		}
	}

	// Failure carries no credential at all.
	failure := Failure("https://vibe.test", "acme")
	fu, err := url.Parse(failure)
	if err != nil {
		t.Fatalf("Failure built an unparsable URL %q: %v", failure, err)
	}
	if len(fu.Query()) != 1 || fu.Query().Get("error") == "" {
		t.Errorf("Failure: want only an error query parameter; got %q", failure)
	}
	if strings.Contains(failure, QueryParam) {
		t.Errorf("Failure must carry no credential; got %q", failure)
	}
}

// TestCancelPathIsRelative pins that CancelPath never carries a scheme/host,
// the one shape difference from Cancel — it is meant for the WhatsApp
// button's relative link.
func TestCancelPathIsRelative(t *testing.T) {
	got := CancelPath("acme", "cred-abc123")
	if strings.HasPrefix(got, "http") {
		t.Errorf("CancelPath must be relative; got %q", got)
	}
	want := "acme/book/cancel?" + QueryParam + "=cred-abc123"
	if got != want {
		t.Errorf("CancelPath = %q, want %q", got, want)
	}
}

// TestCancelIsAbsolute pins the counterpart: Cancel always carries the
// frontend origin.
func TestCancelIsAbsolute(t *testing.T) {
	got := Cancel("https://vibe.test", "acme", "cred-abc123")
	want := "https://vibe.test/acme/book/cancel?" + QueryParam + "=cred-abc123"
	if got != want {
		t.Errorf("Cancel = %q, want %q", got, want)
	}
}

// A complex that never filled in its coordinates used to produce an empty maps
// query, and internal/notifications refused to enqueue a WhatsApp confirmation
// with an unbound button — so that venue lost the whole WhatsApp channel,
// silently, with the email still going out to hide it. A venue always has a
// name, so this always answers.
func TestMapsQuery(t *testing.T) {
	lat, lng := -34.603722, -58.381592

	t.Run("coordinates when the complex has them", func(t *testing.T) {
		got := MapsQuery("Vibe Norte", "Av. Santa Fe 1200", "Buenos Aires", &lat, &lng)
		if got != "-34.603722,-58.381592" {
			t.Errorf("MapsQuery = %q; want the coordinates", got)
		}
	})

	t.Run("the written address when it has none", func(t *testing.T) {
		got := MapsQuery("Vibe Norte", "Av. Santa Fe 1200", "Buenos Aires", nil, nil)
		want := "Vibe+Norte%2C+Av.+Santa+Fe+1200%2C+Buenos+Aires"
		if got != want {
			t.Errorf("MapsQuery = %q; want %q", got, want)
		}
	})

	t.Run("half a coordinate pair is no coordinate pair", func(t *testing.T) {
		if got := MapsQuery("Vibe Norte", "", "", &lat, nil); got != "Vibe+Norte" {
			t.Errorf("MapsQuery = %q; want the name", got)
		}
	})

	t.Run("the name alone is still an answer", func(t *testing.T) {
		if got := MapsQuery("Vibe Norte", "", "", nil, nil); got != "Vibe+Norte" {
			t.Errorf("MapsQuery = %q; want the name", got)
		}
	})

	t.Run("nothing at all is empty rather than a stray comma", func(t *testing.T) {
		if got := MapsQuery("", "  ", "", nil, nil); got != "" {
			t.Errorf("MapsQuery = %q; want empty", got)
		}
	})
}

// TestMapsURL pins that the email link is exactly MapsQuery's answer appended
// to the fixed Google Maps search prefix, and that a venue with nothing to
// show (MapsQuery's last case) gets no link at all rather than a bare prefix.
func TestMapsURL(t *testing.T) {
	lat, lng := -34.603722, -58.381592

	t.Run("wraps the coordinates", func(t *testing.T) {
		got := MapsURL("Vibe Norte", "Av. Santa Fe 1200", "Buenos Aires", &lat, &lng)
		want := "https://www.google.com/maps/search/?api=1&query=-34.603722,-58.381592"
		if got != want {
			t.Errorf("MapsURL = %q, want %q", got, want)
		}
	})

	t.Run("wraps the address fallback", func(t *testing.T) {
		got := MapsURL("Vibe Norte", "Av. Santa Fe 1200", "Buenos Aires", nil, nil)
		want := "https://www.google.com/maps/search/?api=1&query=Vibe+Norte%2C+Av.+Santa+Fe+1200%2C+Buenos+Aires"
		if got != want {
			t.Errorf("MapsURL = %q, want %q", got, want)
		}
	})

	t.Run("empty when MapsQuery is empty", func(t *testing.T) {
		if got := MapsURL("", "  ", "", nil, nil); got != "" {
			t.Errorf("MapsURL = %q, want empty", got)
		}
	})
}

// TestAddress pins the same street/city join cmd/api's reminder cron used to
// do inline, now shared with the confirmation email.
func TestAddress(t *testing.T) {
	tests := []struct {
		name, address, city, want string
	}{
		{"both", "Av. Santa Fe 1200", "Buenos Aires", "Av. Santa Fe 1200, Buenos Aires"},
		{"address only", "Av. Santa Fe 1200", "", "Av. Santa Fe 1200"},
		{"city only", "", "Buenos Aires", "Buenos Aires"},
		{"neither", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Address(tt.address, tt.city); got != tt.want {
				t.Errorf("Address(%q, %q) = %q, want %q", tt.address, tt.city, got, tt.want)
			}
		})
	}
}

func TestBookLinks(t *testing.T) {
	if got := BookPath("vibe-norte"); got != "vibe-norte/book" {
		t.Errorf("BookPath = %q", got)
	}
	if got := Book("https://app.vibe.com.ar", "vibe-norte"); got != "https://app.vibe.com.ar/vibe-norte/book" {
		t.Errorf("Book = %q", got)
	}
}
