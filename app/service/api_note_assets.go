package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	applicationcontent "github.com/yangphere/leanote/app/application/content"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	. "github.com/yangphere/leanote/app/lea"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const apiNoteAssetUploadAction = "api_note_asset_upload"

// APINoteAssetCandidate is an opaque, already bounded request asset. Its
// digest is safe for the controller to bind into a note operation receipt.
type APINoteAssetCandidate struct {
	actorID, noteID, assetID domain.ObjectID
	isAttach                 bool
	stable                   bool
	data                     []byte
	name, title, extension   string
	Digest                   string
}

type APINoteAssetPrepareInput struct {
	ActorID, NoteID, AssetID, OriginalFilename string
	IsAttach                                   bool
	Reader                                     io.ReadCloser
}

type apiPreNoteManifestScanner interface {
	ListActivePreNoteCreateManifests(context.Context, string, int) ([]applicationcontent.CreateManifest, bool, error)
}

type apiPreNoteRecoveryWorkflow interface {
	ParentExists(context.Context, domain.ObjectID, domain.ObjectID) (bool, error)
	Finalize(context.Context, applicationcontent.PreNoteAssetIdentity) error
	Discard(context.Context, applicationcontent.PreNoteAssetIdentity) error
}

type apiPreNoteRuntime struct {
	content   applicationcontent.ContentStore
	manifests applicationcontent.DeleteManifestStore
	lifecycle applicationcontent.DeleteStorage
	creates   *applicationcontent.CreateRepairService
}

func configuredAPIPreNoteRuntime() (apiPreNoteRuntime, error) {
	runtime := apiPreNoteRuntime{
		content: contentStore, manifests: contentDeleteManifests, lifecycle: contentLifecycle, creates: contentCreateRepair,
	}
	if err := runtime.validate(); err != nil {
		return apiPreNoteRuntime{}, err
	}
	return runtime, nil
}

func (runtime apiPreNoteRuntime) validate() error {
	if runtime.content == nil || runtime.manifests == nil || runtime.lifecycle == nil || runtime.creates == nil {
		return applicationcontent.NewError(applicationcontent.ErrorDependency, "api_pre_note_runtime", nil)
	}
	return nil
}

