package data

import (
	"context"
	"time"
)

const (
	// defaultTimeout is the maximum time a single database query should take.
	defaultTimeout = 3 * time.Second
	// txTimeout is the maximum time a database transaction should take.
	//
	// Every refund transaction fits inside it: the refund flow is claim, call the
	// provider, record, and only the first and last are transactions. Nothing waits
	// on MercadoPago with a transaction open, so no longer refund-specific budget
	// exists any more.
	txTimeout = 5 * time.Second
)

// queryContext returns a child context with defaultTimeout applied.
// Use this for all single-query database operations.
func queryContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, defaultTimeout)
}

// txContext returns a child context with txTimeout applied.
// Use this for database transactions that may involve multiple queries.
func txContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, txTimeout)
}

// detachedQueryContext returns a context for a query that must run even though
// the caller's context is already finished — cleanup, in practice — while still
// being bounded.
//
// context.WithoutCancel alone is not enough, and the reason is easy to miss: it
// strips the parent's DEADLINE as well as its cancellation, so the child reports
// Deadline() ok=false. Nothing else bounds such a query — this repository sets
// no statement_timeout anywhere — so a wedged PostgreSQL turns "clean up after
// yourself" into a goroutine and a pooled connection blocked forever.
//
// The budget is defaultTimeout because the work in question is a single query,
// which is exactly what that number is for. A longer one would be a number
// invented for this call site; a shorter one would abort unlocks that were about
// to succeed.
func detachedQueryContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return queryContext(context.WithoutCancel(ctx))
}
