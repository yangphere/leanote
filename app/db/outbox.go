package db

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/yangphere/leanote/app/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	OutboxStatusPending = "pending"
	OutboxStatusSending = "sending"
	OutboxStatusRetry   = "retry"
	OutboxStatusSent    = "sent"
	OutboxStatusDead    = "dead"

	OutboxLeaseTTL    = 2 * time.Minute
	OutboxMaxAttempts = 10
)

type OutboxEvent struct {
	ID             domain.ObjectID `bson:"_id,omitempty"`
	IdempotencyKey string          `bson:"IdempotencyKey,omitempty"`
	Kind           string          `bson:"Kind"`
	AggregateID    domain.ObjectID `bson:"AggregateId"`
	Payload        map[string]any  `bson:"Payload,omitempty"`
	Status         string          `bson:"Status"`
	Attempts       int             `bson:"Attempts"`
	NextAttemptAt  time.Time       `bson:"NextAttemptAt"`
	LeaseUntil     time.Time       `bson:"LeaseUntil,omitempty"`
	LeaseID        string          `bson:"LeaseId,omitempty"`
	LastError      string          `bson:"LastError,omitempty"`
	CreatedAt      time.Time       `bson:"CreatedAt"`
	UpdatedAt      time.Time       `bson:"UpdatedAt"`
}

type OutboxTransport func(context.Context, OutboxEvent) error

// EnqueueOutbox persists an event and is safe to call with a transaction
// context. It deliberately returns the insert error so registration can report
// side_effect/partial_write instead of claiming success.
func EnqueueOutbox(parent context.Context, event OutboxEvent) error {
	_, err := EnqueueOutboxEvent(parent, event)
	return err
}

// EnqueueOutboxEvent is the value-returning form used by callers that need the
// generated event ID for observability or a retry receipt.
func EnqueueOutboxEvent(parent context.Context, event OutboxEvent) (OutboxEvent, error) {
	if Outbox == nil {
		return OutboxEvent{}, fmt.Errorf("enqueue outbox: %w", ErrMongoClientNotInitialized)
	}
	if event.ID.IsZero() && event.IdempotencyKey == "" {
		return OutboxEvent{}, fmt.Errorf("enqueue outbox: stable event identity is required")
	}
	event = normalizeOutboxEvent(event, time.Now())
	if event.Kind == "" || event.AggregateID.IsZero() {
		return OutboxEvent{}, fmt.Errorf("enqueue outbox: invalid event")
	}
	if err := Outbox.InsertContext(parent, event); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			// An acknowledgement can be lost after the insert commits.  A
			// deterministic id/key lets a retry return the committed event
			// instead of creating a second delivery.
			var raw bson.M
			filter := bson.M{"_id": event.ID}
			if event.IdempotencyKey != "" {
				filter["IdempotencyKey"] = event.IdempotencyKey
			}
			lookupCtx, cancel := boundedOperationContext(context.Background())
			lookupErr := Outbox.coll.FindOne(lookupCtx, filter).Decode(&raw)
			cancel()
			if lookupErr == nil {
				if existing, decodeErr := decodeOutbox(raw); decodeErr == nil {
					return existing, nil
				}
			}
		}
		return OutboxEvent{}, fmt.Errorf("enqueue outbox: %w", err)
	}
	return event, nil
}

func normalizeOutboxEvent(event OutboxEvent, now time.Time) OutboxEvent {
	if event.ID.IsZero() {
		if event.IdempotencyKey != "" {
			event.ID = outboxIDForKey(event.IdempotencyKey)
		} else {
			event.ID = NewObjectID()
		}
	}
	if event.Status == "" {
		event.Status = OutboxStatusPending
	}
	if event.NextAttemptAt.IsZero() {
		event.NextAttemptAt = now
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = now
	}
	if event.UpdatedAt.IsZero() {
		event.UpdatedAt = now
	}
	return event
}

func outboxIDForKey(key string) domain.ObjectID {
	digest := sha256.Sum256([]byte(key))
	var id domain.ObjectID
	copy(id[:], digest[:len(id)])
	return id
}

func outboxClaimableFilter(now time.Time) bson.M {
	return bson.M{"$or": []bson.M{
		{"Status": bson.M{"$in": []string{OutboxStatusPending, OutboxStatusRetry}}, "NextAttemptAt": bson.M{"$lte": now}},
		{"Status": OutboxStatusSending, "LeaseUntil": bson.M{"$lte": now}},
		{"Status": OutboxStatusSending, "LeaseUntil": bson.M{"$exists": false}, "UpdatedAt": bson.M{"$lte": now.Add(-OutboxLeaseTTL)}},
	}}
}

