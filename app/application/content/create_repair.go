package content

import (
	"context"
	"errors"
	"io"
	"time"
)

const (
	DefaultCreateAbandonmentDelay = 15 * time.Minute
	DefaultCreateQuarantineDelay  = 24 * time.Hour
)

type CreateRepairCommand struct {
	Identity      CreateIdentity
	Destination   LogicalPath
	Source        io.Reader
	ApplyMetadata func(context.Context) error
}

type CreateRepairStatus string

const (
	CreateRepairPending   CreateRepairStatus = "pending"
	CreateRepairCommitted CreateRepairStatus = "committed"
	CreateRepairDiscarded CreateRepairStatus = "discarded"
)

type CreateRepairResult struct {
	LookupKey string
	Status    CreateRepairStatus
}

type CreateRecoveryResult struct {
	Scanned   int
	Committed int
	Discarded int
	Pending   int
	Truncated bool
}

type CreateRepairService struct {
	Content          ContentStore
	Lifecycle        DeleteStorage
	Manifests        CreateManifestStore
	Rows             CreateRowVerifier
	Now              func() time.Time
	AbandonmentDelay time.Duration
	QuarantineDelay  time.Duration
}

func (service *CreateRepairService) Execute(ctx context.Context, command CreateRepairCommand) (CreateRepairResult, error) {
	if err := service.validate(); err != nil {
		return CreateRepairResult{}, err
	}
	now := service.now()
	if command.Source == nil || command.ApplyMetadata == nil {
		return CreateRepairResult{}, validationError("invalid_create_repair_command", nil)
	}
	manifest, err := NewCreateManifest(command.Identity, command.Destination, now)
	if err != nil {
		return CreateRepairResult{}, err
	}
	if err := service.Manifests.CreateCreateManifest(ctx, manifest); err != nil {
		existing, found, loadErr := service.Manifests.LoadCreateManifest(ctx, manifest.LookupKey)
		if loadErr != nil || !found || !sameCreateIntent(existing, manifest) {
			if loadErr != nil {
				err = errors.Join(err, loadErr)
			}
			return CreateRepairResult{LookupKey: manifest.LookupKey, Status: CreateRepairPending}, conflictError("create_manifest_claim", err)
		}
		manifest = existing
	}
	if manifest.Stage == CreateStageTerminal {
		return terminalCreateResult(manifest)
	}
	if manifest.Stage == CreateStagePrepared {
		assetIdentity := createAssetIdentity(manifest)
		if _, err := service.Content.Publish(ctx, PublishRequest{Identity: assetIdentity, Destination: manifest.Destination, Source: command.Source}); err != nil {
			_, recoveryErr := service.recoverOne(ctx, manifest.LookupKey, now, true)
			return CreateRepairResult{LookupKey: manifest.LookupKey, Status: CreateRepairPending}, NewError(ErrorPartialWrite, "create_publish_unknown", errors.Join(err, recoveryErr))
		}
		published, err := manifest.Published(now)
		if err != nil {
			return CreateRepairResult{LookupKey: manifest.LookupKey, Status: CreateRepairPending}, err
		}
		if err := service.Manifests.CompareAndSwapCreateManifest(ctx, manifest.LookupKey, manifest.Version, manifest.StateDigest, published); err != nil {
			return CreateRepairResult{LookupKey: manifest.LookupKey, Status: CreateRepairPending}, NewError(ErrorPartialWrite, "create_publish_manifest", err)
		}
		manifest = published
	}

	applyErr := command.ApplyMetadata(ctx)
	result, recoveryErr := service.recoverOne(ctx, manifest.LookupKey, service.now(), applyErr != nil)
	if recoveryErr == nil && result.Status == CreateRepairCommitted {
		return result, nil
	}
	return result, NewError(ErrorPartialWrite, "create_metadata_unknown", errors.Join(applyErr, recoveryErr))
}

func (service *CreateRepairService) RecoverAbandoned(ctx context.Context, limit int) (CreateRecoveryResult, error) {
	if err := service.validate(); err != nil {
		return CreateRecoveryResult{}, err
	}
	if limit <= 0 {
		return CreateRecoveryResult{}, validationError("invalid_create_recovery_limit", nil)
	}
	manifests, truncated, err := service.Manifests.ListActiveNonPreNoteCreateManifests(ctx, limit)
	result := CreateRecoveryResult{Scanned: len(manifests), Truncated: truncated}
	if err != nil {
		return result, err
	}
	for _, manifest := range manifests {
		if err := ctx.Err(); err != nil {
			return result, NewError(ErrorTimeout, "create_recovery_canceled", err)
		}
		recovered, err := service.recoverOne(ctx, manifest.LookupKey, service.now(), false)
		if err != nil {
			return result, err
		}
		switch recovered.Status {
		case CreateRepairCommitted:
			result.Committed++
		case CreateRepairDiscarded:
			result.Discarded++
		default:
			result.Pending++
		}
	}
	return result, nil
}

