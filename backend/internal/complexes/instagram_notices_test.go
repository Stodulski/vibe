package complexes

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/instagram"
	"github.com/stodulski/vibe-server/internal/notifications"
)

type igNoticeSent struct {
	notice  notifications.ConnectionNotice
	episode string
}

// stubIGNotifier records the reconnect notices, and collapses the ones the
// queue would collapse: the same complex and episode is one job.
type stubIGNotifier struct {
	mu   sync.Mutex
	sent []igNoticeSent
}

func (n *stubIGNotifier) InstagramNeedsReconnect(notice notifications.ConnectionNotice, episode string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.sent = append(n.sent, igNoticeSent{notice, episode})
}

// distinct is what reaches the owner once the queue's dedup key has had its
// say: one email per complex and episode.
func (n *stubIGNotifier) distinct() []igNoticeSent {
	n.mu.Lock()
	defer n.mu.Unlock()
	seen := map[string]bool{}
	var out []igNoticeSent
	for _, s := range n.sent {
		k := s.notice.ComplexID + "|" + s.episode
		if s.episode == "" || !seen[k] {
			seen[k] = true
			out = append(out, s)
		}
	}
	return out
}

func (s *memIGStore) ComplexOwnerAndName(_ context.Context, id uuid.UUID) (uuid.UUID, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner, ok := s.owners[id]
	if !ok {
		return uuid.Nil, "", data.ErrRecordNotFound
	}
	return owner, "Vibe Palermo", nil
}

func (s *memIGStore) ListUnflaggedExpiredInstagramConnections(_ context.Context, now time.Time, _ int) ([]complexstore.ExpiredInstagramConnection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []complexstore.ExpiredInstagramConnection
	for id, c := range s.conns {
		if !c.NeedsReconnect && !c.ExpiresAt.After(now) {
			out = append(out, complexstore.ExpiredInstagramConnection{ComplexID: id, ExpiresAt: c.ExpiresAt})
		}
	}
	return out, nil
}

func withIGNotices(h *igHarness) *stubIGNotifier {
	n := &stubIGNotifier{}
	h.ig.notify, h.ig.notices = n, h.store
	return n
}

