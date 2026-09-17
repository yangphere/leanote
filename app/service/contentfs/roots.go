// Package contentfs adapts the content application boundary to a local
// filesystem.  Absolute paths remain private to this package.
package contentfs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	application "github.com/yangphere/leanote/app/application/content"
)

type DurableRootConfig struct {
	Data       string
	Quarantine string
}

type ContentRootsConfig struct {
	PrivateFiles DurableRootConfig
	PublicUpload DurableRootConfig
	Temporary    string
	// ServedRoots lists static/document roots reachable through HTTP.  A
	// quarantine root may neither be inside nor contain one of these roots.
	ServedRoots []string
}

type durableRoot struct {
	data       string
	quarantine string
}

// ContentRoots is a validated adapter configuration.  Its absolute paths are
// intentionally unexported so application and controller code cannot bypass
// logical-path resolution.
type ContentRoots struct {
	privateFiles durableRoot
	publicUpload durableRoot
	temporary    string
}

// CanonicalTemporaryPath returns the validated adapter path used to wire
// process backends that cannot consume the opaque TemporaryStore capability.
// It must not be exposed through application or controller contracts.
func (roots *ContentRoots) CanonicalTemporaryPath() (string, error) {
	return roots.dataPath(application.RootTemporary)
}

func ValidateContentRoots(config ContentRootsConfig) (*ContentRoots, error) {
	privateData, err := canonicalDirectory("private_files.data", config.PrivateFiles.Data)
	if err != nil {
		return nil, err
	}
	privateQuarantine, err := canonicalDirectory("private_files.quarantine", config.PrivateFiles.Quarantine)
	if err != nil {
		return nil, err
	}
	publicData, err := canonicalDirectory("public_upload.data", config.PublicUpload.Data)
	if err != nil {
		return nil, err
	}
	publicQuarantine, err := canonicalDirectory("public_upload.quarantine", config.PublicUpload.Quarantine)
	if err != nil {
		return nil, err
	}
	temporary, err := canonicalDirectory("temporary", config.Temporary)
	if err != nil {
		return nil, err
	}

	contentPaths := []namedPath{
		{name: "private_files.data", value: privateData},
		{name: "private_files.quarantine", value: privateQuarantine},
		{name: "public_upload.data", value: publicData},
		{name: "public_upload.quarantine", value: publicQuarantine},
		{name: "temporary", value: temporary},
	}
	if err := rejectOverlaps(contentPaths); err != nil {
		return nil, err
	}

	for index, configured := range config.ServedRoots {
		served, err := canonicalDirectory(fmt.Sprintf("served_roots[%d]", index), configured)
		if err != nil {
			return nil, err
		}
		for _, quarantine := range []namedPath{
			{name: "private_files.quarantine", value: privateQuarantine},
			{name: "public_upload.quarantine", value: publicQuarantine},
		} {
			if pathsOverlap(quarantine.value, served) {
				return nil, fmt.Errorf("content roots: %s overlaps a served root", quarantine.name)
			}
		}
	}

	if err := verifyAtomicRename(privateData, privateQuarantine); err != nil {
		return nil, fmt.Errorf("content roots: private files pair is not on one filesystem: %w", err)
	}
	if err := verifyAtomicRename(publicData, publicQuarantine); err != nil {
		return nil, fmt.Errorf("content roots: public upload pair is not on one filesystem: %w", err)
	}
	if err := verifyWritableDirectory(temporary); err != nil {
		return nil, fmt.Errorf("content roots: temporary root is not writable: %w", err)
	}

	return &ContentRoots{
		privateFiles: durableRoot{data: privateData, quarantine: privateQuarantine},
		publicUpload: durableRoot{data: publicData, quarantine: publicQuarantine},
		temporary:    temporary,
	}, nil
}

func (roots *ContentRoots) dataPath(kind application.RootKind) (string, error) {
	if roots == nil {
		return "", fmt.Errorf("content roots are not configured")
	}
	switch kind {
	case application.RootPrivateFiles:
		return roots.privateFiles.data, nil
	case application.RootPublicUpload:
		return roots.publicUpload.data, nil
	case application.RootTemporary:
		return roots.temporary, nil
	default:
		return "", fmt.Errorf("unknown content root %q", kind)
	}
}

