package complexes

import (
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/validator"
)

var allowedContentTypes = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
}

const maxFileSize = 5 * 1024 * 1024 // 5MB

// uploadKeyPrefix is the storage namespace every object belonging to a complex
// lives under.
//
// PresignUpload builds its keys from this and DeleteUpload refuses keys outside
// it, so the two can never disagree about which objects a complex owns. Written
// as a literal in both places, the delete guard would silently stop matching the
// day the presign layout changed — which is exactly how the cross-tenant delete
// this helper closes came to exist.
func uploadKeyPrefix(complexID uuid.UUID) string {
	return fmt.Sprintf("complexes/%s/", complexID)
}

// PresignUpload handles POST /api/v1/complexes/:id/uploads/presign, returning a
// short-lived URL the browser uploads straight to, so image bytes never pass
// through this service. The object key is namespaced to the complex.
func (h *Handler) PresignUpload(w http.ResponseWriter, r *http.Request) {
	if h.storage == nil {
		h.respond.Error(w, r, http.StatusNotImplemented, "image uploads are not configured")
		return
	}

	var input struct {
		Type        string `json:"type"`
		ContentType string `json:"content_type"`
		FileSize    int64  `json:"file_size"`
	}

	err := httpx.ReadJSON(w, r, &input)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	v.Check(input.Type == "logo" || input.Type == "cover", "type", "must be 'logo' or 'cover'")
	v.Check(allowedContentTypes[input.ContentType] != "", "content_type", "must be image/jpeg, image/png, or image/webp")
	v.Check(input.FileSize > 0, "file_size", "must be greater than 0")
	v.Check(input.FileSize <= maxFileSize, "file_size", fmt.Sprintf("must not exceed %d bytes (5MB)", maxFileSize))

	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}
	// The validated type is the one that gets signed, and its extension is the
	// one the key carries. Both were hardcoded to webp while the validation
	// above accepted three types, so a client legitimately declaring image/jpeg
	// got a URL that only accepts image/webp — the upload either failed at R2
	// or succeeded by lying about itself. allowedContentTypes' extension values
	// were dead until now.
	ext := allowedContentTypes[input.ContentType]
	key := fmt.Sprintf("%s%s/%s.%s", uploadKeyPrefix(complex.ID), input.Type, uuid.New(), ext)

	uploadURL, publicURL, err := h.storage.GeneratePresignedPUT(r.Context(), key, input.ContentType, input.FileSize, 10*time.Minute)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"upload_url": uploadURL,
		"public_url": publicURL,
		"key":        key,
	})
}

// DeleteUpload handles DELETE /api/v1/complexes/:id/uploads. It accepts only
// URLs inside our own storage, and only those inside the calling complex's own
// namespace within it, so neither an arbitrary URL nor another tenant's URL can
// be used to delete something else.
func (h *Handler) DeleteUpload(w http.ResponseWriter, r *http.Request) {
	if h.storage == nil {
		h.respond.Error(w, r, http.StatusNotImplemented, "image uploads are not configured")
		return
	}

	var input struct {
		URL string `json:"url"`
	}

	err := httpx.ReadJSON(w, r, &input)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	v.Check(input.URL != "", "url", "must be provided")
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	key, ok := h.storage.KeyFromPublicURL(input.URL)
	if !ok {
		h.respond.Error(w, r, http.StatusBadRequest, "URL does not belong to this storage")
		return
	}

	// The route guard proved the caller owns the complex in the *path*; nothing
	// so far proves they own the object in the *body*. Every venue's logo_url
	// and cover_url are public (the sitemap enumerates the slugs and the public
	// complex endpoint hands out both URLs), so without this comparison one
	// authenticated owner could delete every rival venue's images.
	//
	// H-12: strings.HasPrefix compares raw strings, and a key of
	// complexes/{A}/../{B}/logo/x.jpg lexically starts with A's prefix — the
	// comparison passed a delete that targets B's object straight through.
	// CPX-04 reproduced this against production: no cross-tenant object was
	// actually removed, but only because the storage SDK normalizes ".." out
	// of the path it sends to R2 while the request's signature was computed
	// over the raw key, so R2 answered SignatureDoesNotMatch (403) and this
	// handler surfaced it as a caller-triggered 500 — an accident of URL
	// normalization stood in for the check, not the check itself.
	//
	// Reject outright rather than clean-and-allow: PresignUpload only ever
	// builds keys shaped complexes/{id}/{type}/{uuid}.{ext}, so a legitimate
	// key never contains a ".." segment at all. A caller-supplied key that
	// does is malformed, and a rejection says so — resolving it with
	// path.Clean and letting a passing comparison decide would silently
	// rewrite what the caller asked for into a different key entirely, which
	// is not a decision this endpoint should make quietly.
	if strings.Contains(key, "..") {
		h.respond.Error(w, r, http.StatusBadRequest, "URL does not belong to this storage")
		return
	}

	// Normalize before comparing, not after — path.Clean the key (collapsing
	// redundant slashes and "." segments) and compare *that* against the
	// prefix, then delete the cleaned key rather than the raw one. This is
	// also what turns the caller-triggered 500 from CPX-04 into a deliberate
	// 4xx: the previous strings.HasPrefix(key, ...) compared raw strings, so
	// complexes/{A}/../{B}/logo/x.jpg lexically started with A's prefix and
	// reached DeleteObject; the only thing that stopped a cross-tenant delete
	// in production was the storage SDK normalizing ".." out of the path it
	// sent to R2 while the request's signature was computed over the raw
	// key, which made R2 answer SignatureDoesNotMatch (403) and this handler
	// turn that into a 500. The ".." rejection above now refuses that key
	// before it gets this far; this comparison is what protects a legitimate
	// key against a mismatched prefix.
	cleanKey := path.Clean(key)

	// A mismatch answers 404, not 403, matching the convention documented in
	// internal/clients: 403 would confirm the object exists and turn this
	// endpoint into a probe for another tenant's storage keys.
	if !strings.HasPrefix(cleanKey, uploadKeyPrefix(complex.ID)) {
		h.respond.NotFound(w, r)
		return
	}

	if err := h.storage.DeleteObject(r.Context(), cleanKey); err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"message": "deleted"})
}