// A token Meta refuses to refresh tells the owner to reconnect, once: the
// next sweeps meet the same flagged connection and say nothing. Once the owner
// reconnects, a new refusal is a new episode and a new email.
func TestARevokedInstagramTokenTellsTheOwnerOncePerEpisode(t *testing.T) {
	h := newIGHarness(t, true)
	n := withIGNotices(h)
	expires := time.Now().Add(10 * 24 * time.Hour).Truncate(time.Second)
	id := h.complex.ID
	h.store.conns[id] = &complexstore.InstagramConnection{ComplexID: id, AccessToken: "tok-revoked", ExpiresAt: expires}
	h.meta.refreshFails["tok-revoked"] = http.StatusBadRequest

	h.ig.RefreshTokens(t.Context())
	h.ig.RefreshTokens(t.Context())

	got := n.distinct()
	if len(got) != 1 {
		t.Fatalf("want one reconnect notice for the episode; got %+v", n.sent)
	}
	if got[0].notice.OwnerID != h.owner.String() || got[0].notice.ComplexID != id.String() || got[0].episode == "" {
		t.Errorf("the notice names the wrong owner or complex, or no episode: %+v", got[0])
	}

	// The owner reconnects: a new token, the flag cleared.
	if err := h.store.SaveInstagramConnection(t.Context(), &complexstore.InstagramConnection{
		ComplexID: id, AccessToken: "tok-revoked-again", ExpiresAt: expires.Add(50 * 24 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	h.meta.refreshFails["tok-revoked-again"] = http.StatusBadRequest
	h.ig.RefreshTokens(t.Context())

	if got := n.distinct(); len(got) != 2 {
		t.Fatalf("want a second notice for the new episode; got %+v", n.sent)
	}
}

// A refresh that failed for any other reason is retried, not announced.
func TestAnInstagramRefreshOutageTellsNobody(t *testing.T) {
	h := newIGHarness(t, true)
	n := withIGNotices(h)
	id := h.complex.ID
	h.store.conns[id] = &complexstore.InstagramConnection{ComplexID: id, AccessToken: "tok-down", ExpiresAt: time.Now().Add(10 * 24 * time.Hour)}
	h.meta.refreshFails["tok-down"] = http.StatusInternalServerError

	h.ig.RefreshTokens(t.Context())
	if len(n.sent) != 0 {
		t.Errorf("an outage was announced: %+v", n.sent)
	}
}

// A token that ran out is flagged by the sweep and its owner told, once.
func TestAnExpiredInstagramTokenTellsTheOwnerOnce(t *testing.T) {
	h := newIGHarness(t, true)
	n := withIGNotices(h)
	id := h.complex.ID
	h.store.conns[id] = &complexstore.InstagramConnection{ComplexID: id, AccessToken: "tok-old", ExpiresAt: time.Now().Add(-time.Hour)}
	h.meta.refreshFails["tok-old"] = http.StatusBadRequest

	out := h.ig.RefreshTokens(t.Context())
	h.ig.RefreshTokens(t.Context())

	if !h.store.conns[id].NeedsReconnect {
		t.Fatal("the expired connection was not flagged")
	}
	if out.Expired != 1 {
		t.Errorf("outcome = %+v, want 1 expired", out)
	}
	if got := n.distinct(); len(got) != 1 || got[0].notice.ComplexID != id.String() {
		t.Errorf("want one reconnect notice; got %+v", n.sent)
	}
}

// The publisher meeting a refused token (code 190) flags the connection and
// tells the owner; the next story finds the connection flagged and says
// nothing more.
func TestAnInvalidTokenWhilePublishingTellsTheOwner(t *testing.T) {
	h := newStoryHarness(t)
	n := &stubIGNotifier{}
	owner := uuid.New()
	h.stories.notify = n
	h.stories.owners = ownerReaderFunc(func(id uuid.UUID) (uuid.UUID, string, error) { return owner, "Vibe Palermo", nil })
	h.meta.createFail = &graphFailure{status: http.StatusBadRequest, code: instagram.CodeInvalidToken}
	h.queueFreed(t, h.now.Add(3*time.Hour))
	h.queueSecondFreed(t, h.now.Add(4*time.Hour))

	h.stories.PublishDue(t.Context())
	h.stories.PublishDue(t.Context())

	got := n.distinct()
	if len(got) != 1 || got[0].notice.OwnerID != owner.String() || got[0].notice.ComplexID != h.complex.String() {
		t.Errorf("want one reconnect notice for the owner; got %+v", n.sent)
	}
}

// A 190 for a token the owner already replaced flags nothing and tells
// nobody: the new connection is fine.
func TestAnInvalidOldTokenWhilePublishingTellsNobody(t *testing.T) {
	h := newStoryHarness(t)
	n := &stubIGNotifier{}
	h.stories.notify = n
	h.stories.owners = ownerReaderFunc(func(uuid.UUID) (uuid.UUID, string, error) { return uuid.New(), "Vibe", nil })
	h.meta.createFail = &graphFailure{status: http.StatusBadRequest, code: instagram.CodeInvalidToken}
	h.meta.onCreate = func() {
		h.store.mu.Lock()
		h.store.accounts[h.complex].ExpiresAt = h.now.Add(60 * 24 * time.Hour)
		h.store.mu.Unlock()
	}
	h.queueFreed(t, h.now.Add(3*time.Hour))

	h.stories.PublishDue(t.Context())
	if len(n.sent) != 0 {
		t.Errorf("a renewed connection was announced as needing reconnect: %+v", n.sent)
	}
}

type ownerReaderFunc func(uuid.UUID) (uuid.UUID, string, error)

func (f ownerReaderFunc) ComplexOwnerAndName(_ context.Context, id uuid.UUID) (uuid.UUID, string, error) {
	return f(id)
}
