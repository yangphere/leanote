package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const (
	// Historical policy: campaigns larger than 1000 recipients must be split.
	BroadcastMaxRecipients   = 1000
	BroadcastMaxSubjectBytes = 998
	BroadcastMaxBodyBytes    = 1 << 20
)

var broadcastBatchIDPattern = submissionIDPattern

var (
	ErrBroadcastValidation = errors.New("invalid broadcast")
	ErrBroadcastConflict   = errors.New("broadcast batch conflict")
)

// BroadcastSubmission is the stable acknowledgement returned after the
// receipt and all recipient events have been durably read back.
type BroadcastSubmission struct {
	BatchID          string
	ReconciliationID string
	RetrySafe        bool
	OutboxIDs        []domain.ObjectID
}

type BroadcastPlan struct {
	Receipt info.BroadcastReceipt
	Events  []db.OutboxEvent
}

func newReconciliationID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("generate reconciliation id: %w", err)
	}
	return hex.EncodeToString(bytes[:]), nil
}

func normalizeBroadcastRecipients(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("%w: recipient count must be between 1 and %d", ErrBroadcastValidation, BroadcastMaxRecipients)
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		// Check the untrimmed value too: TrimSpace would otherwise erase a
		// trailing CR/LF and turn a header-injection attempt into a valid address.
		if strings.ContainsAny(raw, "\r\n") || strings.ContainsAny(value, "\r\n") {
			return nil, fmt.Errorf("%w: recipient contains a header-injection value", ErrBroadcastValidation)
		}
		parsed, err := mail.ParseAddress(value)
		if err != nil || parsed.Address != value {
			return nil, fmt.Errorf("%w: recipient must be a single addr-spec", ErrBroadcastValidation)
		}
		identity := strings.ToLower(value)
		if _, ok := seen[identity]; ok {
			continue
		}
		seen[identity] = struct{}{}
		result = append(result, value)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("%w: recipient list is empty", ErrBroadcastValidation)
	}
	if len(result) > BroadcastMaxRecipients {
		return nil, fmt.Errorf("%w: recipient count must be between 1 and %d", ErrBroadcastValidation, BroadcastMaxRecipients)
	}
	sort.SliceStable(result, func(i, j int) bool { return strings.ToLower(result[i]) < strings.ToLower(result[j]) })
	return result, nil
}

func validateBroadcastText(subject, body string) error {
	if !utf8.ValidString(subject) || !utf8.ValidString(body) {
		return fmt.Errorf("%w: subject/body must be valid UTF-8", ErrBroadcastValidation)
	}
	if strings.TrimSpace(subject) == "" || strings.ContainsAny(subject, "\r\n") || len(subject) > BroadcastMaxSubjectBytes {
		return fmt.Errorf("%w: subject is invalid", ErrBroadcastValidation)
	}
	if strings.TrimSpace(body) == "" || len(body) > BroadcastMaxBodyBytes || strings.IndexByte(body, 0) >= 0 {
		return fmt.Errorf("%w: body is invalid", ErrBroadcastValidation)
	}
	return nil
}

