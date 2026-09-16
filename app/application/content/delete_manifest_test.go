package content

import (
	"crypto/sha256"
	"testing"
	"time"

	"github.com/yangphere/leanote/app/domain"
)

func TestDeleteManifestIdentityKeepsLegacyLookupStableAcrossRestartInputs(t *testing.T) {
	owner := mustDeleteObjectID(t, "507f1f77bcf86cd799439011")
	first := DeleteIdentity{Action: "delete_image", OwnerID: owner, Kind: AssetImage, AssetID: "507f1f77bcf86cd799439012", Generation: 3, ContentDigest: sha256.Sum256([]byte("first"))}
	second := first
	second.Generation = 4
	second.ContentDigest = sha256.Sum256([]byte("second"))

	firstLookup, err := first.LookupKey()
	if err != nil {
		t.Fatal(err)
	}
	secondLookup, err := second.LookupKey()
	if err != nil {
		t.Fatal(err)
	}
	if firstLookup != secondLookup {
		t.Fatalf("legacy lookup changed: %q != %q", firstLookup, secondLookup)
	}
	firstOperation, err := first.OperationKey()
	if err != nil {
		t.Fatal(err)
	}
	secondOperation, err := second.OperationKey()
	if err != nil {
		t.Fatal(err)
	}
	if firstOperation == secondOperation {
		t.Fatal("legacy operation identity omitted generation and content digest")
	}
}

func TestDeleteManifestTerminalMarkerClearsRecoverableContent(t *testing.T) {
	owner := mustDeleteObjectID(t, "507f1f77bcf86cd799439011")
	path, err := ParseLogicalPath(RootPrivateFiles, "a/b/image.png")
	if err != nil {
		t.Fatal(err)
	}
	identity := DeleteIdentity{Action: "delete_image", OwnerID: owner, Kind: AssetImage, AssetID: "507f1f77bcf86cd799439012", Generation: 3, ContentDigest: sha256.Sum256([]byte("image"))}
	manifest, err := NewDeleteManifest(identity, path, time.Unix(1_700_000_000, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	terminal, err := manifest.Terminal(time.Unix(1_700_000_100, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if terminal.Stage != DeleteStageTerminal || terminal.Source != (LogicalPath{}) || terminal.Quarantine != (LogicalPath{}) || terminal.ContentDigest != ([sha256.Size]byte{}) {
		t.Fatalf("terminal retained recoverable content: %+v", terminal)
	}
	if err := terminal.Validate(); err != nil {
		t.Fatalf("terminal validation: %v", err)
	}
}

func mustDeleteObjectID(t *testing.T, value string) domain.ObjectID {
	t.Helper()
	id, err := domain.ParseObjectID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
