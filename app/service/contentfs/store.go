package contentfs

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	application "github.com/yangphere/leanote/app/application/content"
)

type FileStore struct {
	handles       map[application.RootKind]*os.Root
	directorySync func(*os.Root, string) error
	finalOpen     func(*os.Root, string) (*os.File, error)
	finalSync     func(*os.File) error
	finalClose    func(*os.File) error
}

func NewFileStore(roots *ContentRoots) (*FileStore, error) {
	if roots == nil {
		return nil, fmt.Errorf("open content store: roots are required")
	}
	store := &FileStore{
		handles:       make(map[application.RootKind]*os.Root, 3),
		directorySync: syncDirectory,
	}
	for _, kind := range []application.RootKind{
		application.RootPrivateFiles,
		application.RootPublicUpload,
		application.RootTemporary,
	} {
		rootPath, err := roots.dataPath(kind)
		if err != nil {
			_ = store.Close()
			return nil, err
		}
		handle, err := os.OpenRoot(rootPath)
		if err != nil {
			_ = store.Close()
			return nil, fmt.Errorf("open content root %q: %w", kind, err)
		}
		store.handles[kind] = handle
	}
	return store, nil
}

func (store *FileStore) Close() error {
	if store == nil {
		return nil
	}
	var result error
	for kind, handle := range store.handles {
		if handle == nil {
			continue
		}
		if err := handle.Close(); err != nil {
			result = errors.Join(result, fmt.Errorf("close content root %q: %w", kind, err))
		}
		store.handles[kind] = nil
	}
	return result
}

func (store *FileStore) Open(ctx context.Context, logical application.LogicalPath) (application.OpenResult, error) {
	if err := ctx.Err(); err != nil {
		return application.OpenResult{}, application.NewError(application.ErrorTimeout, "open_canceled", err)
	}
	root, name, err := store.resolve(logical)
	if err != nil {
		return application.OpenResult{}, err
	}
	if err := store.walkExisting(root, name, false); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return application.OpenResult{}, application.NewError(application.ErrorNotFound, "content_missing", err)
		}
		var contentErr *application.Error
		if errors.As(err, &contentErr) {
			return application.OpenResult{}, err
		}
		return application.OpenResult{}, application.NewError(application.ErrorStorageUnavailable, "content_inspect", err)
	}
	file, err := root.Open(name)
	if err != nil {
		return application.OpenResult{}, application.NewError(application.ErrorStorageUnavailable, "content_open", err)
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		_ = file.Close()
		return application.OpenResult{}, application.NewError(application.ErrorStorageUnavailable, "content_stat", err)
	}
	return application.OpenResult{Reader: &opaqueReader{file: file}, Size: info.Size()}, nil
}