func recoverAPINotePreNotes(ctx context.Context, scanner apiPreNoteManifestScanner, workflow apiPreNoteRecoveryWorkflow, limit int) (applicationcontent.CreateRecoveryResult, error) {
	if scanner == nil || workflow == nil || limit <= 0 {
		return applicationcontent.CreateRecoveryResult{}, applicationcontent.NewError(applicationcontent.ErrorValidation, "api_pre_note_recovery_input", nil)
	}
	manifests, truncated, err := scanner.ListActivePreNoteCreateManifests(ctx, apiNoteAssetUploadAction, limit)
	result := applicationcontent.CreateRecoveryResult{Scanned: len(manifests), Truncated: truncated}
	if err != nil {
		return result, err
	}
	for _, manifest := range manifests {
		if err := ctx.Err(); err != nil {
			return result, applicationcontent.NewError(applicationcontent.ErrorTimeout, "api_pre_note_recovery_canceled", err)
		}
		identity, err := apiPreNoteIdentityFromManifest(manifest)
		if err != nil {
			return result, err
		}
		committed, err := workflow.ParentExists(ctx, identity.OwnerID, identity.ParentID)
		if err != nil {
			return result, err
		}
		if committed {
			err = workflow.Finalize(ctx, identity)
			if err == nil {
				result.Committed++
			}
		} else {
			err = workflow.Discard(ctx, identity)
			if err == nil {
				result.Discarded++
			}
		}
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

func apiPreNoteIdentityFromManifest(manifest applicationcontent.CreateManifest) (applicationcontent.PreNoteAssetIdentity, error) {
	identity := applicationcontent.PreNoteAssetIdentity{
		Action: manifest.Action, OwnerID: manifest.OwnerID, RecordOwnerID: manifest.RecordOwnerID,
		ParentID: manifest.ParentID, Kind: manifest.Kind, AssetID: manifest.AssetID,
	}
	lookupKey, err := identity.LookupKey()
	if err != nil {
		return applicationcontent.PreNoteAssetIdentity{}, err
	}
	if !manifest.PreNote || manifest.Action != apiNoteAssetUploadAction || lookupKey != manifest.LookupKey {
		return applicationcontent.PreNoteAssetIdentity{}, applicationcontent.NewError(applicationcontent.ErrorConflict, "api_pre_note_recovery_manifest", nil)
	}
	return identity, nil
}

type apiPreNoteStartupRecovery struct {
	scanner apiPreNoteManifestScanner
	runtime apiPreNoteRuntime
}

func (recovery apiPreNoteStartupRecovery) RecoverAPIPreNotes(ctx context.Context, limit int) (applicationcontent.CreateRecoveryResult, error) {
	if err := recovery.runtime.validate(); err != nil {
		return applicationcontent.CreateRecoveryResult{}, err
	}
	return recoverAPINotePreNotes(ctx, recovery.scanner, recovery, limit)
}

func (recovery apiPreNoteStartupRecovery) ParentExists(ctx context.Context, ownerID, noteID domain.ObjectID) (bool, error) {
	if db.Notes == nil {
		return false, db.ErrMongoClientNotInitialized
	}
	var note info.Note
	err := db.Notes.FindContext(ctx, bson.M{"_id": noteID, "UserId": ownerID}).One(&note)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (recovery apiPreNoteStartupRecovery) Finalize(ctx context.Context, identity applicationcontent.PreNoteAssetIdentity) error {
	_, err := recovery.runtime.creates.FinalizePreNote(ctx, identity)
	return err
}

func (recovery apiPreNoteStartupRecovery) Discard(ctx context.Context, identity applicationcontent.PreNoteAssetIdentity) error {
	return cleanupAPINotePreNoteAsset(ctx, recovery.runtime, identity)
}

func (this *AttachService) PrepareAPINoteAsset(input APINoteAssetPrepareInput) (APINoteAssetCandidate, string) {
	defer input.Reader.Close()
	actorID, err := strictAPIAssetID(input.ActorID)
	if err != nil {
		return APINoteAssetCandidate{}, "fileRequired"
	}
	noteID, err := strictAPIAssetID(input.NoteID)
	if err != nil {
		return APINoteAssetCandidate{}, "fileRequired"
	}
	assetID := db.NewObjectID()
	if input.AssetID != "" {
		assetID, err = strictAPIAssetID(input.AssetID)
		if err != nil {
			return APINoteAssetCandidate{}, "fileRequired"
		}
	}
	key := "uploadImageSize"
	if input.IsAttach {
		key = "uploadAttachSize"
	}
	limit, err := configService.GetUploadLimitBytes(key)
	if err != nil {
		return APINoteAssetCandidate{}, "uploadConfigError"
	}
	data, err := applicationcontent.ReadBounded(input.Reader, limit)
	if err != nil {
		var contentErr *applicationcontent.Error
		if errors.As(err, &contentErr) && contentErr.Category == applicationcontent.ErrorTooLarge {
			return APINoteAssetCandidate{}, "fileIsTooLarge"
		}
		return APINoteAssetCandidate{}, "fileRequired"
	}
	if len(data) == 0 {
		return APINoteAssetCandidate{}, "fileRequired"
	}
	title, err := applicationcontent.CleanVisibleText(input.OriginalFilename, !input.IsAttach)
	if err != nil {
		if input.IsAttach {
			return APINoteAssetCandidate{}, "invalidFilename"
		}
		return APINoteAssetCandidate{}, "invalidTitle"
	}
	_, extension := SplitFilename(title)
	extension = strings.ToLower(extension)
	if !input.IsAttach {
		media, err := applicationcontent.ValidateImage(data, extension, applicationcontent.HardImageBudget())
		if err != nil {
			return APINoteAssetCandidate{}, "notImage"
		}
		extension = media.Extension
	}
	digest := sha256.Sum256(data)
	return APINoteAssetCandidate{actorID: actorID, noteID: noteID, assetID: assetID, isAttach: input.IsAttach, stable: input.AssetID != "", data: data, title: title, extension: extension, Digest: fmtDigest(digest)}, ""
}

func strictAPIAssetID(value string) (domain.ObjectID, error) {
	id, err := domain.ParseObjectID(strings.TrimSpace(value))
	if err != nil || id.IsZero() {
		return domain.ObjectID{}, errors.New("invalid API asset identity")
	}
	return id, nil
}

func fmtDigest(digest [sha256.Size]byte) string { return fmt.Sprintf("%x", digest) }

func apiPreNoteAssetIdentity(ownerID, noteID domain.ObjectID, file info.NoteFile) (applicationcontent.PreNoteAssetIdentity, error) {
	assetID, err := strictAPIAssetID(file.FileId)
	if err != nil {
		return applicationcontent.PreNoteAssetIdentity{}, err
	}
	kind := applicationcontent.AssetImage
	if file.IsAttach {
		kind = applicationcontent.AssetAttachment
	}
	identity := applicationcontent.PreNoteAssetIdentity{
		Action: apiNoteAssetUploadAction, OwnerID: ownerID, RecordOwnerID: ownerID,
		ParentID: noteID, Kind: kind, AssetID: assetID.Hex(),
	}
	if _, err := identity.LookupKey(); err != nil {
		return applicationcontent.PreNoteAssetIdentity{}, err
	}
	return identity, nil
}

func (this *AttachService) PublishAPINoteAsset(candidate APINoteAssetCandidate) (bool, string, string) {
	if contentCreateRepair == nil {
		return false, "partial_write", ""
	}
	name := candidate.assetID.Hex() + candidate.extension
	kind := applicationcontent.AssetImage
	directory := "images"
	if candidate.isAttach {
		kind, directory = applicationcontent.AssetAttachment, "attachs"
	}
	storedPath := "files/" + GetRandomFilePath(candidate.actorID.Hex(), candidate.assetID.Hex()) + "/" + directory + "/" + name
	logical, err := applicationcontent.ParseStoredPath(storedPath)
	if err != nil {
		return false, "partial_write", ""
	}
	digest := sha256.Sum256(candidate.data)
	identity := applicationcontent.CreateIdentity{Action: apiNoteAssetUploadAction, OwnerID: candidate.actorID, RecordOwnerID: candidate.actorID, ParentID: candidate.noteID, Kind: kind, AssetID: candidate.assetID.Hex(), Generation: 0, PreNote: true, ContentDigest: digest, ContentSize: int64(len(candidate.data))}
	createdAt := time.Now()
	if candidate.stable {
		createdAt = time.Unix(0, 0).UTC()
	}
	if candidate.isAttach {
		row := info.Attach{AttachId: candidate.assetID, NoteId: candidate.noteID, UploadUserId: candidate.actorID, Name: name, Title: candidate.title, Type: strings.TrimPrefix(candidate.extension, "."), Path: storedPath, Size: int64(len(candidate.data)), CreatedTime: createdAt}
		identity.RecordDigest = contentCreateAttachmentRecordDigest(row)
		_, err = contentCreateRepair.Execute(context.Background(), applicationcontent.CreateRepairCommand{Identity: identity, Destination: logical, Source: bytes.NewReader(candidate.data), ApplyMetadata: func(ctx context.Context) error { return this.insertAttachIfMissing(ctx, row, candidate.actorID) }})
	} else {
		row := info.File{FileId: candidate.assetID, UserId: candidate.actorID, Name: name, Title: candidate.title, Path: storedPath, Size: int64(len(candidate.data)), CreatedTime: createdAt}
		identity.RecordDigest = contentCreateImageRecordDigest(row)
		_, err = contentCreateRepair.Execute(context.Background(), applicationcontent.CreateRepairCommand{Identity: identity, Destination: logical, Source: bytes.NewReader(candidate.data), ApplyMetadata: func(ctx context.Context) error { return db.Files.InsertContext(ctx, row) }})
	}
	if err != nil {
		return false, "partial_write", ""
	}
	return true, "", candidate.assetID.Hex()
}

// CleanupAPINoteAssets is the pre-note counterpart of note delete. It proves
// the note is absent, then uses the frozen create manifest and the ordinary
// delete lifecycle to remove only request-owned assets immediately.
func (this *AttachService) CleanupAPINoteAssets(noteID, ownerID string, files []info.NoteFile) error {
	note, err := strictAPIAssetID(noteID)
	if err != nil {
		return err
	}
	owner, err := strictAPIAssetID(ownerID)
	if err != nil {
		return err
	}
	var existing info.Note
	err = db.Notes.FindContext(context.Background(), bson.M{"_id": note, "UserId": owner}).One(&existing)
	if err == nil {
		return errors.New("api note create already committed")
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return err
	}
	runtime, err := configuredAPIPreNoteRuntime()
	if err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(files))
	for _, file := range files {
		if !file.HasBody || strings.TrimSpace(file.FileId) == "" {
			continue
		}
		identity, err := apiPreNoteAssetIdentity(owner, note, file)
		if err != nil {
			return err
		}
		lookupKey, err := identity.LookupKey()
		if err != nil {
			return err
		}
		if _, found := seen[lookupKey]; found {
			continue
		}
		seen[lookupKey] = struct{}{}
		if err := cleanupAPINotePreNoteAsset(context.Background(), runtime, identity); err != nil {
			return err
		}
	}
	return nil
}

func cleanupAPINotePreNoteAsset(ctx context.Context, runtime apiPreNoteRuntime, identity applicationcontent.PreNoteAssetIdentity) error {
	if err := runtime.validate(); err != nil {
		return err
	}
	manifest, err := runtime.creates.PreNoteManifest(ctx, identity)
	if err != nil {
		return err
	}
	if manifest.Stage == applicationcontent.CreateStageTerminal {
		if manifest.Outcome == applicationcontent.CreateOutcomeDiscarded {
			return nil
		}
		return applicationcontent.NewError(applicationcontent.ErrorConflict, "api_pre_note_cleanup_committed", nil)
	}
	repository := apiPreNoteDeleteRepository{ownerID: identity.OwnerID, noteID: identity.ParentID, recordDigest: manifest.RecordDigest}
	assetID, err := strictAPIAssetID(identity.AssetID)
	if err != nil {
		return err
	}
	_, rowErr := repository.LoadDeleteAsset(ctx, identity.Kind, assetID)
	if errors.Is(rowErr, mongo.ErrNoDocuments) {
		return runtime.creates.DiscardPreNote(ctx, identity)
	}
	if rowErr != nil {
		return rowErr
	}
	if _, err := runtime.creates.VerifyPreNote(ctx, identity); err != nil {
		return err
	}
	deleteService := applicationcontent.DeleteAssetService{
		Repository: repository, Content: runtime.content, Manifests: runtime.manifests, Storage: runtime.lifecycle,
	}
	if err := deleteService.Delete(ctx, applicationcontent.DeleteAssetCommand{
		Action: "discard_api_note_pre_note_asset", OwnerID: identity.OwnerID, Kind: identity.Kind, AssetID: assetID, OperationID: manifest.LookupKey,
	}); err != nil {
		return err
	}
	return runtime.creates.DiscardPreNote(ctx, identity)
}

// FinalizeAPINoteAssets retires pre-note manifests only after the note write
// is visible under the same owner. A retry can safely repeat this transition.
func (this *AttachService) FinalizeAPINoteAssets(noteID, ownerID string, files []info.NoteFile) error {
	note, err := strictAPIAssetID(noteID)
	if err != nil {
		return err
	}
	owner, err := strictAPIAssetID(ownerID)
	if err != nil {
		return err
	}
	if contentCreateRepair == nil {
		return applicationcontent.NewError(applicationcontent.ErrorDependency, "api_pre_note_finalize_runtime", nil)
	}
	var existing info.Note
	if err := db.Notes.FindContext(context.Background(), bson.M{"_id": note, "UserId": owner}).One(&existing); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(files))
	for _, file := range files {
		if !file.HasBody || strings.TrimSpace(file.FileId) == "" {
			continue
		}
		identity, err := apiPreNoteAssetIdentity(owner, note, file)
		if err != nil {
			return err
		}
		lookupKey, err := identity.LookupKey()
		if err != nil {
			return err
		}
		if _, found := seen[lookupKey]; found {
			continue
		}
		seen[lookupKey] = struct{}{}
		if _, err := contentCreateRepair.FinalizePreNote(context.Background(), identity); err != nil {
			return err
		}
	}
	return nil
}

type apiPreNoteDeleteRepository struct {
	ownerID      domain.ObjectID
	noteID       domain.ObjectID
	recordDigest [sha256.Size]byte
}

func (repository apiPreNoteDeleteRepository) LoadDeleteAsset(ctx context.Context, kind applicationcontent.AssetKind, assetID domain.ObjectID) (applicationcontent.DeleteAssetMetadata, error) {
	if err := repository.noteAbsent(ctx); err != nil {
		return applicationcontent.DeleteAssetMetadata{}, err
	}
	switch kind {
	case applicationcontent.AssetAttachment:
		var attachment info.Attach
		if err := db.Attachs.FindContext(ctx, bson.M{"_id": assetID, "NoteId": repository.noteID, "UploadUserId": repository.ownerID}).One(&attachment); err != nil {
			return applicationcontent.DeleteAssetMetadata{}, err
		}
		if contentCreateAttachmentRecordDigest(attachment) != repository.recordDigest {
			return applicationcontent.DeleteAssetMetadata{}, applicationcontent.NewError(applicationcontent.ErrorConflict, "api_pre_note_attachment_record", nil)
		}
		return applicationcontent.DeleteAssetMetadata{AssetID: attachment.AttachId, OwnerID: repository.ownerID, Kind: kind, StoredPath: attachment.Path, Size: attachment.Size}, nil
	case applicationcontent.AssetImage:
		var image info.File
		if err := db.Files.FindContext(ctx, bson.M{"_id": assetID, "UserId": repository.ownerID, "Type": ""}).One(&image); err != nil {
			return applicationcontent.DeleteAssetMetadata{}, err
		}
		if contentCreateImageRecordDigest(image) != repository.recordDigest {
			return applicationcontent.DeleteAssetMetadata{}, applicationcontent.NewError(applicationcontent.ErrorConflict, "api_pre_note_image_record", nil)
		}
		if err := repository.imageUnreferenced(ctx, assetID); err != nil {
			return applicationcontent.DeleteAssetMetadata{}, err
		}
		return applicationcontent.DeleteAssetMetadata{AssetID: image.FileId, OwnerID: repository.ownerID, Kind: kind, StoredPath: image.Path, Size: image.Size}, nil
	default:
		return applicationcontent.DeleteAssetMetadata{}, applicationcontent.NewError(applicationcontent.ErrorValidation, "api_pre_note_delete_kind", nil)
	}
}

func (repository apiPreNoteDeleteRepository) DeleteAsset(ctx context.Context, ownerID domain.ObjectID, kind applicationcontent.AssetKind, assetID domain.ObjectID) (bool, error) {
	if ownerID != repository.ownerID {
		return false, applicationcontent.NewError(applicationcontent.ErrorConflict, "api_pre_note_delete_owner", nil)
	}
	if err := repository.noteAbsent(ctx); err != nil {
		return false, err
	}
	switch kind {
	case applicationcontent.AssetAttachment:
		var attachment info.Attach
		if err := db.Attachs.FindContext(ctx, bson.M{"_id": assetID, "NoteId": repository.noteID, "UploadUserId": ownerID}).One(&attachment); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return false, nil
			}
			return false, err
		}
		if contentCreateAttachmentRecordDigest(attachment) != repository.recordDigest {
			return false, applicationcontent.NewError(applicationcontent.ErrorConflict, "api_pre_note_attachment_record", nil)
		}
		err := db.Attachs.RemoveContext(ctx, bson.M{"_id": assetID, "NoteId": repository.noteID, "UploadUserId": ownerID})
		if errors.Is(err, mongo.ErrNoDocuments) {
			return false, nil
		}
		return err == nil, err
	case applicationcontent.AssetImage:
		var image info.File
		if err := db.Files.FindContext(ctx, bson.M{"_id": assetID, "UserId": ownerID, "Type": ""}).One(&image); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return false, nil
			}
			return false, err
		}
		if contentCreateImageRecordDigest(image) != repository.recordDigest {
			return false, applicationcontent.NewError(applicationcontent.ErrorConflict, "api_pre_note_image_record", nil)
		}
		if err := repository.imageUnreferenced(ctx, assetID); err != nil {
			return false, err
		}
		err := db.Files.RemoveContext(ctx, bson.M{"_id": assetID, "UserId": ownerID, "Type": ""})
		if errors.Is(err, mongo.ErrNoDocuments) {
			return false, nil
		}
		return err == nil, err
	default:
		return false, applicationcontent.NewError(applicationcontent.ErrorValidation, "api_pre_note_delete_kind", nil)
	}
}

