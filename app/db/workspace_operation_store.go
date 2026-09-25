package db

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	applicationnotes "github.com/yangphere/leanote/app/application/notes"
	"github.com/yangphere/leanote/app/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type workspaceOperationDocument struct {
	OperationID  string                            `bson:"_id"`
	OwnerID      domain.ObjectID                   `bson:"OwnerId"`
	ResourceID   domain.ObjectID                   `bson:"ResourceId,omitempty"`
	Kind         string                            `bson:"Kind"`
	InputDigest  string                            `bson:"InputDigest"`
	Assets       []applicationnotes.OperationAsset `bson:"Assets,omitempty"`
	StepNames    []string                          `bson:"StepNames,omitempty"`
	StepUSNs     map[string]int                    `bson:"StepUsns,omitempty"`
	BeforeState  []byte                            `bson:"BeforeState,omitempty"`
	DesiredState []byte                            `bson:"DesiredState,omitempty"`
	ResultState  []byte                            `bson:"ResultState,omitempty"`
	AssignedUSN  int                               `bson:"AssignedUsn,omitempty"`
	AppliedSteps []string                          `bson:"AppliedSteps,omitempty"`
	CurrentStep  string                            `bson:"CurrentStep,omitempty"`
	FailedStep   string                            `bson:"FailedStep,omitempty"`
	Status       applicationnotes.OperationStatus  `bson:"Status"`
	LastError    string                            `bson:"LastError,omitempty"`
	LeaseID      string                            `bson:"LeaseId,omitempty"`
	LeaseUntil   time.Time                         `bson:"LeaseUntil,omitempty"`
	Version      int64                             `bson:"Version"`
	CreatedAt    time.Time                         `bson:"CreatedAt"`
	UpdatedAt    time.Time                         `bson:"UpdatedAt"`
	TerminalAt   time.Time                         `bson:"TerminalAt,omitempty"`
}

type MongoWorkspaceOperationStore struct{}

// BeginWorkspaceOperation records a pending operation before an external
// asset write starts. Pending receipts are never terminal-TTL candidates.
func BeginWorkspaceOperation(ctx context.Context, receipt applicationnotes.OperationReceipt) (applicationnotes.OperationReceipt, error) {
	return (MongoWorkspaceOperationStore{}).Begin(ctx, receipt)
}

// FailWorkspaceOperation closes a pre-side-effect receipt when the request
// cannot proceed. It deliberately stores only a stable step label: an upload
// error may contain user data or a filesystem path, and a failed pre-upload
// receipt must not retain either the recovery payload or that detail.
func FailWorkspaceOperation(ctx context.Context, ownerID domain.ObjectID, operationID, failedStep string) error {
	store := MongoWorkspaceOperationStore{}
	receipt, err := store.Claim(ctx, ownerID, operationID, time.Now(), applicationnotes.OperationLeaseTTL)
	if err != nil {
		return err
	}
	if applicationnotes.IsTerminalOperation(receipt.Status) {
		return nil
	}
	receipt.Status = applicationnotes.OperationFailed
	receipt.FailedStep = failedStep
	receipt.LastError = "operation_failed"
	_, err = store.Save(ctx, receipt)
	return err
}

func (MongoWorkspaceOperationStore) Begin(ctx context.Context, receipt applicationnotes.OperationReceipt) (applicationnotes.OperationReceipt, error) {
	if WorkspaceOperations == nil {
		return applicationnotes.OperationReceipt{}, ErrMongoClientNotInitialized
	}
	document := operationDocument(receipt)
	document.Version = 1
	if err := WorkspaceOperations.InsertContext(ctx, document); err == nil {
		return operationReceipt(document), nil
	} else if !mongo.IsDuplicateKeyError(err) {
		return applicationnotes.OperationReceipt{}, fmt.Errorf("begin workspace operation: %w", err)
	}
	existing, err := (MongoWorkspaceOperationStore{}).Get(ctx, receipt.OwnerID, receipt.OperationID)
	if err != nil {
		return applicationnotes.OperationReceipt{}, fmt.Errorf("read existing workspace operation: %w", err)
	}
	if existing.OwnerID != receipt.OwnerID || existing.ResourceID != receipt.ResourceID || existing.Kind != receipt.Kind || existing.InputDigest != receipt.InputDigest {
		return applicationnotes.OperationReceipt{}, applicationnotes.ErrOperationConflict
	}
	return existing, nil
}