// FinalizePreNote turns a verified pre-note publish into a committed marker
// only after the caller has proved that the parent note committed.
func (service *CreateRepairService) FinalizePreNote(ctx context.Context, identity PreNoteAssetIdentity) (CreateRepairResult, error) {
	manifest, err := service.VerifyPreNote(ctx, identity)
	if err != nil {
		return CreateRepairResult{}, err
	}
	if manifest.Stage == CreateStageTerminal {
		return terminalCreateResult(manifest)
	}
	return service.finish(ctx, manifest, CreateOutcomeCommitted, service.now())
}

// PreNoteManifest returns the frozen, logical-path-only manifest for a
// request-owned asset. The service adapter uses it to bind a delete lifecycle
// to the same identity rather than guessing from a database row.
func (service *CreateRepairService) PreNoteManifest(ctx context.Context, identity PreNoteAssetIdentity) (CreateManifest, error) {
	return service.loadPreNote(ctx, identity)
}

// VerifyPreNote confirms both the exact metadata row and published bytes
// before the caller marks a successful parent note as committed.
func (service *CreateRepairService) VerifyPreNote(ctx context.Context, identity PreNoteAssetIdentity) (CreateManifest, error) {
	manifest, err := service.loadPreNote(ctx, identity)
	if err != nil {
		return CreateManifest{}, err
	}
	if manifest.Stage == CreateStageTerminal {
		if manifest.Outcome == CreateOutcomeCommitted {
			return manifest, nil
		}
		return CreateManifest{}, conflictError("pre_note_verify_discarded", nil)
	}
	recovered, err := service.recoverOne(ctx, manifest.LookupKey, service.now(), false)
	if err != nil || recovered.Status != CreateRepairCommitted {
		if err == nil {
			err = conflictError("pre_note_verify_incomplete", nil)
		}
		return CreateManifest{}, err
	}
	return service.loadPreNote(ctx, identity)
}

// DiscardPreNote is the terminal step after the caller's dedicated metadata
// repository has quarantined, deleted, purged, and verified the asset. It
// refuses to scrub the create manifest while either the row or file remains.
func (service *CreateRepairService) DiscardPreNote(ctx context.Context, identity PreNoteAssetIdentity) error {
	manifest, err := service.loadPreNote(ctx, identity)
	if err != nil {
		return err
	}
	if manifest.Stage == CreateStageTerminal {
		if manifest.Outcome == CreateOutcomeDiscarded {
			return nil
		}
		return conflictError("pre_note_discard_committed", nil)
	}
	rowStatus, err := service.Rows.VerifyCreateRow(ctx, manifest)
	if err != nil {
		return NewError(ErrorDependency, "pre_note_discard_row_verify", err)
	}
	if rowStatus != CreateRowAbsent {
		return conflictError("pre_note_discard_row_remains", nil)
	}
	if manifest.Stage == CreateStagePrepared || manifest.Stage == CreateStagePublished {
		verification, err := service.Content.Verify(ctx, VerifyRequest{
			Identity: createAssetIdentity(manifest), Destination: manifest.Destination, ExpectedDigest: manifest.ContentDigest,
		})
		if err != nil {
			return err
		}
		switch verification.Status {
		case VerificationNotApplied:
			_, err = service.finish(ctx, manifest, CreateOutcomeDiscarded, service.now())
			return err
		case VerificationApplied:
			quarantine, err := service.Lifecycle.Quarantine(ctx, manifest.Destination, manifest.LookupKey, manifest.ContentDigest, manifest.ContentSize)
			if err != nil {
				return err
			}
			quarantined, err := manifest.Quarantined(quarantine, service.now())
			if err != nil {
				return err
			}
			if err := service.Manifests.CompareAndSwapCreateManifest(ctx, manifest.LookupKey, manifest.Version, manifest.StateDigest, quarantined); err != nil {
				return NewError(ErrorPartialWrite, "pre_note_discard_quarantine_manifest", err)
			}
			manifest = quarantined
		default:
			return conflictError("pre_note_discard_content_unknown", nil)
		}
	}
	if manifest.Stage != CreateStageQuarantined {
		return conflictError("pre_note_discard_stage", nil)
	}
	if err := service.Lifecycle.Purge(ctx, manifest.Quarantine, manifest.ContentDigest, manifest.ContentSize); err != nil {
		return err
	}
	_, err = service.finish(ctx, manifest, CreateOutcomeDiscarded, service.now())
	return err
}

