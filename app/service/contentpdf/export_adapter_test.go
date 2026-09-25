package contentpdf

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"testing"

	application "github.com/yangphere/leanote/app/application/content"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	"github.com/yangphere/leanote/app/service/contentremote"
)

type fakePDFRepository struct {
	note       info.Note
	content    info.NoteContent
	file       info.File
	canRead    bool
	err        error
	fileOwner  domain.ObjectID
	contentFor domain.ObjectID
}

func (repository *fakePDFRepository) FindNote(context.Context, domain.ObjectID) (info.Note, error) {
	return repository.note, repository.err
}

func (repository *fakePDFRepository) FindNoteContent(_ context.Context, _, ownerID domain.ObjectID) (info.NoteContent, error) {
	repository.contentFor = ownerID
	return repository.content, repository.err
}

func (repository *fakePDFRepository) CanReadNote(context.Context, info.Note, domain.ObjectID) (bool, error) {
	return repository.canRead, repository.err
}

func (repository *fakePDFRepository) FindFile(_ context.Context, ownerID, _ domain.ObjectID) (info.File, error) {
	repository.fileOwner = ownerID
	return repository.file, repository.err
}

type fakeContentStore struct {
	path   application.LogicalPath
	data   []byte
	reader io.ReadCloser
	size   int64
	cancel context.CancelFunc
}

func (store *fakeContentStore) Open(_ context.Context, path application.LogicalPath) (application.OpenResult, error) {
	store.path = path
	if store.cancel != nil {
		store.cancel()
	}
	if store.reader != nil {
		return application.OpenResult{Reader: store.reader, Size: store.size}, nil
	}
	return application.OpenResult{Reader: io.NopCloser(bytes.NewReader(store.data)), Size: int64(len(store.data))}, nil
}

func TestResourcePortObservesCancellationAfterOpenAndClosesReader(t *testing.T) {
	owner := mustPDFObjectID(t, "507f1f77bcf86cd799439011")
	fileID := mustPDFObjectID(t, "507f1f77bcf86cd799439014")
	reader := &trackingPDFReader{}
	repository := &fakePDFRepository{file: info.File{FileId: fileID, UserId: owner, Path: "files/a/b/image.png"}}
	ctx, cancel := context.WithCancel(context.Background())
	store := &fakeContentStore{reader: reader, size: 1, cancel: cancel}
	_, err := (ResourcePort{Repository: repository, Store: store}).LoadAuthorized(ctx, owner, fileID, 1)
	if pdfErrorCategory(err) != application.ErrorTimeout || !errors.Is(err, context.Canceled) {
		t.Fatalf("LoadAuthorized() cancellation error=%v category=%q", err, pdfErrorCategory(err))
	}
	if reader.read || !reader.closed {
		t.Fatalf("canceled reader read=%v closed=%v", reader.read, reader.closed)
	}
}

type trackingPDFReader struct {
	read   bool
	closed bool
	data   []byte
	close  error
}

func (reader *trackingPDFReader) Read(buffer []byte) (int, error) {
	reader.read = true
	if len(reader.data) == 0 {
		return 0, io.EOF
	}
	written := copy(buffer, reader.data)
	reader.data = reader.data[written:]
	return written, nil
}

func (reader *trackingPDFReader) Close() error {
	reader.closed = true
	return reader.close
}

func TestResourcePortDoesNotReturnArtifactWhenReaderCloseFails(t *testing.T) {
	owner := mustPDFObjectID(t, "507f1f77bcf86cd799439011")
	fileID := mustPDFObjectID(t, "507f1f77bcf86cd799439014")
	data := onePixelPNG(t)
	reader := &trackingPDFReader{data: append([]byte(nil), data...), close: errors.New("close failed")}
	repository := &fakePDFRepository{file: info.File{FileId: fileID, UserId: owner, Path: "files/a/b/image.png"}}
	store := &fakeContentStore{reader: reader, size: int64(len(data))}
	resource, err := (ResourcePort{Repository: repository, Store: store}).LoadAuthorized(context.Background(), owner, fileID, int64(len(data)))
	if pdfErrorCategory(err) != application.ErrorStorageUnavailable || len(resource.Data) != 0 || !reader.closed {
		t.Fatalf("resource=%+v error=%v category=%q closed=%v", resource, err, pdfErrorCategory(err), reader.closed)
	}
}

func TestResourcePortRejectsProjectedBudgetBeforeReadingContent(t *testing.T) {
	owner := mustPDFObjectID(t, "507f1f77bcf86cd799439011")
	fileID := mustPDFObjectID(t, "507f1f77bcf86cd799439014")
	reader := &trackingPDFReader{}
	repository := &fakePDFRepository{file: info.File{FileId: fileID, UserId: owner, Path: "files/a/b/image.png"}}
	store := &fakeContentStore{reader: reader, size: 2}
	if _, err := (ResourcePort{Repository: repository, Store: store}).LoadAuthorized(context.Background(), owner, fileID, 1); pdfErrorCategory(err) != application.ErrorTooLarge {
		t.Fatalf("LoadAuthorized() error=%v", err)
	}
	if reader.read || !reader.closed {
		t.Fatalf("oversized reader read=%v closed=%v", reader.read, reader.closed)
	}
}

