package service

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	applicationcontent "github.com/yangphere/leanote/app/application/content"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/info"
)

type archiveStore map[applicationcontent.LogicalPath][]byte

func (archiveStore) CreateTemporary(_ context.Context, prefix string) (applicationcontent.TemporaryArtifact, error) {
	file, err := os.CreateTemp("", prefix)
	if err != nil {
		return nil, err
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return nil, err
	}
	return &testTemporaryArtifact{File: file, name: file.Name()}, nil
}

type testTemporaryArtifact struct {
	*os.File
	name string
}

type testTemporaryReader struct {
	*os.File
	name string
}

type attachmentReadCloser struct {
	reader   io.Reader
	readErr  error
	closeErr error
}

func (reader *attachmentReadCloser) Read(buffer []byte) (int, error) {
	if reader.readErr != nil {
		return 0, reader.readErr
	}
	return reader.reader.Read(buffer)
}

func (reader *attachmentReadCloser) Close() error { return reader.closeErr }

func (artifact *testTemporaryArtifact) Seal(ctx context.Context) (io.ReadCloser, int64, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, errors.Join(err, artifact.Abort())
	}
	if err := artifact.File.Sync(); err != nil {
		return nil, 0, errors.Join(err, artifact.Abort())
	}
	if err := artifact.File.Close(); err != nil {
		return nil, 0, errors.Join(err, os.Remove(artifact.name))
	}
	file, err := os.Open(artifact.name)
	if err != nil {
		return nil, 0, errors.Join(err, os.Remove(artifact.name))
	}
	info, err := file.Stat()
	if err != nil {
		return nil, 0, errors.Join(err, file.Close(), os.Remove(artifact.name))
	}
	return &testTemporaryReader{File: file, name: artifact.name}, info.Size(), nil
}

func (artifact *testTemporaryArtifact) Abort() error {
	return errors.Join(artifact.File.Close(), os.Remove(artifact.name))
}

func (reader *testTemporaryReader) Close() error {
	return errors.Join(reader.File.Close(), os.Remove(reader.name))
}

func (store archiveStore) Open(_ context.Context, path applicationcontent.LogicalPath) (applicationcontent.OpenResult, error) {
	data, ok := store[path]
	if !ok {
		return applicationcontent.OpenResult{}, applicationcontent.NewError(applicationcontent.ErrorNotFound, "missing", nil)
	}
	return applicationcontent.OpenResult{Reader: io.NopCloser(bytes.NewReader(data)), Size: int64(len(data))}, nil
}

func TestCleanAttachmentTitlesForCopyRejectsWholeSetBeforeMutation(t *testing.T) {
	original := []info.Attach{{Title: "first.txt"}, {Title: strings.Repeat("a", applicationcontent.MaxVisibleTextBytes+1)}}
	cleaned, err := cleanAttachmentTitlesForCopy(original)
	if err == nil || cleaned != nil {
		t.Fatalf("cleanAttachmentTitlesForCopy() = %#v, %v", cleaned, err)
	}
	if original[0].Title != "first.txt" {
		t.Fatalf("input mutated: %#v", original)
	}
}

func (store archiveStore) Publish(_ context.Context, request applicationcontent.PublishRequest) (applicationcontent.PublishResult, error) {
	data, err := io.ReadAll(request.Source)
	if err != nil {
		return applicationcontent.PublishResult{}, err
	}
	digest := sha256.Sum256(data)
	if digest != request.Identity.Digest {
		return applicationcontent.PublishResult{}, applicationcontent.NewError(applicationcontent.ErrorConflict, "source_digest_mismatch", nil)
	}
	if existing, ok := store[request.Destination]; ok {
		if sha256.Sum256(existing) != digest {
			return applicationcontent.PublishResult{}, applicationcontent.NewError(applicationcontent.ErrorConflict, "destination_digest_mismatch", nil)
		}
		return applicationcontent.PublishResult{Status: applicationcontent.PublishAlreadyApplied, Destination: request.Destination, Digest: digest, Size: int64(len(data))}, nil
	}
	store[request.Destination] = append([]byte(nil), data...)
	return applicationcontent.PublishResult{Status: applicationcontent.PublishApplied, Destination: request.Destination, Digest: digest, Size: int64(len(data))}, nil
}

func (store archiveStore) Verify(_ context.Context, request applicationcontent.VerifyRequest) (applicationcontent.VerifyResult, error) {
	data, ok := store[request.Destination]
	if !ok {
		return applicationcontent.VerifyResult{Status: applicationcontent.VerificationNotApplied}, nil
	}
	digest := sha256.Sum256(data)
	if digest != request.ExpectedDigest {
		return applicationcontent.VerifyResult{Status: applicationcontent.VerificationConflict, Digest: digest, Size: int64(len(data))}, nil
	}
	return applicationcontent.VerifyResult{Status: applicationcontent.VerificationApplied, Digest: digest, Size: int64(len(data))}, nil
}

func (archiveStore) Close() error { return nil }

func TestOpenAttachmentArchiveUsesLogicalStoreAndSafeNames(t *testing.T) {
	store := archiveStore{
		{Kind: applicationcontent.RootPrivateFiles, Value: "owner/one.txt"}: []byte("one"),
		{Kind: applicationcontent.RootPrivateFiles, Value: "owner/two.txt"}: []byte("two"),
	}
	reader, err := openAttachmentArchive(context.Background(), store, []info.Attach{
		{Path: "files/owner/two.txt", Title: "../report.txt"},
		{Path: "files/owner/one.txt", Title: "report.txt"},
	})
	if err != nil {
		t.Fatalf("openAttachmentArchive() error = %v", err)
	}
	defer reader.Close()
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		t.Fatalf("gzip.NewReader() error = %v", err)
	}
	tarReader := tar.NewReader(gzipReader)
	for index, want := range []struct{ name, body string }{{"report.txt", "one"}, {"report (2).txt", "two"}} {
		header, err := tarReader.Next()
		if err != nil {
			t.Fatalf("entry %d: %v", index, err)
		}
		body, err := io.ReadAll(tarReader)
		if err != nil || header.Name != want.name || string(body) != want.body {
			t.Fatalf("entry %d = %q %q, %v", index, header.Name, body, err)
		}
	}
}

