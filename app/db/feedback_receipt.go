package db

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var ErrFeedbackMutationConflict = errors.New("feedback mutation identity conflict")

// FeedbackMutation is the durable three-document boundary for a suggestion.
// The caller must supply deterministic IDs for retry-safe submissions.
type FeedbackMutation struct {
	Suggestion info.Suggestion
	Receipt    info.FeedbackReceipt
	Outbox     OutboxEvent
}

// CommitFeedbackMutation writes suggestion, receipt and outbox in one Mongo
// transaction when available. Standalone deployments use an explicit
// compensation path and return a partial-write error if cleanup is uncertain.
func CommitFeedbackMutation(ctx context.Context, mutation FeedbackMutation) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if Suggestions == nil || FeedbackReceipts == nil || Outbox == nil {
		return fmt.Errorf("feedback mutation: %w", ErrMongoClientNotInitialized)
	}
	if mutation.Suggestion.Id.IsZero() || mutation.Receipt.ReceiptId.IsZero() || mutation.Outbox.AggregateID.IsZero() {
		return errors.New("feedback mutation: stable identities are required")
	}
	apply := func(applyCtx context.Context, track bool) (bool, bool, error) {
		insertedSuggestion, err := ensureFeedbackSuggestion(applyCtx, mutation.Suggestion)
		if err != nil {
			return false, false, fmt.Errorf("persist suggestion: %w", err)
		}
		insertedReceipt, err := ensureFeedbackReceipt(applyCtx, mutation.Receipt)
		if err != nil {
			return insertedSuggestion, false, fmt.Errorf("persist feedback receipt: %w", err)
		}
		outbox, err := EnqueueOutboxEvent(applyCtx, mutation.Outbox)
		if err != nil {
			return insertedSuggestion, insertedReceipt, fmt.Errorf("enqueue feedback outbox: %w", err)
		}
		mutation.Outbox = outbox
		if err := strictFeedbackReadBack(applyCtx, mutation); err != nil {
			return insertedSuggestion, insertedReceipt, err
		}
		_ = track
		return insertedSuggestion, insertedReceipt, nil
	}

	if client != nil {
		session, err := client.StartSession()
		if err != nil {
			return fmt.Errorf("feedback mutation session: %w", err)
		}
		defer session.EndSession(context.Background())
		_, txErr := session.WithTransaction(ctx, func(txCtx context.Context) (any, error) {
			_, _, err := apply(txCtx, false)
			return nil, err
		})
		if txErr == nil {
			return strictFeedbackReadBack(context.Background(), mutation)
		}
		if !transactionUnsupported(txErr) {
			return txErr
		}
	}

	// The explicit fallback is only used when the server rejects transactions.
	// Compensation keeps an acknowledged suggestion from being mistaken for a
	// complete feedback submission.
	insertedSuggestion, insertedReceipt, err := apply(ctx, true)
	if err == nil {
		return strictFeedbackReadBack(context.Background(), mutation)
	}
	// A response can be lost after all three writes commit. Reconcile before
	// compensating; never delete a durable retry result just because the caller
	// observed an error.
	if readErr := strictFeedbackReadBack(context.Background(), mutation); readErr == nil {
		return nil
	}
	cleanupCtx := context.Background()
	if insertedSuggestion {
		if cleanupErr := Suggestions.RemoveOneMatchedContext(cleanupCtx, map[string]any{"_id": mutation.Suggestion.Id}); cleanupErr != nil {
			return fmt.Errorf("%w: %v; suggestion compensation: %v", ErrPartialWrite, err, cleanupErr)
		}
	}
	if insertedReceipt {
		if cleanupErr := FeedbackReceipts.RemoveOneMatchedContext(cleanupCtx, map[string]any{"_id": mutation.Receipt.ReceiptId}); cleanupErr != nil {
			return fmt.Errorf("%w: %v; receipt compensation: %v", ErrPartialWrite, err, cleanupErr)
		}
	}
	// The outbox may have committed even if its enqueue acknowledgement was
	// lost. It is intentionally left for reconciliation unless read-back has
	// proven it absent; deleting it would risk dropping a send.
	return fmt.Errorf("%w: %v", ErrPartialWrite, err)
}

