package social

import (
	"context"
	"time"
)

// TaskThreadsReconnect is the queue task that asks the owner to reconnect the
// Threads account. It is delivered by the same mailer as the other reports.
const TaskThreadsReconnect = "social_threads_reconnect"

// ThreadsConnection is the one Threads account Vibe publishes to. AccessToken
// is the long-lived token in plain text here only: the store seals it at rest,
// and nothing logs or reports it.
type ThreadsConnection struct {
	UserID      string
	Username    string
	AccessToken string
	// ConnectedAt is when the owner last authorized the account. Meta asks for
	// a new authorization about 90 days after it.
	ConnectedAt time.Time
	// RefreshedAt is when the current token was issued.
	RefreshedAt time.Time
	// ExpiresAt is when the current token stops working.
	ExpiresAt time.Time
	// KeywordSearchRequested is true when the consent that produced this token
	// asked for ThreadsKeywordScope. A connection made before that scope existed
	// reads false, and keyword search needs the owner to authorize again.
	KeywordSearchRequested bool
}

// ThreadsStore keeps the Threads connection. The table holds one row.
type ThreadsStore interface {
	// LoadThreads returns the connection; ok is false when none exists.
	LoadThreads(ctx context.Context) (conn ThreadsConnection, ok bool, err error)
	// SaveThreads replaces the connection and seals its token.
	SaveThreads(ctx context.Context, conn ThreadsConnection) error
}

// ThreadsDisconnecter removes the connection when Meta reports that the owner
// removed the app or asked for their data to be deleted. It removes only the
// account the request names, and reports the username it removed.
type ThreadsDisconnecter interface {
	DisconnectThreads(ctx context.Context, userID string) (username string, removed bool, err error)
}

// ThreadsToken is what a Threads OAuth call returns. The short-lived exchange
// answers without ExpiresIn; the long-lived and refreshed tokens answer with it.
type ThreadsToken struct {
	AccessToken string
	UserID      string
	ExpiresIn   time.Duration
}

// ThreadsAPI is the part of the Threads client the connection flow and the
// maintenance job use.
type ThreadsAPI interface {
	// AuthorizeURL is the page the owner grants access on.
	AuthorizeURL(state string) string
	// Exchange trades the code from the consent page for a short-lived token.
	Exchange(ctx context.Context, code string) (ThreadsToken, error)
	// LongLived trades a short-lived token for a 60-day one.
	LongLived(ctx context.Context, token string) (ThreadsToken, error)
	// Refresh renews a long-lived token that is at least a day old.
	Refresh(ctx context.Context, token string) (ThreadsToken, error)
	// Me returns the id and username the token belongs to.
	Me(ctx context.Context, token string) (id, username string, err error)
}
