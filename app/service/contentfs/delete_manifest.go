package contentfs

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	application "github.com/yangphere/leanote/app/application/content"
)

// DeleteManifestStore persists only generic content deletion recovery state.
// It intentionally has no USN, history, or note receipt fields.
type DeleteManifestStore struct {
	handles       map[application.RootKind]*os.Root
	directorySync func(*os.Root, string) error
}

const (
	deleteManifestRetention = 7 * 24 * time.Hour
	maxDeleteManifestBytes  = 1 << 20
)

func NewDeleteManifestStore(roots *ContentRoots) (*DeleteManifestStore, error) {
	if roots == nil {
		return nil, fmt.Errorf("delete manifest store: roots are required")
	}
	store := &DeleteManifestStore{handles: make(map[application.RootKind]*os.Root, 2), directorySync: syncDirectory}
	for _, kind := range []application.RootKind{application.RootPrivateFiles, application.RootPublicUpload} {
		rootPath, err := roots.quarantinePath(kind)
		if err != nil {
			_ = store.Close()
			return nil, err
		}
		handle, err := os.OpenRoot(rootPath)
		if err != nil {
			_ = store.Close()
			return nil, fmt.Errorf("open delete manifest root %q: %w", kind, err)
		}
		store.handles[kind] = handle
	}
	return store, nil
}

func (store *DeleteManifestStore) Close() error {
	var result error
	for kind, root := range store.handles {
		if root != nil {
			result = errors.Join(result, root.Close())
			store.handles[kind] = nil
		}
	}
	return result
}

func (store *DeleteManifestStore) Create(ctx context.Context, manifest application.DeleteManifest) error {
	if err := ctx.Err(); err != nil {
		return application.NewError(application.ErrorTimeout, "delete_manifest_canceled", err)
	}
	if err := manifest.Validate(); err != nil {
		return err
	}
	root, name, err := store.location(manifest.Source.Kind, manifest.LookupKey)
	if err != nil {
		return err
	}
	return store.withLease(root, manifest.LookupKey, func() error {
		if err := ensureManifestParent(root, filepath.Dir(name)); err != nil {
			return err
		}
		return store.writeManifestNoReplace(root, name, manifest)
	})
}

func (store *DeleteManifestStore) LoadDeleteManifest(ctx context.Context, lookupKey string) (application.DeleteManifest, bool, error) {
	if err := ctx.Err(); err != nil {
		return application.DeleteManifest{}, false, application.NewError(application.ErrorTimeout, "delete_manifest_canceled", err)
	}
	if !validLookupKey(lookupKey) {
		return application.DeleteManifest{}, false, application.NewError(application.ErrorValidation, "delete_manifest_lookup", nil)
	}
	var result application.DeleteManifest
	found := false
	for kind, root := range store.handles {
		if root == nil {
			return application.DeleteManifest{}, false, application.NewError(application.ErrorStorageUnavailable, "delete_manifest_root_unavailable", nil)
		}
		manifest, exists, err := readManifest(root, manifestName(lookupKey))
		if err != nil {
			return application.DeleteManifest{}, false, err
		}
		if !exists {
			continue
		}
		if manifest.Source.Kind != kind && manifest.Stage != application.DeleteStageTerminal {
			return application.DeleteManifest{}, false, application.NewError(application.ErrorConflict, "delete_manifest_root_mismatch", nil)
		}
		if found {
			return application.DeleteManifest{}, false, application.NewError(application.ErrorConflict, "delete_manifest_lookup_ambiguous", nil)
		}
		result, found = manifest, true
	}
	return result, found, nil
}

func (store *DeleteManifestStore) CompareAndSwap(ctx context.Context, lookupKey string, version uint64, digest [32]byte, next application.DeleteManifest) error {
	if err := ctx.Err(); err != nil {
		return application.NewError(application.ErrorTimeout, "delete_manifest_canceled", err)
	}
	if !validLookupKey(lookupKey) || lookupKey != next.LookupKey || next.Version != version+1 || next.StateDigest == digest {
		return application.NewError(application.ErrorValidation, "delete_manifest_cas_input", nil)
	}
	if err := next.Validate(); err != nil {
		return err
	}
	kind := next.Source.Kind
	if next.Stage == application.DeleteStageTerminal {
		current, found, err := store.LoadDeleteManifest(ctx, lookupKey)
		if err != nil {
			return err
		}
		if !found {
			return application.NewError(application.ErrorConflict, "delete_manifest_missing", nil)
		}
		kind = current.Source.Kind
	}
	root, name, err := store.location(kind, lookupKey)
	if err != nil {
		return err
	}
	return store.withLease(root, lookupKey, func() error {
		current, found, err := readManifest(root, name)
		if err != nil {
			return err
		}
		if !found || current.Version != version || current.StateDigest != digest {
			return application.NewError(application.ErrorConflict, "delete_manifest_stale", nil)
		}
		return store.replaceManifest(root, name, next)
	})
}

