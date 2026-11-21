package complexes

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/mp"
	"github.com/stodulski/vibe-server/internal/notifications"
)

type mpNoticeSent struct {
	kind   string
	notice notifications.ConnectionNotice
	event  string
}

type stubMPNotifier struct {
	mu        sync.Mutex
	sent      []mpNoticeSent
	refuseErr error
}

func (n *stubMPNotifier) record(kind string, notice notifications.ConnectionNotice, event string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.sent = append(n.sent, mpNoticeSent{kind, notice, event})
}

func (n *stubMPNotifier) MercadoPagoConnected(notice notifications.ConnectionNotice, event string) {
	n.record("connected", notice, event)
}

func (n *stubMPNotifier) MercadoPagoDisconnected(notice notifications.ConnectionNotice, event string) {
	n.record("disconnected", notice, event)
}

func (n *stubMPNotifier) MercadoPagoRefreshFailed(notice notifications.ConnectionNotice, episode string) error {
	if n.refuseErr != nil {
		return n.refuseErr
	}
	n.record("refresh_failed", notice, episode)
	return nil
}

func (n *stubMPNotifier) of(kind string) []mpNoticeSent {
	n.mu.Lock()
	defer n.mu.Unlock()
	var out []mpNoticeSent
	for _, s := range n.sent {
		if s.kind == kind {
			out = append(out, s)
		}
	}
	return out
}

// stubRefreshNotices is the episode marker the database keeps: the last
// episode the owner was told about, per complex.
type stubRefreshNotices struct {
	mu   sync.Mutex
	told map[uuid.UUID]string
}

func (s *stubRefreshNotices) MPRefreshFailureNotified(_ context.Context, id uuid.UUID, episode string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.told[id] == episode, nil
}

func (s *stubRefreshNotices) RecordMPRefreshFailureNotice(_ context.Context, id uuid.UUID, episode string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.told[id] = episode
	return nil
}

func TestConnectingMercadoPagoTellsTheOwner(t *testing.T) {
	ownerID := uuid.New()
	complex := &complexstore.Complex{ID: uuid.New(), OwnerID: ownerID, Name: "Vibe Palermo"}

	t.Run("a connection that was stored", func(t *testing.T) {
		f := newFixture(t)
		f.store.complex = complex
		f.payments.tokens = &mp.OAuthTokens{AccessToken: "AT", RefreshToken: "RT", UserID: 12345}

		w := httptest.NewRecorder()
		f.handler.ConnectMercadoPago(w, ownerRequest(t, http.MethodPost, "/", ownerID, complex, nil, connectBody))
		if w.Code != http.StatusOK {
			t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
		}

		got := f.notify.of("connected")
		if len(got) != 1 {
			t.Fatalf("want one connected notice; got %+v", f.notify.sent)
		}
		if got[0].notice.OwnerID != ownerID.String() || got[0].notice.ComplexID != complex.ID.String() || got[0].event == "" {
			t.Errorf("the notice names the wrong owner or complex, or has no event: %+v", got[0])
		}
	})

	t.Run("a connection the store refused", func(t *testing.T) {
		f := newFixture(t)
		f.store.complex = complex
		f.store.connectErr = ErrActiveBookings
		f.payments.tokens = &mp.OAuthTokens{AccessToken: "AT", RefreshToken: "RT", UserID: 12345}

		f.handler.ConnectMercadoPago(httptest.NewRecorder(), ownerRequest(t, http.MethodPost, "/", ownerID, complex, nil, connectBody))
		if len(f.notify.sent) != 0 {
			t.Errorf("a refused connection was announced: %+v", f.notify.sent)
		}
	})
}

