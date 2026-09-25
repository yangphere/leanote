package db

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var ErrBroadcastMutationConflict = errors.New("broadcast mutation identity conflict")

// BroadcastMutation is the durable boundary for an administrator batch. The
// receipt and every recipient event must be observable before the caller is
// told that the batch was accepted.
type BroadcastMutation struct {
	Receipt info.BroadcastReceipt
	Events  []OutboxEvent
}

// CommitBroadcastMutation uses a Mongo transaction when available. Standalone
// deployments use deterministic identities and reconciliation; an uncertain
// write is never reported as a successful enqueue.
func CommitBroadcastMutation(ctx context.Context, mutation BroadcastMutation) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if BroadcastReceipts == nil || Outbox == nil {
		return fmt.Errorf("broadcast mutation: %w", ErrMongoClientNotInitialized)
	}
	if mutation.Receipt.ReceiptId.IsZero() || mutation.Receipt.Kind != "broadcast" || mutation.Receipt.BatchId == "" {
		return errors.New("broadcast mutation: receipt identity is required")
	}
	for _, event := range mutation.Events {
		if event.ID.IsZero() && event.IdempotencyKey == "" {
			return errors.New("broadcast mutation: event identity is required")
		}
	}
	apply := func(applyCtx context.Context) error {
		if err := ensureBroadcastReceipt(applyCtx, mutation.Receipt); err != nil {
			return fmt.Errorf("persist broadcast receipt: %w", err)
		}
		for _, event := range mutation.Events {
			if _, err := EnqueueOutboxEvent(applyCtx, event); err != nil {
				return fmt.Errorf("enqueue broadcast event: %w", err)
			}
		}
		return strictBroadcastReadBack(applyCtx, mutation)
	}
	if client != nil {
		session, err := client.StartSession()
		if err != nil {
			return fmt.Errorf("broadcast mutation session: %w", err)
		}
		defer session.EndSession(context.Background())
		_, txErr := session.WithTransaction(ctx, func(txCtx context.Context) (any, error) {
			return nil, apply(txCtx)
		})
		if txErr == nil {
			return strictBroadcastReadBack(context.Background(), mutation)
		}
		if !transactionUnsupported(txErr) {
			return txErr
		}
	}
	if err := apply(ctx); err != nil {
		if readErr := strictBroadcastReadBack(context.Background(), mutation); readErr == nil {
			return nil
		}
		return fmt.Errorf("%w: %v", ErrPartialWrite, err)
	}
	return strictBroadcastReadBack(context.Background(), mutation)
}

func ensureBroadcastReceipt(ctx context.Context, want info.BroadcastReceipt) error {
	var got info.BroadcastReceipt
	err := BroadcastReceipts.FindIdContext(ctx, want.ReceiptId).One(&got)
	if err == nil {
		if got.Kind != want.Kind || got.ActorId != want.ActorId || got.BatchId != want.BatchId ||
			got.TargetDigest != want.TargetDigest || got.BodyDigest != want.BodyDigest ||
			got.SchemaVersion != want.SchemaVersion || got.RetrySafe != want.RetrySafe || got.ReconciliationId != want.ReconciliationId ||
			!reflect.DeepEqual(got.RecipientSnapshot, want.RecipientSnapshot) || !reflect.DeepEqual(got.OutboxIds, want.OutboxIds) {
			return ErrBroadcastMutationConflict
		}
		return nil
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return err
	}
	return BroadcastReceipts.InsertContext(ctx, want)
}

func strictBroadcastReadBack(ctx context.Context, mutation BroadcastMutation) error {
	var receipt info.BroadcastReceipt
	if err := BroadcastReceipts.FindIdContext(ctx, mutation.Receipt.ReceiptId).One(&receipt); err != nil {
		return fmt.Errorf("broadcast read-back receipt: %w", err)
	}
	if receipt.Kind != "broadcast" || receipt.ActorId != mutation.Receipt.ActorId || receipt.BatchId != mutation.Receipt.BatchId ||
		receipt.TargetDigest != mutation.Receipt.TargetDigest || receipt.BodyDigest != mutation.Receipt.BodyDigest ||
		receipt.SchemaVersion != mutation.Receipt.SchemaVersion || receipt.RetrySafe != mutation.Receipt.RetrySafe ||
		receipt.ReconciliationId != mutation.Receipt.ReconciliationId ||
		!reflect.DeepEqual(receipt.RecipientSnapshot, mutation.Receipt.RecipientSnapshot) || !reflect.DeepEqual(receipt.OutboxIds, mutation.Receipt.OutboxIds) {
		return errors.New("broadcast read-back receipt mismatch")
	}
	if len(mutation.Events) != len(receipt.OutboxIds) {
		return errors.New("broadcast read-back event count mismatch")
	}
	for _, event := range mutation.Events {
		stored, err := GetOutboxEvent(ctx, event.ID)
		if err != nil {
			return fmt.Errorf("broadcast read-back event: %w", err)
		}
		if stored.Kind != "broadcast" || stored.AggregateID != event.AggregateID || stored.IdempotencyKey != event.IdempotencyKey || !reflect.DeepEqual(stored.Payload, event.Payload) {
			return errors.New("broadcast read-back event mismatch")
		}
	}
	return nil
}

