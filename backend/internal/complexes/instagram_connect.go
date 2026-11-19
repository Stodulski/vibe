package complexes

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/instagram"
)

// This file is a complex's Instagram connection: Business Login for Instagram,
// so that Vibe can later publish stories for the complex. It is a flow of its
// own next to the MercadoPago one and shares its attempt store shape
// (MPOAuthAttempts, under the "igoauth:" namespace).
//
// The flow differs from MercadoPago's in one way that shapes everything here:
// Meta redirects the owner's browser to the API itself
// (GET /api/v1/instagram/callback), which has no complex in its path and no
// guard. So the attempt is bound twice before anything is exchanged:
//
//   - to the browser, by an HttpOnly state cookie set on connect and only sent
//     to the callback path. Without it, an owner could start a connection for
//     their own complex and get someone else to approve it with their
//     Instagram account, which would hand the first owner publishing rights
//     on the second one's account.
//   - to the complex and the owner, by the server-side attempt the state
//     names: single use, 15 minutes, and spent before the code is exchanged.
//
// The complex the callback writes for comes from that attempt, never from the
// request, and the write runs scoped to it (data.ContextWithTenant), so the
// route needs no cross-tenant grant.

const (
	// InstagramCallbackPath is the redirect URI's path, registered with Meta,
	// and the only path the state cookie is sent to.
	InstagramCallbackPath = "/api/v1/instagram/callback"
	instagramStateCookie  = "instagram_oauth_state"
	instagramAttemptTTL   = 15 * time.Minute
	// instagramRefreshWindow is how close to expiry a token is refreshed, and
	// instagramRefreshMinAge how old it must be: Meta refuses to refresh a
	// token younger than 24 hours.
	instagramRefreshWindow = 15 * 24 * time.Hour
	instagramRefreshMinAge = 24 * time.Hour
	instagramRefreshBatch  = 500
	// instagramSettingsPath is where the callback sends the owner back to.
	instagramSettingsPath = "/settings?tab=instagram&instagram="
)

// The reasons the callback reports to the panel with instagram=error.
const (
	instagramReasonDenied      = "denied"
	instagramReasonExpired     = "expired"
	instagramReasonFailed      = "failed"
	instagramReasonUnavailable = "unavailable"
)

// InstagramAPI is Business Login for Instagram. *instagram.Client satisfies it.
type InstagramAPI interface {
	AuthorizeURL(state string) string
	ExchangeCode(ctx context.Context, code string) (instagram.ShortToken, error)
	LongLived(ctx context.Context, shortToken string) (instagram.Token, error)
	Refresh(ctx context.Context, longToken string) (instagram.Token, error)
	Me(ctx context.Context, token string) (instagram.Account, error)
}

// InstagramStore is where connections live. complexstore.InstagramStore
// satisfies it; the token is sealed and opened inside it.
type InstagramStore interface {
	SaveInstagramConnection(ctx context.Context, c *complexstore.InstagramConnection) error
	GetInstagramConnection(ctx context.Context, complexID uuid.UUID) (*complexstore.InstagramConnection, error)
	DeleteInstagramConnection(ctx context.Context, complexID uuid.UUID) (bool, error)
	ListInstagramTokensDueForRefresh(ctx context.Context, now, refreshBefore, refreshedBefore time.Time, limit int) ([]complexstore.DueInstagramToken, []uuid.UUID, error)
	UpdateInstagramToken(ctx context.Context, complexID uuid.UUID, oldExpiresAt time.Time, token string, expiresAt, refreshedAt time.Time) (bool, error)
	MarkExpiredInstagramConnections(ctx context.Context, now time.Time) (int64, error)
	MarkInstagramNeedsReconnect(ctx context.Context, complexID uuid.UUID, oldExpiresAt time.Time) (bool, error)
	InstagramComplexOwner(ctx context.Context, complexID uuid.UUID) (uuid.UUID, error)
}

// InstagramConfig tunes the connection flow.
type InstagramConfig struct {
	// Available is config.Instagram.Available(): the flag is on and the app id
	// and secret are set. Off, status says enabled:false and connect and the
	// callback refuse; status and disconnect still work.
	Available bool
	// Secure marks the state cookie Secure; it is off only in development.
	Secure bool
	// FrontendURL is the panel origin the callback redirects to.
	FrontendURL    string
	TrustedProxies httpx.TrustedProxies
	Now            func() time.Time
}

// InstagramDeps is what the connection needs from outside.
type InstagramDeps struct {
	API      InstagramAPI
	Store    InstagramStore
	Attempts OAuthAttemptStore
	Audit    Recorder
	Respond  *httpx.Responder
	Logger   *slog.Logger
}

