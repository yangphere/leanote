package contentfs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"sync"

	application "github.com/yangphere/leanote/app/application/content"
)

func (store *FileStore) CreateTemporary(ctx context.Context, prefix string) (application.TemporaryArtifact, error) {
	if err := ctx.Err(); err != nil {
		return nil, application.NewError(application.ErrorTimeout, "temporary_create_cancelled", err)
	}
	if prefix == "" || strings.ContainsAny(prefix, `/\\`) {
		return nil, application.NewError(application.ErrorValidation, "temporary_prefix_invalid", nil)
	}
	root := store.handles[application.RootTemporary]
	if root == nil {
		return nil, application.NewError(application.ErrorStorageUnavailable, "temporary_root_unavailable", nil)
	}
	for attempt := 0; attempt < 32; attempt++ {
		random := make([]byte, 16)
		if _, err := rand.Read(random); err != nil {
			return nil, application.NewError(application.ErrorStorageUnavailable, "temporary_random", err)
		}
		name := prefix + hex.EncodeToString(random)
		file, err := root.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			return &temporaryArtifact{file: file, root: root, name: name}, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, application.NewError(application.ErrorStorageUnavailable, "temporary_create", err)
		}
	}
	return nil, application.NewError(application.ErrorStorageUnavailable, "temporary_namespace_exhausted", nil)
}

type temporaryArtifact struct {
	mu       sync.Mutex
	file     *os.File
	root     *os.Root
	name     string
	finished bool
}

func (artifact *temporaryArtifact) Write(buffer []byte) (int, error) {
	artifact.mu.Lock()
	defer artifact.mu.Unlock()
	if artifact.finished {
		return 0, fs.ErrClosed
	}
	return artifact.file.Write(buffer)
}

func (artifact *temporaryArtifact) Seal(ctx context.Context) (io.ReadCloser, int64, error) {
	artifact.mu.Lock()
	defer artifact.mu.Unlock()
	if artifact.finished {
		return nil, 0, application.NewError(application.ErrorConflict, "temporary_already_finished", nil)
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, errors.Join(
			application.NewError(application.ErrorTimeout, "temporary_seal_cancelled", err),
			artifact.abortLocked(),
		)
	}
	if err := artifact.file.Sync(); err != nil {
		return nil, 0, errors.Join(
			application.NewError(application.ErrorStorageUnavailable, "temporary_sync", err),
			artifact.abortLocked(),
		)
	}
	if err := artifact.file.Close(); err != nil {
		artifact.finished = true
		return nil, 0, errors.Join(
			application.NewError(application.ErrorStorageUnavailable, "temporary_writer_close", err),
			artifact.removeLocked(),
		)
	}
	artifact.finished = true

	reader, err := artifact.root.Open(artifact.name)
	if err != nil {
		return nil, 0, errors.Join(
			application.NewError(application.ErrorStorageUnavailable, "temporary_reopen", err),
			artifact.removeLocked(),
		)
	}
	info, statErr := reader.Stat()
	if statErr != nil || !info.Mode().IsRegular() {
		return nil, 0, errors.Join(
			application.NewError(application.ErrorStorageUnavailable, "temporary_stat", errors.Join(statErr, nonRegularTemporaryError(info))),
			reader.Close(),
			artifact.removeLocked(),
		)
	}
	return &temporaryReader{file: reader, root: artifact.root, name: artifact.name}, info.Size(), nil
}

func (artifact *temporaryArtifact) Abort() error {
	artifact.mu.Lock()
	defer artifact.mu.Unlock()
	return artifact.abortLocked()
}

func (artifact *temporaryArtifact) abortLocked() error {
	if artifact.finished {
		return nil
	}
	artifact.finished = true
	return errors.Join(artifact.file.Close(), artifact.removeLocked())
}

func (artifact *temporaryArtifact) removeLocked() error {
	err := artifact.root.Remove(artifact.name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func nonRegularTemporaryError(info fs.FileInfo) error {
	if info == nil || info.Mode().IsRegular() {
		return nil
	}
	return fmt.Errorf("temporary artifact is not a regular file")
}

type temporaryReader struct {
	mu      sync.Mutex
	file    *os.File
	root    *os.Root
	name    string
	removed bool
}

func (reader *temporaryReader) Read(buffer []byte) (int, error) {
	return reader.file.Read(buffer)
}

func (reader *temporaryReader) Seek(offset int64, whence int) (int64, error) {
	return reader.file.Seek(offset, whence)
}

func (reader *temporaryReader) Close() error {
	reader.mu.Lock()
	defer reader.mu.Unlock()
	if reader.removed {
		return nil
	}
	closeErr := reader.file.Close()
	removeErr := reader.root.Remove(reader.name)
	if removeErr == nil || errors.Is(removeErr, fs.ErrNotExist) {
		reader.removed = true
		removeErr = nil
	}
	return errors.Join(closeErr, removeErr)
}

var _ application.TemporaryArtifact = (*temporaryArtifact)(nil)
