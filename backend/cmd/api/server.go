package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// shutdownTimeout is how long in-flight requests get to finish once a stop
// signal arrives. It bounds srv.Shutdown only; the cleanup that follows runs
// whether or not that wait succeeded.
const shutdownTimeout = 30 * time.Second

// wait for shutdown signal, graceful shutdown); splitting would fragment a single linear sequence
// into arbitrarily-named helpers without clarifying it.
//
//nolint:funlen // flat sequential server-lifecycle wiring (configure http.Server, start listener,
func (app *application) serve() error {
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", app.config.Port),
		Handler: app.routes(),
		// ReadHeaderTimeout is the Slowloris bound: ReadTimeout does not
		// cover a peer that opens a connection and then sends its headers a
		// byte at a time, because the read deadline it sets is only useful
		// once there is a request to read. All four come from configuration
		// (internal/platform/config.HTTP), so a deployment whose proxy caps
		// requests lower can match it without a rebuild.
		ReadHeaderTimeout: app.config.HTTP.ReadHeaderTimeout,
		ReadTimeout:       app.config.HTTP.ReadTimeout,
		WriteTimeout:      app.config.HTTP.WriteTimeout,
		IdleTimeout:       app.config.HTTP.IdleTimeout,
		ErrorLog:          nil,
	}

	shutdownError := make(chan error)

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		s := <-quit

		app.logger.Info("shutting down server", "signal", s.String())

		shutdownError <- app.gracefulShutdown(srv, shutdownTimeout)
	}()

	app.logger.Info("starting server", "addr", srv.Addr, "env", app.config.Env)

	err := srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	err = <-shutdownError
	if err != nil {
		return err
	}

	app.logger.Info("stopped server", "addr", srv.Addr)

	return nil
}

// gracefulShutdown stops the server and then everything the process owns behind
// it, returning whatever srv.Shutdown decided.
//
// Two things about the order are load-bearing.
//
// First, app.shutdown is closed BEFORE srv.Shutdown rather than after it.
// srv.Shutdown waits for in-flight requests and — by documented design — does
// not cancel their contexts. The SSE stream in internal/realtime/stream.go
// returns only on r.Context().Done() or on this channel, so with the close
// afterwards the two waited on each other: the stream waited for a signal that
// srv.Shutdown was holding, and srv.Shutdown waited for the stream. One open
// dashboard burned the entire timeout on every deploy. The channel exists
// precisely to break that cycle, so it has to be closed while Shutdown is
// waiting, not after it gives up.
//
// Closing it early is correct for each of its three consumers:
//
//   - The scheduler loops (internal/scheduler) return at their next select.
//     A run already in progress is unaffected — it holds its own
//     context.Background() bounded by jobTimeout — and app.wg.Wait() below
//     still waits for it. What stops is the STARTING of new cron work during
//     the drain, which is what should stop: taking a fresh advisory lock and
//     beginning a sweep in a process that is going away is work that will be
//     interrupted, not work that will finish.
//   - The in-memory rate limiter's eviction loop (internal/middleware) returns,
//     so its client map stops being pruned. Nothing degrades: eviction only
//     reclaims memory for addresses that have gone quiet, the map is bounded by
//     the addresses seen in the remaining seconds of this process's life, and
//     the limiters themselves keep enforcing their buckets for every request
//     still being served.
//   - The SSE streams end and their connections close, which is the whole
//     point. A dashboard reconnects to the next instance; holding the old one
//     open buys the client nothing and costs the deploy its timeout.
//
// The realtime hub and the notifier are NOT consumers of this channel — each
// owns a separate one, closed by its own Shutdown below — so closing early does
// not cut the hub's Redis subscription or the notifier's workers out from under
// work that is still running.
//
// Second, the cleanup runs even when srv.Shutdown returns an error. It used to
// return early on that path, which skipped every step below: the notifier queue
// is at-most-once and destructive-pop, so every task a worker held in memory was
// lost, and the Redis client was left unclosed. A shutdown that timed out is the
// case that needs the drain most, not least — a hung request is exactly when
// workers are most likely to be mid-task. The error is still reported to the
// caller, so the process exit code keeps saying the stop was not clean.
func (app *application) gracefulShutdown(srv *http.Server, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	close(app.shutdown)

	err := srv.Shutdown(ctx)
	if err != nil {
		app.logger.Error("shutdown: server did not stop cleanly, draining anyway", "error", err)
	}

	app.logger.Info("completing background tasks")
	app.wg.Wait()

	app.events.Shutdown()

	app.logger.Info("draining the job queue")
	app.jobs.Shutdown()

	if app.rdb != nil {
		if closeErr := app.rdb.Close(); closeErr != nil {
			app.logger.Error("shutdown: failed to close redis client", "error", closeErr)
		}
	}

	return err
}
