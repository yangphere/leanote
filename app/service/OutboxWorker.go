package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/yangphere/leanote/app/db"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const defaultOutboxPollInterval = time.Second

// OutboxWorker owns production scheduling around the persistence layer's
// lease-fenced delivery operation. It drains ready work synchronously and
// waits on a cancellable timer when the queue is empty or a retry was recorded.
type OutboxWorker struct {
	pollInterval time.Duration
	now          func() time.Time
	nextEvent    func(context.Context, time.Time) (db.OutboxEvent, error)
	deliverEvent func(context.Context, db.OutboxEvent, time.Time, db.OutboxTransport) error
	transport    db.OutboxTransport
	onError      func(error)
}

func NewOutboxWorker(transport db.OutboxTransport) *OutboxWorker {
	return &OutboxWorker{transport: transport, pollInterval: defaultOutboxPollInterval}
}

func (w *OutboxWorker) SetErrorHandler(handler func(error)) {
	w.onError = handler
}

func (w *OutboxWorker) Run(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	for {
		wait, err := w.processNext(ctx)
		if err != nil && w.onError != nil {
			w.onError(err)
		}
		if ctx.Err() != nil {
			return
		}
		if !wait {
			continue
		}
		interval := w.pollInterval
		if interval <= 0 {
			interval = defaultOutboxPollInterval
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
		}
	}
}

func (w *OutboxWorker) processNext(ctx context.Context) (bool, error) {
	if w == nil || w.transport == nil {
		return true, errors.New("outbox worker transport is not configured")
	}
	now := time.Now
	if w.now != nil {
		now = w.now
	}
	at := now()
	next := db.NextOutboxEvent
	if w.nextEvent != nil {
		next = w.nextEvent
	}
	event, err := next(ctx, at)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return true, nil
	}
	if err != nil {
		return true, fmt.Errorf("find deliverable outbox event: %w", err)
	}
	deliver := func(parent context.Context, event db.OutboxEvent, at time.Time, transport db.OutboxTransport) error {
		return db.DeliverOutbox(parent, event.ID, at, transport)
	}
	if w.deliverEvent != nil {
		deliver = w.deliverEvent
	}
	err = deliver(ctx, event, at, w.transport)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return true, nil
	}
	if err != nil {
		return true, fmt.Errorf("deliver outbox event %s: %w", event.ID.Hex(), err)
	}
	return false, nil
}
