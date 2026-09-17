package contentfs

import (
	"context"
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

type LifecycleStore struct {
	dataRoots       map[application.RootKind]*os.Root
	quarantineRoots map[application.RootKind]*os.Root
	directorySync   func(*os.Root, string) error
	finalOpen       func(*os.Root, string) (*os.File, error)
	finalSync       func(*os.File) error
	finalClose      func(*os.File) error
	removalSync     func(*os.Root, string) error
}

func NewLifecycleStore(roots *ContentRoots) (*LifecycleStore, error) {
	if roots == nil {
		return nil, fmt.Errorf("content lifecycle: roots are required")
	}
	store := &LifecycleStore{
		dataRoots: make(map[application.RootKind]*os.Root, 2), quarantineRoots: make(map[application.RootKind]*os.Root, 2), directorySync: syncDirectory,
	}
	for _, kind := range []application.RootKind{application.RootPrivateFiles, application.RootPublicUpload} {
		dataPath, err := roots.dataPath(kind)
		if err != nil {
			_ = store.Close()
			return nil, err
		}
		quarantinePath, err := roots.quarantinePath(kind)
		if err != nil {
			_ = store.Close()
			return nil, err
		}
		dataRoot, err := os.OpenRoot(dataPath)
		if err != nil {
			_ = store.Close()
			return nil, err
		}
		quarantineRoot, err := os.OpenRoot(quarantinePath)
		if err != nil {
			_ = dataRoot.Close()
			_ = store.Close()
			return nil, err
		}
		store.dataRoots[kind], store.quarantineRoots[kind] = dataRoot, quarantineRoot
	}
	return store, nil
}

func (store *LifecycleStore) Close() error {
	if store == nil {
		return nil
	}
	var result error
	for _, roots := range []map[application.RootKind]*os.Root{store.dataRoots, store.quarantineRoots} {
		for kind, root := range roots {
			if root != nil {
				result = errors.Join(result, root.Close())
				roots[kind] = nil
			}
		}
	}
	return result
}

func (store *LifecycleStore) Quarantine(ctx context.Context, source application.LogicalPath, lookupKey string, digest [sha256.Size]byte, size int64) (application.LogicalPath, error) {
	if err := ctx.Err(); err != nil {
		return application.LogicalPath{}, application.NewError(application.ErrorTimeout, "quarantine_canceled", err)
	}
	dataRoot, quarantineRoot, sourceName, quarantine, err := store.resolve(source, lookupKey)
	if err != nil {
		return application.LogicalPath{}, err
	}
	quarantineName := filepath.FromSlash(quarantine.Value)
	if err := ensureLifecycleParent(quarantineRoot, filepath.Dir(quarantineName), store.directorySync); err != nil {
		return application.LogicalPath{}, err
	}
	found, err := store.relocateRooted(ctx, dataRoot, sourceName, quarantineRoot, quarantineName, digest, size)
	if err != nil {
		return application.LogicalPath{}, err
	}
	if !found {
		return application.LogicalPath{}, application.NewError(application.ErrorNotFound, "quarantine_source_missing", nil)
	}
	return quarantine, nil
}

func (store *LifecycleStore) Restore(ctx context.Context, source, quarantine application.LogicalPath, digest [sha256.Size]byte, size int64) error {
	if err := ctx.Err(); err != nil {
		return application.NewError(application.ErrorTimeout, "restore_canceled", err)
	}
	dataRoot, quarantineRoot, sourceName, _, err := store.resolve(source, strings.Repeat("0", 64))
	if err != nil {
		return err
	}
	if quarantine.Kind != source.Kind {
		return application.NewError(application.ErrorValidation, "restore_root", nil)
	}
	parsed, err := application.ParseLogicalPath(quarantine.Kind, quarantine.Value)
	if err != nil || parsed != quarantine {
		return application.NewError(application.ErrorUnsafePath, "restore_quarantine", err)
	}
	quarantineName := filepath.FromSlash(quarantine.Value)
	found, err := store.relocateRooted(ctx, quarantineRoot, quarantineName, dataRoot, sourceName, digest, size)
	if err != nil {
		return err
	}
	if !found {
		return application.NewError(application.ErrorUnknownResult, "restore_missing", nil)
	}
	return nil
}

func (store *LifecycleStore) Purge(ctx context.Context, quarantine application.LogicalPath, digest [sha256.Size]byte, size int64) error {
	if err := ctx.Err(); err != nil {
		return application.NewError(application.ErrorTimeout, "purge_canceled", err)
	}
	parsed, err := application.ParseLogicalPath(quarantine.Kind, quarantine.Value)
	if err != nil || parsed != quarantine {
		return application.NewError(application.ErrorUnsafePath, "purge_quarantine", err)
	}
	root := store.quarantineRoots[quarantine.Kind]
	if root == nil {
		return application.NewError(application.ErrorStorageUnavailable, "purge_root", nil)
	}
	name := filepath.FromSlash(quarantine.Value)
	file, exists, err := openVerifiedLifecycleFile(ctx, root, name, digest, size)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if err := removeOpenedLifecycleFile(file); err != nil {
		_ = file.Close()
		return application.NewError(application.ErrorUnsupportedFS, "purge_remove_exact", err)
	}
	closeErr := file.Close()
	syncErr := store.businessRemovalBarrier(root, name)
	if closeErr != nil || syncErr != nil {
		return application.NewError(application.ErrorUnknownResult, "purge_sync", errors.Join(closeErr, syncErr))
	}
	return nil
}

func (store *LifecycleStore) resolve(source application.LogicalPath, lookupKey string) (*os.Root, *os.Root, string, application.LogicalPath, error) {
	parsed, err := application.ParseLogicalPath(source.Kind, source.Value)
	_, lookupErr := hex.DecodeString(lookupKey)
	if err != nil || parsed != source || len(lookupKey) != sha256.Size*2 || lookupErr != nil {
		if err == nil {
			err = lookupErr
		}
		return nil, nil, "", application.LogicalPath{}, application.NewError(application.ErrorValidation, "lifecycle_input", err)
	}
	dataRoot, quarantineRoot := store.dataRoots[source.Kind], store.quarantineRoots[source.Kind]
	if dataRoot == nil || quarantineRoot == nil {
		return nil, nil, "", application.LogicalPath{}, application.NewError(application.ErrorStorageUnavailable, "lifecycle_root", nil)
	}
	quarantine := application.LogicalPath{Kind: source.Kind, Value: "objects/" + lookupKey[:2] + "/" + lookupKey}
	return dataRoot, quarantineRoot, filepath.FromSlash(source.Value), quarantine, nil
}

func ensureLifecycleParent(root *os.Root, parent string, directorySync func(*os.Root, string) error) error {
	current := ""
	for _, segment := range strings.Split(filepath.ToSlash(parent), "/") {
		if segment == "" || segment == "." {
			continue
		}
		current = filepath.Join(current, segment)
		info, err := root.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			created := false
			if err := root.Mkdir(current, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
				return application.NewError(application.ErrorStorageUnavailable, "quarantine_parent", err)
			} else if err == nil {
				created = true
			}
			info, err = root.Lstat(current)
			if err == nil && created {
				if syncErr := platformParentBarrier(root, filepath.Dir(current), newDurabilityOps(directorySync, nil, nil, nil)); syncErr != nil {
					return application.NewError(application.ErrorUnknownResult, "quarantine_parent_sync", syncErr)
				}
			}
		}
		if err != nil || !info.IsDir() || isIndirection(info) {
			return application.NewError(application.ErrorUnsafePath, "quarantine_parent", err)
		}
	}
	return nil
}

