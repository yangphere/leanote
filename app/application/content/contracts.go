// Package content contains framework-neutral contracts and safety primitives
// for note assets, uploads, archives, and rendering.
//
// HTTP, MongoDB, and process adapters belong outside this package.  The
// application boundary deals in logical paths, stable identities, streams,
// and typed outcomes; it never exposes an operating-system path to callers.
package content

import (
	"context"
	"crypto/sha256"
	"io"

	"github.com/yangphere/leanote/app/domain"
)

// RootKind identifies one configured logical content root.
type RootKind string

const (
	RootPrivateFiles RootKind = "private_files"
	RootPublicUpload RootKind = "public_upload"
	RootTemporary    RootKind = "temporary"
)

// AssetKind describes the resource being published.  It is intentionally a
// string so adapters can add a domain-specific kind without importing a
// persistence package.
type AssetKind string

const (
	AssetImage      AssetKind = "image"
	AssetAttachment AssetKind = "attachment"
	AssetArchive    AssetKind = "archive"
	AssetPDF        AssetKind = "pdf"
)

// LogicalPath is a root-relative path.  Value always uses '/' separators and
// never contains an absolute or parent segment.  It is safe to persist and to
// return in DTOs; it is not an OS path.
type LogicalPath struct {
	Kind  RootKind
	Value string
}

// AssetIdentity is the stable identity used by retryable writes.  Destination
// is derived from operation/generation/owner/kind/source, while Digest binds
// the bytes and output-affecting metadata.
type AssetIdentity struct {
	OperationID   string
	Generation    int64
	OwnerID       domain.ObjectID
	Kind          AssetKind
	SourceID      string
	DestinationID string
	Digest        [sha256.Size]byte
}

// PublishRequest writes Source to Destination with no-clobber semantics.
// Destination must be stable for the identity; callers should use
// StableDestination when they do not already have a frozen logical path.
type PublishRequest struct {
	Identity    AssetIdentity
	Destination LogicalPath
	Source      io.Reader
}

type PublishStatus string

const (
	PublishApplied        PublishStatus = "applied"
	PublishAlreadyApplied PublishStatus = "already_applied"
)

type PublishResult struct {
	Status      PublishStatus
	Destination LogicalPath
	Digest      [sha256.Size]byte
	Size        int64
}

type VerificationStatus string

const (
	VerificationApplied    VerificationStatus = "applied"
	VerificationNotApplied VerificationStatus = "not_applied"
	VerificationConflict   VerificationStatus = "conflict"
	VerificationUnknown    VerificationStatus = "unknown"
)

// VerifyRequest optionally checks owner-scoped metadata and projection after
// verifying bytes.  A dependency error is unknown, never not-applied.
type VerifyRequest struct {
	Identity         AssetIdentity
	Destination      LogicalPath
	ExpectedDigest   [sha256.Size]byte
	VerifyMetadata   func(context.Context) (bool, error)
	VerifyProjection func(context.Context) (bool, error)
}

type VerifyResult struct {
	Status VerificationStatus
	Digest [sha256.Size]byte
	Size   int64
}

type OpenResult struct {
	Reader io.ReadCloser
	Size   int64
}

// ContentStore is the application-facing storage seam.  Implementations may
// use a filesystem, object store, or a test double, but must preserve the
// typed outcomes and no-clobber contract.
type ContentStore interface {
	Open(context.Context, LogicalPath) (OpenResult, error)
	Publish(context.Context, PublishRequest) (PublishResult, error)
	Verify(context.Context, VerifyRequest) (VerifyResult, error)
}
