package notes

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"
)

const OperationLeaseTTL = 30 * time.Second

// ExecuteStandalone resumes or starts one durable standalone mutation. Every
// state transition is fenced by the receipt store's lease and version CAS.
func ExecuteStandalone(ctx context.Context, plan MutationPlan, store OperationStore, now time.Time) (MutationResult, error) {
	result := MutationResult{Mode: "compensation"}
	if err := validatePlan(plan, store); err != nil {
		return result, err
	}
	receipt, err := store.Begin(ctx, OperationReceipt{
		OperationID: plan.OperationID, OwnerID: plan.OwnerID, ResourceID: plan.ResourceID,
		Kind: plan.Kind, InputDigest: plan.InputDigest, Assets: append([]OperationAsset(nil), plan.Assets...),
		StepNames:   operationStepNames(plan),
		StepUSNs:    cloneStepUSNs(nil),
		BeforeState: append([]byte(nil), plan.BeforeState...), DesiredState: append([]byte(nil), plan.DesiredState...),
		Status: OperationPending, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return result, err
	}
	if receipt.Status == OperationCommitted {
		if plan.RestoreResultState != nil && len(receipt.ResultState) > 0 {
			if err := plan.RestoreResultState(append([]byte(nil), receipt.ResultState...)); err != nil {
				return result, fmt.Errorf("restore workspace operation result state: %w", err)
			}
		}
		if plan.RestoreDesiredState != nil && len(receipt.DesiredState) > 0 {
			if err := plan.RestoreDesiredState(append([]byte(nil), receipt.DesiredState...)); err != nil {
				return result, fmt.Errorf("restore workspace operation desired state: %w", err)
			}
		}
		if plan.RestoreAssignedUSN != nil && receipt.AssignedUSN > 0 {
			plan.RestoreAssignedUSN(receipt.AssignedUSN)
		}
		result.Committed = true
		result.AppliedSteps = append([]string(nil), receipt.AppliedSteps...)
		result.Operation = receipt
		return result, nil
	}
	if IsTerminalOperation(receipt.Status) {
		result.Mode = "terminal"
		result.AppliedSteps = append([]string(nil), receipt.AppliedSteps...)
		result.Operation = receipt
		return result, fmt.Errorf("%w: %s", ErrOperationTerminal, receipt.Status)
	}
	if len(receipt.StepNames) > 0 && !sameOperationStepNames(receipt.StepNames, plan) {
		result.PartialWrite = receipt.CurrentStep != "" || len(receipt.AppliedSteps) > 0 || receipt.AssignedUSN > 0
		result.AppliedSteps = append([]string(nil), receipt.AppliedSteps...)
		result.FailedStep = receipt.CurrentStep
		result.Operation = receipt
		return result, fmt.Errorf("%w: operation step plan changed", ErrOperationPending)
	}
	if plan.RestoreBeforeState != nil {
		if len(receipt.BeforeState) == 0 {
			// A resumable operation without its original before-image cannot be
			// safely rebuilt from the current database state.  Fail closed and
			// leave the receipt available for an explicit repair decision.
			return result, fmt.Errorf("%w: missing before state", ErrOperationPending)
		}
		if err := plan.RestoreBeforeState(append([]byte(nil), receipt.BeforeState...)); err != nil {
			return result, fmt.Errorf("restore workspace operation before state: %w", err)
		}
	}
	if plan.RestoreDesiredState != nil && len(receipt.DesiredState) > 0 {
		if err := plan.RestoreDesiredState(append([]byte(nil), receipt.DesiredState...)); err != nil {
			return result, fmt.Errorf("restore workspace operation desired state: %w", err)
		}
	}
	receipt, err = store.Claim(ctx, plan.OwnerID, plan.OperationID, now, OperationLeaseTTL)
	if err != nil {
		return result, err
	}
	if receipt.Status == OperationCommitted {
		if plan.RestoreResultState != nil && len(receipt.ResultState) > 0 {
			if err := plan.RestoreResultState(append([]byte(nil), receipt.ResultState...)); err != nil {
				return result, fmt.Errorf("restore workspace operation result state: %w", err)
			}
		}
		if plan.RestoreAssignedUSN != nil && receipt.AssignedUSN > 0 {
			plan.RestoreAssignedUSN(receipt.AssignedUSN)
		}
		result.Committed = true
		result.AppliedSteps = append([]string(nil), receipt.AppliedSteps...)
		result.Operation = receipt
		return result, nil
	}
	if IsTerminalOperation(receipt.Status) {
		result.Mode = "terminal"
		result.AppliedSteps = append([]string(nil), receipt.AppliedSteps...)
		result.Operation = receipt
		return result, fmt.Errorf("%w: %s", ErrOperationTerminal, receipt.Status)
	}
	if plan.RestoreAssignedUSN != nil && receipt.AssignedUSN > 0 {
		plan.RestoreAssignedUSN(receipt.AssignedUSN)
	}
	if receipt.CurrentStep != "" && !planContainsStep(plan, receipt.CurrentStep) {
		return pendingStep(ctx, result, receipt, store, receipt.CurrentStep, ErrUnknownStepResult)
	}

	for _, step := range plan.Steps {
		restoreStepAssignedUSN(step, &receipt)
		if slices.Contains(receipt.AppliedSteps, step.Name) {
			continue
		}
		if receipt.CurrentStep == step.Name {
			if step.Verify == nil {
				if !step.ReplaySafe {
					return pendingStep(ctx, result, receipt, store, step.Name, ErrUnknownStepResult)
				}
			} else {
				applied, verifyErr := step.Verify(ctx)
				if verifyErr != nil {
					captureStepAssignedUSN(step, &receipt)
					return pendingStep(ctx, result, receipt, store, step.Name, verifyErr)
				}
				if applied {
					receipt.AppliedSteps = append(receipt.AppliedSteps, step.Name)
					receipt.CurrentStep = ""
					captureStepAssignedUSN(step, &receipt)
					captureAssignedUSN(plan, &receipt)
					captureResultState(plan, &receipt)
					if err = saveReceipt(ctx, store, &receipt); err != nil {
						return partialResult(result, receipt, step.Name, err)
					}
					continue
				}
				if !step.ReplaySafe && step.Compensate == nil {
					captureStepAssignedUSN(step, &receipt)
					return pendingStep(ctx, result, receipt, store, step.Name, ErrUnknownStepResult)
				}
			}
		}

		receipt.CurrentStep = step.Name
		receipt.FailedStep = ""
		receipt.LastError = ""
		if err = saveReceipt(ctx, store, &receipt); err != nil {
			return partialResult(result, receipt, step.Name, err)
		}
		if err = step.Apply(ctx); err != nil {
			captureStepAssignedUSN(step, &receipt)
			if step.Verify != nil {
				applied, verifyErr := step.Verify(ctx)
				if verifyErr != nil {
					captureStepAssignedUSN(step, &receipt)
					return pendingStep(ctx, result, receipt, store, step.Name, verifyErr)
				}
				if applied {
					receipt.AppliedSteps = append(receipt.AppliedSteps, step.Name)
					receipt.CurrentStep = ""
					captureStepAssignedUSN(step, &receipt)
					captureAssignedUSN(plan, &receipt)
					captureResultState(plan, &receipt)
					if err = saveReceipt(ctx, store, &receipt); err != nil {
						return partialResult(result, receipt, step.Name, err, true)
					}
					continue
				}
			}
			if plan.FailurePolicy == FailurePending {
				return pendingStep(ctx, result, receipt, store, step.Name, err)
			}
			return compensateFailure(ctx, result, plan, receipt, store, step, err)
		}
		receipt.AppliedSteps = append(receipt.AppliedSteps, step.Name)
		receipt.CurrentStep = ""
		captureStepAssignedUSN(step, &receipt)
		captureAssignedUSN(plan, &receipt)
		captureResultState(plan, &receipt)
		if err = saveReceipt(ctx, store, &receipt); err != nil {
			return partialResult(result, receipt, step.Name, err)
		}
	}

	receipt.Status = OperationCommitted
	if plan.CaptureResultState != nil {
		receipt.ResultState = append([]byte(nil), plan.CaptureResultState()...)
	}
	receipt.CurrentStep = ""
	receipt.FailedStep = ""
	receipt.LastError = ""
	if err = saveReceipt(ctx, store, &receipt); err != nil {
		return partialResult(result, receipt, "commit_receipt", err)
	}
	result.Committed = true
	result.AppliedSteps = append([]string(nil), receipt.AppliedSteps...)
	result.Operation = receipt
	return result, nil
}

// RedactTerminalReceipt removes replay payloads once an operation can no
// longer be resumed.  Keeping the identity and outcome is sufficient for
// idempotency, while before/desired state may contain note content and must
// not become a permanent history store.
func RedactTerminalReceipt(receipt *OperationReceipt) {
	if receipt == nil {
		return
	}
	switch receipt.Status {
	case OperationCompensated, OperationFailed:
		receipt.BeforeState = nil
		receipt.DesiredState = nil
		receipt.ResultState = nil
		receipt.CurrentStep = ""
		receipt.LastError = ""
	case OperationCommitted:
		receipt.BeforeState = nil
		receipt.DesiredState = nil
		receipt.LastError = ""
	}
}

func captureResultState(plan MutationPlan, receipt *OperationReceipt) {
	if plan.CaptureResultState == nil || receipt == nil {
		return
	}
	receipt.ResultState = append([]byte(nil), plan.CaptureResultState()...)
}

func planContainsStep(plan MutationPlan, name string) bool {
	for _, step := range plan.Steps {
		if step.Name == name {
			return true
		}
	}
	return false
}

func operationStepNames(plan MutationPlan) []string {
	names := make([]string, 0, len(plan.Steps))
	for _, step := range plan.Steps {
		names = append(names, step.Name)
	}
	return names
}

func sameOperationStepNames(receiptNames []string, plan MutationPlan) bool {
	planNames := operationStepNames(plan)
	if len(receiptNames) != len(planNames) {
		return false
	}
	for index := range receiptNames {
		if receiptNames[index] != planNames[index] {
			return false
		}
	}
	return true
}

func validatePlan(plan MutationPlan, store OperationStore) error {
	if store == nil || plan.OperationID == "" || plan.OwnerID.IsZero() || plan.Kind == "" || plan.InputDigest == "" {
		return fmt.Errorf("workspace operation: incomplete durable identity")
	}
	if len(plan.Steps) == 0 {
		return fmt.Errorf("workspace operation: no steps")
	}
	seen := make(map[string]struct{}, len(plan.Steps))
	for _, step := range plan.Steps {
		if step.Name == "" || step.Apply == nil {
			return fmt.Errorf("workspace operation: invalid step")
		}
		if _, exists := seen[step.Name]; exists {
			return fmt.Errorf("workspace operation: duplicate step %s", step.Name)
		}
		seen[step.Name] = struct{}{}
	}
	return nil
}

func captureAssignedUSN(plan MutationPlan, receipt *OperationReceipt) {
	if plan.AssignedUSN != nil {
		if usn := plan.AssignedUSN(); usn > 0 {
			receipt.AssignedUSN = usn
		}
	}
}

func captureStepAssignedUSN(step MutationStep, receipt *OperationReceipt) {
	if receipt == nil || step.AssignedUSN == nil {
		return
	}
	if usn := step.AssignedUSN(); usn > 0 {
		if receipt.StepUSNs == nil {
			receipt.StepUSNs = make(map[string]int)
		}
		receipt.StepUSNs[step.Name] = usn
	}
}

func restoreStepAssignedUSN(step MutationStep, receipt *OperationReceipt) {
	if receipt == nil || step.RestoreAssignedUSN == nil {
		return
	}
	if usn := receipt.StepUSNs[step.Name]; usn > 0 {
		step.RestoreAssignedUSN(usn)
	}
}

func cloneStepUSNs(source map[string]int) map[string]int {
	if len(source) == 0 {
		return nil
	}
	cloned := make(map[string]int, len(source))
	for name, usn := range source {
		cloned[name] = usn
	}
	return cloned
}

func compensateFailure(ctx context.Context, result MutationResult, plan MutationPlan, receipt OperationReceipt, store OperationStore, failed MutationStep, cause error) (MutationResult, error) {
	receipt.Status = OperationCompensating
	receipt.FailedStep = failed.Name
	receipt.LastError = safeOperationError(cause)
	captureStepAssignedUSN(failed, &receipt)
	captureAssignedUSN(plan, &receipt)
	var err error
	if err = saveReceipt(ctx, store, &receipt); err != nil {
		return partialResult(result, receipt, failed.Name, fmt.Errorf("record failed step: %w", err))
	}

	compensate := func(step MutationStep) error {
		if step.Compensate == nil {
			return nil
		}
		if err := step.Compensate(ctx); err != nil {
			return fmt.Errorf("compensate %s: %w", step.Name, err)
		}
		return nil
	}
	// A step that returns an error is not marked applied. Each Apply boundary
	// must be atomic or clean its own partial writes; compensating it here could
	// delete a pre-existing record after a duplicate-key error. Unknown results
	// after a successful Apply are recovered through CurrentStep + Verify.
	retained := make([]string, 0, len(receipt.AppliedSteps))
	for index := len(receipt.AppliedSteps) - 1; index >= 0; index-- {
		name := receipt.AppliedSteps[index]
		for _, step := range plan.Steps {
			if step.Name == name {
				if step.Compensate == nil {
					retained = append(retained, name)
					break
				}
				if err := compensate(step); err != nil {
					return failReceipt(ctx, result, receipt, store, name, err)
				}
				break
			}
		}
	}
	hadAppliedWrites := len(receipt.AppliedSteps) > 0
	receipt.CurrentStep = ""
	slices.Reverse(retained)
	receipt.AppliedSteps = retained
	switch {
	case len(retained) > 0:
		receipt.Status = OperationRepairPending
	case hadAppliedWrites:
		receipt.Status = OperationCompensated
	default:
		receipt.Status = OperationFailed
	}
	err = saveReceipt(ctx, store, &receipt)
	if err != nil {
		return partialResult(result, receipt, failed.Name, fmt.Errorf("save compensation state: %w", err))
	}
	result.PartialWrite = hadAppliedWrites
	result.FailedStep = failed.Name
	result.Operation = receipt
	return result, fmt.Errorf("workspace operation step %s: %w", failed.Name, cause)
}

func pendingStep(ctx context.Context, result MutationResult, receipt OperationReceipt, store OperationStore, step string, cause error) (MutationResult, error) {
	receipt.Status = OperationRepairPending
	receipt.FailedStep = step
	receipt.LastError = safeOperationError(cause)
	updated, saveErr := store.Save(ctx, receipt)
	if saveErr == nil {
		receipt = updated
	} else {
		cause = fmt.Errorf("%v; record pending operation: %w", cause, saveErr)
	}
	// CurrentStep means Apply may have reached its external boundary even when
	// no completed step or assigned USN has been recorded yet.
	result.PartialWrite = true
	result.AppliedSteps = append([]string(nil), receipt.AppliedSteps...)
	result.FailedStep = step
	result.Operation = receipt
	return result, fmt.Errorf("%w: %s: %v", ErrOperationPending, step, cause)
}

func failReceipt(ctx context.Context, result MutationResult, receipt OperationReceipt, store OperationStore, step string, cause error) (MutationResult, error) {
	receipt.Status = OperationFailed
	receipt.FailedStep = step
	receipt.LastError = safeOperationError(cause)
	updated, saveErr := store.Save(ctx, receipt)
	if saveErr == nil {
		receipt = updated
	} else {
		cause = fmt.Errorf("%v; record operation failure: %w", cause, saveErr)
	}
	return partialResult(result, receipt, step, cause, true)
}

func partialResult(result MutationResult, receipt OperationReceipt, step string, err error, possibleWrite ...bool) (MutationResult, error) {
	result.PartialWrite = len(receipt.AppliedSteps) > 0 || receipt.AssignedUSN > 0
	if len(possibleWrite) > 0 {
		result.PartialWrite = result.PartialWrite || possibleWrite[0]
	}
	result.AppliedSteps = append([]string(nil), receipt.AppliedSteps...)
	result.FailedStep = step
	result.Operation = receipt
	return result, err
}

func saveReceipt(ctx context.Context, store OperationStore, receipt *OperationReceipt) error {
	updated, err := store.Save(ctx, *receipt)
	if err != nil {
		return err
	}
	*receipt = updated
	return nil
}

func safeOperationError(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return "timeout"
	case errors.Is(err, ErrUnknownStepResult):
		return "unknown_result"
	case errors.Is(err, ErrOperationPending):
		return "pending"
	case errors.Is(err, ErrOperationCAS):
		return "state_conflict"
	default:
		return "operation_failed"
	}
}
