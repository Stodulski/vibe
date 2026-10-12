package waitlist

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/timezone"
)

func newTestHandler(st *fakeStore, cx *fakeComplexes) *Handler {
	respond := httpx.NewResponder(slog.New(slog.NewTextHandler(io.Discard, nil)))
	return NewHandler(newTestService(st, cx, &fakeNotifier{}), respond)
}

func postJoin(t *testing.T, h *Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/public/complexes/vibe-padel/waitlist", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.SetPathValue("slug", "vibe-padel")
	w := httptest.NewRecorder()
	h.Join(w, r)
	return w
}

func dayBody(day, email string) string {
	return `{"date":"` + day + `","email":"` + email + `"}`
}

func TestJoinAnswers204AndStoresTheTrimmedLowercasedEntry(t *testing.T) {
	complexID := uuid.New()
	st := &fakeStore{}
	h := newTestHandler(st, activeComplex(complexID))
	day := timezone.Today().AddDate(0, 0, 3).Format("2006-01-02")

	w := postJoin(t, h, dayBody(" "+day+" ", "  Ana@Example.COM "))

	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204; got %d (%s)", w.Code, w.Body.String())
	}
	if len(st.inserted) != 1 || st.inserted[0].email != "ana@example.com" || st.inserted[0].complexID != complexID {
		t.Fatalf("want one stored entry, ana@example.com; got %+v", st.inserted)
	}
}

func TestJoinAcceptsTodayAndTheLastDayOfTheHorizon(t *testing.T) {
	for _, offset := range []int{0, 365} {
		st := &fakeStore{}
		h := newTestHandler(st, activeComplex(uuid.New()))
		day := timezone.Today().AddDate(0, 0, offset).Format("2006-01-02")

		w := postJoin(t, h, dayBody(day, "ana@example.com"))

		if w.Code != http.StatusNoContent {
			t.Errorf("offset %d: want 204; got %d (%s)", offset, w.Code, w.Body.String())
		}
	}
}

func TestJoinRejectsAMalformedBody(t *testing.T) {
	h := newTestHandler(&fakeStore{}, activeComplex(uuid.New()))

	w := postJoin(t, h, `{"date":`)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for malformed JSON; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestJoinRejectsAnInvalidEmail(t *testing.T) {
	day := timezone.Today().AddDate(0, 0, 3).Format("2006-01-02")
	tooLong := strings.Repeat("a", 250) + "@x.co"
	for _, email := range []string{"", "not-an-email", tooLong} {
		h := newTestHandler(&fakeStore{}, activeComplex(uuid.New()))

		w := postJoin(t, h, dayBody(day, email))

		if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), `"email"`) {
			t.Errorf("email %q: want 422 naming email; got %d (%s)", email[:min(len(email), 20)], w.Code, w.Body.String())
		}
	}
}

func TestJoinRejectsADateThatIsNotADay(t *testing.T) {
	h := newTestHandler(&fakeStore{}, activeComplex(uuid.New()))

	w := postJoin(t, h, dayBody("18-10-2026", "ana@example.com"))

	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), `"date"`) {
		t.Fatalf("want 422 naming date; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestJoinRejectsAPastDate(t *testing.T) {
	h := newTestHandler(&fakeStore{}, activeComplex(uuid.New()))
	yesterday := timezone.Today().AddDate(0, 0, -1).Format("2006-01-02")

	w := postJoin(t, h, dayBody(yesterday, "ana@example.com"))

	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), `"date"`) {
		t.Fatalf("want 422 naming date for a past day; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestJoinRejectsADateBeyondTheBookingHorizon(t *testing.T) {
	st := &fakeStore{}
	h := newTestHandler(st, activeComplex(uuid.New()))
	beyond := timezone.Today().AddDate(0, 0, 366).Format("2006-01-02")

	w := postJoin(t, h, dayBody(beyond, "ana@example.com"))

	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), `"date"`) {
		t.Fatalf("want 422 naming date beyond the horizon; got %d (%s)", w.Code, w.Body.String())
	}
	if len(st.inserted) != 0 {
		t.Fatalf("an out-of-window day must not be stored")
	}
}

func TestJoinAnswers404ForAnUnknownComplex(t *testing.T) {
	h := newTestHandler(&fakeStore{}, &fakeComplexes{err: data.ErrRecordNotFound})
	day := timezone.Today().AddDate(0, 0, 3).Format("2006-01-02")

	w := postJoin(t, h, dayBody(day, "ana@example.com"))

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404 for an unknown slug; got %d (%s)", w.Code, w.Body.String())
	}
}