func FindBroadcastReceipt(ctx context.Context, actorID domain.ObjectID, batchID string) (info.BroadcastReceipt, error) {
	if BroadcastReceipts == nil {
		return info.BroadcastReceipt{}, ErrMongoClientNotInitialized
	}
	var receipt info.BroadcastReceipt
	err := BroadcastReceipts.FindContext(ctx, map[string]any{"ActorId": actorID, "BatchId": batchID, "Kind": "broadcast"}).One(&receipt)
	return receipt, err
}

// MarkBroadcastReceiptForOutbox derives the aggregate batch state from all
// recipient events. A single sent/retry event must never make a partially
// processed batch look terminal.
func MarkBroadcastReceiptForOutbox(ctx context.Context, eventID domain.ObjectID, _ string, category string, now time.Time) error {
	if BroadcastReceipts == nil || Outbox == nil {
		return ErrMongoClientNotInitialized
	}
	if eventID.IsZero() {
		return errors.New("broadcast receipt transport identity is incomplete")
	}
	var receipt info.BroadcastReceipt
	if err := BroadcastReceipts.FindContext(ctx, bson.M{"Kind": "broadcast", "OutboxIds": eventID.Hex()}).One(&receipt); err != nil {
		return err
	}
	ids := make([]domain.ObjectID, 0, len(receipt.OutboxIds))
	for _, raw := range receipt.OutboxIds {
		id, err := domain.ParseObjectID(raw)
		if err != nil || id.IsZero() {
			return errors.New("broadcast receipt contains an invalid outbox identity")
		}
		ids = append(ids, id)
	}
	var events []OutboxEvent
	if err := Outbox.FindContext(ctx, bson.M{"_id": bson.M{"$in": ids}, "Kind": "broadcast"}).All(&events); err != nil {
		return err
	}
	if len(events) != len(ids) {
		return errors.New("broadcast receipt outbox read-back is incomplete")
	}
	state, terminal := aggregateBroadcastState(events)
	set := map[string]any{"State": state}
	set["ErrorCategory"] = category
	if terminal {
		set["TerminalAt"] = now
		set["ExpiresAt"] = now.Add(30 * 24 * time.Hour)
	}
	set["Version"] = receipt.Version + 1
	versionFilter := any(receipt.Version)
	if receipt.Version == 0 {
		// Receipts created before the CAS field was introduced have no Version.
		// Treat the missing field as the initial version exactly once.
		versionFilter = bson.M{"$in": []any{int64(0), 0, nil}}
	}
	if err := BroadcastReceipts.UpdateOneMatchedContext(ctx, bson.M{"_id": receipt.ReceiptId, "Kind": "broadcast", "OutboxIds": eventID.Hex(), "Version": versionFilter}, bson.M{"$set": set}); err != nil {
		return err
	}
	var readBack info.BroadcastReceipt
	if err := BroadcastReceipts.FindIdContext(ctx, receipt.ReceiptId).One(&readBack); err != nil {
		return err
	}
	if readBack.Version != receipt.Version+1 || readBack.State != state || (terminal && (readBack.TerminalAt.IsZero() || readBack.ExpiresAt.IsZero())) {
		return errors.New("broadcast receipt transport read-back mismatch")
	}
	return nil
}

func aggregateBroadcastState(events []OutboxEvent) (string, bool) {
	if len(events) == 0 {
		return "enqueued", false
	}
	allTerminal := true
	allSent := true
	allCancelled := true
	state := "enqueued"
	for _, event := range events {
		switch event.Status {
		case OutboxStatusSent:
			allCancelled = false
		case OutboxStatusCancelled:
			allSent = false
		default:
			allSent = false
			allCancelled = false
		}
		if event.Status != OutboxStatusSent && event.Status != OutboxStatusCancelled && event.Status != OutboxStatusDead {
			allTerminal = false
		}
		switch event.Status {
		case OutboxStatusHandoffUnknown:
			state = OutboxStatusHandoffUnknown
		case OutboxStatusDead:
			if state != OutboxStatusHandoffUnknown {
				state = OutboxStatusDead
			}
		case OutboxStatusRetry, OutboxStatusSending:
			if state != OutboxStatusHandoffUnknown && state != OutboxStatusDead {
				state = OutboxStatusRetry
			}
		case OutboxStatusPending:
			if state == "enqueued" {
				state = OutboxStatusPending
			}
		}
	}
	if allSent {
		return OutboxStatusSent, true
	}
	if allCancelled {
		return OutboxStatusCancelled, true
	}
	return state, allTerminal
}

// CollectExpiredBroadcastReceipts removes only terminal receipts after the
// retention window and verifies every deletion by read-back.
func CollectExpiredBroadcastReceipts(ctx context.Context, now time.Time) (int, error) {
	if BroadcastReceipts == nil {
		return 0, ErrMongoClientNotInitialized
	}
	var receipts []info.BroadcastReceipt
	if err := BroadcastReceipts.FindContext(ctx, bson.M{"ExpiresAt": bson.M{"$lte": now}, "TerminalAt": bson.M{"$exists": true}}).All(&receipts); err != nil {
		return 0, err
	}
	removed := 0
	for _, receipt := range receipts {
		if err := BroadcastReceipts.RemoveOneMatchedContext(ctx, bson.M{"_id": receipt.ReceiptId, "ExpiresAt": bson.M{"$lte": now}}); err != nil {
			return removed, err
		}
		var verify info.BroadcastReceipt
		if err := BroadcastReceipts.FindIdContext(ctx, receipt.ReceiptId).One(&verify); err == nil {
			return removed, fmt.Errorf("broadcast receipt GC read-back found %s", receipt.ReceiptId.Hex())
		}
		removed++
	}
	return removed, nil
}
