package jobs

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// DedupKey builds the unique key that makes an Enqueue idempotent.
//
// It is a hash of the job type and the parts that identify the piece of work —
// for a notification, the recipient, the business object it is about (a
// booking) and the event that produced it. Two Enqueues that agree on all of
// those are the same delivery, and the second one does nothing.
//
// That closes one way a second copy of a notification gets queued (JOB-04): a
// webhook MercadoPago delivered twice, or a caller that enqueued the same
// delivery again, used to add a second job and so a second confirmation email
// and a second WhatsApp message. It does not close the other way. A job already
// queued whose claim is reclaimed after a lost acknowledgement runs again, and
// the provider is called again. Idempotency at the provider would not help even
// if it existed: each call is an honest, distinct send.
//
// It is hashed rather than concatenated because the parts are caller data —
// an email address, a phone number — and the queue is durable storage that
// outlives the row it was built from. A fixed-width digest also keeps the
// unique index off a column whose length is the caller's business.
//
// Every part is included, in order, separated by a byte that cannot occur in
// one, so ("a", "bc") and ("ab", "c") are different keys.
//
// The key protects for as long as its row lives: it is Config.Retention's
// DeleteDone sweep that frees one, by deleting the done job holding it.
func DedupKey(jobType string, parts ...string) string {
	h := sha256.New()
	h.Write([]byte(jobType))
	for _, p := range parts {
		h.Write([]byte{0})
		h.Write([]byte(strings.TrimSpace(p)))
	}
	return hex.EncodeToString(h.Sum(nil))
}
