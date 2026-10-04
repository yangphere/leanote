package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type SuggestionService struct {
}

var submissionIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

var (
	ErrSuggestionValidation = errors.New("invalid suggestion")
	ErrSuggestionConflict   = errors.New("suggestion submission conflict")
	ErrSuggestionSideEffect = errors.New("suggestion side effect failed")
)

// SuggestionSubmission captures the durable result returned to the HTTP
// adapter. RetrySafe is false for the legacy anonymous request shape.
type SuggestionSubmission struct {
	SuggestionID     domain.ObjectID
	OutboxID         domain.ObjectID
	RetrySafe        bool
	ReconciliationID string
}

func validateSuggestionInput(suggestion info.Suggestion, submissionID string) error {
	if !utf8.ValidString(suggestion.Suggestion) {
		return fmt.Errorf("%w: suggestion must be valid UTF-8", ErrSuggestionValidation)
	}
	trimmed := strings.TrimSpace(suggestion.Suggestion)
	if trimmed == "" {
		return fmt.Errorf("%w: suggestion is blank", ErrSuggestionValidation)
	}
	// Trim is used only to reject an all-whitespace submission. The stored
	// value is authoritative for both limits so leading/trailing whitespace
	// cannot bypass the rune budget.
	if utf8.RuneCountInString(suggestion.Suggestion) > 2000 || len(suggestion.Suggestion) > 8192 {
		return fmt.Errorf("%w: suggestion exceeds size limit", ErrSuggestionValidation)
	}
	for _, r := range suggestion.Suggestion {
		// Newline, carriage return and tab are valid feedback formatting;
		// reject the remaining C0 controls and DEL.
		if (r < 0x20 && r != '\n' && r != '\r' && r != '\t') || r == 0x7f {
			return fmt.Errorf("%w: suggestion contains control characters", ErrSuggestionValidation)
		}
	}
	if suggestion.Addr != "" {
		if len(suggestion.Addr) > 254 || strings.ContainsAny(suggestion.Addr, "\r\n") {
			return fmt.Errorf("%w: invalid contact address", ErrSuggestionValidation)
		}
		for _, r := range suggestion.Addr {
			if r < 0x20 || r == 0x7f {
				return fmt.Errorf("%w: invalid contact address", ErrSuggestionValidation)
			}
		}
		parsed, err := mail.ParseAddress(suggestion.Addr)
		if err != nil || parsed.Address != suggestion.Addr {
			return fmt.Errorf("%w: contact address must be a single addr-spec", ErrSuggestionValidation)
		}
	}
	if submissionID != "" && !submissionIDPattern.MatchString(submissionID) {
		return fmt.Errorf("%w: submissionId must be 32 lowercase hexadecimal characters", ErrSuggestionValidation)
	}
	if suggestion.UserId.IsZero() && submissionID != "" {
		return fmt.Errorf("%w: anonymous submissions cannot declare submissionId", ErrSuggestionValidation)
	}
	return nil
}

// AddSuggestionDurable persists the feedback document and a durable outbox
// event synchronously. The request path never starts a goroutine. A missing
// recipient configuration fails closed before any document is written.
func (s *SuggestionService) AddSuggestionDurable(ctx context.Context, suggestion info.Suggestion, submissionID string) (SuggestionSubmission, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := validateSuggestionInput(suggestion, submissionID); err != nil {
		return SuggestionSubmission{}, err
	}
	if db.Suggestions == nil || db.Outbox == nil {
		return SuggestionSubmission{}, fmt.Errorf("%w: persistence is not initialized", ErrSuggestionSideEffect)
	}
	recipients, err := normalizeFeedbackRecipients()
	if err != nil {
		return SuggestionSubmission{}, err
	}
	actor := suggestion.UserId.Hex()
	identity := actor + ":" + submissionID
	if suggestion.Id.IsZero() && submissionID != "" {
		suggestion.Id = db.OutboxEventIDForKey("feedback-suggestion:" + identity)
	}
	if submissionID != "" {
		existing, err := db.FindFeedbackReceipt(ctx, suggestion.UserId, submissionID)
		if err == nil {
			if existing.TargetDigest != digestText(suggestion.Addr) || existing.BodyDigest != digestText(suggestion.Suggestion) {
				return SuggestionSubmission{}, ErrSuggestionConflict
			}
			return readFeedbackResult(ctx, existing, suggestion)
		}
		if !errors.Is(err, mongo.ErrNoDocuments) {
			return SuggestionSubmission{}, fmt.Errorf("%w: lookup receipt: %v", ErrSuggestionSideEffect, err)
		}
	}
	if suggestion.Id.IsZero() {
		if submissionID == "" {
			suggestion.Id = db.NewObjectID()
		}
	}
	key := "feedback:" + suggestion.Id.Hex()
	if submissionID != "" {
		key = "feedback:" + identity
	}
	now := time.Now().UTC()
	reconciliationID := suggestion.Id.Hex()
	receipt := info.FeedbackReceipt{
		ReceiptId: suggestionReceiptID(identity, submissionID), SchemaVersion: 1, Kind: "feedback",
		SuggestionId: suggestion.Id,
		ActorId:      suggestion.UserId, SubmissionId: submissionID, State: "enqueued",
		TargetDigest: digestText(suggestion.Addr), BodyDigest: digestText(suggestion.Suggestion),
		RetrySafe: submissionID != "", RecipientSnapshot: append([]string(nil), recipients...),
		ReconciliationId: reconciliationID, Version: 1, CreatedAt: now,
	}
	event := db.OutboxEvent{IdempotencyKey: key, Kind: "feedback", AggregateID: suggestion.Id,
		Payload: map[string]any{"recipients": recipients, "subject": "建议", "body": suggestion.Suggestion, "addr": suggestion.Addr}}
	// The event ID is deterministic for retry-safe submissions and is recorded
	// in the receipt before the three-document mutation starts.
	event.ID = db.OutboxEventIDForKey(key)
	receipt.OutboxIds = []string{event.ID.Hex()}
	mutation := db.FeedbackMutation{Suggestion: suggestion, Receipt: receipt, Outbox: event}
	if err := db.CommitFeedbackMutation(ctx, mutation); err != nil {
		if submissionID != "" {
			if existing, lookupErr := db.FindFeedbackReceipt(ctx, suggestion.UserId, submissionID); lookupErr == nil {
				if existing.TargetDigest == receipt.TargetDigest && existing.BodyDigest == receipt.BodyDigest {
					return readFeedbackResult(ctx, existing, suggestion)
				}
				return SuggestionSubmission{}, ErrSuggestionConflict
			} else if !errors.Is(lookupErr, mongo.ErrNoDocuments) {
				return SuggestionSubmission{}, fmt.Errorf("%w: reconcile receipt: %v", ErrSuggestionSideEffect, lookupErr)
			}
		}
		return SuggestionSubmission{SuggestionID: suggestion.Id, ReconciliationID: reconciliationID}, fmt.Errorf("%w: %v", ErrSuggestionSideEffect, err)
	}
	return SuggestionSubmission{SuggestionID: suggestion.Id, OutboxID: event.ID, RetrySafe: submissionID != "", ReconciliationID: reconciliationID}, nil
}

