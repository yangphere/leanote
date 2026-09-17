package content

import (
	"context"
	"errors"
	"time"

	"github.com/yangphere/leanote/app/domain"
)

type DeleteAssetMetadata struct {
	AssetID    domain.ObjectID
	OwnerID    domain.ObjectID
	Kind       AssetKind
	StoredPath string
	Size       int64
	Generation int64
}

type DeleteAssetRepository interface {
	LoadDeleteAsset(context.Context, AssetKind, domain.ObjectID) (DeleteAssetMetadata, error)
	DeleteAsset(context.Context, domain.ObjectID, AssetKind, domain.ObjectID) (bool, error)
	DeleteAssetAbsent(context.Context, domain.ObjectID, AssetKind, domain.ObjectID) (bool, error)
}

type DeleteAssetCommand struct {
	Action      string
	OwnerID     domain.ObjectID
	Kind        AssetKind
	AssetID     domain.ObjectID
	OperationID string
}

// DeleteAssetService applies the same manifest/quarantine/verify lifecycle to
// content kinds whose metadata projection is supplied by an adapter.
type DeleteAssetService struct {
	Repository DeleteAssetRepository
	Content    ContentStore
	Manifests  DeleteManifestStore
	Storage    DeleteStorage
	Now        func() time.Time
}

func (service DeleteAssetService) Delete(ctx context.Context, command DeleteAssetCommand) error {
	if command.Action == "" || command.OwnerID.IsZero() || command.AssetID.IsZero() || command.Kind == "" {
		return validationError("asset_delete_input", nil)
	}
	if service.Repository == nil || service.Content == nil || service.Manifests == nil || service.Storage == nil {
		return contentError(ErrorDependency, "asset_delete_dependency_missing", nil)
	}
	now := service.Now
	if now == nil {
		now = time.Now
	}
	identity := DeleteIdentity{Action: command.Action, OwnerID: command.OwnerID, Kind: command.Kind, AssetID: command.AssetID.Hex(), OperationID: command.OperationID}
	lookupKey, err := identity.LookupKey()
	if err != nil {
		return err
	}
	manifest, found, err := service.Manifests.LoadDeleteManifest(ctx, lookupKey)
	if err != nil {
		return err
	}
	if !found {
		asset, err := service.Repository.LoadDeleteAsset(ctx, command.Kind, command.AssetID)
		if err != nil {
			return attachmentDependencyError("asset_delete_lookup", err)
		}
		if asset.AssetID != command.AssetID || asset.OwnerID != command.OwnerID || asset.Kind != command.Kind || asset.Generation < 0 || asset.Size <= 0 {
			return contentError(ErrorNotFound, "asset_delete_missing", nil)
		}
		source, err := ParseStoredPath(asset.StoredPath)
		if err != nil {
			return err
		}
		digest, err := contentDigest(ctx, service.Content, source, asset.Size)
		if err != nil {
			return err
		}
		identity.Generation = asset.Generation
		identity.ContentDigest = digest
		identity.ContentSize = asset.Size
		manifest, err = NewDeleteManifest(identity, source, now())
		if err != nil {
			return err
		}
		if err := service.Manifests.Create(ctx, manifest); err != nil {
			return err
		}
	} else if manifest.OwnerID != command.OwnerID || manifest.AssetID != command.AssetID.Hex() || manifest.Kind != command.Kind || manifest.Action != command.Action {
		return conflictError("asset_delete_manifest_identity", nil)
	} else if command.OperationID != "" {
		operationKey, err := identity.OperationKey()
		if err != nil || operationKey != manifest.OperationKey {
			return conflictError("asset_delete_operation", err)
		}
	}
	if manifest.Stage == DeleteStageTerminal {
		return nil
	}
	quarantineVerified := false
	if manifest.Stage == DeleteStagePrepared {
		quarantine, err := service.Storage.Quarantine(ctx, manifest.Source, manifest.LookupKey, manifest.ContentDigest, manifest.ContentSize)
		if err != nil {
			return err
		}
		next, err := manifest.Advance(DeleteStageQuarantined, quarantine, now())
		if err != nil {
			return err
		}
		if err := service.Manifests.CompareAndSwap(ctx, manifest.LookupKey, manifest.Version, manifest.StateDigest, next); err != nil {
			return err
		}
		manifest = next
		quarantineVerified = true
	}
	if manifest.Stage == DeleteStageQuarantined {
		if !quarantineVerified {
			quarantine, err := service.Storage.Quarantine(ctx, manifest.Source, manifest.LookupKey, manifest.ContentDigest, manifest.ContentSize)
			if err != nil {
				return err
			}
			if quarantine != manifest.Quarantine {
				return conflictError("asset_delete_quarantine_identity", nil)
			}
		}
		absent, err := service.Repository.DeleteAssetAbsent(ctx, command.OwnerID, command.Kind, command.AssetID)
		if err != nil {
			return attachmentDependencyError("asset_delete_verify", err)
		}
		if !absent {
			deleted, deleteErr := service.Repository.DeleteAsset(ctx, command.OwnerID, command.Kind, command.AssetID)
			if deleteErr != nil || !deleted {
				restoreErr := service.Storage.Restore(ctx, manifest.Source, manifest.Quarantine, manifest.ContentDigest, manifest.ContentSize)
				if restoreErr != nil {
					return contentError(ErrorPartialWrite, "asset_delete_restore", errors.Join(deleteErr, restoreErr))
				}
				if deleteErr != nil {
					return attachmentDependencyError("asset_delete_metadata", deleteErr)
				}
				return conflictError("asset_delete_metadata", nil)
			}
		}
		next, err := manifest.Advance(DeleteStageMetadata, manifest.Quarantine, now())
		if err != nil {
			return err
		}
		if err := service.Manifests.CompareAndSwap(ctx, manifest.LookupKey, manifest.Version, manifest.StateDigest, next); err != nil {
			return err
		}
		manifest = next
	}
	if manifest.Stage == DeleteStageMetadata {
		if err := service.Storage.Purge(ctx, manifest.Quarantine, manifest.ContentDigest, manifest.ContentSize); err != nil {
			return err
		}
		next, err := manifest.Terminal(now())
		if err != nil {
			return err
		}
		if err := service.Manifests.CompareAndSwap(ctx, manifest.LookupKey, manifest.Version, manifest.StateDigest, next); err != nil {
			return err
		}
	}
	return nil
}
