package db

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	applicationnotes "github.com/yangphere/leanote/app/application/notes"
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
	for _, collectionName := range collectionNames {
		models := modelsByCollection[collectionName]
		for _, model := range models {
			if err := preflightUniqueIndex(ctx, database.Collection(collectionName), collectionName, model); err != nil {
				return err
			}
		}
	}
	for _, collectionName := range collectionNames {
		models := modelsByCollection[collectionName]
		if _, err := database.Collection(collectionName).Indexes().CreateMany(ctx, models); err != nil {
			return fmt.Errorf("create %s persistence indexes: %w", collectionName, err)
		}
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
