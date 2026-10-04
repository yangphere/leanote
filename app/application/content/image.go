package content

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"path"
	"strings"
	"time"

	"github.com/yangphere/leanote/app/domain"
)

type PrepareImageUploadRequest struct {
	Reader       io.Reader
	Limit        int64
	OriginalName string
	DisplayName  string
	Budget       ImageBudget
}

type PreparedImageUpload struct {
	Data        []byte
	DisplayName string
	Extension   string
	MediaType   string
	Metadata    ImageMetadata
}

func PrepareImageUpload(request PrepareImageUploadRequest) (PreparedImageUpload, error) {
	name := path.Base(strings.ReplaceAll(request.OriginalName, `\`, "/"))
	name, err := CleanVisibleText(name, false)
	if err != nil {
		return PreparedImageUpload{}, err
	}
	displayName := request.DisplayName
	if displayName == "" {
		displayName = name
	}
	displayName, err = CleanVisibleText(displayName, false)
	if err != nil {
		return PreparedImageUpload{}, err
	}
	extension := strings.ToLower(path.Ext(name))
	data, err := ReadBounded(request.Reader, request.Limit)
	if err != nil {
		return PreparedImageUpload{}, err
	}
	metadata, err := ValidateImage(data, extension, request.Budget)
	if err != nil {
		return PreparedImageUpload{}, err
	}
	return PreparedImageUpload{Data: data, DisplayName: displayName, Extension: metadata.Extension, MediaType: metadata.MIME, Metadata: metadata}, nil
}

type StoredImage struct {
	ImageID     domain.ObjectID
	OwnerID     domain.ObjectID
	AlbumID     domain.ObjectID
	StoredName  string
	DisplayName string
	StoredPath  string
	Size        int64
	Generation  int64
}

type ImageNote struct {
	NoteID     domain.ObjectID
	OwnerID    domain.ObjectID
	NotebookID domain.ObjectID
	Public     bool
}

type ImageRepository interface {
	LoadImage(context.Context, domain.ObjectID) (StoredImage, error)
	ReferencingNotes(context.Context, domain.ObjectID) ([]ImageNote, error)
	DeleteImageAndProjection(context.Context, domain.ObjectID, domain.ObjectID) (bool, error)
	ImageAbsent(context.Context, domain.ObjectID, domain.ObjectID) (bool, error)
}

type ImagePermissionPort interface {
	CanReadNote(context.Context, ImageNote, domain.ObjectID) (bool, error)
}

type ImageAccessService struct {
	Repository  ImageRepository
	Permissions ImagePermissionPort
	Store       ContentStore
}

type ImageDownload struct {
	Reader io.ReadCloser
	Size   int64
	Name   string
}

func (service ImageAccessService) Open(ctx context.Context, actorID, imageID domain.ObjectID) (ImageDownload, error) {
	if service.Repository == nil {
		return ImageDownload{}, contentError(ErrorDependency, "image_repository_missing", nil)
	}
	if service.Store == nil {
		return ImageDownload{}, contentError(ErrorDependency, "image_store_missing", nil)
	}
	if imageID.IsZero() {
		return ImageDownload{}, validationError("image_identity", nil)
	}
	image, err := service.Repository.LoadImage(ctx, imageID)
	if err != nil {
		return ImageDownload{}, attachmentDependencyError("image_lookup", err)
	}
	if image.ImageID != imageID || image.OwnerID.IsZero() || image.StoredPath == "" || image.Size < 0 {
		return ImageDownload{}, conflictError("image_identity", nil)
	}
	if actorID != image.OwnerID {
		notes, err := service.Repository.ReferencingNotes(ctx, imageID)
		if err != nil {
			return ImageDownload{}, attachmentDependencyError("image_references", err)
		}
		allowed := false
		for _, note := range notes {
			if note.NoteID.IsZero() || note.OwnerID.IsZero() {
				return ImageDownload{}, conflictError("image_reference_identity", nil)
			}
			if note.Public {
				allowed = true
			}
		}
		if !allowed && !actorID.IsZero() {
			if service.Permissions == nil {
				return ImageDownload{}, contentError(ErrorDependency, "image_permission_port_missing", nil)
			}
			var permissionErr error
			for _, note := range notes {
				readable, err := service.Permissions.CanReadNote(ctx, note, actorID)
				if err != nil {
					permissionErr = errors.Join(permissionErr, err)
					continue
				}
				if readable {
					allowed = true
					break
				}
			}
			if !allowed && permissionErr != nil {
				return ImageDownload{}, attachmentDependencyError("image_permission", permissionErr)
			}
		}
		if !allowed {
			return ImageDownload{}, contentError(ErrorUnauthorized, "image_read", nil)
		}
	}
	logical, err := ParseStoredPath(image.StoredPath)
	if err != nil {
		return ImageDownload{}, err
	}
	opened, err := service.Store.Open(ctx, logical)
	if err != nil {
		return ImageDownload{}, err
	}
	if opened.Reader == nil {
		return ImageDownload{}, storageError("image_reader_missing", nil)
	}
	if opened.Size != image.Size {
		return ImageDownload{}, conflictError("image_size_mismatch", opened.Reader.Close())
	}
	return ImageDownload{Reader: opened.Reader, Size: opened.Size, Name: image.StoredName}, nil
}

type AlbumMutation struct {
	AlbumID domain.ObjectID
	Name    string
}

func NormalizeAlbumMutation(albumIDText, name string) (AlbumMutation, error) {
	albumID, err := domain.ParseObjectID(albumIDText)
	if err != nil || albumID.IsZero() {
		return AlbumMutation{}, validationError("album_identity", err)
	}
	name, err = CleanVisibleText(name, false)
	if err != nil {
		return AlbumMutation{}, err
	}
	return AlbumMutation{AlbumID: albumID, Name: name}, nil
}

type DeleteManifestStore interface {
	DeleteManifestReader
	Create(context.Context, DeleteManifest) error
	CompareAndSwap(context.Context, string, uint64, [sha256.Size]byte, DeleteManifest) error
}

type DeleteStorage interface {
	Quarantine(context.Context, LogicalPath, string, [sha256.Size]byte, int64) (LogicalPath, error)
	Restore(context.Context, LogicalPath, LogicalPath, [sha256.Size]byte, int64) error
	Purge(context.Context, LogicalPath, [sha256.Size]byte, int64) error
}

type ImageDeleteCommand struct {
	OwnerID     domain.ObjectID
	ImageID     domain.ObjectID
	OperationID string
}

type ImageDeleteResult struct {
	Deleted bool
}

type ImageDeleteService struct {
	Repository ImageRepository
	Content    ContentStore
	Manifests  DeleteManifestStore
	Storage    DeleteStorage
	Now        func() time.Time
}

func (service ImageDeleteService) Delete(ctx context.Context, command ImageDeleteCommand) (ImageDeleteResult, error) {
	if command.OwnerID.IsZero() || command.ImageID.IsZero() {
		return ImageDeleteResult{}, validationError("image_delete_input", nil)
	}
	if service.Repository == nil || service.Content == nil || service.Manifests == nil || service.Storage == nil {
		return ImageDeleteResult{}, contentError(ErrorDependency, "image_delete_dependency_missing", nil)
	}
	now := service.Now
	if now == nil {
		now = time.Now
	}
	identity := DeleteIdentity{Action: "delete_image", OwnerID: command.OwnerID, Kind: AssetImage, AssetID: command.ImageID.Hex(), OperationID: command.OperationID}
	lookupKey, err := identity.LookupKey()
	if err != nil {
		return ImageDeleteResult{}, err
	}
	manifest, found, err := service.Manifests.LoadDeleteManifest(ctx, lookupKey)
	if err != nil {
		return ImageDeleteResult{}, err
	}
	if !found {
		image, err := service.Repository.LoadImage(ctx, command.ImageID)
		if err != nil {
			return ImageDeleteResult{}, attachmentDependencyError("image_delete_lookup", err)
		}
		if image.ImageID != command.ImageID || image.OwnerID != command.OwnerID || image.Generation < 0 {
			return ImageDeleteResult{}, contentError(ErrorNotFound, "image_delete_missing", nil)
		}
		source, err := ParseStoredPath(image.StoredPath)
		if err != nil {
			return ImageDeleteResult{}, err
		}
		digest, err := contentDigest(ctx, service.Content, source, image.Size)
		if err != nil {
			return ImageDeleteResult{}, err
		}
		identity.Generation = image.Generation
		identity.ContentDigest = digest
		identity.ContentSize = image.Size
		manifest, err = NewDeleteManifest(identity, source, now())
		if err != nil {
			return ImageDeleteResult{}, err
		}
		if err := service.Manifests.Create(ctx, manifest); err != nil {
			return ImageDeleteResult{}, err
		}
	} else if manifest.OwnerID != command.OwnerID || manifest.AssetID != command.ImageID.Hex() || manifest.Kind != AssetImage {
		return ImageDeleteResult{}, conflictError("image_delete_manifest_identity", nil)
	} else if command.OperationID != "" {
		operationKey, err := identity.OperationKey()
		if err != nil || operationKey != manifest.OperationKey {
			return ImageDeleteResult{}, conflictError("image_delete_operation", err)
		}
	}
	if manifest.Stage == DeleteStageTerminal {
		return ImageDeleteResult{Deleted: true}, nil
	}
	quarantineVerified := false
	if manifest.Stage == DeleteStagePrepared {
		quarantine, err := service.Storage.Quarantine(ctx, manifest.Source, manifest.LookupKey, manifest.ContentDigest, manifest.ContentSize)
		if err != nil {
			return ImageDeleteResult{}, err
		}
		next, err := manifest.Advance(DeleteStageQuarantined, quarantine, now())
		if err != nil {
			return ImageDeleteResult{}, err
		}
		if err := service.Manifests.CompareAndSwap(ctx, manifest.LookupKey, manifest.Version, manifest.StateDigest, next); err != nil {
			return ImageDeleteResult{}, err
		}
		manifest = next
		quarantineVerified = true
	}
	if manifest.Stage == DeleteStageQuarantined {
		if !quarantineVerified {
			quarantine, err := service.Storage.Quarantine(ctx, manifest.Source, manifest.LookupKey, manifest.ContentDigest, manifest.ContentSize)
			if err != nil {
				return ImageDeleteResult{}, err
			}
			if quarantine != manifest.Quarantine {
				return ImageDeleteResult{}, conflictError("image_delete_quarantine_identity", nil)
			}
		}
		absent, err := service.Repository.ImageAbsent(ctx, command.OwnerID, command.ImageID)
		if err != nil {
			return ImageDeleteResult{}, attachmentDependencyError("image_delete_verify", err)
		}
		if !absent {
			deleted, deleteErr := service.Repository.DeleteImageAndProjection(ctx, command.OwnerID, command.ImageID)
			if deleteErr != nil || !deleted {
				restoreErr := service.Storage.Restore(ctx, manifest.Source, manifest.Quarantine, manifest.ContentDigest, manifest.ContentSize)
				if restoreErr != nil {
					return ImageDeleteResult{}, contentError(ErrorPartialWrite, "image_delete_restore", errors.Join(deleteErr, restoreErr))
				}
				if deleteErr != nil {
					return ImageDeleteResult{}, attachmentDependencyError("image_delete_metadata", deleteErr)
				}
				return ImageDeleteResult{}, conflictError("image_delete_metadata", nil)
			}
		}
		next, err := manifest.Advance(DeleteStageMetadata, manifest.Quarantine, now())
		if err != nil {
			return ImageDeleteResult{}, err
		}
		if err := service.Manifests.CompareAndSwap(ctx, manifest.LookupKey, manifest.Version, manifest.StateDigest, next); err != nil {
			return ImageDeleteResult{}, err
		}
		manifest = next
	}
	if manifest.Stage == DeleteStageMetadata {
		if err := service.Storage.Purge(ctx, manifest.Quarantine, manifest.ContentDigest, manifest.ContentSize); err != nil {
			return ImageDeleteResult{}, err
		}
		next, err := manifest.Terminal(now())
		if err != nil {
			return ImageDeleteResult{}, err
		}
		if err := service.Manifests.CompareAndSwap(ctx, manifest.LookupKey, manifest.Version, manifest.StateDigest, next); err != nil {
			return ImageDeleteResult{}, err
		}
	}
	return ImageDeleteResult{Deleted: true}, nil
}

func contentDigest(ctx context.Context, store ContentStore, source LogicalPath, expectedSize int64) ([sha256.Size]byte, error) {
	opened, err := store.Open(ctx, source)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	if opened.Reader == nil {
		return [sha256.Size]byte{}, storageError("image_delete_reader_missing", nil)
	}
	if opened.Size != expectedSize {
		return [sha256.Size]byte{}, conflictError("image_delete_size", opened.Reader.Close())
	}
	hash := sha256.New()
	_, readErr := io.Copy(hash, opened.Reader)
	closeErr := opened.Reader.Close()
	if readErr != nil || closeErr != nil {
		return [sha256.Size]byte{}, contentError(ErrorStorageUnavailable, "image_delete_digest", errors.Join(readErr, closeErr))
	}
	var digest [sha256.Size]byte
	copy(digest[:], hash.Sum(nil))
	return digest, nil
}
