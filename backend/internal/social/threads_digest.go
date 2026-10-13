package social

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/stodulski/vibe-server/internal/jobs"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// ThreadsDigestOutcome names why one weekly run mailed or did not.
type ThreadsDigestOutcome string

// The outcomes of one run of the digest.
const (
	ThreadsDigestNotDue       ThreadsDigestOutcome = "not_due"
	ThreadsDigestNoConnection ThreadsDigestOutcome = "no_connection"
	ThreadsDigestNotRequested ThreadsDigestOutcome = "not_requested"
	ThreadsDigestEmpty        ThreadsDigestOutcome = "empty"
	ThreadsDigestQueued       ThreadsDigestOutcome = "queued"
)

const (
	digestSubject  = "Threads: conversaciones de la semana"
	digestIntro    = "Publicaciones de otras cuentas de los últimos 7 días, por palabra clave. Solo lectura: no se respondió a nadie."
	digestWindow   = 7 * 24 * time.Hour
	digestMaxPosts = 30
	digestExcerpt  = 200
)

// digestModes are the two orders each keyword is searched in.
var digestModes = []string{KeywordSearchTop, KeywordSearchRecent}

// ThreadsDigestConfig tunes the weekly digest.
type ThreadsDigestConfig struct {
	To       string
	Keywords []string
	Now      func() time.Time
}

// ThreadsDigest mails the week's conversations about the configured keywords.
// It reads only: it never replies to or interacts with a post.
type ThreadsDigest struct {
	store  ThreadsStore
	search KeywordSearcher
	queue  Enqueuer
	cfg    ThreadsDigestConfig
	logger *slog.Logger
}

// NewThreadsDigest returns the weekly digest.
func NewThreadsDigest(store ThreadsStore, search KeywordSearcher, queue Enqueuer, cfg ThreadsDigestConfig, logger *slog.Logger) *ThreadsDigest {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &ThreadsDigest{store: store, search: search, queue: queue, cfg: cfg, logger: logger}
}

// Run does one pass. It does nothing outside Monday morning in Buenos Aires.
// The job runs hourly, so the two hours from 09:00 let one failed tick be
// retried; the queue's dedup key keeps the week to one email either way.
func (d *ThreadsDigest) Run(ctx context.Context) (ThreadsDigestOutcome, error) {
	now := d.cfg.Now().In(timezone.Argentina)
	if !digestDue(now) {
		return ThreadsDigestNotDue, nil
	}
	conn, ok, err := d.store.LoadThreads(ctx)
	if err != nil {
		return "", fmt.Errorf("social: load threads connection: %w", err)
	}
	if !ok {
		return ThreadsDigestNoConnection, nil
	}
	if !conn.KeywordSearchRequested {
		d.logger.Info("social: threads keyword digest skipped, the connection predates the keyword scope")
		return ThreadsDigestNotRequested, nil
	}
	hits, err := d.collect(ctx, conn.Username, now)
	if err != nil {
		return "", err
	}
	if len(hits) == 0 {
		d.logger.Info("social: threads keyword digest has no conversations this week")
		return ThreadsDigestEmpty, nil
	}
	year, week := now.ISOWeek()
	payload := ReportPayload{
		To:      d.cfg.To,
		Subject: digestSubject,
		Lines:   append([]string{digestIntro}, digestLines(hits, d.cfg.Keywords)...),
	}
	d.queue.Enqueue(TaskReport, payload, jobs.DedupKey(TaskReport, d.cfg.To, fmt.Sprintf("threads-digest:%d-W%02d", year, week)))
	d.logger.Info("social: threads keyword digest queued", "posts", len(hits))
	return ThreadsDigestQueued, nil
}

// digestDue is true on Monday between 09:00 and 10:59 in Buenos Aires.
func digestDue(now time.Time) bool {
	return now.Weekday() == time.Monday && (now.Hour() == 9 || now.Hour() == 10)
}

type digestHit struct {
	keyword string
	post    ThreadsKeywordPost
}

// collect searches every keyword in both orders and keeps each other account's
// post from the last seven days once, under the first keyword that found it.
// Our own posts are dropped, so while the app is not approved, Meta's answer
// (our own posts only) yields nothing to mail. The best posts come first: those
// with replies, then the newest, capped at digestMaxPosts.
func (d *ThreadsDigest) collect(ctx context.Context, own string, now time.Time) ([]digestHit, error) {
	cutoff := now.Add(-digestWindow)
	seen := map[string]bool{}
	var hits []digestHit
	for _, kw := range d.cfg.Keywords {
		for _, mode := range digestModes {
			posts, err := d.search.KeywordSearch(ctx, kw, mode)
			if err != nil {
				return nil, fmt.Errorf("social: threads keyword search %q: %w", kw, err)
			}
			for _, p := range posts {
				if p.ID == "" || seen[p.ID] || strings.EqualFold(p.Username, own) || p.Timestamp.Before(cutoff) || p.Timestamp.After(now) {
					continue
				}
				seen[p.ID] = true
				hits = append(hits, digestHit{keyword: kw, post: p})
			}
		}
	}
	slices.SortStableFunc(hits, func(a, b digestHit) int {
		switch {
		case a.post.HasReplies != b.post.HasReplies:
			if a.post.HasReplies {
				return -1
			}
			return 1
		case a.post.Timestamp.After(b.post.Timestamp):
			return -1
		case a.post.Timestamp.Before(b.post.Timestamp):
			return 1
		default:
			return 0
		}
	})
	if len(hits) > digestMaxPosts {
		hits = hits[:digestMaxPosts]
	}
	return hits, nil
}

// digestLines groups the hits by keyword, in the configured keyword order. Each
// post is two lines: who, when and where, then an excerpt of its text.
func digestLines(hits []digestHit, keywords []string) []string {
	var lines []string
	for _, kw := range keywords {
		var group []digestHit
		for _, h := range hits {
			if h.keyword == kw {
				group = append(group, h)
			}
		}
		if len(group) == 0 {
			continue
		}
		lines = append(lines, "Palabra clave: "+kw)
		for _, h := range group {
			p := h.post
			lines = append(lines, "@"+p.Username+" · "+p.Timestamp.In(timezone.Argentina).Format("02/01/2006")+" · "+p.Permalink)
			lines = append(lines, excerpt(p.Text))
		}
	}
	return lines
}

// excerpt collapses whitespace and keeps the first digestExcerpt characters.
func excerpt(text string) string {
	clean := strings.Join(strings.Fields(text), " ")
	runes := []rune(clean)
	if len(runes) <= digestExcerpt {
		return clean
	}
	return string(runes[:digestExcerpt]) + "…"
}
