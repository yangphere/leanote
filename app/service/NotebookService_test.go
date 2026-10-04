package service

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	applicationnotes "github.com/yangphere/leanote/app/application/notes"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
)

var (
	notebookReceiptMongoOnce sync.Once
	notebookReceiptMongoErr  error
)

func useNotebookReceiptTestDatabase(t *testing.T) {
	t.Helper()
	uri := os.Getenv("LEANOTE_DB_TEST_URI")
	if uri == "" {
		if os.Getenv("LEANOTE_REQUIRE_MONGO_TESTS") != "1" {
			connection, err := net.DialTimeout("tcp", "127.0.0.1:27017", 200*time.Millisecond)
			if err != nil {
				t.Skipf("MongoDB unavailable for notebook receipt tests: %v", err)
			}
			_ = connection.Close()
		}
		uri = "mongodb://127.0.0.1:27017/?serverSelectionTimeoutMS=1000"
	}
	notebookReceiptMongoOnce.Do(func() {
		notebookReceiptMongoErr = db.InitWithError(uri, "notebook_service_receipt_test")
	})
	if err := notebookReceiptMongoErr; err != nil {
		if os.Getenv("LEANOTE_DB_TEST_URI") != "" || os.Getenv("LEANOTE_REQUIRE_MONGO_TESTS") == "1" {
			t.Fatalf("MongoDB fixture is required but unavailable: %v", err)
		}
		t.Skipf("MongoDB unavailable for notebook receipt tests: %v", err)
	}
	collections := []*db.Collection{db.Users, db.Notebooks, db.WorkspaceOperations}
	for _, collection := range collections {
		if _, err := collection.RemoveAll(map[string]any{}); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, collection := range collections {
			if _, err := collection.RemoveAll(map[string]any{}); err != nil {
				t.Errorf("clean notebook receipt collection: %v", err)
			}
		}
	})
}

