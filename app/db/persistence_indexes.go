package db

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
	"time"

	applicationnotes "github.com/yangphere/leanote/app/application/notes"
	"github.com/yangphere/leanote/app/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	SessionIdleTTL                          = 3 * time.Hour
	SessionTTLSeconds                 int32 = 10800
	WorkspaceOperationTerminalTTL           = 30 * 24 * time.Hour
	WorkspaceOperationTerminalSeconds int32 = 30 * 24 * 60 * 60
)

// PersistenceIndexConflictError makes startup failure actionable when a new
// unique index would reject existing records.  Recovery is an explicit data
// repair operation; startup never deletes or silently excludes conflicts.
type PersistenceIndexConflictError struct {
	Collection string
	Index      string
	Count      int64
}

func (e *PersistenceIndexConflictError) Error() string {
	return fmt.Sprintf("persistence index preflight found %d duplicate groups for %s.%s", e.Count, e.Collection, e.Index)
}

type CustomDomainPreflightError struct {
	InvalidCount    int64
	DuplicateGroups int64
	KeyDigests      []string
}

func (e *CustomDomainPreflightError) Error() string {
	return fmt.Sprintf("user_blogs.Domain preflight found %d invalid or noncanonical values and %d canonical duplicate groups (key digests: %v)", e.InvalidCount, e.DuplicateGroups, e.KeyDigests)
}

// PersistenceIndexModels is the single source of truth for indexes owned by
// the persistence boundary. Partial filters keep legacy documents with empty
// identity fields readable while enforcing uniqueness for new identities.
func PersistenceIndexModels() map[string][]mongo.IndexModel {
	return map[string][]mongo.IndexModel{
		"sessions": {
			{
				Keys: bson.D{{Key: "SessionId", Value: 1}},
				Options: options.Index().
					SetName("sessions_SessionId_unique").
					SetUnique(true).
					SetPartialFilterExpression(nonEmptyStringFilter("SessionId")),
			},
			{
				Keys: bson.D{{Key: "UpdatedTime", Value: 1}},
				Options: options.Index().
					SetName("sessions_UpdatedTime_ttl_10800").
					SetExpireAfterSeconds(SessionTTLSeconds),
			},
		},
		"tokens": {
			{
				Keys: bson.D{{Key: "UserId", Value: 1}, {Key: "Type", Value: 1}},
				Options: options.Index().
					SetName("tokens_UserId_Type_unique").
					SetUnique(true).
					SetPartialFilterExpression(bson.M{
						"UserId": bson.M{"$exists": true},
						"Type":   bson.M{"$exists": true},
					}),
			},
		},
		"outbox": {
			{
				Keys:    bson.D{{Key: "Status", Value: 1}, {Key: "NextAttemptAt", Value: 1}},
				Options: options.Index().SetName("outbox_Status_NextAttemptAt"),
			},
			{
				Keys:    bson.D{{Key: "Kind", Value: 1}, {Key: "AggregateId", Value: 1}, {Key: "Status", Value: 1}},
				Options: options.Index().SetName("outbox_Kind_AggregateId_Status"),
			},
			{
				Keys: bson.D{{Key: "IdempotencyKey", Value: 1}},
				Options: options.Index().
					SetName("outbox_IdempotencyKey_unique").
					SetUnique(true).
					SetPartialFilterExpression(nonEmptyStringFilter("IdempotencyKey")),
			},
		},
		"workspace_operations": {
			{
				Keys:    bson.D{{Key: "Status", Value: 1}, {Key: "LeaseUntil", Value: 1}},
				Options: options.Index().SetName("workspace_operations_Status_LeaseUntil"),
			},
			{
				Keys:    bson.D{{Key: "OwnerId", Value: 1}, {Key: "Kind", Value: 1}, {Key: "Status", Value: 1}, {Key: "CreatedAt", Value: -1}},
				Options: options.Index().SetName("workspace_operations_Owner_Kind_Status_CreatedAt"),
			},
			{
				Keys:    bson.D{{Key: "OwnerId", Value: 1}, {Key: "ResourceId", Value: 1}, {Key: "Kind", Value: 1}},
				Options: options.Index().SetName("workspace_operations_Owner_Resource_Kind"),
			},
			{
				Keys: bson.D{{Key: "TerminalAt", Value: 1}},
				Options: options.Index().
					SetName("workspace_operations_TerminalAt_ttl_30d").
					SetExpireAfterSeconds(WorkspaceOperationTerminalSeconds).
					SetPartialFilterExpression(bson.M{"Status": bson.M{"$in": []applicationnotes.OperationStatus{
						applicationnotes.OperationCommitted,
						applicationnotes.OperationCompensated,
						applicationnotes.OperationFailed,
					}}}),
			},
		},
		"users": {
			{
				Keys: bson.D{{Key: "Email", Value: 1}},
				Options: options.Index().
					SetName("users_Email_unique").
					SetUnique(true).
					SetPartialFilterExpression(nonEmptyStringFilter("Email")),
			},
			{
				Keys: bson.D{{Key: "Username", Value: 1}},
				Options: options.Index().
					SetName("users_Username_unique").
					SetUnique(true).
					SetPartialFilterExpression(nonEmptyStringFilter("Username")),
			},
			{
				Keys: bson.D{{Key: "ThirdType", Value: 1}, {Key: "ThirdUserId", Value: 1}},
				Options: options.Index().
					SetName("users_ThirdType_ThirdUserId_unique").
					SetUnique(true).
					SetPartialFilterExpression(nonEmptyStringFilter("ThirdUserId")),
			},
		},
		"user_blogs": {
			{
				Keys: bson.D{{Key: "Domain", Value: 1}},
				Options: options.Index().
					SetName("user_blogs_Domain_unique").
					SetUnique(true).
					SetPartialFilterExpression(nonEmptyStringFilter("Domain")),
			},
		},
		"share_notes":     shareNoteIndexModels(),
		"share_notebooks": shareNotebookIndexModels(),
		"has_share_notes": {
			{
				Keys: bson.D{{Key: "UserId", Value: 1}, {Key: "ToUserId", Value: 1}},
				Options: options.Index().
					SetName("has_share_notes_owner_recipient_unique").
					SetUnique(true).
					SetPartialFilterExpression(bson.M{
						"UserId":   bson.M{"$type": "objectId"},
						"ToUserId": bson.M{"$type": "objectId"},
					}),
			},
		},
		"comment_submission_receipts": {
			{
				Keys: bson.D{{Key: "ActorId", Value: 1}, {Key: "SubmissionId", Value: 1}},
				Options: options.Index().
					SetName("comment_submission_receipts_actor_submission_unique").
					SetUnique(true).
					SetPartialFilterExpression(bson.M{
						"ActorId":      bson.M{"$type": "objectId"},
						"SubmissionId": bson.M{"$type": "string", "$gt": ""},
					}),
			},
			{
				Keys:    bson.D{{Key: "NoteId", Value: 1}, {Key: "Status", Value: 1}},
				Options: options.Index().SetName("comment_submission_receipts_note_status"),
			},
		},
	}
}