func verifiedLifecycleFile(ctx context.Context, root *os.Root, name string, digest [sha256.Size]byte, size int64) (bool, error) {
	file, exists, err := openVerifiedLifecycleFile(ctx, root, name, digest, size)
	if !exists || err != nil {
		return exists, err
	}
	if err := file.Close(); err != nil {
		return false, application.NewError(application.ErrorStorageUnavailable, "lifecycle_digest", err)
	}
	return true, nil
}

func openVerifiedLifecycleFile(ctx context.Context, root *os.Root, name string, digest [sha256.Size]byte, size int64) (*os.File, bool, error) {
	if size <= 0 {
		return nil, false, application.NewError(application.ErrorValidation, "lifecycle_size", nil)
	}
	info, err := root.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil || !info.Mode().IsRegular() || isIndirection(info) {
		return nil, false, application.NewError(application.ErrorUnsafePath, "lifecycle_file", err)
	}
	if info.Size() != size {
		return nil, false, application.NewError(application.ErrorConflict, "lifecycle_size_mismatch", nil)
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, false, application.NewError(application.ErrorStorageUnavailable, "lifecycle_open", err)
	}
	hash := sha256.New()
	_, readErr := copyLifecycleBytes(ctx, hash, file)
	if ctxErr := ctx.Err(); ctxErr != nil && readErr != nil {
		return nil, false, application.NewError(application.ErrorTimeout, "lifecycle_digest_canceled", errors.Join(readErr, file.Close(), ctxErr))
	}
	if readErr != nil {
		return nil, false, application.NewError(application.ErrorStorageUnavailable, "lifecycle_digest", errors.Join(readErr, file.Close()))
	}
	openedInfo, err := file.Stat()
	if err != nil || openedInfo.Size() != size || !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		_ = file.Close()
		return nil, false, application.NewError(application.ErrorConflict, "lifecycle_open_identity", err)
	}
	var actual [sha256.Size]byte
	copy(actual[:], hash.Sum(nil))
	if actual != digest {
		_ = file.Close()
		return nil, false, application.NewError(application.ErrorConflict, "lifecycle_digest_mismatch", nil)
	}
	return file, true, nil
}

