package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	applicationnotes "github.com/yangphere/leanote/app/application/notes"
	"github.com/yangphere/leanote/app/domain"
)

func TestAttachNumUpdateRequiresCommittedNoteGenerationForAPIReconcile(t *testing.T) {
	noteID, err := domain.ParseObjectID("507f1f77bcf86cd799439011")
	if err != nil {
		t.Fatal(err)
	}
	ownerID, err := domain.ParseObjectID("507f1f77bcf86cd799439012")
	if err != nil {
		t.Fatal(err)
	}
	expectedUSN := 17
	filter := attachNumUpdateFilter(noteID, ownerID, &expectedUSN)
	if filter["Usn"] != expectedUSN || filter["IsDeleted"] != false {
		t.Fatalf("generation-scoped filter=%v", filter)
	}
}

func TestPublishFileNoClobberRejectsDifferentRetryWithoutOverwriting(t *testing.T) {
	target := filepath.Join(t.TempDir(), "asset.bin")
	if err := os.WriteFile(target, []byte("committed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := publishFileNoClobber(target, []byte("different"), 0600); err == nil {
		t.Fatal("different retry unexpectedly replaced the committed file")
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != "committed" {
		t.Fatalf("target=%q err=%v", got, err)
	}
}

func TestPublishFileNoClobberTreatsSameBytesAsReplay(t *testing.T) {
	target := filepath.Join(t.TempDir(), "asset.bin")
	if err := publishFileNoClobber(target, []byte("same"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := publishFileNoClobber(target, []byte("same"), 0600); err != nil {
		t.Fatalf("same-byte retry failed: %v", err)
	}
}

func TestWebAttachUploadIdentityBindsBytesAndMetadata(t *testing.T) {
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	note, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	attach := StableWebAttachID(owner, note, "upload-1")
	oneID, oneDigest, err := webAttachUploadIdentity(owner, note, attach, "upload-1", "report.txt", "txt", 4, []byte("one"))
	if err != nil {
		t.Fatal(err)
	}
	twoID, twoDigest, err := webAttachUploadIdentity(owner, note, attach, "upload-1", "report.txt", "txt", 4, []byte("two"))
	if err != nil {
		t.Fatal(err)
	}
	if oneID == "" || oneID != twoID || oneDigest == twoDigest {
		t.Fatalf("operation=%q/%q digest=%q/%q", oneID, twoID, oneDigest, twoDigest)
	}
	_, retryDigest, err := webAttachUploadIdentity(owner, note, attach, "upload-1", "report.txt", "txt", 99, []byte("one"))
	if err != nil || retryDigest != oneDigest {
		t.Fatalf("committed-generation retry digest=%q want=%q err=%v", retryDigest, oneDigest, err)
	}
}

func TestWebAttachUploadRecoveryFreezesOriginalNoteGeneration(t *testing.T) {
	state := webAttachUploadState{ExpectedUSN: 4}
	payload, err := applicationnotes.CanonicalState(state)
	if err != nil {
		t.Fatal(err)
	}

	// Simulate rebuilding the request after the inner note mutation advanced
	// the currently visible note generation.
	state.ExpectedUSN = 99
	if err := json.Unmarshal(payload, &state); err != nil {
		t.Fatal(err)
	}
	if state.ExpectedUSN != 4 {
		t.Fatalf("restored generation=%d, want original 4", state.ExpectedUSN)
	}
}

func TestWebAttachDeleteIdentitySurvivesMissingAttachmentRow(t *testing.T) {
	actor, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	attachOne, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	attachTwo, _ := domain.ParseObjectID("507f1f77bcf86cd799439013")
	oneID, oneDigest, err := webAttachDeleteIdentity(actor, attachOne, "delete-1")
	if err != nil {
		t.Fatal(err)
	}
	retryID, retryDigest, err := webAttachDeleteIdentity(actor, attachOne, "delete-1")
	if err != nil {
		t.Fatal(err)
	}
	otherID, otherDigest, err := webAttachDeleteIdentity(actor, attachTwo, "delete-1")
	if err != nil {
		t.Fatal(err)
	}
	if oneID == "" || oneID != retryID || oneDigest != retryDigest || oneID != otherID || oneDigest == otherDigest {
		t.Fatalf("one=%q/%q retry=%q/%q other=%q/%q", oneID, oneDigest, retryID, retryDigest, otherID, otherDigest)
	}
}

func TestStableCopiedImageIdentityBindsOperationSourceAndOwner(t *testing.T) {
	one := stableCopiedImageID("copy-op", "507f1f77bcf86cd799439011", "507f1f77bcf86cd799439012")
	retry := stableCopiedImageID("copy-op", "507f1f77bcf86cd799439011", "507f1f77bcf86cd799439012")
	otherOperation := stableCopiedImageID("copy-op-2", "507f1f77bcf86cd799439011", "507f1f77bcf86cd799439012")
	otherSource := stableCopiedImageID("copy-op", "507f1f77bcf86cd799439013", "507f1f77bcf86cd799439012")
	otherOwner := stableCopiedImageID("copy-op", "507f1f77bcf86cd799439011", "507f1f77bcf86cd799439014")
	if one.IsZero() || one != retry || one == otherOperation || one == otherSource || one == otherOwner {
		t.Fatalf("one=%s retry=%s operation=%s source=%s owner=%s", one.Hex(), retry.Hex(), otherOperation.Hex(), otherSource.Hex(), otherOwner.Hex())
	}
}

func TestAttachMutationIdentityIncludesObservedNoteGeneration(t *testing.T) {
	noteID, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	ownerID, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	one := attachMutationIdentity(ownerID, noteID, "507f1f77bcf86cd799439013", 4)
	two := attachMutationIdentity(ownerID, noteID, "507f1f77bcf86cd799439013", 5)
	if one == "" || one == two {
		t.Fatal("attachment mutation identity reused a stale note generation")
	}
}

func TestStableWebAttachIDReusesClientOperationAndScopesNote(t *testing.T) {
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	note, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	one := StableWebAttachID(owner, note, "upload-1")
	two := StableWebAttachID(owner, note, "upload-1")
	otherNote, _ := domain.ParseObjectID("507f1f77bcf86cd799439013")
	three := StableWebAttachID(owner, otherNote, "upload-1")
	if one.IsZero() || one != two || one == three {
		t.Fatal("web attachment identity is not stable and note-scoped")
	}
}
