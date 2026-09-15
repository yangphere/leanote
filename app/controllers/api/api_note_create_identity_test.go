package api

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
)

func TestAPINoteCreateOperationDigestIncludesUploadedBytes(t *testing.T) {
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	noteID, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	request := info.ApiNote{NoteId: noteID.Hex(), Files: []info.NoteFile{{LocalFileId: "local", HasBody: true}}}
	one, firstDigest, _, err := newAPINoteCreateOperationWithContent(owner, noteID, request, map[int][]byte{0: []byte("first")})
	if err != nil {
		t.Fatal(err)
	}
	two, secondDigest, _, err := newAPINoteCreateOperationWithContent(owner, noteID, request, map[int][]byte{0: []byte("second")})
	if err != nil {
		t.Fatal(err)
	}
	if one != two {
		t.Fatal("client NoteId must keep one operation scope")
	}
	if firstDigest == secondDigest {
		t.Fatal("different uploaded bytes must conflict with the same create operation")
	}
}

func TestAPINoteCreateOperationFreezesStableAssetIdentity(t *testing.T) {
	owner, err := domain.ParseObjectID("507f1f77bcf86cd799439011")
	if err != nil {
		t.Fatal(err)
	}
	noteID, err := domain.ParseObjectID("507f1f77bcf86cd799439012")
	if err != nil {
		t.Fatal(err)
	}
	request := info.ApiNote{
		NoteId:  noteID.Hex(),
		Title:   "stable",
		Content: "body",
		Files: []info.NoteFile{
			{LocalFileId: "local-image", HasBody: true},
			{LocalFileId: "local-attach", HasBody: true, IsAttach: true},
		},
	}
	firstID, firstDigest, firstAssets, err := newAPINoteCreateOperation(owner, noteID, request)
	if err != nil {
		t.Fatal(err)
	}
	secondID, secondDigest, secondAssets, err := newAPINoteCreateOperation(owner, noteID, request)
	if err != nil {
		t.Fatal(err)
	}
	if firstID != secondID || firstDigest != secondDigest || len(firstAssets) != 2 || len(secondAssets) != 2 {
		t.Fatalf("first=(%s,%s,%+v) second=(%s,%s,%+v)", firstID, firstDigest, firstAssets, secondID, secondDigest, secondAssets)
	}
	for index := range firstAssets {
		if firstAssets[index] != secondAssets[index] || firstAssets[index].AssetID == "" {
			t.Fatalf("asset %d is not stable: first=%+v second=%+v", index, firstAssets[index], secondAssets[index])
		}
	}
	if firstAssets[0].AssetID == firstAssets[1].AssetID {
		t.Fatalf("asset slots collided: %+v", firstAssets)
	}

	changed := request
	changed.Content = "different"
	changedID, changedDigest, _, err := newAPINoteCreateOperation(owner, noteID, changed)
	if err != nil {
		t.Fatal(err)
	}
	if changedID != firstID || changedDigest == firstDigest {
		t.Fatalf("same client NoteId did not preserve operation scope: first=(%s,%s) changed=(%s,%s)", firstID, firstDigest, changedID, changedDigest)
	}
}

func TestStableAPIOploadPathIsScopedByOwnerAndAssetKind(t *testing.T) {
	assetID := "507f1f77bcf86cd799439014"
	attachPath := stableAPIUploadPath("507f1f77bcf86cd799439011", assetID, true)
	imagePath := stableAPIUploadPath("507f1f77bcf86cd799439011", assetID, false)
	otherOwnerPath := stableAPIUploadPath("507f1f77bcf86cd799439099", assetID, true)
	otherAssetPath := stableAPIUploadPath("507f1f77bcf86cd799439011", "507f1f77bcf86cd799439015", true)
	if attachPath == "" || attachPath == imagePath || attachPath == otherOwnerPath || attachPath == otherAssetPath {
		t.Fatalf("paths are not isolated: attach=%q image=%q owner=%q asset=%q", attachPath, imagePath, otherOwnerPath, otherAssetPath)
	}
}

func TestAPIUpdateAssetSeedChangesWithTheRequest(t *testing.T) {
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	noteID, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	request := info.ApiNote{NoteId: noteID.Hex(), Content: "body", Files: []info.NoteFile{{LocalFileId: "local", HasBody: true}}}
	first := stableAPIUpdateAssetSeed(owner, noteID, 4, request)
	second := stableAPIUpdateAssetSeed(owner, noteID, 4, request)
	request.Content = "changed"
	changed := stableAPIUpdateAssetSeed(owner, noteID, 4, request)
	if first == "" || first != second || first == changed {
		t.Fatalf("seeds are not stable and request-scoped: first=%q second=%q changed=%q", first, second, changed)
	}
}

func TestStableAPIUpdateAssetSeedIncludesUploadedBytes(t *testing.T) {
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	noteID, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	request := info.ApiNote{NoteId: noteID.Hex(), Usn: 4, Files: []info.NoteFile{{LocalFileId: "local-1", HasBody: true}}}
	one := stableAPIUpdateAssetSeedWithContent(owner, noteID, 4, request, map[int][]byte{0: []byte("first")})
	two := stableAPIUpdateAssetSeedWithContent(owner, noteID, 4, request, map[int][]byte{0: []byte("second")})
	if one == "" || two == "" || one == two {
		t.Fatal("update asset identity must include uploaded bytes")
	}
}

func TestRemoveAPIUploadFilesIsIdempotent(t *testing.T) {
	base := t.TempDir()
	relative := filepath.Join("files", "owner", "asset", "images", "asset.png")
	path := filepath.Join(base, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("asset"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := removeAPIUploadFiles(base, relative); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("uploaded file still exists: %v", err)
	}
	if err := removeAPIUploadFiles(base, relative); err != nil {
		t.Fatalf("second cleanup was not idempotent: %v", err)
	}
}

func TestRemoveAPIUploadFilesRejectsPathOutsideBase(t *testing.T) {
	base := t.TempDir()
	outside := base + "-outside.txt"
	if err := os.WriteFile(outside, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(outside) })
	if err := removeAPIUploadFiles(base, outside); err == nil {
		t.Fatal("outside upload path was accepted")
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside file was removed: %v", err)
	}
}

func TestDurableAPIAssetFileRequiresContainedRegularFile(t *testing.T) {
	base := t.TempDir()
	outside := base + "-outside.txt"
	if err := os.WriteFile(outside, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(outside) })
	if durableAPIAssetFile(base, "") || durableAPIAssetFile(base, ".") {
		t.Fatal("empty path or directory was accepted as a durable asset")
	}
	path := filepath.Join(base, "files", "asset.bin")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("asset"), 0600); err != nil {
		t.Fatal(err)
	}
	if !durableAPIAssetFile(base, filepath.Join("files", "asset.bin")) {
		t.Fatal("contained regular asset was rejected")
	}
	if durableAPIAssetFile(base, outside) {
		t.Fatal("outside asset path was accepted")
	}
}
