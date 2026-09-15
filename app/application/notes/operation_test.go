package notes

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/yangphere/leanote/app/domain"
)

type memoryOperationStore struct {
	mu             sync.Mutex
	receipts       map[string]OperationReceipt
	failSaveOnce   func(OperationReceipt) bool
	redactTerminal bool
}

func newMemoryOperationStore() *memoryOperationStore {
	return &memoryOperationStore{receipts: make(map[string]OperationReceipt)}
}

func (s *memoryOperationStore) Begin(_ context.Context, receipt OperationReceipt) (OperationReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.receipts[receipt.OperationID]; ok {
		if existing.InputDigest != receipt.InputDigest || existing.OwnerID != receipt.OwnerID || existing.ResourceID != receipt.ResourceID || existing.Kind != receipt.Kind {
			return OperationReceipt{}, ErrOperationConflict
		}
		return existing, nil
	}
	receipt.Version = 1
	s.receipts[receipt.OperationID] = receipt
	return receipt, nil
}

func (s *memoryOperationStore) Claim(_ context.Context, ownerID domain.ObjectID, id string, now time.Time, ttl time.Duration) (OperationReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	receipt := s.receipts[id]
	if receipt.OwnerID != ownerID {
		return OperationReceipt{}, errors.New("not found")
	}
	if IsTerminalOperation(receipt.Status) {
		return receipt, nil
	}
	if receipt.LeaseID != "" && now.Before(receipt.LeaseUntil) {
		return OperationReceipt{}, ErrOperationBusy
	}
	receipt.Status = OperationRunning
	receipt.LeaseID = "lease"
	receipt.LeaseUntil = now.Add(ttl)
	receipt.Version++
	s.receipts[id] = receipt
	return receipt, nil
}

func (s *memoryOperationStore) Save(_ context.Context, receipt OperationReceipt) (OperationReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current := s.receipts[receipt.OperationID]
	if receipt.LeaseID == "" || IsTerminalOperation(current.Status) || current.Version != receipt.Version || current.LeaseID != receipt.LeaseID {
		return OperationReceipt{}, ErrOperationCAS
	}
	if s.failSaveOnce != nil && s.failSaveOnce(receipt) {
		s.failSaveOnce = nil
		return OperationReceipt{}, errors.New("unknown save result")
	}
	if s.redactTerminal {
		RedactTerminalReceipt(&receipt)
	}
	receipt.Version++
	if receipt.Status == OperationCommitted || receipt.Status == OperationCompensated || receipt.Status == OperationFailed || receipt.Status == OperationRepairPending {
		receipt.LeaseID = ""
		receipt.LeaseUntil = time.Time{}
	}
	s.receipts[receipt.OperationID] = receipt
	return receipt, nil
}

func TestExecuteStandaloneVerifiesApplyErrorBeforeCompensation(t *testing.T) {
	store := newMemoryOperationStore()
	applied := false
	applyCount := 0
	compensateCount := 0
	plan := MutationPlan{
		OperationID: "note_save:apply-timeout", OwnerID: operationTestOwner(t), Kind: "note_save", InputDigest: "digest",
		Steps: []MutationStep{{
			Name: "note",
			Apply: func(context.Context) error {
				applyCount++
				applied = true
				return context.DeadlineExceeded
			},
			Verify:     func(context.Context) (bool, error) { return applied, nil },
			Compensate: func(context.Context) error { compensateCount++; return nil },
		}},
	}
	result, err := ExecuteStandalone(context.Background(), plan, store, time.Unix(100, 0))
	if err != nil || !result.Committed || applyCount != 1 || compensateCount != 0 {
		t.Fatalf("result=%+v applyCount=%d compensateCount=%d err=%v", result, applyCount, compensateCount, err)
	}
}

func TestExecuteStandaloneDoesNotReportReceiptSaveBeforeApplyAsPartialWrite(t *testing.T) {
	store := newMemoryOperationStore()
	store.failSaveOnce = func(receipt OperationReceipt) bool { return receipt.CurrentStep == "note" }
	applied := false
	plan := MutationPlan{
		OperationID: "note_save:receipt-timeout", OwnerID: operationTestOwner(t), Kind: "note_save", InputDigest: "digest",
		Steps: []MutationStep{{Name: "note", Apply: func(context.Context) error { applied = true; return nil }}},
	}
	result, err := ExecuteStandalone(context.Background(), plan, store, time.Unix(100, 0))
	if err == nil || result.PartialWrite || applied {
		t.Fatalf("result=%+v applied=%v err=%v", result, applied, err)
	}
}

