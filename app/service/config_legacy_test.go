package service

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestIsLegacyEmptyConfigDocumentRecognizesHistoricalPlaceholder(t *testing.T) {
	document, err := bson.Marshal(bson.D{
		{Key: "_id", Value: bson.NewObjectID()},
		{Key: "UserId", Value: bson.NewObjectID()},
		{Key: "StringConfigs", Value: bson.D{}},
		{Key: "ArrayConfigs", Value: bson.D{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !isLegacyEmptyConfigDocument(document) {
		t.Fatal("historical empty config document was not recognized")
	}
}

func TestIsLegacyEmptyConfigDocumentRejectsNonEmptyOrUnknownDocuments(t *testing.T) {
	tests := []struct {
		name     string
		document bson.D
	}{
		{
			name: "non-empty legacy map",
			document: bson.D{
				{Key: "StringConfigs", Value: bson.D{{Key: "siteUrl", Value: "https://example.test"}}},
				{Key: "ArrayConfigs", Value: bson.D{}},
			},
		},
		{
			name: "unknown field",
			document: bson.D{
				{Key: "StringConfigs", Value: bson.D{}},
				{Key: "ArrayConfigs", Value: bson.D{}},
				{Key: "Key", Value: ""},
			},
		},
		{
			name: "missing array map",
			document: bson.D{
				{Key: "StringConfigs", Value: bson.D{}},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document, err := bson.Marshal(test.document)
			if err != nil {
				t.Fatal(err)
			}
			if isLegacyEmptyConfigDocument(document) {
				t.Fatal("malformed config document was recognized as a legacy placeholder")
			}
		})
	}
}
