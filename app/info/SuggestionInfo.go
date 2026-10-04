package info

import (
	"time"

	"github.com/yangphere/leanote/app/domain"
)

// 建议
type Suggestion struct {
	Id         domain.ObjectID `bson:"_id"`
	UserId     domain.ObjectID `bson:"UserId"`
	Addr       string          `bson:"Addr"`
	Suggestion string          `bson:"Suggestion"`
}

// FeedbackReceipt is the durable retry identity for authenticated feedback.
// Anonymous legacy submissions intentionally omit SubmissionId and are not
// covered by the unique partial index.
type FeedbackReceipt struct {
	ReceiptId         domain.ObjectID `bson:"_id,omitempty"`
	SuggestionId      domain.ObjectID `bson:"SuggestionId,omitempty"`
	SchemaVersion     int32           `bson:"SchemaVersion"`
	Kind              string          `bson:"Kind"`
	ActorId           domain.ObjectID `bson:"ActorId"`
	SubmissionId      string          `bson:"SubmissionId,omitempty"`
	State             string          `bson:"State"`
	TargetDigest      string          `bson:"TargetDigest"`
	BodyDigest        string          `bson:"BodyDigest"`
	ErrorCategory     string          `bson:"ErrorCategory,omitempty"`
	ReconciliationId  string          `bson:"ReconciliationId,omitempty"`
	RetrySafe         bool            `bson:"RetrySafe"`
	RecipientSnapshot []string        `bson:"RecipientSnapshot"`
	OutboxIds         []string        `bson:"OutboxIds"`
	Version           int64           `bson:"Version"`
	CreatedAt         time.Time       `bson:"CreatedAt"`
	TerminalAt        time.Time       `bson:"TerminalAt,omitempty"`
	ExpiresAt         time.Time       `bson:"ExpiresAt,omitempty"`
}

// BroadcastReceipt freezes one administrator mail batch and its recipient
// event identities. It is deliberately separate from feedback receipts so a
// broadcast can never collide with a feedback submission.
type BroadcastReceipt struct {
	ReceiptId         domain.ObjectID `bson:"_id,omitempty"`
	SchemaVersion     int32           `bson:"SchemaVersion"`
	Kind              string          `bson:"Kind"`
	ActorId           domain.ObjectID `bson:"ActorId"`
	BatchId           string          `bson:"BatchId"`
	State             string          `bson:"State"`
	TargetDigest      string          `bson:"TargetDigest"`
	BodyDigest        string          `bson:"BodyDigest"`
	ErrorCategory     string          `bson:"ErrorCategory,omitempty"`
	RetrySafe         bool            `bson:"RetrySafe"`
	ReconciliationId  string          `bson:"ReconciliationId,omitempty"`
	RecipientSnapshot []string        `bson:"RecipientSnapshot"`
	OutboxIds         []string        `bson:"OutboxIds"`
	CreatedAt         time.Time       `bson:"CreatedAt"`
	TerminalAt        time.Time       `bson:"TerminalAt,omitempty"`
	ExpiresAt         time.Time       `bson:"ExpiresAt,omitempty"`
	Version           int64           `bson:"Version"`
}

// UpgradeCheckpoint is a lease/fencing record for one deterministic data
// migration step. Version is incremented on every CAS transition.
type UpgradeCheckpoint struct {
	CheckpointId  domain.ObjectID `bson:"_id,omitempty"`
	OperationId   string          `bson:"OperationId"`
	StepKey       string          `bson:"StepKey"`
	InputDigest   string          `bson:"InputDigest"`
	TargetScope   string          `bson:"TargetScope"`
	State         string          `bson:"State"`
	Version       int64           `bson:"Version"`
	LeaseId       string          `bson:"LeaseId,omitempty"`
	Fence         int64           `bson:"Fence"`
	ResultDigest  string          `bson:"ResultDigest,omitempty"`
	ErrorCategory string          `bson:"ErrorCategory,omitempty"`
	CreatedAt     time.Time       `bson:"CreatedAt"`
	UpdatedAt     time.Time       `bson:"UpdatedAt"`
	LeaseUntil    time.Time       `bson:"LeaseUntil,omitempty"`
	CompletedAt   time.Time       `bson:"CompletedAt,omitempty"`
}