func (MongoWorkspaceOperationStore) Claim(ctx context.Context, ownerID domain.ObjectID, operationID string, now time.Time, ttl time.Duration) (applicationnotes.OperationReceipt, error) {
	if WorkspaceOperations == nil {
		return applicationnotes.OperationReceipt{}, ErrMongoClientNotInitialized
	}
	leaseID, err := newWorkspaceLeaseID()
	if err != nil {
		return applicationnotes.OperationReceipt{}, err
	}
	filter := bson.M{
		"_id":     operationID,
		"OwnerId": ownerID,
		"Status":  bson.M{"$nin": terminalWorkspaceOperationStatuses()},
		"$or": []bson.M{
			{"LeaseId": bson.M{"$exists": false}},
			{"LeaseId": ""},
			{"LeaseUntil": bson.M{"$lte": now}},
		},
	}
	var document workspaceOperationDocument
	opCtx, cancel := boundedOperationContext(ctx)
	defer cancel()
	err = WorkspaceOperations.coll.FindOneAndUpdate(
		opCtx, filter,
		bson.M{"$set": bson.M{"Status": applicationnotes.OperationRunning, "LeaseId": leaseID, "LeaseUntil": now.Add(ttl), "UpdatedAt": now}, "$inc": bson.M{"Version": 1}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&document)
	if err == nil {
		return operationReceipt(document), nil
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return applicationnotes.OperationReceipt{}, fmt.Errorf("claim workspace operation: %w", err)
	}
	existing, getErr := (MongoWorkspaceOperationStore{}).Get(ctx, ownerID, operationID)
	if getErr != nil {
		return applicationnotes.OperationReceipt{}, getErr
	}
	if applicationnotes.IsTerminalOperation(existing.Status) {
		return existing, nil
	}
	return applicationnotes.OperationReceipt{}, applicationnotes.ErrOperationBusy
}

func (MongoWorkspaceOperationStore) Save(ctx context.Context, receipt applicationnotes.OperationReceipt) (applicationnotes.OperationReceipt, error) {
	if WorkspaceOperations == nil {
		return applicationnotes.OperationReceipt{}, ErrMongoClientNotInitialized
	}
	if receipt.LeaseID == "" {
		return applicationnotes.OperationReceipt{}, applicationnotes.ErrOperationCAS
	}
	now := time.Now()
	applicationnotes.RedactTerminalReceipt(&receipt)
	if isTerminalWorkspaceOperation(receipt.Status) {
		if receipt.TerminalAt.IsZero() {
			receipt.TerminalAt = now
		}
	} else {
		receipt.TerminalAt = time.Time{}
	}
	set := bson.M{
		"AssignedUsn": receipt.AssignedUSN, "AppliedSteps": receipt.AppliedSteps,
		"StepUsns":    receipt.StepUSNs,
		"CurrentStep": receipt.CurrentStep, "FailedStep": receipt.FailedStep,
		"Status": receipt.Status, "LastError": receipt.LastError, "UpdatedAt": now,
		"BeforeState": receipt.BeforeState, "DesiredState": receipt.DesiredState,
		"ResultState": receipt.ResultState,
		"TerminalAt":  receipt.TerminalAt,
		"LeaseUntil":  now.Add(applicationnotes.OperationLeaseTTL),
	}
	if receipt.Status == applicationnotes.OperationCommitted || receipt.Status == applicationnotes.OperationCompensated || receipt.Status == applicationnotes.OperationFailed || receipt.Status == applicationnotes.OperationRepairPending {
		set["LeaseId"] = ""
		set["LeaseUntil"] = time.Time{}
	}
	filter := bson.M{
		"_id": receipt.OperationID, "OwnerId": receipt.OwnerID, "LeaseId": receipt.LeaseID, "Version": receipt.Version,
		"Status": bson.M{"$nin": terminalWorkspaceOperationStatuses()},
	}
	var document workspaceOperationDocument
	opCtx, cancel := boundedOperationContext(ctx)
	defer cancel()
	err := WorkspaceOperations.coll.FindOneAndUpdate(
		opCtx, filter, bson.M{"$set": set, "$inc": bson.M{"Version": 1}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&document)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return applicationnotes.OperationReceipt{}, applicationnotes.ErrOperationCAS
	}
	if err != nil {
		return applicationnotes.OperationReceipt{}, fmt.Errorf("save workspace operation: %w", err)
	}
	return operationReceipt(document), nil
}

// DeleteTerminal removes only completed receipts for one owner/resource.
// Pending and repair-pending receipts are intentionally never TTL eligible;
// they are the durable recovery queue.
func (MongoWorkspaceOperationStore) DeleteTerminal(ctx context.Context, ownerID, resourceID domain.ObjectID) error {
	if WorkspaceOperations == nil {
		return ErrMongoClientNotInitialized
	}
	_, err := WorkspaceOperations.RemoveAllContext(ctx, bson.M{
		"OwnerId": ownerID, "ResourceId": resourceID,
		"Status": bson.M{"$in": []applicationnotes.OperationStatus{
			applicationnotes.OperationCommitted, applicationnotes.OperationCompensated, applicationnotes.OperationFailed,
		}},
	})
	return err
}

func DeleteTerminalWorkspaceOperations(ctx context.Context, ownerID, resourceID domain.ObjectID) error {
	return (MongoWorkspaceOperationStore{}).DeleteTerminal(ctx, ownerID, resourceID)
}

// DeleteWorkspaceOperationsForResource removes every receipt for a resource
// after its permanent deletion has completed. Pending receipts cannot be
// replayed once the resource is gone and may still contain recovery payloads.
func (MongoWorkspaceOperationStore) DeleteForResource(ctx context.Context, ownerID, resourceID domain.ObjectID) error {
	if WorkspaceOperations == nil {
		return ErrMongoClientNotInitialized
	}
	_, err := WorkspaceOperations.RemoveAllContext(ctx, bson.M{"OwnerId": ownerID, "ResourceId": resourceID})
	return err
}

func DeleteWorkspaceOperationsForResource(ctx context.Context, ownerID, resourceID domain.ObjectID) error {
	return (MongoWorkspaceOperationStore{}).DeleteForResource(ctx, ownerID, resourceID)
}

// QuarantineWorkspaceOperationsForResource closes every operation for a
// resource except the cleanup operation itself. Permanent deletion makes all
// other recovery plans invalid; they must not remain claimable or retain a
// replay payload while cleanup is waiting on a retry.
func (MongoWorkspaceOperationStore) QuarantineForResource(ctx context.Context, ownerID, resourceID domain.ObjectID, exceptOperationID string) error {
	if WorkspaceOperations == nil {
		return ErrMongoClientNotInitialized
	}
	filter := bson.M{"OwnerId": ownerID, "ResourceId": resourceID}
	if exceptOperationID != "" {
		filter["_id"] = bson.M{"$ne": exceptOperationID}
	}
	now := time.Now()
	if _, err := WorkspaceOperations.UpdateAllContext(ctx, filter, bson.M{
		"$unset": bson.M{
			"Assets": "", "BeforeState": "", "DesiredState": "", "ResultState": "", "StepUsns": "",
			"CurrentStep": "", "LeaseId": "", "LeaseUntil": "",
		},
		"$set": bson.M{"LastError": "resource_deleted", "TerminalAt": now, "UpdatedAt": now},
	}); err != nil {
		return fmt.Errorf("quarantine workspace operations: %w", err)
	}
	closeFilter := bson.M{
		"OwnerId": ownerID, "ResourceId": resourceID,
		"Status": bson.M{"$nin": terminalWorkspaceOperationStatuses()},
	}
	if exceptOperationID != "" {
		closeFilter["_id"] = bson.M{"$ne": exceptOperationID}
	}
	if _, err := WorkspaceOperations.UpdateAllContext(ctx, closeFilter, bson.M{
		"$set": bson.M{
			"Status": applicationnotes.OperationFailed, "FailedStep": "resource_deleted",
			"TerminalAt": now, "UpdatedAt": now,
		},
	}); err != nil {
		return fmt.Errorf("close quarantined workspace operations: %w", err)
	}
	return nil
}

func QuarantineWorkspaceOperationsForResource(ctx context.Context, ownerID, resourceID domain.ObjectID, exceptOperationID string) error {
	return (MongoWorkspaceOperationStore{}).QuarantineForResource(ctx, ownerID, resourceID, exceptOperationID)
}

func (MongoWorkspaceOperationStore) Get(ctx context.Context, ownerID domain.ObjectID, operationID string) (applicationnotes.OperationReceipt, error) {
	if WorkspaceOperations == nil {
		return applicationnotes.OperationReceipt{}, ErrMongoClientNotInitialized
	}
	var document workspaceOperationDocument
	err := WorkspaceOperations.FindContext(ctx, bson.M{"_id": operationID, "OwnerId": ownerID}).One(&document)
	if err != nil {
		return applicationnotes.OperationReceipt{}, err
	}
	return operationReceipt(document), nil
}

func UnfinishedWorkspaceOperation(ctx context.Context, ownerID, resourceID domain.ObjectID, kind string) (applicationnotes.OperationReceipt, bool, error) {
	if WorkspaceOperations == nil {
		return applicationnotes.OperationReceipt{}, false, ErrMongoClientNotInitialized
	}
	filter := bson.M{"OwnerId": ownerID, "Kind": kind, "Status": bson.M{"$nin": terminalWorkspaceOperationStatuses()}}
	if !resourceID.IsZero() {
		filter["ResourceId"] = resourceID
	}
	var documents []workspaceOperationDocument
	if err := WorkspaceOperations.FindContext(ctx, filter).Limit(2).All(&documents); err != nil {
		return applicationnotes.OperationReceipt{}, false, fmt.Errorf("find unfinished workspace operation: %w", err)
	}
	if len(documents) > 1 {
		return applicationnotes.OperationReceipt{}, false, fmt.Errorf("multiple unfinished %s operations for owner %s", kind, ownerID.Hex())
	}
	if len(documents) == 0 {
		return applicationnotes.OperationReceipt{}, false, nil
	}
	return operationReceipt(documents[0]), true, nil
}

func LatestCommittedWorkspaceOperation(ctx context.Context, ownerID, resourceID domain.ObjectID, kind string, assignedUSN int) (applicationnotes.OperationReceipt, bool, error) {
	if WorkspaceOperations == nil {
		return applicationnotes.OperationReceipt{}, false, ErrMongoClientNotInitialized
	}
	filter := bson.M{"OwnerId": ownerID, "Kind": kind, "Status": applicationnotes.OperationCommitted}
	if !resourceID.IsZero() {
		filter["ResourceId"] = resourceID
	}
	if assignedUSN > 0 {
		filter["AssignedUsn"] = assignedUSN
	}
	var document workspaceOperationDocument
	err := WorkspaceOperations.FindContext(ctx, filter).Sort("-CreatedAt", "-_id").One(&document)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return applicationnotes.OperationReceipt{}, false, nil
	}
	if err != nil {
		return applicationnotes.OperationReceipt{}, false, fmt.Errorf("find committed workspace operation: %w", err)
	}
	return operationReceipt(document), true, nil
}

