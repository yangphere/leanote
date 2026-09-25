package db

import (
	"reflect"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestPersistenceIndexModelsDeclareShareBoundaries(t *testing.T) {
	models := PersistenceIndexModels()
	if len(models["user_blogs"]) != 1 {
		t.Fatalf("user_blogs custom-domain index missing: %#v", models["user_blogs"])
	}
	customDomainOptions := indexOptions(models["user_blogs"][0])
	if customDomainOptions.Name == nil || *customDomainOptions.Name != "user_blogs_Domain_unique" || customDomainOptions.Unique == nil || !*customDomainOptions.Unique {
		t.Fatalf("custom-domain index options=%+v", customDomainOptions)
	}
	want := map[string][]string{
		"share_notes":     {"share_notes_owner_note_person_unique", "share_notes_owner_note_group_unique", "share_notes_recipient_person_owner_note", "share_notes_recipient_group_owner_note"},
		"share_notebooks": {"share_notebooks_owner_notebook_person_unique", "share_notebooks_owner_notebook_group_unique", "share_notebooks_recipient_person_owner_notebook", "share_notebooks_recipient_group_owner_notebook"},
		"has_share_notes": {"has_share_notes_owner_recipient_unique"},
	}
	for collection, names := range want {
		seen := map[string]bool{}
		for _, model := range models[collection] {
			indexOptions := indexOptions(model)
			if indexOptions.Name != nil {
				seen[*indexOptions.Name] = true
			}
		}
		for _, name := range names {
			if !seen[name] {
				t.Errorf("%s missing index %q", collection, name)
			}
		}
	}
	if got := indexKeys(models["share_notes"][0]); len(got) != 3 || got[0] != "UserId" || got[1] != "NoteId" || got[2] != "ToUserId" {
		t.Fatalf("share note personal keys=%v", got)
	}
	for _, model := range models["share_notes"][:2] {
		indexOptions := indexOptions(model)
		if indexOptions.Unique == nil || !*indexOptions.Unique || indexOptions.PartialFilterExpression == nil {
			t.Fatalf("share grant index options=%+v", indexOptions)
		}
	}
}

func TestShareValidatorsRequireExactlyOneRecipient(t *testing.T) {
	validators := ShareCollectionValidators()
	for _, collection := range []string{"share_notes", "share_notebooks"} {
		validator, ok := validators[collection]
		if !ok {
			t.Fatalf("validator missing for %s", collection)
		}
		if _, ok := validator["$and"]; !ok {
			t.Fatalf("validator for %s lacks conjunction: %#v", collection, validator)
		}
	}
	for _, collection := range []string{"share_notes", "share_notebooks"} {
		validator := validators[collection]
		properties := validator["$and"].([]bson.M)[0]["$jsonSchema"].(bson.M)["properties"].(bson.M)
		expiresAt := properties["ExpiresAt"].(bson.M)
		bsonTypes := expiresAt["bsonType"].([]string)
		if len(bsonTypes) != 2 || bsonTypes[0] != "date" || bsonTypes[1] != "null" {
			t.Fatalf("validator for %s has invalid ExpiresAt types: %#v", collection, bsonTypes)
		}
	}
	for collection, validator := range validators {
		filter := shareInvalidDocumentQuery(validator)
		if !reflect.DeepEqual(filter["$nor"], []bson.M{validator}) {
			t.Fatalf("preflight for %s does not reject the complete validator: %#v", collection, filter)
		}
	}
}

func TestShareRecipientExpressionRejectsZeroObjectIDs(t *testing.T) {
	validator := ShareCollectionValidators()["share_notes"]
	expression := validator["$and"].([]bson.M)[1]["$expr"]
	if expression == nil {
		t.Fatal("validator missing shared recipient expression")
	}
	expressionParts := expression.(bson.M)["$and"].([]bson.M)
	if len(expressionParts) != 3 || !reflect.DeepEqual(expressionParts[0], shareValidRecipientExpression()) {
		t.Fatalf("validator does not require recipient and non-zero owner/resource: %#v", expression)
	}
	zero := bson.ObjectID{}
	if !zero.IsZero() {
		t.Fatal("zero ObjectID fixture is not zero")
	}
}