// relocateRooted copies between two already-open roots and only then removes
// the verified source. All name resolution stays handle-relative, so replacing
// either configured absolute root path cannot redirect the operation. A crash
// after the copy leaves two digest-identical files; the next call completes by
// removing the source.
func (store *LifecycleStore) relocateRooted(ctx context.Context, sourceRoot *os.Root, sourceName string, destinationRoot *os.Root, destinationName string, digest [sha256.Size]byte, size int64) (bool, error) {
	sourceExists, err := verifiedLifecycleFile(ctx, sourceRoot, sourceName, digest, size)
	if err != nil {
		return false, err
	}
	destinationExists, err := verifiedLifecycleFile(ctx, destinationRoot, destinationName, digest, size)
	if err != nil {
		return false, err
	}
	if !sourceExists {
		if destinationExists {
			if err := store.publicationBarrier(destinationRoot, destinationName); err != nil {
				return false, application.NewError(application.ErrorUnknownResult, "lifecycle_destination_sync", err)
			}
		}
		return destinationExists, nil
	}
	if !destinationExists {
		if err := store.copyLifecycleFile(ctx, sourceRoot, sourceName, destinationRoot, destinationName, digest, size); err != nil {
			return false, err
		}
		if err := store.publicationBarrier(destinationRoot, destinationName); err != nil {
			return false, application.NewError(application.ErrorUnknownResult, "lifecycle_destination_sync", err)
		}
	} else if err := store.publicationBarrier(destinationRoot, destinationName); err != nil {
		return false, application.NewError(application.ErrorUnknownResult, "lifecycle_destination_sync", err)
	}
	// Re-read the source immediately before removing its rooted name. If it was
	// replaced during the copy, leave both sides for explicit recovery.
	verifiedSource, exists, err := openVerifiedLifecycleFile(ctx, sourceRoot, sourceName, digest, size)
	if err != nil {
		return false, err
	} else if !exists {
		return true, nil
	}
	if err := removeOpenedLifecycleFile(verifiedSource); err != nil {
		_ = verifiedSource.Close()
		return false, application.NewError(application.ErrorUnsupportedFS, "lifecycle_source_remove_exact", err)
	}
	closeErr := verifiedSource.Close()
	syncErr := store.businessRemovalBarrier(sourceRoot, sourceName)
	if closeErr != nil || syncErr != nil {
		return false, application.NewError(application.ErrorUnknownResult, "lifecycle_source_sync", errors.Join(closeErr, syncErr))
	}
	return true, nil
}

