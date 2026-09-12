package complexes

import (
	"fmt"
	"net/http"

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

// PresignUpload handles POST /api/v1/complexes/{id}/uploads/presign, returning a
// short-lived URL the browser uploads straight to, so image bytes never pass
// through this service. The object key is namespaced to the complex.
func (h *Handler) PresignUpload(w http.ResponseWriter, r *http.Request) {
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

	upload, err := h.svc.PresignUpload(r.Context(), complex.ID, input.Type, input.ContentType, input.FileSize)
	if err != nil {
		h.respond.DomainError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"upload_url": upload.UploadURL,
		"public_url": upload.PublicURL,
		"key":        upload.Key,
	})
}

// DeleteUpload handles DELETE /api/v1/complexes/{id}/uploads. It accepts only
// URLs inside our own storage, and only those inside the calling complex's own
// namespace within it, so neither an arbitrary URL nor another tenant's URL can
// be used to delete something else. See Service.DeleteUpload for why.
func (h *Handler) DeleteUpload(w http.ResponseWriter, r *http.Request) {
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

	err = h.svc.DeleteUpload(r.Context(), complex.ID, input.URL)
	if err != nil {
		// A mismatch answers 404, not 403, matching the convention documented
		// in internal/clients: 403 would confirm the object exists and turn
		// this endpoint into a probe for another tenant's storage keys. That
		// is the shared data.ErrRecordNotFound answer, so it needs no entry of
		// its own here.
		h.respond.DomainError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"message": "deleted"})
}
