package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/yangphere/leanote/app/info"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var (
	ErrUpgradeLeaseHeld          = errors.New("upgrade checkpoint lease is held")
	ErrUpgradeCheckpointConflict = errors.New("upgrade checkpoint input conflict")
)

const UpgradeCheckpointLease = 2 * time.Minute

func UpgradeOperationID(kind, targetScope string) string {
	return OutboxEventIDForKey("upgrade-operation:" + kind + ":" + targetScope).Hex()
}

func AcquireUpgradeCheckpoint(ctx context.Context, operationID, stepKey, inputDigest, targetScope string, now time.Time) (info.UpgradeCheckpoint, bool, error) {
	if UpgradeCheckpoints == nil {
		return info.UpgradeCheckpoint{}, false, ErrMongoClientNotInitialized
	}
	if operationID == "" || stepKey == "" || inputDigest == "" || targetScope == "" {
		return info.UpgradeCheckpoint{}, false, errors.New("upgrade checkpoint identity is incomplete")
	}
	var current info.UpgradeCheckpoint
	findErr := UpgradeCheckpoints.FindContext(ctx, map[string]any{"OperationId": operationID, "StepKey": stepKey, "TargetScope": targetScope}).One(&current)
	if errors.Is(findErr, mongo.ErrNoDocuments) {
		leaseID := NewObjectID().Hex()
		candidate := info.UpgradeCheckpoint{
			CheckpointId: OutboxEventIDForKey("upgrade-checkpoint:" + operationID + ":" + stepKey + ":" + targetScope),
			OperationId:  operationID, StepKey: stepKey, InputDigest: inputDigest, TargetScope: targetScope,
			State: "running", Version: 1, LeaseId: leaseID, Fence: 1,
			CreatedAt: now, UpdatedAt: now, LeaseUntil: now.Add(UpgradeCheckpointLease),
		}
		if err := UpgradeCheckpoints.InsertContext(ctx, candidate); err != nil {
			if mongo.IsDuplicateKeyError(err) {
				return AcquireUpgradeCheckpoint(ctx, operationID, stepKey, inputDigest, targetScope, now)
			}
			return info.UpgradeCheckpoint{}, false, err
		}
		return candidate, false, strictUpgradeCheckpointReadBack(ctx, candidate)
	}
	if findErr != nil {
		return info.UpgradeCheckpoint{}, false, findErr
	}
	if current.InputDigest != inputDigest {
		return info.UpgradeCheckpoint{}, false, ErrUpgradeCheckpointConflict
	}
	if current.State == "completed" {
		return current, true, nil
	}
	if current.State == "running" && current.LeaseUntil.After(now) {
		return info.UpgradeCheckpoint{}, false, ErrUpgradeLeaseHeld
	}
	leaseID := NewObjectID().Hex()
	update := map[string]any{"$set": map[string]any{"State": "running", "LeaseId": leaseID, "LeaseUntil": now.Add(UpgradeCheckpointLease), "UpdatedAt": now, "ErrorCategory": ""}, "$inc": map[string]any{"Version": 1, "Fence": 1}}
	if err := UpgradeCheckpoints.UpdateOneMatchedContext(ctx, map[string]any{"_id": current.CheckpointId, "Version": current.Version, "Fence": current.Fence, "InputDigest": inputDigest}, update); err != nil {
		return info.UpgradeCheckpoint{}, false, fmt.Errorf("claim upgrade checkpoint: %w", err)
	}
	var claimed info.UpgradeCheckpoint
	if err := UpgradeCheckpoints.FindIdContext(ctx, current.CheckpointId).One(&claimed); err != nil {
		return info.UpgradeCheckpoint{}, false, err
	}
	if claimed.State != "running" || claimed.LeaseId != leaseID || claimed.Version != current.Version+1 || claimed.Fence != current.Fence+1 {
		return info.UpgradeCheckpoint{}, false, errors.New("upgrade checkpoint claim read-back mismatch")
	}
	return claimed, false, nil
}

func CompleteUpgradeCheckpoint(ctx context.Context, checkpoint info.UpgradeCheckpoint, resultDigest string, now time.Time) (info.UpgradeCheckpoint, error) {
	if resultDigest == "" {
		return info.UpgradeCheckpoint{}, errors.New("upgrade result digest is required")
	}
	update := map[string]any{"$set": map[string]any{"State": "completed", "ResultDigest": resultDigest, "CompletedAt": now, "UpdatedAt": now, "LeaseId": "", "LeaseUntil": time.Time{}}, "$inc": map[string]any{"Version": 1}}
	if err := UpgradeCheckpoints.UpdateOneMatchedContext(ctx, map[string]any{"_id": checkpoint.CheckpointId, "State": "running", "Version": checkpoint.Version, "Fence": checkpoint.Fence, "LeaseId": checkpoint.LeaseId}, update); err != nil {
		return info.UpgradeCheckpoint{}, fmt.Errorf("complete upgrade checkpoint: %w", err)
	}
	var completed info.UpgradeCheckpoint
	if err := UpgradeCheckpoints.FindIdContext(ctx, checkpoint.CheckpointId).One(&completed); err != nil {
		return info.UpgradeCheckpoint{}, err
	}
	if completed.State != "completed" || completed.ResultDigest != resultDigest || completed.Version != checkpoint.Version+1 {
		return info.UpgradeCheckpoint{}, errors.New("upgrade completion read-back mismatch")
	}
	return completed, nil
}

func FailUpgradeCheckpoint(ctx context.Context, checkpoint info.UpgradeCheckpoint, category string, now time.Time) error {
	if category == "" {
		category = "unknown"
	}
	return UpgradeCheckpoints.UpdateOneMatchedContext(ctx, map[string]any{"_id": checkpoint.CheckpointId, "State": "running", "Version": checkpoint.Version, "Fence": checkpoint.Fence, "LeaseId": checkpoint.LeaseId}, map[string]any{"$set": map[string]any{"State": "failed", "ErrorCategory": category, "UpdatedAt": now, "LeaseId": "", "LeaseUntil": time.Time{}}, "$inc": map[string]any{"Version": 1}})
}

func strictUpgradeCheckpointReadBack(ctx context.Context, want info.UpgradeCheckpoint) error {
	var got info.UpgradeCheckpoint
	if err := UpgradeCheckpoints.FindIdContext(ctx, want.CheckpointId).One(&got); err != nil {
		return err
	}
	if got.OperationId != want.OperationId || got.StepKey != want.StepKey || got.TargetScope != want.TargetScope || got.InputDigest != want.InputDigest || got.State != "running" || got.LeaseId != want.LeaseId {
		return errors.New("upgrade checkpoint read-back mismatch")
	}
	return nil
}