func (service *CreateRepairService) recoverOne(ctx context.Context, lookupKey string, now time.Time, allowImmediateQuarantine bool) (CreateRepairResult, error) {
	manifest, found, err := service.Manifests.LoadCreateManifest(ctx, lookupKey)
	if err != nil || !found {
		if err == nil {
			err = conflictError("create_manifest_missing", nil)
		}
		return CreateRepairResult{LookupKey: lookupKey, Status: CreateRepairPending}, err
	}
	if manifest.Stage == CreateStageTerminal {
		return terminalCreateResult(manifest)
	}
	rowStatus, err := service.Rows.VerifyCreateRow(ctx, manifest)
	if err != nil {
		return CreateRepairResult{LookupKey: lookupKey, Status: CreateRepairPending}, NewError(ErrorDependency, "create_row_verify", err)
	}
	if rowStatus == CreateRowConflict {
		return CreateRepairResult{LookupKey: lookupKey, Status: CreateRepairPending}, conflictError("create_row_conflict", nil)
	}
	if rowStatus == CreateRowExact {
		if manifest.Stage == CreateStageQuarantined {
			if err := service.Lifecycle.Restore(ctx, manifest.Destination, manifest.Quarantine, manifest.ContentDigest, manifest.ContentSize); err != nil {
				return CreateRepairResult{LookupKey: lookupKey, Status: CreateRepairPending}, err
			}
		}
		verification, err := service.Content.Verify(ctx, VerifyRequest{
			Identity: createAssetIdentity(manifest), Destination: manifest.Destination, ExpectedDigest: manifest.ContentDigest,
		})
		if err != nil || verification.Status != VerificationApplied {
			if err == nil {
				err = conflictError("create_content_missing_for_row", nil)
			}
			return CreateRepairResult{LookupKey: lookupKey, Status: CreateRepairPending}, err
		}
		if manifest.PreNote {
			return CreateRepairResult{LookupKey: manifest.LookupKey, Status: CreateRepairCommitted}, nil
		}
		return service.finish(ctx, manifest, CreateOutcomeCommitted, now)
	}
	if rowStatus != CreateRowAbsent {
		return CreateRepairResult{LookupKey: lookupKey, Status: CreateRepairPending}, conflictError("create_row_status", nil)
	}

	if manifest.Stage == CreateStageQuarantined {
		if now.Before(manifest.UpdatedAt.Add(service.quarantineDelay())) {
			return CreateRepairResult{LookupKey: lookupKey, Status: CreateRepairPending}, nil
		}
		// Verify absence again immediately before irreversible purge.
		status, err := service.Rows.VerifyCreateRow(ctx, manifest)
		if err != nil {
			return CreateRepairResult{LookupKey: lookupKey, Status: CreateRepairPending}, NewError(ErrorDependency, "create_row_reverify", err)
		}
		if status == CreateRowExact {
			if err := service.Lifecycle.Restore(ctx, manifest.Destination, manifest.Quarantine, manifest.ContentDigest, manifest.ContentSize); err != nil {
				return CreateRepairResult{LookupKey: lookupKey, Status: CreateRepairPending}, err
			}
			return service.finish(ctx, manifest, CreateOutcomeCommitted, now)
		}
		if status != CreateRowAbsent {
			return CreateRepairResult{LookupKey: lookupKey, Status: CreateRepairPending}, conflictError("create_row_conflict", nil)
		}
		if err := service.Lifecycle.Purge(ctx, manifest.Quarantine, manifest.ContentDigest, manifest.ContentSize); err != nil {
			return CreateRepairResult{LookupKey: lookupKey, Status: CreateRepairPending}, err
		}
		return service.finish(ctx, manifest, CreateOutcomeDiscarded, now)
	}

	if !allowImmediateQuarantine && now.Before(manifest.CreatedAt.Add(service.abandonmentDelay())) {
		return CreateRepairResult{LookupKey: lookupKey, Status: CreateRepairPending}, nil
	}
	quarantine, err := service.Lifecycle.Quarantine(ctx, manifest.Destination, manifest.LookupKey, manifest.ContentDigest, manifest.ContentSize)
	if errorCode(err) == "quarantine_source_missing" {
		return service.finish(ctx, manifest, CreateOutcomeDiscarded, now)
	}
	if err != nil {
		return CreateRepairResult{LookupKey: lookupKey, Status: CreateRepairPending}, err
	}
	quarantined, err := manifest.Quarantined(quarantine, now)
	if err != nil {
		return CreateRepairResult{LookupKey: lookupKey, Status: CreateRepairPending}, err
	}
	if err := service.Manifests.CompareAndSwapCreateManifest(ctx, manifest.LookupKey, manifest.Version, manifest.StateDigest, quarantined); err != nil {
		return CreateRepairResult{LookupKey: lookupKey, Status: CreateRepairPending}, NewError(ErrorPartialWrite, "create_quarantine_manifest", err)
	}
	return CreateRepairResult{LookupKey: lookupKey, Status: CreateRepairPending}, nil
}

