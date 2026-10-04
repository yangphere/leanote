package contentfs

import (
	"context"
	"crypto/sha256"
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
	"time"

	application "github.com/yangphere/leanote/app/application/content"
)

func (store *DeleteManifestStore) CreateCreateManifest(ctx context.Context, manifest application.CreateManifest) error {
	if err := ctx.Err(); err != nil {
		return application.NewError(application.ErrorTimeout, "create_manifest_canceled", err)
	}
	if err := manifest.Validate(); err != nil {
		return err
	}
	root, name, err := store.createLocation(manifest.Root, manifest.LookupKey)
	if err != nil {
		return err
	}
	return store.withLease(root, manifest.LookupKey, func() error {
		if err := ensureManifestParent(root, filepath.Dir(name), store.directorySync); err != nil {
			return err
		}
		return store.writeCreateManifestNoReplace(root, name, manifest)
	})
}

func (store *DeleteManifestStore) LoadCreateManifest(ctx context.Context, lookupKey string) (application.CreateManifest, bool, error) {
	if err := ctx.Err(); err != nil {
		return application.CreateManifest{}, false, application.NewError(application.ErrorTimeout, "create_manifest_canceled", err)
	}
	if !validLookupKey(lookupKey) {
		return application.CreateManifest{}, false, application.NewError(application.ErrorValidation, "create_manifest_lookup", nil)
	}
	var result application.CreateManifest
	found := false
	for kind, root := range store.handles {
		manifest, exists, err := readCreateManifest(root, createManifestName(lookupKey))
		if err != nil {
			return application.CreateManifest{}, false, err
		}
		if !exists {
			continue
		}
		if err := store.publicationBarrier(root, createManifestName(lookupKey)); err != nil {
			return application.CreateManifest{}, false, application.NewError(application.ErrorUnknownResult, "create_manifest_load_sync", err)
		}
		if manifest.Root != kind || found {
			return application.CreateManifest{}, false, application.NewError(application.ErrorConflict, "create_manifest_lookup_ambiguous", nil)
		}
		result, found = manifest, true
	}
	return result, found, nil
}

func (store *DeleteManifestStore) CompareAndSwapCreateManifest(ctx context.Context, lookupKey string, version uint64, digest [sha256.Size]byte, next application.CreateManifest) error {
	if err := ctx.Err(); err != nil {
		return application.NewError(application.ErrorTimeout, "create_manifest_canceled", err)
	}
	if !validLookupKey(lookupKey) || next.LookupKey != lookupKey || next.Version != version+1 || next.StateDigest == digest {
		return application.NewError(application.ErrorValidation, "create_manifest_cas_input", nil)
	}
	if err := next.Validate(); err != nil {
		return err
	}
	root, name, err := store.createLocation(next.Root, lookupKey)
	if err != nil {
		return err
	}
	return store.withLease(root, lookupKey, func() error {
		current, found, err := readCreateManifest(root, name)
		if err != nil {
			return err
		}
		if !found || current.Version != version || current.StateDigest != digest || current.InputDigest != next.InputDigest {
			return application.NewError(application.ErrorConflict, "create_manifest_stale", nil)
		}
		return store.replaceCreateManifest(root, name, next)
	})
}

func (store *DeleteManifestStore) ListActiveCreateManifests(ctx context.Context, limit int) ([]application.CreateManifest, bool, error) {
	return store.listActiveCreateManifests(ctx, limit, nil)
}

// ListActiveNonPreNoteCreateManifests is the generic create-repair scan. It
// excludes pre-note assets because their parent-note decision is owned by the
// API-specific recovery flow and has a separate bounded budget.
func (store *DeleteManifestStore) ListActiveNonPreNoteCreateManifests(ctx context.Context, limit int) ([]application.CreateManifest, bool, error) {
	return store.listActiveCreateManifests(ctx, limit, func(manifest application.CreateManifest) bool {
		return !manifest.PreNote
	})
}