func TestDisconnectingMercadoPagoTellsTheOwner(t *testing.T) {
	ownerID := uuid.New()
	complex := &complexstore.Complex{ID: uuid.New(), OwnerID: ownerID, Name: "Vibe Palermo"}

	t.Run("a disconnection that was stored", func(t *testing.T) {
		f := newFixture(t)
		f.store.complex = complex

		if err := f.service.DisconnectMercadoPago(t.Context(), complex.ID, Actor{UserID: &ownerID}); err != nil {
			t.Fatalf("DisconnectMercadoPago: %v", err)
		}
		got := f.notify.of("disconnected")
		if len(got) != 1 || got[0].notice.OwnerID != ownerID.String() || got[0].notice.ComplexID != complex.ID.String() {
			t.Errorf("want one disconnected notice for the owner; got %+v", f.notify.sent)
		}
	})

	t.Run("a disconnection refused for live bookings", func(t *testing.T) {
		f := newFixture(t)
		f.store.complex = complex
		f.bookings.hasOwnerBookings = true

		if err := f.service.DisconnectMercadoPago(t.Context(), complex.ID, Actor{UserID: &ownerID}); !errors.Is(err, ErrActiveBookings) {
			t.Fatalf("want ErrActiveBookings; got %v", err)
		}
		if len(f.notify.sent) != 0 {
			t.Errorf("a refused disconnection was announced: %+v", f.notify.sent)
		}
	})
}

var invalidGrant = &mp.APIError{StatusCode: http.StatusBadRequest, Body: `{"error":"invalid_grant"}`}

// A refused refresh tells the owner once per episode. The sweep meets the same
// refused token every 12 hours, and checkout meets it on every attempt; only
// the first refusal is news. A successful refresh or a reconnect ends the
// episode, and the next refusal starts a new one.
func TestARefusedRefreshTellsTheOwnerOncePerEpisode(t *testing.T) {
	w := newRefreshWorld(t)
	w.mercado.err = invalidGrant

	refuse := func() {
		t.Helper()
		if _, err := w.f.service.RefreshMPCredentials(t.Context(), w.id, ""); !IsMPRefreshRejected(err) {
			t.Fatalf("want a rejection; got %v", err)
		}
	}

	refuse()
	refuse()
	if got := w.f.notify.of("refresh_failed"); len(got) != 1 {
		t.Fatalf("want one notice for the episode; got %d", len(got))
	} else if got[0].notice.ComplexID != w.id.String() || got[0].event == "" {
		t.Errorf("the notice names the wrong complex or no episode: %+v", got[0])
	}

	// The connection is made healthy again: the owner reconnects (or a
	// refresh succeeds), which stores a new refresh token and ends the episode.
	if err := w.store.UpdateMPCredentials(t.Context(), w.id, "access-new", "refresh-new", "42", 3600); err != nil {
		t.Fatal(err)
	}

	refuse()
	refuse()
	got := w.f.notify.of("refresh_failed")
	if len(got) != 2 {
		t.Fatalf("want a second notice for the new episode; got %d", len(got))
	}
	if got[0].event == got[1].event {
		t.Error("the new episode reused the first one's key; the queue would drop it")
	}
}

// An outage is not a refusal: retrying can succeed, and payments have not
// stopped. Nothing is said.
func TestAnOutageDuringRefreshTellsNobody(t *testing.T) {
	w := newRefreshWorld(t)
	w.mercado.err = &mp.APIError{StatusCode: http.StatusBadGateway}

	if _, err := w.f.service.RefreshMPCredentials(t.Context(), w.id, ""); err == nil {
		t.Fatal("want the outage reported")
	}
	if len(w.f.notify.sent) != 0 {
		t.Errorf("an outage was announced as a refusal: %+v", w.f.notify.sent)
	}
}

// A notice the queue refused is not recorded as told, so the next refusal of
// the same episode tries again.
func TestARefreshNoticeTheQueueRefusedIsTriedAgain(t *testing.T) {
	w := newRefreshWorld(t)
	w.mercado.err = invalidGrant
	w.f.notify.refuseErr = errors.New("queue down")

	_, _ = w.f.service.RefreshMPCredentials(t.Context(), w.id, "")
	w.f.notify.refuseErr = nil
	_, _ = w.f.service.RefreshMPCredentials(t.Context(), w.id, "")

	if got := w.f.notify.of("refresh_failed"); len(got) != 1 {
		t.Errorf("want the notice queued on the second refusal; got %d", len(got))
	}
}