func TestOpenAttachmentArchiveRejectsUnknownStoredPathBeforeStreaming(t *testing.T) {
	reader, err := openAttachmentArchive(context.Background(), archiveStore{}, []info.Attach{{Path: "other/file.txt", Title: "file.txt"}})
	if err == nil || reader != nil {
		t.Fatalf("openAttachmentArchive() = %v, %v", reader, err)
	}
}

func TestOpenAttachmentArchiveReturnsSourceFailureBeforeResponseReader(t *testing.T) {
	reader, err := openAttachmentArchive(context.Background(), archiveStore{}, []info.Attach{{
		Path: "files/owner/missing.txt", Title: "missing.txt",
	}})
	if err == nil || reader != nil {
		t.Fatalf("openAttachmentArchive() = %v, %v", reader, err)
	}
}

func TestMaterializeAttachmentDownloadReadsAndClosesBeforeReturning(t *testing.T) {
	download, err := materializeAttachmentDownload(context.Background(), archiveStore{}, applicationcontent.AttachmentDownload{
		Reader: &attachmentReadCloser{reader: strings.NewReader("one")}, Size: 3, DisplayName: "report.txt",
	})
	if err != nil {
		t.Fatalf("materializeAttachmentDownload() error = %v", err)
	}
	defer download.Reader.Close()
	data, err := io.ReadAll(download.Reader)
	if err != nil || string(data) != "one" || download.Size != 3 || download.DisplayName != "report.txt" {
		t.Fatalf("download=%#v data=%q err=%v", download, data, err)
	}
}

func TestMaterializeAttachmentDownloadPropagatesReadAndCloseFailures(t *testing.T) {
	for _, test := range []struct {
		name   string
		reader *attachmentReadCloser
	}{
		{name: "read", reader: &attachmentReadCloser{reader: strings.NewReader("one"), readErr: errors.New("read failed")}},
		{name: "close", reader: &attachmentReadCloser{reader: strings.NewReader("one"), closeErr: errors.New("close failed")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			download, err := materializeAttachmentDownload(context.Background(), archiveStore{}, applicationcontent.AttachmentDownload{
				Reader: test.reader, Size: 3, DisplayName: "report.txt",
			})
			if err == nil || download.Reader != nil {
				t.Fatalf("materializeAttachmentDownload() = %#v, %v", download, err)
			}
		})
	}
}

func TestMaterializeAttachmentDownloadRejectsSizeMismatch(t *testing.T) {
	download, err := materializeAttachmentDownload(context.Background(), archiveStore{}, applicationcontent.AttachmentDownload{
		Reader: &attachmentReadCloser{reader: strings.NewReader("one")}, Size: 4, DisplayName: "report.txt",
	})
	if err == nil || download.Reader != nil {
		t.Fatalf("materializeAttachmentDownload() = %#v, %v", download, err)
	}
}

func TestMaterializeAttachmentDownloadHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	download, err := materializeAttachmentDownload(ctx, archiveStore{}, applicationcontent.AttachmentDownload{
		Reader: &attachmentReadCloser{reader: strings.NewReader("one")}, Size: 3, DisplayName: "report.txt",
	})
	var contentErr *applicationcontent.Error
	if download.Reader != nil || !errors.As(err, &contentErr) || contentErr.Category != applicationcontent.ErrorTimeout {
		t.Fatalf("materializeAttachmentDownload() = %#v, %v", download, err)
	}
}

func TestMaterializeAttachmentDownloadPreservesInvalidInputCloseFailure(t *testing.T) {
	closeFailure := errors.New("close failed")
	download, err := materializeAttachmentDownload(context.Background(), nil, applicationcontent.AttachmentDownload{
		Reader: &attachmentReadCloser{reader: strings.NewReader("one"), closeErr: closeFailure}, Size: 3,
	})
	if download.Reader != nil || !errors.Is(err, closeFailure) {
		t.Fatalf("materializeAttachmentDownload() = %#v, %v", download, err)
	}
}

func TestOptionalAttachmentActorRejectsMalformedIdentity(t *testing.T) {
	if _, err := optionalAttachmentActor("not-an-object-id"); err == nil {
		t.Fatal("optionalAttachmentActor() accepted malformed identity")
	}
	if actor, err := optionalAttachmentActor(""); err != nil || !actor.IsZero() {
		t.Fatalf("optionalAttachmentActor(empty) = %s, %v", actor.Hex(), err)
	}
}

func TestOpenReadableArchiveLetsAnonymousRequestsReachPublicNoteAuthorization(t *testing.T) {
	savedNotes := db.Notes
	db.Notes = nil
	t.Cleanup(func() { db.Notes = savedNotes })

	reader, title, err := (&AttachService{}).OpenReadableArchive(context.Background(), "", "507f1f77bcf86cd799439013")
	var contentErr *applicationcontent.Error
	if reader != nil || title != "" || !errors.As(err, &contentErr) || contentErr.Category != applicationcontent.ErrorDependency {
		t.Fatalf("OpenReadableArchive() = %v, %q, %v", reader, title, err)
	}
}