func (*fakeContentStore) Publish(context.Context, application.PublishRequest) (application.PublishResult, error) {
	panic("unexpected publish")
}

func (*fakeContentStore) Verify(context.Context, application.VerifyRequest) (application.VerifyResult, error) {
	panic("unexpected verify")
}

func TestNotePortLoadsAuthorizedOwnerBlogAndShareSnapshots(t *testing.T) {
	owner := mustPDFObjectID(t, "507f1f77bcf86cd799439011")
	actor := mustPDFObjectID(t, "507f1f77bcf86cd799439012")
	noteID := mustPDFObjectID(t, "507f1f77bcf86cd799439013")

	tests := []struct {
		name    string
		actor   domain.ObjectID
		isBlog  bool
		canRead bool
	}{
		{name: "owner", actor: owner},
		{name: "public blog", actor: actor, isBlog: true},
		{name: "shared reader", actor: actor, canRead: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &fakePDFRepository{
				note:    info.Note{NoteId: noteID, UserId: owner, Title: "Title", IsBlog: test.isBlog, IsMarkdown: true},
				content: info.NoteContent{NoteId: noteID, UserId: owner, Content: "body"},
				canRead: test.canRead,
			}
			snapshot, err := (NotePort{Repository: repository}).LoadAuthorized(context.Background(), test.actor, noteID)
			if err != nil {
				t.Fatalf("LoadAuthorized() error = %v", err)
			}
			if snapshot.OwnerID != owner || snapshot.NoteID != noteID || snapshot.Title != "Title" || snapshot.HTML != "body" || !snapshot.Markdown {
				t.Fatalf("snapshot = %+v", snapshot)
			}
			if repository.contentFor != owner {
				t.Fatalf("content owner = %s, want %s", repository.contentFor.Hex(), owner.Hex())
			}
		})
	}
}

func TestNotePortFailsClosedForUnauthorizedAndRepositoryErrors(t *testing.T) {
	owner := mustPDFObjectID(t, "507f1f77bcf86cd799439011")
	actor := mustPDFObjectID(t, "507f1f77bcf86cd799439012")
	noteID := mustPDFObjectID(t, "507f1f77bcf86cd799439013")
	repository := &fakePDFRepository{note: info.Note{NoteId: noteID, UserId: owner}}
	if _, err := (NotePort{Repository: repository}).LoadAuthorized(context.Background(), actor, noteID); pdfErrorCategory(err) != application.ErrorUnauthorized {
		t.Fatalf("unauthorized error = %v", err)
	}
	repository.err = errors.New("database unavailable")
	if _, err := (NotePort{Repository: repository}).LoadAuthorized(context.Background(), actor, noteID); pdfErrorCategory(err) != application.ErrorDependency {
		t.Fatalf("repository error = %v", err)
	}
}

type fakeNotePermission struct {
	perm    int
	allowed bool
	err     error
}

func (permission fakeNotePermission) ResolveNotePermission(context.Context, domain.ObjectID, domain.ObjectID, domain.ObjectID) (int, bool, error) {
	return permission.perm, permission.allowed, permission.err
}

func TestNotePortUsesInjectedPermissionSeam(t *testing.T) {
	owner := mustPDFObjectID(t, "507f1f77bcf86cd799439011")
	actor := mustPDFObjectID(t, "507f1f77bcf86cd799439012")
	noteID := mustPDFObjectID(t, "507f1f77bcf86cd799439013")
	repository := &fakePDFRepository{
		note:    info.Note{NoteId: noteID, UserId: owner},
		content: info.NoteContent{NoteId: noteID, UserId: owner, Content: "body"},
		canRead: false,
	}
	snapshot, err := (NotePort{Repository: repository, Permission: fakeNotePermission{perm: 0, allowed: true}}).LoadAuthorized(context.Background(), actor, noteID)
	if err != nil || snapshot.NoteID != noteID {
		t.Fatalf("LoadAuthorized() snapshot=%+v error=%v", snapshot, err)
	}
}

func TestNotePortRejectsTrashBeforePublicRead(t *testing.T) {
	owner := mustPDFObjectID(t, "507f1f77bcf86cd799439011")
	actor := mustPDFObjectID(t, "507f1f77bcf86cd799439012")
	noteID := mustPDFObjectID(t, "507f1f77bcf86cd799439013")
	repository := &fakePDFRepository{note: info.Note{NoteId: noteID, UserId: owner, IsBlog: true, IsTrash: true}}
	if _, err := (NotePort{Repository: repository, Permission: fakeNotePermission{allowed: true}}).LoadAuthorized(context.Background(), actor, noteID); pdfErrorCategory(err) != application.ErrorNotFound {
		t.Fatalf("trash note error=%v category=%q", err, pdfErrorCategory(err))
	}
}

