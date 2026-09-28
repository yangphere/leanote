package main

import (
	"context"
	"sync/atomic"
	"time"
)

const (
	readinessInitialBackoff = time.Second
	readinessMaxBackoff     = 30 * time.Second
)

// startupReadiness implements D-H9 for the plain Go entry: the application is
// ready only after the global configuration snapshot has been published.
//
// Retry semantics: db.InitWithError is safe to repeat. It resets every
// collection global, dials with a bounded connect timeout and, on failure,
// leaves client/database/collections nil. A retry therefore re-dials from a
// clean state until it succeeds once; afterwards only the snapshot load is
// retried. Both run exclusively on the startup/background goroutine while
// the httpserver readiness gate keeps every request (including OnRequest and
// the db.Ping health check) away from the db globals, so the atomic ready
// flag is the single happens-before edge to request goroutines.
type startupReadiness struct {
	ready           atomic.Bool
	databaseReady   bool
	connectDatabase func() error
	loadSnapshot    func() error
	initialBackoff  time.Duration
	maxBackoff      time.Duration
	wait            func(ctx context.Context, delay time.Duration) bool
	logf            func(format string, args ...interface{})
}

func newStartupReadiness(databaseReady bool, connectDatabase, loadSnapshot func() error, logf func(string, ...interface{})) *startupReadiness {
	return &startupReadiness{
		databaseReady:   databaseReady,
		connectDatabase: connectDatabase,
		loadSnapshot:    loadSnapshot,
		initialBackoff:  readinessInitialBackoff,
		maxBackoff:      readinessMaxBackoff,
		wait:            waitWithContext,
		logf:            logf,
	}
}

// Ready is the httpserver.App.Ready gate.
func (s *startupReadiness) Ready() bool { return s.ready.Load() }

// attempt connects MongoDB when the previous connection attempt failed, then
// loads the global configuration snapshot. It flips Ready only on success.
func (s *startupReadiness) attempt() error {
	if !s.databaseReady {
		if err := s.connectDatabase(); err != nil {
			return err
		}
		s.databaseReady = true
	}
	if err := s.loadSnapshot(); err != nil {
		return err
	}
	s.ready.Store(true)
	return nil
}

// run retries attempt with capped exponential backoff until it succeeds or
// ctx is cancelled (shutdown). It returns whether the application is ready.
// The first retry waits initialBackoff: callers have already made (or
// deliberately skipped) an immediate attempt during startup.
func (s *startupReadiness) run(ctx context.Context) bool {
	delay := s.initialBackoff
	lastPending := ""
	for !s.Ready() {
		if !s.wait(ctx, delay) {
			return false
		}
		if err := s.attempt(); err != nil {
			delay *= 2
			if delay > s.maxBackoff {
				delay = s.maxBackoff
			}
			// Errors can carry Mongo topology details; keep the log stable
			// and emit one notice per pending stage instead of one per retry.
			pending := "mongo unavailable"
			if s.databaseReady {
				pending = "global configuration unavailable"
			}
			if pending != lastPending {
				s.logf("startup readiness pending: %s; retrying with backoff up to %s", pending, s.maxBackoff)
				lastPending = pending
			}
			continue
		}
		s.logf("startup readiness reached: global configuration loaded")
	}
	return true
}

func waitWithContext(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