func newWorkspaceLeaseID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("create workspace operation lease: %w", err)
	}
	return hex.EncodeToString(value), nil
}

func operationDocument(receipt applicationnotes.OperationReceipt) workspaceOperationDocument {
	return workspaceOperationDocument{
		OperationID: receipt.OperationID, OwnerID: receipt.OwnerID, ResourceID: receipt.ResourceID,
		Kind: receipt.Kind, InputDigest: receipt.InputDigest, Assets: append([]applicationnotes.OperationAsset(nil), receipt.Assets...),
		StepNames:   append([]string(nil), receipt.StepNames...),
		StepUSNs:    cloneWorkspaceStepUSNs(receipt.StepUSNs),
		BeforeState: append([]byte(nil), receipt.BeforeState...), DesiredState: append([]byte(nil), receipt.DesiredState...),
		ResultState: append([]byte(nil), receipt.ResultState...),
		AssignedUSN: receipt.AssignedUSN, AppliedSteps: append([]string(nil), receipt.AppliedSteps...),
		CurrentStep: receipt.CurrentStep, FailedStep: receipt.FailedStep, Status: receipt.Status,
		LastError: receipt.LastError, LeaseID: receipt.LeaseID, LeaseUntil: receipt.LeaseUntil,
		Version: receipt.Version, CreatedAt: receipt.CreatedAt, UpdatedAt: receipt.UpdatedAt,
		TerminalAt: receipt.TerminalAt,
	}
}

