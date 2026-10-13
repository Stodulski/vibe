package social

import (
	"context"
	"strings"
	"time"
)

// ThreadsKeywordScope is the Threads permission keyword search needs. The
// consent page asks for it, and a connection records that it did.
const ThreadsKeywordScope = "threads_keyword_search"

// The two orders a keyword search can return its results in.
const (
	KeywordSearchTop    = "TOP"
	KeywordSearchRecent = "RECENT"
)

// ThreadsKeywordPost is one public Threads post that a keyword search returned.
type ThreadsKeywordPost struct {
	ID         string
	Text       string
	Permalink  string
	Username   string
	Timestamp  time.Time
	HasReplies bool
}

// KeywordSearcher runs a keyword search as the connected account.
type KeywordSearcher interface {
	KeywordSearch(ctx context.Context, query, searchType string) ([]ThreadsKeywordPost, error)
}

const searchLinePrefix = "Búsqueda por palabra clave: "

// threadsSearchCheck is the keyword-search line of the connection email.
type threadsSearchCheck struct {
	store    ThreadsStore
	search   KeywordSearcher
	keywords []string
}

// NewThreadsSearchCheck returns the keyword-search line of the connection email.
func NewThreadsSearchCheck(store ThreadsStore, search KeywordSearcher, keywords []string) Check {
	return threadsSearchCheck{store: store, search: search, keywords: keywords}
}

// Check names the state of keyword search. A connection that predates the
// keyword scope has no permission to ask for, and needs the owner to authorize
// again. Otherwise a probe decides between approved and pending approval.
func (t threadsSearchCheck) Check(ctx context.Context, _ ConnectionReport) ReportLine {
	conn, ok, err := t.store.LoadThreads(ctx)
	switch {
	case err != nil:
		return ReportLine{Text: searchLinePrefix + "no se pudo verificar.", Problem: true}
	case !ok:
		return ReportLine{Text: searchLinePrefix + "sin permiso (no hay cuenta conectada)."}
	case !conn.KeywordSearchRequested:
		return ReportLine{Text: searchLinePrefix + "sin permiso. Volvé a conectar la cuenta para autorizarla.", Problem: true}
	}
	approved, err := probeKeywordApproval(ctx, t.search, t.keywords, conn.Username)
	switch {
	case err != nil:
		return ReportLine{Text: searchLinePrefix + "no se pudo verificar.", Problem: true}
	case approved:
		return ReportLine{Text: searchLinePrefix + "aprobada."}
	default:
		return ReportLine{Text: searchLinePrefix + "pendiente de aprobación de Meta."}
	}
}

// probeKeywordApproval reports whether a keyword returns a post by someone other
// than the connected account. While the app lacks the permission, Meta answers
// with the account's own posts alone, so only another account's post proves
// approval.
func probeKeywordApproval(ctx context.Context, search KeywordSearcher, keywords []string, own string) (bool, error) {
	for _, kw := range keywords {
		posts, err := search.KeywordSearch(ctx, kw, KeywordSearchTop)
		if err != nil {
			return false, err
		}
		for _, p := range posts {
			if !strings.EqualFold(p.Username, own) {
				return true, nil
			}
		}
	}
	return false, nil
}
