package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"path/filepath"
	"sort"
	"strings"

	applicationcontent "github.com/yangphere/leanote/app/application/content"
	applicationnotes "github.com/yangphere/leanote/app/application/notes"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type contentAssetProvider struct{}

var ContentAssets applicationnotes.ContentAssetPort = contentAssetProvider{}

func (contentAssetProvider) ReconcileNote(ctx context.Context, command applicationnotes.ReconcileNoteAssetsCommand) error {
	receipt, assets, err := loadContentAssetReceipt(ctx, command.OwnerID, command.NoteID, command.OperationID)
	if err != nil {
		return err
	}
	if err := requireContentAssetReceiptKind(receipt, "note_save", "note_create"); err != nil {
		return err
	}
	if command.ActorID.IsZero() {
		return applicationcontent.NewError(applicationcontent.ErrorValidation, "note_asset_reconcile_identity", nil)
	}
	generation, err := contentAssetReceiptGeneration(receipt, command.Generation, true)
	if err != nil {
		return err
	}
	desiredAttachments, desiredImages, err := verifyDesiredNoteAssets(ctx, command.OwnerID, command.NoteID, assets)
	if err != nil {
		return err
	}
	var current []info.Attach
	if err := db.Attachs.FindContext(ctx, bson.M{"NoteId": command.NoteID}).All(&current); err != nil {
		return contentAssetDependency("note_asset_reconcile_list", err)
	}
	for _, attachment := range current {
		if _, keep := desiredAttachments[attachment.AttachId]; keep {
			continue
		}
		if err := noteAssetDeleteService().Delete(ctx, applicationcontent.DeleteAssetCommand{
			Action: "reconcile_note_attachment", OwnerID: command.OwnerID, Kind: applicationcontent.AssetAttachment,
			AssetID: attachment.AttachId, OperationID: command.OperationID + ":" + attachment.AttachId.Hex(),
		}); err != nil {
			return err
		}
	}
	if err := replaceNoteImageProjection(ctx, command.NoteID, desiredImages); err != nil {
		return err
	}
	if err := attachService.updateNoteAttachNumContext(ctx, command.NoteID, command.OwnerID, &generation, command.OperationID); err != nil {
		return contentAssetDependency("note_asset_reconcile_count", err)
	}
	return nil
}

func (contentAssetProvider) VerifyReconcileNote(ctx context.Context, command applicationnotes.ReconcileNoteAssetsCommand) (bool, error) {
	receipt, assets, err := loadContentAssetReceipt(ctx, command.OwnerID, command.NoteID, command.OperationID)
	if err != nil {
		return false, err
	}
	if err := requireContentAssetReceiptKind(receipt, "note_save", "note_create"); err != nil {
		return false, err
	}
	generation, err := contentAssetReceiptGeneration(receipt, command.Generation, false)
	if err != nil {
		return false, err
	}
	desiredAttachments, desiredImages, err := verifyDesiredNoteAssets(ctx, command.OwnerID, command.NoteID, assets)
	if err != nil {
		return false, err
	}
	var current []info.Attach
	if err := db.Attachs.FindContext(ctx, bson.M{"NoteId": command.NoteID}).All(&current); err != nil {
		return false, contentAssetDependency("note_asset_verify_list", err)
	}
	if len(current) != len(desiredAttachments) {
		return false, nil
	}
	for _, attachment := range current {
		if _, ok := desiredAttachments[attachment.AttachId]; !ok {
			return false, nil
		}
	}
	if ok, err := verifyNoteImageProjection(ctx, command.NoteID, desiredImages); err != nil || !ok {
		return ok, err
	}
	var note info.Note
	noteFilter := bson.M{"_id": command.NoteID, "UserId": command.OwnerID, "IsDeleted": false, "AttachNum": len(desiredAttachments)}
	noteFilter["Usn"] = generation
	err = db.Notes.FindContext(ctx, noteFilter).One(&note)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	if err != nil {
		return false, contentAssetDependency("note_asset_verify_note", err)
	}
	return true, nil
}