// ListActivePreNoteCreateManifests returns only active pre-note manifests for
// one owner action, so unrelated active creates cannot starve its recovery.
func (store *DeleteManifestStore) ListActivePreNoteCreateManifests(ctx context.Context, action string, limit int) ([]application.CreateManifest, bool, error) {
	if strings.TrimSpace(action) == "" {
		return nil, false, application.NewError(application.ErrorValidation, "create_manifest_pre_note_action", nil)
	}
	return store.listActiveCreateManifests(ctx, limit, func(manifest application.CreateManifest) bool {
		return manifest.PreNote && manifest.Action == action
	})
}

func (store *DeleteManifestStore) listActiveCreateManifests(ctx context.Context, limit int, include func(application.CreateManifest) bool) ([]application.CreateManifest, bool, error) {
	if limit <= 0 {
		return nil, false, application.NewError(application.ErrorValidation, "create_manifest_list_limit", nil)
	}
	result := make([]application.CreateManifest, 0, limit)
	for _, root := range store.handles {
		remaining := limit - len(result)
		if remaining == 0 {
			more, _, err := createManifestKeys(ctx, root, store.createScan.Add(0x9e3779b97f4a7c15), 1, false, time.Time{}, include)
			if err != nil {
				return result, false, err
			}
			if len(more) > 0 {
				return result, true, nil
			}
			continue
		}
		manifests, truncated, err := createManifestKeys(ctx, root, store.createScan.Add(0x9e3779b97f4a7c15), remaining, false, time.Time{}, include)
		if err != nil {
			return result, false, err
		}
		for _, manifest := range manifests {
			if err := store.publicationBarrier(root, createManifestName(manifest.LookupKey)); err != nil {
				return result, false, application.NewError(application.ErrorUnknownResult, "create_manifest_list_sync", err)
			}
		}
		result = append(result, manifests...)
		if truncated {
			return result, true, nil
		}
	}
	return result, false, nil
}

