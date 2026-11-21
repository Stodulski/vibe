package notifications

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// The account notices carry their recipient in the payload, because the
// account-deleted one has no account left to resolve it from, and each one is
// keyed on the event it reports: the same event is one notice, the next event
// is another.
func TestAccountNoticesAreKeyedOnTheEventTheyReport(t *testing.T) {
	cases := []struct {
		name     string
		taskType string
		send     func(s *Service, event string)
	}{
		{"password changed", TaskEmailPasswordChanged, func(s *Service, event string) {
			s.PasswordChanged(PasswordChangedEmail{To: "ana@example.com"}, event)
		}},
		{"email changed", TaskEmailEmailChanged, func(s *Service, event string) {
			s.EmailChanged(EmailChangedEmail{To: "ana@example.com", NewEmail: "ana@new.example"}, event)
		}},
		{"account deleted", TaskEmailAccountDeleted, func(s *Service, event string) {
			s.AccountDeleted(AccountDeletedEmail{To: "ana@example.com"}, event)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, q, mailer, _ := newTestService(t)
			s.RegisterWorkers()

			c.send(s, "event-1")
			c.send(s, "event-1")
			c.send(s, "event-2")

			if len(q.tasks) != 3 {
				t.Fatalf("want 3 tasks; got %v", q.types())
			}
			if q.tasks[0].dedupKey == "" || q.tasks[0].dedupKey != q.tasks[1].dedupKey {
				t.Error("the same event queued twice has different keys; the queue would send it twice")
			}
			if q.tasks[2].dedupKey == q.tasks[0].dedupKey {
				t.Error("a second event has the first one's key; the queue would drop it")
			}

			raw, _ := json.Marshal(q.tasks[0].payload)
			if err := q.handlers[c.taskType](t.Context(), raw); err != nil {
				t.Fatalf("delivering: %v", err)
			}
			if len(mailer.sent) != 1 || mailer.sent[0].to != "ana@example.com" {
				t.Errorf("the notice did not reach the address it carries; got %v", mailer.sent)
			}
		})
	}
}

// The connection notices go to the owner's current address, resolved at
// delivery, and are keyed on the complex and the event or episode.
func TestConnectionNoticesReachTheOwnerOncePerEpisode(t *testing.T) {
	complexID := uuid.NewString()
	other := uuid.NewString()
	notice := func(id string) ConnectionNotice {
		return ConnectionNotice{OwnerID: uuid.NewString(), ComplexID: id, ComplexName: "Vibe Palermo"}
	}
	cases := []struct {
		name     string
		taskType string
		kind     string
		send     func(s *Service, n ConnectionNotice, event string)
	}{
		{"mp connected", TaskEmailOwnerMPConnected, "owner_mp_connected", func(s *Service, n ConnectionNotice, e string) {
			s.MercadoPagoConnected(n, e)
		}},
		{"mp disconnected", TaskEmailOwnerMPDisconnected, "owner_mp_disconnected", func(s *Service, n ConnectionNotice, e string) {
			s.MercadoPagoDisconnected(n, e)
		}},
		{"mp refresh failed", TaskEmailOwnerMPRefreshFailed, "owner_mp_refresh_failed", func(s *Service, n ConnectionNotice, e string) {
			if err := s.MercadoPagoRefreshFailed(n, e); err != nil {
				t.Fatalf("MercadoPagoRefreshFailed: %v", err)
			}
		}},
		{"instagram reconnect", TaskEmailOwnerInstagramReconnect, "owner_instagram_reconnect", func(s *Service, n ConnectionNotice, e string) {
			s.InstagramNeedsReconnect(n, e)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, q, mailer, users := newTestService(t)
			s.RegisterWorkers()

			c.send(s, notice(complexID), "episode-1")
			c.send(s, notice(complexID), "episode-1")
			c.send(s, notice(complexID), "episode-2")
			c.send(s, notice(other), "episode-1")

			keys := map[string]bool{}
			for _, task := range q.tasks {
				if task.dedupKey == "" {
					t.Fatal("a connection notice was queued without a dedup key")
				}
				keys[task.dedupKey] = true
			}
			if len(keys) != 3 {
				t.Errorf("want 3 distinct keys (same episode once, new episode, other complex); got %d", len(keys))
			}

			users.user.Email = "owner-now@example.com"
			raw, _ := json.Marshal(q.tasks[0].payload)
			if err := q.handlers[c.taskType](t.Context(), raw); err != nil {
				t.Fatalf("delivering: %v", err)
			}
			if len(mailer.sent) != 1 || mailer.sent[0].kind != c.kind || mailer.sent[0].to != "owner-now@example.com" {
				t.Errorf("want one %s to the owner's current address; got %v", c.kind, mailer.sent)
			}
		})
	}
}

// The refresh-failure notice reports a queue refusal, so the caller does not
// record an episode as told when nothing was queued.
func TestMercadoPagoRefreshFailedReturnsARefusedEnqueue(t *testing.T) {
	s, q, _, _ := newTestService(t)
	q.failOn = map[string]error{TaskEmailOwnerMPRefreshFailed: errors.New("queue down")}

	err := s.MercadoPagoRefreshFailed(ConnectionNotice{OwnerID: uuid.NewString(), ComplexID: uuid.NewString()}, "episode-1")
	if err == nil {
		t.Error("a refused enqueue was reported as queued")
	}
}