func (repository apiPreNoteDeleteRepository) DeleteAssetAbsent(ctx context.Context, ownerID domain.ObjectID, kind applicationcontent.AssetKind, assetID domain.ObjectID) (bool, error) {
	if ownerID != repository.ownerID {
		return false, applicationcontent.NewError(applicationcontent.ErrorConflict, "api_pre_note_delete_owner", nil)
	}
	if err := repository.noteAbsent(ctx); err != nil {
		return false, err
	}
	switch kind {
	case applicationcontent.AssetAttachment:
		count, err := db.Attachs.FindContext(ctx, bson.M{"_id": assetID, "NoteId": repository.noteID, "UploadUserId": ownerID}).Count()
		return count == 0, err
	case applicationcontent.AssetImage:
		count, err := db.Files.FindContext(ctx, bson.M{"_id": assetID, "UserId": ownerID, "Type": ""}).Count()
		if err != nil || count != 0 {
			return false, err
		}
		err = repository.imageUnreferenced(ctx, assetID)
		return err == nil, err
	default:
		return false, applicationcontent.NewError(applicationcontent.ErrorValidation, "api_pre_note_delete_kind", nil)
	}
}

func (repository apiPreNoteDeleteRepository) noteAbsent(ctx context.Context) error {
	var note info.Note
	err := db.Notes.FindContext(ctx, bson.M{"_id": repository.noteID, "UserId": repository.ownerID}).One(&note)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil
	}
	if err == nil {
		return applicationcontent.NewError(applicationcontent.ErrorConflict, "api_pre_note_parent_committed", nil)
	}
	return err
}

func (repository apiPreNoteDeleteRepository) imageUnreferenced(ctx context.Context, assetID domain.ObjectID) error {
	count, err := db.NoteImages.FindContext(ctx, bson.M{"ImageId": assetID}).Count()
	if err != nil {
		return err
	}
	if count != 0 {
		return applicationcontent.NewError(applicationcontent.ErrorConflict, "api_pre_note_image_referenced", nil)
	}
	return nil
}