// GCTerminals removes terminal markers strictly older than before. Each
// deletion obtains the same identity lease and re-reads the marker first.
func (store *DeleteManifestStore) GCTerminals(ctx context.Context, before time.Time) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, application.NewError(application.ErrorTimeout, "delete_manifest_gc_canceled", err)
	}
	if before.IsZero() {
		return nil, application.NewError(application.ErrorValidation, "delete_manifest_gc_before", nil)
	}
	removed := []string{}
	for _, root := range store.handles {
		keys, err := terminalManifestKeys(root)
		if err != nil {
			return removed, err
		}
		for _, key := range keys {
			if err := store.withLease(root, key, func() error {
				manifest, found, err := readManifest(root, manifestName(key))
				if err != nil || !found || manifest.Stage != application.DeleteStageTerminal || !manifest.TerminalAt.Add(deleteManifestRetention).Before(before) {
					return err
				}
				if err := root.Remove(manifestName(key)); err != nil {
					return application.NewError(application.ErrorStorageUnavailable, "delete_manifest_gc_remove", err)
				}
				if err := store.directorySync(root, filepath.Dir(manifestName(key))); err != nil {
					return application.NewError(application.ErrorUnknownResult, "delete_manifest_gc_sync", err)
				}
				removed = append(removed, key)
				return nil
			}); err != nil {
				return removed, err
			}
		}
	}
	return removed, nil
}

func (store *DeleteManifestStore) location(kind application.RootKind, lookupKey string) (*os.Root, string, error) {
	root := store.handles[kind]
	if root == nil {
		return nil, "", application.NewError(application.ErrorStorageUnavailable, "delete_manifest_root_unavailable", nil)
	}
	if !validLookupKey(lookupKey) {
		return nil, "", application.NewError(application.ErrorValidation, "delete_manifest_lookup", nil)
	}
	return root, manifestName(lookupKey), nil
}

func manifestName(lookupKey string) string {
	return filepath.Join("manifests", lookupKey[:2], lookupKey+".json")
}

func validLookupKey(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func (store *DeleteManifestStore) withLease(root *os.Root, lookupKey string, operation func() error) error {
	lease := filepath.Join("leases", lookupKey+".lease")
	if err := ensureManifestParent(root, filepath.Dir(lease)); err != nil {
		return err
	}
	file, err := root.OpenFile(lease, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return application.NewError(application.ErrorConflict, "delete_manifest_lease_held", err)
		}
		return application.NewError(application.ErrorStorageUnavailable, "delete_manifest_lease", err)
	}
	if err := errors.Join(file.Sync(), file.Close()); err != nil {
		_ = root.Remove(lease)
		return application.NewError(application.ErrorUnknownResult, "delete_manifest_lease_sync", err)
	}
	defer func() { _ = root.Remove(lease) }()
	return operation()
}

func ensureManifestParent(root *os.Root, parent string) error {
	if parent == "." {
		return nil
	}
	current := ""
	for _, segment := range strings.Split(filepath.ToSlash(parent), "/") {
		current = filepath.Join(current, segment)
		info, err := root.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			if err := root.Mkdir(current, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
				return application.NewError(application.ErrorStorageUnavailable, "delete_manifest_parent", err)
			}
			info, err = root.Lstat(current)
		}
		if err != nil || !info.IsDir() || isIndirection(info) {
			return application.NewError(application.ErrorUnsafePath, "delete_manifest_parent", err)
		}
	}
	return nil
}

func (store *DeleteManifestStore) writeManifestNoReplace(root *os.Root, name string, manifest application.DeleteManifest) error {
	stage, stageName, err := writeManifestStage(root, filepath.Dir(name), manifest)
	if err != nil {
		return err
	}
	defer func() { _ = root.Remove(stageName) }()
	if err := stage.Close(); err != nil {
		return application.NewError(application.ErrorStorageUnavailable, "delete_manifest_close", err)
	}
	if err := root.Link(stageName, name); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return application.NewError(application.ErrorConflict, "delete_manifest_exists", fs.ErrExist)
		}
		return application.NewError(application.ErrorUnsupportedFS, "delete_manifest_no_replace", err)
	}
	if err := store.directorySync(root, filepath.Dir(name)); err != nil {
		return application.NewError(application.ErrorUnknownResult, "delete_manifest_publish_sync", err)
	}
	return nil
}

