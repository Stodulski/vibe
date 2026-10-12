package social

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/storage"
	"github.com/stodulski/vibe-server/internal/validator"
)

// Authenticator decides whether a bearer token may write posts.
type Authenticator interface {
	Verify(ctx context.Context, raw string) error
}

// Store is the persistence the sync endpoints need.
type Store interface {
	// Upsert stores a post's content. It returns ErrFrozen when a network has
	// already published the post, or a publish run holds it.
	Upsert(ctx context.Context, p Post) error
	// StageMedia records an upload so a later post upsert can name it.
	StageMedia(ctx context.Context, postID string, m Media) error
	// StagedMedia returns the staged uploads for postID among names.
	StagedMedia(ctx context.Context, postID string, names []string) ([]Media, error)
	// IsFrozen reports whether a post exists and its content may no longer change.
	IsFrozen(ctx context.Context, id string) (bool, error)
	// CancelUnpublished cancels every scheduled post whose id is not in keep.
	CancelUnpublished(ctx context.Context, keep []string) ([]string, error)
}

// MediaStorage stores an upload and tells where a platform can fetch it.
type MediaStorage interface {
	PutObject(ctx context.Context, obj storage.Object) error
	PublicURL(key string) string
}

const maxReconcileIDs = 500

// Handler serves the content sync endpoints. They are reachable from the
// internet and authenticated by the GitHub OIDC token, not by a session.
type Handler struct {
	auth    Authenticator
	store   Store
	media   MediaStorage
	respond *httpx.Responder
	logger  *slog.Logger
}

// NewHandler returns the sync handler. media may be nil, which answers media
// uploads with 501 rather than storing bytes nowhere.
func NewHandler(auth Authenticator, store Store, media MediaStorage, respond *httpx.Responder, logger *slog.Logger) *Handler {
	return &Handler{auth: auth, store: store, media: media, respond: respond, logger: logger}
}

// PutPost stores the content of post id. The media it names must already be
// uploaded. A post a network has published answers 409 and is left unchanged.
func (h *Handler) PutPost(w http.ResponseWriter, r *http.Request, id string) {
	if !h.authorize(w, r) {
		return
	}
	var input struct {
		PublishAt string   `json:"publish_at"`
		Networks  []string `json:"networks"`
		Type      string   `json:"type"`
		Caption   string   `json:"caption"`
		AltText   string   `json:"alt_text"`
		Media     []string `json:"media"`
		Draft     bool     `json:"draft"`
	}
	if err := httpx.ReadJSON(w, r, &input); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	post, errs := ValidatePost(id, Fields{
		PublishAt: input.PublishAt, Networks: input.Networks, Type: input.Type,
		Caption: input.Caption, AltText: input.AltText, Media: input.Media, Draft: input.Draft,
	})
	if errs != nil {
		h.respond.FailedValidation(w, r, errs)
		return
	}
	if !h.attachStagedMedia(w, r, &post) {
		return
	}

	switch err := h.store.Upsert(r.Context(), post); {
	case errors.Is(err, ErrFrozen):
		h.respond.Refuse(w, r, httpx.Conflict("post is already published or being published; its content is frozen"))
		return
	case err != nil:
		h.respond.ServerError(w, r, err)
		return
	}
	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"id": id, "status": post.Status})
}

// attachStagedMedia replaces the names on post with the uploaded records, in
// the order the post lists them. It answers and returns false when one is
// missing or the store fails.
func (h *Handler) attachStagedMedia(w http.ResponseWriter, r *http.Request, post *Post) bool {
	names := make([]string, 0, len(post.Media))
	for _, m := range post.Media {
		names = append(names, m.Name)
	}
	staged, err := h.store.StagedMedia(r.Context(), post.ID, names)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return false
	}
	byName := make(map[string]Media, len(staged))
	for _, m := range staged {
		byName[m.Name] = m
	}
	media := make([]Media, 0, len(names))
	var missing []string
	for _, n := range names {
		m, ok := byName[n]
		if !ok {
			missing = append(missing, n)
			continue
		}
		media = append(media, m)
	}
	if len(missing) > 0 {
		h.respond.Refuse(w, r, httpx.Unprocessable("media not uploaded yet: "+strings.Join(missing, ", ")))
		return false
	}
	post.Media = media
	return true
}

