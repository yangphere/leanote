package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// WorkspaceNoteMutationLeaseTTL bounds the time for which a crashed process
// can fence a note.  A retry using the same operation identity may renew it;
// unrelated mutations can proceed after expiry.
const WorkspaceNoteMutationLeaseTTL = 5 * time.Minute

var ErrWorkspaceNoteLeaseUnavailable = errors.New("workspace note mutation lease unavailable")

// AcquireWorkspaceNoteMutationLease atomically reserves the observed note
// generation for one durable operation.  A lease owned by the same operation
// is renewable, which lets a process recover after the required note write
// was applied before its receipt was durably advanced.
func AcquireWorkspaceNoteMutationLease(ctx context.Context, noteID, ownerID domain.ObjectID, expectedUSN int, operationID string, now time.Time) error {
	if Notes == nil {
		return ErrMongoClientNotInitialized
	}
	if noteID.IsZero() || ownerID.IsZero() || expectedUSN <= 0 || operationID == "" {
		return fmt.Errorf("acquire workspace note lease: invalid identity")
	}
	if now.IsZero() {
		now = time.Now()
	}
	filter := bson.M{
		"_id":       noteID,
		"UserId":    ownerID,
		"Usn":       expectedUSN,
		"IsDeleted": false,
	}
	AddWorkspaceNoteMutationLeaseFilter(filter, operationID, now)
	var note info.Note
	opCtx, cancel := boundedOperationContext(ctx)
	defer cancel()
	err := Notes.coll.FindOneAndUpdate(
		opCtx,
		filter,
		bson.M{"$set": bson.M{
			"MutationLeaseId":    operationID,
			"MutationLeaseUntil": now.Add(WorkspaceNoteMutationLeaseTTL),
		}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&note)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return ErrWorkspaceNoteLeaseUnavailable
	}
	if err != nil {
		return fmt.Errorf("acquire workspace note lease: %w", err)
	}
	return nil
}

// RenewWorkspaceNoteMutationLease renews a lease by operation identity after
// a required note write may already have advanced its USN.
func RenewWorkspaceNoteMutationLease(ctx context.Context, noteID, ownerID domain.ObjectID, operationID string, now time.Time) error {
	if Notes == nil {
		return ErrMongoClientNotInitialized
	}
	if noteID.IsZero() || ownerID.IsZero() || operationID == "" {
		return fmt.Errorf("renew workspace note lease: invalid identity")
	}
	if now.IsZero() {
		now = time.Now()
	}
	return Notes.UpdateOneMatchedContext(ctx,
		bson.M{"_id": noteID, "UserId": ownerID, "MutationLeaseId": operationID},
		bson.M{"$set": bson.M{"MutationLeaseUntil": now.Add(WorkspaceNoteMutationLeaseTTL)}},
	)
}

// ReleaseWorkspaceNoteMutationLease removes only the lease owned by the
// operation.  It cannot clear a newer operation's fence.
func ReleaseWorkspaceNoteMutationLease(ctx context.Context, noteID, ownerID domain.ObjectID, operationID string) error {
	if Notes == nil {
		return ErrMongoClientNotInitialized
	}
	if noteID.IsZero() || ownerID.IsZero() || operationID == "" {
		return fmt.Errorf("release workspace note lease: invalid identity")
	}
	return Notes.UpdateOneMatchedContext(ctx,
		bson.M{"_id": noteID, "UserId": ownerID, "MutationLeaseId": operationID},
		bson.M{"$unset": bson.M{"MutationLeaseId": "", "MutationLeaseUntil": ""}},
	)
}

// AddWorkspaceNoteMutationLeaseFilter adds the predicate every note
// mutation must use.  An empty operation ID means only an unleased/expired
// note is writable; a non-empty ID additionally permits that operation's own
// lease.
func AddWorkspaceNoteMutationLeaseFilter(filter bson.M, operationID string, now time.Time) {
	if filter == nil {
		return
	}
	if now.IsZero() {
		now = time.Now()
	}
	allowed := []bson.M{
		{"MutationLeaseId": bson.M{"$exists": false}},
		{"MutationLeaseId": ""},
		{"MutationLeaseUntil": bson.M{"$lte": now}},
	}
	if operationID != "" {
		allowed = append(allowed, bson.M{"MutationLeaseId": operationID})
	}
	filter["$or"] = allowed
}