func isTerminalWorkspaceOperation(status applicationnotes.OperationStatus) bool {
	return applicationnotes.IsTerminalOperation(status)
}

func terminalWorkspaceOperationStatuses() []applicationnotes.OperationStatus {
	return []applicationnotes.OperationStatus{
		applicationnotes.OperationCommitted,
		applicationnotes.OperationCompensated,
		applicationnotes.OperationFailed,
	}
}

func operationReceipt(document workspaceOperationDocument) applicationnotes.OperationReceipt {
	return applicationnotes.OperationReceipt{
		OperationID: document.OperationID, OwnerID: document.OwnerID, ResourceID: document.ResourceID,
		Kind: document.Kind, InputDigest: document.InputDigest, Assets: append([]applicationnotes.OperationAsset(nil), document.Assets...),
		StepNames:   append([]string(nil), document.StepNames...),
		StepUSNs:    cloneWorkspaceStepUSNs(document.StepUSNs),
		BeforeState: append([]byte(nil), document.BeforeState...), DesiredState: append([]byte(nil), document.DesiredState...),
		ResultState: append([]byte(nil), document.ResultState...),
		AssignedUSN: document.AssignedUSN, AppliedSteps: append([]string(nil), document.AppliedSteps...),
		CurrentStep: document.CurrentStep, FailedStep: document.FailedStep, Status: document.Status,
		LastError: document.LastError, LeaseID: document.LeaseID, LeaseUntil: document.LeaseUntil,
		Version: document.Version, CreatedAt: document.CreatedAt, UpdatedAt: document.UpdatedAt,
		TerminalAt: document.TerminalAt,
	}
}

func cloneWorkspaceStepUSNs(source map[string]int) map[string]int {
	if len(source) == 0 {
		return nil
	}
	cloned := make(map[string]int, len(source))
	for name, usn := range source {
		cloned[name] = usn
	}
	return cloned
}
