package leads

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/spreadsheet"
	"github.com/stodulski/vibe-server/internal/validator"
)

type captureRequest struct {
	Email string `json:"email"`
}

type sheetPayload struct {
	Token  string `json:"token"`
	Email  string `json:"email"`
	Origen string `json:"origen"`
	Fecha  string `json:"fecha"`
}

// captureOrigin tags every row this handler writes, so it reads apart from
// the landing page's own mailing-list signups in the same spreadsheet.
const captureOrigin = "vibe-client:registro-abandonado"

// CaptureAbandonedRegistration handles POST
// /api/v1/public/leads/abandoned-registration — the safety net for someone
// who typed their email into the register form and left before finishing.
// The caller (often navigator.sendBeacon during page unload) doesn't wait
// on the response, so this always accepts a well-formed request immediately
// and forwards to the spreadsheet webhook best-effort.
func (h *Handler) CaptureAbandonedRegistration(w http.ResponseWriter, r *http.Request) {
	var req captureRequest
	if err := httpx.ReadJSON(w, r, &req); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	v.Check(req.Email != "", "email", "must be provided")
	// H-21: the same defect H-16 fixed on the public booking form, in the other
	// endpoint that takes free text from a caller with no account. EmailRX
	// bounds the shape of an address and not its size, so a regex-legal address
	// of ten thousand characters was accepted with 202 and forwarded to the
	// owner's spreadsheet. 254 is the RFC 5321 limit on a path, which is what
	// internal/bookings/public.go already uses for client_email; the two public
	// endpoints agreeing matters more than the exact number.
	v.Check(len(req.Email) <= 254, "email", "must not be more than 254 characters")
	v.Check(validator.Matches(req.Email, validator.EmailRX), "email", "must be a valid email address")
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	w.WriteHeader(http.StatusAccepted)

	if h.webhookURL == "" {
		return
	}

	if err := h.forward(r.Context(), req.Email); err != nil {
		h.respond.LogError(r, err)
	}
}

// forward posts the lead to the owner's spreadsheet webhook.
//
// H-24: this is the boundary where a caller's email stops being an address and
// becomes a spreadsheet cell, so it is where the formula escape belongs. "=" and
// "+" are both atext, so "=1+1@example.com" is a legal address that EmailRX
// accepts and H-21's length bound has no opinion about; it was forwarded
// verbatim, and Google Sheets evaluates any cell whose text begins with "=",
// "+", "-" or "@".
//
// The validator upstream is deliberately NOT the place for this. "+qa@example.com"
// and "-qa@example.com" are ordinary addresses real people hold and both begin
// with a trigger character, so refusing them would break legitimate signups to
// defend a spreadsheet. The address is fine; the cell is the problem.
//
// The escaped value is what is sent, not what is stored anywhere else — nothing
// on this path keeps the lead locally — so the apostrophe exists only in the
// sheet, which is the one reader that needs it and does not display it.
func (h *Handler) forward(ctx context.Context, email string) error {
	payload := sheetPayload{
		Token:  h.token,
		Email:  spreadsheet.EscapeFormulaCell(email),
		Origen: captureOrigin,
		Fecha:  time.Now().UTC().Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("leads: failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("leads: failed to create request: %w", err)
	}
	req.Header.Set("content-type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("leads: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("leads: webhook returned status %d", resp.StatusCode)
	}

	return nil
}
