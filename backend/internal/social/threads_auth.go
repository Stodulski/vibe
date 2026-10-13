package social

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/stodulski/vibe-server/internal/httpx"
)

const (
	// threadsStateCookie binds the consent page to the browser that asked for
	// it. Without it, anyone could finish someone else's connection with their
	// own Threads code.
	threadsStateCookie = "threads_oauth_state"
	// threadsCallbackPath is the one redirect URI registered with Meta, and the
	// only path the state cookie is sent to.
	threadsCallbackPath = "/api/v1/social/threads/callback"
	threadsStateTTL     = 10 * time.Minute
)

// ThreadsAuthConfig tunes the connection flow.
type ThreadsAuthConfig struct {
	// Configured is false when THREADS_APP_ID or THREADS_APP_SECRET is unset.
	Configured bool
	// Secure marks the state cookie Secure; it is off only in development.
	Secure      bool
	FrontendURL string
	Now         func() time.Time
}

// ThreadsCallback is what Meta sends back to the callback: the code and the
// state, or the error it reported instead.
type ThreadsCallback struct {
	Code      string
	State     string
	ErrorCode string
}

// ThreadsAuth connects Vibe's Threads account. Connect sends the superadmin's
// browser to Meta's consent page; Callback exchanges the code and stores the
// long-lived token, sealed at rest by the store.
type ThreadsAuth struct {
	api     ThreadsAPI
	store   ThreadsStore
	cfg     ThreadsAuthConfig
	respond *httpx.Responder
	logger  *slog.Logger
}

// NewThreadsAuth returns the connection flow.
func NewThreadsAuth(api ThreadsAPI, store ThreadsStore, cfg ThreadsAuthConfig, respond *httpx.Responder, logger *slog.Logger) *ThreadsAuth {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &ThreadsAuth{api: api, store: store, cfg: cfg, respond: respond, logger: logger}
}

// Connect starts the consent flow. The caller is already a superadmin, so this
// only mints the state and redirects.
func (a *ThreadsAuth) Connect(w http.ResponseWriter, r *http.Request) {
	if !a.cfg.Configured {
		a.respond.Refuse(w, r, httpx.NotImplemented("threads is not configured: THREADS_APP_ID and THREADS_APP_SECRET must be set"))
		return
	}
	state, err := newThreadsState()
	if err != nil {
		a.respond.ServerError(w, r, err)
		return
	}
	//nolint:gosec // G124: HttpOnly and SameSite are literal; Secure is set from the environment (off only in development).
	http.SetCookie(w, &http.Cookie{
		Name:     threadsStateCookie,
		Value:    state,
		Path:     threadsCallbackPath,
		MaxAge:   int(threadsStateTTL.Seconds()),
		HttpOnly: true,
		Secure:   a.cfg.Secure,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, a.api.AuthorizeURL(state), http.StatusFound)
}

// Callback finishes the consent flow. It answers with a short page, never JSON,
// because the owner reaches it by a redirect from Meta.
func (a *ThreadsAuth) Callback(w http.ResponseWriter, r *http.Request, in ThreadsCallback) {
	cookie, cookieErr := r.Cookie(threadsStateCookie)
	//nolint:gosec // G124: this clears the cookie (MaxAge -1); HttpOnly and SameSite are literal, Secure follows the environment.
	http.SetCookie(w, &http.Cookie{Name: threadsStateCookie, Value: "", Path: threadsCallbackPath, MaxAge: -1, HttpOnly: true, Secure: a.cfg.Secure, SameSite: http.SameSiteLaxMode})

	switch {
	case in.ErrorCode != "":
		a.page(w, http.StatusBadRequest, "Threads no autorizó la conexión. No se guardó nada.")
		return
	case cookieErr != nil || cookie.Value == "" || in.State == "" ||
		subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(in.State)) != 1:
		a.page(w, http.StatusBadRequest, "La sesión de conexión venció o no coincide. Volvé a empezar desde el enlace de conexión.")
		return
	case in.Code == "":
		a.page(w, http.StatusBadRequest, "Threads no envió el código de autorización.")
		return
	}

	conn, err := a.connect(r.Context(), strings.TrimSuffix(in.Code, "#_"))
	if err != nil {
		a.logger.Error("social: threads connection failed", "error", err)
		a.page(w, http.StatusBadGateway, "No se pudo completar la conexión con Threads. Volvé a intentarlo.")
		return
	}
	a.page(w, http.StatusOK, "Threads conectado: @"+conn.Username+". Ya podés programar posts para Threads.")
}

// connect exchanges the code for a long-lived token and stores it. The account
// the token belongs to must be the one the exchange named.
func (a *ThreadsAuth) connect(ctx context.Context, code string) (ThreadsConnection, error) {
	short, err := a.api.Exchange(ctx, code)
	if err != nil {
		return ThreadsConnection{}, fmt.Errorf("social: exchange threads code: %w", err)
	}
	long, err := a.api.LongLived(ctx, short.AccessToken)
	if err != nil {
		return ThreadsConnection{}, fmt.Errorf("social: long-lived threads token: %w", err)
	}
	id, username, err := a.api.Me(ctx, long.AccessToken)
	if err != nil {
		return ThreadsConnection{}, fmt.Errorf("social: read threads account: %w", err)
	}
	if id != short.UserID {
		return ThreadsConnection{}, errors.New("social: threads account does not match the exchanged user")
	}
	now := a.cfg.Now()
	conn := ThreadsConnection{
		UserID:      id,
		Username:    username,
		AccessToken: long.AccessToken,
		ConnectedAt: now,
		RefreshedAt: now,
		ExpiresAt:   now.Add(long.ExpiresIn),
	}
	if err := a.store.SaveThreads(ctx, conn); err != nil {
		return ThreadsConnection{}, fmt.Errorf("social: store threads connection: %w", err)
	}
	a.logger.Info("social: threads connected", "user_id", id, "username", username)
	return conn, nil
}

func (a *ThreadsAuth) page(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, threadsPageHTML, html.EscapeString(message), html.EscapeString(a.cfg.FrontendURL))
}

const threadsPageHTML = `<!doctype html>
<html lang="es"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Threads</title></head>
<body style="font-family: sans-serif; max-width: 32rem; margin: 4rem auto; padding: 0 1rem;">
<p>%s</p>
<p><a href="%s">Volver a Vibe</a></p>
</body></html>`

func newThreadsState() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("social: state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