func (store *FileStore) Publish(ctx context.Context, request application.PublishRequest) (application.PublishResult, error) {
	result := application.PublishResult{Destination: request.Destination}
	root, destination, err := store.resolve(request.Destination)
	if err != nil {
		return result, err
	}
	if request.Source == nil || request.Identity.OperationID == "" || request.Identity.OwnerID.IsZero() || request.Identity.Kind == "" {
		return result, application.NewError(application.ErrorValidation, "invalid_publish_request", nil)
	}
	if request.Identity.Digest == ([sha256.Size]byte{}) {
		return result, application.NewError(application.ErrorValidation, "missing_digest", nil)
	}
	parent := filepath.Dir(destination)
	if parent == "." {
		parent = "."
	}
	if err := store.ensureParent(root, parent); err != nil {
		return result, err
	}
	if existing, found, err := store.readExisting(root, destination); err != nil {
		return result, err
	} else if found {
		if existing.digest != request.Identity.Digest {
			return result, application.NewError(application.ErrorConflict, "destination_digest_mismatch", nil)
		}
		if err := store.publicationBarrier(root, destination); err != nil {
			return result, application.NewError(application.ErrorUnknownResult, "publish_final_sync", err)
		}
		return existingPublishResult(request.Destination, existing), nil
	}

	stage, stageName, err := createUniqueStage(root, parent)
	if err != nil {
		return result, application.NewError(application.ErrorStorageUnavailable, "create_staging", err)
	}
	stageOpen := true
	cleanupStage := func() error {
		var cleanupErr error
		if stageOpen {
			cleanupErr = errors.Join(cleanupErr, stage.Close())
			stageOpen = false
		}
		if err := root.Remove(stageName); err != nil && !errors.Is(err, fs.ErrNotExist) {
			cleanupErr = errors.Join(cleanupErr, err)
		}
		if err := store.removalBarrier(root, stageName); err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
		}
		return cleanupErr
	}
	failWithCleanup := func(operationErr error) (application.PublishResult, error) {
		if cleanupErr := cleanupStage(); cleanupErr != nil {
			return result, application.NewError(application.ErrorUnknownResult, "staging_cleanup", errors.Join(operationErr, cleanupErr))
		}
		return result, operationErr
	}

	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(stage, hasher), &contextReader{ctx: ctx, reader: request.Source})
	if err != nil {
		return failWithCleanup(application.NewError(application.ErrorStorageUnavailable, "write_staging", err))
	}
	if err := stage.Sync(); err != nil {
		return failWithCleanup(application.NewError(application.ErrorUnsupportedFS, "sync_staging", err))
	}
	if err := stage.Close(); err != nil {
		stageOpen = false
		return failWithCleanup(application.NewError(application.ErrorStorageUnavailable, "close_staging", err))
	}
	stageOpen = false
	var digest [sha256.Size]byte
	copy(digest[:], hasher.Sum(nil))
	if digest != request.Identity.Digest {
		return failWithCleanup(application.NewError(application.ErrorConflict, "source_digest_mismatch", nil))
	}

	if err := root.Link(stageName, destination); err != nil {
		if errors.Is(err, fs.ErrExist) {
			if _, cleanupErr := failWithCleanup(nil); cleanupErr != nil {
				return result, cleanupErr
			}
			existing, found, verifyErr := store.readExisting(root, destination)
			if verifyErr != nil {
				return result, verifyErr
			}
			if found {
				if existing.digest != request.Identity.Digest {
					return result, application.NewError(application.ErrorConflict, "destination_digest_mismatch", nil)
				}
				if err := store.publicationBarrier(root, destination); err != nil {
					return result, application.NewError(application.ErrorUnknownResult, "publish_final_sync", err)
				}
				return existingPublishResult(request.Destination, existing), nil
			}
			return result, application.NewError(application.ErrorUnknownResult, "publish_race", err)
		}
		return failWithCleanup(application.NewError(application.ErrorUnsupportedFS, "no_replace_publish", err))
	}
	if err := store.publicationBarrier(root, destination); err != nil {
		return failWithCleanup(application.NewError(application.ErrorUnknownResult, "publish_final_sync", err))
	}
	if err := cleanupStage(); err != nil {
		return result, application.NewError(application.ErrorUnknownResult, "publish_cleanup_or_sync", err)
	}
	result.Status = application.PublishApplied
	result.Digest = digest
	result.Size = written
	return result, nil
}

func (store *FileStore) Verify(ctx context.Context, request application.VerifyRequest) (application.VerifyResult, error) {
	result := application.VerifyResult{Status: application.VerificationUnknown}
	root, destination, err := store.resolve(request.Destination)
	if err != nil {
		return result, err
	}
	expected := request.ExpectedDigest
	if expected == ([sha256.Size]byte{}) {
		expected = request.Identity.Digest
	}
	if expected == ([sha256.Size]byte{}) {
		return result, application.NewError(application.ErrorValidation, "missing_digest", nil)
	}
	verified, found, err := store.readExisting(root, destination)
	if err != nil {
		return result, err
	}
	if !found {
		result.Status = application.VerificationNotApplied
		return result, nil
	}
	result.Digest = verified.digest
	result.Size = verified.size
	if verified.digest != expected {
		result.Status = application.VerificationConflict
		return result, nil
	}
	if err := store.publicationBarrier(root, destination); err != nil {
		return result, application.NewError(application.ErrorUnknownResult, "verify_final_sync", err)
	}
	for _, dependency := range []func(context.Context) (bool, error){request.VerifyMetadata, request.VerifyProjection} {
		if dependency == nil {
			continue
		}
		applied, dependencyErr := dependency(ctx)
		if dependencyErr != nil {
			return result, application.NewError(application.ErrorDependency, "verify_dependency", dependencyErr)
		}
		if !applied {
			result.Status = application.VerificationNotApplied
			return result, nil
		}
	}
	result.Status = application.VerificationApplied
	return result, nil
}

func (store *FileStore) resolve(logical application.LogicalPath) (*os.Root, string, error) {
	parsed, err := application.ParseLogicalPath(logical.Kind, logical.Value)
	if err != nil || parsed != logical {
		if err == nil {
			err = fmt.Errorf("logical path changed during validation")
		}
		return nil, "", application.NewError(application.ErrorUnsafePath, "invalid_logical_path", err)
	}
	root := store.handles[logical.Kind]
	if root == nil {
		return nil, "", application.NewError(application.ErrorStorageUnavailable, "root_unavailable", nil)
	}
	return root, filepath.FromSlash(logical.Value), nil
}

