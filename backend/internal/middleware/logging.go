package middleware

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/httpx"
)

// slowRequest is the duration above which a request is always logged, whatever
// the sampling setting says.
//
// The whole reason this middleware exists is that a request which merely takes
// ten seconds produced no output at all. Sampling it away would put the failure
// back exactly where it was, so the sampler never sees a slow request.
const slowRequest = time.Second

// numLatencyBounds is len(latencyBounds), spelled as a constant because the
// histogram is a fixed-size array of atomics: a slice of them would have to be
// allocated and could be resized, and neither belongs on a per-request path.
const numLatencyBounds = 11

// latencyBounds are the upper bounds of the latency histogram, in ascending
// order. There is one more bucket than there are bounds: the overflow.
//
// Fixed buckets rather than a reservoir because the answer has to survive a
// scrape being missed and has to be cheap: recording a request is one atomic
// add on a bucket index found by a linear scan of eleven values, with no lock
// and no allocation. The resolution is deliberately coarse at the top — the
// difference between four seconds and five does not change what an operator
// does, whereas the difference between 50ms and 500ms does.
var latencyBounds = [numLatencyBounds]time.Duration{
	5 * time.Millisecond,
	10 * time.Millisecond,
	25 * time.Millisecond,
	50 * time.Millisecond,
	100 * time.Millisecond,
	250 * time.Millisecond,
	500 * time.Millisecond,
	time.Second,
	2500 * time.Millisecond,
	5 * time.Second,
	10 * time.Second,
}

// metrics is the request surface: volume, outcome, concurrency and latency.
//
// It is unsampled on purpose. Logging can be thinned because a log line is one
// request's story and losing nine in ten still leaves the shape; a percentile
// computed from a tenth of the requests is a different number wearing the same
// name. Whatever the sampler drops, these counters still counted.
type metrics struct {
	inFlight   atomic.Int64
	total      atomic.Int64
	streamed   atomic.Int64
	totalNanos atomic.Int64
	maxNanos   atomic.Int64
	// byClass is indexed by status/100, so index 2 is the 2xx count. Index 0
	// holds responses whose status was never written, which net/http turns
	// into a 200 on the wire but which is worth being able to see separately.
	byClass [6]atomic.Int64
	// buckets is cumulative-by-index, one per latencyBounds entry plus the
	// overflow.
	buckets [numLatencyBounds + 1]atomic.Int64
	// sampleTick drives the "1 in N" decision for successful requests.
	sampleTick atomic.Int64
}

func (m *metrics) begin() { m.inFlight.Add(1) }

// end records one finished request. streamed responses are counted but kept
// out of the latency histogram: an SSE stream is open for as long as the
// dashboard is, so a handful of them would push every percentile into the
// overflow bucket and the number an operator reads as "the API is slow" would
// really mean "somebody left a tab open".
func (m *metrics) end(status int, d time.Duration, streamed bool) {
	m.inFlight.Add(-1)
	m.total.Add(1)

	class := status / 100
	if class < 0 || class >= len(m.byClass) {
		class = 0
	}
	m.byClass[class].Add(1)

	if streamed {
		m.streamed.Add(1)
		return
	}

	nanos := d.Nanoseconds()
	m.totalNanos.Add(nanos)
	for {
		current := m.maxNanos.Load()
		if nanos <= current || m.maxNanos.CompareAndSwap(current, nanos) {
			break
		}
	}

	m.buckets[bucketOf(d)].Add(1)
}

// bucketOf returns the histogram index d falls in.
func bucketOf(d time.Duration) int {
	for i, bound := range latencyBounds {
		if d <= bound {
			return i
		}
	}
	return len(latencyBounds)
}

