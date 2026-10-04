package db

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ShareSchemaConflictError reports historical documents that cannot satisfy
// the share grant contract. Startup must stop until these records are repaired.
type ShareSchemaConflictError struct {
	Collection string
	Reason     string
	Count      int64
}

func (e *ShareSchemaConflictError) Error() string {
	return fmt.Sprintf("share schema preflight found %d invalid documents for %s (%s)", e.Count, e.Collection, e.Reason)
}

func shareGrantValidator(resource string) bson.M {
	zeroID := bson.ObjectID{}
	return bson.M{
		"$and": []bson.M{
			{"$jsonSchema": bson.M{
				"bsonType": "object",
				"required": []string{"UserId", resource, "Perm"},
				"properties": bson.M{
					"UserId":    bson.M{"bsonType": "objectId"},
					resource:    bson.M{"bsonType": "objectId"},
					"ToUserId":  bson.M{"bsonType": "objectId"},
					"ToGroupId": bson.M{"bsonType": "objectId"},
					"Perm":      bson.M{"bsonType": "int", "enum": []int32{0, 1}},
					"ExpiresAt": bson.M{"bsonType": []string{"date", "null"}},
				},
			}},
			{"$expr": bson.M{"$and": []bson.M{
				shareValidRecipientExpression(),
				{"$ne": []interface{}{"$UserId", zeroID}},
				{"$ne": []interface{}{"$" + resource, zeroID}},
			}}},
		},
	}
}

func shareInvalidDocumentQuery(validator bson.M) bson.M {
	return bson.M{"$nor": []bson.M{validator}}
}

func shareValidRecipientExpression() bson.M {
	zeroID := bson.ObjectID{}
	return bson.M{"$or": []bson.M{
		{"$and": []bson.M{
			{"$eq": []interface{}{bson.M{"$type": "$ToUserId"}, "objectId"}},
			{"$eq": []interface{}{bson.M{"$type": "$ToGroupId"}, "missing"}},
			{"$ne": []interface{}{"$ToUserId", zeroID}},
		}},
		{"$and": []bson.M{
			{"$eq": []interface{}{bson.M{"$type": "$ToGroupId"}, "objectId"}},
			{"$eq": []interface{}{bson.M{"$type": "$ToUserId"}, "missing"}},
			{"$ne": []interface{}{"$ToGroupId", zeroID}},
		}},
	}}
}

func ShareCollectionValidators() map[string]bson.M {
	return map[string]bson.M{
		"share_notes":     shareGrantValidator("NoteId"),
		"share_notebooks": shareGrantValidator("NotebookId"),
	}
}

func preflightShareDocuments(ctx context.Context) error {
	for collectionName, validator := range ShareCollectionValidators() {
		collection := database.Collection(collectionName)
		count, err := collection.CountDocuments(ctx, shareInvalidDocumentQuery(validator))
		if err != nil {
			return fmt.Errorf("preflight %s schema: %w", collectionName, err)
		}
		if count > 0 {
			return &ShareSchemaConflictError{Collection: collectionName, Reason: "document violates share grant validator", Count: count}
		}
	}
	return nil
}

func ensureShareValidators(ctx context.Context) error {
	for collectionName, validator := range ShareCollectionValidators() {
		createErr := database.CreateCollection(ctx, collectionName, options.CreateCollection().
			SetValidator(validator).
			SetValidationLevel("strict").
			SetValidationAction("error"))
		if createErr == nil {
			continue
		}
		if err := database.RunCommand(ctx, bson.D{
			{Key: "collMod", Value: collectionName},
			{Key: "validator", Value: validator},
			{Key: "validationLevel", Value: "strict"},
			{Key: "validationAction", Value: "error"},
		}).Err(); err != nil {
			return fmt.Errorf("configure %s validator: create=%v collMod=%w", collectionName, createErr, err)
		}
	}
	return nil
}
