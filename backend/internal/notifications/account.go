package notifications

import (
	"context"

	"github.com/stodulski/vibe-server/internal/jobs"
)

// The security notices of an account: the password changed, the address moved,
// the account was deleted. Each one tells the person about something that
// already happened, so it carries no link that does anything and no token, and
// it has no opt-out: it is how someone learns their account was touched by
// someone else.
const (
	TaskEmailPasswordChanged = "email:password_changed"
	TaskEmailEmailChanged    = "email:email_changed"
	TaskEmailAccountDeleted  = "email:account_deleted"
)

// PasswordChangedEmail tells an account its password was changed, whether from
// a signed-in session or through a reset link.
type PasswordChangedEmail struct {
	To        string `json:"to"`
	FirstName string `json:"first_name"`
}

// EmailChangedEmail tells the account's PREVIOUS address that the account now
// answers at NewEmail. It goes to the old address because that is the one the
// owner still reads if the change was not theirs.
type EmailChangedEmail struct {
	To        string `json:"to"`
	FirstName string `json:"first_name"`
	NewEmail  string `json:"new_email"`
}

// AccountDeletedEmail confirms an account was deleted. The address is captured
// before the row goes and travels in the payload: there is no account left to
// resolve it from at delivery.
type AccountDeletedEmail struct {
	To        string `json:"to"`
	FirstName string `json:"first_name"`
}

// PasswordChanged queues the notice of one password change. eventID names that
// change (the reset token spent, or a fresh id for a change made from a
// session), so a second change later is a second notice and a repeated enqueue
// of the same change is one.
func (s *Service) PasswordChanged(e PasswordChangedEmail, eventID string) {
	s.queue.Enqueue(TaskEmailPasswordChanged, e, dedup(TaskEmailPasswordChanged, e.To, eventID))
}

// EmailChanged queues the notice to the previous address of one confirmed
// email change. requestID is the change request that was confirmed.
func (s *Service) EmailChanged(e EmailChangedEmail, requestID string) {
	s.queue.Enqueue(TaskEmailEmailChanged, e, dedup(TaskEmailEmailChanged, e.To, requestID))
}

// AccountDeleted queues the confirmation that an account was deleted. userID
// is the account, which is deleted once.
func (s *Service) AccountDeleted(e AccountDeletedEmail, userID string) {
	s.queue.Enqueue(TaskEmailAccountDeleted, e, dedup(TaskEmailAccountDeleted, e.To, userID))
}

func (s *Service) registerAccountWorkers() {
	s.queue.RegisterHandler(handle(TaskEmailPasswordChanged, func(ctx context.Context, p PasswordChangedEmail) error {
		return s.mailer.SendPasswordChanged(ctx, p.To, p.FirstName)
	}))
	s.queue.RegisterHandler(handle(TaskEmailEmailChanged, func(ctx context.Context, p EmailChangedEmail) error {
		return s.mailer.SendEmailChanged(ctx, p.To, p.FirstName, p.NewEmail)
	}))
	s.queue.RegisterHandler(handle(TaskEmailAccountDeleted, func(ctx context.Context, p AccountDeletedEmail) error {
		return s.mailer.SendAccountDeleted(ctx, p.To, p.FirstName)
	}))
}

// The notices about a complex's connections to MercadoPago and Instagram. They
// go to the owner, whose address is resolved at delivery like every owner
// email.
const (
	TaskEmailOwnerMPConnected          = "email:owner_mp_connected"
	TaskEmailOwnerMPDisconnected       = "email:owner_mp_disconnected"
	TaskEmailOwnerMPRefreshFailed      = "email:owner_mp_refresh_failed"
	TaskEmailOwnerInstagramReconnect   = "email:owner_instagram_reconnect"
	connectionNoticeMPConnected        = "mp_connected"
	connectionNoticeMPDisconnected     = "mp_disconnected"
	connectionNoticeMPRefreshFailed    = "mp_refresh_failed"
	connectionNoticeInstagramReconnect = "instagram_reconnect"
)

// ConnectionNotice is an owner notice about one complex's connection.
type ConnectionNotice struct {
	OwnerID     string `json:"owner_id"`
	ComplexID   string `json:"-"`
	ComplexName string `json:"complex_name"`
}

// MercadoPagoConnected tells the owner the complex can now take payments
// through MercadoPago. eventID names this connection.
func (s *Service) MercadoPagoConnected(n ConnectionNotice, eventID string) {
	s.queue.Enqueue(TaskEmailOwnerMPConnected, n, connectionKey(TaskEmailOwnerMPConnected, n, eventID))
}

// MercadoPagoDisconnected tells the owner the complex stopped taking online
// payments. eventID names this disconnection.
func (s *Service) MercadoPagoDisconnected(n ConnectionNotice, eventID string) {
	s.queue.Enqueue(TaskEmailOwnerMPDisconnected, n, connectionKey(TaskEmailOwnerMPDisconnected, n, eventID))
}

// MercadoPagoRefreshFailed tells the owner MercadoPago refused to renew the
// connection, so online payments stop until they reconnect. episode names the
// failure episode: the same episode is one notice however often it is queued.
// It returns the queue's refusal, because the caller records the episode as
// told only once the notice is recorded.
func (s *Service) MercadoPagoRefreshFailed(n ConnectionNotice, episode string) error {
	return s.queue.TryEnqueue(TaskEmailOwnerMPRefreshFailed, n, connectionKey(TaskEmailOwnerMPRefreshFailed, n, episode))
}

// InstagramNeedsReconnect tells the owner the complex's Instagram account
// stopped accepting Vibe, so stories pause until they reconnect. episode names
// the token that stopped working: one notice per episode.
func (s *Service) InstagramNeedsReconnect(n ConnectionNotice, episode string) {
	s.queue.Enqueue(TaskEmailOwnerInstagramReconnect, n, connectionKey(TaskEmailOwnerInstagramReconnect, n, episode))
}

// connectionKey is the dedup key of a connection notice: the task, the complex
// and the event or episode. A blank part is no key, for the reason dedup gives.
func connectionKey(taskType string, n ConnectionNotice, event string) string {
	if n.ComplexID == "" || event == "" {
		return ""
	}
	return jobs.DedupKey(taskType, n.ComplexID, event)
}

func (s *Service) registerConnectionWorkers() {
	for taskType, kind := range map[string]string{
		TaskEmailOwnerMPConnected:        connectionNoticeMPConnected,
		TaskEmailOwnerMPDisconnected:     connectionNoticeMPDisconnected,
		TaskEmailOwnerMPRefreshFailed:    connectionNoticeMPRefreshFailed,
		TaskEmailOwnerInstagramReconnect: connectionNoticeInstagramReconnect,
	} {
		s.queue.RegisterHandler(handle(taskType, func(ctx context.Context, p ConnectionNotice) error {
			to, err := s.ownerAddress(ctx, taskType, p.OwnerID)
			if err != nil {
				return err
			}
			return s.sendConnectionNotice(ctx, kind, to, p.ComplexName)
		}))
	}
}

func (s *Service) sendConnectionNotice(ctx context.Context, kind, to, complexName string) error {
	switch kind {
	case connectionNoticeMPConnected:
		return s.mailer.SendOwnerMPConnected(ctx, to, complexName)
	case connectionNoticeMPDisconnected:
		return s.mailer.SendOwnerMPDisconnected(ctx, to, complexName)
	case connectionNoticeMPRefreshFailed:
		return s.mailer.SendOwnerMPRefreshFailed(ctx, to, complexName)
	default:
		return s.mailer.SendOwnerInstagramReconnect(ctx, to, complexName)
	}
}