func (contentAssetProvider) CopyNote(ctx context.Context, command applicationnotes.CopyNoteAssetsCommand) (applicationnotes.CopyNoteAssetsResult, error) {
	receipt, assets, err := loadContentAssetReceipt(ctx, command.DestinationOwnerID, command.DestinationNoteID, command.OperationID)
	if err != nil {
		return applicationnotes.CopyNoteAssetsResult{}, err
	}
	if err := requireContentAssetReceiptKind(receipt, "note_create"); err != nil {
		return applicationnotes.CopyNoteAssetsResult{}, err
	}
	if command.ActorID.IsZero() || command.SourceOwnerID.IsZero() || strings.TrimSpace(command.AssetOperationID) == "" {
		return applicationnotes.CopyNoteAssetsResult{}, applicationcontent.NewError(applicationcontent.ErrorValidation, "note_asset_copy_identity", nil)
	}
	generation, err := contentAssetReceiptGeneration(receipt, command.Generation, true)
	if err != nil {
		return applicationnotes.CopyNoteAssetsResult{}, err
	}
	command.Generation = generation
	if err := validateFrozenCopyManifest(assets); err != nil {
		return applicationnotes.CopyNoteAssetsResult{}, err
	}
	content, err := noteImageService.CopyNoteImagesWithManifest(
		command.SourceNoteID.Hex(), command.SourceOwnerID.Hex(), command.DestinationNoteID.Hex(), command.Content,
		command.DestinationOwnerID.Hex(), command.AssetOperationID, assets,
	)
	if err != nil {
		return applicationnotes.CopyNoteAssetsResult{}, applicationcontent.NewError(applicationcontent.ErrorPartialWrite, "note_asset_copy_images", err)
	}
	for _, asset := range assets {
		if !asset.IsAttach {
			continue
		}
		if err := copyFrozenAttachment(ctx, command, asset); err != nil {
			return applicationnotes.CopyNoteAssetsResult{}, err
		}
	}
	count := 0
	for _, asset := range assets {
		if asset.IsAttach {
			count++
		}
	}
	if err := setNoteAttachNum(ctx, command.DestinationNoteID, command.DestinationOwnerID, command.Generation, count); err != nil {
		return applicationnotes.CopyNoteAssetsResult{}, err
	}
	return applicationnotes.CopyNoteAssetsResult{Content: content}, nil
}

func (contentAssetProvider) VerifyCopyNote(ctx context.Context, command applicationnotes.CopyNoteAssetsCommand) (bool, error) {
	receipt, assets, err := loadContentAssetReceipt(ctx, command.DestinationOwnerID, command.DestinationNoteID, command.OperationID)
	if err != nil {
		return false, err
	}
	if err := requireContentAssetReceiptKind(receipt, "note_create"); err != nil {
		return false, err
	}
	generation, err := contentAssetReceiptGeneration(receipt, command.Generation, true)
	if err != nil {
		return false, err
	}
	command.Generation = generation
	if err := validateFrozenCopyManifest(assets); err != nil {
		return false, err
	}
	if ok, err := verifyFrozenCopyRows(ctx, command, assets); err != nil || !ok {
		return ok, err
	}
	attachments, images, err := verifyDesiredNoteAssets(ctx, command.DestinationOwnerID, command.DestinationNoteID, assets)
	if err != nil {
		return false, err
	}
	if ok, err := verifyNoteImageProjection(ctx, command.DestinationNoteID, images); err != nil || !ok {
		return ok, err
	}
	var note info.Note
	err = db.Notes.FindContext(ctx, bson.M{
		"_id": command.DestinationNoteID, "UserId": command.DestinationOwnerID,
		"Usn": command.Generation, "AttachNum": len(attachments),
	}).One(&note)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	return err == nil, contentAssetDependency("note_asset_copy_verify", err)
}

