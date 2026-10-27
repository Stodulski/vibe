package complexes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
)

// Whatever the owner pastes, a handle, an @handle or a profile link, the column
// stores the bare handle. Anything that does not reduce to one is refused.
func TestNormalizeInstagram(t *testing.T) {
	tests := []struct {
		raw    string
		want   string
		wantOK bool
	}{
		{raw: "", want: "", wantOK: true},
		{raw: "   ", want: "", wantOK: true},
		{raw: "club.padel", want: "club.padel", wantOK: true},
		{raw: "  @Club_Padel ", want: "club_padel", wantOK: true},
		{raw: "instagram.com/club.padel", want: "club.padel", wantOK: true},
		{raw: "https://www.instagram.com/club.padel/?hl=es", want: "club.padel", wantOK: true},
		{raw: "http://m.instagram.com/club.padel#top", want: "club.padel", wantOK: true},
		{raw: "https://www.instagram.com/@club.padel", want: "club.padel", wantOK: true},
		{raw: strings.Repeat("a", 30), want: strings.Repeat("a", 30), wantOK: true},
		{raw: strings.Repeat("a", 31), wantOK: false},
		{raw: "club padel", wantOK: false},
		{raw: "club-padel", wantOK: false},
		{raw: "@", wantOK: false},
		{raw: "https://facebook.com/clubpadel", wantOK: false},
		{raw: "https://www.instagram.com/club.padel/reels", wantOK: false},
		{raw: "https://www.instagram.com/", wantOK: false},
	}
	for _, tt := range tests {
		got, ok := normalizeInstagram(tt.raw)
		if ok != tt.wantOK || got != tt.want {
			t.Errorf("normalizeInstagram(%q) = (%q, %v), want (%q, %v)", tt.raw, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestCreateStoresTheInstagramHandle(t *testing.T) {
	f := newFixture(t)
	body := strings.Replace(validComplex, `"phone":`, `"instagram":"https://www.instagram.com/Vibe.Palermo/","phone":`, 1)

	w := httptest.NewRecorder()
	f.handler.Create(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), nil, nil, body))

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
	}
	if f.store.inserted == nil || f.store.inserted.Instagram == nil || *f.store.inserted.Instagram != "vibe.palermo" {
		t.Errorf("want the bare handle stored; got %+v", f.store.inserted)
	}
	if !strings.Contains(w.Body.String(), `"instagram":"vibe.palermo"`) {
		t.Errorf("the owner response must carry the handle; got %s", w.Body.String())
	}
}

func TestCreateWithoutInstagramStoresNull(t *testing.T) {
	for name, body := range map[string]string{
		"omitted": validComplex,
		"blank":   strings.Replace(validComplex, `"phone":`, `"instagram":"  ","phone":`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)

			w := httptest.NewRecorder()
			f.handler.Create(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), nil, nil, body))

			if w.Code != http.StatusCreated {
				t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
			}
			if f.store.inserted == nil || f.store.inserted.Instagram != nil {
				t.Errorf("want no Instagram stored; got %+v", f.store.inserted)
			}
		})
	}
}

func TestCreateRefusesAnInvalidInstagram(t *testing.T) {
	f := newFixture(t)
	body := strings.Replace(validComplex, `"phone":`, `"instagram":"club padel","phone":`, 1)

	w := httptest.NewRecorder()
	f.handler.Create(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), nil, nil, body))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), instagramMessage) {
		t.Errorf("the error must name the instagram field; got %s", w.Body.String())
	}
	if f.store.inserted != nil {
		t.Errorf("a refused create must not persist; got %+v", f.store.inserted)
	}
}

func TestUpdateStoresTheInstagramHandle(t *testing.T) {
	f := newFixture(t)
	complex := &complexstore.Complex{ID: uuid.New(), Name: "Vibe Palermo"}

	w := httptest.NewRecorder()
	f.handler.Update(w, ownerRequest(t, http.MethodPatch, "/", uuid.New(), complex, nil,
		`{"instagram":"@VibePalermo"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if f.store.updated == nil || f.store.updated.Instagram == nil || *f.store.updated.Instagram != "vibepalermo" {
		t.Errorf("want the bare handle stored; got %+v", f.store.updated)
	}
}

// A blank value clears the account; an omitted one leaves it as it was.
func TestUpdateClearsOrKeepsTheInstagram(t *testing.T) {
	tests := []struct {
		name string
		body string
		want *string
	}{
		{name: "blank clears", body: `{"instagram":"  "}`, want: nil},
		{name: "omitted keeps", body: `{"name":"Vibe Palermo Norte"}`, want: new("anterior")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			complex := &complexstore.Complex{ID: uuid.New(), Name: "Vibe Palermo", Instagram: new("anterior")}

			w := httptest.NewRecorder()
			f.handler.Update(w, ownerRequest(t, http.MethodPatch, "/", uuid.New(), complex, nil, tt.body))

			if w.Code != http.StatusOK {
				t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
			}
			got := f.store.updated.Instagram
			if (got == nil) != (tt.want == nil) || (got != nil && *got != *tt.want) {
				t.Errorf("instagram = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateRefusesAnInvalidInstagram(t *testing.T) {
	f := newFixture(t)
	complex := &complexstore.Complex{ID: uuid.New(), Name: "Vibe Palermo"}

	w := httptest.NewRecorder()
	f.handler.Update(w, ownerRequest(t, http.MethodPatch, "/", uuid.New(), complex, nil,
		`{"instagram":"https://facebook.com/vibe"}`))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
	if f.store.updated != nil {
		t.Errorf("a refused update must not persist; got %+v", f.store.updated)
	}
}

func TestGetPublicCarriesTheInstagramHandle(t *testing.T) {
	f := newFixture(t)
	f.store.complex = &complexstore.Complex{ID: uuid.New(), Slug: "vibe", Name: "Vibe", IsActive: true, Instagram: new("vibe.palermo")}

	r := withSlug(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), "vibe")
	w := httptest.NewRecorder()
	f.handler.GetPublic(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"instagram":"vibe.palermo"`) {
		t.Errorf("the public page must carry the handle; got %s", w.Body.String())
	}
}