// snapshot renders the counters as a flat map for the metrics endpoint.
//
// Durations are milliseconds with microsecond resolution: this API's fast
// paths answer in well under a millisecond, and an integer millisecond field
// reports every one of them as zero — which reads exactly like a middleware
// that is not measuring anything.
func (m *metrics) snapshot() map[string]any {
	total := m.total.Load()
	streamed := m.streamed.Load()
	measured := total - streamed

	out := map[string]any{
		"requests_total":      total,
		"requests_in_flight":  m.inFlight.Load(),
		"responses_1xx":       m.byClass[1].Load(),
		"responses_2xx":       m.byClass[2].Load(),
		"responses_3xx":       m.byClass[3].Load(),
		"responses_4xx":       m.byClass[4].Load(),
		"responses_5xx":       m.byClass[5].Load(),
		"responses_unwritten": m.byClass[0].Load(),
		"streamed_total":      streamed,
		"latency_ms_max":      millis(time.Duration(m.maxNanos.Load())),
	}

	if measured > 0 {
		out["latency_ms_mean"] = millis(time.Duration(m.totalNanos.Load() / measured))
	} else {
		out["latency_ms_mean"] = 0.0
	}

	counts := make([]int64, len(m.buckets))
	histogram := make(map[string]int64, len(m.buckets))
	var cumulative int64
	for i := range m.buckets {
		counts[i] = m.buckets[i].Load()
		cumulative += counts[i]
		histogram[bucketLabel(i)] = cumulative
	}
	out["latency_histogram_le_ms"] = histogram

	for _, q := range []struct {
		name     string
		fraction float64
	}{{"p50", 0.50}, {"p90", 0.90}, {"p99", 0.99}} {
		out["latency_ms_"+q.name] = percentile(counts, measured, q.fraction, time.Duration(m.maxNanos.Load()))
	}

	return out
}

// bucketLabel names bucket i by its upper bound in milliseconds, "inf" for the
// overflow. Every bound is a whole number of milliseconds by construction.
func bucketLabel(i int) string {
	if i >= len(latencyBounds) {
		return "inf"
	}
	return strconv.FormatInt(latencyBounds[i].Milliseconds(), 10)
}

// percentile returns the upper bound of the bucket the quantile falls in.
//
// It is an upper bound, not an interpolation: saying "the p99 is at most 250ms"
// from counts that really only prove that is honest, where interpolating
// invents a decimal the data does not contain. The overflow bucket reports the
// observed maximum, which is the only real figure available up there.
func percentile(counts []int64, measured int64, fraction float64, max time.Duration) float64 {
	if measured == 0 {
		return 0
	}

	target := int64(float64(measured) * fraction)
	if target < 1 {
		target = 1
	}

	var cumulative int64
	for i, count := range counts {
		cumulative += count
		if cumulative >= target {
			if i >= len(latencyBounds) {
				return millis(max)
			}
			return millis(latencyBounds[i])
		}
	}
	return millis(max)
}

func millis(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000
}

// Metrics returns the request counters as a flat map, for whatever surface the
// composition root chooses to expose them on.
//
// It is a snapshot rather than a live view: the caller gets numbers that were
// all read within a few microseconds of each other, so requests_in_flight and
// requests_total cannot disagree by a whole scrape interval.
func (m *Middleware) Metrics() map[string]any {
	return m.metrics.snapshot()
}

// recorder captures what the handler below actually sent, so the log line and
// the counters describe the response rather than the intention.
type recorder struct {
	http.ResponseWriter
	// Written and read from the serving goroutine only — the deferred log runs
	// on it too — so no synchronisation is needed. Same argument as
	// responseTracker in chain.go.
	status  int
	written int64
	flushed bool
}

func (rec *recorder) WriteHeader(status int) {
	if rec.status == 0 {
		rec.status = status
	}
	rec.ResponseWriter.WriteHeader(status)
}

func (rec *recorder) Write(b []byte) (int, error) {
	if rec.status == 0 {
		rec.status = http.StatusOK
	}
	n, err := rec.ResponseWriter.Write(b)
	rec.written += int64(n)
	return n, err
}