func (roots *ContentRoots) quarantinePath(kind application.RootKind) (string, error) {
	if roots == nil {
		return "", fmt.Errorf("content roots are not configured")
	}
	switch kind {
	case application.RootPrivateFiles:
		return roots.privateFiles.quarantine, nil
	case application.RootPublicUpload:
		return roots.publicUpload.quarantine, nil
	default:
		return "", fmt.Errorf("content root %q has no durable quarantine", kind)
	}
}

type namedPath struct {
	name  string
	value string
}

func canonicalDirectory(name, value string) (string, error) {
	if value == "" || !filepath.IsAbs(value) {
		return "", fmt.Errorf("content roots: %s must be an absolute path", name)
	}
	info, err := os.Stat(value)
	if err != nil {
		return "", fmt.Errorf("content roots: %s is inaccessible: %w", name, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("content roots: %s is not a directory", name)
	}
	canonical, err := filepath.EvalSymlinks(value)
	if err != nil {
		return "", fmt.Errorf("content roots: resolve %s: %w", name, err)
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return "", fmt.Errorf("content roots: canonicalize %s: %w", name, err)
	}
	return filepath.Clean(canonical), nil
}

func rejectOverlaps(paths []namedPath) error {
	for left := 0; left < len(paths); left++ {
		for right := left + 1; right < len(paths); right++ {
			if pathsOverlap(paths[left].value, paths[right].value) {
				return fmt.Errorf("content roots: %s overlaps %s", paths[left].name, paths[right].name)
			}
		}
	}
	return nil
}

func pathsOverlap(left, right string) bool {
	if runtime.GOOS == "windows" {
		left = strings.ToLower(left)
		right = strings.ToLower(right)
	}
	return containsPath(left, right) || containsPath(right, left)
}

func containsPath(parent, child string) bool {
	relative, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

var rootProbePayload = []byte("leanote-content-root-probe")

func verifyAtomicRename(dataRoot, quarantineRoot string) (err error) {
	probe, err := os.CreateTemp(dataRoot, ".leanote-content-volume-probe-")
	if err != nil {
		return err
	}
	source := probe.Name()
	destination := filepath.Join(quarantineRoot, filepath.Base(source))
	probeOpen := true
	defer func() {
		if probeOpen {
			err = errors.Join(err, probe.Close())
		}
		err = errors.Join(err, cleanupRootProbe(source, destination, os.Remove))
	}()
	if err := probe.Chmod(0o600); err != nil {
		return err
	}
	if _, err := probe.Write(rootProbePayload); err != nil {
		return err
	}
	if err := probe.Sync(); err != nil {
		return err
	}
	closeErr := probe.Close()
	probeOpen = false
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(source, destination); err != nil {
		return err
	}
	if err := verifyRootProbeContents(destination); err != nil {
		return err
	}
	if err := os.Remove(destination); err != nil {
		return err
	}
	return nil
}

func verifyWritableDirectory(root string) (err error) {
	probe, err := os.CreateTemp(root, ".leanote-content-write-probe-")
	if err != nil {
		return err
	}
	name := probe.Name()
	probeOpen := true
	defer func() {
		if probeOpen {
			err = errors.Join(err, probe.Close())
		}
		err = errors.Join(err, cleanupRootProbe(name, "", os.Remove))
	}()
	if err := probe.Chmod(0o600); err != nil {
		return err
	}
	if _, err := probe.Write(rootProbePayload); err != nil {
		return err
	}
	if err := probe.Sync(); err != nil {
		return err
	}
	closeErr := probe.Close()
	probeOpen = false
	if closeErr != nil {
		return closeErr
	}
	if err := verifyRootProbeContents(name); err != nil {
		return err
	}
	return os.Remove(name)
}

func verifyRootProbeContents(name string) error {
	file, err := os.Open(name)
	if err != nil {
		return err
	}
	data, readErr := io.ReadAll(io.LimitReader(file, int64(len(rootProbePayload)+1)))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil {
		return errors.Join(readErr, closeErr)
	}
	if !bytes.Equal(data, rootProbePayload) {
		return fmt.Errorf("content root probe read-back mismatch")
	}
	return nil
}

func cleanupRootProbe(source, destination string, remove func(string) error) error {
	var result error
	for _, name := range []string{source, destination} {
		if name == "" {
			continue
		}
		if err := remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
			result = errors.Join(result, fmt.Errorf("remove content root probe: %w", err))
		}
	}
	return result
}