// Instagram serves the connection routes and runs the token refresh.
type Instagram struct {
	api      InstagramAPI
	store    InstagramStore
	attempts OAuthAttemptStore
	audit    Recorder
	respond  *httpx.Responder
	logger   *slog.Logger
	cfg      InstagramConfig
}

// NewInstagram returns the connection flow.
func NewInstagram(deps InstagramDeps, cfg InstagramConfig) *Instagram {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Instagram{
		api: deps.API, store: deps.Store, attempts: deps.Attempts, audit: deps.Audit,
		respond: deps.Respond, logger: deps.Logger, cfg: cfg,
	}
}

// InstagramCallback is what Meta sends back: the code and the state, or the
// error it reported instead.
type InstagramCallback struct {
	Code  string
	State string
	Error string
}

// instagramAttemptRecord is what an attempt stores under its state.
type instagramAttemptRecord struct {
	ComplexID uuid.UUID `json:"complex_id"`
	UserID    uuid.UUID `json:"user_id"`
}

// Status handles GET /api/v1/complexes/{id}/instagram.
func (ig *Instagram) Status(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		ig.respond.ServerError(w, r, errors.New("missing complex in context"))
		return
	}
	body := httpx.Envelope{
		"enabled":         ig.cfg.Available,
		"connected":       false,
		"needs_reconnect": false,
		"username":        nil,
		"account_type":    nil,
		"expires_at":      nil,
	}
	conn, err := ig.store.GetInstagramConnection(r.Context(), complex.ID)
	switch {
	case errors.Is(err, data.ErrRecordNotFound):
	case err != nil:
		ig.respond.ServerError(w, r, err)
		return
	default:
		usable := !conn.NeedsReconnect && conn.ExpiresAt.After(ig.cfg.Now())
		body["connected"] = usable
		body["needs_reconnect"] = !usable
		body["username"] = conn.Username
		body["account_type"] = conn.AccountType
		body["expires_at"] = conn.ExpiresAt.UTC()
	}
	ig.respond.JSON(w, r, http.StatusOK, body)
}

// Connect handles POST /api/v1/complexes/{id}/instagram/connect: it registers
// the attempt, binds it to this browser, and answers the consent page URL.
func (ig *Instagram) Connect(w http.ResponseWriter, r *http.Request) {
	if !ig.cfg.Available {
		ig.respond.Refuse(w, r, httpx.NotImplemented("instagram connection is not available"))
		return
	}
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		ig.respond.ServerError(w, r, errors.New("missing complex in context"))
		return
	}
	user, ok := httpx.ContextGetAuthenticatedUser(r)
	if !ok {
		ig.respond.ServerError(w, r, errors.New("missing user in context"))
		return
	}

	state, err := newInstagramState(complex.ID)
	if err != nil {
		ig.respond.ServerError(w, r, err)
		return
	}
	payload, err := json.Marshal(instagramAttemptRecord{ComplexID: complex.ID, UserID: user.ID})
	if err != nil {
		ig.respond.ServerError(w, r, fmt.Errorf("encoding instagram oauth attempt: %w", err))
		return
	}
	if err := ig.attempts.Store(r.Context(), complex.ID.String(), state, payload, instagramAttemptTTL); err != nil {
		ig.respond.ServerError(w, r, err)
		return
	}

	//nolint:gosec // G124: HttpOnly and SameSite are literal; Secure is set from the environment (off only in development).
	http.SetCookie(w, &http.Cookie{
		Name:     instagramStateCookie,
		Value:    state,
		Path:     InstagramCallbackPath,
		MaxAge:   int(instagramAttemptTTL.Seconds()),
		HttpOnly: true,
		Secure:   ig.cfg.Secure,
		SameSite: http.SameSiteLaxMode,
	})
	ig.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"authorize_url": ig.api.AuthorizeURL(state)})
}