// Flush both keeps SSE working through this wrapper and is the signal that
// this is a streaming response — see metrics.end.
func (rec *recorder) Flush() {
	if rec.status == 0 {
		rec.status = http.StatusOK
	}
	rec.flushed = true
	if flusher, ok := rec.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Unwrap lets http.ResponseController reach the real writer, which is how the
// stream handler clears its write deadline.
func (rec *recorder) Unwrap() http.ResponseWriter { return rec.ResponseWriter }

// LogRequests writes one structured line per request and keeps the counters
// behind Metrics.
//
// It sits directly inside RequestID and directly outside RecoverPanic, and
// both halves of that are deliberate. Inside RequestID, so every line carries
// the id the client was handed in X-Request-ID and a support ticket quoting it
// finds the request. Outside RecoverPanic, so a panicking handler is logged as
// the 500 the client actually received rather than as an unwritten response —
// the deferred record below still runs while the stack unwinds, so even the
// re-panic path RecoverPanic takes for an already-started response leaves a
// line behind.
func (m *Middleware) LogRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := m.now()
		rec := &recorder{ResponseWriter: w}
		// The slot authentication writes the actor's id into. It is placed
		// here, before the chain below runs, because this middleware wraps the
		// authenticator: the request it holds is not the one authentication
		// enriched, so the id has to come back through something shared.
		// Captured before the handler runs: the deferred log below is the one
		// place that must still be able to say which request this was, and by
		// the time it runs the handler may have replaced r entirely.
		ctx, actor := httpx.ContextWithActor(r.Context())
		r = r.WithContext(ctx)
		requestID := httpx.ContextGetRequestID(r)
		method, path := r.Method, r.URL.Path
		ip := m.clientIP(r)

		m.metrics.begin()
		defer func() {
			elapsed := m.now().Sub(start)
			status := rec.status
			if status == 0 {
				// net/http sends 200 for a handler that wrote nothing; the
				// counters keep it separate but the log line should say what
				// the client got.
				status = http.StatusOK
			}
			m.metrics.end(rec.status, elapsed, rec.flushed)

			if !m.shouldLog(status, elapsed) {
				return
			}

			attrs := []slog.Attr{
				slog.String("method", method),
				slog.String("route", routeOf(path)),
				slog.String("path", sanitize(path)),
				slog.Int("status", status),
				slog.Float64("duration_ms", millis(elapsed)),
				slog.Int64("bytes", rec.written),
				slog.String("request_id", requestID),
				slog.String("ip", ip),
			}
			// Only on a request that authenticated, and only the id: "which
			// account did this" is the question a support ticket asks, and the
			// id is the whole of the answer. Nothing else about the user
			// belongs in a log line.
			if id := actor.ID(); id != "" {
				attrs = append(attrs, slog.String("user_id", id))
			}
			m.logger.LogAttrs(ctx, slog.LevelInfo, "request", attrs...)
		}()

		next.ServeHTTP(rec, r)
	})
}

// shouldLog decides whether this request earns a line.
//
// Sampling is off by default (RequestLogSample of 0 or 1 logs everything) and
// is a deliberate choice rather than an oversight: this is a booking API for a
// countable number of venues, one line per request is affordable at its real
// volume, and the first question after an incident is almost always about one
// specific request rather than about the aggregate. The knob exists because
// that stops being true at some size, and because it is much easier to turn a
// documented flag up than to add sampling under load.
//
// What sampling never touches: anything that failed, and anything slow. Those
// are the lines the endpoint was built for.
func (m *Middleware) shouldLog(status int, d time.Duration) bool {
	if status >= http.StatusBadRequest || d >= slowRequest {
		return true
	}
	if m.cfg.RequestLogSample <= 1 {
		return true
	}
	return m.metrics.sampleTick.Add(1)%int64(m.cfg.RequestLogSample) == 0
}

// maxRouteLength bounds the route and path fields. A caller can put a kilobyte
// in a URL; nobody should be able to choose how wide our log lines are.
const maxRouteLength = 160

// routeOf collapses a request path to the route template it was matched by, so
// the same endpoint groups together instead of producing one distinct value per
// booking.
//
// The chain runs outside the router, so the matched template is not available
// here. Recognising the variable segments by shape gets the same answer for
// this API's routes, which name their entities by UUID, and it fails safe: an
// unrecognised segment is left alone rather than guessed at.
func routeOf(path string) string {
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		if isVariableSegment(segment) {
			segments[i] = ":id"
		}
	}
	return sanitize(strings.Join(segments, "/"))
}

// isVariableSegment reports whether a path segment is an entity identifier
// rather than a fixed part of the route.
func isVariableSegment(segment string) bool {
	if segment == "" {
		return false
	}
	if _, err := uuid.Parse(segment); err == nil {
		return true
	}
	for _, r := range segment {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// sanitize bounds a caller-supplied string and strips the control characters a
// caller could otherwise use to forge a line break in the log.
func sanitize(s string) string {
	if len(s) > maxRouteLength {
		s = s[:maxRouteLength] + "..."
	}
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return '�'
		}
		return r
	}, s)
}
