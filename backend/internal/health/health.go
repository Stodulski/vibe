// Package health answers whether the process can serve traffic, and why not
// when it cannot.
//
// Two endpoints answer that. Live says whether the process is running and
// touches no dependency, so a restart policy can watch it without turning a
// database outage into a restart loop. Check is the readiness probe the load
// balancer polls: it says only up or degraded, plus which subsystem is impaired.
// It names that subsystem even though it is unauthenticated, because "payments"
// is close to worthless to an attacker and is the difference between an uptime
// monitor that can page someone and one that reports green while revenue is zero.
//
// Pool figures, queue backlogs and process counters are not on either endpoint:
// they are not public information, and the superadmin healthcheck that used to
// carry them is gone. The cron job logs queue depth and pool figures instead.
package health

import (
	"context"
	"net/http"
	"time"

	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/openapi/gen"
)

// pingTimeout bounds each dependency probe. A health check that can hang is
// worse than useless: the balancer times out and pulls a healthy instance.
const pingTimeout = 2 * time.Second

// Dependency states, as reported in the response.
const (
	stateOK            = "ok"
	stateUnreachable   = "unreachable"
	stateNotConfigured = "not configured"
)

// Overall states, as reported in the response.
const (
	statusAvailable = "available"
	statusDegraded  = "degraded"
)

// Impairment names, as reported in the "impaired" list. They name a capability
// the product has lost, not the vendor that lost it: "payments" is what an
// operator is paged about, and it stays true if the provider is ever swapped.
const (
	impairedDatabase = "database"
	impairedCache    = "cache"
	impairedPayments = "payments"
)

// Pinger is a dependency that can be asked whether it is reachable.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Breaker is one external dependency's circuit breaker, as the health
// endpoints see it.
//
// The interface is declared here rather than imported so this package does not
// depend on the breaker implementation, and so the composition root is the
// only place that decides which dependency is which.
type Breaker interface {
	// State is "closed", "open" or "half-open".
	State() string
	// BlocksRevenue reports whether an open circuit means no client, on any
	// tenant, can complete a payment. Only the payment provider does: losing
	// WhatsApp or the mailer costs notifications, which is bad, but the
	// product still takes money.
	BlocksRevenue() bool
}

// QueueStats is one durable work queue's backlog.
//
// Both sweepers log a completion count bounded by their own batch limit, so a
// backlog of fifty and a backlog of fifty thousand produce the same line.
// These are the figures that tell them apart.
type QueueStats struct {
	// Name is the queue: "webhook_events", "failed_refunds".
	Name string `json:"name"`
	// Pending is work waiting to be picked up.
	Pending int64 `json:"pending"`
	// Processing is work claimed by a worker. A number that does not fall is a
	// worker that died holding rows.
	Processing int64 `json:"processing"`
	// Exhausted is work that spent its retries and will never be attempted
	// again without someone intervening. On these two queues that is money:
	// a payment never reconciled, a refund never paid.
	Exhausted int64 `json:"exhausted"`
	// OldestDueSeconds is how long the oldest item that is already due has
	// been waiting. Depth alone cannot distinguish a queue that is draining
	// steadily from one that has stopped; this can.
	OldestDueSeconds int64 `json:"oldest_due_seconds"`
}

// QueueReporter reports the backlog of the durable work queues.
type QueueReporter interface {
	QueueStats(ctx context.Context) ([]QueueStats, error)
}

// Handler serves the health endpoints.
type Handler struct {
	// database must be reachable for the process to be usable at all.
	database Pinger
	// cache is optional: the application degrades to in-memory behaviour
	// without Redis, so its absence is reported, not failed.
	cache    Pinger
	breakers []Breaker
	respond  *httpx.Responder
	version  string
}

// Dependencies is everything the handler probes. Every field is optional
// except Database and Respond; an absent one is reported as not configured or
// left out of the response rather than failing the check.
type Dependencies struct {
	Database Pinger
	Cache    Pinger
	// Breakers are the external-dependency circuit breakers. Without them the
	// check is blind to the payment stack: with MercadoPago down, no client on
	// any tenant can pay and this endpoint answered 200 available.
	Breakers []Breaker
	Respond  *httpx.Responder
}

// Config is what the handler needs to describe this deployment.
type Config struct {
	Version string
}

// NewHandler returns a Handler. A nil cache means Redis is not configured,
// which is a supported deployment rather than a fault.
func NewHandler(d Dependencies, cfg Config) *Handler {
	return &Handler{
		database: d.Database,
		cache:    d.Cache,
		breakers: d.Breakers,
		respond:  d.Respond,
		version:  cfg.Version,
	}
}

// ping probes a dependency under its own bounded timeout.
func ping(ctx context.Context, p Pinger) string {
	if p == nil {
		return stateNotConfigured
	}

	ctx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	if err := p.Ping(ctx); err != nil {
		return stateUnreachable
	}
	return stateOK
}