// Callback handles GET /api/v1/instagram/callback, where Meta sends the
// owner's browser. It always answers with a redirect to the panel, carrying
// instagram=connected or instagram=error&reason=....
func (ig *Instagram) Callback(w http.ResponseWriter, r *http.Request, in InstagramCallback) {
	cookie, cookieErr := r.Cookie(instagramStateCookie)
	//nolint:gosec // G124: this clears the cookie (MaxAge -1); HttpOnly and SameSite are literal, Secure follows the environment.
	http.SetCookie(w, &http.Cookie{
		Name: instagramStateCookie, Value: "", Path: InstagramCallbackPath, MaxAge: -1,
		HttpOnly: true, Secure: ig.cfg.Secure, SameSite: http.SameSiteLaxMode,
	})

	if !ig.cfg.Available {
		ig.redirect(w, r, "", instagramReasonUnavailable)
		return
	}
	if cookieErr != nil || cookie.Value == "" || in.State == "" ||
		subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(in.State)) != 1 {
		ig.logger.Warn("instagram connect: state missing or not this browser's")
		ig.redirect(w, r, "", instagramReasonExpired)
		return
	}

	rec, err := ig.spendAttempt(r.Context(), in.State)
	if err != nil {
		if !errors.Is(err, ErrOAuthAttemptInvalid) {
			ig.logger.Error("instagram connect: could not spend the attempt", "error", err)
		}
		ig.redirect(w, r, "", instagramReasonExpired)
		return
	}
	if in.Error != "" || in.Code == "" {
		ig.logger.Info("instagram connect: consent not given", "complex_id", rec.ComplexID, "error", in.Error)
		ig.redirect(w, r, "", instagramReasonDenied)
		return
	}

	if !ig.stillOwns(r.Context(), rec) {
		ig.redirect(w, r, "", instagramReasonFailed)
		return
	}

	conn, err := ig.connect(r.Context(), rec.ComplexID, in.Code)
	if err != nil {
		reason := instagramReasonFailed
		if errors.Is(err, instagram.ErrUnavailable) {
			reason = instagramReasonUnavailable
		}
		ig.logger.Error("instagram connect: failed", "complex_id", rec.ComplexID, "error", err)
		ig.redirect(w, r, "", reason)
		return
	}

	userID := rec.UserID
	ig.audit.Record(audit.Entry{
		UserID:     &userID,
		ComplexID:  &rec.ComplexID,
		Action:     "instagram_connect",
		EntityType: "complex",
		EntityID:   &rec.ComplexID,
		NewValue:   map[string]string{"username": conn.Username, "account_type": conn.AccountType},
		IPAddress:  httpx.ClientIPFrom(r, ig.cfg.TrustedProxies),
	})
	ig.logger.Info("instagram connect: connected", "complex_id", rec.ComplexID, "username", conn.Username)
	ig.redirect(w, r, "connected", "")
}

// spendAttempt consumes the attempt the state names. The complex id prefixing
// the state only locates the attempt; the attempt's own record is what is
// trusted, and it must agree.
// stillOwns reports whether the person who started the attempt still owns the
// complex: it may have changed hands while they were on the consent page.
func (ig *Instagram) stillOwns(ctx context.Context, rec *instagramAttemptRecord) bool {
	owner, err := ig.store.InstagramComplexOwner(data.ContextWithTenant(ctx, rec.ComplexID), rec.ComplexID)
	if err != nil || owner != rec.UserID {
		ig.logger.Warn("instagram connect: the person who started it no longer owns the complex", "complex_id", rec.ComplexID, "error", err)
		return false
	}
	return true
}

func (ig *Instagram) spendAttempt(ctx context.Context, state string) (*instagramAttemptRecord, error) {
	prefix, _, found := strings.Cut(state, ".")
	complexID, err := uuid.Parse(prefix)
	if !found || err != nil {
		return nil, ErrOAuthAttemptInvalid
	}
	payload, err := ig.attempts.Consume(ctx, complexID.String(), state)
	if err != nil {
		return nil, err
	}
	var rec instagramAttemptRecord
	if err := json.Unmarshal(payload, &rec); err != nil {
		return nil, fmt.Errorf("decoding instagram oauth attempt: %w", err)
	}
	if rec.ComplexID != complexID {
		return nil, ErrOAuthAttemptInvalid
	}
	return &rec, nil
}

// connect exchanges the code, reads the account and stores the connection,
// scoped to the complex the attempt named.
func (ig *Instagram) connect(ctx context.Context, complexID uuid.UUID, code string) (*complexstore.InstagramConnection, error) {
	short, err := ig.api.ExchangeCode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}
	long, err := ig.api.LongLived(ctx, short.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("long-lived token: %w", err)
	}
	account, err := ig.api.Me(ctx, long.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("read account: %w", err)
	}
	if account.ID != "" && account.ID != short.UserID {
		return nil, errors.New("the token's account is not the one the code was issued for")
	}
	now := ig.cfg.Now()
	conn := &complexstore.InstagramConnection{
		ComplexID:       complexID,
		InstagramUserID: account.UserID,
		Username:        account.Username,
		AccountType:     account.AccountType,
		AccessToken:     long.AccessToken,
		ExpiresAt:       now.Add(long.ExpiresIn),
		ConnectedAt:     now,
		RefreshedAt:     now,
	}
	if err := ig.store.SaveInstagramConnection(data.ContextWithTenant(ctx, complexID), conn); err != nil {
		return nil, err
	}
	return conn, nil
}

