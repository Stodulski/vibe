// Package health answers whether the process can serve traffic, and why not
// when it cannot.
//
// There are two endpoints because they have two audiences. The public one is
// polled by the platform's load balancer and says only up or degraded, plus
// which subsystem is impaired. The detailed one is for an operator debugging
// an incident: connection-pool figures, circuit-breaker states, queue backlogs
// and the process's own request counters. It is behind the superadmin role,
// because pool saturation, dependency topology and traffic volume are not
// public information.
//
// # Why the detailed endpoint is where production metrics live
//
// The alternative was expvar on /debug/vars, which this service already
// computes and then registers only when the environment is "development" — so
// in production the numbers exist and nothing can reach them. Making that
// route unconditional would put goroutine counts, memory statistics and the
// process command line on the open internet. Adding a second authenticated
// route would duplicate the guard, the responder and the "what is this
// process" preamble that the detailed check already carries.
//
// So metrics live on GET /api/v1/admin/healthcheck. It is already
// superadmin-gated, it is already the page an operator opens during an
// incident, and "is it healthy" and "what are its numbers" are the same
// question asked at two levels of detail. The cost is real and worth naming:
// this is not a Prometheus exposition format and it is not scrapeable without
// a superadmin credential. That is the right trade while the operator is a
// person; the day it becomes a scraper, the answer is a separate route bound
// to the private network, not a public /metrics.
package health

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/openapi/gen"
)

// pingTimeout bounds each dependency probe. A health check that can hang is
// worse than useless: the balancer times out and pulls a healthy instance.
const pingTimeout = 2 * time.Second

// queueTimeout bounds the queue-depth query, which is an aggregate over two
// tables and is only ever asked for by the detailed endpoint. Same argument as
// pingTimeout, with more room because it is a real query rather than a ping.
const queueTimeout = 3 * time.Second

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

// PoolReporter is an optional capability: a dependency that can also describe
// its connection pool. Implementations that cannot are simply omitted from the
// detailed response.
type PoolReporter interface {
	PoolStats() map[string]int64
}

