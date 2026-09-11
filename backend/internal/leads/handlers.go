package leads

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/spreadsheet"
	"github.com/stodulski/vibe-server/internal/validator"
)

type captureRequest struct {
	Email string `json:"email"`
	// Source names the form the person left. Empty means the password
	// register form; see captureOrigin and captureOriginGoogle.
	Source string `json:"source"`
}

type sheetPayload struct {
	Token  string `json:"token"`
	Email  string `json:"email"`
	Origen string `json:"origen"`
	Fecha  string `json:"fecha"`
}

// sheetAnswer is what the Apps Script web app writes back (see `responder` in
// landing/scripts/apps-script-lista.gs). Every outcome arrives as HTTP 200:
// the script reports its own failures only in this body, `ok: false` with a
// reason, so the status code alone says nothing about whether the row exists.
type sheetAnswer struct {
	OK       bool   `json:"ok"`
	Error    string `json:"error"`
	Repetido bool   `json:"repetido"`
}

// maxSheetAnswerBytes bounds how much of the webhook's answer is read. A real
// answer is a few dozen bytes; anything larger is not the script talking.
const maxSheetAnswerBytes = 64 << 10

// errSheetRefused is the class of failure the spreadsheet reports in its body
// rather than its status: a wrong token, an address the script would not take,
// or an exception inside it. It is wrapped with the script's own reason.
var errSheetRefused = errors.New("leads: spreadsheet refused the lead")

// captureOrigin tags the rows the password register form produces, so they
// read apart from the landing page's own mailing-list signups in the same
// spreadsheet.
const captureOrigin = "vibe-client:registro-abandonado"

// captureOriginGoogle tags the rows the Google sign-up produces: the address
// was verified by Google and prefilled, and the person left on the step that
// asks for the phone number. It is a warmer lead than a typed address, which
// is why the spreadsheet gets to tell the two apart.
const captureOriginGoogle = "vibe-client:registro-google-abandonado"

const (
	sourceRegister = "register"
	sourceGoogle   = "google"
)

// originFor maps a request's source onto the spreadsheet origin. The caller
// has already validated the value; an empty source is the register form.
func originFor(source string) string {
	if source == sourceGoogle {
		return captureOriginGoogle
	}
	return captureOrigin
}

// CaptureAbandonedRegistration handles POST
// /api/v1/public/leads/abandoned-registration — the safety net for someone
// who typed their email into the register form, or signed in with Google and
// left before giving the phone number, without finishing the account.
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
	v.Check(validator.PermittedValue(req.Source, "", sourceRegister, sourceGoogle), "source", "must be one of register, google")
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	w.WriteHeader(http.StatusAccepted)

	if h.webhookURL == "" {
		return
	}

	if err := h.forward(r.Context(), req.Email, originFor(req.Source)); err != nil {
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
func (h *Handler) forward(ctx context.Context, email, origin string) error {
	payload := sheetPayload{
		Token:  h.token,
		Email:  spreadsheet.EscapeFormulaCell(email),
		Origen: origin,
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

	return readSheetAnswer(resp.Body)
}

// readSheetAnswer turns the webhook's body into the success or failure the
// status code does not carry.
//
// Apps Script answers 200 whatever happened and puts the verdict in JSON:
// `{"ok":true}`, `{"ok":true,"repetido":true}` (already in the sheet, which is
// a success here), or `{"ok":false,"error":"token invalido"}`. Reading only
// the status meant a rotated token, a script republished under another URL, or
// an exception inside it dropped every lead while the logs stayed clean. The
// body is therefore required to parse and to say ok: an empty or non-JSON
// answer is treated as a failure too, deliberately, because nothing this
// handler talks to answers that way on purpose, and a misconfigured URL
// (a login page, a 200 from the wrong host) is exactly what would.
func readSheetAnswer(body io.Reader) error {
	raw, err := io.ReadAll(io.LimitReader(body, maxSheetAnswerBytes))
	if err != nil {
		return fmt.Errorf("leads: reading the webhook answer: %w", err)
	}

	var answer sheetAnswer
	if err := json.Unmarshal(raw, &answer); err != nil {
		return fmt.Errorf("leads: webhook answered something other than its JSON verdict (%q): %w",
			truncate(strings.TrimSpace(string(raw)), 120), err)
	}
	if !answer.OK {
		return fmt.Errorf("%w: %s", errSheetRefused, answer.Error)
	}
	return nil
}

// truncate keeps the first n bytes of s for a log line.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
