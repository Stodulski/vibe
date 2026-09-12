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
// That is the property JOB-04 was missing. The notifier was at-least-once with
// no key at all, so a webhook MercadoPago delivered twice, or a sweep that
// reclaimed a claim whose acknowledgement had been lost, sent the client a
// second confirmation email and a second WhatsApp message. Idempotency at the
// provider does not help: both deliveries are honest, distinct sends.
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
