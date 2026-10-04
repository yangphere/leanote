package content

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/yangphere/leanote/app/domain"
)

type CreateStage string

const (
	CreateStagePrepared    CreateStage = "prepared"
	CreateStagePublished   CreateStage = "published"
	CreateStageQuarantined CreateStage = "quarantined"
	CreateStageTerminal    CreateStage = "terminal"
)

type CreateOutcome string

const (
	CreateOutcomeCommitted CreateOutcome = "committed"
	CreateOutcomeDiscarded CreateOutcome = "discarded"
)

// CreateIdentity binds generated storage identity to the exact owner-scoped
// metadata row that may make the content live. PreNote assets have a parent
// note ID but no committed note generation yet; they must be finalized or
// discarded explicitly before their manifest is terminalized.
type CreateIdentity struct {
	Action        string
	OwnerID       domain.ObjectID
	RecordOwnerID domain.ObjectID
	ParentID      domain.ObjectID
	Kind          AssetKind
	AssetID       string
	Generation    int64
	PreNote       bool
	ContentDigest [sha256.Size]byte
	RecordDigest  [sha256.Size]byte
	ContentSize   int64
}

func (identity CreateIdentity) LookupKey() (string, error) {
	if err := identity.validate(); err != nil {
		return "", err
	}
	return createLookupKey(identity.Action, identity.OwnerID, identity.RecordOwnerID, identity.ParentID, identity.Kind, identity.AssetID, identity.PreNote), nil
}

func (identity CreateIdentity) validate() error {
	if err := validateCreateBinding(identity.Action, identity.OwnerID, identity.RecordOwnerID, identity.ParentID, identity.Kind, identity.AssetID, identity.Generation, identity.PreNote); err != nil {
		return err
	}
	if identity.ContentDigest == ([sha256.Size]byte{}) || identity.RecordDigest == ([sha256.Size]byte{}) || identity.ContentSize <= 0 {
		return validationError("invalid_create_identity", nil)
	}
	return nil
}

// PreNoteAssetIdentity is the minimal, deterministic lookup identity used
// after a note create either commits or fails. It deliberately excludes bytes
// and row metadata so cleanup can locate the frozen create manifest.
type PreNoteAssetIdentity struct {
	Action        string
	OwnerID       domain.ObjectID
	RecordOwnerID domain.ObjectID
	ParentID      domain.ObjectID
	Kind          AssetKind
	AssetID       string
}

func (identity PreNoteAssetIdentity) LookupKey() (string, error) {
	if err := validateCreateBinding(identity.Action, identity.OwnerID, identity.RecordOwnerID, identity.ParentID, identity.Kind, identity.AssetID, 0, true); err != nil {
		return "", err
	}
	return createLookupKey(identity.Action, identity.OwnerID, identity.RecordOwnerID, identity.ParentID, identity.Kind, identity.AssetID, true), nil
}

func createLookupKey(action string, ownerID, recordOwnerID, parentID domain.ObjectID, kind AssetKind, assetID string, preNote bool) string {
	parts := []string{"create", action, ownerID.Hex(), recordOwnerID.Hex(), parentID.Hex(), string(kind), assetID}
	if preNote {
		parts = append(parts, "pre_note")
	}
	return deleteKey(parts...)
}

func validateCreateBinding(action string, ownerID, recordOwnerID, parentID domain.ObjectID, kind AssetKind, assetID string, generation int64, preNote bool) error {
	if strings.TrimSpace(action) == "" || ownerID.IsZero() || recordOwnerID.IsZero() || kind == "" || strings.TrimSpace(assetID) == "" || strings.ContainsAny(action+assetID, "\x00") {
		return validationError("invalid_create_identity", nil)
	}
	if preNote {
		if parentID.IsZero() || generation != 0 {
			return validationError("invalid_pre_note_create_identity", nil)
		}
		return nil
	}
	if kind == AssetAttachment {
		if parentID.IsZero() {
			return validationError("create_attachment_parent_required", nil)
		}
		if generation <= 0 {
			return validationError("create_attachment_generation_required", nil)
		}
		return nil
	}
	if generation != 0 {
		return validationError("create_generation_not_allowed", nil)
	}
	if !parentID.IsZero() {
		return validationError("create_parent_not_allowed", nil)
	}
	return nil
}

type CreateManifest struct {
	LookupKey     string
	Action        string
	OwnerID       domain.ObjectID
	RecordOwnerID domain.ObjectID
	ParentID      domain.ObjectID
	Kind          AssetKind
	AssetID       string
	Generation    int64
	PreNote       bool
	Root          RootKind
	Version       uint64
	StateDigest   [sha256.Size]byte
	InputDigest   [sha256.Size]byte
	ContentDigest [sha256.Size]byte
	RecordDigest  [sha256.Size]byte
	ContentSize   int64
	Stage         CreateStage
	Destination   LogicalPath
	Quarantine    LogicalPath
	Outcome       CreateOutcome
	CreatedAt     time.Time
	UpdatedAt     time.Time
	TerminalAt    time.Time
}

