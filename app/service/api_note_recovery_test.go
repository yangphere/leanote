package service

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	applicationcontent "github.com/yangphere/leanote/app/application/content"
	"github.com/yangphere/leanote/app/domain"
)

func TestRecoverAPINotePreNotesFinalizesAssetsForCommittedParent(t *testing.T) {
	manifest := testAPIPreNoteManifest(t)
	store := &recordingAPIPreNoteManifestStore{manifests: []applicationcontent.CreateManifest{manifest}}
	workflow := &recordingAPIPreNoteWorkflow{parents: map[domain.ObjectID]bool{manifest.ParentID: true}}

	result, err := recoverAPINotePreNotes(context.Background(), store, workflow, 7)
	if err != nil {
		t.Fatal(err)
	}
	if store.action != apiNoteAssetUploadAction || store.limit != 7 {
		t.Fatalf("scan action=%q limit=%d", store.action, store.limit)
	}
	if result.Scanned != 1 || result.Committed != 1 || result.Discarded != 0 || result.Pending != 0 || result.Truncated {
		t.Fatalf("result=%+v", result)
	}
	if len(workflow.finalized) != 1 || workflow.finalized[0].AssetID != manifest.AssetID || len(workflow.discarded) != 0 {
		t.Fatalf("workflow finalized=%+v discarded=%+v", workflow.finalized, workflow.discarded)
	}
}

func TestRecoverAPINotePreNotesDiscardsAssetsForMissingParent(t *testing.T) {
	manifest := testAPIPreNoteManifest(t)
	store := &recordingAPIPreNoteManifestStore{manifests: []applicationcontent.CreateManifest{manifest}, truncated: true}
	workflow := &recordingAPIPreNoteWorkflow{parents: map[domain.ObjectID]bool{manifest.ParentID: false}}

	result, err := recoverAPINotePreNotes(context.Background(), store, workflow, 3)
	if err != nil {
		t.Fatal(err)
	}
	if result.Scanned != 1 || result.Committed != 0 || result.Discarded != 1 || result.Pending != 0 || !result.Truncated {
		t.Fatalf("result=%+v", result)
	}
	if len(workflow.finalized) != 0 || len(workflow.discarded) != 1 || workflow.discarded[0].ParentID != manifest.ParentID {
		t.Fatalf("workflow finalized=%+v discarded=%+v", workflow.finalized, workflow.discarded)
	}
}

func testAPIPreNoteManifest(t *testing.T) applicationcontent.CreateManifest {
	t.Helper()
	owner, err := domain.ParseObjectID("507f1f77bcf86cd799439011")
	if err != nil {
		t.Fatal(err)
	}
	note, err := domain.ParseObjectID("507f1f77bcf86cd799439012")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := applicationcontent.NewCreateManifest(applicationcontent.CreateIdentity{
		Action: apiNoteAssetUploadAction, OwnerID: owner, RecordOwnerID: owner, ParentID: note,
		Kind: applicationcontent.AssetImage, AssetID: "507f1f77bcf86cd799439013", PreNote: true,
		ContentDigest: sha256.Sum256([]byte("asset")), RecordDigest: sha256.Sum256([]byte("row")), ContentSize: 5,
	}, applicationcontent.LogicalPath{Kind: applicationcontent.RootPrivateFiles, Value: "owner/images/asset.png"}, time.Unix(1_800_000_000, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}

type recordingAPIPreNoteManifestStore struct {
	action    string
	limit     int
	manifests []applicationcontent.CreateManifest
	truncated bool
}

func (store *recordingAPIPreNoteManifestStore) ListActivePreNoteCreateManifests(_ context.Context, action string, limit int) ([]applicationcontent.CreateManifest, bool, error) {
	store.action, store.limit = action, limit
	return store.manifests, store.truncated, nil
}

type recordingAPIPreNoteWorkflow struct {
	parents   map[domain.ObjectID]bool
	finalized []applicationcontent.PreNoteAssetIdentity
	discarded []applicationcontent.PreNoteAssetIdentity
}

func (workflow *recordingAPIPreNoteWorkflow) ParentExists(_ context.Context, _ domain.ObjectID, noteID domain.ObjectID) (bool, error) {
	return workflow.parents[noteID], nil
}

func (workflow *recordingAPIPreNoteWorkflow) Finalize(_ context.Context, identity applicationcontent.PreNoteAssetIdentity) error {
	workflow.finalized = append(workflow.finalized, identity)
	return nil
}

func (workflow *recordingAPIPreNoteWorkflow) Discard(_ context.Context, identity applicationcontent.PreNoteAssetIdentity) error {
	workflow.discarded = append(workflow.discarded, identity)
	return nil
}
