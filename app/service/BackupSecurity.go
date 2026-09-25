package service

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

type ExecutableDescriptor struct {
	Path     string
	SHA256   string
	PolicyID string
}

func BuildExecutableDescriptor(path string, allowlist []string, policyID string) (ExecutableDescriptor, error) {
	canonical, err := ValidateExecutable(path, allowlist)
	if err != nil {
		return ExecutableDescriptor{}, err
	}
	file, err := os.Open(canonical)
	if err != nil {
		return ExecutableDescriptor{}, fmt.Errorf("%w: open executable: %v", ErrBackupValidation, err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return ExecutableDescriptor{}, fmt.Errorf("%w: hash executable: %v", ErrBackupValidation, err)
	}
	if strings.TrimSpace(policyID) == "" {
		return ExecutableDescriptor{}, fmt.Errorf("%w: executable policy is required", ErrBackupValidation)
	}
	return ExecutableDescriptor{Path: canonical, SHA256: hex.EncodeToString(hash.Sum(nil)), PolicyID: policyID}, nil
}

var (
	ErrBackupValidation = errors.New("invalid backup request")
	ErrBackupPath       = errors.New("backup path is outside the configured root")
	backupIDPattern     = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
)

// BackupLimits are deliberately finite so archive creation cannot become an
// unbounded memory, disk or CPU operation.
type BackupLimits struct {
	MaxCopies       int
	MaxBytes        int64
	MaxFiles        int
	MaxSourceBytes  int64
	MaxArchiveBytes int64
}

var DefaultBackupLimits = BackupLimits{
	MaxCopies: 30, MaxBytes: 20 << 30, MaxFiles: 100000,
	MaxSourceBytes: 10 << 30, MaxArchiveBytes: 4 << 30,
}

// ConfiguredDatabaseIdentity excludes credentials while binding metadata to
// the database that produced it.
type ConfiguredDatabaseIdentity struct {
	Scheme       string
	ClusterHost  string
	ClusterPort  string
	DatabaseName string
	AuthSource   string
	TLSMode      string
}

func (identity ConfiguredDatabaseIdentity) Canonical() (ConfiguredDatabaseIdentity, error) {
	identity.Scheme = strings.ToLower(strings.TrimSpace(identity.Scheme))
	identity.ClusterHost = strings.ToLower(strings.TrimSpace(identity.ClusterHost))
	identity.ClusterPort = strings.TrimSpace(identity.ClusterPort)
	identity.DatabaseName = strings.TrimSpace(identity.DatabaseName)
	identity.AuthSource = strings.TrimSpace(identity.AuthSource)
	identity.TLSMode = strings.ToLower(strings.TrimSpace(identity.TLSMode))
	if identity.Scheme != "mongodb" && identity.Scheme != "mongodb+srv" {
		return ConfiguredDatabaseIdentity{}, fmt.Errorf("%w: unsupported database scheme", ErrBackupValidation)
	}
	if identity.ClusterHost == "" || identity.DatabaseName == "" || strings.ContainsAny(identity.DatabaseName, "\\/\x00") {
		return ConfiguredDatabaseIdentity{}, fmt.Errorf("%w: database host/name is invalid", ErrBackupValidation)
	}
	if identity.Scheme == "mongodb" && identity.ClusterPort == "" {
		return ConfiguredDatabaseIdentity{}, fmt.Errorf("%w: database port is required", ErrBackupValidation)
	}
	if identity.TLSMode == "" {
		identity.TLSMode = "disabled"
	}
	return identity, nil
}

func (identity ConfiguredDatabaseIdentity) Digest() (string, error) {
	canonical, err := identity.Canonical()
	if err != nil {
		return "", err
	}
	values := []string{canonical.Scheme, canonical.ClusterHost, canonical.ClusterPort, canonical.DatabaseName, canonical.AuthSource, canonical.TLSMode}
	hash := sha256.New()
	var length [4]byte
	for _, value := range values {
		if len(value) > int(^uint32(0)) {
			return "", fmt.Errorf("%w: identity field is too long", ErrBackupValidation)
		}
		binary.BigEndian.PutUint32(length[:], uint32(len(value)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write([]byte(value))
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// CredentialProviderRef is an opaque handoff to the interface/infrastructure
// owner. It is safe to persist because it contains no credential material.
type CredentialProviderRef struct {
	ProviderKind string
	OpaqueHandle string
	Owner        string
}

func (ref CredentialProviderRef) Validate() error {
	if strings.TrimSpace(ref.ProviderKind) == "" || strings.TrimSpace(ref.OpaqueHandle) == "" || strings.TrimSpace(ref.Owner) == "" {
		return fmt.Errorf("%w: credential provider reference is incomplete", ErrBackupValidation)
	}
	return nil
}

func ValidateContainedPath(root, candidate string) (string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("%w: resolve backup root: %v", ErrBackupPath, err)
	}
	rootAbs, err = filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return "", fmt.Errorf("%w: resolve backup root: %v", ErrBackupPath, err)
	}
	if rootInfo, statErr := os.Stat(rootAbs); statErr != nil || !rootInfo.IsDir() {
		return "", fmt.Errorf("%w: backup root is unavailable", ErrBackupPath)
	}
	candidateAbs, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("%w: resolve candidate: %v", ErrBackupPath, err)
	}
	cleanCandidate := filepath.Clean(candidateAbs)
	relative, err := filepath.Rel(rootAbs, cleanCandidate)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", ErrBackupPath
	}
	if info, statErr := os.Lstat(cleanCandidate); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("%w: symlink is not allowed", ErrBackupPath)
	}
	parent := filepath.Dir(cleanCandidate)
	for parent != rootAbs {
		if info, statErr := os.Lstat(parent); statErr != nil || info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("%w: symlinked parent is not allowed", ErrBackupPath)
		}
		parent = filepath.Dir(parent)
	}
	return cleanCandidate, nil
}

