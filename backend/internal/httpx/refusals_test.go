package httpx_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
)

func newResponder() *httpx.Responder {
	return httpx.NewResponder(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func answer(t *testing.T, write func(http.ResponseWriter, *http.Request)) (int, string) {
	t.Helper()
	w := httptest.NewRecorder()
	write(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/thing", nil))
	return w.Code, w.Body.String()
}

// The shared sentinels are the reason DomainError exists: every module raises
// them and none of them should have to know what status they earn.
func TestDomainErrorAnswersTheSharedSentinels(t *testing.T) {
	rs := newResponder()

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{"not found", data.ErrRecordNotFound, http.StatusNotFound, "could not be found"},
		{"wrapped not found", fmt.Errorf("reading court: %w", data.ErrRecordNotFound),
			http.StatusNotFound, "could not be found"},
		{"invalid cursor", data.ErrInvalidCursor, http.StatusBadRequest, "invalid cursor value"},
		{"edit conflict", data.ErrEditConflict, http.StatusConflict, "edit conflict"},
		{"cooldown", data.ErrCooldownActive, http.StatusTooManyRequests, "rate limit exceeded"},
		{"anything else is a fault", errors.New("the database is on fire"),
			http.StatusInternalServerError, "could not process your request"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := answer(t, func(w http.ResponseWriter, r *http.Request) {
				rs.DomainError(w, r, tt.err)
			})
			if status != tt.wantStatus {
				t.Errorf("status = %d, want %d (body %s)", status, tt.wantStatus, body)
			}
			if !strings.Contains(body, tt.wantBody) {
				t.Errorf("body = %s, want it to mention %q", body, tt.wantBody)
			}
		})
	}
}

// A fault must never leak its message to the client — that is the whole reason
// an unmapped error goes through ServerError rather than being printed.
func TestDomainErrorDoesNotLeakAFaultsMessage(t *testing.T) {
	rs := newResponder()
	_, body := answer(t, func(w http.ResponseWriter, r *http.Request) {
		rs.DomainError(w, r, errors.New("dsn=postgres://user:hunter2@db/vibe"))
	})
	if strings.Contains(body, "hunter2") {
		t.Fatalf("the 500 body carried the underlying error: %s", body)
	}
}

var errDomain = errors.New("this module's own refusal")

func TestADomainTableIsConsultedBeforeTheSharedSentinels(t *testing.T) {
	refuser := newResponder().WithRefusals(httpx.Refusals{
		errDomain: httpx.Conflict("you cannot do that yet"),
		// A domain is allowed to disagree with the shared answer for its own
		// routes, so the table wins.
		data.ErrRecordNotFound: httpx.Gone("that venue is no longer listed"),
	})

	status, body := answer(t, func(w http.ResponseWriter, r *http.Request) {
		refuser.DomainError(w, r, fmt.Errorf("deleting: %w", errDomain))
	})
	if status != http.StatusConflict || !strings.Contains(body, "you cannot do that yet") {
		t.Errorf("table entry: status %d, body %s", status, body)
	}

	status, body = answer(t, func(w http.ResponseWriter, r *http.Request) {
		refuser.DomainError(w, r, data.ErrRecordNotFound)
	})
	if status != http.StatusGone || !strings.Contains(body, "no longer listed") {
		t.Errorf("table override: status %d, body %s", status, body)
	}

	status, _ = answer(t, func(w http.ResponseWriter, r *http.Request) {
		refuser.DomainError(w, r, data.ErrInvalidCursor)
	})
	if status != http.StatusBadRequest {
		t.Errorf("fallthrough to the shared sentinels: status %d", status)
	}
}

// DomainErrorWith is for the sentinel that means two different things to a
// person depending on the route. The status stays the table's; only the
// sentence is the handler's.
func TestDomainErrorWithKeepsTheTablesStatus(t *testing.T) {
	refuser := newResponder().WithRefusals(httpx.Refusals{
		errDomain: httpx.Conflict("the table's sentence"),
	})

	status, body := answer(t, func(w http.ResponseWriter, r *http.Request) {
		refuser.DomainErrorWith(w, r, errDomain, "the handler's sentence")
	})
	if status != http.StatusConflict {
		t.Errorf("status = %d, want 409", status)
	}
	if !strings.Contains(body, "the handler's sentence") || strings.Contains(body, "the table's sentence") {
		t.Errorf("body = %s, want the handler's message", body)
	}
}

// A refusal with no message writes the status alone. The WhatsApp webhook
// depends on it: Meta reads the code and nothing else.
func TestARefusalWithoutAMessageWritesNoBody(t *testing.T) {
	rs := newResponder()
	status, body := answer(t, func(w http.ResponseWriter, r *http.Request) {
		rs.Refuse(w, r, httpx.Unauthorized(nil))
	})
	if status != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", status)
	}
	if body != "" {
		t.Errorf("body = %q, want it empty", body)
	}
}

func TestIsErrorStatus(t *testing.T) {
	for status, want := range map[int]bool{
		http.StatusOK: false, http.StatusFound: false, http.StatusNotModified: false,
		http.StatusBadRequest: true, http.StatusNotFound: true, http.StatusInternalServerError: true,
	} {
		if got := httpx.IsErrorStatus(status); got != want {
			t.Errorf("IsErrorStatus(%d) = %v, want %v", status, got, want)
		}
	}
}
