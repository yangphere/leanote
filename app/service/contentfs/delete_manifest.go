package contentfs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	application "github.com/yangphere/leanote/app/application/content"
)

// DeleteManifestStore persists only generic content deletion recovery state.
// It intentionally has no USN, history, or note receipt fields.
type DeleteManifestStore struct {
	handles       map[application.RootKind]*os.Root
	directorySync func(*os.Root, string) error
	finalOpen     func(*os.Root, string) (*os.File, error)
	finalSync     func(*os.File) error
	finalClose    func(*os.File) error
	deleteScan    atomic.Uint64
	createScan    atomic.Uint64
}

const (
	deleteManifestRetention = 7 * 24 * time.Hour
	maxDeleteManifestBytes  = 1 << 20
	maxLeaseRecordBytes     = 4 << 10
)

type manifestLeaseRecord struct {
	Version    uint32    `json:"version"`
	LookupKey  string    `json:"lookupKey"`
	Owner      string    `json:"owner"`
	Epoch      uint64    `json:"epoch"`
	AcquiredAt time.Time `json:"acquiredAt"`
}

func NewDeleteManifestStore(roots *ContentRoots) (*DeleteManifestStore, error) {
	if roots == nil {
		return nil, fmt.Errorf("delete manifest store: roots are required")
	}
	store := &DeleteManifestStore{handles: make(map[application.RootKind]*os.Root, 2), directorySync: syncDirectory}
	seed := uint64(time.Now().UTC().Unix()/int64(time.Minute/time.Second)) * 0x9e3779b97f4a7c15
	store.deleteScan.Store(seed)
	store.createScan.Store(seed + 0x517cc1b727220a95)
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
		if err := ensureManifestParent(root, filepath.Dir(name), store.directorySync); err != nil {
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
		if err := store.publicationBarrier(root, manifestName(lookupKey)); err != nil {
			return application.DeleteManifest{}, false, application.NewError(application.ErrorUnknownResult, "delete_manifest_load_sync", err)
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

type TerminalGCResult struct {
	Scanned   int
	Removed   []string
	Truncated bool
}

// GCTerminals removes terminal markers strictly older than before. Each
// deletion obtains the same identity lease and re-reads the marker first.
func (store *DeleteManifestStore) GCTerminals(ctx context.Context, before time.Time) ([]string, error) {
	removed := []string{}
	for {
		result, err := store.GCTerminalsBounded(ctx, before, 128)
		removed = append(removed, result.Removed...)
		if err != nil || !result.Truncated || len(result.Removed) == 0 {
			return removed, err
		}
	}
}

// GCTerminalsBounded examines at most limit valid manifest records. It returns
// explicit progress so startup maintenance is bounded and observable.
func (store *DeleteManifestStore) GCTerminalsBounded(ctx context.Context, before time.Time, limit int) (TerminalGCResult, error) {
	if err := ctx.Err(); err != nil {
		return TerminalGCResult{}, application.NewError(application.ErrorTimeout, "delete_manifest_gc_canceled", err)
	}
	if before.IsZero() || limit <= 0 {
		return TerminalGCResult{}, application.NewError(application.ErrorValidation, "delete_manifest_gc_input", nil)
	}
	result := TerminalGCResult{Removed: []string{}}
	for _, root := range store.handles {
		remaining := limit - result.Scanned
		if remaining == 0 {
			result.Truncated = true
			break
		}
		keys, truncated, err := terminalManifestKeys(ctx, root, store.deleteScan.Add(0x9e3779b97f4a7c15), before, remaining)
		if err != nil {
			return result, err
		}
		result.Scanned += len(keys)
		result.Truncated = result.Truncated || truncated
		for _, key := range keys {
			if err := ctx.Err(); err != nil {
				return result, application.NewError(application.ErrorTimeout, "delete_manifest_gc_canceled", err)
			}
			if err := store.withLease(root, key, func() error {
				manifest, found, err := readManifest(root, manifestName(key))
				if err != nil || !found || manifest.Stage != application.DeleteStageTerminal || !manifest.TerminalAt.Add(deleteManifestRetention).Before(before) {
					return err
				}
				if err := ctx.Err(); err != nil {
					return application.NewError(application.ErrorTimeout, "delete_manifest_gc_canceled", err)
				}
				if err := root.Remove(manifestName(key)); err != nil {
					return application.NewError(application.ErrorStorageUnavailable, "delete_manifest_gc_remove", err)
				}
				if err := store.removalBarrier(root, manifestName(key)); err != nil {
					return application.NewError(application.ErrorUnknownResult, "delete_manifest_gc_sync", err)
				}
				result.Removed = append(result.Removed, key)
				return nil
			}); err != nil {
				return result, err
			}
		}
		if truncated {
			break
		}
	}
	return result, nil
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
	leaseName, recordName := manifestLeaseNames(lookupKey)
	// Lease paths are permanent coordination artifacts. The kernel lock is the
	// live mutual-exclusion authority; if a crash loses a newly created lease
	// directory entry, the process lock is already released and the path can be
	// recreated without losing manifest, row, or content state.
	if err := ensureManifestParent(root, filepath.Dir(leaseName), func(*os.Root, string) error { return nil }); err != nil {
		return err
	}
	file, err := root.OpenFile(leaseName, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return application.NewError(application.ErrorStorageUnavailable, "delete_manifest_lease", err)
	}
	unlock, held, lockErr := tryLockFile(file)
	if held {
		_ = file.Close()
		return application.NewError(application.ErrorConflict, "delete_manifest_lease_held", nil)
	}
	if lockErr != nil {
		_ = file.Close()
		return application.NewError(application.ErrorUnsupportedFS, "delete_manifest_lease_lock", lockErr)
	}

	previous, found, err := readLeaseRecord(root, recordName)
	if err != nil {
		_ = unlock()
		_ = file.Close()
		return err
	}
	epoch := uint64(1)
	if found {
		if previous.Epoch == ^uint64(0) {
			_ = unlock()
			_ = file.Close()
			return application.NewError(application.ErrorConflict, "delete_manifest_lease_epoch_exhausted", nil)
		}
		epoch = previous.Epoch + 1
	}
	ownerBytes := make([]byte, 16)
	if _, err := rand.Read(ownerBytes); err != nil {
		_ = unlock()
		_ = file.Close()
		return application.NewError(application.ErrorStorageUnavailable, "delete_manifest_lease_owner", err)
	}
	record := manifestLeaseRecord{Version: 1, LookupKey: lookupKey, Owner: hex.EncodeToString(ownerBytes), Epoch: epoch, AcquiredAt: time.Now().UTC()}
	if err := store.writeLeaseRecord(root, recordName, record); err != nil {
		_ = unlock()
		_ = file.Close()
		return err
	}

	operationErr := operation()
	releaseErr := errors.Join(unlock(), file.Close())
	if releaseErr != nil {
		return application.NewError(application.ErrorUnknownResult, "delete_manifest_lease_release", errors.Join(operationErr, releaseErr))
	}
	return operationErr
}

func manifestLeaseNames(lookupKey string) (string, string) {
	shard := lookupKey[:2]
	return filepath.Join("leases", shard+".lease"), filepath.Join("leases", shard+".json")
}

func readLeaseRecord(root *os.Root, name string) (manifestLeaseRecord, bool, error) {
	info, err := root.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return manifestLeaseRecord{}, false, nil
	}
	if err != nil || isIndirection(info) || !manifestModeSafe(info) || info.Size() > maxLeaseRecordBytes {
		return manifestLeaseRecord{}, false, application.NewError(application.ErrorConflict, "delete_manifest_lease_corrupt", err)
	}
	file, err := root.Open(name)
	if err != nil {
		return manifestLeaseRecord{}, false, application.NewError(application.ErrorStorageUnavailable, "delete_manifest_lease_record_open", err)
	}
	decoder := json.NewDecoder(io.LimitReader(file, maxLeaseRecordBytes))
	var record manifestLeaseRecord
	decodeErr := decoder.Decode(&record)
	if decodeErr == nil {
		var trailing any
		if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
			if err == nil {
				err = errors.New("lease record has trailing JSON value")
			}
			decodeErr = err
		}
	}
	closeErr := file.Close()
	if decodeErr != nil || closeErr != nil || record.Version != 1 || !validLookupKey(record.LookupKey) || len(record.Owner) != 32 || record.Epoch == 0 || record.AcquiredAt.IsZero() {
		return manifestLeaseRecord{}, false, application.NewError(application.ErrorConflict, "delete_manifest_lease_corrupt", errors.Join(decodeErr, closeErr))
	}
	if _, err := hex.DecodeString(record.Owner); err != nil {
		return manifestLeaseRecord{}, false, application.NewError(application.ErrorConflict, "delete_manifest_lease_corrupt", err)
	}
	return record, true, nil
}

func (store *DeleteManifestStore) writeLeaseRecord(root *os.Root, name string, record manifestLeaseRecord) error {
	data, err := json.Marshal(record)
	if err != nil {
		return application.NewError(application.ErrorStorageUnavailable, "delete_manifest_lease_record_encode", err)
	}
	stage, stageName, err := createUniqueStage(root, filepath.Dir(name))
	if err != nil {
		return application.NewError(application.ErrorStorageUnavailable, "delete_manifest_lease_record_stage", err)
	}
	cleanup := func() { _ = root.Remove(stageName) }
	if _, err := stage.Write(data); err != nil {
		_ = stage.Close()
		cleanup()
		return application.NewError(application.ErrorStorageUnavailable, "delete_manifest_lease_record_write", err)
	}
	if err := errors.Join(stage.Chmod(0o600), stage.Sync(), stage.Close()); err != nil {
		cleanup()
		return application.NewError(application.ErrorUnknownResult, "delete_manifest_lease_record_sync", err)
	}
	if err := root.Rename(stageName, name); err != nil {
		cleanup()
		return application.NewError(application.ErrorUnsupportedFS, "delete_manifest_lease_record_replace", err)
	}
	// The owner/epoch record is diagnostic fencing under the permanent kernel
	// lock, not durable business state. A crash that loses this directory entry
	// also releases the lock; the next owner safely recreates the record. File
	// sync and atomic replacement still prevent live readers from seeing a torn
	// record. Manifest/data publication keeps the strict directory-sync rule.
	return nil
}

func ensureManifestParent(root *os.Root, parent string, directorySync func(*os.Root, string) error) error {
	if parent == "." {
		return nil
	}
	current := ""
	for _, segment := range strings.Split(filepath.ToSlash(parent), "/") {
		current = filepath.Join(current, segment)
		info, err := root.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			created := false
			if err := root.Mkdir(current, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
				return application.NewError(application.ErrorStorageUnavailable, "delete_manifest_parent", err)
			} else if err == nil {
				created = true
			}
			info, err = root.Lstat(current)
			if err == nil && created {
				if syncErr := platformParentBarrier(root, filepath.Dir(current), newDurabilityOps(directorySync, nil, nil, nil)); syncErr != nil {
					return application.NewError(application.ErrorUnknownResult, "delete_manifest_parent_sync", syncErr)
				}
			}
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
			if syncErr := store.publicationBarrier(root, name); syncErr != nil {
				return application.NewError(application.ErrorUnknownResult, "delete_manifest_publish_sync", syncErr)
			}
			return application.NewError(application.ErrorConflict, "delete_manifest_exists", fs.ErrExist)
		}
		return application.NewError(application.ErrorUnsupportedFS, "delete_manifest_no_replace", err)
	}
	if err := store.publicationBarrier(root, name); err != nil {
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
	if err := store.publicationBarrier(root, name); err != nil {
		return application.NewError(application.ErrorUnknownResult, "delete_manifest_replace_sync", err)
	}
	return nil
}

func (store *DeleteManifestStore) durabilityOps() durabilityOps {
	return newDurabilityOps(store.directorySync, store.finalOpen, store.finalSync, store.finalClose)
}

func (store *DeleteManifestStore) publicationBarrier(root *os.Root, name string) error {
	return platformPublicationBarrier(root, name, store.durabilityOps())
}

func (store *DeleteManifestStore) removalBarrier(root *os.Root, name string) error {
	return platformRemovalBarrier(root, name, store.durabilityOps())
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

type rankedManifestKey struct {
	key  string
	rank uint64
}

func terminalManifestKeys(ctx context.Context, root *os.Root, cursor uint64, before time.Time, limit int) ([]string, bool, error) {
	ranked := make([]rankedManifestKey, 0, limit+1)
	startShard := int(cursor >> 56)
	for offset := 0; offset < 256; offset++ {
		if err := ctx.Err(); err != nil {
			return nil, false, application.NewError(application.ErrorTimeout, "delete_manifest_gc_canceled", err)
		}
		shard := fmt.Sprintf("%02x", (startShard+offset)&0xff)
		shardDir, err := root.Open(filepath.Join("manifests", shard))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, false, application.NewError(application.ErrorStorageUnavailable, "delete_manifest_gc_shard", err)
		}
		for {
			entries, readErr := shardDir.ReadDir(32)
			for _, entry := range entries {
				if err := ctx.Err(); err != nil {
					_ = shardDir.Close()
					return nil, false, application.NewError(application.ErrorTimeout, "delete_manifest_gc_canceled", err)
				}
				key := strings.TrimSuffix(entry.Name(), ".json")
				if entry.IsDir() || !validLookupKey(key) {
					continue
				}
				manifest, found, err := readManifest(root, manifestName(key))
				if err != nil {
					_ = shardDir.Close()
					return nil, false, err
				}
				if found && manifest.Stage == application.DeleteStageTerminal && manifest.TerminalAt.Add(deleteManifestRetention).Before(before) {
					ranked = insertRankedManifestKey(ranked, key, cursor, limit+1)
				}
			}
			if errors.Is(readErr, io.EOF) {
				break
			}
			if readErr != nil {
				_ = shardDir.Close()
				return nil, false, application.NewError(application.ErrorStorageUnavailable, "delete_manifest_gc_list", readErr)
			}
		}
		if err := shardDir.Close(); err != nil {
			return nil, false, application.NewError(application.ErrorStorageUnavailable, "delete_manifest_gc_list", err)
		}
	}
	truncated := len(ranked) > limit
	if truncated {
		ranked = ranked[:limit]
	}
	keys := make([]string, len(ranked))
	for index := range ranked {
		keys[index] = ranked[index].key
	}
	return keys, truncated, nil
}

func insertRankedManifestKey(ranked []rankedManifestKey, key string, cursor uint64, capacity int) []rankedManifestKey {
	prefix, _ := strconv.ParseUint(key[:16], 16, 64)
	ranked = append(ranked, rankedManifestKey{key: key, rank: prefix - cursor})
	sort.Slice(ranked, func(left, right int) bool {
		if ranked[left].rank == ranked[right].rank {
			return ranked[left].key < ranked[right].key
		}
		return ranked[left].rank < ranked[right].rank
	})
	if len(ranked) > capacity {
		ranked = ranked[:capacity]
	}
	return ranked
}

var _ application.DeleteManifestReader = (*DeleteManifestStore)(nil)