func digestBroadcastRecipients(values []string) string {
	normalized := append([]string(nil), values...)
	sort.Slice(normalized, func(i, j int) bool { return strings.ToLower(normalized[i]) < strings.ToLower(normalized[j]) })
	hash := sha256.New()
	for _, value := range normalized {
		_, _ = hash.Write([]byte(strings.ToLower(value)))
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func digestBroadcastBody(subject, body string) string {
	digest := sha256.Sum256([]byte(subject + "\x00" + body))
	return hex.EncodeToString(digest[:])
}

// BuildBroadcastPlan performs all validation and freezes recipient/event
// identities without touching Mongo. Tests and adapters can use it to prove
// that invalid requests have no persistence side effect.
func BuildBroadcastPlan(actorID domain.ObjectID, batchID string, recipients []string, subject, body string) (BroadcastSubmission, BroadcastPlan, error) {
	if actorID.IsZero() {
		return BroadcastSubmission{}, BroadcastPlan{}, fmt.Errorf("%w: authenticated admin actor is required", ErrBroadcastValidation)
	}
	retrySafe := batchID != ""
	if batchID == "" {
		var err error
		batchID, err = newReconciliationID()
		if err != nil {
			return BroadcastSubmission{}, BroadcastPlan{}, err
		}
	} else if !broadcastBatchIDPattern.MatchString(batchID) {
		return BroadcastSubmission{}, BroadcastPlan{}, fmt.Errorf("%w: batchId must be 32 lowercase hexadecimal characters", ErrBroadcastValidation)
	}
	list, err := normalizeBroadcastRecipients(recipients)
	if err != nil {
		return BroadcastSubmission{}, BroadcastPlan{}, err
	}
	if err := validateBroadcastText(subject, body); err != nil {
		return BroadcastSubmission{}, BroadcastPlan{}, err
	}
	reconciliationID := batchID
	receiptID := db.OutboxEventIDForKey("broadcast-receipt:" + actorID.Hex() + ":" + batchID)
	receipt := info.BroadcastReceipt{
		ReceiptId: receiptID, SchemaVersion: 1, Kind: "broadcast", ActorId: actorID, BatchId: batchID,
		State: "enqueued", TargetDigest: digestBroadcastRecipients(list), BodyDigest: digestBroadcastBody(subject, body),
		RetrySafe: retrySafe, ReconciliationId: reconciliationID, RecipientSnapshot: append([]string(nil), list...), CreatedAt: time.Now().UTC(),
	}
	events := make([]db.OutboxEvent, 0, len(list))
	outboxIDs := make([]domain.ObjectID, 0, len(list))
	for _, recipient := range list {
		key := "broadcast:" + batchID + ":" + strings.ToLower(recipient)
		event := db.OutboxEvent{ID: db.OutboxEventIDForKey(key), IdempotencyKey: key, Kind: "broadcast", AggregateID: receiptID,
			Payload: map[string]any{"batchId": batchID, "recipientId": strings.ToLower(recipient), "email": recipient, "subject": subject, "body": body}}
		events = append(events, event)
		outboxIDs = append(outboxIDs, event.ID)
	}
	for _, id := range outboxIDs {
		receipt.OutboxIds = append(receipt.OutboxIds, id.Hex())
	}
	return BroadcastSubmission{BatchID: batchID, ReconciliationID: reconciliationID, RetrySafe: retrySafe, OutboxIDs: outboxIDs}, BroadcastPlan{Receipt: receipt, Events: events}, nil
}

// EnqueueBroadcast is the admin mail boundary. It never invokes SMTP and
// never launches a request goroutine.
func (this *EmailService) EnqueueBroadcast(ctx context.Context, actorID domain.ObjectID, batchID string, recipients []string, subject, body string) (BroadcastSubmission, error) {
	submission, plan, err := BuildBroadcastPlan(actorID, batchID, recipients, subject, body)
	if err != nil {
		return BroadcastSubmission{}, err
	}
	if db.BroadcastReceipts == nil || db.Outbox == nil {
		return submission, fmt.Errorf("%w: persistence is not initialized", ErrBroadcastValidation)
	}
	// A client supplied batch is replay-safe. Lookup errors other than a clean
	// miss must remain visible instead of creating a second batch.
	if submission.RetrySafe {
		existing, lookupErr := db.FindBroadcastReceipt(ctx, actorID, submission.BatchID)
		if lookupErr == nil {
			if existing.TargetDigest != plan.Receipt.TargetDigest || existing.BodyDigest != plan.Receipt.BodyDigest || !sameStrings(existing.RecipientSnapshot, plan.Receipt.RecipientSnapshot) {
				return BroadcastSubmission{}, ErrBroadcastConflict
			}
			return broadcastSubmissionFromReceipt(existing), nil
		}
		if !errors.Is(lookupErr, mongo.ErrNoDocuments) {
			return BroadcastSubmission{}, fmt.Errorf("%w: lookup receipt: %v", ErrBroadcastValidation, lookupErr)
		}
	}
	if err := db.CommitBroadcastMutation(ctx, db.BroadcastMutation{Receipt: plan.Receipt, Events: plan.Events}); err != nil {
		if submission.RetrySafe {
			if existing, lookupErr := db.FindBroadcastReceipt(ctx, actorID, submission.BatchID); lookupErr == nil {
				if existing.TargetDigest == plan.Receipt.TargetDigest && existing.BodyDigest == plan.Receipt.BodyDigest && sameStrings(existing.RecipientSnapshot, plan.Receipt.RecipientSnapshot) {
					return broadcastSubmissionFromReceipt(existing), nil
				}
				return BroadcastSubmission{}, ErrBroadcastConflict
			} else if !errors.Is(lookupErr, mongo.ErrNoDocuments) {
				return BroadcastSubmission{}, fmt.Errorf("%w: reconcile receipt: %v", ErrBroadcastValidation, lookupErr)
			}
		}
		return submission, err
	}
	return submission, nil
}

func broadcastSubmissionFromReceipt(receipt info.BroadcastReceipt) BroadcastSubmission {
	result := BroadcastSubmission{BatchID: receipt.BatchId, ReconciliationID: receipt.ReconciliationId, RetrySafe: receipt.RetrySafe}
	for _, raw := range receipt.OutboxIds {
		if id, err := domain.ParseObjectID(raw); err == nil {
			result.OutboxIDs = append(result.OutboxIDs, id)
		}
	}
	return result
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
