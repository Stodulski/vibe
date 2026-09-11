package auth

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	"github.com/stodulski/vibe-server/internal/httpx"
)

// Actions this module records.
//
// The line is not "every state transition"; it is: record what leaves no other
// trace. A sign-in leaves a refresh-token row that the next rotation deletes. A
// failed one bumps a counter the next success clears. A sign-out deletes a row.
// A password change overwrites the hash in place, so the previous credential and
// the moment it was replaced are gone. An account deletion takes the whole row
// and everything cascading off it. None of those can be reconstructed afterwards
// from anything but this table.
//
// What that line excludes, and why:
//
//   - register — an account creation writes a users row with its own created_at
//     that lives as long as the account. Nothing about it is lost.
//   - verify-email and resend-verification — the outcome is a durable flag on the
//     users row, and proving control of an address grants no credential.
//   - refresh — an ordinary rotation happens every few minutes per open session.
//     Recording it would swamp the one table that has no retention job
//     (idx_audit_log_created_at) and bury the entries below in it. Its abnormal case,
//     actionRefreshReuse, is recorded.
//   - the reads (GET /auth/me) — neither changes what the
//     account can do.
//   - the refusals that change nothing: a malformed body, a 409 on deleting an
//     account that still has live bookings, a 500 from a store. Those are not
//     acts against an account, and recording a store failure as a failed sign-in
//     would be a lie about what happened.
//
// The one deliberate omission inside the set is a reason on actionLoginFailed:
// see record's comment on why every failed attempt is recorded identically.
const (
	actionLogin                = "login"
	actionLoginFailed          = "login_failed"
	actionLogout               = "logout"
	actionEmailChange          = "email_change"
	actionPasswordChange       = "password_change"
	actionPasswordResetRequest = "password_reset_request"
	actionPasswordReset        = "password_reset"
	actionAccountDelete        = "account_delete"
	actionRefreshReuse         = "refresh_reuse"
)

// entityUser is the entity type every entry this module writes carries. The
// thing acted on is always an account, never a venue's data.
const entityUser = "user"

// Actors named in the value on the two paths where audit_log.user_id cannot
// name one.
const (
	// actorSelf is the account acting on itself. It appears on the deletion
	// entry, which is written once the users row is already gone — see
	// Handler.DeleteAccount.
	actorSelf = "self"
	// actorUnknown is whoever presented an already-rotated refresh token. It is
	// deliberately not the account: attributing a suspected theft to its victim
	// is the one reading of that entry that must not be possible.
	actorUnknown = "unknown"
)

// maxRecordedAddress is the longest address SMTP will carry, RFC 5321 §4.5.3.1.
const maxRecordedAddress = 254

// accountEvent is the audit value for one act against an account.
//
// There is no field here for a password, a password hash, an access or refresh
// token, or a single-use verification or reset token, and the absence is the
// design. Those are live credentials; this table is long-lived, has no retention
// job, and is read by people who are not the account's owner, so a credential
// landing in it is a credential published. data.Booking.LinkToken and
// data.Complex's MercadoPago fields keep out of the same table with `json:"-"`,
// and audit.Recorder.Record encodes with json.Marshal, so a tagged field cannot
// reach it. Here the shape is stronger than a tag: there is nowhere to put one.
//
// It also means a caller cannot hand over the whole *data.User and let the tags
// decide. That would be safe today, and would silently stop being safe the day
// somebody adds a field to data.User without one.
type accountEvent struct {
	// Email is the address the act concerned. On a sign-in that succeeded, on a
	// sign-out, on a password change, it is the account's own address, which
	// matters most on the entries that outlive the users row. On a failed
	// sign-in or a reset request it is the address that was typed, which may
	// belong to nobody — and it is then the only thing on the entry that ties
	// the attempt to anything, because UserID is deliberately nil there.
	Email string `json:"email,omitempty"`
	// Actor says who acted where audit_log.user_id could not: actorSelf or
	// actorUnknown.
	Actor string `json:"actor,omitempty"`
	// Method distinguishes how a session was established, when it is
	// anything other than the ordinary password sign-in: "google" for Sign
	// in with Google (see Handler.startSession). Empty and omitted on every
	// other entry this module writes, including an ordinary password login.
	Method string `json:"method,omitempty"`
}

// record writes one entry for an act against an account.
//
// ComplexID is nil on every entry this module writes, and that is what decides
// who may read them. These are platform-level acts against a person, not
// changes to a venue's data: an owner's sign-in history is not their tenants'
// business, and a failed sign-in carries an address somebody typed, which may
// not be theirs at all. A NULL complex_id is invisible to the tenant-facing
// trail — that query is `complex_id = $1` against the complex the ownership
// guard resolved, and NULL matches no id (data.listAuditLogsSQL) — so these
// reach only a superadmin, through the admin trail.
//
// ComplexID, EntityType and IPAddress are set here rather than at the nine call
// sites, so that none of the three can be got wrong in one place out of nine.
//
// On UserID the rule is the one every other module uses: it names the
// authenticated actor the request carried, and nothing else. It is therefore nil
// on every unauthenticated path, and on a failed sign-in it is nil even when the
// address does have an account. That is not laziness about a value we hold. A
// failed attempt against a real account writing a real id, and one against a
// nonexistent address writing NULL, would make the trail answer exactly the
// question the login response was just changed to stop answering — with the
// account's email joined in beside it. The same reasoning forbids a reason field
// distinguishing "wrong password" from "no such account" from "not verified":
// the trail records the address that was tried and the fact that no session came
// of it, and whether that address has an account is a join a reader entitled to
// make can make for themselves.
func (h *Handler) record(r *http.Request, e audit.Entry) {
	e.ComplexID = nil
	e.EntityType = entityUser
	e.IPAddress = httpx.ClientIP(r, h.cfg.TrustProxies)

	h.audit.Record(e)
}

// accountID copies an account's id for an entry.
//
// audit.Recorder hands the entry to a background goroutine, so a pointer taken
// into a struct the handler still owns and still writes would be read off the
// request goroutine. Record already encodes OldValue and NewValue up front for
// exactly that reason; the id is copied here for the same one, and costs
// nothing.
func accountID(id uuid.UUID) *uuid.UUID {
	return &id
}

// recordedAddress is an address as it goes into an audit value.
//
// On the failure paths this is unvalidated request input: Login checks only that
// it is non-empty, and ForgotPassword does not check at all. Bodies are capped
// at 1 MiB (httpx.maxRequestBodyBytes), audit_log has no retention job
// (idx_audit_log_created_at), and every failed attempt writes a row — so uncapped, one
// request buys a megabyte of permanent storage and a script buys as much as it
// likes.
//
// Nothing that could be a real address is shortened, because the cap is the
// longest address SMTP will carry. Anything longer is truncated rather than
// dropped: what was attempted is still the most useful thing on the entry, and
// its first 254 bytes still identify a campaign. Truncation can land mid-rune,
// so the result is made valid UTF-8 — json.Marshal would otherwise substitute
// replacement characters of its own.
func recordedAddress(email string) string {
	if len(email) > maxRecordedAddress {
		email = strings.ToValidUTF8(email[:maxRecordedAddress], "")
	}
	return email
}