func (service *CreateRepairService) loadPreNote(ctx context.Context, identity PreNoteAssetIdentity) (CreateManifest, error) {
	if err := service.validate(); err != nil {
		return CreateManifest{}, err
	}
	lookupKey, err := identity.LookupKey()
	if err != nil {
		return CreateManifest{}, err
	}
	manifest, found, err := service.Manifests.LoadCreateManifest(ctx, lookupKey)
	if err != nil {
		return CreateManifest{}, err
	}
	if !found {
		return CreateManifest{}, conflictError("pre_note_manifest_missing", nil)
	}
	if !manifest.PreNote || manifest.Action != identity.Action || manifest.OwnerID != identity.OwnerID || manifest.RecordOwnerID != identity.RecordOwnerID || manifest.ParentID != identity.ParentID || manifest.Kind != identity.Kind || manifest.AssetID != identity.AssetID {
		return CreateManifest{}, conflictError("pre_note_manifest_identity", nil)
	}
	return manifest, nil
}

func (service *CreateRepairService) finish(ctx context.Context, manifest CreateManifest, outcome CreateOutcome, now time.Time) (CreateRepairResult, error) {
	terminal, err := manifest.Terminal(outcome, now)
	if err != nil {
		return CreateRepairResult{LookupKey: manifest.LookupKey, Status: CreateRepairPending}, err
	}
	if err := service.Manifests.CompareAndSwapCreateManifest(ctx, manifest.LookupKey, manifest.Version, manifest.StateDigest, terminal); err != nil {
		return CreateRepairResult{LookupKey: manifest.LookupKey, Status: CreateRepairPending}, NewError(ErrorPartialWrite, "create_terminal_manifest", err)
	}
	status := CreateRepairCommitted
	if outcome == CreateOutcomeDiscarded {
		status = CreateRepairDiscarded
	}
	return CreateRepairResult{LookupKey: manifest.LookupKey, Status: status}, nil
}

func (service *CreateRepairService) validate() error {
	if service == nil || service.Content == nil || service.Lifecycle == nil || service.Manifests == nil || service.Rows == nil {
		return NewError(ErrorDependency, "create_repair_dependency", nil)
	}
	return nil
}

func (service *CreateRepairService) now() time.Time {
	if service.Now != nil {
		return service.Now().UTC()
	}
	return time.Now().UTC()
}

func (service *CreateRepairService) abandonmentDelay() time.Duration {
	if service.AbandonmentDelay > 0 {
		return service.AbandonmentDelay
	}
	return DefaultCreateAbandonmentDelay
}

func (service *CreateRepairService) quarantineDelay() time.Duration {
	if service.QuarantineDelay > 0 {
		return service.QuarantineDelay
	}
	return DefaultCreateQuarantineDelay
}

func createAssetIdentity(manifest CreateManifest) AssetIdentity {
	return AssetIdentity{
		OperationID: manifest.LookupKey, OwnerID: manifest.OwnerID, Kind: manifest.Kind,
		SourceID: manifest.AssetID, DestinationID: manifest.AssetID, Digest: manifest.ContentDigest,
	}
}

func sameCreateIntent(left, right CreateManifest) bool {
	return left.LookupKey == right.LookupKey && left.InputDigest == right.InputDigest
}

func terminalCreateResult(manifest CreateManifest) (CreateRepairResult, error) {
	switch manifest.Outcome {
	case CreateOutcomeCommitted:
		return CreateRepairResult{LookupKey: manifest.LookupKey, Status: CreateRepairCommitted}, nil
	case CreateOutcomeDiscarded:
		return CreateRepairResult{LookupKey: manifest.LookupKey, Status: CreateRepairDiscarded}, conflictError("create_operation_discarded", nil)
	default:
		return CreateRepairResult{LookupKey: manifest.LookupKey, Status: CreateRepairPending}, conflictError("create_terminal_outcome", nil)
	}
}

func errorCode(err error) string {
	var contentErr *Error
	if errors.As(err, &contentErr) {
		return contentErr.Code
	}
	return ""
}
