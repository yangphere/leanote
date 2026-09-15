package notes

import (
	"testing"

	"github.com/yangphere/leanote/app/domain"
)

func TestSaveOperationIdentityUsesGenerationOnlyWithoutExpectedUSN(t *testing.T) {
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	noteID, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	command := SaveNoteCommand{ActorUserID: owner.Hex(), NoteID: noteID.Hex(), Metadata: map[string]any{"Title": "same"}}
	one, _, err := NewSaveOperationIdentity(owner, noteID, command, 10)
	if err != nil {
		t.Fatal(err)
	}
	two, _, _ := NewSaveOperationIdentity(owner, noteID, command, 11)
	if one == two {
		t.Fatal("Web save identity reused a stale committed receipt across note generations")
	}
	expected := 10
	command.ExpectedUSN = &expected
	three, _, _ := NewSaveOperationIdentity(owner, noteID, command, 10)
	four, _, _ := NewSaveOperationIdentity(owner, noteID, command, 11)
	if three != four {
		t.Fatal("expected-USN identity must not depend on a later repository generation")
	}
}

func TestSaveOperationIdentityIncludesAssetPlan(t *testing.T) {
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	noteID, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	command := SaveNoteCommand{ActorUserID: owner.Hex(), NoteID: noteID.Hex(), ExpectedUSN: intPointer(10)}
	withoutAssets, _, err := NewSaveOperationIdentity(owner, noteID, command, 10)
	if err != nil {
		t.Fatal(err)
	}
	command.AssetWork = &AssetMutation{Assets: []OperationAsset{{AssetID: "asset-one", Index: 0}}}
	withAsset, _, err := NewSaveOperationIdentity(owner, noteID, command, 10)
	if err != nil {
		t.Fatal(err)
	}
	if withAsset == withoutAssets {
		t.Fatal("asset reconciliation must have a distinct durable operation identity")
	}
	command.AssetWork.Assets[0].AssetID = "asset-two"
	changedAsset, _, err := NewSaveOperationIdentity(owner, noteID, command, 10)
	if err != nil {
		t.Fatal(err)
	}
	if changedAsset == withAsset {
		t.Fatal("different asset plans must not reuse a projection receipt")
	}
}

func intPointer(value int) *int {
	return &value
}