func (store *FileStore) ensureParent(root *os.Root, parent string) error {
	if parent == "." {
		return nil
	}
	current := ""
	for _, segment := range strings.Split(filepath.ToSlash(parent), "/") {
		if current == "" {
			current = segment
		} else {
			current = filepath.Join(current, segment)
		}
		info, err := root.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			before := filepath.Dir(current)
			if before == "." {
				before = "."
			}
			if err := root.Mkdir(current, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
				return application.NewError(application.ErrorStorageUnavailable, "create_parent", err)
			}
			if err := platformParentBarrier(root, before, store.durabilityOps()); err != nil {
				return application.NewError(application.ErrorUnsupportedFS, "sync_parent", err)
			}
			info, err = root.Lstat(current)
		}
		if err != nil {
			return application.NewError(application.ErrorStorageUnavailable, "inspect_parent", err)
		}
		if !info.IsDir() || isIndirection(info) {
			return application.NewError(application.ErrorUnsafePath, "indirect_parent", nil)
		}
	}
	return store.walkExisting(root, parent, true)
}

func (store *FileStore) durabilityOps() durabilityOps {
	return newDurabilityOps(store.directorySync, store.finalOpen, store.finalSync, store.finalClose)
}

func (store *FileStore) publicationBarrier(root *os.Root, name string) error {
	return platformPublicationBarrier(root, name, store.durabilityOps())
}

func (store *FileStore) removalBarrier(root *os.Root, name string) error {
	return platformRemovalBarrier(root, name, store.durabilityOps())
}

func (store *FileStore) walkExisting(root *os.Root, name string, finalMustBeDirectory bool) error {
	current := ""
	for index, segment := range strings.Split(filepath.ToSlash(name), "/") {
		if segment == "." || segment == "" {
			continue
		}
		if current == "" {
			current = segment
		} else {
			current = filepath.Join(current, segment)
		}
		info, err := root.Lstat(current)
		if err != nil {
			return err
		}
		if isIndirection(info) {
			return application.NewError(application.ErrorUnsafePath, "indirect_path", nil)
		}
		last := index == len(strings.Split(filepath.ToSlash(name), "/"))-1
		if (!last || finalMustBeDirectory) && !info.IsDir() {
			return application.NewError(application.ErrorUnsafePath, "non_directory_parent", nil)
		}
		if last && !finalMustBeDirectory && !info.Mode().IsRegular() {
			return application.NewError(application.ErrorUnsafePath, "non_regular_destination", nil)
		}
	}
	return nil
}

type verifiedFile struct {
	digest [sha256.Size]byte
	size   int64
}

func (store *FileStore) readExisting(root *os.Root, name string) (verifiedFile, bool, error) {
	if err := store.walkExisting(root, name, false); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return verifiedFile{}, false, nil
		}
		var contentErr *application.Error
		if errors.As(err, &contentErr) {
			return verifiedFile{}, false, err
		}
		return verifiedFile{}, false, application.NewError(application.ErrorStorageUnavailable, "inspect_destination", err)
	}
	file, err := root.Open(name)
	if err != nil {
		return verifiedFile{}, false, application.NewError(application.ErrorStorageUnavailable, "open_destination", err)
	}
	hasher := sha256.New()
	size, readErr := io.Copy(hasher, file)
	closeErr := file.Close()
	if readErr != nil || closeErr != nil {
		return verifiedFile{}, false, application.NewError(application.ErrorStorageUnavailable, "read_destination", errors.Join(readErr, closeErr))
	}
	var digest [sha256.Size]byte
	copy(digest[:], hasher.Sum(nil))
	verified := verifiedFile{digest: digest, size: size}
	return verified, true, nil
}

func existingPublishResult(destination application.LogicalPath, file verifiedFile) application.PublishResult {
	return application.PublishResult{
		Status: application.PublishAlreadyApplied, Destination: destination,
		Digest: file.digest, Size: file.size,
	}
}

func createUniqueStage(root *os.Root, parent string) (*os.File, string, error) {
	for attempt := 0; attempt < 32; attempt++ {
		random := make([]byte, 16)
		if _, err := rand.Read(random); err != nil {
			return nil, "", err
		}
		name := ".leanote-stage-" + hex.EncodeToString(random)
		if parent != "." {
			name = filepath.Join(parent, name)
		}
		file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			return file, name, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, "", err
		}
	}
	return nil, "", fmt.Errorf("staging namespace exhausted")
}

func syncDirectory(root *os.Root, name string) error {
	directory, err := root.Open(name)
	if err != nil {
		return err
	}
	return errors.Join(directory.Sync(), directory.Close())
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

type opaqueReader struct {
	file *os.File
}

func (reader *opaqueReader) Read(buffer []byte) (int, error) { return reader.file.Read(buffer) }
func (reader *opaqueReader) Close() error                    { return reader.file.Close() }

func (reader *contextReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.reader.Read(buffer)
}

var _ application.ContentStore = (*FileStore)(nil)
