package contentfs

import (
	"context"
	"crypto/sha256"
	"errors"
	"io/fs"
	"os"
	"testing"
	"time"

	application "github.com/yangphere/leanote/app/application/content"
	"github.com/yangphere/leanote/app/domain"
)

func TestCreateManifestStoreNoReplaceCASListAndTerminalGC(t *testing.T) {
	roots, err := ValidateContentRoots(newRootConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewDeleteManifestStore(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	store.directorySync = func(*os.Root, string) error { return nil }
	now := time.Unix(1_800_000_000, 0).UTC()
	manifest := newCreateManifest(t, now)
	if err := store.CreateCreateManifest(context.Background(), manifest); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateCreateManifest(context.Background(), manifest); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("duplicate CreateCreateManifest() error=%v", err)
	}
	active, truncated, err := store.ListActiveCreateManifests(context.Background(), 1)
	if err != nil || truncated || len(active) != 1 || active[0].LookupKey != manifest.LookupKey {
		t.Fatalf("ListActiveCreateManifests() active=%v truncated=%v err=%v", active, truncated, err)
	}
	published, err := manifest.Published(now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CompareAndSwapCreateManifest(context.Background(), manifest.LookupKey, manifest.Version, manifest.StateDigest, published); err != nil {
		t.Fatal(err)
	}
	terminal, err := published.Terminal(application.CreateOutcomeCommitted, now.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CompareAndSwapCreateManifest(context.Background(), manifest.LookupKey, published.Version, published.StateDigest, terminal); err != nil {
		t.Fatal(err)
	}
	gc, err := store.GCCreateTerminalsBounded(context.Background(), terminal.TerminalAt.Add(7*24*time.Hour+time.Nanosecond), 1)
	if err != nil || len(gc.Removed) != 1 || gc.Removed[0] != manifest.LookupKey {
		t.Fatalf("GCCreateTerminalsBounded() result=%+v err=%v", gc, err)
	}
	if _, found, err := store.LoadCreateManifest(context.Background(), manifest.LookupKey); err != nil || found {
		t.Fatalf("LoadCreateManifest() found=%v err=%v", found, err)
	}
}

func TestCreateManifestScansActiveAndTerminalBudgetsIndependently(t *testing.T) {
	roots, err := ValidateContentRoots(newRootConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewDeleteManifestStore(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	store.directorySync = func(*os.Root, string) error { return nil }
	now := time.Unix(1_800_000_000, 0).UTC()
	terminal := newCreateManifest(t, now)
	published, err := terminal.Published(now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	terminal, err = published.Terminal(application.CreateOutcomeCommitted, now.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateCreateManifest(context.Background(), terminal); err != nil {
		t.Fatal(err)
	}

	active := newCreateManifest(t, now.Add(3*time.Second))
	active.AssetID = "507f1f77bcf86cd799439013"
	active.LookupKey = ""
	active.InputDigest = [sha256.Size]byte{}
	active.StateDigest = [sha256.Size]byte{}
	active, err = application.NewCreateManifest(application.CreateIdentity{
		Action: "web_image_upload", OwnerID: active.OwnerID, RecordOwnerID: active.RecordOwnerID, Kind: application.AssetImage,
		AssetID: active.AssetID, ContentDigest: sha256.Sum256([]byte("image")), RecordDigest: sha256.Sum256([]byte("row-2")), ContentSize: 5,
	}, application.LogicalPath{Kind: application.RootPrivateFiles, Value: "owner/image-2.png"}, now.Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateCreateManifest(context.Background(), active); err != nil {
		t.Fatal(err)
	}

	manifests, _, err := store.ListActiveCreateManifests(context.Background(), 1)
	if err != nil || len(manifests) != 1 || manifests[0].LookupKey != active.LookupKey {
		t.Fatalf("ListActiveCreateManifests() = %+v, err=%v", manifests, err)
	}
	gc, err := store.GCCreateTerminalsBounded(context.Background(), now.Add(8*24*time.Hour), 1)
	if err != nil || len(gc.Removed) != 1 || gc.Removed[0] != terminal.LookupKey {
		t.Fatalf("GCCreateTerminalsBounded() = %+v, err=%v", gc, err)
	}
}

func TestCreateManifestStoreListsOnlyActionScopedPreNotes(t *testing.T) {
	roots, err := ValidateContentRoots(newRootConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewDeleteManifestStore(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	store.directorySync = func(*os.Root, string) error { return nil }
	now := time.Unix(1_800_000_000, 0).UTC()
	owner, err := domain.ParseObjectID("507f1f77bcf86cd799439011")
	if err != nil {
		t.Fatal(err)
	}
	note, err := domain.ParseObjectID("507f1f77bcf86cd799439012")
	if err != nil {
		t.Fatal(err)
	}
	for _, identity := range []application.CreateIdentity{
		{Action: "web_image_upload", OwnerID: owner, RecordOwnerID: owner, Kind: application.AssetImage, AssetID: "507f1f77bcf86cd799439013", ContentDigest: sha256.Sum256([]byte("web")), RecordDigest: sha256.Sum256([]byte("web-row")), ContentSize: 3},
		{Action: "other_pre_note", OwnerID: owner, RecordOwnerID: owner, ParentID: note, Kind: application.AssetImage, AssetID: "507f1f77bcf86cd799439014", PreNote: true, ContentDigest: sha256.Sum256([]byte("other")), RecordDigest: sha256.Sum256([]byte("other-row")), ContentSize: 5},
		{Action: "api_note_asset_upload", OwnerID: owner, RecordOwnerID: owner, ParentID: note, Kind: application.AssetImage, AssetID: "507f1f77bcf86cd799439015", PreNote: true, ContentDigest: sha256.Sum256([]byte("api")), RecordDigest: sha256.Sum256([]byte("api-row")), ContentSize: 3},
	} {
		manifest, err := application.NewCreateManifest(identity, application.LogicalPath{Kind: application.RootPrivateFiles, Value: "owner/images/" + identity.AssetID + ".png"}, now)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.CreateCreateManifest(context.Background(), manifest); err != nil {
			t.Fatal(err)
		}
	}

	manifests, truncated, err := store.ListActivePreNoteCreateManifests(context.Background(), "api_note_asset_upload", 1)
	if err != nil || truncated || len(manifests) != 1 || manifests[0].Action != "api_note_asset_upload" {
		t.Fatalf("ListActivePreNoteCreateManifests() manifests=%+v truncated=%t err=%v", manifests, truncated, err)
	}
	nonPreNotes, truncated, err := store.ListActiveNonPreNoteCreateManifests(context.Background(), 1)
	if err != nil || truncated || len(nonPreNotes) != 1 || nonPreNotes[0].PreNote || nonPreNotes[0].Action != "web_image_upload" {
		t.Fatalf("ListActiveNonPreNoteCreateManifests() manifests=%+v truncated=%t err=%v", nonPreNotes, truncated, err)
	}
}

func newCreateManifest(t *testing.T, now time.Time) application.CreateManifest {
	t.Helper()
	owner, err := domain.ParseObjectID("507f1f77bcf86cd799439011")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := application.NewCreateManifest(application.CreateIdentity{
		Action: "web_image_upload", OwnerID: owner, RecordOwnerID: owner, Kind: application.AssetImage,
		AssetID: "507f1f77bcf86cd799439012", ContentDigest: sha256.Sum256([]byte("image")), RecordDigest: sha256.Sum256([]byte("row")), ContentSize: 5,
	}, application.LogicalPath{Kind: application.RootPrivateFiles, Value: "owner/image.png"}, now)
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}