func (contentAssetProvider) DeleteNote(ctx context.Context, command applicationnotes.DeleteNoteAssetsCommand) error {
	receipt, assets, err := loadContentAssetReceipt(ctx, command.OwnerID, command.NoteID, command.OperationID)
	if err != nil {
		return err
	}
	if err := requireContentAssetReceiptKind(receipt, "note_delete_cleanup"); err != nil {
		return err
	}
	if command.ActorID.IsZero() {
		return applicationcontent.NewError(applicationcontent.ErrorValidation, "note_asset_delete_actor", nil)
	}
	for _, asset := range assets {
		if !asset.IsAttach {
			continue
		}
		assetID, err := domain.ParseObjectID(asset.AssetID)
		if err != nil {
			return applicationcontent.NewError(applicationcontent.ErrorConflict, "note_asset_delete_manifest", err)
		}
		if err := noteAssetDeleteService().Delete(ctx, applicationcontent.DeleteAssetCommand{
			Action: "delete_note_attachment", OwnerID: command.OwnerID, Kind: applicationcontent.AssetAttachment,
			AssetID: assetID, OperationID: command.OperationID + ":" + asset.AssetID,
		}); err != nil {
			return err
		}
	}
	if _, err := db.NoteImages.RemoveAllContext(ctx, bson.M{"NoteId": command.NoteID}); err != nil {
		return contentAssetDependency("note_asset_delete_projection", err)
	}
	return nil
}

func (contentAssetProvider) VerifyDeleteNote(ctx context.Context, command applicationnotes.DeleteNoteAssetsCommand) (bool, error) {
	receipt, assets, err := loadContentAssetReceipt(ctx, command.OwnerID, command.NoteID, command.OperationID)
	if err != nil {
		return false, err
	}
	if err := requireContentAssetReceiptKind(receipt, "note_delete_cleanup"); err != nil {
		return false, err
	}
	for _, asset := range assets {
		if !asset.IsAttach {
			continue
		}
		assetID, err := domain.ParseObjectID(asset.AssetID)
		if err != nil {
			return false, applicationcontent.NewError(applicationcontent.ErrorConflict, "note_asset_delete_manifest", err)
		}
		absent, err := (mongoNoteAssetDeleteRepository{}).DeleteAssetAbsent(ctx, command.OwnerID, applicationcontent.AssetAttachment, assetID)
		if err != nil || !absent {
			return false, contentAssetDependency("note_asset_delete_verify", err)
		}
	}
	var image info.NoteImage
	err = db.NoteImages.FindContext(ctx, bson.M{"NoteId": command.NoteID}).One(&image)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return true, nil
	}
	return false, contentAssetDependency("note_asset_delete_verify_projection", err)
}

func loadContentAssetReceipt(ctx context.Context, ownerID, noteID domain.ObjectID, operationID string) (applicationnotes.OperationReceipt, []applicationnotes.OperationAsset, error) {
	if ownerID.IsZero() || noteID.IsZero() || strings.TrimSpace(operationID) == "" {
		return applicationnotes.OperationReceipt{}, nil, applicationcontent.NewError(applicationcontent.ErrorValidation, "note_asset_receipt_identity", nil)
	}
	receipt, err := db.GetWorkspaceOperation(ctx, ownerID, operationID)
	if err != nil {
		return applicationnotes.OperationReceipt{}, nil, contentAssetDependency("note_asset_receipt", err)
	}
	if receipt.OperationID != operationID || receipt.OwnerID != ownerID || receipt.ResourceID != noteID {
		return applicationnotes.OperationReceipt{}, nil, applicationcontent.NewError(applicationcontent.ErrorConflict, "note_asset_receipt_scope", nil)
	}
	if receipt.Status == applicationnotes.OperationFailed || receipt.Status == applicationnotes.OperationCompensated {
		return applicationnotes.OperationReceipt{}, nil, applicationcontent.NewError(applicationcontent.ErrorConflict, "note_asset_receipt_terminal", nil)
	}
	assets := append([]applicationnotes.OperationAsset(nil), receipt.Assets...)
	if err := validateContentAssetManifest(assets); err != nil {
		return applicationnotes.OperationReceipt{}, nil, err
	}
	return receipt, assets, nil
}