func (store *LifecycleStore) copyLifecycleFile(ctx context.Context, sourceRoot *os.Root, sourceName string, destinationRoot *os.Root, destinationName string, digest [sha256.Size]byte, size int64) error {
	source, err := sourceRoot.Open(sourceName)
	if err != nil {
		return application.NewError(application.ErrorStorageUnavailable, "lifecycle_source_open", err)
	}
	info, err := source.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != size {
		_ = source.Close()
		return application.NewError(application.ErrorUnsafePath, "lifecycle_source_file", err)
	}
	destination, err := destinationRoot.OpenFile(destinationName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		_ = source.Close()
		if errors.Is(err, fs.ErrExist) {
			return application.NewError(application.ErrorConflict, "lifecycle_destination_exists", err)
		}
		return application.NewError(application.ErrorStorageUnavailable, "lifecycle_destination_create", err)
	}
	cleanup := func() error {
		closeErr := errors.Join(source.Close(), destination.Close())
		removeErr := destinationRoot.Remove(destinationName)
		if errors.Is(removeErr, fs.ErrNotExist) {
			removeErr = nil
		}
		var syncErr error
		if removeErr == nil {
			syncErr = platformRemovalBarrier(destinationRoot, destinationName, store.durabilityOps())
		}
		return errors.Join(closeErr, removeErr, syncErr)
	}
	hash := sha256.New()
	_, copyErr := copyLifecycleBytes(ctx, io.MultiWriter(destination, hash), source)
	if copyErr == nil {
		copyErr = destination.Sync()
	}
	if copyErr != nil {
		cleanupErr := cleanup()
		if cleanupErr != nil {
			return application.NewError(application.ErrorUnknownResult, "lifecycle_copy_cleanup", errors.Join(copyErr, cleanupErr))
		}
		if ctxErr := ctx.Err(); ctxErr != nil && errors.Is(copyErr, ctxErr) {
			return application.NewError(application.ErrorTimeout, "lifecycle_copy_canceled", errors.Join(copyErr, ctxErr))
		}
		return application.NewError(application.ErrorUnknownResult, "lifecycle_copy", copyErr)
	}
	var actual [sha256.Size]byte
	copy(actual[:], hash.Sum(nil))
	if actual != digest {
		if cleanupErr := cleanup(); cleanupErr != nil {
			return application.NewError(application.ErrorUnknownResult, "lifecycle_copy_cleanup", cleanupErr)
		}
		return application.NewError(application.ErrorConflict, "lifecycle_copy_digest", nil)
	}
	if err := errors.Join(source.Close(), destination.Close()); err != nil {
		return application.NewError(application.ErrorUnknownResult, "lifecycle_copy_close", err)
	}
	return nil
}

func (store *LifecycleStore) durabilityOps() durabilityOps {
	return newDurabilityOps(store.directorySync, store.finalOpen, store.finalSync, store.finalClose)
}

func (store *LifecycleStore) publicationBarrier(root *os.Root, name string) error {
	return platformPublicationBarrier(root, name, store.durabilityOps())
}

func (store *LifecycleStore) businessRemovalBarrier(root *os.Root, name string) error {
	if store.removalSync != nil {
		return store.removalSync(root, name)
	}
	return platformBusinessRemovalBarrier(root, name, store.durabilityOps())
}

func copyLifecycleBytes(ctx context.Context, destination io.Writer, source io.Reader) (int64, error) {
	return io.Copy(destination, &contextReader{ctx: ctx, reader: source})
}

var _ application.DeleteStorage = (*LifecycleStore)(nil)