// NextOutboxEvent returns the identity of the oldest deliverable event. The
// actual lease is still acquired by DeliverOutbox, so multiple workers may
// safely observe the same candidate without delivering it twice.
func NextOutboxEvent(parent context.Context, now time.Time) (OutboxEvent, error) {
	if Outbox == nil {
		return OutboxEvent{}, fmt.Errorf("find next outbox event: %w", ErrMongoClientNotInitialized)
	}
	ctx, cancel := boundedOperationContext(parent)
	defer cancel()
	var raw bson.M
	err := Outbox.coll.FindOne(
		ctx,
		outboxClaimableFilter(now),
		options.FindOne().
			SetProjection(bson.M{"_id": 1}).
			SetSort(bson.D{{Key: "NextAttemptAt", Value: 1}, {Key: "CreatedAt", Value: 1}, {Key: "_id", Value: 1}}),
	).Decode(&raw)
	if err != nil {
		return OutboxEvent{}, err
	}
	id, err := decodeObjectIDValue(raw["_id"])
	if err != nil || id.IsZero() {
		return OutboxEvent{}, fmt.Errorf("decode next outbox event id: %w", err)
	}
	return OutboxEvent{ID: id}, nil
}

// DeliverOutbox claims and delivers one event synchronously. No goroutine is
// created here; the caller owns scheduling and can retry the returned
// side_effect error through a durable worker loop.
func DeliverOutbox(parent context.Context, eventID domain.ObjectID, now time.Time, transport OutboxTransport) error {
	if Outbox == nil {
		return fmt.Errorf("deliver outbox: %w", ErrMongoClientNotInitialized)
	}
	if transport == nil {
		return fmt.Errorf("deliver outbox: nil transport")
	}
	claimCtx, cancelClaim := boundedOperationContext(parent)
	defer cancelClaim()
	leaseID := NewObjectID().Hex()
	var raw bson.M
	filter := outboxClaimableFilter(now)
	filter["_id"] = eventID
	err := Outbox.coll.FindOneAndUpdate(claimCtx, filter, bson.M{
		"$set": bson.M{"Status": OutboxStatusSending, "LeaseUntil": now.Add(OutboxLeaseTTL), "LeaseId": leaseID, "UpdatedAt": now},
		"$inc": bson.M{"Attempts": 1},
	}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&raw)
	if err != nil {
		return err
	}
	event, err := decodeOutbox(raw)
	if err != nil {
		attempts := outboxAttemptCount(raw)
		if updateErr := recordOutboxFailure(eventID, leaseID, attempts, now, fmt.Errorf("decode outbox: %w", err)); updateErr != nil {
			return fmt.Errorf("decode outbox: %w (record retry state: %v)", err, updateErr)
		}
		return fmt.Errorf("decode outbox: %w", err)
	}
	transportCtx, cancelTransport := boundedOperationContext(parent)
	transportErr := transport(transportCtx, event)
	cancelTransport()
	if transportErr != nil {
		updateErr := recordOutboxFailure(event.ID, event.LeaseID, event.Attempts, now, transportErr)
		if updateErr != nil {
			return fmt.Errorf("%w: %v (record retry state: %v)", ErrSideEffect, transportErr, updateErr)
		}
		return fmt.Errorf("%w: %v", ErrSideEffect, transportErr)
	}
	statusCtx, cancelStatus := boundedOperationContext(context.Background())
	defer cancelStatus()
	result, err := Outbox.coll.UpdateOne(statusCtx, bson.M{"_id": event.ID, "Status": OutboxStatusSending, "LeaseId": event.LeaseID}, bson.M{
		"$set":   bson.M{"Status": OutboxStatusSent, "LastError": "", "UpdatedAt": now},
		"$unset": bson.M{"LeaseUntil": "", "LeaseId": ""},
	})
	if err != nil {
		return fmt.Errorf("%w: mark sent: %v", ErrSideEffect, err)
	}
	if result.MatchedCount != 1 {
		return fmt.Errorf("%w: mark sent: claim is no longer held", ErrSideEffect)
	}
	return nil
}

func recordOutboxFailure(eventID domain.ObjectID, leaseID string, attempts int, now time.Time, deliveryErr error) error {
	status := OutboxStatusRetry
	retryAt := now.Add(outboxRetryDelay(attempts))
	if attempts >= OutboxMaxAttempts {
		status = OutboxStatusDead
		retryAt = now
	}
	statusCtx, cancel := boundedOperationContext(context.Background())
	defer cancel()
	result, err := Outbox.coll.UpdateOne(statusCtx, bson.M{"_id": eventID, "Status": OutboxStatusSending, "LeaseId": leaseID}, bson.M{
		"$set":   bson.M{"Status": status, "NextAttemptAt": retryAt, "LastError": deliveryErr.Error(), "UpdatedAt": now},
		"$unset": bson.M{"LeaseUntil": "", "LeaseId": ""},
	})
	if err != nil {
		return err
	}
	if result.MatchedCount != 1 {
		return errors.New("outbox claim is no longer held")
	}
	return nil
}

