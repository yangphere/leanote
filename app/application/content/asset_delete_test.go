package content

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yangphere/leanote/app/domain"
)

type fakeDeleteAssetRepository struct {
	asset     DeleteAssetMetadata
	absent    bool
	deleteErr error
	deletes   int
}

func (repository *fakeDeleteAssetRepository) LoadDeleteAsset(context.Context, AssetKind, domain.ObjectID) (DeleteAssetMetadata, error) {
	return repository.asset, nil
}

func (repository *fakeDeleteAssetRepository) DeleteAsset(context.Context, domain.ObjectID, AssetKind, domain.ObjectID) (bool, error) {
	repository.deletes++
	if repository.deleteErr != nil {
		return false, repository.deleteErr
	}
	repository.absent = true
	return true, nil
}

func (repository *fakeDeleteAssetRepository) DeleteAssetAbsent(context.Context, domain.ObjectID, AssetKind, domain.ObjectID) (bool, error) {
	return repository.absent, nil
}

func TestDeleteAssetServiceResumesSameManifestAfterMetadataFailure(t *testing.T) {
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	assetID, _ := domain.ParseObjectID("507f1f77bcf86cd799439013")
	data := []byte("attachment")
	repository := &fakeDeleteAssetRepository{asset: DeleteAssetMetadata{
		AssetID: assetID, OwnerID: owner, Kind: AssetAttachment,
		StoredPath: "files/owner/attachment", Size: int64(len(data)), Generation: 7,
	}, deleteErr: errors.New("mongo unavailable")}
	manifests := &fakeDeleteManifestStore{}
	storage := &fakeDeleteStorage{}
	service := DeleteAssetService{
		Repository: repository, Content: fakeAttachmentStore{data: data}, Manifests: manifests, Storage: storage,
		Now: func() time.Time { return time.Unix(1_800_000_000, 0).UTC() },
	}
	command := DeleteAssetCommand{
		Action: "delete_note_attachment", OwnerID: owner, Kind: AssetAttachment,
		AssetID: assetID, OperationID: "note-cleanup:attachment",
	}
	if err := service.Delete(context.Background(), command); err == nil || !storage.restored {
		t.Fatalf("first Delete() error=%v restored=%v", err, storage.restored)
	}
	if manifests.manifest.Stage != DeleteStageQuarantined {
		t.Fatalf("first manifest stage=%s", manifests.manifest.Stage)
	}
	repository.deleteErr = nil
	if err := service.Delete(context.Background(), command); err != nil {
		t.Fatalf("retry Delete() error=%v", err)
	}
	if repository.deletes != 2 || storage.quarantines != 2 || !storage.purged || manifests.manifest.Stage != DeleteStageTerminal {
		t.Fatalf("retry state deletes=%d quarantines=%d purged=%v manifest=%s", repository.deletes, storage.quarantines, storage.purged, manifests.manifest.Stage)
	}
	if err := service.Delete(context.Background(), command); err != nil || repository.deletes != 2 {
		t.Fatalf("terminal replay error=%v deletes=%d", err, repository.deletes)
	}
}

func TestDeleteAssetServiceRejectsDifferentOperationForExistingIdentity(t *testing.T) {
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	assetID, _ := domain.ParseObjectID("507f1f77bcf86cd799439013")
	data := []byte("attachment")
	repository := &fakeDeleteAssetRepository{asset: DeleteAssetMetadata{
		AssetID: assetID, OwnerID: owner, Kind: AssetAttachment,
		StoredPath: "files/owner/attachment", Size: int64(len(data)), Generation: 7,
	}}
	service := DeleteAssetService{
		Repository: repository, Content: fakeAttachmentStore{data: data}, Manifests: &fakeDeleteManifestStore{}, Storage: &fakeDeleteStorage{},
	}
	first := DeleteAssetCommand{Action: "delete_note_attachment", OwnerID: owner, Kind: AssetAttachment, AssetID: assetID, OperationID: "first"}
	if err := service.Delete(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	first.OperationID = "second"
	if err := service.Delete(context.Background(), first); errorCategoryOf(err) != ErrorConflict {
		t.Fatalf("different operation error=%v", err)
	}
}

var _ DeleteAssetRepository = (*fakeDeleteAssetRepository)(nil)