func (store *DeleteManifestStore) GCCreateTerminalsBounded(ctx context.Context, before time.Time, limit int) (TerminalGCResult, error) {
	if before.IsZero() || limit <= 0 {
		return TerminalGCResult{}, application.NewError(application.ErrorValidation, "create_manifest_gc_input", nil)
	}
	result := TerminalGCResult{Removed: []string{}}
	for _, root := range store.handles {
		remaining := limit - result.Scanned
		if remaining == 0 {
			result.Truncated = true
			break
		}
		manifests, truncated, err := createManifestKeys(ctx, root, store.createScan.Add(0x9e3779b97f4a7c15), remaining, true, before, nil)
		if err != nil {
			return result, err
		}
		result.Scanned += len(manifests)
		result.Truncated = result.Truncated || truncated
		for _, scanned := range manifests {
			key := scanned.LookupKey
			if err := store.withLease(root, key, func() error {
				manifest, found, err := readCreateManifest(root, createManifestName(key))
				if err != nil || !found || manifest.Stage != application.CreateStageTerminal || !manifest.TerminalAt.Add(deleteManifestRetention).Before(before) {
					return err
				}
				if err := ctx.Err(); err != nil {
					return application.NewError(application.ErrorTimeout, "create_manifest_gc_canceled", err)
				}
				if err := root.Remove(createManifestName(key)); err != nil {
					return application.NewError(application.ErrorStorageUnavailable, "create_manifest_gc_remove", err)
				}
				if err := store.removalBarrier(root, createManifestName(key)); err != nil {
					return application.NewError(application.ErrorUnknownResult, "create_manifest_gc_sync", err)
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

func (store *DeleteManifestStore) createLocation(kind application.RootKind, lookupKey string) (*os.Root, string, error) {
	root := store.handles[kind]
	if root == nil {
		return nil, "", application.NewError(application.ErrorStorageUnavailable, "create_manifest_root_unavailable", nil)
	}
	if !validLookupKey(lookupKey) {
		return nil, "", application.NewError(application.ErrorValidation, "create_manifest_lookup", nil)
	}
	return root, createManifestName(lookupKey), nil
}

func createManifestName(lookupKey string) string {
	return filepath.Join("creates", lookupKey[:2], lookupKey+".json")
}

func (store *DeleteManifestStore) writeCreateManifestNoReplace(root *os.Root, name string, manifest application.CreateManifest) error {
	stage, stageName, err := writeCreateManifestStage(root, filepath.Dir(name), manifest)
	if err != nil {
		return err
	}
	defer func() { _ = root.Remove(stageName) }()
	if err := stage.Close(); err != nil {
		return application.NewError(application.ErrorStorageUnavailable, "create_manifest_close", err)
	}
	if err := root.Link(stageName, name); err != nil {
		if errors.Is(err, fs.ErrExist) {
			if syncErr := store.publicationBarrier(root, name); syncErr != nil {
				return application.NewError(application.ErrorUnknownResult, "create_manifest_publish_sync", syncErr)
			}
			return application.NewError(application.ErrorConflict, "create_manifest_exists", fs.ErrExist)
		}
		return application.NewError(application.ErrorUnsupportedFS, "create_manifest_no_replace", err)
	}
	if err := store.publicationBarrier(root, name); err != nil {
		return application.NewError(application.ErrorUnknownResult, "create_manifest_publish_sync", err)
	}
	return nil
}

func (store *DeleteManifestStore) replaceCreateManifest(root *os.Root, name string, manifest application.CreateManifest) error {
	stage, stageName, err := writeCreateManifestStage(root, filepath.Dir(name), manifest)
	if err != nil {
		return err
	}
	if err := stage.Close(); err != nil {
		_ = root.Remove(stageName)
		return application.NewError(application.ErrorStorageUnavailable, "create_manifest_close", err)
	}
	if err := root.Rename(stageName, name); err != nil {
		_ = root.Remove(stageName)
		return application.NewError(application.ErrorUnsupportedFS, "create_manifest_atomic_replace", err)
	}
	if err := store.publicationBarrier(root, name); err != nil {
		return application.NewError(application.ErrorUnknownResult, "create_manifest_replace_sync", err)
	}
	return nil
}

func writeCreateManifestStage(root *os.Root, parent string, manifest application.CreateManifest) (*os.File, string, error) {
	data, err := json.Marshal(manifest)
	if err != nil {
		return nil, "", application.NewError(application.ErrorStorageUnavailable, "create_manifest_encode", err)
	}
	stage, stageName, err := createUniqueStage(root, parent)
	if err != nil {
		return nil, "", application.NewError(application.ErrorStorageUnavailable, "create_manifest_stage", err)
	}
	if _, err := stage.Write(data); err != nil {
		_ = stage.Close()
		_ = root.Remove(stageName)
		return nil, "", application.NewError(application.ErrorStorageUnavailable, "create_manifest_write", err)
	}
	if err := errors.Join(stage.Chmod(0o600), stage.Sync()); err != nil {
		_ = stage.Close()
		_ = root.Remove(stageName)
		return nil, "", application.NewError(application.ErrorUnsupportedFS, "create_manifest_stage_sync", err)
	}
	return stage, stageName, nil
}

func readCreateManifest(root *os.Root, name string) (application.CreateManifest, bool, error) {
	info, err := root.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return application.CreateManifest{}, false, nil
	}
	if err != nil || isIndirection(info) || !manifestModeSafe(info) || info.Size() > maxDeleteManifestBytes {
		return application.CreateManifest{}, false, application.NewError(application.ErrorConflict, "create_manifest_unsafe", err)
	}
	file, err := root.Open(name)
	if err != nil {
		return application.CreateManifest{}, false, application.NewError(application.ErrorStorageUnavailable, "create_manifest_open", err)
	}
	decoder := json.NewDecoder(io.LimitReader(file, maxDeleteManifestBytes))
	var manifest application.CreateManifest
	decodeErr := decoder.Decode(&manifest)
	if decodeErr == nil {
		var trailing any
		if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
			if err == nil {
				err = errors.New("create manifest has trailing JSON value")
			}
			decodeErr = err
		}
	}
	closeErr := file.Close()
	if decodeErr != nil || closeErr != nil || manifest.Validate() != nil {
		return application.CreateManifest{}, false, application.NewError(application.ErrorConflict, "create_manifest_corrupt", errors.Join(decodeErr, closeErr, manifest.Validate()))
	}
	return manifest, true, nil
}

type rankedCreateManifest struct {
	manifest application.CreateManifest
	rank     uint64
}

func createManifestKeys(ctx context.Context, root *os.Root, cursor uint64, limit int, terminals bool, before time.Time, include func(application.CreateManifest) bool) ([]application.CreateManifest, bool, error) {
	if limit <= 0 {
		return nil, true, nil
	}
	ranked := make([]rankedCreateManifest, 0, limit+1)
	startShard := int(cursor >> 56)
	for offset := 0; offset < 256; offset++ {
		if err := ctx.Err(); err != nil {
			return nil, false, application.NewError(application.ErrorTimeout, "create_manifest_list_canceled", err)
		}
		shard := fmt.Sprintf("%02x", (startShard+offset)&0xff)
		shardDir, err := root.Open(filepath.Join("creates", shard))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, false, application.NewError(application.ErrorStorageUnavailable, "create_manifest_list_shard", err)
		}
		for {
			entries, readErr := shardDir.ReadDir(32)
			for _, entry := range entries {
				if err := ctx.Err(); err != nil {
					_ = shardDir.Close()
					return nil, false, application.NewError(application.ErrorTimeout, "create_manifest_list_canceled", err)
				}
				key := strings.TrimSuffix(entry.Name(), ".json")
				if entry.IsDir() || !validLookupKey(key) {
					continue
				}
				manifest, found, err := readCreateManifest(root, createManifestName(key))
				if err != nil {
					_ = shardDir.Close()
					return nil, false, err
				}
				matches := found && manifest.Stage != application.CreateStageTerminal
				if terminals {
					matches = found && manifest.Stage == application.CreateStageTerminal && manifest.TerminalAt.Add(deleteManifestRetention).Before(before)
				}
				if matches && (terminals || include == nil || include(manifest)) {
					ranked = insertRankedCreateManifest(ranked, manifest, cursor, limit+1)
				}
			}
			if errors.Is(readErr, io.EOF) {
				break
			}
			if readErr != nil {
				_ = shardDir.Close()
				return nil, false, application.NewError(application.ErrorStorageUnavailable, "create_manifest_list", readErr)
			}
		}
		if err := shardDir.Close(); err != nil {
			return nil, false, application.NewError(application.ErrorStorageUnavailable, "create_manifest_list", err)
		}
	}
	truncated := len(ranked) > limit
	if truncated {
		ranked = ranked[:limit]
	}
	manifests := make([]application.CreateManifest, len(ranked))
	for index := range ranked {
		manifests[index] = ranked[index].manifest
	}
	return manifests, truncated, nil
}

func insertRankedCreateManifest(ranked []rankedCreateManifest, manifest application.CreateManifest, cursor uint64, capacity int) []rankedCreateManifest {
	prefix, _ := strconv.ParseUint(manifest.LookupKey[:16], 16, 64)
	ranked = append(ranked, rankedCreateManifest{manifest: manifest, rank: prefix - cursor})
	sort.Slice(ranked, func(left, right int) bool {
		if ranked[left].rank == ranked[right].rank {
			return ranked[left].manifest.LookupKey < ranked[right].manifest.LookupKey
		}
		return ranked[left].rank < ranked[right].rank
	})
	if len(ranked) > capacity {
		ranked = ranked[:capacity]
	}
	return ranked
}

var _ application.CreateManifestStore = (*DeleteManifestStore)(nil)