// Breaker is one external dependency's circuit breaker, as the health
// endpoints see it.
//
// The interface is declared here rather than imported so this package does not
// depend on the breaker implementation, and so the composition root is the
// only place that decides which dependency is which.
type Breaker interface {
	// Name identifies the dependency: "mercadopago", "whatsapp", "mailer".
	Name() string
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

// MetricsSource is the process's own counters: request volume, latency
// distribution, requests in flight, goroutines, the notifier's queue counters.
type MetricsSource interface {
	Metrics() map[string]any
}

// Handler serves the health endpoints.
type Handler struct {
	// database must be reachable for the process to be usable at all.
	database Pinger
	// cache is optional: the application degrades to in-memory behaviour
	// without Redis, so its absence is reported, not failed.
	cache       Pinger
	dbPool      PoolReporter
	cachePool   PoolReporter
	breakers    []Breaker
	queues      QueueReporter
	metrics     MetricsSource
	respond     *httpx.Responder
	environment string
	version     string
}

// Dependencies is everything the handler probes. Every field is optional
// except Database and Respond; an absent one is reported as not configured or
// left out of the response rather than failing the check.
type Dependencies struct {
	Database  Pinger
	Cache     Pinger
	DBPool    PoolReporter
	CachePool PoolReporter
	// Breakers are the external-dependency circuit breakers. Without them the
	// check is blind to the payment stack: with MercadoPago down, no client on
	// any tenant can pay and this endpoint answered 200 available.
	Breakers []Breaker
	Queues   QueueReporter
	Metrics  MetricsSource
	Respond  *httpx.Responder
}

// Config is what the handler needs to describe this deployment.
type Config struct {
	Environment string
	Version     string
}

// NewHandler returns a Handler. A nil cache means Redis is not configured,
// which is a supported deployment rather than a fault. Either pooler may be
// nil; those figures are then left out of the detailed response.
func NewHandler(d Dependencies, cfg Config) *Handler {
	return &Handler{
		database:    d.Database,
		cache:       d.Cache,
		dbPool:      d.DBPool,
		cachePool:   d.CachePool,
		breakers:    d.Breakers,
		queues:      d.Queues,
		metrics:     d.Metrics,
		respond:     d.Respond,
		environment: cfg.Environment,
		version:     cfg.Version,
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

// breakerStates reads every configured breaker and reports whether any of the
// ones that gate revenue is not closed.
//
// Half-open counts as impaired. It means the breaker is testing a dependency
// that was failing a moment ago and is admitting at most a couple of probes:
// the payment path is not working, it is being retried.
func (h *Handler) breakerStates() (states map[string]string, revenueBlocked bool) {
	if len(h.breakers) == 0 {
		return nil, false
	}

	states = make(map[string]string, len(h.breakers))
	for _, breaker := range h.breakers {
		state := breaker.State()
		states[breaker.Name()] = state
		if state != "closed" && breaker.BlocksRevenue() {
			revenueBlocked = true
		}
	}
	return states, revenueBlocked
}

// report is what both endpoints compute: the overall verdict, the HTTP code
// that goes with it, and the evidence.
type report struct {
	status   string
	code     int
	deps     map[string]string
	breakers map[string]string
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
	breakers, revenueBlocked := h.breakerStates()

	rep := report{
		status:   statusAvailable,
		code:     http.StatusOK,
		deps:     deps,
		breakers: breakers,
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

// toGenQueueStats maps a queue backlog snapshot, as read from the store, into
// the generated wire type. The internal type keeps int64 counters because it
// is also the QueueReporter contract implemented outside this package; the
// generated schema declares plain "integer" (Go int), so this is the explicit
// translation HTTP-08 requires rather than a reuse of the store-facing type.
func toGenQueueStats(stats []QueueStats) []gen.QueueStats {
	if stats == nil {
		return nil
	}

	out := make([]gen.QueueStats, len(stats))
	for i, s := range stats {
		out[i] = gen.QueueStats{
			Name:             s.Name,
			Pending:          int(s.Pending),
			Processing:       int(s.Processing),
			Exhausted:        int(s.Exhausted),
			OldestDueSeconds: int(s.OldestDueSeconds),
		}
	}
	return out
}

// toGenBreakers maps breaker states into the generated enum type. It returns
// nil when no breakers are configured, so the field stays absent from the
// response rather than serializing as an empty object.
func toGenBreakers(states map[string]string) *map[string]gen.HealthDetailedBreakers {
	if states == nil {
		return nil
	}

	out := make(map[string]gen.HealthDetailedBreakers, len(states))
	for name, state := range states {
		out[name] = gen.HealthDetailedBreakers(state)
	}
	return &out
}

// toHealthStatusImpaired maps the impaired list into the public status type's
// enum. oapi-codegen declares a distinct (identically valued) enum type per
// schema, so HealthStatus and HealthDetailed each need their own conversion.
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

// toHealthDetailedImpaired is the same mapping for the detailed type.
func toHealthDetailedImpaired(impaired []string) *[]gen.HealthDetailedImpaired {
	if len(impaired) == 0 {
		return nil
	}

	out := make([]gen.HealthDetailedImpaired, len(impaired))
	for i, name := range impaired {
		out[i] = gen.HealthDetailedImpaired(name)
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

// envelopeFromHealthDetailed is the same flattening for the detailed type.
func envelopeFromHealthDetailed(d gen.HealthDetailed) httpx.Envelope {
	body := httpx.Envelope{
		"status":       d.Status,
		"version":      d.Version,
		"environment":  d.Environment,
		"dependencies": d.Dependencies,
	}
	if d.Impaired != nil {
		body["impaired"] = *d.Impaired
	}
	if d.Breakers != nil {
		body["breakers"] = *d.Breakers
	}
	if d.Metrics != nil {
		body["metrics"] = *d.Metrics
	}
	if d.QueuesError != nil {
		body["queues_error"] = *d.QueuesError
	}
	if d.Queues != nil {
		body["queues"] = *d.Queues
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

// Detailed handles GET /api/v1/admin/healthcheck: everything Check knows, plus
// the pool figures, breaker states, queue backlogs and process counters an
// operator needs during an incident.
func (h *Handler) Detailed(w http.ResponseWriter, r *http.Request) {
	rep := h.assess(r.Context())

	deps := rep.deps
	for prefix, pool := range map[string]PoolReporter{"db_pool": h.dbPool, "redis_pool": h.cachePool} {
		if pool == nil {
			continue
		}
		for key, value := range pool.PoolStats() {
			deps[prefix+"_"+key] = strconv.FormatInt(value, 10)
		}
	}

	resp := gen.HealthDetailed{
		Status:       gen.HealthDetailedStatus(rep.status),
		Version:      h.version,
		Environment:  h.environment,
		Dependencies: deps,
		Impaired:     toHealthDetailedImpaired(rep.impaired),
		Breakers:     toGenBreakers(rep.breakers),
	}
	if h.metrics != nil {
		metrics := h.metrics.Metrics()
		resp.Metrics = &metrics
	}
	if queues, err := h.queueStats(r.Context()); err != nil {
		// A failed backlog query must not fail the health check: the endpoint's
		// first job is to say whether the process is serving, and it still can.
		// Reported rather than swallowed, so the gap is visible as a gap.
		errMsg := err.Error()
		resp.QueuesError = &errMsg
	} else if queues != nil {
		genQueues := toGenQueueStats(queues)
		resp.Queues = &genQueues
	}

	h.respond.JSON(w, r, rep.code, envelopeFromHealthDetailed(resp))
}

// queueStats reads the backlogs under their own deadline.
func (h *Handler) queueStats(ctx context.Context) ([]QueueStats, error) {
	if h.queues == nil {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(ctx, queueTimeout)
	defer cancel()

	return h.queues.QueueStats(ctx)
}
