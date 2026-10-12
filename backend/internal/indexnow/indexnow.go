// Package indexnow tells search engines that a complex's public pages changed.
//
// It speaks the IndexNow protocol, which Bing, Yandex and the others take. The
// landing's own workflow submits after each landing deploy, but creating,
// editing or deleting a complex never deploys, so without this package nothing
// told the engines about a new venue, a rename or a removal.
//
// A change is recorded as a durable job, never posted from the request. The job
// carries the URLs it submits, and its dedup key is the complex, the debounce
// window and the URL set: every save inside one window is one submission, while
// a change that touches other URLs (a rename, a city move) is not swallowed by a
// submission already queued for an earlier save.
package indexnow

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/jobs"
)

const (
	// Key is the IndexNow key. It is public by design: the protocol verifies the
	// host by fetching this exact text from https://<host>/<Key>.txt, and that
	// file is landing/public/<Key>.txt. Change both together; the test in this
	// package fails when they drift apart.
	Key = "469b1eba6dac3b5f5da13d17ea885719"

	// ProductionOrigin is the storefront's origin. IndexNow is switched on only
	// for the production deployment serving it; see Enabled.
	ProductionOrigin = "https://vibe.com.ar"

	// TaskSubmit is the job type of one submission.
	TaskSubmit = "indexnow.submit"

	// debounceWindow is how long a burst of saves is collapsed into one submission.
	debounceWindow = 10 * time.Minute
)

// Enabled reports whether this deployment submits to IndexNow. Only production,
// serving the public origin, does. Tests, local development and previews never
// reach the real endpoint, and production needs no new environment variable,
// because both facts are already part of its configuration (ENV=production and
// PUBLIC_URL=https://vibe.com.ar).
func Enabled(env, publicBaseURL string) bool {
	return env == "production" && strings.TrimRight(publicBaseURL, "/") == ProductionOrigin
}

// Snapshot is what a complex's public presence looked like at one moment: the
// slug its page answered under, the city whose hub lists it, and whether it was
// served at all (switched on and not deleted).
type Snapshot struct {
	Slug    string
	City    string
	Visible bool
}

// SnapshotOf reads a complex's public presence off its row. It is the one place
// that decides what "served" means for the purposes of this package.
func SnapshotOf(c *complexstore.Complex) Snapshot {
	return Snapshot{Slug: c.Slug, City: c.City, Visible: c.IsActive}
}

// Change is one write to a complex, as the complexes service saw it. Before is
// the zero Snapshot for a create and After the zero Snapshot for a delete.
type Change struct {
	ComplexID uuid.UUID
	Before    Snapshot
	After     Snapshot
	// RequestID is the request that made the change. It travels into the job's
	// log lines so a submission can be joined to the request that caused it.
	RequestID string
}

// URLsFor lists the public addresses a change affects, sorted and without
// duplicates. A complex's own page and its city hub are affected whenever it was
// served before the change or is served after it. The /canchas index is affected
// only when the complex enters or leaves the served set. A complex that was not
// served on either side affects nothing.
func URLsFor(origin string, c Change) []string {
	base := strings.TrimRight(origin, "/")
	set := map[string]bool{}
	addSide := func(s Snapshot) {
		set[base+"/c/"+s.Slug] = true
		if hub := complexstore.CitySlug(s.City); hub != "" {
			set[base+"/canchas/"+hub] = true
		}
	}
	if c.Before.Visible {
		addSide(c.Before)
	}
	if c.After.Visible {
		addSide(c.After)
	}
	if c.Before.Visible != c.After.Visible {
		set[base+"/canchas"] = true
	}

	urls := make([]string, 0, len(set))
	for u := range set {
		urls = append(urls, u)
	}
	slices.Sort(urls)
	return urls
}

// Queue is the durable queue submissions are recorded in. jobs.Enqueuer satisfies it.
type Queue interface {
	EnqueueAt(taskType string, payload any, runAt time.Time, dedupKey string)
}

// Service records submissions and posts them. Outside production, or without a
// durable queue, it does nothing at all.
type Service struct {
	enabled bool
	origin  string
	queue   Queue
	client  *Client
	logger  *slog.Logger
	now     func() time.Time
}

// NewService returns the IndexNow notifier for the deployment described by env
// and origin. A nil queue disables it.
func NewService(env, origin string, queue Queue, client *Client, logger *slog.Logger) *Service {
	return &Service{
		enabled: queue != nil && Enabled(env, origin),
		origin:  origin,
		queue:   queue,
		client:  client,
		logger:  logger,
		now:     time.Now,
	}
}

// ComplexChanged records one submission for the change, unless it touches no
// served page or the service is disabled.
//
// The job is due when the debounce window closes, not a fixed delay after the
// change. A submission posted while another save in the same window is still to
// come would share its dedup key and drop that later save, leaving it unannounced.
// Due at the window's end, every save in the window is in place before the
// engines are told.
func (s *Service) ComplexChanged(c Change) {
	if !s.enabled {
		return
	}
	urls := URLsFor(s.origin, c)
	if len(urls) == 0 {
		return
	}

	window := int64(debounceWindow / time.Second)
	bucket := s.now().Unix() / window
	runAt := time.Unix((bucket+1)*window, 0)
	key := jobs.DedupKey(TaskSubmit, c.ComplexID.String(), strconv.FormatInt(bucket, 10), strings.Join(urls, "\n"))
	s.queue.EnqueueAt(TaskSubmit, submission{URLs: urls, RequestID: c.RequestID}, runAt, key)
}

// submission is the payload of one job: the URLs to post and the request that
// caused them.
type submission struct {
	URLs      []string `json:"urls"`
	RequestID string   `json:"request_id,omitempty"`
}

// Submit is the job handler. A refusal the endpoint would give again is logged
// as a warning and dead-lettered. An outage is returned, for the queue to retry.
func (s *Service) Submit(ctx context.Context, payload json.RawMessage) error {
	var sub submission
	if err := json.Unmarshal(payload, &sub); err != nil {
		return fmt.Errorf("indexnow: undecodable submission: %w", jobs.ErrPermanent)
	}
	if len(sub.URLs) == 0 {
		return nil
	}

	err := s.client.Submit(ctx, sub.URLs)
	switch {
	case err == nil:
		s.logger.Info("indexnow: submitted", "urls", len(sub.URLs), "request_id", sub.RequestID)
	case jobs.Classify(err) == jobs.OutcomePermanent:
		s.logger.Warn("indexnow: submission refused, not retried",
			"urls", len(sub.URLs), "request_id", sub.RequestID, "error", err)
	default:
		s.logger.Warn("indexnow: submission failed, will retry",
			"urls", len(sub.URLs), "request_id", sub.RequestID, "error", err)
	}
	return err
}
