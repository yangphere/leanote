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
	OutboxStatusPending        = "pending"
	OutboxStatusSending        = "sending"
	OutboxStatusUnconfirmed    = "unconfirmed"
	OutboxStatusClaimed        = "claimed"
	OutboxStatusHandoffPending = "handoff_pending"
	OutboxStatusHandedOff      = "handed_off"
	OutboxStatusHandoffUnknown = "handoff_unknown"
	OutboxStatusRetry          = "retry"
	OutboxStatusSent           = "sent"
	OutboxStatusDead           = "dead"
	OutboxStatusCancelled      = "cancelled"

	OutboxLeaseTTL    = 2 * time.Minute
	OutboxMaxAttempts = 10
)

var (
	ErrOutboxHandoffUnknown    = errors.New("outbox transport handoff is unknown")
	ErrOutboxAlreadyHandedOff  = errors.New("outbox transport has already been handed off")
	ErrOutboxTransportRejected = errors.New("outbox transport rejected the message")
)

type OutboxEvent struct {
	ID                   domain.ObjectID `bson:"_id,omitempty"`
	IdempotencyKey       string          `bson:"IdempotencyKey,omitempty"`
	Kind                 string          `bson:"Kind"`
	AggregateID          domain.ObjectID `bson:"AggregateId"`
	Payload              map[string]any  `bson:"Payload,omitempty"`
	Status               string          `bson:"Status"`
	Attempts             int             `bson:"Attempts"`
	NextAttemptAt        time.Time       `bson:"NextAttemptAt"`
	LeaseUntil           time.Time       `bson:"LeaseUntil,omitempty"`
	LeaseID              string          `bson:"LeaseId,omitempty"`
	LastError            string          `bson:"LastError,omitempty"`
	CreatedAt            time.Time       `bson:"CreatedAt"`
	UpdatedAt            time.Time       `bson:"UpdatedAt"`
	Version              int64           `bson:"Version"`
	CancelRequested      bool            `bson:"CancelRequested"`
	CancelledAt          time.Time       `bson:"CancelledAt,omitempty"`
	TransportHandedOffAt time.Time       `bson:"TransportHandedOffAt,omitempty"`
}

type OutboxTransport func(context.Context, OutboxEvent) error