func digestText(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func suggestionReceiptID(identity, submissionID string) domain.ObjectID {
	if submissionID == "" {
		return db.NewObjectID()
	}
	return db.OutboxEventIDForKey("feedback-receipt:" + identity)
}

func readFeedbackResult(ctx context.Context, receipt info.FeedbackReceipt, suggestion info.Suggestion) (SuggestionSubmission, error) {
	if len(receipt.OutboxIds) == 0 {
		return SuggestionSubmission{}, fmt.Errorf("%w: receipt has no outbox identity", ErrSuggestionSideEffect)
	}
	outboxID, err := domain.ParseObjectID(receipt.OutboxIds[0])
	if err != nil {
		return SuggestionSubmission{}, fmt.Errorf("%w: receipt outbox identity is invalid", ErrSuggestionSideEffect)
	}
	suggestionID := receipt.SuggestionId
	if suggestionID.IsZero() {
		// Receipts created before SuggestionId was added can still be reconciled
		// through their outbox identity.  Do not infer an ID from mutable input.
		event, eventErr := db.GetOutboxEvent(ctx, outboxID)
		if eventErr != nil {
			return SuggestionSubmission{}, fmt.Errorf("%w: read feedback outbox: %v", ErrSuggestionSideEffect, eventErr)
		}
		suggestionID = event.AggregateID
	}
	var existing info.Suggestion
	if err := db.Suggestions.FindIdContext(ctx, suggestionID).One(&existing); err != nil {
		return SuggestionSubmission{}, fmt.Errorf("%w: read suggestion: %v", ErrSuggestionSideEffect, err)
	} else if existing.Suggestion != suggestion.Suggestion || existing.Addr != suggestion.Addr || existing.UserId != suggestion.UserId {
		return SuggestionSubmission{}, ErrSuggestionConflict
	}
	return SuggestionSubmission{SuggestionID: suggestionID, OutboxID: outboxID, RetrySafe: receipt.RetrySafe, ReconciliationID: receipt.ReconciliationId}, nil
}

// normalizeFeedbackRecipients validates the complete configured list. A
// malformed entry is an operator/configuration error; silently dropping it
// would create a durable feedback record with an incomplete audience.
func normalizeFeedbackRecipients() ([]string, error) {
	if configService == nil {
		return nil, fmt.Errorf("%w: feedbackRecipients is not configured", ErrSuggestionValidation)
	}
	return ValidateFeedbackRecipients(configService.GetGlobalArrayConfig("feedbackRecipients"))
}

// ValidateFeedbackRecipients is also used by configuration adapters so an
// invalid recipient can never be persisted and later silently dropped.
func ValidateFeedbackRecipients(values []string) ([]string, error) {
	if len(values) == 0 || len(values) > 20 {
		return nil, fmt.Errorf("%w: feedbackRecipients must contain 1..20 addresses", ErrSuggestionValidation)
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || strings.ContainsAny(value, "\r\n") {
			return nil, fmt.Errorf("%w: feedbackRecipients contains an empty or header-injection value", ErrSuggestionValidation)
		}
		parsed, err := mail.ParseAddress(value)
		if err != nil || parsed.Address != value {
			return nil, fmt.Errorf("%w: feedbackRecipients contains an invalid addr-spec", ErrSuggestionValidation)
		}
		identity := strings.ToLower(value)
		if _, ok := seen[identity]; ok {
			return nil, fmt.Errorf("%w: feedbackRecipients contains duplicate address", ErrSuggestionValidation)
		}
		seen[identity] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

// 得到某博客具体信息
func (this *SuggestionService) AddSuggestion(suggestion info.Suggestion) bool {
	if err := validateSuggestionInput(suggestion, ""); err != nil {
		return false
	}
	if suggestion.Id.IsZero() {
		suggestion.Id = db.NewObjectID()
	}
	return db.Insert(db.Suggestions, suggestion)
}