// redirect sends the owner back to the panel's settings.
func (ig *Instagram) redirect(w http.ResponseWriter, r *http.Request, outcome, reason string) {
	target := strings.TrimSuffix(ig.cfg.FrontendURL, "/") + instagramSettingsPath
	if outcome != "" {
		target += outcome
	} else {
		target += "error&reason=" + url.QueryEscape(reason)
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// Disconnect handles DELETE /api/v1/complexes/{id}/instagram. It deletes the
// connection; disconnecting a complex with none is not an error. The token is
// not revoked at Meta: Instagram's API documents no revocation endpoint for
// the app to call; the owner removes the app from their Instagram settings.
func (ig *Instagram) Disconnect(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		ig.respond.ServerError(w, r, errors.New("missing complex in context"))
		return
	}
	deleted, err := ig.store.DeleteInstagramConnection(r.Context(), complex.ID)
	if err != nil {
		ig.respond.ServerError(w, r, err)
		return
	}
	if deleted {
		var userID *uuid.UUID
		if user, ok := httpx.ContextGetAuthenticatedUser(r); ok {
			userID = &user.ID
		}
		ig.audit.Record(audit.Entry{
			UserID:     userID,
			ComplexID:  &complex.ID,
			Action:     "instagram_disconnect",
			EntityType: "complex",
			EntityID:   &complex.ID,
			IPAddress:  httpx.ClientIPFrom(r, ig.cfg.TrustedProxies),
		})
	}
	ig.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"connected": false})
}

// InstagramRefreshOutcome is what one refresh run did, for its log line.
type InstagramRefreshOutcome struct {
	// Revoked are the tokens Meta refused to refresh, now flagged for reconnect;
	// Skipped the refreshes a reconnect raced, which stored nothing.
	Expired, Refreshed, Failed, Unreadable, Revoked, Skipped int
}

// RefreshTokens is the daily sweep: it marks expired connections as needing a
// reconnect, then refreshes every live token expiring within 15 days that is
// at least 24 hours old. A failure is logged and the token is tried again on
// the next run; it is never a reason to stop the sweep. ctx must cross
// tenants (the cron wrapper sets the bypass).
func (ig *Instagram) RefreshTokens(ctx context.Context) InstagramRefreshOutcome {
	var out InstagramRefreshOutcome
	if !ig.cfg.Available {
		return out
	}
	now := ig.cfg.Now()
	expired, err := ig.store.MarkExpiredInstagramConnections(ctx, now)
	if err != nil {
		ig.logger.Error("instagram refresh: could not mark expired connections", "error", err)
	}
	out.Expired = int(expired)

	due, unreadable, err := ig.store.ListInstagramTokensDueForRefresh(ctx, now, now.Add(instagramRefreshWindow), now.Add(-instagramRefreshMinAge), instagramRefreshBatch)
	if err != nil {
		ig.logger.Error("instagram refresh: could not list due tokens", "error", err)
		return out
	}
	for _, complexID := range unreadable {
		ig.logger.Error("instagram refresh: stored token cannot be opened", "complex_id", complexID)
	}
	out.Unreadable = len(unreadable)

	for _, d := range due {
		tok, err := ig.api.Refresh(ctx, d.AccessToken)
		if errors.Is(err, instagram.ErrRejected) {
			// Meta answered and refused: the owner revoked access. Say so now
			// rather than reporting connected until the token runs out.
			if _, markErr := ig.store.MarkInstagramNeedsReconnect(ctx, d.ComplexID, d.ExpiresAt); markErr != nil {
				ig.logger.Error("instagram refresh: could not flag a revoked token", "complex_id", d.ComplexID, "error", markErr)
			}
			out.Revoked++
			ig.logger.Warn("instagram refresh: Meta refused the refresh; the owner must reconnect", "complex_id", d.ComplexID, "error", err)
			continue
		}
		if err != nil {
			out.Failed++
			ig.logger.Warn("instagram refresh: refresh failed, retrying next run", "complex_id", d.ComplexID, "error", err)
			continue
		}
		at := ig.cfg.Now()
		stored, err := ig.store.UpdateInstagramToken(ctx, d.ComplexID, d.ExpiresAt, tok.AccessToken, at.Add(tok.ExpiresIn), at)
		if err != nil {
			out.Failed++
			ig.logger.Error("instagram refresh: could not store the refreshed token", "complex_id", d.ComplexID, "error", err)
			continue
		}
		if !stored {
			out.Skipped++
			continue
		}
		out.Refreshed++
	}
	return out
}

// newInstagramState is the complex id, a dot, and 32 random bytes. The id only
// tells the callback where the attempt is; the random part is what makes the
// state unguessable.
func newInstagramState(complexID uuid.UUID) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("instagram state: %w", err)
	}
	return complexID.String() + "." + base64.RawURLEncoding.EncodeToString(raw), nil
}
