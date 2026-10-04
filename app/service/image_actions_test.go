package service

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	applicationcontent "github.com/yangphere/leanote/app/application/content"
)

func TestImageActionsRejectMalformedIdentitiesBeforeMongo(t *testing.T) {
	if _, err := fileService.OpenReadableImage(context.Background(), "actor", "image"); err == nil {
		t.Fatal("OpenReadableImage accepted malformed identities")
	}
	if _, err := fileService.ListImagesReadable(context.Background(), "actor", "", "", 1, 12); err == nil {
		t.Fatal("ListImagesReadable accepted malformed actor")
	}
	if ok, _ := fileService.DeleteImageWithOperation(context.Background(), "actor", "image", ""); ok {
		t.Fatal("DeleteImageWithOperation accepted malformed identities")
	}
	if ok, _ := fileService.CopyImage("actor", "image", "owner"); ok {
		t.Fatal("CopyImage accepted malformed identities")
	}
}

func TestUploadImageActionRejectsMalformedActorBeforeReading(t *testing.T) {
	reader := &countingReader{Reader: strings.NewReader("not an image")}
	_, err := fileService.UploadImage(context.Background(), ImageUploadInput{
		ActorID: "actor", Reader: reader, Limit: 1024, OriginalName: "a.png", Budget: applicationcontent.HardImageBudget(),
	})
	if err == nil || reader.Reads != 0 {
		t.Fatalf("UploadImage() err=%v reads=%d", err, reader.Reads)
	}
}

func TestAlbumActionsRejectBlankDefaultMutationBeforeMongo(t *testing.T) {
	if ok := albumService.UpdateAlbum("", "507f1f77bcf86cd799439011", "name"); ok {
		t.Fatal("UpdateAlbum mutated blank default album")
	}
	if ok, _ := albumService.DeleteAlbum("507f1f77bcf86cd799439011", ""); ok {
		t.Fatal("DeleteAlbum deleted blank default album")
	}
}

func TestMaterializeImageDownloadStopsOnCancellation(t *testing.T) {
	savedStore := contentStore
	contentStore = archiveStore{}
	defer func() { contentStore = savedStore }()
	ctx, cancel := context.WithCancel(context.Background())
	reader := &cancelingImageReader{reader: strings.NewReader("image"), cancel: cancel}
	download, err := materializeImageDownload(ctx, applicationcontent.ImageDownload{Reader: reader, Size: 5, Name: "image.png"})
	var contentErr *applicationcontent.Error
	if download.Reader != nil || !errors.As(err, &contentErr) || contentErr.Category != applicationcontent.ErrorTimeout || !reader.closed {
		t.Fatalf("materializeImageDownload() = %#v, err=%v, closed=%v", download, err, reader.closed)
	}
}

type countingReader struct {
	*strings.Reader
	Reads int
}

func (reader *countingReader) Read(buffer []byte) (int, error) {
	reader.Reads++
	return reader.Reader.Read(buffer)
}

type cancelingImageReader struct {
	reader io.Reader
	cancel context.CancelFunc
	closed bool
}

func (reader *cancelingImageReader) Read(buffer []byte) (int, error) {
	reader.cancel()
	return reader.reader.Read(buffer)
}

func (reader *cancelingImageReader) Close() error {
	reader.closed = true
	return nil
}