func TestResourcePortUsesOwnerScopedLookupAndSafeContentStore(t *testing.T) {
	owner := mustPDFObjectID(t, "507f1f77bcf86cd799439011")
	fileID := mustPDFObjectID(t, "507f1f77bcf86cd799439014")
	data := onePixelPNG(t)
	repository := &fakePDFRepository{file: info.File{FileId: fileID, UserId: owner, Path: "files/a/b/image.png"}}
	store := &fakeContentStore{data: data}
	resource, err := (ResourcePort{Repository: repository, Store: store}).LoadAuthorized(context.Background(), owner, fileID, int64(len(data)))
	if err != nil {
		t.Fatalf("LoadAuthorized() error = %v", err)
	}
	if repository.fileOwner != owner || store.path.Kind != application.RootPrivateFiles || store.path.Value != "a/b/image.png" {
		t.Fatalf("owner=%s path=%+v", repository.fileOwner.Hex(), store.path)
	}
	if resource.MIME != "image/png" || !bytes.Equal(resource.Data, data) {
		t.Fatalf("resource MIME=%q bytes=%d", resource.MIME, len(resource.Data))
	}
}

type fakeRemoteFetcher struct {
	result contentremote.Result
	limit  int64
	url    string
}

func (fetcher *fakeRemoteFetcher) Fetch(_ context.Context, value string, limit int64) (contentremote.Result, error) {
	fetcher.url, fetcher.limit = value, limit
	return fetcher.result, nil
}

func TestRemoteResourcePortUsesSharedFetcherAndFailClosedUploadLimit(t *testing.T) {
	image := onePixelPNG(t)
	fetcher := &fakeRemoteFetcher{result: contentremote.Result{Data: image, Metadata: application.ImageMetadata{MIME: "image/png", Extension: ".png"}}}
	port := RemoteResourcePort{Fetcher: fetcher, UploadImageBytes: func() (int64, error) { return 1024, nil }}
	resource, err := port.LoadPublic(context.Background(), "https://cdn.example/image.png", 512)
	if err != nil || resource.MIME != "image/png" || !bytes.Equal(resource.Data, image) {
		t.Fatalf("resource=%+v err=%v", resource, err)
	}
	if fetcher.url != "https://cdn.example/image.png" || fetcher.limit != 512 {
		t.Fatalf("fetch url=%q limit=%d", fetcher.url, fetcher.limit)
	}
	port.UploadImageBytes = func() (int64, error) { return 0, errors.New("missing config") }
	if _, err := port.LoadPublic(context.Background(), "https://cdn.example/image.png", 512); pdfErrorCategory(err) != application.ErrorDependency {
		t.Fatalf("invalid config error=%v", err)
	}
}

func TestMongoPDFRepositoryFailsClosedWhenDatabaseIsUninitialized(t *testing.T) {
	savedNotes, savedContents, savedFiles := db.Notes, db.NoteContents, db.Files
	db.Notes, db.NoteContents, db.Files = nil, nil, nil
	t.Cleanup(func() {
		db.Notes, db.NoteContents, db.Files = savedNotes, savedContents, savedFiles
	})
	id := mustPDFObjectID(t, "507f1f77bcf86cd799439011")
	repository := MongoPDFRepository{}
	if _, err := repository.FindNote(context.Background(), id); !errors.Is(err, db.ErrMongoClientNotInitialized) {
		t.Fatalf("FindNote() error = %v", err)
	}
	if _, err := repository.FindNoteContent(context.Background(), id, id); !errors.Is(err, db.ErrMongoClientNotInitialized) {
		t.Fatalf("FindNoteContent() error = %v", err)
	}
	if _, err := repository.FindFile(context.Background(), id, id); !errors.Is(err, db.ErrMongoClientNotInitialized) {
		t.Fatalf("FindFile() error = %v", err)
	}
}

func mustPDFObjectID(t *testing.T, value string) domain.ObjectID {
	t.Helper()
	id, err := domain.ParseObjectID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func onePixelPNG(t *testing.T) []byte {
	t.Helper()
	var output bytes.Buffer
	value := image.NewRGBA(image.Rect(0, 0, 1, 1))
	value.Set(0, 0, color.RGBA{R: 1, A: 255})
	if err := png.Encode(&output, value); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func pdfErrorCategory(err error) application.ErrorCategory {
	var contentErr *application.Error
	if errors.As(err, &contentErr) {
		return contentErr.Category
	}
	return ""
}
