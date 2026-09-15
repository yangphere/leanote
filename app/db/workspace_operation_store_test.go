package db

import (
	"context"
	"errors"
	"testing"
	"time"

	applicationnotes "github.com/yangphere/leanote/app/application/notes"
	"github.com/yangphere/leanote/app/domain"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func useWorkspaceOperationTestCollection(t *testing.T) {
	t.Helper()
	_, raw := testCollection(t)
	collection := raw.Database().Collection("workspace_operations_test")
	if err := collection.Drop(context.Background()); err != nil {
		t.Fatal(err)
	}
	saved := WorkspaceOperations
	WorkspaceOperations = wrapCollection(collection)
	t.Cleanup(func() { WorkspaceOperations = saved })
}

func TestMongoWorkspaceOperationStorePersistsAndFencesTransitions(t *testing.T) {
	useWorkspaceOperationTestCollection(t)
	store := MongoWorkspaceOperationStore{}
	now := time.Unix(100, 0)
	initial := applicationnotes.OperationReceipt{
		OperationID: "note_save:persist", OwnerID: workspaceTestOwner(t), Kind: "note_save",
		InputDigest: "digest", DesiredState: []byte(`{"title":"first"}`), Status: applicationnotes.OperationPending,
		CreatedAt: now, UpdatedAt: now,
	}
	begun, err := store.Begin(context.Background(), initial)
	if err != nil || begun.Version != 1 {
		t.Fatalf("begun=%+v err=%v", begun, err)
	}
	claimed, err := store.Claim(context.Background(), initial.OwnerID, initial.OperationID, now, time.Minute)
	if err != nil || claimed.LeaseID == "" || claimed.Version != 2 {
		t.Fatalf("claimed=%+v err=%v", claimed, err)
	}
	if _, err := store.Claim(context.Background(), initial.OwnerID, initial.OperationID, now.Add(time.Second), time.Minute); !errors.Is(err, applicationnotes.ErrOperationBusy) {
		t.Fatalf("second claim err=%v", err)
	}
	stale := claimed
	claimed.CurrentStep = "note"
	saved, err := store.Save(context.Background(), claimed)
	if err != nil || saved.Version != 3 || saved.CurrentStep != "note" {
		t.Fatalf("saved=%+v err=%v", saved, err)
	}
	if _, err := store.Save(context.Background(), stale); !errors.Is(err, applicationnotes.ErrOperationCAS) {
		t.Fatalf("stale save err=%v", err)
	}
	got, err := store.Get(context.Background(), initial.OwnerID, initial.OperationID)
	if err != nil || got.CurrentStep != "note" || got.LeaseID != saved.LeaseID {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func TestMongoWorkspaceOperationStoreUsesDigestAsIdempotencyContract(t *testing.T) {
	useWorkspaceOperationTestCollection(t)
	store := MongoWorkspaceOperationStore{}
	now := time.Unix(100, 0)
	initial := applicationnotes.OperationReceipt{
		OperationID: "note_create:stable", OwnerID: workspaceTestOwner(t), Kind: "note_create",
		InputDigest: "same", BeforeState: []byte(`{"missing":true}`), DesiredState: []byte(`{"time":100}`),
		Status: applicationnotes.OperationPending, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := store.Begin(context.Background(), initial); err != nil {
		t.Fatal(err)
	}
	retry := initial
	retry.BeforeState = []byte(`{"partiallyApplied":true}`)
	retry.DesiredState = []byte(`{"time":200}`)
	got, err := store.Begin(context.Background(), retry)
	if err != nil || string(got.DesiredState) != string(initial.DesiredState) {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	retry.InputDigest = "different"
	if _, err := store.Begin(context.Background(), retry); !errors.Is(err, applicationnotes.ErrOperationConflict) {
		t.Fatalf("conflicting begin err=%v", err)
	}
}

func TestMongoWorkspaceOperationStoreCommittedReceiptIsImmutable(t *testing.T) {
	useWorkspaceOperationTestCollection(t)
	store := MongoWorkspaceOperationStore{}
	now := time.Unix(100, 0)
	initial := applicationnotes.OperationReceipt{
		OperationID: "note_save:terminal", OwnerID: workspaceTestOwner(t), Kind: "note_save",
		InputDigest: "digest", Status: applicationnotes.OperationPending, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := store.Begin(context.Background(), initial); err != nil {
		t.Fatal(err)
	}
	claimed, err := store.Claim(context.Background(), initial.OwnerID, initial.OperationID, now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	claimed.Status = applicationnotes.OperationCommitted
	committed, err := store.Save(context.Background(), claimed)
	if err != nil {
		t.Fatal(err)
	}
	committed.Status = applicationnotes.OperationRunning
	if _, err := store.Save(context.Background(), committed); !errors.Is(err, applicationnotes.ErrOperationCAS) {
		t.Fatalf("rewrite committed receipt err=%v", err)
	}
}

func TestMongoWorkspaceOperationStoreDoesNotReclaimTerminalReceipts(t *testing.T) {
	useWorkspaceOperationTestCollection(t)
	store := MongoWorkspaceOperationStore{}
	now := time.Unix(100, 0)
	owner := workspaceTestOwner(t)
	for _, status := range []applicationnotes.OperationStatus{
		applicationnotes.OperationCompensated,
		applicationnotes.OperationFailed,
	} {
		operationID := "note_save:terminal-" + string(status)
		initial := applicationnotes.OperationReceipt{
			OperationID: operationID, OwnerID: owner, ResourceID: owner, Kind: "note_save",
			InputDigest: operationID, BeforeState: []byte("private content"),
			DesiredState: []byte("private result"), Status: applicationnotes.OperationPending,
			CreatedAt: now, UpdatedAt: now,
		}
		claimed, err := store.Begin(context.Background(), initial)
		if err != nil {
			t.Fatal(err)
		}
		claimed, err = store.Claim(context.Background(), owner, operationID, now, time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		claimed.Status = status
		terminal, err := store.Save(context.Background(), claimed)
		if err != nil || terminal.Status != status || len(terminal.BeforeState) != 0 || len(terminal.DesiredState) != 0 {
			t.Fatalf("status=%s terminal=%+v err=%v", status, terminal, err)
		}

		retry, err := store.Claim(context.Background(), owner, operationID, now.Add(time.Hour), time.Minute)
		if err != nil || retry.Status != status || retry.LeaseID != "" {
			t.Fatalf("status=%s retry=%+v err=%v", status, retry, err)
		}
		if _, err := store.Save(context.Background(), retry); !errors.Is(err, applicationnotes.ErrOperationCAS) {
			t.Fatalf("status=%s terminal rewrite err=%v", status, err)
		}
	}
}

func TestMongoWorkspaceOperationStoreRoundTripsTerminalAt(t *testing.T) {
	useWorkspaceOperationTestCollection(t)
	store := MongoWorkspaceOperationStore{}
	now := time.Unix(100, 0)
	terminalAt := now.Add(time.Minute)
	initial := applicationnotes.OperationReceipt{
		OperationID: "note_save:terminal-at", OwnerID: workspaceTestOwner(t), Kind: "note_save",
		InputDigest: "digest", Status: applicationnotes.OperationPending, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := store.Begin(context.Background(), initial); err != nil {
		t.Fatal(err)
	}
	claimed, err := store.Claim(context.Background(), initial.OwnerID, initial.OperationID, now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	claimed.Status = applicationnotes.OperationCommitted
	claimed.TerminalAt = terminalAt
	if _, err := store.Save(context.Background(), claimed); err != nil {
		t.Fatal(err)
	}

	got, err := store.Get(context.Background(), initial.OwnerID, initial.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.TerminalAt.Equal(terminalAt) {
		t.Fatalf("terminal timestamp was not persisted: got %v want %v", got.TerminalAt, terminalAt)
	}
}

func TestMongoWorkspaceOperationStoreScopesReadClaimAndSaveByOwner(t *testing.T) {
	useWorkspaceOperationTestCollection(t)
	store := MongoWorkspaceOperationStore{}
	owner := workspaceTestOwner(t)
	other, err := domain.ParseObjectID("507f1f77bcf86cd799439099")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(100, 0)
	initial := applicationnotes.OperationReceipt{
		OperationID: "note_save:owner", OwnerID: owner, Kind: "note_save", InputDigest: "digest",
		Status: applicationnotes.OperationPending, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := store.Begin(context.Background(), initial); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(context.Background(), other, initial.OperationID); !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("cross-owner get err=%v", err)
	}
	if _, err := store.Claim(context.Background(), other, initial.OperationID, now, time.Minute); !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("cross-owner claim err=%v", err)
	}
	claimed, err := store.Claim(context.Background(), owner, initial.OperationID, now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	claimed.OwnerID = other
	if _, err := store.Save(context.Background(), claimed); !errors.Is(err, applicationnotes.ErrOperationCAS) {
		t.Fatalf("cross-owner save err=%v", err)
	}
}

func TestMongoWorkspaceOperationStoreDeletesAllResourceReceiptsAfterPermanentDelete(t *testing.T) {
	useWorkspaceOperationTestCollection(t)
	store := MongoWorkspaceOperationStore{}
	owner := workspaceTestOwner(t)
	resource, err := domain.ParseObjectID("507f1f77bcf86cd799439012")
	if err != nil {
		t.Fatal(err)
	}
	otherResource, err := domain.ParseObjectID("507f1f77bcf86cd799439099")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(100, 0)
	for _, receipt := range []applicationnotes.OperationReceipt{
		{OperationID: "note_save:pending", OwnerID: owner, ResourceID: resource, Kind: "note_save", InputDigest: "pending", BeforeState: []byte("private content"), Status: applicationnotes.OperationPending, CreatedAt: now, UpdatedAt: now},
		{OperationID: "note_save:other-resource", OwnerID: owner, ResourceID: otherResource, Kind: "note_save", InputDigest: "other", Status: applicationnotes.OperationPending, CreatedAt: now, UpdatedAt: now},
	} {
		if _, err := store.Begin(context.Background(), receipt); err != nil {
			t.Fatal(err)
		}
	}
	terminal := applicationnotes.OperationReceipt{
		OperationID: "note_save:committed", OwnerID: owner, ResourceID: resource, Kind: "note_save", InputDigest: "committed", Status: applicationnotes.OperationPending, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := store.Begin(context.Background(), terminal); err != nil {
		t.Fatal(err)
	}
	claimed, err := store.Claim(context.Background(), owner, terminal.OperationID, now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	claimed.Status = applicationnotes.OperationCommitted
	if _, err := store.Save(context.Background(), claimed); err != nil {
		t.Fatal(err)
	}

	if err := store.DeleteForResource(context.Background(), owner, resource); err != nil {
		t.Fatal(err)
	}
	for _, operationID := range []string{"note_save:pending", "note_save:committed"} {
		if _, err := store.Get(context.Background(), owner, operationID); !errors.Is(err, mongo.ErrNoDocuments) {
			t.Fatalf("operation %s survived permanent delete: %v", operationID, err)
		}
	}
	if _, err := store.Get(context.Background(), owner, "note_save:other-resource"); err != nil {
		t.Fatalf("other resource receipt was deleted: %v", err)
	}
}

func TestMongoWorkspaceOperationStoreQuarantinesRecoveryReceiptsBeforePermanentDelete(t *testing.T) {
	useWorkspaceOperationTestCollection(t)
	store := MongoWorkspaceOperationStore{}
	owner := workspaceTestOwner(t)
	resource, err := domain.ParseObjectID("507f1f77bcf86cd799439012")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(100, 0)
	for _, operationID := range []string{"note_save:pending", "note_save:repair", "note_delete_cleanup"} {
		if _, err := store.Begin(context.Background(), applicationnotes.OperationReceipt{
			OperationID: operationID, OwnerID: owner, ResourceID: resource, Kind: "note_save", InputDigest: operationID,
			BeforeState: []byte("private content"), DesiredState: []byte("private desired"),
			StepUSNs: map[string]int{"note": 11}, Status: applicationnotes.OperationPending, CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatal(err)
		}
	}
	repair, err := store.Claim(context.Background(), owner, "note_save:repair", now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	repair.Status = applicationnotes.OperationRepairPending
	if _, err := store.Save(context.Background(), repair); err != nil {
		t.Fatal(err)
	}

	if err := store.QuarantineForResource(context.Background(), owner, resource, "note_delete_cleanup"); err != nil {
		t.Fatal(err)
	}
	for _, operationID := range []string{"note_save:pending", "note_save:repair"} {
		got, err := store.Get(context.Background(), owner, operationID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != applicationnotes.OperationFailed || len(got.BeforeState) != 0 || len(got.DesiredState) != 0 || len(got.StepUSNs) != 0 || got.LeaseID != "" || got.TerminalAt.IsZero() {
			t.Fatalf("quarantined operation=%s receipt=%+v", operationID, got)
		}
	}
	cleanup, err := store.Get(context.Background(), owner, "note_delete_cleanup")
	if err != nil {
		t.Fatal(err)
	}
	if cleanup.Status != applicationnotes.OperationPending || string(cleanup.DesiredState) != "private desired" || len(cleanup.StepUSNs) != 1 {
		t.Fatalf("cleanup receipt was changed: %+v", cleanup)
	}
}