func ValidateOpaqueBackupID(value string) error {
	if !backupIDPattern.MatchString(value) {
		return fmt.Errorf("%w: backup identity is invalid", ErrBackupValidation)
	}
	return nil
}

func ValidateExecutable(path string, allowlist []string) (string, error) {
	if !filepath.IsAbs(path) || strings.ContainsAny(path, "\x00\r\n;&|$`") {
		return "", fmt.Errorf("%w: executable path must be an absolute regular file", ErrBackupValidation)
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("%w: resolve executable: %v", ErrBackupValidation, err)
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return "", fmt.Errorf("%w: stat executable: %v", ErrBackupValidation, err)
	}
	executableBit := info.Mode()&0111 != 0
	if runtime.GOOS == "windows" {
		extension := strings.ToLower(filepath.Ext(canonical))
		executableBit = extension == ".exe" || extension == ".cmd" || extension == ".bat"
	}
	if !info.Mode().IsRegular() || !executableBit {
		return "", fmt.Errorf("%w: executable is not an executable regular file", ErrBackupValidation)
	}
	if len(allowlist) == 0 {
		return "", fmt.Errorf("%w: executable allowlist is empty", ErrBackupValidation)
	}
	allowed := false
	for _, item := range allowlist {
		candidate, candidateErr := filepath.EvalSymlinks(item)
		if candidateErr == nil && samePath(canonical, candidate) {
			allowed = true
			break
		}
	}
	if !allowed {
		return "", fmt.Errorf("%w: executable is not in the allowlist", ErrBackupValidation)
	}
	return canonical, nil
}

// NormalizeExecutableAllowlist validates and canonicalizes every configured
// tool path before it is persisted. Empty lists are rejected so runtime
// execution cannot silently fall back to an unbounded path policy.
func NormalizeExecutableAllowlist(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("%w: executable allowlist is empty", ErrBackupValidation)
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		canonical, err := ValidateExecutable(value, []string{value})
		if err != nil {
			return nil, err
		}
		key := canonical
		if os.PathSeparator == '\\' {
			key = strings.ToLower(key)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, canonical)
	}
	return result, nil
}

func samePath(a, b string) bool {
	if os.PathSeparator == '\\' {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func BuildMongoDumpArgs(executable, host, port, database, output string, username string) []string {
	args := []string{"--host", host, "--port", port, "--db", database, "--out", output}
	_ = username // credentials are supplied through stdin by the caller
	return append([]string{executable}, args...)
}

func BuildMongoRestoreArgs(executable, host, port, database, input string, username string) []string {
	args := []string{"--host", host, "--port", port, "--db", database, "--drop", input}
	_ = username // credentials are supplied through stdin by the caller
	return append([]string{executable}, args...)
}

// StableBackupPaths returns deterministic, safe relative file names for an
// archive. It rejects symlinks and traversal before any archive is created.
func StableBackupPaths(root string, limits BackupLimits) ([]string, int64, error) {
	if limits.MaxFiles <= 0 || limits.MaxSourceBytes <= 0 {
		return nil, 0, fmt.Errorf("%w: invalid archive budget", ErrBackupValidation)
	}
	rootInfo, err := os.Stat(root)
	if err != nil || !rootInfo.IsDir() {
		return nil, 0, fmt.Errorf("%w: backup directory is unavailable", ErrBackupPath)
	}
	paths := make([]string, 0)
	var total int64
	err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: symlink in backup tree", ErrBackupPath)
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%w: non-regular file in backup tree", ErrBackupPath)
		}
		if len(paths) >= limits.MaxFiles || info.Size() > limits.MaxSourceBytes-total {
			return fmt.Errorf("%w: archive source budget exceeded", ErrBackupValidation)
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return ErrBackupPath
		}
		paths = append(paths, filepath.ToSlash(relative))
		total += info.Size()
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	sort.Strings(paths)
	return paths, total, nil
}
