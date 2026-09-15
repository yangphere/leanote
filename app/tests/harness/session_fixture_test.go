package harness

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestSessionFixtureHasExpectedShapeAndUniqueNonEmptyStringSessionIDs(t *testing.T) {
	repoRoot, err := findRepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}

	fixturePath := filepath.Join(repoRoot, "mongodb_backup", "leanote_install_data", "sessions.bson")
	fixture, err := os.Open(fixturePath)
	if err != nil {
		t.Fatalf("open session fixture: %v", err)
	}
	defer fixture.Close()

	const (
		expectedDocumentCount = 26
		retainedRecordID      = "5447a23b99c37b159d000004"
		removedDuplicateID    = "5447ba6099c37b19d0000001"
	)

	seen := make(map[string]string)
	recordIDs := make(map[string]struct{})
	documentCount := 0
	for documentIndex := 0; ; documentIndex++ {
		document, err := bson.ReadDocument(fixture)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("read session fixture document %d: %v", documentIndex, err)
		}
		documentCount++
		recordIDs[fixtureRecordID(document, documentIndex)] = struct{}{}

		sessionID, ok := document.Lookup("SessionId").StringValueOK()
		if !ok || sessionID == "" {
			continue
		}

		recordID := fixtureRecordID(document, documentIndex)
		if firstRecordID, exists := seen[sessionID]; exists {
			t.Errorf("duplicate non-empty string SessionId in records %s and %s", firstRecordID, recordID)
			continue
		}
		seen[sessionID] = recordID
	}

	if documentCount != expectedDocumentCount {
		t.Errorf("session fixture document count = %d, want %d", documentCount, expectedDocumentCount)
	}
	if _, exists := recordIDs[retainedRecordID]; !exists {
		t.Errorf("session fixture is missing retained record %s", retainedRecordID)
	}
	if _, exists := recordIDs[removedDuplicateID]; exists {
		t.Errorf("session fixture still contains removed duplicate record %s", removedDuplicateID)
	}
}

func fixtureRecordID(document bson.Raw, documentIndex int) string {
	if id, ok := document.Lookup("_id").ObjectIDOK(); ok {
		return id.Hex()
	}
	return fmt.Sprintf("document[%d]", documentIndex)
}