func requireContentAssetReceiptKind(receipt applicationnotes.OperationReceipt, allowed ...string) error {
	for _, kind := range allowed {
		if receipt.Kind == kind {
			return nil
		}
	}
	return applicationcontent.NewError(applicationcontent.ErrorConflict, "note_asset_receipt_kind", nil)
}

func contentAssetReceiptGeneration(receipt applicationnotes.OperationReceipt, supplied int, requireSupplied bool) (int, error) {
	if receipt.AssignedUSN <= 0 {
		return 0, applicationcontent.NewError(applicationcontent.ErrorConflict, "note_asset_receipt_generation", nil)
	}
	if supplied <= 0 {
		if requireSupplied {
			return 0, applicationcontent.NewError(applicationcontent.ErrorValidation, "note_asset_generation_required", nil)
		}
		return receipt.AssignedUSN, nil
	}
	if supplied != receipt.AssignedUSN {
		return 0, applicationcontent.NewError(applicationcontent.ErrorConflict, "note_asset_generation_conflict", nil)
	}
	return receipt.AssignedUSN, nil
}

func validateContentAssetManifest(assets []applicationnotes.OperationAsset) error {
	seen := make(map[string]struct{}, len(assets))
	lastIndex := -1
	for position, asset := range assets {
		if !db.IsValidObjectIDHex(asset.AssetID) || asset.Index < 0 || (position > 0 && asset.Index <= lastIndex) {
			return applicationcontent.NewError(applicationcontent.ErrorConflict, "note_asset_receipt_manifest", nil)
		}
		if _, duplicate := seen[asset.AssetID]; duplicate {
			return applicationcontent.NewError(applicationcontent.ErrorConflict, "note_asset_receipt_duplicate", nil)
		}
		seen[asset.AssetID] = struct{}{}
		lastIndex = asset.Index
	}
	return nil
}

func validateFrozenCopyManifest(assets []applicationnotes.OperationAsset) error {
	for _, asset := range assets {
		if !db.IsValidObjectIDHex(asset.LocalFileID) || !validSHA256Text(asset.ContentSHA256) || !validSHA256Text(asset.RecordSHA256) {
			return applicationcontent.NewError(applicationcontent.ErrorConflict, "note_asset_copy_frozen_manifest", nil)
		}
	}
	return nil
}

