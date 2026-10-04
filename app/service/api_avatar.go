package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	applicationcontent "github.com/yangphere/leanote/app/application/content"
	"github.com/yangphere/leanote/app/domain"
	. "github.com/yangphere/leanote/app/lea"
)

const apiAvatarMaxBytes int64 = 5 * 1024 * 1024

// APIAvatarPublication describes the wire-compatible public path produced by
// the API avatar adapter. The bytes are published through the configured
// content store and no Files row is created.
type APIAvatarPublication struct {
	Path    string
	Logical applicationcontent.LogicalPath
	Digest  [sha256.Size]byte
	Size    int64
}

// PublishAPIAvatar writes an API avatar to the public upload root while
// preserving the legacy path shape. The caller owns the subsequent user row
// update and must call CleanupAPIAvatar when that update fails.
func PublishAPIAvatar(ctx context.Context, actorID, originalName string, data []byte) (APIAvatarPublication, error) {
	owner, err := domain.ParseObjectID(strings.TrimSpace(actorID))
	if err != nil || owner.IsZero() {
		if err == nil {
			err = domain.ErrInvalidObjectID
		}
		return APIAvatarPublication{}, fmt.Errorf("api avatar owner: %w", err)
	}
	if len(data) == 0 || int64(len(data)) > apiAvatarMaxBytes {
		return APIAvatarPublication{}, fmt.Errorf("api avatar size is outside the allowed range")
	}
	_, ext := SplitFilename(originalName)
	ext = strings.ToLower(ext)
	switch ext {
	case ".gif", ".jpg", ".png", ".bmp", ".jpeg":
	default:
		return APIAvatarPublication{}, fmt.Errorf("api avatar extension is not supported")
	}
	guid := NewGuid()
	if guid == "" {
		return APIAvatarPublication{}, fmt.Errorf("api avatar identity unavailable")
	}
	path := "public/upload/" + owner.Hex() + "/images/logo/" + guid + ext
	logical, err := applicationcontent.ParseStoredPath(path)
	if err != nil {
		return APIAvatarPublication{}, err
	}
	digest := sha256.Sum256(data)
	if contentStore == nil {
		return APIAvatarPublication{}, fmt.Errorf("api avatar content store unavailable")
	}
	_, err = contentStore.Publish(ctx, applicationcontent.PublishRequest{
		Identity: applicationcontent.AssetIdentity{
			OperationID: guid, OwnerID: owner, Kind: applicationcontent.AssetImage,
			SourceID: owner.Hex(), DestinationID: guid, Digest: digest,
		},
		Destination: logical,
		Source:      bytes.NewReader(data),
	})
	if err != nil {
		return APIAvatarPublication{}, err
	}
	return APIAvatarPublication{Path: path, Logical: logical, Digest: digest, Size: int64(len(data))}, nil
}

// CleanupAPIAvatar removes a publication after the metadata update failed.
// It uses the configured content lifecycle instead of constructing an OS path
// in the HTTP adapter.
func CleanupAPIAvatar(ctx context.Context, publication APIAvatarPublication) error {
	if publication.Logical.Value == "" || contentLifecycle == nil {
		return fmt.Errorf("api avatar cleanup unavailable")
	}
	quarantine, err := contentLifecycle.Quarantine(ctx, publication.Logical, hex.EncodeToString(publication.Digest[:]), publication.Digest, publication.Size)
	if err != nil {
		return err
	}
	return contentLifecycle.Purge(ctx, quarantine, publication.Digest, publication.Size)
}