func ensureFeedbackSuggestion(ctx context.Context, want info.Suggestion) (bool, error) {
	var got info.Suggestion
	err := Suggestions.FindIdContext(ctx, want.Id).One(&got)
	if err == nil {
		if got.UserId != want.UserId || got.Addr != want.Addr || got.Suggestion != want.Suggestion {
			return false, ErrFeedbackMutationConflict
		}
		return false, nil
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return false, err
	}
	if err := Suggestions.InsertContext(ctx, want); err != nil {
		return false, err
	}
	return true, nil
}

func ensureFeedbackReceipt(ctx context.Context, want info.FeedbackReceipt) (bool, error) {
	var got info.FeedbackReceipt
	err := FeedbackReceipts.FindIdContext(ctx, want.ReceiptId).One(&got)
	if err == nil {
		if got.Kind != want.Kind || got.ActorId != want.ActorId || got.SubmissionId != want.SubmissionId ||
			got.TargetDigest != want.TargetDigest || got.BodyDigest != want.BodyDigest || got.SuggestionId != want.SuggestionId {
			return false, ErrFeedbackMutationConflict
		}
		if got.SchemaVersion != want.SchemaVersion || got.RetrySafe != want.RetrySafe || (want.Version != 0 && got.Version != want.Version) ||
			got.ReconciliationId != want.ReconciliationId ||
			!reflect.DeepEqual(got.RecipientSnapshot, want.RecipientSnapshot) || !reflect.DeepEqual(got.OutboxIds, want.OutboxIds) {
			return false, ErrFeedbackMutationConflict
		}
		return false, nil
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return false, err
	}
	if err := FeedbackReceipts.InsertContext(ctx, want); err != nil {
		return false, err
	}
	return true, nil
}

func strictFeedbackReadBack(ctx context.Context, mutation FeedbackMutation) error {
	var suggestion info.Suggestion
	if err := Suggestions.FindIdContext(ctx, mutation.Suggestion.Id).One(&suggestion); err != nil {
		return fmt.Errorf("feedback read-back suggestion: %w", err)
	}
	if suggestion.UserId != mutation.Suggestion.UserId || suggestion.Addr != mutation.Suggestion.Addr || suggestion.Suggestion != mutation.Suggestion.Suggestion {
		return ErrFeedbackMutationConflict
	}
	var receipt info.FeedbackReceipt
	if err := FeedbackReceipts.FindIdContext(ctx, mutation.Receipt.ReceiptId).One(&receipt); err != nil {
		return fmt.Errorf("feedback read-back receipt: %w", err)
	}
	var event OutboxEvent
	if err := Outbox.FindIdContext(ctx, mutation.Outbox.ID).One(&event); err != nil {
		return fmt.Errorf("feedback read-back outbox: %w", err)
	}
	if event.Kind != "feedback" || event.AggregateID != mutation.Suggestion.Id {
		return errors.New("feedback read-back outbox identity mismatch")
	}
	if receipt.Kind != "feedback" || receipt.ActorId != mutation.Receipt.ActorId || receipt.SubmissionId != mutation.Receipt.SubmissionId ||
		receipt.TargetDigest != mutation.Receipt.TargetDigest || receipt.BodyDigest != mutation.Receipt.BodyDigest || receipt.SuggestionId != mutation.Suggestion.Id ||
		receipt.RetrySafe != mutation.Receipt.RetrySafe || receipt.SchemaVersion != mutation.Receipt.SchemaVersion ||
		receipt.ReconciliationId != mutation.Receipt.ReconciliationId || (mutation.Receipt.Version != 0 && receipt.Version != mutation.Receipt.Version) ||
		!reflect.DeepEqual(receipt.RecipientSnapshot, mutation.Receipt.RecipientSnapshot) ||
		receipt.OutboxIds == nil || len(receipt.OutboxIds) != 1 || receipt.OutboxIds[0] != mutation.Outbox.ID.Hex() {
		return errors.New("feedback read-back receipt is incomplete")
	}
	if event.IdempotencyKey != mutation.Outbox.IdempotencyKey || !reflect.DeepEqual(event.Payload, mutation.Outbox.Payload) {
		return ErrFeedbackMutationConflict
	}
	return nil
}

