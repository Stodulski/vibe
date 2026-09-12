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
		_ = rdb.Close()
		return nil, fmt.Errorf("redis: connecting to %s: %w", opt.Addr, err)
	}

	return rdb, nil
}
