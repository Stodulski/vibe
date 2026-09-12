package main

import (
	"io"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stodulski/vibe-server/internal/stores"
)

// validTestDeps returns a deps value that satisfies validateDeps, for tests
// that need to remove exactly one store and observe the rejection.
func validTestDeps(t *testing.T) deps {
	t.Helper()

	return deps{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		models: stores.Stores{
			Users:             &mockUserStore{},
			UserIdentities:    &mockUserIdentityStore{},
			Tokens:            &mockTokenStore{},
			Complexes:         &mockComplexStore{},
			Courts:            &mockCourtStore{},
			Bookings:          &mockBookingStore{},
			BookingLinkTokens: &mockBookingLinkTokenStore{},
			Clients:           &mockClientStore{},
			Payments:          &mockPaymentStore{},
			EmailVerification: &mockEmailVerificationStore{},
			PasswordReset:     &mockPasswordResetStore{},
			FailedRefunds:     &mockFailedRefundStore{},
			WebhookEvents:     &mockWebhookEventStore{},
			SlotLocks:         &mockSlotLockStore{},
			Reports:           &mockReportStore{},
			Admin:             &mockAdminStore{},
			Audit:             &mockAuditStore{},
			Locks:             &mockLockStore{},
		},
	}
}

// TestNewApplicationRejectsAMissingStore covers the gap unwiredDependencies()
// could not: a store stores.Stores composes, left nil, is caught before any
// constructor runs — not mid-sequence, with several handlers already built.
func TestNewApplicationRejectsAMissingStore(t *testing.T) {
	d := validTestDeps(t)
	d.models.PasswordReset = nil

	_, err := newApplication(config{}, d)
	if err == nil {
		t.Fatal("newApplication: want an error for a missing store; got nil")
	}
	if !strings.Contains(err.Error(), "PasswordReset") {
		t.Errorf("newApplication: error %q does not name the missing store", err.Error())
	}
}

// TestNewApplicationRejectsEveryMissingStore is the same check for every one
// of the 17 stores, not just PasswordReset — each is a leaf a caller could
// leave nil independently.
func TestNewApplicationRejectsEveryMissingStore(t *testing.T) {
	cases := []struct {
		name string
		zero func(*stores.Stores)
	}{
		{"Users", func(m *stores.Stores) { m.Users = nil }},
		{"UserIdentities", func(m *stores.Stores) { m.UserIdentities = nil }},
		{"Complexes", func(m *stores.Stores) { m.Complexes = nil }},
		{"Courts", func(m *stores.Stores) { m.Courts = nil }},
		{"Bookings", func(m *stores.Stores) { m.Bookings = nil }},
		{"BookingLinkTokens", func(m *stores.Stores) { m.BookingLinkTokens = nil }},
		{"Tokens", func(m *stores.Stores) { m.Tokens = nil }},
		{"Clients", func(m *stores.Stores) { m.Clients = nil }},
		{"Payments", func(m *stores.Stores) { m.Payments = nil }},
		{"EmailVerification", func(m *stores.Stores) { m.EmailVerification = nil }},
		{"PasswordReset", func(m *stores.Stores) { m.PasswordReset = nil }},
		{"FailedRefunds", func(m *stores.Stores) { m.FailedRefunds = nil }},
		{"WebhookEvents", func(m *stores.Stores) { m.WebhookEvents = nil }},
		{"SlotLocks", func(m *stores.Stores) { m.SlotLocks = nil }},
		{"Admin", func(m *stores.Stores) { m.Admin = nil }},
		{"Audit", func(m *stores.Stores) { m.Audit = nil }},
		{"Reports", func(m *stores.Stores) { m.Reports = nil }},
		{"Locks", func(m *stores.Stores) { m.Locks = nil }},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := validTestDeps(t)
			c.zero(&d.models)

			_, err := newApplication(config{}, d)
			if err == nil {
				t.Fatalf("newApplication: want an error with models.%s nil; got nil", c.name)
			}
			if !strings.Contains(err.Error(), c.name) {
				t.Errorf("newApplication: error %q does not name %q", err.Error(), c.name)
			}
		})
	}
}

// TestNewApplicationCanBeCalledTwiceInOneProcess is the spec's "called twice"
// scenario: newApplication must launch no goroutine and register no
// process-global, so nothing about calling it a second time — in the same
// process, with equivalent deps — can panic or double-register anything.
func TestNewApplicationCanBeCalledTwiceInOneProcess(t *testing.T) {
	cfg := config{env: "test", jwt: struct{ secret string }{secret: testJWTSecret}}

	first, err := newApplication(cfg, validTestDeps(t))
	if err != nil {
		t.Fatalf("first newApplication call: %v", err)
	}
	second, err := newApplication(cfg, validTestDeps(t))
	if err != nil {
		t.Fatalf("second newApplication call: %v", err)
	}

	if first == second {
		t.Error("newApplication: two calls returned the same *application")
	}
}