// FindFeedbackReceipt returns a retry-safe receipt by its unique identity.
func FindFeedbackReceipt(ctx context.Context, actorID domain.ObjectID, submissionID string) (info.FeedbackReceipt, error) {
	if FeedbackReceipts == nil {
		return info.FeedbackReceipt{}, ErrMongoClientNotInitialized
	}
	var receipt info.FeedbackReceipt
	err := FeedbackReceipts.FindContext(ctx, map[string]any{
		"ActorId": actorID, "SubmissionId": submissionID, "Kind": "feedback", "RetrySafe": true,
	}).One(&receipt)
	return receipt, err
}

// CollectExpiredFeedbackReceipts removes only terminal receipts whose expiry
// is confirmed by a read-back. Each deletion is bounded independently.
func CollectExpiredFeedbackReceipts(ctx context.Context, now time.Time) (int, error) {
	if FeedbackReceipts == nil {
		return 0, ErrMongoClientNotInitialized
	}
	var receipts []info.FeedbackReceipt
	if err := FeedbackReceipts.FindContext(ctx, map[string]any{
		"ExpiresAt":  map[string]any{"$lte": now},
		"TerminalAt": map[string]any{"$exists": true},
	}).All(&receipts); err != nil {
		return 0, err
	}
	removed := 0
	for _, receipt := range receipts {
		if err := FeedbackReceipts.RemoveOneMatchedContext(ctx, map[string]any{"_id": receipt.ReceiptId, "ExpiresAt": map[string]any{"$lte": now}}); err != nil {
			return removed, err
		}
		var verify info.FeedbackReceipt
		if err := FeedbackReceipts.FindIdContext(ctx, receipt.ReceiptId).One(&verify); err == nil {
			return removed, fmt.Errorf("feedback receipt GC read-back found %s", receipt.ReceiptId.Hex())
		}
		removed++
	}
	return removed, nil
}

// MarkFeedbackReceiptForOutbox mirrors the durable transport state into the
// receipt. It is intentionally keyed by the frozen outbox ID, never by body
// text or a time window.
func MarkFeedbackReceiptForOutbox(ctx context.Context, eventID domain.ObjectID, state, category string, now time.Time) error {
	if FeedbackReceipts == nil {
		return ErrMongoClientNotInitialized
	}
	if eventID.IsZero() || state == "" {
		return errors.New("feedback receipt transport identity is incomplete")
	}
	var receipt info.FeedbackReceipt
	if err := FeedbackReceipts.FindContext(ctx, map[string]any{"Kind": "feedback", "OutboxIds": eventID.Hex()}).One(&receipt); err != nil {
		return err
	}
	set := map[string]any{"State": state, "Version": receipt.Version + 1}
	if category == "" {
		set["ErrorCategory"] = ""
	} else {
		set["ErrorCategory"] = category
	}
	if state == "sent" || state == "dead" || state == "cancelled" {
		set["TerminalAt"] = now
		set["ExpiresAt"] = now.Add(30 * 24 * time.Hour)
	}
	if err := FeedbackReceipts.UpdateOneMatchedContext(ctx, map[string]any{
		"_id": receipt.ReceiptId, "Kind": "feedback", "OutboxIds": eventID.Hex(), "Version": receipt.Version,
	}, map[string]any{"$set": set}); err != nil {
		return err
	}
	var readBack info.FeedbackReceipt
	if err := FeedbackReceipts.FindIdContext(ctx, receipt.ReceiptId).One(&readBack); err != nil {
		return err
	}
	if readBack.Version != receipt.Version+1 || readBack.State != state || (state == "sent" && readBack.TerminalAt.IsZero()) {
		return errors.New("feedback receipt transport read-back mismatch")
	}
	return nil
}
