package api

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
)

func TestAPIBaseControllerHasNoStorageOrMongoDependencies(t *testing.T) {
	data, err := os.ReadFile("ApiBaseController.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"\"os\"", "\"path/filepath\"", "\"io/ioutil\"", "app/db", "mongo-driver", "revel.BasePath", "os.", "filepath."} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("ApiBaseController retains forbidden dependency %q", forbidden)
		}
	}
}

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

func TestAPINoteCreateOperationFreezesExistingAssetReference(t *testing.T) {
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	noteID, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	const existingID = "507f1f77bcf86cd799439013"
	request := info.ApiNote{NoteId: noteID.Hex(), Files: []info.NoteFile{{FileId: existingID, HasBody: false, IsAttach: true}}}
	_, _, assets, err := newAPINoteCreateOperationWithContent(owner, noteID, request, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 1 || assets[0].AssetID != existingID || assets[0].Index != 0 || !assets[0].IsAttach {
		t.Fatalf("assets=%+v, want frozen existing attachment", assets)
	}
}

func TestAPINoteCreateOperationRejectsInvalidExistingAssetReference(t *testing.T) {
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	noteID, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	request := info.ApiNote{NoteId: noteID.Hex(), Files: []info.NoteFile{{FileId: "not-an-object-id"}}}
	if _, _, _, err := newAPINoteCreateOperationWithContent(owner, noteID, request, nil); err == nil {
		t.Fatal("invalid existing asset reference was accepted")
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

func TestAPIAssetContentDigestIsSHA256(t *testing.T) {
	data := []byte("uploaded asset bytes")
	want := sha256.Sum256(data)
	if got := apiAssetContentDigest(data); got != hex.EncodeToString(want[:]) {
		t.Fatalf("digest=%q want=%q", got, hex.EncodeToString(want[:]))
	}
}
