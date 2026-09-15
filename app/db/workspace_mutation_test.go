package db

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"sync"
	"testing"

	applicationnotes "github.com/yangphere/leanote/app/application/notes"
	"github.com/yangphere/leanote/app/domain"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func TestAllocateUserUSNIsAtomicPerUser(t *testing.T) {
	_, raw := testCollection(t)
	collection := raw.Database().Collection("workspace_usn_users")
	if err := collection.Drop(context.Background()); err != nil {
		t.Fatal(err)
	}
	owner := workspaceTestOwner(t)
	if _, err := collection.InsertOne(context.Background(), map[string]any{"_id": owner, "Usn": 0}); err != nil {
		t.Fatal(err)
	}
	saved := Users
	Users = wrapCollection(collection)
	t.Cleanup(func() { Users = saved })

	const workers = 32
	values := make([]int, workers)
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for i := range workers {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			values[index], errs[index] = AllocateUserUSN(context.Background(), owner)
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	sort.Ints(values)
	for index, value := range values {
		if value != index+1 {
			t.Fatalf("values=%v", values)
		}
	}
}

func workspaceTestOwner(t *testing.T) domain.ObjectID {
	t.Helper()
	id, err := domain.ParseObjectID("507f1f77bcf86cd799439011")
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestExecuteWorkspaceMutationUsesTransactionAndResetsRetryState(t *testing.T) {
	var applied []string
	attempt := 0
	plan := WorkspaceMutationPlan{OwnerID: workspaceTestOwner(t), Steps: []WorkspaceMutationStep{
		{Name: "note", Apply: func(context.Context) error { applied = append(applied, "note"); return nil }},
		{Name: "content", Apply: func(context.Context) error { applied = append(applied, "content"); return nil }},
	}}
	runner := func(ctx context.Context, apply func(context.Context) error) error {
		attempt++
		if err := apply(ctx); err != nil {
			return err
		}
		attempt++
		return apply(ctx)
	}
	result, err := ExecuteWorkspaceMutation(context.Background(), plan, runner)
	if err != nil || !result.Committed || result.Mode != "transaction" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if attempt != 2 || !reflect.DeepEqual(result.AppliedSteps, []string{"note", "content"}) {
		t.Fatalf("attempt=%d appliedSteps=%v", attempt, result.AppliedSteps)
	}
}

func TestExecuteWorkspaceMutationCompensatesWithoutRollingBackAllocator(t *testing.T) {
	var events []string
	plan := WorkspaceMutationPlan{OwnerID: workspaceTestOwner(t), Steps: []WorkspaceMutationStep{
		{Name: "usn", Apply: func(context.Context) error { events = append(events, "allocate"); return nil }},
		{Name: "note", Apply: func(context.Context) error { events = append(events, "insert-note"); return nil }, Compensate: func(context.Context) error { events = append(events, "remove-note"); return nil }},
		{Name: "content", Apply: func(context.Context) error { return errors.New("insert failed") }},
	}}
	result, err := ExecuteWorkspaceMutation(context.Background(), plan, nil)
	if !errors.Is(err, ErrPartialWrite) || !result.PartialWrite || result.FailedStep != "content" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if !reflect.DeepEqual(events, []string{"allocate", "insert-note", "remove-note"}) {
		t.Fatalf("events=%v", events)
	}
}

func TestExecuteWorkspaceMutationDoesNotFallbackOnTransientTransactionError(t *testing.T) {
	called := false
	plan := WorkspaceMutationPlan{OwnerID: workspaceTestOwner(t), Steps: []WorkspaceMutationStep{{
		Name: "note", Apply: func(context.Context) error { called = true; return nil },
	}}}
	runner := func(context.Context, func(context.Context) error) error {
		return mongo.CommandError{Code: 251, Message: "NoSuchTransaction"}
	}
	result, err := ExecuteWorkspaceMutation(context.Background(), plan, runner)
	if err == nil || called || result.Mode != "transaction" {
		t.Fatalf("result=%+v called=%v err=%v", result, called, err)
	}
}

func TestExecuteWorkspaceMutationPreservesFailureBeforeAnyWrite(t *testing.T) {
	want := errors.New("allocate failed")
	plan := WorkspaceMutationPlan{OwnerID: workspaceTestOwner(t), Steps: []WorkspaceMutationStep{{
		Name: "allocate_usn",
		Apply: func(context.Context) error {
			return want
		},
	}}}
	result, err := ExecuteWorkspaceMutation(context.Background(), plan, nil)
	if !errors.Is(err, want) || errors.Is(err, ErrPartialWrite) || result.PartialWrite {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestRunWorkspaceMutationCommitsDurableReceiptWithBusinessWrites(t *testing.T) {
	_, raw := testCollection(t)
	databaseRef := raw.Database()
	users := databaseRef.Collection("workspace_run_users")
	operations := databaseRef.Collection("workspace_run_operations")
	resources := databaseRef.Collection("workspace_run_resources")
	for _, collection := range []*mongo.Collection{users, operations, resources} {
		if err := collection.Drop(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	owner := workspaceTestOwner(t)
	if _, err := users.InsertOne(context.Background(), map[string]any{"_id": owner, "Usn": 0}); err != nil {
		t.Fatal(err)
	}
	savedClient, savedUsers, savedOperations := client, Users, WorkspaceOperations
	client = databaseRef.Client()
	Users = wrapCollection(users)
	WorkspaceOperations = wrapCollection(operations)
	t.Cleanup(func() { client, Users, WorkspaceOperations = savedClient, savedUsers, savedOperations })

	assigned := 0
	plan := WorkspaceMutationPlan{
		OperationID: "note_create:durable", OwnerID: owner, ResourceID: owner,
		Kind: "note_create", InputDigest: "digest", DesiredState: []byte(`{"title":"durable"}`),
		AssignedUSN: func() int { return assigned }, RestoreAssignedUSN: func(usn int) { assigned = usn },
		Steps: []WorkspaceMutationStep{
			{Name: "allocate_usn", Apply: func(ctx context.Context) error {
				var err error
				assigned, err = AllocateUserUSN(ctx, owner)
				return err
			}, Verify: func(context.Context) (bool, error) { return false, nil }},
			{Name: "resource", Apply: func(ctx context.Context) error {
				_, err := resources.InsertOne(ctx, map[string]any{"_id": owner, "UserId": owner, "Usn": assigned})
				return err
			}, Verify: func(ctx context.Context) (bool, error) {
				err := resources.FindOne(ctx, map[string]any{"_id": owner, "UserId": owner, "Usn": assigned}).Err()
				if errors.Is(err, mongo.ErrNoDocuments) {
					return false, nil
				}
				return err == nil, err
			}, Compensate: func(ctx context.Context) error {
				_, err := resources.DeleteOne(ctx, map[string]any{"_id": owner, "UserId": owner})
				return err
			}},
		},
	}
	result, err := RunWorkspaceMutation(context.Background(), plan)
	if err != nil || !result.Committed || assigned != 1 {
		t.Fatalf("result=%+v assigned=%d err=%v", result, assigned, err)
	}
	receipt, err := GetWorkspaceOperation(context.Background(), owner, plan.OperationID)
	if err != nil || receipt.Status != applicationnotes.OperationCommitted || receipt.AssignedUSN != assigned {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	second, err := RunWorkspaceMutation(context.Background(), plan)
	if err != nil || !second.Committed || assigned != 1 {
		t.Fatalf("second=%+v assigned=%d err=%v", second, assigned, err)
	}
}

func TestApplicationMutationPlanPreservesFailurePolicy(t *testing.T) {
	plan := WorkspaceMutationPlan{
		OperationID: "projection:policy", OwnerID: workspaceTestOwner(t), Kind: "projection", InputDigest: "digest",
		FailurePolicy: applicationnotes.FailurePending,
		Steps:         []WorkspaceMutationStep{{Name: "projection", Apply: func(context.Context) error { return nil }}},
	}
	converted := applicationMutationPlan(plan)
	if converted.FailurePolicy != applicationnotes.FailurePending {
		t.Fatalf("FailurePolicy=%q want %q", converted.FailurePolicy, applicationnotes.FailurePending)
	}
}