func NewCreateManifest(identity CreateIdentity, destination LogicalPath, now time.Time) (CreateManifest, error) {
	lookup, err := identity.LookupKey()
	if err != nil {
		return CreateManifest{}, err
	}
	parsed, err := ParseLogicalPath(destination.Kind, destination.Value)
	if err != nil || parsed != destination || now.IsZero() {
		return CreateManifest{}, validationError("invalid_create_manifest_input", err)
	}
	manifest := CreateManifest{
		LookupKey: lookup, Action: identity.Action, OwnerID: identity.OwnerID, RecordOwnerID: identity.RecordOwnerID,
		ParentID: identity.ParentID, Kind: identity.Kind, AssetID: identity.AssetID, Generation: identity.Generation, PreNote: identity.PreNote, Root: destination.Kind, Version: 1,
		ContentDigest: identity.ContentDigest, RecordDigest: identity.RecordDigest, ContentSize: identity.ContentSize, Stage: CreateStagePrepared, Destination: destination,
		CreatedAt: now.UTC(), UpdatedAt: now.UTC(),
	}
	manifest.InputDigest = createInputDigest(identity, destination)
	manifest.StateDigest = manifest.digest()
	return manifest, manifest.Validate()
}

func (manifest CreateManifest) Published(now time.Time) (CreateManifest, error) {
	if err := manifest.Validate(); err != nil {
		return CreateManifest{}, err
	}
	if manifest.Stage == CreateStagePublished {
		return manifest, nil
	}
	if manifest.Stage != CreateStagePrepared || now.IsZero() {
		return CreateManifest{}, conflictError("create_manifest_stage", nil)
	}
	manifest.Version++
	manifest.Stage = CreateStagePublished
	manifest.UpdatedAt = now.UTC()
	manifest.StateDigest = manifest.digest()
	return manifest, manifest.Validate()
}

func (manifest CreateManifest) Quarantined(path LogicalPath, now time.Time) (CreateManifest, error) {
	if err := manifest.Validate(); err != nil {
		return CreateManifest{}, err
	}
	if manifest.Stage == CreateStageQuarantined && manifest.Quarantine == path {
		return manifest, nil
	}
	parsed, err := ParseLogicalPath(path.Kind, path.Value)
	if manifest.Stage == CreateStageTerminal || err != nil || parsed != path || path.Kind != manifest.Destination.Kind || now.IsZero() {
		return CreateManifest{}, conflictError("create_manifest_quarantine", err)
	}
	manifest.Version++
	manifest.Stage = CreateStageQuarantined
	manifest.Quarantine = path
	manifest.UpdatedAt = now.UTC()
	manifest.StateDigest = manifest.digest()
	return manifest, manifest.Validate()
}

func (manifest CreateManifest) Terminal(outcome CreateOutcome, now time.Time) (CreateManifest, error) {
	if err := manifest.Validate(); err != nil {
		return CreateManifest{}, err
	}
	if manifest.Stage == CreateStageTerminal {
		if manifest.Outcome != outcome {
			return CreateManifest{}, conflictError("create_manifest_terminal_outcome", nil)
		}
		return manifest, nil
	}
	if (outcome != CreateOutcomeCommitted && outcome != CreateOutcomeDiscarded) || now.IsZero() {
		return CreateManifest{}, validationError("invalid_create_terminal", nil)
	}
	manifest.Version++
	manifest.Stage = CreateStageTerminal
	manifest.Outcome = outcome
	manifest.Destination = LogicalPath{}
	manifest.Quarantine = LogicalPath{}
	manifest.ContentDigest = [sha256.Size]byte{}
	manifest.RecordDigest = [sha256.Size]byte{}
	manifest.ContentSize = 0
	manifest.UpdatedAt = now.UTC()
	manifest.TerminalAt = now.UTC()
	manifest.StateDigest = manifest.digest()
	return manifest, manifest.Validate()
}