func (store *DeleteManifestStore) replaceManifest(root *os.Root, name string, manifest application.DeleteManifest) error {
	stage, stageName, err := writeManifestStage(root, filepath.Dir(name), manifest)
	if err != nil {
		return err
	}
	if err := stage.Close(); err != nil {
		_ = root.Remove(stageName)
		return application.NewError(application.ErrorStorageUnavailable, "delete_manifest_close", err)
	}
	if err := root.Rename(stageName, name); err != nil {
		_ = root.Remove(stageName)
		return application.NewError(application.ErrorUnsupportedFS, "delete_manifest_atomic_replace", err)
	}
	if err := store.directorySync(root, filepath.Dir(name)); err != nil {
		return application.NewError(application.ErrorUnknownResult, "delete_manifest_replace_sync", err)
	}
	return nil
}

func writeManifestStage(root *os.Root, parent string, manifest application.DeleteManifest) (*os.File, string, error) {
	data, err := json.Marshal(manifest)
	if err != nil {
		return nil, "", application.NewError(application.ErrorStorageUnavailable, "delete_manifest_encode", err)
	}
	stage, stageName, err := createUniqueStage(root, parent)
	if err != nil {
		return nil, "", application.NewError(application.ErrorStorageUnavailable, "delete_manifest_stage", err)
	}
	if _, err := stage.Write(data); err != nil {
		_ = stage.Close()
		_ = root.Remove(stageName)
		return nil, "", application.NewError(application.ErrorStorageUnavailable, "delete_manifest_write", err)
	}
	if err := stage.Chmod(0o600); err != nil {
		_ = stage.Close()
		_ = root.Remove(stageName)
		return nil, "", application.NewError(application.ErrorStorageUnavailable, "delete_manifest_mode", err)
	}
	if err := stage.Sync(); err != nil {
		_ = stage.Close()
		_ = root.Remove(stageName)
		return nil, "", application.NewError(application.ErrorUnsupportedFS, "delete_manifest_stage_sync", err)
	}
	return stage, stageName, nil
}

func readManifest(root *os.Root, name string) (application.DeleteManifest, bool, error) {
	info, err := root.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return application.DeleteManifest{}, false, nil
	}
	if err != nil || isIndirection(info) || !manifestModeSafe(info) {
		return application.DeleteManifest{}, false, application.NewError(application.ErrorConflict, "delete_manifest_unsafe", err)
	}
	file, err := root.Open(name)
	if err != nil {
		return application.DeleteManifest{}, false, application.NewError(application.ErrorStorageUnavailable, "delete_manifest_open", err)
	}
	info, statErr := file.Stat()
	if statErr != nil {
		closeErr := file.Close()
		return application.DeleteManifest{}, false, application.NewError(application.ErrorStorageUnavailable, "delete_manifest_stat", errors.Join(statErr, closeErr))
	}
	if info.Size() > maxDeleteManifestBytes {
		closeErr := file.Close()
		return application.DeleteManifest{}, false, application.NewError(application.ErrorConflict, "delete_manifest_too_large", closeErr)
	}
	decoder := json.NewDecoder(io.LimitReader(file, maxDeleteManifestBytes))
	var manifest application.DeleteManifest
	decodeErr := decoder.Decode(&manifest)
	if decodeErr == nil {
		var trailing any
		if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
			if err == nil {
				err = errors.New("delete manifest has trailing JSON value")
			}
			decodeErr = err
		}
	}
	closeErr := file.Close()
	if decodeErr != nil || closeErr != nil || manifest.Validate() != nil {
		return application.DeleteManifest{}, false, application.NewError(application.ErrorConflict, "delete_manifest_corrupt", errors.Join(decodeErr, closeErr, manifest.Validate()))
	}
	return manifest, true, nil
}

func terminalManifestKeys(root *os.Root) ([]string, error) {
	directory, err := root.Open("manifests")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, application.NewError(application.ErrorStorageUnavailable, "delete_manifest_gc_open", err)
	}
	defer directory.Close()
	keys := []string{}
	shards, err := directory.ReadDir(-1)
	if err != nil {
		return nil, application.NewError(application.ErrorStorageUnavailable, "delete_manifest_gc_list", err)
	}
	for _, shard := range shards {
		if !shard.IsDir() || len(shard.Name()) != 2 {
			continue
		}
		shardDir, err := root.Open(filepath.Join("manifests", shard.Name()))
		if err != nil {
			return nil, application.NewError(application.ErrorStorageUnavailable, "delete_manifest_gc_shard", err)
		}
		entries, readErr := shardDir.ReadDir(-1)
		closeErr := shardDir.Close()
		if readErr != nil || closeErr != nil {
			return nil, application.NewError(application.ErrorStorageUnavailable, "delete_manifest_gc_list", errors.Join(readErr, closeErr))
		}
		for _, entry := range entries {
			name := strings.TrimSuffix(entry.Name(), ".json")
			if !entry.IsDir() && validLookupKey(name) {
				keys = append(keys, name)
			}
		}
	}
	return keys, nil
}

var _ application.DeleteManifestReader = (*DeleteManifestStore)(nil)
