// Package redis owns the Redis client: how it is built and tuned. It is the
// only place outside its own users that names go-redis, so the composition
// root wires a client without importing a driver.
package redis

import (
	"context"
	"errors"
	"fmt"
	"net"
	"runtime"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Client is the Redis client the application holds, aliased for the same
// reason db.Pool is: every consumer already takes go-redis's own type.
type Client = goredis.Client

// Options is go-redis's own option set, exposed so a test can build a client
// pointed at a fake server without importing the driver.
type Options = goredis.Options

// NewClient builds a client from options directly, skipping Open's URL
// parsing and tuning. It exists for tests.
func NewClient(opt *Options) *Client { return goredis.NewClient(opt) }

// ErrNotConfigured says REDIS_URL was empty. It is a distinct error because
// "nobody configured Redis" and "Redis refused the connection" are different
// operator mistakes with the same consequence.
var ErrNotConfigured = errors.New("redis: REDIS_URL is not set")

// pingTimeout bounds the connectivity check Open makes before handing the
// client back.
const pingTimeout = 5 * time.Second

// Config is what Open needs.
type Config struct {
	URL string
}

// Open parses the URL, applies the pool tuning this deployment needs, and
// returns a client that has answered a ping.
func Open(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.URL == "" {
		return nil, ErrNotConfigured
	}

	opt, err := goredis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("redis: parsing REDIS_URL: %w", err)
	}

	// Explicit pool tuning: 4 BRPop workers hold connections permanently,
	// plus concurrent rate-limit, blacklist, SSE subscriber, and cache operations.
	opt.PoolSize = 10 * runtime.GOMAXPROCS(0)
	opt.MinIdleConns = 5
	opt.PoolFIFO = true // prefer recently used connections to reduce stale-conn errors
	opt.ConnMaxIdleTime = 5 * time.Minute
	opt.DialTimeout = 5 * time.Second
	opt.ReadTimeout = 3 * time.Second
	opt.WriteTimeout = 3 * time.Second
	opt.MaxRetries = 3
	// Aggressive TCP keepalive: Railway's proxy drops idle connections.
	// 30s probes keep long-lived connections (BRPop workers, SSE subscriber) alive.
	opt.Dialer = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return (&net.Dialer{
			Timeout:   opt.DialTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext(ctx, network, addr)
	}

	rdb := goredis.NewClient(opt)

	ctx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close() //nolint:errcheck // the ping already failed; this only releases the socket
		return nil, fmt.Errorf("redis: connecting to %s: %w", opt.Addr, err)
	}

	return rdb, nil
}

// KeyPrefix is the namespace every key this deployment writes lives under:
// "vibe:<env>:".
//
// It exists because nothing else guaranteed it (RED-01). Every key in this
// service was already built from a constant or a helper with a domain prefix —
// "cache:user:", "bl:token:", "sse:events" — and not one of them named the
// application or the environment. Two deployments pointed at one Redis, which
// is the ordinary shape of a managed instance with a staging database beside
// the production one, then share the token blacklist, the user cache and the
// SSE channel: a session revoked in staging is revoked in production, and a
// user edited in one is served stale from the other. Nothing fails; it just
// quietly answers with the wrong tenant's data.
//
// The environment is in the key rather than in a Redis logical database
// because a logical database is selected by the connection URL, which is the
// thing an operator gets wrong, and a key prefix is visible in every KEYS,
// SCAN and MONITOR line while a database number is not.
//
// An empty environment is deliberately not treated as the default one. It
// means nobody said, and sharing a namespace with development on a guess is
// exactly the collision this exists to prevent.
func KeyPrefix(env string) string {
	if env == "" {
		env = "unset"
	}
	return "vibe:" + env + ":"
}