func validSHA256Text(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func verifyDesiredNoteAssets(ctx context.Context, ownerID, noteID domain.ObjectID, assets []applicationnotes.OperationAsset) (map[domain.ObjectID]struct{}, []domain.ObjectID, error) {
	attachments := make(map[domain.ObjectID]struct{})
	images := make([]domain.ObjectID, 0)
	for _, asset := range assets {
		assetID := db.MustObjectIDFromHex(asset.AssetID)
		if asset.IsAttach {
			var row info.Attach
			if err := db.Attachs.FindContext(ctx, bson.M{"_id": assetID, "NoteId": noteID}).One(&row); err != nil {
				return nil, nil, contentAssetDependency("note_asset_attachment_row", err)
			}
			if err := verifyNoteAssetContent(ctx, ownerID, applicationcontent.AssetAttachment, asset.AssetID, row.Path, row.Size, asset.ContentSHA256); err != nil {
				return nil, nil, err
			}
			attachments[assetID] = struct{}{}
			continue
		}
		var row info.File
		if err := db.Files.FindContext(ctx, bson.M{"_id": assetID, "UserId": ownerID, "Type": ""}).One(&row); err != nil {
			return nil, nil, contentAssetDependency("note_asset_image_row", err)
		}
		// API image preparation may normalize the uploaded bytes before publish;
		// the receipt digest binds request input while storage verification hashes
		// the normalized object described by the frozen owner row.
		if err := verifyNoteAssetContent(ctx, ownerID, applicationcontent.AssetImage, asset.AssetID, row.Path, row.Size, ""); err != nil {
			return nil, nil, err
		}
		images = append(images, assetID)
	}
	sort.Slice(images, func(i, j int) bool { return images[i].Hex() < images[j].Hex() })
	return attachments, images, nil
}

func verifyNoteAssetContent(ctx context.Context, ownerID domain.ObjectID, kind applicationcontent.AssetKind, assetID, storedPath string, size int64, digestText string) error {
	if contentStore == nil {
		return applicationcontent.NewError(applicationcontent.ErrorDependency, "note_asset_content_store", nil)
	}
	logical, err := applicationcontent.ParseStoredPath(storedPath)
	if err != nil {
		return err
	}
	digest, err := noteAssetDigest(ctx, logical, size, digestText)
	if err != nil {
		return err
	}
	verified, err := contentStore.Verify(ctx, applicationcontent.VerifyRequest{
		Identity:    applicationcontent.AssetIdentity{OperationID: "note-asset-verify", OwnerID: ownerID, Kind: kind, SourceID: assetID, DestinationID: assetID, Digest: digest},
		Destination: logical, ExpectedDigest: digest,
	})
	if err != nil {
		return err
	}
	if verified.Status != applicationcontent.VerificationApplied || verified.Size != size {
		return applicationcontent.NewError(applicationcontent.ErrorConflict, "note_asset_content_verify", nil)
	}
	return nil
}

func noteAssetDigest(ctx context.Context, logical applicationcontent.LogicalPath, size int64, digestText string) ([sha256.Size]byte, error) {
	var digest [sha256.Size]byte
	if digestText != "" {
		decoded, err := hex.DecodeString(digestText)
		if err != nil || len(decoded) != sha256.Size {
			return digest, applicationcontent.NewError(applicationcontent.ErrorConflict, "note_asset_digest", err)
		}
		copy(digest[:], decoded)
		return digest, nil
	}
	opened, err := contentStore.Open(ctx, logical)
	if err != nil {
		return digest, err
	}
	if opened.Reader == nil || opened.Size != size {
		if opened.Reader != nil {
			_ = opened.Reader.Close()
		}
		return digest, applicationcontent.NewError(applicationcontent.ErrorConflict, "note_asset_size", nil)
	}
	hash := sha256.New()
	_, readErr := io.Copy(hash, opened.Reader)
	closeErr := opened.Reader.Close()
	if readErr != nil || closeErr != nil {
		return digest, applicationcontent.NewError(applicationcontent.ErrorStorageUnavailable, "note_asset_read", errors.Join(readErr, closeErr))
	}
	copy(digest[:], hash.Sum(nil))
	return digest, nil
}

func replaceNoteImageProjection(ctx context.Context, noteID domain.ObjectID, imageIDs []domain.ObjectID) error {
	if _, err := db.NoteImages.RemoveAllContext(ctx, bson.M{"NoteId": noteID}); err != nil {
		return contentAssetDependency("note_asset_image_projection_clear", err)
	}
	for _, imageID := range imageIDs {
		if err := db.NoteImages.InsertContext(ctx, info.NoteImage{NoteId: noteID, ImageId: imageID}); err != nil {
			return contentAssetDependency("note_asset_image_projection_insert", err)
		}
	}
	return nil
}

func verifyNoteImageProjection(ctx context.Context, noteID domain.ObjectID, want []domain.ObjectID) (bool, error) {
	var rows []info.NoteImage
	if err := db.NoteImages.FindContext(ctx, bson.M{"NoteId": noteID}).All(&rows); err != nil {
		return false, contentAssetDependency("note_asset_image_projection_verify", err)
	}
	got := make([]domain.ObjectID, 0, len(rows))
	for _, row := range rows {
		got = append(got, row.ImageId)
	}
	sort.Slice(got, func(i, j int) bool { return got[i].Hex() < got[j].Hex() })
	if len(got) != len(want) {
		return false, nil
	}
	for i := range want {
		if got[i] != want[i] {
			return false, nil
		}
	}
	return true, nil
}

func copyFrozenAttachment(ctx context.Context, command applicationnotes.CopyNoteAssetsCommand, asset applicationnotes.OperationAsset) error {
	if contentStore == nil || contentCreateRepair == nil {
		return applicationcontent.NewError(applicationcontent.ErrorDependency, "note_asset_copy_runtime", nil)
	}
	if !db.IsValidObjectIDHex(asset.LocalFileID) || asset.AssetID != stableCopiedAttachID(command.AssetOperationID, asset.LocalFileID, command.DestinationOwnerID.Hex()).Hex() {
		return applicationcontent.NewError(applicationcontent.ErrorConflict, "note_asset_copy_manifest", nil)
	}
	destinationID := db.MustObjectIDFromHex(asset.AssetID)
	var source info.Attach
	if err := db.Attachs.FindContext(ctx, bson.M{"_id": db.MustObjectIDFromHex(asset.LocalFileID), "NoteId": command.SourceNoteID}).One(&source); err != nil {
		return contentAssetDependency("note_asset_copy_source", err)
	}
	data, err := readAttachmentSource(ctx, source)
	if err != nil {
		return err
	}
	destination, err := copiedAttachmentDestination(source, command.DestinationOwnerID, command.DestinationNoteID, command.AssetOperationID)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(data)
	recordDigest := contentCreateAttachmentRecordDigest(destination)
	if asset.ContentSHA256 != hex.EncodeToString(digest[:]) || asset.RecordSHA256 != hex.EncodeToString(recordDigest[:]) {
		return applicationcontent.NewError(applicationcontent.ErrorConflict, "note_asset_copy_frozen_attachment", nil)
	}
	var existing info.Attach
	existingErr := db.Attachs.FindContext(ctx, bson.M{"_id": destinationID, "NoteId": command.DestinationNoteID}).One(&existing)
	if existingErr == nil {
		if contentCreateAttachmentRecordDigest(existing) != recordDigest {
			return applicationcontent.NewError(applicationcontent.ErrorConflict, "note_asset_copy_attachment_row", nil)
		}
		return verifyNoteAssetContent(ctx, command.DestinationOwnerID, applicationcontent.AssetAttachment, asset.AssetID, existing.Path, existing.Size, asset.ContentSHA256)
	}
	if !errors.Is(existingErr, mongo.ErrNoDocuments) {
		return contentAssetDependency("note_asset_copy_destination", existingErr)
	}
	destinationPath, err := applicationcontent.ParseStoredPath(destination.Path)
	if err != nil {
		return err
	}
	_, err = contentCreateRepair.Execute(ctx, applicationcontent.CreateRepairCommand{
		Identity: applicationcontent.CreateIdentity{
			Action: "note_attachment_copy", OwnerID: command.DestinationOwnerID, RecordOwnerID: command.DestinationOwnerID,
			ParentID: command.DestinationNoteID, Kind: applicationcontent.AssetAttachment, AssetID: destinationID.Hex(),
			Generation: int64(command.Generation), ContentDigest: digest, RecordDigest: recordDigest, ContentSize: int64(len(data)),
		},
		Destination: destinationPath, Source: bytes.NewReader(data),
		ApplyMetadata: func(ctx context.Context) error {
			return attachService.insertAttachIfMissing(ctx, destination, command.DestinationOwnerID)
		},
	})
	return err
}

func readAttachmentSource(ctx context.Context, source info.Attach) ([]byte, error) {
	logical, err := applicationcontent.ParseStoredPath(source.Path)
	if err != nil {
		return nil, err
	}
	opened, err := contentStore.Open(ctx, logical)
	if err != nil {
		return nil, err
	}
	if opened.Reader == nil || opened.Size != source.Size {
		if opened.Reader != nil {
			_ = opened.Reader.Close()
		}
		return nil, applicationcontent.NewError(applicationcontent.ErrorConflict, "note_asset_copy_size", nil)
	}
	data, readErr := io.ReadAll(io.LimitReader(opened.Reader, source.Size+1))
	closeErr := opened.Reader.Close()
	if readErr != nil || closeErr != nil || int64(len(data)) != source.Size {
		return nil, applicationcontent.NewError(applicationcontent.ErrorStorageUnavailable, "note_asset_copy_read", errors.Join(readErr, closeErr))
	}
	return data, nil
}

func copiedAttachmentDestination(source info.Attach, ownerID, noteID domain.ObjectID, operationID string) (info.Attach, error) {
	title, err := applicationcontent.CleanVisibleText(source.Title, false)
	if err != nil {
		return info.Attach{}, err
	}
	destination := source
	destination.AttachId = stableCopiedAttachID(operationID, source.AttachId.Hex(), ownerID.Hex())
	destination.NoteId = noteID
	destination.UploadUserId = ownerID
	destination.Name = destination.AttachId.Hex() + filepath.Ext(source.Name)
	destination.Title = title
	destination.Path = "files/" + ownerID.Hex() + "/" + destination.AttachId.Hex() + "/attachs/" + destination.Name
	return destination, nil
}

func verifyFrozenCopyRows(ctx context.Context, command applicationnotes.CopyNoteAssetsCommand, assets []applicationnotes.OperationAsset) (bool, error) {
	for _, asset := range assets {
		if asset.IsAttach {
			expectedID := stableCopiedAttachID(command.AssetOperationID, asset.LocalFileID, command.DestinationOwnerID.Hex())
			if asset.AssetID != expectedID.Hex() {
				return false, applicationcontent.NewError(applicationcontent.ErrorConflict, "note_asset_copy_verify_attachment_identity", nil)
			}
			var source info.Attach
			if err := db.Attachs.FindContext(ctx, bson.M{"_id": db.MustObjectIDFromHex(asset.LocalFileID), "NoteId": command.SourceNoteID}).One(&source); err != nil {
				return false, contentAssetDependency("note_asset_copy_verify_source_attachment", err)
			}
			expected, err := copiedAttachmentDestination(source, command.DestinationOwnerID, command.DestinationNoteID, command.AssetOperationID)
			if err != nil {
				return false, err
			}
			var actual info.Attach
			if err := db.Attachs.FindContext(ctx, bson.M{"_id": expected.AttachId, "NoteId": command.DestinationNoteID}).One(&actual); err != nil {
				return false, contentAssetDependency("note_asset_copy_verify_attachment", err)
			}
			expectedDigest := contentCreateAttachmentRecordDigest(expected)
			if hex.EncodeToString(expectedDigest[:]) != asset.RecordSHA256 || contentCreateAttachmentRecordDigest(actual) != expectedDigest {
				return false, nil
			}
			continue
		}
		expectedID := stableCopiedImageID(command.AssetOperationID+":image:"+asset.LocalFileID, asset.LocalFileID, command.DestinationOwnerID.Hex())
		if asset.AssetID != expectedID.Hex() {
			return false, applicationcontent.NewError(applicationcontent.ErrorConflict, "note_asset_copy_verify_image_identity", nil)
		}
		var source info.File
		if err := db.Files.FindContext(ctx, bson.M{"_id": db.MustObjectIDFromHex(asset.LocalFileID), "UserId": command.SourceOwnerID, "Type": ""}).One(&source); err != nil {
			return false, contentAssetDependency("note_asset_copy_verify_source_image", err)
		}
		expected := copiedImageDestination(source, command.DestinationOwnerID, command.AssetOperationID+":image:"+asset.LocalFileID)
		var actual info.File
		if err := db.Files.FindContext(ctx, bson.M{"_id": expected.FileId, "UserId": command.DestinationOwnerID, "Type": ""}).One(&actual); err != nil {
			return false, contentAssetDependency("note_asset_copy_verify_image", err)
		}
		expectedDigest := contentCreateImageRecordDigest(expected)
		if hex.EncodeToString(expectedDigest[:]) != asset.RecordSHA256 || contentCreateImageRecordDigest(actual) != expectedDigest {
			return false, nil
		}
	}
	return true, nil
}

func setNoteAttachNum(ctx context.Context, noteID, ownerID domain.ObjectID, generation, count int) error {
	if err := db.Notes.UpdateOneMatchedContext(ctx, bson.M{
		"_id": noteID, "UserId": ownerID, "Usn": generation, "IsDeleted": false,
	}, bson.M{"$set": bson.M{"AttachNum": count}}); err != nil {
		return contentAssetDependency("note_asset_copy_count", err)
	}
	return nil
}

func noteAssetDeleteService() applicationcontent.DeleteAssetService {
	return applicationcontent.DeleteAssetService{
		Repository: mongoNoteAssetDeleteRepository{}, Content: contentStore, Manifests: contentDeleteManifests, Storage: contentLifecycle,
	}
}

type mongoNoteAssetDeleteRepository struct{}

func (mongoNoteAssetDeleteRepository) LoadDeleteAsset(ctx context.Context, kind applicationcontent.AssetKind, assetID domain.ObjectID) (applicationcontent.DeleteAssetMetadata, error) {
	if kind != applicationcontent.AssetAttachment {
		return applicationcontent.DeleteAssetMetadata{}, applicationcontent.NewError(applicationcontent.ErrorValidation, "note_asset_delete_kind", nil)
	}
	var attachment info.Attach
	if err := db.Attachs.FindContext(ctx, bson.M{"_id": assetID}).One(&attachment); err != nil {
		return applicationcontent.DeleteAssetMetadata{}, err
	}
	var note info.Note
	if err := db.Notes.FindContext(ctx, bson.M{"_id": attachment.NoteId}).One(&note); err != nil {
		return applicationcontent.DeleteAssetMetadata{}, err
	}
	return applicationcontent.DeleteAssetMetadata{
		AssetID: attachment.AttachId, OwnerID: note.UserId, Kind: kind, StoredPath: attachment.Path,
		Size: attachment.Size, Generation: int64(note.Usn),
	}, nil
}

func (mongoNoteAssetDeleteRepository) DeleteAsset(ctx context.Context, ownerID domain.ObjectID, kind applicationcontent.AssetKind, assetID domain.ObjectID) (bool, error) {
	if kind != applicationcontent.AssetAttachment {
		return false, applicationcontent.NewError(applicationcontent.ErrorValidation, "note_asset_delete_kind", nil)
	}
	var attachment info.Attach
	if err := db.Attachs.FindContext(ctx, bson.M{"_id": assetID}).One(&attachment); err != nil {
		return false, err
	}
	var note info.Note
	if err := db.Notes.FindContext(ctx, bson.M{"_id": attachment.NoteId, "UserId": ownerID}).One(&note); err != nil {
		return false, err
	}
	err := db.Attachs.RemoveContext(ctx, bson.M{"_id": assetID, "NoteId": attachment.NoteId})
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	return err == nil, err
}

func (mongoNoteAssetDeleteRepository) DeleteAssetAbsent(ctx context.Context, ownerID domain.ObjectID, kind applicationcontent.AssetKind, assetID domain.ObjectID) (bool, error) {
	if kind != applicationcontent.AssetAttachment {
		return false, applicationcontent.NewError(applicationcontent.ErrorValidation, "note_asset_delete_kind", nil)
	}
	var attachment info.Attach
	err := db.Attachs.FindContext(ctx, bson.M{"_id": assetID}).One(&attachment)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	var note info.Note
	err = db.Notes.FindContext(ctx, bson.M{"_id": attachment.NoteId, "UserId": ownerID}).One(&note)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, applicationcontent.NewError(applicationcontent.ErrorConflict, "note_asset_delete_owner", nil)
	}
	return false, err
}

func contentAssetDependency(code string, err error) error {
	if err == nil {
		return nil
	}
	var contentErr *applicationcontent.Error
	if errors.As(err, &contentErr) {
		return err
	}
	return applicationcontent.NewError(applicationcontent.ErrorDependency, code, err)
}

var _ applicationnotes.ContentAssetPort = contentAssetProvider{}
var _ applicationcontent.DeleteAssetRepository = mongoNoteAssetDeleteRepository{}
