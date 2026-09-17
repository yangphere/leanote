package content

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"image"
	"image/png"
	"io"
	"testing"
	"time"

	"github.com/yangphere/leanote/app/domain"
)

func imagePNG(t *testing.T) []byte {
	t.Helper()
	var output bytes.Buffer
	if err := png.Encode(&output, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestPrepareImageUploadUsesBoundedDecodedMedia(t *testing.T) {
	data := imagePNG(t)
	prepared, err := PrepareImageUpload(PrepareImageUploadRequest{
		Reader: bytes.NewReader(data), Limit: int64(len(data)), OriginalName: `..\photo.PNG`, Budget: HardImageBudget(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.DisplayName != "photo.PNG" || prepared.Extension != ".png" || prepared.MediaType != "image/png" || !bytes.Equal(prepared.Data, data) {
		t.Fatalf("PrepareImageUpload() = %#v", prepared)
	}
}

func TestPrepareImageUploadRejectsOversizeAndExtensionMismatch(t *testing.T) {
	data := imagePNG(t)
	if _, err := PrepareImageUpload(PrepareImageUploadRequest{Reader: bytes.NewReader(data), Limit: int64(len(data) - 1), OriginalName: "photo.png", Budget: HardImageBudget()}); errorCategoryOf(err) != ErrorTooLarge {
		t.Fatalf("oversize error = %v", err)
	}
	if _, err := PrepareImageUpload(PrepareImageUploadRequest{Reader: bytes.NewReader(data), Limit: int64(len(data)), OriginalName: "photo.jpg", Budget: HardImageBudget()}); errorCategoryOf(err) != ErrorUnsupportedMedia {
		t.Fatalf("extension error = %v", err)
	}
}

type fakeImageRepository struct {
	image      StoredImage
	notes      []ImageNote
	deleteErr  error
	deleted    bool
	projection bool
}

func (repository *fakeImageRepository) LoadImage(context.Context, domain.ObjectID) (StoredImage, error) {
	return repository.image, nil
}

func (repository *fakeImageRepository) ReferencingNotes(context.Context, domain.ObjectID) ([]ImageNote, error) {
	return repository.notes, nil
}

func (repository *fakeImageRepository) DeleteImageAndProjection(context.Context, domain.ObjectID, domain.ObjectID) (bool, error) {
	if repository.deleteErr != nil {
		return false, repository.deleteErr
	}
	repository.deleted = true
	repository.projection = true
	return true, nil
}

func (repository *fakeImageRepository) ImageAbsent(context.Context, domain.ObjectID, domain.ObjectID) (bool, error) {
	return repository.deleted, nil
}

type fakeImagePermission struct {
	allowed bool
	err     error
}

func (permission fakeImagePermission) CanReadNote(context.Context, ImageNote, domain.ObjectID) (bool, error) {
	return permission.allowed, permission.err
}

func TestImageAccessRequiresOwnerOrReferencedReadableNote(t *testing.T) {
	actor, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	imageID, _ := domain.ParseObjectID("507f1f77bcf86cd799439013")
	noteID, _ := domain.ParseObjectID("507f1f77bcf86cd799439014")
	store := fakeAttachmentStore{data: []byte("png")}
	repository := &fakeImageRepository{image: StoredImage{ImageID: imageID, OwnerID: owner, StoredPath: "files/owner/a.png", Size: 3}}
	service := ImageAccessService{Repository: repository, Permissions: fakeImagePermission{}, Store: store}
	if _, err := service.Open(context.Background(), actor, imageID); errorCategoryOf(err) != ErrorUnauthorized {
		t.Fatalf("unreferenced Open() error = %v", err)
	}
	repository.notes = []ImageNote{{NoteID: noteID, OwnerID: owner}}
	service.Permissions = fakeImagePermission{allowed: true}
	download, err := service.Open(context.Background(), actor, imageID)
	if err != nil {
		t.Fatal(err)
	}
	defer download.Reader.Close()
}

func TestImageAccessAllowsAnonymousPublishedReference(t *testing.T) {
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	imageID, _ := domain.ParseObjectID("507f1f77bcf86cd799439013")
	noteID, _ := domain.ParseObjectID("507f1f77bcf86cd799439014")
	repository := &fakeImageRepository{
		image: StoredImage{ImageID: imageID, OwnerID: owner, StoredPath: "files/owner/a.png", Size: 3},
		notes: []ImageNote{{NoteID: noteID, OwnerID: owner, Public: true}},
	}
	download, err := (ImageAccessService{Repository: repository, Store: fakeAttachmentStore{data: []byte("png")}}).Open(context.Background(), domain.ObjectID{}, imageID)
	if err != nil {
		t.Fatal(err)
	}
	defer download.Reader.Close()
}

func TestImageAccessPublicReferenceDoesNotDependOnPrivateReferenceOrder(t *testing.T) {
	actor, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	imageID, _ := domain.ParseObjectID("507f1f77bcf86cd799439013")
	privateNoteID, _ := domain.ParseObjectID("507f1f77bcf86cd799439014")
	publicNoteID, _ := domain.ParseObjectID("507f1f77bcf86cd799439015")
	repository := &fakeImageRepository{
		image: StoredImage{ImageID: imageID, OwnerID: owner, StoredPath: "files/owner/a.png", Size: 3},
		notes: []ImageNote{
			{NoteID: privateNoteID, OwnerID: owner},
			{NoteID: publicNoteID, OwnerID: owner, Public: true},
		},
	}
	service := ImageAccessService{Repository: repository, Permissions: fakeImagePermission{err: errors.New("permission unavailable")}, Store: fakeAttachmentStore{data: []byte("png")}}
	download, err := service.Open(context.Background(), actor, imageID)
	if err != nil {
		t.Fatal(err)
	}
	defer download.Reader.Close()
}

func TestImageAccessClassifiesMissingDependenciesAndReader(t *testing.T) {
	imageID, _ := domain.ParseObjectID("507f1f77bcf86cd799439013")
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	if _, err := (ImageAccessService{}).Open(context.Background(), owner, imageID); errorCategoryOf(err) != ErrorDependency {
		t.Fatalf("missing repository error = %v", err)
	}
	repository := &fakeImageRepository{image: StoredImage{ImageID: imageID, OwnerID: owner, StoredPath: "files/owner/a.png", Size: 3}}
	if _, err := (ImageAccessService{Repository: repository}).Open(context.Background(), owner, imageID); errorCategoryOf(err) != ErrorDependency {
		t.Fatalf("missing store error = %v", err)
	}
	if _, err := (ImageAccessService{Repository: repository, Store: fakeAttachmentStore{nilReader: true}}).Open(context.Background(), owner, imageID); errorCategoryOf(err) != ErrorStorageUnavailable {
		t.Fatalf("missing reader error = %v", err)
	}
}

func TestNormalizeAlbumMutationRejectsBlankDefaultAndCleansName(t *testing.T) {
	if _, err := NormalizeAlbumMutation("", "name"); errorCategoryOf(err) != ErrorValidation {
		t.Fatalf("blank album error = %v", err)
	}
	albumID := "507f1f77bcf86cd799439013"
	mutation, err := NormalizeAlbumMutation(albumID, "  photos\x00 ")
	if err != nil || mutation.AlbumID.Hex() != albumID || mutation.Name != "photos" {
		t.Fatalf("NormalizeAlbumMutation() = %#v, %v", mutation, err)
	}
}

type fakeDeleteManifestStore struct{ manifest DeleteManifest }

func (store *fakeDeleteManifestStore) LoadDeleteManifest(context.Context, string) (DeleteManifest, bool, error) {
	return store.manifest, store.manifest.LookupKey != "", nil
}

func (store *fakeDeleteManifestStore) Create(_ context.Context, manifest DeleteManifest) error {
	store.manifest = manifest
	return nil
}

func (store *fakeDeleteManifestStore) CompareAndSwap(_ context.Context, _ string, _ uint64, _ [sha256.Size]byte, next DeleteManifest) error {
	store.manifest = next
	return nil
}

type fakeDeleteStorage struct {
	quarantined bool
	quarantines int
	restored    bool
	purged      bool
}

func (storage *fakeDeleteStorage) Quarantine(context.Context, LogicalPath, string, [sha256.Size]byte, int64) (LogicalPath, error) {
	storage.quarantined = true
	storage.quarantines++
	return LogicalPath{Kind: RootPrivateFiles, Value: "quarantine/image"}, nil
}

func (storage *fakeDeleteStorage) Restore(context.Context, LogicalPath, LogicalPath, [sha256.Size]byte, int64) error {
	storage.restored = true
	return nil
}

func (storage *fakeDeleteStorage) Purge(context.Context, LogicalPath, [sha256.Size]byte, int64) error {
	storage.purged = true
	return nil
}

func TestImageDeleteRunsDurableLifecycleToTerminal(t *testing.T) {
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	imageID, _ := domain.ParseObjectID("507f1f77bcf86cd799439013")
	data := []byte("image")
	repository := &fakeImageRepository{image: StoredImage{ImageID: imageID, OwnerID: owner, StoredPath: "files/owner/a.png", Size: int64(len(data)), Generation: 7}}
	manifests := &fakeDeleteManifestStore{}
	storage := &fakeDeleteStorage{}
	service := ImageDeleteService{Repository: repository, Content: fakeAttachmentStore{data: data}, Manifests: manifests, Storage: storage, Now: func() time.Time { return time.Unix(1_700_000_000, 0).UTC() }}
	result, err := service.Delete(context.Background(), ImageDeleteCommand{OwnerID: owner, ImageID: imageID})
	if err != nil || !result.Deleted || manifests.manifest.Stage != DeleteStageTerminal || !repository.projection || !storage.purged {
		t.Fatalf("Delete() = %#v, manifest=%+v, err=%v", result, manifests.manifest, err)
	}
}

func TestImageDeleteRestoresSourceWhenMetadataMutationFails(t *testing.T) {
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	imageID, _ := domain.ParseObjectID("507f1f77bcf86cd799439013")
	data := []byte("image")
	repository := &fakeImageRepository{image: StoredImage{ImageID: imageID, OwnerID: owner, StoredPath: "files/owner/a.png", Size: int64(len(data)), Generation: 7}, deleteErr: errors.New("mongo down")}
	storage := &fakeDeleteStorage{}
	manifests := &fakeDeleteManifestStore{}
	service := ImageDeleteService{Repository: repository, Content: fakeAttachmentStore{data: data}, Manifests: manifests, Storage: storage, Now: time.Now}
	if _, err := service.Delete(context.Background(), ImageDeleteCommand{OwnerID: owner, ImageID: imageID}); err == nil || !storage.restored {
		t.Fatalf("Delete() error=%v restored=%v", err, storage.restored)
	}
	repository.deleteErr = nil
	result, err := service.Delete(context.Background(), ImageDeleteCommand{OwnerID: owner, ImageID: imageID})
	if err != nil || !result.Deleted || storage.quarantines != 2 || manifests.manifest.Stage != DeleteStageTerminal {
		t.Fatalf("retry Delete() = %#v, err=%v quarantines=%d manifest=%+v", result, err, storage.quarantines, manifests.manifest)
	}
}

var _ io.Reader = (*bytes.Reader)(nil)