func (event OutboxEvent) IsConfirmedCommentNotification() bool {
	if event.Kind != "comment" || event.CancelRequested {
		return false
	}
	switch event.Status {
	case OutboxStatusPending, OutboxStatusRetry, OutboxStatusClaimed,
		OutboxStatusHandoffPending, OutboxStatusHandedOff,
		OutboxStatusHandoffUnknown, OutboxStatusSent, OutboxStatusDead:
		return true
	default:
		return false
	}
}

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
		if event.Kind == "comment" {
			event.Status = OutboxStatusUnconfirmed
		} else {
			event.Status = OutboxStatusPending
		}
	}
	if event.Version == 0 {
		event.Version = 1
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

func OutboxEventIDForKey(key string) domain.ObjectID {
	return outboxIDForKey(key)
}

func outboxClaimableFilter(now time.Time) bson.M {
	return bson.M{"$or": []bson.M{
		{"Status": bson.M{"$in": []string{OutboxStatusPending, OutboxStatusRetry}}, "NextAttemptAt": bson.M{"$lte": now}},
		{"Kind": "comment", "Status": bson.M{"$in": []string{OutboxStatusClaimed, OutboxStatusHandoffPending}}, "CancelRequested": bson.M{"$ne": true}, "LeaseUntil": bson.M{"$lte": now}},
		{"Status": OutboxStatusSending, "LeaseUntil": bson.M{"$lte": now}},
		{"Status": OutboxStatusSending, "LeaseUntil": bson.M{"$exists": false}, "UpdatedAt": bson.M{"$lte": now.Add(-OutboxLeaseTTL)}},
	}}
}

func commentOutboxClaimableFilter(now time.Time) bson.M {
	return bson.M{"Kind": "comment", "CancelRequested": bson.M{"$ne": true}, "$or": []bson.M{
		{"Status": bson.M{"$in": []string{OutboxStatusPending, OutboxStatusRetry}}, "NextAttemptAt": bson.M{"$lte": now}},
		{"Status": bson.M{"$in": []string{OutboxStatusClaimed, OutboxStatusHandoffPending}}, "LeaseUntil": bson.M{"$lte": now}},
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
	var kindProbe struct {
		Kind string `bson:"Kind"`
	}
	if err := Outbox.FindContext(parent, bson.M{"_id": eventID}).One(&kindProbe); err != nil {
		return fmt.Errorf("deliver outbox: inspect event kind: %w", err)
	}
	if kindProbe.Kind == "comment" {
		return deliverCommentOutbox(parent, eventID, now, transport)
	}
	claimCtx, cancelClaim := boundedOperationContext(parent)
	defer cancelClaim()
	leaseID := NewObjectID().Hex()
	var raw bson.M
	filter := outboxClaimableFilter(now)
	filter["_id"] = eventID
	filter["Kind"] = bson.M{"$ne": "comment"}
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

func deliverCommentOutbox(parent context.Context, eventID domain.ObjectID, now time.Time, transport OutboxTransport) error {
	leaseID := NewObjectID().Hex()
	claimCtx, cancel := boundedOperationContext(parent)
	defer cancel()
	var raw bson.M
	filter := commentOutboxClaimableFilter(now)
	filter["_id"] = eventID
	filter["Version"] = bson.M{"$gte": 1}
	err := Outbox.coll.FindOneAndUpdate(claimCtx,
		filter,
		bson.M{"$set": bson.M{"Status": OutboxStatusClaimed, "LeaseUntil": now.Add(OutboxLeaseTTL), "LeaseId": leaseID, "UpdatedAt": now}, "$inc": bson.M{"Attempts": 1, "Version": 1}},
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&raw)
	if err != nil {
		return err
	}
	event, err := decodeOutbox(raw)
	if err != nil {
		return err
	}
	if err := commentOutboxCAS(event, OutboxStatusClaimed, OutboxStatusHandoffPending, now, leaseID, false); err != nil {
		return err
	}
	event.Version++
	if err := commentOutboxCAS(event, OutboxStatusHandoffPending, OutboxStatusHandedOff, now, leaseID, true); err != nil {
		return err
	}
	event.Version++
	event.Status = OutboxStatusHandedOff
	event.TransportHandedOffAt = now
	transportCtx, cancelTransport := boundedOperationContext(parent)
	transportErr := transport(transportCtx, event)
	cancelTransport()
	if transportErr != nil {
		if errors.Is(transportErr, ErrOutboxTransportRejected) {
			if err := markCommentTransportRejected(event, now); err != nil {
				return fmt.Errorf("%w: event %s state update failed", ErrOutboxHandoffUnknown, event.ID.Hex())
			}
			return fmt.Errorf("%w: comment transport rejected event %s", ErrSideEffect, event.ID.Hex())
		}
		if err := markCommentHandoffUnknown(event, now); err != nil {
			return fmt.Errorf("%w: event %s state update failed", ErrOutboxHandoffUnknown, event.ID.Hex())
		}
		return fmt.Errorf("%w: event %s", ErrOutboxHandoffUnknown, event.ID.Hex())
	}
	statusCtx, cancelStatus := boundedOperationContext(context.Background())
	defer cancelStatus()
	result, err := Outbox.coll.UpdateOne(statusCtx, bson.M{"_id": event.ID, "Kind": "comment", "Status": OutboxStatusHandedOff, "Version": event.Version, "LeaseId": leaseID, "CancelRequested": bson.M{"$ne": true}}, bson.M{"$set": bson.M{"Status": OutboxStatusSent, "LastError": "", "UpdatedAt": now}, "$inc": bson.M{"Version": 1}, "$unset": bson.M{"LeaseUntil": "", "LeaseId": ""}})
	if err != nil {
		return fmt.Errorf("%w: mark sent: %v", ErrSideEffect, err)
	}
	if result.MatchedCount != 1 {
		return fmt.Errorf("%w: comment claim is no longer held", ErrSideEffect)
	}
	return nil
}

func commentOutboxCAS(event OutboxEvent, from, to string, now time.Time, leaseID string, handoff bool) error {
	set := bson.M{"Status": to, "UpdatedAt": now}
	if handoff {
		set["TransportHandedOffAt"] = now
	}
	ctx, cancel := boundedOperationContext(context.Background())
	defer cancel()
	result, err := Outbox.coll.UpdateOne(ctx, bson.M{"_id": event.ID, "Kind": "comment", "Status": from, "Version": event.Version, "LeaseId": leaseID, "CancelRequested": bson.M{"$ne": true}}, bson.M{"$set": set, "$inc": bson.M{"Version": 1}})
	if err != nil {
		return err
	}
	if result.MatchedCount != 1 {
		return fmt.Errorf("%w: comment state transition %s -> %s lost", ErrSideEffect, from, to)
	}
	return nil
}

func markCommentHandoffUnknown(event OutboxEvent, now time.Time) error {
	ctx, cancel := boundedOperationContext(context.Background())
	defer cancel()
	result, err := Outbox.coll.UpdateOne(ctx, bson.M{"_id": event.ID, "Kind": "comment", "Status": OutboxStatusHandedOff, "LeaseId": event.LeaseID, "Version": event.Version}, bson.M{"$set": bson.M{"Status": OutboxStatusHandoffUnknown, "LastError": "transport_handoff_unknown", "UpdatedAt": now}, "$inc": bson.M{"Version": 1}, "$unset": bson.M{"LeaseUntil": "", "LeaseId": ""}})
	if err != nil {
		return err
	}
	if result.MatchedCount != 1 {
		return errors.New("comment outbox handoff state changed")
	}
	return nil
}

func markCommentTransportRejected(event OutboxEvent, now time.Time) error {
	status := OutboxStatusRetry
	retryAt := now.Add(outboxRetryDelay(event.Attempts))
	if event.Attempts >= OutboxMaxAttempts {
		status = OutboxStatusDead
		retryAt = now
	}
	ctx, cancel := boundedOperationContext(context.Background())
	defer cancel()
	result, err := Outbox.coll.UpdateOne(ctx, bson.M{"_id": event.ID, "Kind": "comment", "Status": OutboxStatusHandedOff, "LeaseId": event.LeaseID, "Version": event.Version}, bson.M{
		"$set":   bson.M{"Status": status, "NextAttemptAt": retryAt, "LastError": "transport_rejected", "UpdatedAt": now},
		"$inc":   bson.M{"Version": 1},
		"$unset": bson.M{"LeaseUntil": "", "LeaseId": "", "TransportHandedOffAt": ""},
	})
	if err != nil {
		return err
	}
	if result.MatchedCount != 1 {
		return errors.New("comment outbox handoff state changed")
	}
	return nil
}

// ConfirmCommentOutbox is the only transition that makes an unconfirmed
// comment notification visible to workers.
func ConfirmCommentOutbox(ctx context.Context, eventID domain.ObjectID, now time.Time) error {
	if Outbox == nil {
		return ErrMongoClientNotInitialized
	}
	var event OutboxEvent
	if err := Outbox.FindContext(ctx, bson.M{"_id": eventID, "Kind": "comment"}).One(&event); err != nil {
		return err
	}
	if event.IsConfirmedCommentNotification() {
		return nil
	}
	result, err := Outbox.coll.UpdateOne(ctx, bson.M{"_id": eventID, "Kind": "comment", "Status": OutboxStatusUnconfirmed, "Version": event.Version, "CancelRequested": bson.M{"$ne": true}}, bson.M{"$set": bson.M{"Status": OutboxStatusPending, "NextAttemptAt": now, "UpdatedAt": now}, "$inc": bson.M{"Version": 1}})
	if err != nil {
		return err
	}
	if result.MatchedCount != 1 {
		return errors.New("comment outbox confirmation CAS failed")
	}
	return nil
}

func GetOutboxEvent(ctx context.Context, eventID domain.ObjectID) (OutboxEvent, error) {
	if Outbox == nil {
		return OutboxEvent{}, ErrMongoClientNotInitialized
	}
	var event OutboxEvent
	if err := Outbox.FindContext(ctx, bson.M{"_id": eventID}).One(&event); err != nil {
		return OutboxEvent{}, err
	}
	return event, nil
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

// CancelOutboxForAggregate prevents a comment notification from being sent
// after the comment has been deleted. The worker's lease is deliberately
// invalidated by the same CAS, so a worker that was only holding a snapshot
// cannot mark the event sent.
func CancelOutboxForAggregate(ctx context.Context, aggregateID domain.ObjectID, reason string) error {
	if Outbox == nil {
		return ErrMongoClientNotInitialized
	}
	if aggregateID.IsZero() {
		return errors.New("outbox aggregate id is required")
	}
	var events []OutboxEvent
	if err := Outbox.FindContext(ctx, bson.M{"AggregateId": aggregateID, "Kind": "comment"}).All(&events); err != nil {
		return err
	}
	for _, event := range events {
		if event.Status == OutboxStatusCancelled {
			continue
		}
		if event.Status == OutboxStatusHandedOff || event.Status == OutboxStatusSent || event.Status == OutboxStatusHandoffUnknown || !event.TransportHandedOffAt.IsZero() {
			return fmt.Errorf("%w: event %s", ErrOutboxAlreadyHandedOff, event.ID.Hex())
		}
		result, err := Outbox.coll.UpdateOne(ctx, bson.M{"_id": event.ID, "Kind": "comment", "Status": event.Status, "Version": event.Version, "CancelRequested": bson.M{"$ne": true}, "TransportHandedOffAt": bson.M{"$exists": false}}, bson.M{"$set": bson.M{"Status": OutboxStatusCancelled, "CancelRequested": true, "CancelledAt": time.Now(), "LastError": reason, "UpdatedAt": time.Now()}, "$inc": bson.M{"Version": 1}, "$unset": bson.M{"LeaseUntil": "", "LeaseId": ""}})
		if err != nil {
			return err
		}
		if result.MatchedCount != 1 {
			return fmt.Errorf("%w: cancel CAS lost for %s", ErrSideEffect, event.ID.Hex())
		}
	}
	return nil
}

// VerifyOutboxCancelledForAggregate confirms that every comment notification
// for an aggregate is in a terminal non-deliverable state. An empty result is
// valid for legacy comments that never created an outbox event.
func VerifyOutboxCancelledForAggregate(ctx context.Context, aggregateID domain.ObjectID) (bool, error) {
	if Outbox == nil {
		return false, ErrMongoClientNotInitialized
	}
	if aggregateID.IsZero() {
		return false, errors.New("outbox aggregate id is required")
	}
	var events []OutboxEvent
	if err := Outbox.FindContext(ctx, bson.M{"AggregateId": aggregateID, "Kind": "comment"}).All(&events); err != nil {
		return false, err
	}
	for _, event := range events {
		if event.Status == OutboxStatusCancelled {
			continue
		}
		if event.Status == OutboxStatusHandedOff || event.Status == OutboxStatusSent || event.Status == OutboxStatusHandoffUnknown || !event.TransportHandedOffAt.IsZero() {
			return false, fmt.Errorf("%w: event %s", ErrOutboxAlreadyHandedOff, event.ID.Hex())
		}
		return false, nil
	}
	return true, nil
}

func OutboxEventExists(ctx context.Context, idempotencyKey string) (bool, error) {
	if Outbox == nil {
		return false, ErrMongoClientNotInitialized
	}
	var event OutboxEvent
	err := Outbox.FindContext(ctx, bson.M{"IdempotencyKey": idempotencyKey}).One(&event)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
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

func (s *MemoryOutboxStore) ConfirmComment(id domain.ObjectID, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	event, ok := s.events[id]
	if !ok {
		return mongo.ErrNoDocuments
	}
	if event.IsConfirmedCommentNotification() {
		return nil
	}
	if event.Kind != "comment" || event.Status != OutboxStatusUnconfirmed || event.CancelRequested {
		return errors.New("comment outbox confirmation CAS failed")
	}
	event.Status, event.NextAttemptAt, event.UpdatedAt, event.Version = OutboxStatusPending, now, now, event.Version+1
	s.events[id] = event
	return nil
}

func (s *MemoryOutboxStore) CancelComment(id domain.ObjectID, reason string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	event, ok := s.events[id]
	if !ok {
		return mongo.ErrNoDocuments
	}
	if event.Kind != "comment" {
		return errors.New("outbox event is not a comment")
	}
	if event.Status == OutboxStatusCancelled {
		return nil
	}
	if event.Status == OutboxStatusHandedOff || event.Status == OutboxStatusSent || event.Status == OutboxStatusHandoffUnknown || !event.TransportHandedOffAt.IsZero() {
		return ErrOutboxAlreadyHandedOff
	}
	if event.CancelRequested {
		return nil
	}
	event.Status, event.CancelRequested, event.CancelledAt, event.LastError, event.LeaseID, event.LeaseUntil, event.UpdatedAt, event.Version = OutboxStatusCancelled, true, now, reason, "", time.Time{}, now, event.Version+1
	s.events[id] = event
	return nil
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
	if event.Kind == "comment" {
		s.mu.Unlock()
		return s.deliverComment(ctx, event, now, transport)
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

func (s *MemoryOutboxStore) deliverComment(ctx context.Context, event OutboxEvent, now time.Time, transport OutboxTransport) error {
	s.mu.Lock()
	if event.Status == OutboxStatusUnconfirmed || event.Status == OutboxStatusCancelled || event.Status == OutboxStatusHandoffUnknown {
		s.mu.Unlock()
		return fmt.Errorf("comment outbox event is not deliverable in state %s", event.Status)
	}
	if event.Status == OutboxStatusHandedOff || !event.TransportHandedOffAt.IsZero() {
		s.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrOutboxHandoffUnknown, event.ID.Hex())
	}
	switch event.Status {
	case OutboxStatusPending, OutboxStatusRetry:
		if now.Before(event.NextAttemptAt) {
			s.mu.Unlock()
			return fmt.Errorf("outbox event not ready until %s", event.NextAttemptAt.Format(time.RFC3339Nano))
		}
	case OutboxStatusClaimed, OutboxStatusHandoffPending:
		if now.Before(event.LeaseUntil) {
			s.mu.Unlock()
			return fmt.Errorf("outbox event lease held until %s", event.LeaseUntil.Format(time.RFC3339Nano))
		}
	default:
		s.mu.Unlock()
		return fmt.Errorf("comment outbox event is not deliverable in state %s", event.Status)
	}
	event.Status, event.Attempts = OutboxStatusClaimed, event.Attempts+1
	event.LeaseID, event.LeaseUntil, event.UpdatedAt = NewObjectID().Hex(), now.Add(OutboxLeaseTTL), now
	event.Version++
	s.events[event.ID] = event
	event.Status = OutboxStatusHandoffPending
	event.Version++
	if current := s.events[event.ID]; current.Status != OutboxStatusClaimed || current.Version != event.Version-1 || current.CancelRequested {
		s.mu.Unlock()
		return fmt.Errorf("%w: comment claim changed", ErrSideEffect)
	}
	s.events[event.ID] = event
	if current := s.events[event.ID]; current.Status != OutboxStatusHandoffPending || current.CancelRequested {
		s.mu.Unlock()
		return fmt.Errorf("%w: comment cancellation won", ErrSideEffect)
	}
	event.Status, event.TransportHandedOffAt, event.Version = OutboxStatusHandedOff, now, event.Version+1
	s.events[event.ID] = event
	leaseID := event.LeaseID
	s.mu.Unlock()
	transportErr := transport(ctx, event)
	s.mu.Lock()
	defer s.mu.Unlock()
	if transportErr != nil {
		current := s.events[event.ID]
		if current.Status != OutboxStatusHandedOff || current.LeaseID != leaseID {
			return fmt.Errorf("%w: comment claim is no longer held", ErrSideEffect)
		}
		if errors.Is(transportErr, ErrOutboxTransportRejected) {
			event.Status, event.NextAttemptAt, event.LastError = OutboxStatusRetry, now.Add(outboxRetryDelay(event.Attempts)), "transport_rejected"
			if event.Attempts >= OutboxMaxAttempts {
				event.Status, event.NextAttemptAt = OutboxStatusDead, now
			}
			event.LeaseID, event.LeaseUntil, event.TransportHandedOffAt, event.Version = "", time.Time{}, time.Time{}, current.Version+1
			s.events[event.ID] = event
			return fmt.Errorf("%w: comment transport rejected event %s", ErrSideEffect, event.ID.Hex())
		}
		event.Status, event.LastError, event.LeaseID, event.LeaseUntil, event.Version = OutboxStatusHandoffUnknown, "transport_handoff_unknown", "", time.Time{}, current.Version+1
		s.events[event.ID] = event
		return fmt.Errorf("%w: event %s", ErrOutboxHandoffUnknown, event.ID.Hex())
	}
	current := s.events[event.ID]
	if current.Status != OutboxStatusHandedOff || current.Version != event.Version || current.LeaseID != leaseID {
		return fmt.Errorf("%w: comment claim is no longer held", ErrSideEffect)
	}
	event.Status, event.LeaseID, event.LeaseUntil, event.LastError, event.Version = OutboxStatusSent, "", time.Time{}, "", current.Version+1
	s.events[event.ID] = event
	return nil
}