func shareNoteIndexModels() []mongo.IndexModel {
	return []mongo.IndexModel{
		shareIndexModel("share_notes_owner_note_person_unique", "UserId", "NoteId", "ToUserId", true),
		shareIndexModel("share_notes_owner_note_group_unique", "UserId", "NoteId", "ToGroupId", true),
		shareIndexModel("share_notes_recipient_person_owner_note", "ToUserId", "UserId", "NoteId", false),
		shareIndexModel("share_notes_recipient_group_owner_note", "ToGroupId", "UserId", "NoteId", false),
	}
}

func shareNotebookIndexModels() []mongo.IndexModel {
	return []mongo.IndexModel{
		shareIndexModel("share_notebooks_owner_notebook_person_unique", "UserId", "NotebookId", "ToUserId", true),
		shareIndexModel("share_notebooks_owner_notebook_group_unique", "UserId", "NotebookId", "ToGroupId", true),
		shareIndexModel("share_notebooks_recipient_person_owner_notebook", "ToUserId", "UserId", "NotebookId", false),
		shareIndexModel("share_notebooks_recipient_group_owner_notebook", "ToGroupId", "UserId", "NotebookId", false),
	}
}

func shareIndexModel(name string, first, second, recipient string, unique bool) mongo.IndexModel {
	return mongo.IndexModel{
		Keys: bson.D{{Key: first, Value: 1}, {Key: second, Value: 1}, {Key: recipient, Value: 1}},
		Options: options.Index().
			SetName(name).
			SetUnique(unique).
			SetPartialFilterExpression(bson.M{recipient: bson.M{"$type": "objectId"}}),
	}
}

func nonEmptyStringFilter(field string) bson.M {
	// Partial indexes accept comparison operators; every non-empty normalized
	// string sorts after the empty string, while missing/non-string legacy
	// values stay outside the uniqueness constraint.
	return bson.M{field: bson.M{"$type": "string", "$gt": ""}}
}

