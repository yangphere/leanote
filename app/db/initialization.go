package db

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yangphere/leanote/app/domain"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type InitializationStep struct {
	Name       string
	Apply      func(context.Context) error
	Compensate func(context.Context) error
}

type UserInitializationPlan struct {
	UserID        domain.ObjectID
	Steps         []InitializationStep
	Outbox        *OutboxEvent
	EnqueueOutbox func(context.Context, OutboxEvent) error
}

type UserInitializationResult struct {
	Mode         string
	Committed    bool
	Compensated  bool
	PartialWrite bool
	AppliedSteps []string
	FailedStep   string
}

// TransactionRunner lets tests and callers provide a transaction boundary
// without importing the Mongo driver into service code.
type TransactionRunner func(context.Context, func(context.Context) error) error

// RunUserInitialization executes user creation, required initialization and
// outbox enqueue in one transaction when Mongo supports it. Standalone Mongo
// deployments use an explicit idempotent compensation path and never report
// partially applied work as a normal commit.
func RunUserInitialization(ctx context.Context, plan UserInitializationPlan) (UserInitializationResult, error) {
	if client == nil {
		return ExecuteUserInitialization(ctx, plan, nil)
	}
	session, err := client.StartSession()
	if err != nil {
		// A live client that cannot start a session may be disconnected or
		// otherwise unhealthy. That is not evidence that the deployment lacks
		// transactions, so replaying writes outside a transaction is unsafe.
		return UserInitializationResult{}, fmt.Errorf("start initialization transaction: %w", err)
	}
	defer session.EndSession(context.Background())
	runner := func(parent context.Context, apply func(context.Context) error) error {
		_, err := session.WithTransaction(parent, func(txCtx context.Context) (any, error) {
			return nil, apply(txCtx)
		})
		return err
	}
	return ExecuteUserInitialization(ctx, plan, runner)
}

func ExecuteUserInitialization(ctx context.Context, plan UserInitializationPlan, transaction TransactionRunner) (UserInitializationResult, error) {
	result := UserInitializationResult{}
	if ctx == nil {
		ctx = context.Background()
	}
	if plan.UserID.IsZero() {
		return result, fmt.Errorf("initialize user: invalid user id")
	}
	for _, step := range plan.Steps {
		if step.Name == "" || step.Apply == nil {
			return result, fmt.Errorf("initialize user: invalid step")
		}
	}

	applyAll := func(applyCtx context.Context) error {
		// mongo.WithTransaction may invoke the callback more than once after a
		// transient transaction error.  Attempt bookkeeping describes only the
		// current callback invocation; retaining entries from a prior attempt
		// would make a successful commit look like duplicate application.
		result.AppliedSteps = nil
		result.FailedStep = ""
		for _, step := range plan.Steps {
			if err := step.Apply(applyCtx); err != nil {
				result.FailedStep = step.Name
				return fmt.Errorf("initialize user step %s: %w", step.Name, err)
			}
			result.AppliedSteps = append(result.AppliedSteps, step.Name)
		}
		if plan.Outbox != nil {
			enqueue := plan.EnqueueOutbox
			if enqueue == nil {
				enqueue = EnqueueOutbox
			}
			if err := enqueue(applyCtx, *plan.Outbox); err != nil {
				result.FailedStep = "outbox"
				return fmt.Errorf("%w: enqueue outbox: %v", ErrSideEffect, err)
			}
			result.AppliedSteps = append(result.AppliedSteps, "outbox")
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
		// The transaction was rejected by the server before a durable commit;
		// reset per-attempt bookkeeping before the compensation path.
		result.Mode = "compensation"
		result.AppliedSteps = nil
		result.FailedStep = ""
	}
	return executeCompensation(ctx, plan, applyAll, &result)
}

func executeCompensation(ctx context.Context, plan UserInitializationPlan, applyAll func(context.Context) error, result *UserInitializationResult) (UserInitializationResult, error) {
	result.Mode = "compensation"
	result.AppliedSteps = nil
	if err := applyAll(ctx); err == nil {
		result.Compensated = true
		return *result, nil
	} else {
		result.PartialWrite = true
		applied := append([]string(nil), result.AppliedSteps...)
		for i := len(applied) - 1; i >= 0; i-- {
			for _, step := range plan.Steps {
				if step.Name != applied[i] || step.Compensate == nil {
					continue
				}
				if compensationErr := step.Compensate(ctx); compensationErr != nil {
					return *result, fmt.Errorf("%w: compensate %s: %v; original: %w", ErrPartialWrite, step.Name, compensationErr, err)
				}
				break
			}
		}
		return *result, &PersistenceError{Code: ErrPartialWrite.Error(), Cause: err}
	}
}

func transactionUnsupported(err error) bool {
	if err == nil {
		return false
	}
	var commandErr mongo.CommandError
	if errors.As(err, &commandErr) {
		// 251 (NoSuchTransaction) and 263 (operation not supported in
		// transaction) can be transient/transaction-state failures.  They do
		// not prove that the deployment lacks transaction capability and must
		// never trigger a non-transactional replay.  Standalone Mongo reports
		// capability as code 20 with the explicit replica-set wording.
		if commandErr.Code != 20 {
			return false
		}
		message := strings.ToLower(commandErr.Message + " " + commandErr.Name)
		return strings.Contains(message, "transaction") &&
			(strings.Contains(message, "replica set") ||
				strings.Contains(message, "replica-set") ||
				strings.Contains(message, "mongos") ||
				strings.Contains(message, "not supported"))
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "transaction numbers are only allowed") ||
		strings.Contains(message, "transactions are not supported") ||
		strings.Contains(message, "replica set") && strings.Contains(message, "transaction")
}
