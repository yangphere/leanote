package content

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"strings"
	"unicode/utf8"
)

var legacyRootPrefixes = []struct {
	prefix string
	kind   RootKind
}{
	{prefix: "public/upload/", kind: RootPublicUpload},
	{prefix: "upload/", kind: RootPublicUpload},
	{prefix: "files/", kind: RootPrivateFiles},
}

// ParseLogicalPath validates a path that has already been assigned to a root.
// Backslashes are rejected so a persisted path has exactly one interpretation
// on Windows and Unix.
func ParseLogicalPath(kind RootKind, value string) (LogicalPath, error) {
	if !validRootKind(kind) {
		return LogicalPath{}, unsafePathError("unknown_root", nil)
	}
	if value == "" {
		return LogicalPath{}, unsafePathError("empty_path", nil)
	}
	if strings.ContainsRune(value, '\\') {
		return LogicalPath{}, unsafePathError("ambiguous_separator", nil)
	}
	if err := validatePortablePath(value); err != nil {
		return LogicalPath{}, err
	}
	return LogicalPath{Kind: kind, Value: value}, nil
}

// ParseStoredPath accepts only the historical database prefixes documented by
// the content contract.  It normalizes old Windows separators before passing
// the relative portion through the same logical-path validator.
func ParseStoredPath(value string) (LogicalPath, error) {
	if value == "" || strings.IndexByte(value, 0) >= 0 {
		return LogicalPath{}, unsafePathError("invalid_stored_path", nil)
	}
	if isAbsoluteOrVolumePath(value) {
		return LogicalPath{}, unsafePathError("absolute_path", nil)
	}
	normalized := strings.ReplaceAll(value, "\\", "/")
	for _, candidate := range legacyRootPrefixes {
		if strings.HasPrefix(normalized, candidate.prefix) {
			return ParseLogicalPath(candidate.kind, strings.TrimPrefix(normalized, candidate.prefix))
		}
	}
	return LogicalPath{}, unsafePathError("unknown_prefix", nil)
}

// StableDestination derives an opaque, deterministic path without placing an
// operation ID, owner ID, or source ID in the filename.
func StableDestination(identity AssetIdentity, kind RootKind, namespace, extension string) (LogicalPath, error) {
	if identity.OperationID == "" || identity.Generation < 0 || identity.OwnerID.IsZero() || identity.Kind == "" || identity.SourceID == "" {
		return LogicalPath{}, validationError("invalid_asset_identity", nil)
	}
	if namespace == "" || strings.ContainsAny(namespace, "/\\") {
		return LogicalPath{}, validationError("invalid_namespace", nil)
	}
	if extension != "" {
		if !strings.HasPrefix(extension, ".") || len(extension) > 16 || strings.ContainsAny(extension[1:], ". /\\") {
			return LogicalPath{}, validationError("invalid_extension", nil)
		}
		extension = strings.ToLower(extension)
	}
	input := strings.Join([]string{
		identity.OperationID,
		fmt.Sprintf("%d", identity.Generation),
		identity.OwnerID.Hex(),
		string(identity.Kind),
		identity.SourceID,
		identity.DestinationID,
		hex.EncodeToString(identity.Digest[:]),
	}, "\x00")
	digest := sha256.Sum256([]byte(input))
	name := hex.EncodeToString(digest[:]) + extension
	return ParseLogicalPath(kind, path.Join(namespace, name[:2], name[2:4], name))
}

func validRootKind(kind RootKind) bool {
	switch kind {
	case RootPrivateFiles, RootPublicUpload, RootTemporary:
		return true
	default:
		return false
	}
}

func validatePortablePath(value string) error {
	if !utf8.ValidString(value) {
		return unsafePathError("invalid_utf8", nil)
	}
	if isAbsoluteOrVolumePath(value) {
		return unsafePathError("absolute_path", nil)
	}
	if strings.HasSuffix(value, "/") || strings.Contains(value, "//") {
		return unsafePathError("empty_segment", nil)
	}
	for _, segment := range strings.Split(value, "/") {
		if err := validatePortableSegment(segment); err != nil {
			return err
		}
	}
	if cleaned := path.Clean(value); cleaned != value || cleaned == "." || strings.HasPrefix(cleaned, "../") {
		return unsafePathError("unclean_path", nil)
	}
	return nil
}

func validatePortableSegment(segment string) error {
	if segment == "" || segment == "." || segment == ".." {
		return unsafePathError("dot_or_empty_segment", nil)
	}
	if strings.HasSuffix(segment, " ") || strings.HasSuffix(segment, ".") {
		return unsafePathError("ambiguous_segment", nil)
	}
	for _, r := range segment {
		if r == 0 || r < 0x20 || r == 0x7f || r == ':' {
			return unsafePathError("control_or_volume_character", nil)
		}
	}
	base := strings.ToUpper(segment)
	if dot := strings.IndexByte(base, '.'); dot >= 0 {
		base = base[:dot]
	}
	if isReservedDeviceName(base) {
		return unsafePathError("reserved_device_name", nil)
	}
	return nil
}

func isAbsoluteOrVolumePath(value string) bool {
	if strings.HasPrefix(value, "/") || strings.HasPrefix(value, "\\") {
		return true
	}
	if len(value) >= 2 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) && value[1] == ':' {
		return true
	}
	lower := strings.ToLower(value)
	return strings.HasPrefix(lower, `\\?\`) || strings.HasPrefix(lower, `\\.\`) || strings.HasPrefix(lower, `\??\`)
}

func isReservedDeviceName(value string) bool {
	switch value {
	case "CON", "PRN", "AUX", "NUL", "CLOCK$":
		return true
	}
	if len(value) == 4 {
		prefix, digit := value[:3], value[3]
		return (prefix == "COM" || prefix == "LPT") && digit >= '1' && digit <= '9'
	}
	return false
}