func TestExecuteStandaloneStoresRedactedFailureCode(t *testing.T) {
	store := newMemoryOperationStore()
	plan := MutationPlan{
		OperationID: "note_save:redacted", OwnerID: operationTestOwner(t), Kind: "note_save", InputDigest: "digest",
		Steps: []MutationStep{{Name: "note", Apply: func(context.Context) error { return errors.New("private note content") }}},
	}
	result, err := ExecuteStandalone(context.Background(), plan, store, time.Unix(100, 0))
	if err == nil || result.Operation.LastError != "operation_failed" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func (s *memoryOperationStore) Get(_ context.Context, ownerID domain.ObjectID, id string) (OperationReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	receipt, ok := s.receipts[id]
	if !ok || receipt.OwnerID != ownerID {
		return OperationReceipt{}, errors.New("not found")
	}
	return receipt, nil
}

func operationTestOwner(t *testing.T) domain.ObjectID {
	t.Helper()
	id, err := domain.ParseObjectID("507f1f77bcf86cd799439011")
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestExecuteStandaloneReturnsCommittedReceiptOnRetry(t *testing.T) {
	store := newMemoryOperationStore()
	applyCount := 0
	plan := MutationPlan{
		OperationID: "note_create:one", OwnerID: operationTestOwner(t), Kind: "note_create", InputDigest: "digest",
		Steps: []MutationStep{{Name: "note", Apply: func(context.Context) error { applyCount++; return nil }}},
	}
	first, err := ExecuteStandalone(context.Background(), plan, store, time.Unix(100, 0))
	if err != nil || !first.Committed {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, err := ExecuteStandalone(context.Background(), plan, store, time.Unix(101, 0))
	if err != nil || !second.Committed || applyCount != 1 {
		t.Fatalf("second=%+v applyCount=%d err=%v", second, applyCount, err)
	}
}

func TestExecuteStandaloneVerifiesUnknownStepBeforeRetry(t *testing.T) {
	store := newMemoryOperationStore()
	store.failSaveOnce = func(receipt OperationReceipt) bool {
		return receipt.CurrentStep == "" && reflect.DeepEqual(receipt.AppliedSteps, []string{"note"})
	}
	applied := false
	applyCount := 0
	plan := MutationPlan{
		OperationID: "note_save:one", OwnerID: operationTestOwner(t), Kind: "note_save", InputDigest: "digest",
		Steps: []MutationStep{{
			Name:   "note",
			Apply:  func(context.Context) error { applyCount++; applied = true; return nil },
			Verify: func(context.Context) (bool, error) { return applied, nil },
		}},
	}
	first, err := ExecuteStandalone(context.Background(), plan, store, time.Unix(100, 0))
	if err == nil || !first.PartialWrite {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	// Simulate process loss after the lease expires. The durable CurrentStep
	// remains note, so Verify closes the unknown result without applying twice.
	second, err := ExecuteStandalone(context.Background(), plan, store, time.Unix(131, 0))
	if err != nil || !second.Committed || applyCount != 1 {
		t.Fatalf("second=%+v applyCount=%d err=%v", second, applyCount, err)
	}
}

func TestExecuteStandaloneDoesNotReplayCompensatedOperation(t *testing.T) {
	store := newMemoryOperationStore()
	fail := true
	events := []string{}
	plan := MutationPlan{
		OperationID: "tag_detach:one", OwnerID: operationTestOwner(t), Kind: "tag_detach", InputDigest: "digest",
		Steps: []MutationStep{
			{Name: "tag", Apply: func(context.Context) error { events = append(events, "tag"); return nil }, Compensate: func(context.Context) error { events = append(events, "undo-tag"); return nil }},
			{Name: "note", Apply: func(context.Context) error {
				if fail {
					return errors.New("write failed")
				}
				events = append(events, "note")
				return nil
			}},
		},
	}
	first, err := ExecuteStandalone(context.Background(), plan, store, time.Unix(100, 0))
	if err == nil || !first.PartialWrite || !reflect.DeepEqual(events, []string{"tag", "undo-tag"}) {
		t.Fatalf("first=%+v events=%v err=%v", first, events, err)
	}
	fail = false
	second, err := ExecuteStandalone(context.Background(), plan, store, time.Unix(101, 0))
	if !errors.Is(err, ErrOperationTerminal) || second.Committed || !reflect.DeepEqual(events, []string{"tag", "undo-tag"}) {
		t.Fatalf("second=%+v events=%v err=%v", second, events, err)
	}
}

func TestExecuteStandaloneNeverCompensatesAnUnappliedFailingStep(t *testing.T) {
	store := newMemoryOperationStore()
	compensated := false
	plan := MutationPlan{
		OperationID: "note_create:duplicate", OwnerID: operationTestOwner(t), Kind: "note_create", InputDigest: "digest",
		Steps: []MutationStep{{
			Name:       "note",
			Apply:      func(context.Context) error { return errors.New("duplicate key") },
			Compensate: func(context.Context) error { compensated = true; return nil },
		}},
	}
	result, err := ExecuteStandalone(context.Background(), plan, store, time.Unix(100, 0))
	if err == nil || result.PartialWrite || compensated {
		t.Fatalf("result=%+v compensated=%v err=%v", result, compensated, err)
	}
}

func TestRepairFailurePreservesProgressAndDoesNotBlindlyReplayUnknownProvider(t *testing.T) {
	store := newMemoryOperationStore()
	providerCalls := 0
	plan := MutationPlan{
		OperationID: "projection:pending", OwnerID: operationTestOwner(t), Kind: "projection", InputDigest: "digest",
		FailurePolicy: FailurePending,
		Steps: []MutationStep{
			{Name: "count", Apply: func(context.Context) error { return nil }},
			{Name: "asset", Apply: func(context.Context) error { providerCalls++; return errors.New("unknown provider result") }},
		},
	}
	first, err := ExecuteStandalone(context.Background(), plan, store, time.Unix(100, 0))
	if !errors.Is(err, ErrOperationPending) || !first.PartialWrite || first.Operation.Status != OperationRepairPending ||
		!reflect.DeepEqual(first.Operation.AppliedSteps, []string{"count"}) || first.Operation.CurrentStep != "asset" {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, err := ExecuteStandalone(context.Background(), plan, store, time.Unix(101, 0))
	if !errors.Is(err, ErrOperationPending) || providerCalls != 1 || second.Operation.CurrentStep != "asset" {
		t.Fatalf("second=%+v providerCalls=%d err=%v", second, providerCalls, err)
	}
}

func TestVerifyFailureReportsUnknownCurrentStepAsPartialWrite(t *testing.T) {
	store := newMemoryOperationStore()
	plan := MutationPlan{
		OperationID: "note_save:verify-error", OwnerID: operationTestOwner(t), Kind: "note_save", InputDigest: "digest",
		Steps: []MutationStep{{
			Name:   "note",
			Apply:  func(context.Context) error { return context.DeadlineExceeded },
			Verify: func(context.Context) (bool, error) { return false, errors.New("verify unavailable") },
		}},
	}
	result, err := ExecuteStandalone(context.Background(), plan, store, time.Unix(100, 0))
	if err == nil || !result.PartialWrite || result.Operation.CurrentStep != "note" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestVerifyErrorAfterApplyErrorRemainsRepairableAcrossCalls(t *testing.T) {
	store := newMemoryOperationStore()
	store.redactTerminal = true
	applyCalls := 0
	verifyCalls := 0
	restoredBefore := false
	restoredDesired := false
	plan := MutationPlan{
		OperationID: "note_save:verify-unavailable", OwnerID: operationTestOwner(t), Kind: "note_save", InputDigest: "digest",
		BeforeState: []byte(`{"content":"before"}`), DesiredState: []byte(`{"content":"desired"}`),
		RestoreBeforeState: func(state []byte) error {
			restoredBefore = string(state) == `{"content":"before"}`
			return nil
		},
		RestoreDesiredState: func(state []byte) error {
			restoredDesired = string(state) == `{"content":"desired"}`
			return nil
		},
		Steps: []MutationStep{{
			Name: "note",
			Apply: func(context.Context) error {
				applyCalls++
				return context.DeadlineExceeded
			},
			Verify: func(context.Context) (bool, error) {
				verifyCalls++
				if verifyCalls == 1 {
					return false, errors.New("verify unavailable")
				}
				return true, nil
			},
		}},
	}

	first, err := ExecuteStandalone(context.Background(), plan, store, time.Unix(100, 0))
	if !errors.Is(err, ErrOperationPending) || !first.PartialWrite || first.Operation.Status != OperationRepairPending ||
		first.Operation.CurrentStep != "note" || string(first.Operation.BeforeState) != `{"content":"before"}` ||
		string(first.Operation.DesiredState) != `{"content":"desired"}` {
		t.Fatalf("first=%+v err=%v", first, err)
	}

	restoredBefore = false
	restoredDesired = false
	second, err := ExecuteStandalone(context.Background(), plan, store, time.Unix(101, 0))
	if err != nil || !second.Committed || applyCalls != 1 || verifyCalls != 2 || !restoredBefore || !restoredDesired {
		t.Fatalf("second=%+v applyCalls=%d verifyCalls=%d restoredBefore=%v restoredDesired=%v err=%v",
			second, applyCalls, verifyCalls, restoredBefore, restoredDesired, err)
	}
}

func TestChangedPlanCannotCommitWhileDurableCurrentStepIsMissing(t *testing.T) {
	store := newMemoryOperationStore()
	firstPlan := MutationPlan{
		OperationID: "tag_detach:changed-plan", OwnerID: operationTestOwner(t), Kind: "tag_detach", InputDigest: "digest",
		FailurePolicy: FailurePending,
		Steps:         []MutationStep{{Name: "note:one", Apply: func(context.Context) error { return errors.New("unknown result") }}},
	}
	first, err := ExecuteStandalone(context.Background(), firstPlan, store, time.Unix(100, 0))
	if !errors.Is(err, ErrOperationPending) || first.Operation.CurrentStep != "note:one" {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	changedPlan := firstPlan
	changedPlan.Steps = []MutationStep{{Name: "note:two", Apply: func(context.Context) error { return nil }}}
	second, err := ExecuteStandalone(context.Background(), changedPlan, store, time.Unix(101, 0))
	if !errors.Is(err, ErrOperationPending) || second.Committed || second.Operation.CurrentStep != "note:one" {
		t.Fatalf("second=%+v err=%v", second, err)
	}
}

func TestRepairReplaySafeProjectionResumesWithoutRepeatingCompletedUSN(t *testing.T) {
	store := newMemoryOperationStore()
	allocated := 0
	projectionCalls := 0
	failProjection := true
	plan := MutationPlan{
		OperationID: "notebook_drag:resume", OwnerID: operationTestOwner(t), Kind: "notebook_drag", InputDigest: "digest",
		FailurePolicy: FailurePending,
		Steps: []MutationStep{
			{Name: "notebook:first", ReplaySafe: true, Apply: func(context.Context) error { allocated++; return nil }},
			{Name: "notebook:second", ReplaySafe: true, Apply: func(context.Context) error {
				projectionCalls++
				if failProjection {
					return errors.New("temporary projection failure")
				}
				return nil
			}},
		},
	}
	first, err := ExecuteStandalone(context.Background(), plan, store, time.Unix(100, 0))
	if !errors.Is(err, ErrOperationPending) || first.Operation.CurrentStep != "notebook:second" || allocated != 1 {
		t.Fatalf("first=%+v allocated=%d err=%v", first, allocated, err)
	}
	failProjection = false
	second, err := ExecuteStandalone(context.Background(), plan, store, time.Unix(101, 0))
	if err != nil || !second.Committed || allocated != 1 || projectionCalls != 2 {
		t.Fatalf("second=%+v allocated=%d projectionCalls=%d err=%v", second, allocated, projectionCalls, err)
	}
}

func TestExecuteStandalonePersistsStepUSNForUnknownBatchResult(t *testing.T) {
	store := newMemoryOperationStore()
	assigned := 0
	applyCalls := 0
	verifyCalls := 0
	plan := MutationPlan{
		OperationID: "notebook_sort:step-usn", OwnerID: operationTestOwner(t), Kind: "notebook_sort", InputDigest: "digest",
		FailurePolicy: FailurePending,
		Steps: []MutationStep{{
			Name: "notebook:one",
			Apply: func(context.Context) error {
				applyCalls++
				assigned = 23
				return errors.New("result uncertain")
			},
			Verify: func(context.Context) (bool, error) {
				verifyCalls++
				return assigned == 23 && verifyCalls > 1, nil
			},
			AssignedUSN:        func() int { return assigned },
			RestoreAssignedUSN: func(usn int) { assigned = usn },
		}},
	}
	first, err := ExecuteStandalone(context.Background(), plan, store, time.Unix(100, 0))
	if !errors.Is(err, ErrOperationPending) || !first.PartialWrite || first.Operation.StepUSNs["notebook:one"] != 23 {
		t.Fatalf("first=%+v err=%v", first, err)
	}

	assigned = 0
	second, err := ExecuteStandalone(context.Background(), plan, store, time.Unix(131, 0))
	if err != nil || !second.Committed || applyCalls != 1 || assigned != 23 {
		t.Fatalf("second=%+v assigned=%d applyCalls=%d err=%v", second, assigned, applyCalls, err)
	}
}

func TestExecuteStandaloneFreezesAndRestoresNonSensitiveResultState(t *testing.T) {
	store := newMemoryOperationStore()
	owner, err := domain.ParseObjectID("507f1f77bcf86cd799439011")
	if err != nil {
		t.Fatal(err)
	}
	captured := []byte(`{"items":[{"resource":"note-1","committed":true,"usn":9}]}`)
	plan := MutationPlan{
		OperationID: "batch-result", OwnerID: owner, ResourceID: owner,
		Kind: "note_batch_move", InputDigest: "digest", FailurePolicy: FailurePending,
		Steps:              []MutationStep{{Name: "items", ReplaySafe: true, Apply: func(context.Context) error { return nil }}},
		CaptureResultState: func() []byte { return append([]byte(nil), captured...) },
	}
	first, err := ExecuteStandalone(context.Background(), plan, store, time.Now())
	if err != nil || !first.Committed || string(first.Operation.ResultState) != string(captured) {
		t.Fatalf("first result=%+v err=%v", first, err)
	}

	var restored []byte
	plan.CaptureResultState = nil
	plan.RestoreResultState = func(value []byte) error {
		restored = append([]byte(nil), value...)
		return nil
	}
	retry, err := ExecuteStandalone(context.Background(), plan, store, time.Now().Add(time.Second))
	if err != nil || !retry.Committed || string(restored) != string(captured) {
		t.Fatalf("retry result=%+v restored=%q err=%v", retry, restored, err)
	}
}

func TestRepairVerifyClosesProviderErrorWithoutReplay(t *testing.T) {
	store := newMemoryOperationStore()
	providerCalls := 0
	finalState := false
	plan := MutationPlan{
		OperationID: "note_projection:verify", OwnerID: operationTestOwner(t), Kind: "note_projection", InputDigest: "digest",
		FailurePolicy: FailurePending,
		Steps: []MutationStep{{
			Name: "image_index", ReplaySafe: true,
			Apply: func(context.Context) error {
				providerCalls++
				finalState = true
				return context.DeadlineExceeded
			},
			Verify: func(context.Context) (bool, error) { return finalState, nil },
		}},
	}
	result, err := ExecuteStandalone(context.Background(), plan, store, time.Unix(100, 0))
	if err != nil || !result.Committed || providerCalls != 1 {
		t.Fatalf("result=%+v providerCalls=%d err=%v", result, providerCalls, err)
	}
}

func TestCompensationRetainsIrreversibleUSNProgressForResume(t *testing.T) {
	store := newMemoryOperationStore()
	fail := true
	allocated := 0
	noteWrites := 0
	plan := MutationPlan{
		OperationID: "note_create:resume", OwnerID: operationTestOwner(t), Kind: "note_create", InputDigest: "digest",
		Steps: []MutationStep{
			{Name: "allocate_usn", ReplaySafe: true, Apply: func(context.Context) error { allocated++; return nil }},
			{Name: "note", Apply: func(context.Context) error {
				noteWrites++
				if fail {
					return errors.New("write failed")
				}
				return nil
			}, Compensate: func(context.Context) error { return nil }},
		},
	}
	first, err := ExecuteStandalone(context.Background(), plan, store, time.Unix(100, 0))
	if err == nil || first.Operation.Status != OperationRepairPending || !reflect.DeepEqual(first.Operation.AppliedSteps, []string{"allocate_usn"}) {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	fail = false
	second, err := ExecuteStandalone(context.Background(), plan, store, time.Unix(101, 0))
	if err != nil || !second.Committed || allocated != 1 || noteWrites != 2 {
		t.Fatalf("second=%+v allocated=%d noteWrites=%d err=%v", second, allocated, noteWrites, err)
	}
}

func TestRedactTerminalReceiptRemovesReplayPayload(t *testing.T) {
	for _, kind := range []string{"note_save", "note_create", "note_delete_cleanup", "note_projection"} {
		receipt := OperationReceipt{OperationID: "op", Kind: kind, Status: OperationCommitted,
			BeforeState: []byte("private content"), DesiredState: []byte("private history"), StepUSNs: map[string]int{"step": 1}}
		RedactTerminalReceipt(&receipt)
		if len(receipt.BeforeState) != 0 || len(receipt.DesiredState) != 0 || len(receipt.StepUSNs) != 1 || receipt.OperationID != "op" {
			t.Fatalf("kind=%s receipt=%+v", kind, receipt)
		}
	}
}

func TestRedactTerminalFailureRemovesCapturedResultState(t *testing.T) {
	for _, status := range []OperationStatus{OperationCompensated, OperationFailed} {
		receipt := OperationReceipt{
			OperationID: "op", Status: status,
			BeforeState: []byte("private content"), DesiredState: []byte("private desired"),
			ResultState: []byte("partial response"),
		}
		RedactTerminalReceipt(&receipt)
		if len(receipt.BeforeState) != 0 || len(receipt.DesiredState) != 0 || len(receipt.ResultState) != 0 {
			t.Fatalf("status=%s receipt=%+v", status, receipt)
		}
	}
}

func TestExecuteStandaloneSkipsRedactedCommittedReplayState(t *testing.T) {
	store := newMemoryOperationStore()
	owner := operationTestOwner(t)
	store.receipts["note_save:redacted-committed"] = OperationReceipt{
		OperationID: "note_save:redacted-committed", OwnerID: owner, Kind: "note_save",
		InputDigest: "digest", Status: OperationCommitted, AppliedSteps: []string{"note"}, Version: 1,
	}
	restored := false
	plan := MutationPlan{
		OperationID: "note_save:redacted-committed", OwnerID: owner, Kind: "note_save", InputDigest: "digest",
		Steps:               []MutationStep{{Name: "note", Apply: func(context.Context) error { return nil }}},
		RestoreDesiredState: func([]byte) error { restored = true; return errors.New("must not restore redacted state") },
	}
	result, err := ExecuteStandalone(context.Background(), plan, store, time.Unix(100, 0))
	if err != nil || !result.Committed || restored {
		t.Fatalf("result=%+v restored=%v err=%v", result, restored, err)
	}
}

func TestExecuteStandaloneDoesNotReplayTerminalFailureOrCompensation(t *testing.T) {
	for _, status := range []OperationStatus{OperationCompensated, OperationFailed} {
		t.Run(string(status), func(t *testing.T) {
			store := newMemoryOperationStore()
			owner := operationTestOwner(t)
			operationID := "note_save:terminal-" + string(status)
			store.receipts[operationID] = OperationReceipt{
				OperationID: operationID, OwnerID: owner, Kind: "note_save", InputDigest: "digest",
				BeforeState: []byte("private content"), DesiredState: []byte("private result"),
				Status: status, Version: 1,
			}
			applyCount := 0
			plan := MutationPlan{
				OperationID: operationID, OwnerID: owner, Kind: "note_save", InputDigest: "digest",
				Steps: []MutationStep{{Name: "note", Apply: func(context.Context) error { applyCount++; return nil }}},
			}
			result, err := ExecuteStandalone(context.Background(), plan, store, time.Unix(100, 0))
			if !errors.Is(err, ErrOperationTerminal) || result.Committed || result.Operation.Status != status || applyCount != 0 {
				t.Fatalf("result=%+v applyCount=%d err=%v", result, applyCount, err)
			}
		})
	}
}