func TestSortNotebooksClientOperationRetryAfterResponseLossDoesNotRepeatWrites(t *testing.T) {
	useNotebookReceiptTestDatabase(t)
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	first, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	second, _ := domain.ParseObjectID("507f1f77bcf86cd799439013")
	if err := db.Users.Insert(info.User{UserId: owner, Usn: 10}); err != nil {
		t.Fatal(err)
	}
	if err := db.Notebooks.Insert(
		info.Notebook{NotebookId: first, UserId: owner, Seq: 0, Usn: 3},
		info.Notebook{NotebookId: second, UserId: owner, Seq: 1, Usn: 4},
	); err != nil {
		t.Fatal(err)
	}
	sequences := map[string]int{first.Hex(): 1, second.Hex(): 0}
	const clientOperationID = "sort-response-loss"
	service := NotebookService{}
	if !service.SortNotebooks(owner.Hex(), sequences, clientOperationID) {
		t.Fatal("first sort failed")
	}
	request := notebookSortRequest{OperationID: clientOperationID, Sequences: sequences}
	operationID, _, _, err := notebookOperationIdentity("notebook_sort", owner, first, clientOperationID, request)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := db.GetWorkspaceOperation(context.Background(), owner, operationID)
	if err != nil || receipt.Status != applicationnotes.OperationCommitted {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	var afterFirst info.User
	if err := db.Users.FindId(owner).One(&afterFirst); err != nil || afterFirst.Usn != 12 {
		t.Fatalf("user after first=%+v err=%v", afterFirst, err)
	}
	if !service.SortNotebooks(owner.Hex(), sequences, clientOperationID) {
		t.Fatal("response-loss retry failed")
	}
	var afterRetry info.User
	if err := db.Users.FindId(owner).One(&afterRetry); err != nil || afterRetry.Usn != afterFirst.Usn {
		t.Fatalf("retry allocated another USN: before=%d after=%d err=%v", afterFirst.Usn, afterRetry.Usn, err)
	}
	if got := service.GetNotebook(first.Hex(), owner.Hex()); got.Usn != receipt.StepUSNs["notebook:"+first.Hex()] || got.Seq != 1 {
		t.Fatalf("first notebook=%+v receipt=%+v", got, receipt)
	}
}

func TestSortNotebooksClientOperationResumesPartialReceiptWithoutRepeatingAppliedStep(t *testing.T) {
	useNotebookReceiptTestDatabase(t)
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	first, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	second, _ := domain.ParseObjectID("507f1f77bcf86cd799439013")
	originals := map[string]info.Notebook{
		first.Hex():  {NotebookId: first, UserId: owner, Seq: 0, Usn: 5},
		second.Hex(): {NotebookId: second, UserId: owner, Seq: 1, Usn: 6},
	}
	if err := db.Users.Insert(info.User{UserId: owner, Usn: 22}); err != nil {
		t.Fatal(err)
	}
	if err := db.Notebooks.Insert(
		info.Notebook{NotebookId: first, UserId: owner, Seq: 1, Usn: 22},
		originals[second.Hex()],
	); err != nil {
		t.Fatal(err)
	}
	sequences := map[string]int{first.Hex(): 1, second.Hex(): 0}
	const clientOperationID = "sort-partial-resume"
	request := notebookSortRequest{OperationID: clientOperationID, Sequences: sequences}
	operationID, digest, desired, err := notebookOperationIdentity("notebook_sort", owner, first, clientOperationID, request)
	if err != nil {
		t.Fatal(err)
	}
	beforeState, _ := json.Marshal(originals)
	now := time.Now()
	store := db.MongoWorkspaceOperationStore{}
	initial, err := store.Begin(context.Background(), applicationnotes.OperationReceipt{
		OperationID: operationID, OwnerID: owner, ResourceID: first, Kind: "notebook_sort",
		InputDigest: digest, BeforeState: beforeState, DesiredState: desired,
		StepNames: []string{"notebook:" + first.Hex(), "notebook:" + second.Hex()},
		Status:    applicationnotes.OperationPending, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := store.Claim(context.Background(), owner, initial.OperationID, now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	claimed.Status = applicationnotes.OperationRepairPending
	claimed.AppliedSteps = []string{"notebook:" + first.Hex()}
	claimed.StepUSNs = map[string]int{"notebook:" + first.Hex(): 22}
	claimed.FailedStep = "notebook:" + second.Hex()
	if _, err := store.Save(context.Background(), claimed); err != nil {
		t.Fatal(err)
	}

	service := NotebookService{}
	if !service.SortNotebooks(owner.Hex(), sequences, clientOperationID) {
		t.Fatal("partial receipt did not resume")
	}
	receipt, err := db.GetWorkspaceOperation(context.Background(), owner, operationID)
	if err != nil || receipt.Status != applicationnotes.OperationCommitted {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	if receipt.StepUSNs["notebook:"+first.Hex()] != 22 || receipt.StepUSNs["notebook:"+second.Hex()] != 23 {
		t.Fatalf("step USNs=%v", receipt.StepUSNs)
	}
	var user info.User
	if err := db.Users.FindId(owner).One(&user); err != nil || user.Usn != 23 {
		t.Fatalf("user=%+v err=%v", user, err)
	}
	if got := service.GetNotebook(first.Hex(), owner.Hex()); got.Usn != 22 {
		t.Fatalf("already-applied step ran again: %+v", got)
	}
	if !service.SortNotebooks(owner.Hex(), sequences, clientOperationID) {
		t.Fatal("committed retry failed")
	}
	var afterRetry info.User
	if err := db.Users.FindId(owner).One(&afterRetry); err != nil || afterRetry.Usn != 23 {
		t.Fatalf("committed retry allocated USN: %+v err=%v", afterRetry, err)
	}
}

func TestDragNotebooksClientOperationRetryAfterResponseLossDoesNotRepeatWrites(t *testing.T) {
	useNotebookReceiptTestDatabase(t)
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	current, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	parent, _ := domain.ParseObjectID("507f1f77bcf86cd799439013")
	sibling, _ := domain.ParseObjectID("507f1f77bcf86cd799439014")
	if err := db.Users.Insert(info.User{UserId: owner, Usn: 10}); err != nil {
		t.Fatal(err)
	}
	if err := db.Notebooks.Insert(
		info.Notebook{NotebookId: current, UserId: owner, Seq: 0, Usn: 3},
		info.Notebook{NotebookId: parent, UserId: owner, Seq: 2, Usn: 7},
		info.Notebook{NotebookId: sibling, UserId: owner, Seq: 1, Usn: 4},
	); err != nil {
		t.Fatal(err)
	}
	const clientOperationID = "drag-response-loss"
	siblings := []string{current.Hex(), sibling.Hex()}
	service := NotebookService{}
	if !service.DragNotebooks(owner.Hex(), current.Hex(), parent.Hex(), siblings, clientOperationID) {
		t.Fatal("first drag failed")
	}
	request := notebookDragRequest{OperationID: clientOperationID, Current: current.Hex(), Parent: parent.Hex(), Siblings: siblings}
	operationID, _, _, err := notebookOperationIdentity("notebook_drag", owner, current, clientOperationID, request)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := db.GetWorkspaceOperation(context.Background(), owner, operationID)
	if err != nil || receipt.Status != applicationnotes.OperationCommitted || receipt.StepUSNs["parent"] != 7 {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	var afterFirst info.User
	if err := db.Users.FindId(owner).One(&afterFirst); err != nil || afterFirst.Usn != 12 {
		t.Fatalf("user after first=%+v err=%v", afterFirst, err)
	}
	if !service.DragNotebooks(owner.Hex(), current.Hex(), parent.Hex(), siblings, clientOperationID) {
		t.Fatal("response-loss retry failed")
	}
	var afterRetry info.User
	if err := db.Users.FindId(owner).One(&afterRetry); err != nil || afterRetry.Usn != afterFirst.Usn {
		t.Fatalf("retry allocated another USN: before=%d after=%d err=%v", afterFirst.Usn, afterRetry.Usn, err)
	}
	got := service.GetNotebook(current.Hex(), owner.Hex())
	if got.ParentNotebookId != parent || got.Usn != receipt.StepUSNs["notebook:"+current.Hex()] {
		t.Fatalf("current notebook=%+v receipt=%+v", got, receipt)
	}
}

func TestDragNotebooksClientOperationResumesPartialReceiptWithoutRepeatingAppliedStep(t *testing.T) {
	useNotebookReceiptTestDatabase(t)
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	current, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	parent, _ := domain.ParseObjectID("507f1f77bcf86cd799439013")
	sibling, _ := domain.ParseObjectID("507f1f77bcf86cd799439014")
	originals := notebookDragBeforeState{
		Targets: map[string]info.Notebook{
			current.Hex(): {NotebookId: current, UserId: owner, Seq: 0, Usn: 5},
			sibling.Hex(): {NotebookId: sibling, UserId: owner, Seq: 1, Usn: 6},
		},
		Parent: &info.Notebook{NotebookId: parent, UserId: owner, Seq: 2, Usn: 7},
	}
	if err := db.Users.Insert(info.User{UserId: owner, Usn: 22}); err != nil {
		t.Fatal(err)
	}
	if err := db.Notebooks.Insert(
		info.Notebook{NotebookId: current, UserId: owner, ParentNotebookId: parent, Seq: 0, Usn: 22},
		*originals.Parent,
		originals.Targets[sibling.Hex()],
	); err != nil {
		t.Fatal(err)
	}
	const clientOperationID = "drag-partial-resume"
	siblings := []string{current.Hex(), sibling.Hex()}
	request := notebookDragRequest{OperationID: clientOperationID, Current: current.Hex(), Parent: parent.Hex(), Siblings: siblings}
	operationID, digest, desired, err := notebookOperationIdentity("notebook_drag", owner, current, clientOperationID, request)
	if err != nil {
		t.Fatal(err)
	}
	beforeState, _ := json.Marshal(originals)
	now := time.Now()
	store := db.MongoWorkspaceOperationStore{}
	initial, err := store.Begin(context.Background(), applicationnotes.OperationReceipt{
		OperationID: operationID, OwnerID: owner, ResourceID: current, Kind: "notebook_drag",
		InputDigest: digest, BeforeState: beforeState, DesiredState: desired,
		StepNames: []string{"parent", "notebook:" + current.Hex(), "notebook:" + sibling.Hex()},
		Status:    applicationnotes.OperationPending, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := store.Claim(context.Background(), owner, initial.OperationID, now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	claimed.Status = applicationnotes.OperationRepairPending
	claimed.AppliedSteps = []string{"parent", "notebook:" + current.Hex()}
	claimed.StepUSNs = map[string]int{"parent": 7, "notebook:" + current.Hex(): 22}
	claimed.FailedStep = "notebook:" + sibling.Hex()
	if _, err := store.Save(context.Background(), claimed); err != nil {
		t.Fatal(err)
	}

	service := NotebookService{}
	if !service.DragNotebooks(owner.Hex(), current.Hex(), parent.Hex(), siblings, clientOperationID) {
		t.Fatal("partial drag receipt did not resume")
	}
	receipt, err := db.GetWorkspaceOperation(context.Background(), owner, operationID)
	if err != nil || receipt.Status != applicationnotes.OperationCommitted {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	if receipt.StepUSNs["parent"] != 7 || receipt.StepUSNs["notebook:"+current.Hex()] != 22 || receipt.StepUSNs["notebook:"+sibling.Hex()] != 23 {
		t.Fatalf("step USNs=%v", receipt.StepUSNs)
	}
	var user info.User
	if err := db.Users.FindId(owner).One(&user); err != nil || user.Usn != 23 {
		t.Fatalf("user=%+v err=%v", user, err)
	}
	if !service.DragNotebooks(owner.Hex(), current.Hex(), parent.Hex(), siblings, clientOperationID) {
		t.Fatal("committed drag retry failed")
	}
	var afterRetry info.User
	if err := db.Users.FindId(owner).One(&afterRetry); err != nil || afterRetry.Usn != 23 {
		t.Fatalf("committed retry allocated USN: %+v err=%v", afterRetry, err)
	}
}

func TestNotebookClientOperationIdentityKeepsReceiptStableAcrossRetry(t *testing.T) {
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	resource, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	request := notebookSortRequest{
		OperationID: "sort-after-response-loss",
		Sequences:   map[string]int{resource.Hex(): 4},
	}
	firstID, firstDigest, _, err := notebookOperationIdentity("notebook_sort", owner, resource, request.OperationID, request)
	if err != nil {
		t.Fatal(err)
	}
	// A retry observes a different generation, but must address the original receipt.
	retryID, retryDigest, _, err := notebookOperationIdentity("notebook_sort", owner, resource, request.OperationID, request)
	if err != nil {
		t.Fatal(err)
	}
	if firstID != retryID || firstDigest != retryDigest {
		t.Fatalf("retry identity changed: first=(%s,%s) retry=(%s,%s)", firstID, firstDigest, retryID, retryDigest)
	}
	request.Sequences[resource.Hex()] = 5
	changedInputID, changedInputDigest, _, err := notebookOperationIdentity("notebook_sort", owner, resource, request.OperationID, request)
	if err != nil || changedInputID != firstID || changedInputDigest == firstDigest {
		t.Fatalf("changed input must keep receipt key and change digest: id=%q digest=%q err=%v", changedInputID, changedInputDigest, err)
	}

	request.OperationID = "sort-different-operation"
	_, changedDigest, _, err := notebookOperationIdentity("notebook_sort", owner, resource, request.OperationID, request)
	if err != nil {
		t.Fatal(err)
	}
	if changedDigest == firstDigest {
		t.Fatal("client operation ID was omitted from the input digest")
	}
}

func TestNotebookDragClientOperationIdentityIsStableAndBindsOperationID(t *testing.T) {
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	resource, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	request := notebookDragRequest{
		OperationID: "drag-after-response-loss",
		Current:     resource.Hex(),
		Parent:      "507f1f77bcf86cd799439013",
		Siblings:    []string{resource.Hex()},
	}
	firstID, firstDigest, _, err := notebookOperationIdentity("notebook_drag", owner, resource, request.OperationID, request)
	if err != nil {
		t.Fatal(err)
	}
	retryID, retryDigest, _, err := notebookOperationIdentity("notebook_drag", owner, resource, request.OperationID, request)
	if err != nil || firstID != retryID || firstDigest != retryDigest {
		t.Fatalf("drag retry identity changed: first=(%s,%s) retry=(%s,%s) err=%v", firstID, firstDigest, retryID, retryDigest, err)
	}
	request.Parent = "507f1f77bcf86cd799439014"
	changedInputID, changedInputDigest, _, err := notebookOperationIdentity("notebook_drag", owner, resource, request.OperationID, request)
	if err != nil || changedInputID != firstID || changedInputDigest == firstDigest {
		t.Fatalf("changed drag input must keep receipt key and change digest: id=%q digest=%q err=%v", changedInputID, changedInputDigest, err)
	}
	request.OperationID = "drag-different-operation"
	_, changedDigest, _, err := notebookOperationIdentity("notebook_drag", owner, resource, request.OperationID, request)
	if err != nil {
		t.Fatal(err)
	}
	if changedDigest == firstDigest {
		t.Fatal("drag operation ID was omitted from the input digest")
	}
}

func TestNotebookClientOperationBeforeStateFreezesGenerationsForPartialResume(t *testing.T) {
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	resource, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	before := map[string]info.Notebook{resource.Hex(): {NotebookId: resource, Usn: 17}}
	payload, err := json.Marshal(before)
	if err != nil {
		t.Fatal(err)
	}
	var restored map[string]info.Notebook
	if err := json.Unmarshal(payload, &restored); err != nil {
		t.Fatal(err)
	}
	if restored[resource.Hex()].Usn != 17 {
		t.Fatalf("frozen before state lost generation: %+v", restored)
	}
	request := notebookSortRequest{OperationID: "sort-partial-resume", Sequences: map[string]int{resource.Hex(): 3}}
	firstID, _, _, err := notebookOperationIdentity("notebook_sort", owner, resource, request.OperationID, request)
	if err != nil {
		t.Fatal(err)
	}
	retryID, _, _, err := notebookOperationIdentity("notebook_sort", owner, resource, request.OperationID, request)
	if err != nil || firstID != retryID {
		t.Fatalf("partial resume receipt=%q retry=%q err=%v", firstID, retryID, err)
	}
}

func TestCommittedTagDetachReceiptRetainsPublicUSNResult(t *testing.T) {
	receipt := applicationnotes.OperationReceipt{
		AppliedSteps: []string{"tag_tombstone", "note:507f1f77bcf86cd799439011"},
		StepUSNs:     map[string]int{"tag_tombstone": 8, "note:507f1f77bcf86cd799439011": 9},
	}
	got := tagDetachResultFromReceipt(receipt)
	if got["507f1f77bcf86cd799439011"] != 9 {
		t.Fatalf("result=%v", got)
	}
}

func TestNotebookCountProjectionTreatsTombstoneAsSatisfied(t *testing.T) {
	if !notebookCountProjectionSatisfied(info.Notebook{IsDeleted: true}) {
		t.Fatal("tombstoned notebook count does not need a live projection")
	}
	if notebookCountProjectionSatisfied(info.Notebook{IsDeleted: false}) {
		t.Fatal("active notebook count still requires projection")
	}
}

func TestFrozenDragParentMustMatchRequestedActiveGeneration(t *testing.T) {
	parentID, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	parent := &info.Notebook{NotebookId: parentID, Usn: 7}
	if !frozenParentMatchesRequest(parent, parentID.Hex()) {
		t.Fatal("matching frozen parent was rejected")
	}
	deleted := *parent
	deleted.IsDeleted = true
	if frozenParentMatchesRequest(&deleted, parentID.Hex()) {
		t.Fatal("deleted frozen parent was accepted")
	}
	if frozenParentMatchesRequest(nil, parentID.Hex()) {
		t.Fatal("missing frozen parent was accepted")
	}
	if !frozenParentMatchesRequest(nil, "") {
		t.Fatal("root drag unexpectedly requires a parent")
	}
}