// EnsurePersistenceIndexes idempotently creates all indexes owned by this
// package. It is explicit so tests and deployments can surface duplicate-key
// or incompatible-index failures instead of masking them as readiness.
func EnsurePersistenceIndexes(ctx context.Context) error {
	if database == nil {
		return fmt.Errorf("mongo database is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := boundedOperationContext(ctx)
	defer cancel()

	modelsByCollection := PersistenceIndexModels()
	collectionNames := make([]string, 0, len(modelsByCollection))
	for collectionName := range modelsByCollection {
		collectionNames = append(collectionNames, collectionName)
	}
	sort.Strings(collectionNames)
	if err := preflightCustomDomains(ctx); err != nil {
		return err
	}
	for _, collectionName := range collectionNames {
		models := modelsByCollection[collectionName]
		for _, model := range models {
			if err := preflightUniqueIndex(ctx, database.Collection(collectionName), collectionName, model); err != nil {
				return err
			}
		}
	}
	if err := preflightShareDocuments(ctx); err != nil {
		return err
	}
	for _, collectionName := range collectionNames {
		models := modelsByCollection[collectionName]
		if _, err := database.Collection(collectionName).Indexes().CreateMany(ctx, models); err != nil {
			return fmt.Errorf("create %s persistence indexes: %w", collectionName, err)
		}
	}
	if err := ensureShareValidators(ctx); err != nil {
		return err
	}
	return nil
}

func preflightCustomDomains(ctx context.Context) error {
	collection := database.Collection("user_blogs")
	cursor, err := collection.Find(ctx,
		bson.M{"Domain": bson.M{"$exists": true, "$ne": ""}},
		options.Find().SetProjection(bson.M{"Domain": 1}))
	if err != nil {
		return fmt.Errorf("preflight user_blogs.Domain: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()
	seen := make(map[string]bool)
	duplicates := make(map[string]bool)
	conflict := &CustomDomainPreflightError{}
	addDigest := func(value string) {
		if len(conflict.KeyDigests) < 3 {
			digest := sha256.Sum256([]byte(value))
			conflict.KeyDigests = append(conflict.KeyDigests, fmt.Sprintf("%x", digest[:8]))
		}
	}
	for cursor.Next(ctx) {
		var document struct {
			Domain any `bson:"Domain"`
		}
		if err := cursor.Decode(&document); err != nil {
			return fmt.Errorf("decode user_blogs.Domain preflight: %w", err)
		}
		rawDomain, ok := document.Domain.(string)
		if !ok {
			conflict.InvalidCount++
			addDigest(fmt.Sprintf("%T:%v", document.Domain, document.Domain))
			continue
		}
		canonical, err := domain.CanonicalizeStoredCustomDomain(rawDomain)
		if err != nil || canonical != rawDomain {
			conflict.InvalidCount++
			addDigest(rawDomain)
		}
		if err != nil {
			continue
		}
		if seen[canonical] && !duplicates[canonical] {
			duplicates[canonical] = true
			conflict.DuplicateGroups++
			addDigest(canonical)
		}
		seen[canonical] = true
	}
	if err := cursor.Err(); err != nil {
		return fmt.Errorf("scan user_blogs.Domain preflight: %w", err)
	}
	if conflict.InvalidCount > 0 || conflict.DuplicateGroups > 0 {
		return conflict
	}
	return nil
}

func preflightUniqueIndex(ctx context.Context, collection *mongo.Collection, collectionName string, model mongo.IndexModel) error {
	options, err := persistenceIndexOptions(model)
	if err != nil {
		return fmt.Errorf("read %s index options: %w", collectionName, err)
	}
	if options.Unique == nil || !*options.Unique {
		return nil
	}
	keys, ok := model.Keys.(bson.D)
	if !ok || len(keys) == 0 {
		return fmt.Errorf("read %s unique index keys: unsupported %T", collectionName, model.Keys)
	}
	groupID := bson.D{}
	for _, key := range keys {
		groupID = append(groupID, bson.E{Key: key.Key, Value: "$" + key.Key})
	}
	pipeline := mongo.Pipeline{}
	if options.PartialFilterExpression != nil {
		pipeline = append(pipeline, bson.D{{Key: "$match", Value: options.PartialFilterExpression}})
	}
	pipeline = append(pipeline,
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: groupID},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
		bson.D{{Key: "$match", Value: bson.D{{Key: "count", Value: bson.D{{Key: "$gt", Value: 1}}}}}},
		bson.D{{Key: "$count", Value: "count"}},
	)
	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return fmt.Errorf("preflight %s.%s: %w", collectionName, indexName(options), err)
	}
	defer func() { _ = cursor.Close(ctx) }()
	var result []struct {
		Count int64 `bson:"count"`
	}
	if err := cursor.All(ctx, &result); err != nil {
		return fmt.Errorf("read preflight %s.%s: %w", collectionName, indexName(options), err)
	}
	if len(result) == 0 || result[0].Count == 0 {
		return nil
	}
	return &PersistenceIndexConflictError{Collection: collectionName, Index: indexName(options), Count: result[0].Count}
}

func persistenceIndexOptions(model mongo.IndexModel) (options.IndexOptions, error) {
	var result options.IndexOptions
	if model.Options == nil {
		return result, nil
	}
	for _, apply := range model.Options.List() {
		if err := apply(&result); err != nil {
			return result, err
		}
	}
	return result, nil
}

func indexName(options options.IndexOptions) string {
	if options.Name == nil || *options.Name == "" {
		return "unnamed"
	}
	return *options.Name
}

func IsPersistenceIndexConflict(err error) bool {
	var conflict *PersistenceIndexConflictError
	return errors.As(err, &conflict)
}
