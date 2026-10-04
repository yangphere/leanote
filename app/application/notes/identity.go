package notes

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yangphere/leanote/app/domain"
)

// NewOperationIdentity canonicalizes command input and derives a stable,
// non-secret receipt key. encoding/json sorts map keys, so field order in an
// adapter cannot create a second identity for the same command.
func NewOperationIdentity(kind string, ownerID, resourceID domain.ObjectID, input any) (string, string, []byte, error) {
	if kind == "" || ownerID.IsZero() {
		return "", "", nil, fmt.Errorf("workspace operation identity: invalid scope")
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return "", "", nil, fmt.Errorf("workspace operation identity: %w", err)
	}
	digestInput := append([]byte(kind+"\x00"+ownerID.Hex()+"\x00"+resourceID.Hex()+"\x00"), payload...)
	digest := sha256.Sum256(digestInput)
	digestText := hex.EncodeToString(digest[:])
	return kind + ":" + digestText[:32], digestText, payload, nil
}

// NewResourceOperationIdentity returns an operation key scoped only to the
// resource.  The request digest remains separate and is checked by the
// receipt store, so changing the body cannot turn one client retry into a
// second operation while still rejecting a conflicting request.
func NewResourceOperationIdentity(kind string, ownerID, resourceID domain.ObjectID, input any) (string, string, []byte, error) {
	if kind == "" || ownerID.IsZero() || resourceID.IsZero() {
		return "", "", nil, fmt.Errorf("workspace operation identity: invalid scope")
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return "", "", nil, fmt.Errorf("workspace operation identity: %w", err)
	}
	digestInput := append([]byte(kind+"\x00"+ownerID.Hex()+"\x00"+resourceID.Hex()+"\x00"), payload...)
	digest := sha256.Sum256(digestInput)
	digestText := hex.EncodeToString(digest[:])
	operationInput := kind + "\x00" + ownerID.Hex() + "\x00" + resourceID.Hex()
	operationDigest := sha256.Sum256([]byte(operationInput))
	return kind + ":" + hex.EncodeToString(operationDigest[:])[:32], digestText, payload, nil
}

// NewClientOperationIdentity scopes a client generation to the owner and
// action without storing the caller's raw value. The request digest remains a
// separate field on the receipt and rejects reuse with different input.
func NewClientOperationIdentity(kind string, ownerID domain.ObjectID, clientOperationID string) (string, error) {
	clientOperationID = strings.TrimSpace(clientOperationID)
	if kind == "" || ownerID.IsZero() || clientOperationID == "" {
		return "", fmt.Errorf("workspace client operation identity: invalid scope")
	}
	input := kind + "\x00" + ownerID.Hex() + "\x00" + clientOperationID
	digest := sha256.Sum256([]byte(input))
	return kind + ":" + hex.EncodeToString(digest[:])[:32], nil
}

func CanonicalState(value any) ([]byte, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("workspace operation state: %w", err)
	}
	return payload, nil
}

func NewSaveOperationIdentity(ownerID, noteID domain.ObjectID, command SaveNoteCommand, currentGeneration int) (string, string, error) {
	input := saveOperationIdentityInput(command)
	if command.ExpectedUSN == nil {
		input.Generation = currentGeneration
	}
	operationID, digest, _, err := NewOperationIdentity("note_save", ownerID, noteID, input)
	return operationID, digest, err
}

// NewSaveOperationDigest describes the caller's command without an observed
// generation. It is used with an explicit client operation generation so a
// retry can keep the same digest after the required write has committed.
func NewSaveOperationDigest(ownerID, noteID domain.ObjectID, command SaveNoteCommand) (string, error) {
	_, digest, _, err := NewOperationIdentity("note_save", ownerID, noteID, saveOperationIdentityInput(command))
	return digest, err
}

type saveOperationIdentity struct {
	ActorID          string
	NoteID           string
	OperationID      string
	ExpectedUSN      *int
	Metadata         map[string]any
	Content          *string
	Abstract         *string
	Generation       int
	AssetWorkPresent bool
	Assets           []OperationAsset
}

func saveOperationIdentityInput(command SaveNoteCommand) saveOperationIdentity {
	var assets []OperationAsset
	assetWorkPresent := command.AssetWork != nil
	if assetWorkPresent {
		assets = append(assets, command.AssetWork.Assets...)
	}
	return saveOperationIdentity{
		ActorID: command.ActorUserID, NoteID: command.NoteID, OperationID: command.OperationID,
		ExpectedUSN: command.ExpectedUSN, Metadata: command.Metadata, Content: command.Content,
		Abstract: command.Abstract, AssetWorkPresent: assetWorkPresent, Assets: assets,
	}
}