// PutMedia stores one file of a post under its content hash and stages it for
// the post upsert. The body is the raw file.
func (h *Handler) PutMedia(w http.ResponseWriter, r *http.Request, id, name string) {
	if !h.authorize(w, r) {
		return
	}
	format, err := CheckMediaUpload(name, r.Header.Get("Content-Type"), 1)
	if err != nil {
		h.refuseMedia(w, r, err)
		return
	}
	if !postIDPattern.MatchString(id) {
		h.respond.Refuse(w, r, httpx.Unprocessable("post id must be lowercase letters, digits and hyphens"))
		return
	}
	if h.media == nil {
		h.respond.Refuse(w, r, httpx.NotImplemented("media storage is not configured"))
		return
	}
	if h.frozen(w, r, id) {
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, SizeLimit(format)))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			h.refuseMedia(w, r, ErrMediaTooLarge)
			return
		}
		h.respond.BadRequest(w, r, err)
		return
	}
	if len(body) == 0 {
		h.refuseMedia(w, r, ErrMediaSize)
		return
	}
	if !SniffMatches(format, body) {
		h.respond.Refuse(w, r, httpx.UnsupportedMediaType("the body is not the file type its name declares"))
		return
	}
	h.storeMedia(w, r, id, name, format, body)
}

func (h *Handler) storeMedia(w http.ResponseWriter, r *http.Request, id, name string, format MediaFormat, body []byte) {
	sum := sha256.Sum256(body)
	digest := hex.EncodeToString(sum[:])
	key := MediaKey(digest, format)
	url := h.media.PublicURL(key)
	if url == "" {
		h.respond.Refuse(w, r, httpx.Unavailable("the public media URL is not configured"))
		return
	}
	contentType := wantContentType(format)
	if err := h.media.PutObject(r.Context(), storage.Object{Key: key, ContentType: contentType, Body: body}); err != nil {
		h.respond.ServerError(w, r, err)
		return
	}
	m := Media{Name: name, URL: url, ContentType: contentType, SHA256: digest, SizeBytes: int64(len(body))}
	if err := h.store.StageMedia(r.Context(), id, m); err != nil {
		h.respond.ServerError(w, r, err)
		return
	}
	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"name": m.Name, "url": m.URL, "sha256": m.SHA256, "content_type": m.ContentType, "size_bytes": m.SizeBytes,
	})
}

// Reconcile cancels every scheduled post whose id is not in the list the sync
// sends. The list must be present, even when empty.
func (h *Handler) Reconcile(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	var input struct {
		IDs []string `json:"ids"`
	}
	if err := httpx.ReadJSON(w, r, &input); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}
	v := validator.New()
	v.Check(input.IDs != nil, "ids", "must be provided, possibly empty")
	v.Check(len(input.IDs) <= maxReconcileIDs, "ids", "must hold 500 ids at most")
	for _, id := range input.IDs {
		v.Check(postIDPattern.MatchString(id), "ids", "contains an invalid post id")
	}
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	cancelled, err := h.store.CancelUnpublished(r.Context(), input.IDs)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}
	if cancelled == nil {
		cancelled = []string{}
	}
	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"cancelled": cancelled})
}

// authorize checks the bearer token and answers the request itself when it
// fails. It returns true when the caller may continue.
func (h *Handler) authorize(w http.ResponseWriter, r *http.Request) bool {
	raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	raw = strings.TrimSpace(raw)
	if !ok || raw == "" {
		h.respond.InvalidAuthenticationToken(w, r)
		return false
	}
	err := h.auth.Verify(r.Context(), raw)
	switch {
	case err == nil:
		return true
	case errors.Is(err, ErrForbidden):
		h.respond.Refuse(w, r, httpx.Forbidden("this token is not from the social sync workflow"))
	default:
		h.logger.Info("social: token refused", "error", err)
		h.respond.InvalidAuthenticationToken(w, r)
	}
	return false
}

// frozen answers 409 and returns true when post id may no longer change. A
// store failure answers 500 and also returns true, so the caller stops.
func (h *Handler) frozen(w http.ResponseWriter, r *http.Request, id string) bool {
	frozen, err := h.store.IsFrozen(r.Context(), id)
	switch {
	case err != nil:
		h.respond.ServerError(w, r, err)
		return true
	case frozen:
		h.respond.Refuse(w, r, httpx.Conflict("post is already published or being published; its media is frozen"))
		return true
	default:
		return false
	}
}

func (h *Handler) refuseMedia(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrMediaType):
		h.respond.Refuse(w, r, httpx.UnsupportedMediaType(err.Error()))
	case errors.Is(err, ErrMediaTooLarge):
		h.respond.Refuse(w, r, httpx.TooLarge(err.Error()))
	default:
		h.respond.Refuse(w, r, httpx.Unprocessable(err.Error()))
	}
}
