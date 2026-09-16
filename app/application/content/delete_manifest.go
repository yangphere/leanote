package content

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"github.com/yangphere/leanote/app/domain"
)

type DeleteStage string

const (
	DeleteStagePrepared    DeleteStage = "prepared"
	DeleteStageQuarantined DeleteStage = "quarantined"
	DeleteStageMetadata    DeleteStage = "metadata_mutated"
	DeleteStageTerminal    DeleteStage = "terminal"
)

// DeleteIdentity is separate from notes receipts. LookupKey survives deletion
// of the metadata row; OperationKey freezes the legacy generation and digest
// when the caller does not provide an explicit operation ID.
type DeleteIdentity struct {
	Action        string
	OwnerID       domain.ObjectID
	Kind          AssetKind
	AssetID       string
	OperationID   string
	Generation    int64
	ContentDigest [sha256.Size]byte
}

func (identity DeleteIdentity) LookupKey() (string, error) {
	if err := identity.validateBase(); err != nil {
		return "", err
	}
	return deleteKey("lookup", identity.Action, identity.OwnerID.Hex(), string(identity.Kind), identity.AssetID), nil
}

func (identity DeleteIdentity) OperationKey() (string, error) {
	lookup, err := identity.LookupKey()
	if err != nil {
		return "", err
	}
	if identity.OperationID != "" {
		return deleteKey("operation", lookup, identity.OperationID), nil
	}
	if identity.Generation < 0 || identity.ContentDigest == ([sha256.Size]byte{}) {
		return "", validationError("legacy_delete_identity_incomplete", nil)
	}
	return deleteKey("legacy", lookup, stringInt(identity.Generation), hex.EncodeToString(identity.ContentDigest[:])), nil
}

func (identity DeleteIdentity) validateBase() error {
	if strings.TrimSpace(identity.Action) == "" || identity.OwnerID.IsZero() || identity.Kind == "" || strings.TrimSpace(identity.AssetID) == "" {
		return validationError("invalid_delete_identity", nil)
	}
	if strings.ContainsAny(identity.Action+identity.AssetID+identity.OperationID, "\x00") || len(identity.OperationID) > 256 {
		return validationError("invalid_delete_identity", nil)
	}
	return nil
}