// optionalApplicationFields are the fields newApplication may legitimately
// leave nil, each with the condition that makes it optional. Everything else
// must be wired.
//
// This map is the whole point of the test below. A field listed here is a
// decision somebody made and wrote down; a field missing from both this map
// and the constructor is a field nobody decided about, and that is what the
// test refuses to let through.
var optionalApplicationFields = map[string]string{
	"db":        "supplied by main(); the unit harness runs on mock stores and has no pool",
	"queues":    "derived from db, and deliberately left nil rather than wrapping a nil pool — a queueProbe{pool: nil} stored in the interface would be non-nil and panic on first read",
	"rdb":       "nil when Redis is not configured; every consumer degrades",
	"notifier":  "built from rdb, so nil for the same reason",
	"blacklist": "token revocation is Redis-backed; nil without it",
	"wa":        "nil unless WHATSAPP_TOKEN and WHATSAPP_PHONE_NUMBER_ID are both set",
	"storage":   "nil unless object storage is configured; uploads answer 503",
}

// TestNewApplicationWiresEveryField is the check unwiredDependencies() was
// reaching for, done in the one way that cannot rot.
//
// That guard listed 10 of 33 fields by hand. A hand-written list is a snapshot
// of what somebody remembered on the day they wrote it, so the gap between it
// and the struct grows every time a field is added — silently, because nothing
// about adding a field disturbs the list.
//
// Reflection closes that gap in both directions. Every nillable field must
// either come back wired or appear in optionalApplicationFields with a reason,
// so a new field fails this test until somebody decides which it is. That is
// the same fail-closed shape TestRoutePolicyTableIsComplete uses for routes.
//
// It also covers a regression the compile-time ordering cannot catch. Wrong
// ORDER is a build error now, because handlers take locals. Wrong SOURCE is
// not: changing bookings.Dependencies.Refunds from the local paymentsHandler
// back to app.payments still compiles, reads the field before the publish
// block sets it, and hands bookings a nil — which is exactly the defect this
// change exists to remove.
func TestNewApplicationWiresEveryField(t *testing.T) {
	app, err := newApplication(config{env: "test"}, validTestDeps(t))
	if err != nil {
		t.Fatalf("newApplication with every store supplied: %v", err)
	}

	// Elem() rather than ValueOf(*app): application embeds a sync.WaitGroup,
	// and dereferencing to pass it by value copies that lock.
	v := reflect.ValueOf(app).Elem()
	typ := v.Type()

	for i := range typ.NumField() {
		name := typ.Field(i).Name
		field := v.Field(i)

		// Value types (config, models, wg, whatsappEnabled) have no nil to be
		// caught in, so they are outside what this test can say anything about.
		if !nillable(field.Kind()) {
			continue
		}

		reason, optional := optionalApplicationFields[name]
		if field.IsNil() && !optional {
			t.Errorf("application.%s came back nil and is not listed as optional; "+
				"either wire it in newApplication or add it to optionalApplicationFields "+
				"with the condition that makes it optional", name)
			continue
		}
		if optional && reason == "" {
			t.Errorf("application.%s is listed as optional with no reason", name)
		}
	}
}

// nillable reports whether a kind can hold nil, and so whether "was this
// wired?" is a question that means anything for it.
func nillable(k reflect.Kind) bool {
	return k == reflect.Pointer || k == reflect.Interface || k == reflect.Chan ||
		k == reflect.Map || k == reflect.Slice || k == reflect.Func
}

// TestNewApplicationWiresTheRedisBackedPath covers the half of the constructor
// that nothing reached: every miniredis test in this repo runs against a single
// component — auth, middleware, realtime, notifier — and both harnesses leave
// deps.rdb nil, so the branch newApplication takes WITH Redis had never been
// executed by any test.
//
// That is the branch production actually runs: main() refuses to boot without
// Redis. Proving the components work in isolation says nothing about whether
// the composition root hands them the client.
func TestNewApplicationWiresTheRedisBackedPath(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	d := validTestDeps(t)
	d.rdb = rdb

	app, err := newApplication(config{env: "test"}, d)
	if err != nil {
		t.Fatalf("newApplication with Redis: %v", err)
	}
	t.Cleanup(func() { close(app.shutdown) })

	// The three fields that exist only on this branch. With deps.rdb nil the
	// first is nil and the second falls back to memoryQueue, so asserting them
	// is what distinguishes "Redis was passed" from "Redis was ignored".
	if app.notifier == nil {
		t.Error("app.notifier is nil with Redis configured; the durable queue was not built")
	}
	if _, ok := app.queue.(taskQueue); !ok {
		t.Errorf("app.queue is %T; with Redis configured it must be the notifier-backed queue, "+
			"not the in-memory fallback", app.queue)
	}
	if app.blacklist == nil {
		t.Error("app.blacklist is nil with Redis configured; token revocation would be a no-op")
	}

	// Optional-field bookkeeping has to agree with reality: everything listed
	// as "nil without Redis" must actually be non-nil once Redis is supplied,
	// or the reason written next to it is wrong.
	for _, name := range []string{"rdb", "notifier", "blacklist"} {
		if reflect.ValueOf(app).Elem().FieldByName(name).IsNil() {
			t.Errorf("application.%s is documented as nil only without Redis, but is nil with it", name)
		}
	}
}