// revenueBlocked reports whether a breaker that gates revenue is not closed.
//
// Half-open counts as impaired. It means the breaker is testing a dependency
// that was failing a moment ago and is admitting at most a couple of probes:
// the payment path is not working, it is being retried.
func (h *Handler) revenueBlocked() bool {
	for _, breaker := range h.breakers {
		if breaker.State() != "closed" && breaker.BlocksRevenue() {
			return true
		}
	}
	return false
}

// report is what the public probe computes: the overall verdict, the HTTP code
// that goes with it, and the impaired subsystems.
type report struct {
	status   string
	code     int
	impaired []string
}

// assess probes everything and decides the verdict.
//
// Only the database can take the instance out of rotation.
//
// An unreachable cache is reported as degraded but still answers 200, because
// the application falls back to in-memory behaviour — returning 503 there would
// pull every instance out of the balancer over a dependency the process can
// survive without.
//
// An open payments breaker is the same shape of decision and deserves saying
// out loud, because the obvious answer is wrong. It is the most severe thing
// this endpoint can report — no client on any tenant can pay — and it still
// answers 200. Restarting the container does not restore MercadoPago; it
// removes the instance that was still serving the schedule, the dashboard and
// every booking a venue takes at the counter, and if the provider outage is
// platform-wide it removes all of them at once, turning a payment outage into
// a total one. So: visible in the body, loud in "impaired", never in the exit
// code the orchestrator reads.
func (h *Handler) assess(ctx context.Context) report {
	deps := map[string]string{
		"database": ping(ctx, h.database),
		"redis":    ping(ctx, h.cache),
	}
	revenueBlocked := h.revenueBlocked()

	rep := report{
		status: statusAvailable,
		code:   http.StatusOK,
	}

	if deps["database"] == stateUnreachable {
		rep.impaired = append(rep.impaired, impairedDatabase)
		rep.code = http.StatusServiceUnavailable
	}
	if deps["redis"] == stateUnreachable {
		rep.impaired = append(rep.impaired, impairedCache)
	}
	if revenueBlocked {
		rep.impaired = append(rep.impaired, impairedPayments)
	}
	if len(rep.impaired) > 0 {
		rep.status = statusDegraded
	}

	return rep
}

// toHealthStatusImpaired maps the impaired list into the public status type's
// enum. oapi-codegen declares a distinct (identically valued) enum type per
// schema, so the list needs this conversion to become HealthStatus's own type.
func toHealthStatusImpaired(impaired []string) *[]gen.HealthStatusImpaired {
	if len(impaired) == 0 {
		return nil
	}

	out := make([]gen.HealthStatusImpaired, len(impaired))
	for i, name := range impaired {
		out[i] = gen.HealthStatusImpaired(name)
	}
	return &out
}

// envelopeFromHealthStatus flattens a generated HealthStatus into the
// envelope httpx.Responder.JSON expects, keeping each field's omitempty
// behaviour: a nil Impaired stays entirely absent rather than serializing as
// null or an empty list.
func envelopeFromHealthStatus(s gen.HealthStatus) httpx.Envelope {
	body := httpx.Envelope{
		"status":  s.Status,
		"version": s.Version,
	}
	if s.Impaired != nil {
		body["impaired"] = *s.Impaired
	}
	return body
}

// Live handles GET /api/v1/livez: is this process running.
//
// It touches no dependency on purpose, and that is the whole difference
// between it and Check. Liveness and readiness answer different questions and
// have different consequences: an unreachable database means this instance
// cannot serve, so readiness pulls it out of rotation — but it does not mean
// the process is broken, and restarting every replica while the database is
// down replaces a partial outage with a restart loop that cannot end until the
// database comes back. Check stays the readiness probe; this one is what a
// restart policy should watch.
func (h *Handler) Live(w http.ResponseWriter, r *http.Request) {
	resp := gen.HealthStatus{
		Status:  gen.HealthStatusStatus(statusAvailable),
		Version: h.version,
	}
	h.respond.JSON(w, r, http.StatusOK, envelopeFromHealthStatus(resp))
}

// Check handles GET /api/v1/healthcheck, the load balancer's readiness probe.
//
// It names the impaired subsystem even though it is unauthenticated. "payments"
// tells a reader that our payment provider is unhappy, which is close to
// worthless to an attacker and is the difference between an uptime monitor
// that can page someone and one that reports green while revenue is zero.
func (h *Handler) Check(w http.ResponseWriter, r *http.Request) {
	rep := h.assess(r.Context())

	resp := gen.HealthStatus{
		Status:   gen.HealthStatusStatus(rep.status),
		Version:  h.version,
		Impaired: toHealthStatusImpaired(rep.impaired),
	}

	h.respond.JSON(w, r, rep.code, envelopeFromHealthStatus(resp))
}