func deleteKey(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

func stringInt(value int64) string {
	return strconv.FormatInt(value, 10)
}

type DeleteManifest struct {
	LookupKey     string
	OperationKey  string
	Action        string
	OwnerID       domain.ObjectID
	Kind          AssetKind
	AssetID       string
	Version       uint64
	StateDigest   [sha256.Size]byte
	ContentDigest [sha256.Size]byte
	Generation    int64
	Stage         DeleteStage
	Source        LogicalPath
	Quarantine    LogicalPath
	CreatedAt     time.Time
	UpdatedAt     time.Time
	TerminalAt    time.Time
}

func NewDeleteManifest(identity DeleteIdentity, source LogicalPath, now time.Time) (DeleteManifest, error) {
	lookup, err := identity.LookupKey()
	if err != nil {
		return DeleteManifest{}, err
	}
	operation, err := identity.OperationKey()
	if err != nil {
		return DeleteManifest{}, err
	}
	if _, err := ParseLogicalPath(source.Kind, source.Value); err != nil {
		return DeleteManifest{}, err
	}
	manifest := DeleteManifest{
		LookupKey: lookup, OperationKey: operation, Action: identity.Action, OwnerID: identity.OwnerID,
		Kind: identity.Kind, AssetID: identity.AssetID, Version: 1, ContentDigest: identity.ContentDigest,
		Generation: identity.Generation, Stage: DeleteStagePrepared, Source: source, CreatedAt: now.UTC(), UpdatedAt: now.UTC(),
	}
	manifest.StateDigest = manifest.digest()
	return manifest, manifest.Validate()
}

func (manifest DeleteManifest) Terminal(now time.Time) (DeleteManifest, error) {
	if err := manifest.Validate(); err != nil {
		return DeleteManifest{}, err
	}
	if manifest.Stage == DeleteStageTerminal {
		return manifest, nil
	}
	manifest.Version++
	manifest.Stage = DeleteStageTerminal
	manifest.Source = LogicalPath{}
	manifest.Quarantine = LogicalPath{}
	manifest.ContentDigest = [sha256.Size]byte{}
	manifest.TerminalAt = now.UTC()
	manifest.UpdatedAt = now.UTC()
	manifest.StateDigest = manifest.digest()
	return manifest, manifest.Validate()
}

func (manifest DeleteManifest) Advance(stage DeleteStage, quarantine LogicalPath, now time.Time) (DeleteManifest, error) {
	if err := manifest.Validate(); err != nil {
		return DeleteManifest{}, err
	}
	if (manifest.Stage == DeleteStagePrepared && stage != DeleteStageQuarantined) ||
		(manifest.Stage == DeleteStageQuarantined && stage != DeleteStageMetadata) {
		return DeleteManifest{}, conflictError("delete_manifest_stage_regression", nil)
	}
	if _, err := ParseLogicalPath(quarantine.Kind, quarantine.Value); err != nil {
		return DeleteManifest{}, err
	}
	if quarantine.Kind != manifest.Source.Kind {
		return DeleteManifest{}, validationError("delete_manifest_quarantine_root", nil)
	}
	manifest.Version++
	manifest.Stage = stage
	manifest.Quarantine = quarantine
	manifest.UpdatedAt = now.UTC()
	manifest.StateDigest = manifest.digest()
	return manifest, manifest.Validate()
}

func (manifest DeleteManifest) Validate() error {
	if manifest.LookupKey == "" || manifest.OperationKey == "" || manifest.OwnerID.IsZero() || manifest.Kind == "" || manifest.AssetID == "" || manifest.Version == 0 || manifest.CreatedAt.IsZero() || manifest.UpdatedAt.IsZero() {
		return validationError("invalid_delete_manifest", nil)
	}
	switch manifest.Stage {
	case DeleteStagePrepared:
		if manifest.Source == (LogicalPath{}) || manifest.ContentDigest == ([sha256.Size]byte{}) {
			return validationError("prepared_delete_manifest_incomplete", nil)
		}
	case DeleteStageQuarantined, DeleteStageMetadata:
		if manifest.Source == (LogicalPath{}) || manifest.Quarantine == (LogicalPath{}) || manifest.ContentDigest == ([sha256.Size]byte{}) {
			return validationError("active_delete_manifest_incomplete", nil)
		}
	case DeleteStageTerminal:
		if manifest.Source != (LogicalPath{}) || manifest.Quarantine != (LogicalPath{}) || manifest.ContentDigest != ([sha256.Size]byte{}) || manifest.TerminalAt.IsZero() {
			return validationError("terminal_delete_manifest_leaks_content", nil)
		}
	default:
		return validationError("unknown_delete_manifest_stage", nil)
	}
	if manifest.StateDigest != manifest.digest() {
		return conflictError("delete_manifest_digest_mismatch", nil)
	}
	return nil
}

func (manifest DeleteManifest) digest() [sha256.Size]byte {
	parts := []string{manifest.LookupKey, manifest.OperationKey, manifest.Action, manifest.OwnerID.Hex(), string(manifest.Kind), manifest.AssetID,
		stringInt(int64(manifest.Version)), hex.EncodeToString(manifest.ContentDigest[:]), stringInt(manifest.Generation), string(manifest.Stage),
		string(manifest.Source.Kind), manifest.Source.Value, string(manifest.Quarantine.Kind), manifest.Quarantine.Value,
		manifest.CreatedAt.UTC().Format(time.RFC3339Nano), manifest.UpdatedAt.UTC().Format(time.RFC3339Nano), manifest.TerminalAt.UTC().Format(time.RFC3339Nano)}
	return sha256.Sum256([]byte(strings.Join(parts, "\x00")))
}

// DeleteManifestReader supports process-loss recovery without exposing OS
// paths or relying on in-memory quarantine handles.
type DeleteManifestReader interface {
	LoadDeleteManifest(context.Context, string) (DeleteManifest, bool, error)
}