func (manifest CreateManifest) Validate() error {
	if manifest.LookupKey == "" || strings.TrimSpace(manifest.Action) == "" || manifest.OwnerID.IsZero() || manifest.RecordOwnerID.IsZero() || manifest.Kind == "" || manifest.AssetID == "" || manifest.Root == "" || manifest.Version == 0 || manifest.InputDigest == ([sha256.Size]byte{}) || manifest.CreatedAt.IsZero() || manifest.UpdatedAt.IsZero() {
		return validationError("invalid_create_manifest", nil)
	}
	if err := validateCreateBinding(manifest.Action, manifest.OwnerID, manifest.RecordOwnerID, manifest.ParentID, manifest.Kind, manifest.AssetID, manifest.Generation, manifest.PreNote); err != nil {
		return validationError("invalid_create_manifest_identity", err)
	}
	identity := CreateIdentity{Action: manifest.Action, OwnerID: manifest.OwnerID, RecordOwnerID: manifest.RecordOwnerID, ParentID: manifest.ParentID, Kind: manifest.Kind, AssetID: manifest.AssetID, Generation: manifest.Generation, PreNote: manifest.PreNote, ContentDigest: manifest.ContentDigest, RecordDigest: manifest.RecordDigest, ContentSize: manifest.ContentSize}
	if manifest.Stage != CreateStageTerminal {
		lookup, err := identity.LookupKey()
		if err != nil || lookup != manifest.LookupKey {
			return validationError("invalid_create_manifest_identity", err)
		}
		if parsed, err := ParseLogicalPath(manifest.Destination.Kind, manifest.Destination.Value); err != nil || parsed != manifest.Destination {
			return validationError("invalid_create_destination", err)
		}
	}
	switch manifest.Stage {
	case CreateStagePrepared, CreateStagePublished:
		if manifest.Quarantine != (LogicalPath{}) || manifest.Outcome != "" || !manifest.TerminalAt.IsZero() {
			return validationError("invalid_active_create_manifest", nil)
		}
	case CreateStageQuarantined:
		if manifest.Quarantine == (LogicalPath{}) || manifest.Outcome != "" || !manifest.TerminalAt.IsZero() {
			return validationError("invalid_quarantined_create_manifest", nil)
		}
	case CreateStageTerminal:
		if manifest.Outcome == "" || manifest.Destination != (LogicalPath{}) || manifest.Quarantine != (LogicalPath{}) || manifest.ContentDigest != ([sha256.Size]byte{}) || manifest.RecordDigest != ([sha256.Size]byte{}) || manifest.ContentSize != 0 || manifest.TerminalAt.IsZero() {
			return validationError("invalid_terminal_create_manifest", nil)
		}
	default:
		return validationError("unknown_create_manifest_stage", nil)
	}
	if manifest.StateDigest != manifest.digest() {
		return conflictError("create_manifest_digest_mismatch", nil)
	}
	return nil
}

func (manifest CreateManifest) digest() [sha256.Size]byte {
	parts := []string{
		manifest.LookupKey, manifest.Action, manifest.OwnerID.Hex(), manifest.RecordOwnerID.Hex(), manifest.ParentID.Hex(), string(manifest.Kind), manifest.AssetID, stringInt(manifest.Generation), string(manifest.Root),
		stringInt(int64(manifest.Version)), hex.EncodeToString(manifest.InputDigest[:]), hex.EncodeToString(manifest.ContentDigest[:]), hex.EncodeToString(manifest.RecordDigest[:]), stringInt(manifest.ContentSize), string(manifest.Stage),
		string(manifest.Destination.Kind), manifest.Destination.Value, string(manifest.Quarantine.Kind), manifest.Quarantine.Value, string(manifest.Outcome),
		manifest.CreatedAt.UTC().Format(time.RFC3339Nano), manifest.UpdatedAt.UTC().Format(time.RFC3339Nano), manifest.TerminalAt.UTC().Format(time.RFC3339Nano),
	}
	if manifest.PreNote {
		parts = append(parts, "pre_note")
	}
	return sha256.Sum256([]byte(strings.Join(parts, "\x00")))
}

func createInputDigest(identity CreateIdentity, destination LogicalPath) [sha256.Size]byte {
	parts := []string{
		identity.Action, identity.OwnerID.Hex(), identity.RecordOwnerID.Hex(), identity.ParentID.Hex(), string(identity.Kind), identity.AssetID, stringInt(identity.Generation),
		hex.EncodeToString(identity.ContentDigest[:]), hex.EncodeToString(identity.RecordDigest[:]), stringInt(identity.ContentSize), string(destination.Kind), destination.Value,
	}
	if identity.PreNote {
		parts = append(parts, "pre_note")
	}
	return sha256.Sum256([]byte(strings.Join(parts, "\x00")))
}

type CreateManifestStore interface {
	CreateCreateManifest(context.Context, CreateManifest) error
	LoadCreateManifest(context.Context, string) (CreateManifest, bool, error)
	CompareAndSwapCreateManifest(context.Context, string, uint64, [sha256.Size]byte, CreateManifest) error
	ListActiveCreateManifests(context.Context, int) ([]CreateManifest, bool, error)
	ListActiveNonPreNoteCreateManifests(context.Context, int) ([]CreateManifest, bool, error)
}

type CreateRowStatus string

const (
	CreateRowExact    CreateRowStatus = "exact"
	CreateRowAbsent   CreateRowStatus = "absent"
	CreateRowConflict CreateRowStatus = "conflict"
)

type CreateRowVerifier interface {
	VerifyCreateRow(context.Context, CreateManifest) (CreateRowStatus, error)
}
