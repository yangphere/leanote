package db

import (
	"context"
	"fmt"
	"time"

	applicationnotes "github.com/yangphere/leanote/app/application/notes"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// WorkspaceMutationStep is one required write in a notes-workspace command.
// ReplaySafe is explicit: an unknown standalone result is retried only when
// the provider is idempotent, otherwise Verify must prove the final state.
type WorkspaceMutationStep struct {
	Name               string
	Apply              func(context.Context) error
	Verify             func(context.Context) (bool, error)
	Compensate         func(context.Context) error
	ReplaySafe         bool
	AssignedUSN        func() int
	RestoreAssignedUSN func(int)
}

type WorkspaceMutationPlan struct {
	OperationID         string
	OwnerID             domain.ObjectID
	ResourceID          domain.ObjectID
	Kind                string
	InputDigest         string
	Assets              []applicationnotes.OperationAsset
	BeforeState         []byte
	DesiredState        []byte
	Steps               []WorkspaceMutationStep
	AssignedUSN         func() int
	RestoreAssignedUSN  func(int)
	RestoreBeforeState  func([]byte) error
	RestoreDesiredState func([]byte) error
	CaptureResultState  func() []byte
	RestoreResultState  func([]byte) error
	FailurePolicy       applicationnotes.FailurePolicy
}

type WorkspaceMutationResult struct {
	Mode         string
	Committed    bool
	PartialWrite bool
	AppliedSteps []string
	FailedStep   string
	Operation    applicationnotes.OperationReceipt
}

type MongoWorkspaceRepository struct{}

func (MongoWorkspaceRepository) AllocateUserUSN(ctx context.Context, ownerID domain.ObjectID) (int, error) {
	return AllocateUserUSN(ctx, ownerID)
}

type MongoWorkspaceUnitOfWork struct{}

func (MongoWorkspaceUnitOfWork) Run(ctx context.Context, plan applicationnotes.MutationPlan) (applicationnotes.MutationResult, error) {
	legacyPlan := WorkspaceMutationPlan{
		OperationID: plan.OperationID, OwnerID: plan.OwnerID, ResourceID: plan.ResourceID,
		Kind: plan.Kind, InputDigest: plan.InputDigest, Assets: append([]applicationnotes.OperationAsset(nil), plan.Assets...),
		BeforeState: plan.BeforeState, DesiredState: plan.DesiredState,
		AssignedUSN: plan.AssignedUSN, RestoreAssignedUSN: plan.RestoreAssignedUSN,
		RestoreBeforeState: plan.RestoreBeforeState, RestoreDesiredState: plan.RestoreDesiredState,
		CaptureResultState: plan.CaptureResultState, RestoreResultState: plan.RestoreResultState,
		FailurePolicy: plan.FailurePolicy,
	}
	for _, step := range plan.Steps {
		legacyPlan.Steps = append(legacyPlan.Steps, WorkspaceMutationStep{
			Name: step.Name, Apply: step.Apply, Verify: step.Verify, Compensate: step.Compensate, ReplaySafe: step.ReplaySafe,
			AssignedUSN: step.AssignedUSN, RestoreAssignedUSN: step.RestoreAssignedUSN,
		})
	}
	result, err := RunWorkspaceMutation(ctx, legacyPlan)
	return applicationnotes.MutationResult{
		Mode: result.Mode, Committed: result.Committed, PartialWrite: result.PartialWrite,
		AppliedSteps: result.AppliedSteps, FailedStep: result.FailedStep,
		Operation: result.Operation,
	}, err
}

func (MongoWorkspaceUnitOfWork) GetOperation(ctx context.Context, ownerID domain.ObjectID, operationID string) (applicationnotes.OperationReceipt, error) {
	return GetWorkspaceOperation(ctx, ownerID, operationID)
}

var _ applicationnotes.WorkspaceRepository = MongoWorkspaceRepository{}
var _ applicationnotes.WorkspaceUnitOfWork = MongoWorkspaceUnitOfWork{}

// AllocateUserUSN atomically increments and returns a user's workspace USN.
// It is the only allocator for note, notebook and tag mutations.
func AllocateUserUSN(ctx context.Context, ownerID domain.ObjectID) (int, error) {
	if ownerID.IsZero() {
		return 0, fmt.Errorf("allocate workspace usn: invalid owner id")
	}
	if Users == nil {
		return 0, ErrMongoClientNotInitialized
	}
	if ctx == nil {
		ctx = context.Background()
	}
	opCtx, cancel := boundedOperationContext(ctx)
	defer cancel()
	var user info.User
	err := Users.coll.FindOneAndUpdate(
		opCtx,
		bson.M{"_id": ownerID},
		bson.M{"$inc": bson.M{"Usn": 1}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&user)
	if err != nil {
		Users.logFailure("allocate workspace usn", err)
		return 0, fmt.Errorf("allocate workspace usn: %w", err)
	}
	return user.Usn, nil
}

// RunWorkspaceMutation uses a Mongo transaction when available and the
// explicit compensation path only when the server proves that transactions
// are unsupported. Other transaction errors are returned without replaying
// writes outside the transaction.
func RunWorkspaceMutation(ctx context.Context, plan WorkspaceMutationPlan) (WorkspaceMutationResult, error) {
	if err := validateWorkspaceMutationPlan(plan); err != nil {
		return WorkspaceMutationResult{}, err
	}
	if plan.OperationID == "" || plan.Kind == "" || plan.InputDigest == "" {
		return WorkspaceMutationResult{}, fmt.Errorf("workspace mutation: incomplete durable identity")
	}
	if client == nil {
		return WorkspaceMutationResult{}, ErrMongoClientNotInitialized
	}
	session, err := client.StartSession()
	if err != nil {
		return WorkspaceMutationResult{}, fmt.Errorf("start workspace transaction: %w", err)
	}
	defer session.EndSession(context.Background())
	durablePlan := applicationMutationPlan(plan)
	transactionResult := applicationnotes.MutationResult{}
	_, err = session.WithTransaction(ctx, func(txCtx context.Context) (any, error) {
		var callbackErr error
		transactionResult, callbackErr = applicationnotes.ExecuteStandalone(txCtx, durablePlan, MongoWorkspaceOperationStore{}, time.Now())
		return nil, callbackErr
	})
	if err == nil {
		return WorkspaceMutationResult{
			Mode: "transaction", Committed: transactionResult.Committed, PartialWrite: transactionResult.PartialWrite,
			AppliedSteps: transactionResult.AppliedSteps, FailedStep: transactionResult.FailedStep,
			Operation: transactionResult.Operation,
		}, nil
	}
	if !transactionUnsupported(err) {
		return WorkspaceMutationResult{
			Mode: "transaction", PartialWrite: false,
			FailedStep: transactionResult.FailedStep,
		}, err
	}

	durable, durableErr := applicationnotes.ExecuteStandalone(ctx, durablePlan, MongoWorkspaceOperationStore{}, time.Now())
	return WorkspaceMutationResult{
		Mode: durable.Mode, Committed: durable.Committed, PartialWrite: durable.PartialWrite,
		AppliedSteps: durable.AppliedSteps, FailedStep: durable.FailedStep,
		Operation: durable.Operation,
	}, durableErr
}

func GetWorkspaceOperation(ctx context.Context, ownerID domain.ObjectID, operationID string) (applicationnotes.OperationReceipt, error) {
	return (MongoWorkspaceOperationStore{}).Get(ctx, ownerID, operationID)
}

// RunWorkspaceRepair persists progress for idempotent projections that run
// after the required workspace mutation has committed.
func RunWorkspaceRepair(ctx context.Context, plan WorkspaceMutationPlan) (WorkspaceMutationResult, error) {
	if err := validateWorkspaceMutationPlan(plan); err != nil {
		return WorkspaceMutationResult{}, err
	}
	durable, err := applicationnotes.ExecuteStandalone(ctx, applicationMutationPlan(plan), MongoWorkspaceOperationStore{}, time.Now())
	return WorkspaceMutationResult{
		Mode: durable.Mode, Committed: durable.Committed, PartialWrite: durable.PartialWrite,
		AppliedSteps: durable.AppliedSteps, FailedStep: durable.FailedStep,
		Operation: durable.Operation,
	}, err
}

func applicationMutationPlan(plan WorkspaceMutationPlan) applicationnotes.MutationPlan {
	durablePlan := applicationnotes.MutationPlan{
		OperationID: plan.OperationID, OwnerID: plan.OwnerID, ResourceID: plan.ResourceID,
		Kind: plan.Kind, InputDigest: plan.InputDigest, Assets: append([]applicationnotes.OperationAsset(nil), plan.Assets...),
		BeforeState: plan.BeforeState, DesiredState: plan.DesiredState,
		AssignedUSN: plan.AssignedUSN, RestoreAssignedUSN: plan.RestoreAssignedUSN,
		RestoreBeforeState:  plan.RestoreBeforeState,
		RestoreDesiredState: plan.RestoreDesiredState,
		CaptureResultState:  plan.CaptureResultState, RestoreResultState: plan.RestoreResultState,
		FailurePolicy: plan.FailurePolicy,
	}
	for _, step := range plan.Steps {
		durablePlan.Steps = append(durablePlan.Steps, applicationnotes.MutationStep{
			Name: step.Name, Apply: step.Apply, Verify: step.Verify, Compensate: step.Compensate, ReplaySafe: step.ReplaySafe,
			AssignedUSN: step.AssignedUSN, RestoreAssignedUSN: step.RestoreAssignedUSN,
		})
	}
	return durablePlan
}

// ExecuteWorkspaceMutation is exported so focused service tests can inject a
// transaction runner without importing the Mongo driver.
func ExecuteWorkspaceMutation(ctx context.Context, plan WorkspaceMutationPlan, transaction TransactionRunner) (WorkspaceMutationResult, error) {
	result := WorkspaceMutationResult{}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := validateWorkspaceMutationPlan(plan); err != nil {
		return result, err
	}

	applyAll := func(applyCtx context.Context) error {
		result.AppliedSteps = nil
		result.FailedStep = ""
		for _, step := range plan.Steps {
			if err := step.Apply(applyCtx); err != nil {
				result.FailedStep = step.Name
				return fmt.Errorf("workspace mutation step %s: %w", step.Name, err)
			}
			result.AppliedSteps = append(result.AppliedSteps, step.Name)
		}
		return nil
	}

	if transaction != nil {
		result.Mode = "transaction"
		if err := transaction(ctx, applyAll); err == nil {
			result.Committed = true
			return result, nil
		} else if !transactionUnsupported(err) {
			return result, err
		}
		result.AppliedSteps = nil
		result.FailedStep = ""
	}

	result.Mode = "compensation"
	if err := applyAll(ctx); err != nil {
		result.PartialWrite = len(result.AppliedSteps) > 0
		original := err
		if !result.PartialWrite {
			return result, original
		}
		for i := len(result.AppliedSteps) - 1; i >= 0; i-- {
			name := result.AppliedSteps[i]
			for _, step := range plan.Steps {
				if step.Name != name || step.Compensate == nil {
					continue
				}
				if compensateErr := step.Compensate(ctx); compensateErr != nil {
					return result, fmt.Errorf("%w: compensate %s: %v; original: %w", ErrPartialWrite, name, compensateErr, original)
				}
				break
			}
		}
		return result, &PersistenceError{Code: ErrPartialWrite.Error(), Cause: original}
	}
	result.Committed = true
	return result, nil
}

func validateWorkspaceMutationPlan(plan WorkspaceMutationPlan) error {
	if plan.OwnerID.IsZero() {
		return fmt.Errorf("workspace mutation: invalid owner id")
	}
	if len(plan.Steps) == 0 {
		return fmt.Errorf("workspace mutation: no steps")
	}
	seen := make(map[string]struct{}, len(plan.Steps))
	for _, step := range plan.Steps {
		if step.Name == "" || step.Apply == nil {
			return fmt.Errorf("workspace mutation: invalid step")
		}
		if _, exists := seen[step.Name]; exists {
			return fmt.Errorf("workspace mutation: duplicate step %s", step.Name)
		}
		seen[step.Name] = struct{}{}
	}
	return nil
}