func decodeOutbox(raw bson.M) (OutboxEvent, error) {
	var event OutboxEvent
	if err := decodeBSONMap(raw, &event); err != nil {
		return event, err
	}
	return event, nil
}

func outboxAttemptCount(raw bson.M) int {
	attempts, err := intValue(raw["Attempts"])
	if err != nil || attempts < 1 {
		return 1
	}
	return attempts
}

func outboxRetryDelay(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	minutes := math.Pow(2, float64(attempts-1))
	if minutes > 60 {
		minutes = 60
	}
	return time.Duration(minutes) * time.Minute
}

// MemoryOutboxStore is a deterministic seam for service tests. Production
// code uses the Mongo functions above; this store has the same durable state
// transitions without starting background work.
type MemoryOutboxStore struct {
	mu     sync.Mutex
	events map[domain.ObjectID]OutboxEvent
}

func NewMemoryOutboxStore() *MemoryOutboxStore {
	return &MemoryOutboxStore{events: make(map[domain.ObjectID]OutboxEvent)}
}

func (s *MemoryOutboxStore) Enqueue(_ context.Context, event OutboxEvent) error {
	if s == nil {
		return errors.New("nil outbox store")
	}
	if event.ID.IsZero() && event.IdempotencyKey == "" {
		return errors.New("outbox stable event identity is required")
	}
	now := time.Now()
	event = normalizeOutboxEvent(event, now)
	if event.Kind == "" || event.AggregateID.IsZero() {
		return errors.New("invalid outbox event")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, exists := s.events[event.ID]; exists {
		if event.IdempotencyKey != "" && existing.IdempotencyKey == event.IdempotencyKey {
			return nil
		}
		return fmt.Errorf("outbox event %s already exists", event.ID.Hex())
	}
	s.events[event.ID] = event
	return nil
}

func (s *MemoryOutboxStore) Get(id domain.ObjectID) (OutboxEvent, bool) {
	if s == nil {
		return OutboxEvent{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	event, ok := s.events[id]
	return event, ok
}

func (s *MemoryOutboxStore) Deliver(ctx context.Context, id domain.ObjectID, now time.Time, transport OutboxTransport) error {
	if s == nil {
		return errors.New("nil outbox store")
	}
	if transport == nil {
		return errors.New("nil transport")
	}
	s.mu.Lock()
	event, ok := s.events[id]
	if !ok {
		s.mu.Unlock()
		return mongo.ErrNoDocuments
	}
	if event.Status == OutboxStatusSent {
		s.mu.Unlock()
		return nil
	}
	if event.Status == OutboxStatusDead {
		s.mu.Unlock()
		return fmt.Errorf("outbox event exceeded maximum attempts")
	}
	if event.Status == OutboxStatusSending && now.Before(event.LeaseUntil) {
		s.mu.Unlock()
		return fmt.Errorf("outbox event lease held until %s", event.LeaseUntil.Format(time.RFC3339Nano))
	}
	if event.Status != OutboxStatusSending && now.Before(event.NextAttemptAt) {
		s.mu.Unlock()
		return fmt.Errorf("outbox event not ready until %s", event.NextAttemptAt.Format(time.RFC3339Nano))
	}
	event.Status = OutboxStatusSending
	event.Attempts++
	event.LeaseUntil = now.Add(OutboxLeaseTTL)
	event.LeaseID = NewObjectID().Hex()
	event.UpdatedAt = now
	s.events[id] = event
	s.mu.Unlock()

	if err := transport(ctx, event); err != nil {
		s.mu.Lock()
		current, ok := s.events[id]
		if !ok || current.Status != OutboxStatusSending || current.LeaseID != event.LeaseID {
			s.mu.Unlock()
			return fmt.Errorf("%w: outbox claim is no longer held", ErrSideEffect)
		}
		event.Status = OutboxStatusRetry
		event.NextAttemptAt = now.Add(outboxRetryDelay(event.Attempts))
		if event.Attempts >= OutboxMaxAttempts {
			event.Status = OutboxStatusDead
			event.NextAttemptAt = now
		}
		event.LeaseUntil = time.Time{}
		event.LeaseID = ""
		event.LastError = err.Error()
		event.UpdatedAt = now
		s.events[id] = event
		s.mu.Unlock()
		return fmt.Errorf("%w: %v", ErrSideEffect, err)
	}

	s.mu.Lock()
	current, ok := s.events[id]
	if !ok || current.Status != OutboxStatusSending || current.LeaseID != event.LeaseID {
		s.mu.Unlock()
		return fmt.Errorf("%w: outbox claim is no longer held", ErrSideEffect)
	}
	event.Status = OutboxStatusSent
	event.LastError = ""
	event.LeaseUntil = time.Time{}
	event.LeaseID = ""
	event.UpdatedAt = now
	s.events[id] = event
	s.mu.Unlock()
	return nil
}
