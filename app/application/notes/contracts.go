package notes

import (
	"context"
	"errors"
	"time"

	"github.com/yangphere/leanote/app/domain"
)

var (
	ErrOperationConflict = errors.New("workspace operation identity conflict")
	ErrOperationBusy     = errors.New("workspace operation is leased")
	ErrOperationCAS      = errors.New("workspace operation state changed")
	ErrOperationTerminal = errors.New("workspace operation is terminal")
	ErrUnknownStepResult = errors.New("workspace operation step result is unknown")
	ErrOperationPending  = errors.New("workspace operation requires explicit repair")
)

type ErrorCategory string

const (
	ErrorValidation   ErrorCategory = "validation"
	ErrorNotFound     ErrorCategory = "not_found"
	ErrorUnauthorized ErrorCategory = "unauthorized"
	ErrorConflict     ErrorCategory = "conflict"
	ErrorDuplicate    ErrorCategory = "duplicate"
	ErrorStorage      ErrorCategory = "storage"
	ErrorTimeout      ErrorCategory = "timeout"
	ErrorPartialWrite ErrorCategory = "partial_write"
	ErrorSideEffect   ErrorCategory = "side_effect"
)

type CommandResult struct {
	Committed    bool
	PartialWrite bool
	RetrySafe    bool
	USN          int
	Error        ErrorCategory
	AppliedSteps []string
	FailedStep   string
	Items        []ItemResult
}

type ItemResult struct {
	ResourceID string
	USN        int
	Committed  bool
	Error      ErrorCategory
}

func (r CommandResult) OK() bool {
	if !r.Committed || r.Error != "" || r.PartialWrite {
		return false
	}
	for _, item := range r.Items {
		if !item.Committed || item.Error != "" {
			return false
		}
	}
	return true
}

// SaveNoteCommand preserves adapter field presence without exposing BSON.
type SaveNoteCommand struct {
	ActorUserID string
	NoteID      string
	// OperationID is an optional client-generated operation generation. When
	// present, the adapter scopes it to owner and action before persisting it;
	// an omitted value retains the legacy non-retry-safe behavior.
	OperationID string
	ExpectedUSN *int
	Metadata    map[string]any
	Content     *string
	Abstract    *string
	UpdatedTime time.Time
	AssetWork   *AssetMutation
}

// AssetMutation is supplied by the content boundary. It runs only after the
// note's required ExpectedUSN mutation has committed and is recorded in the
// durable projection receipt for retry.
type AssetMutation struct {
	// OperationID is filled by SaveNote after canonical identity is known. It
	// is not client input and is used only to fence the asset adapter's write.
	OperationID string `json:"-"`
	Assets      []OperationAsset
	// Apply receives the generation of the committed note mutation. Providers
	// must use it for any resource write that belongs to this reconciliation.
	Apply  func(context.Context, int) error
	Verify func(context.Context) (bool, error)
}

// WorkspaceRepository is the persistence port exposed to notes use cases.
// Implementations keep BSON queries and driver values behind the adapter.
type WorkspaceRepository interface {
	AllocateUserUSN(context.Context, domain.ObjectID) (int, error)
}

// WorkspaceUnitOfWork owns topology selection. It may use a transaction or a
// durable standalone operation, but must never silently replay a transient
// transaction failure outside the transaction.
type WorkspaceUnitOfWork interface {
	Run(context.Context, MutationPlan) (MutationResult, error)
	GetOperation(context.Context, domain.ObjectID, string) (OperationReceipt, error)
}

type SharePermissionPort interface {
	CanUpdateNote(context.Context, domain.ObjectID, domain.ObjectID, domain.ObjectID) (bool, error)
}

type ContentAssetPort interface {
	ReconcileNote(context.Context, string, domain.ObjectID, domain.ObjectID) error
	CopyNote(context.Context, string, domain.ObjectID, domain.ObjectID, domain.ObjectID) error
	DeleteNote(context.Context, string, domain.ObjectID, domain.ObjectID) error
}

type PublishingProjectionPort interface {
	RepairNote(context.Context, string, domain.ObjectID, domain.ObjectID) error
}

type OperationStatus string

const (
	OperationPending       OperationStatus = "pending"
	OperationRunning       OperationStatus = "running"
	OperationCompensating  OperationStatus = "compensating"
	OperationCompensated   OperationStatus = "compensated"
	OperationCommitted     OperationStatus = "committed"
	OperationFailed        OperationStatus = "failed"
	OperationRepairPending OperationStatus = "repair_pending"
)

// IsTerminalOperation reports whether a receipt has reached an immutable
// outcome. Only repair_pending receipts may be claimed again; failed and
// compensated receipts have already closed their recovery path.
func IsTerminalOperation(status OperationStatus) bool {
	switch status {
	case OperationCommitted, OperationCompensated, OperationFailed:
		return true
	default:
		return false
	}
}

// OperationReceipt is the durable recovery record for a multi-write command.
// BeforeState and DesiredState are canonical JSON owned by the application
// command, never BSON or driver values.
type OperationReceipt struct {
	OperationID string
	OwnerID     domain.ObjectID
	ResourceID  domain.ObjectID
	Kind        string
	InputDigest string
	Assets      []OperationAsset
	// StepNames freezes the complete ordered target set at Begin time.  It is
	// deliberately metadata-only so a retry cannot silently add a resource
	// discovered from a later database read.
	StepNames []string
	// StepUSNs freezes the USN allocated by a multi-resource step. A single
	// receipt may contain several resource mutations, so AssignedUSN alone is
	// insufficient to verify the current step after process loss.
	StepUSNs     map[string]int
	BeforeState  []byte
	DesiredState []byte
	// ResultState contains the minimal outcome needed to replay the exact
	// terminal command response. It must never contain note content, tokens or
	// other secret material and, unlike recovery state, is retained after
	// commit.
	ResultState  []byte
	AssignedUSN  int
	AppliedSteps []string
	CurrentStep  string
	FailedStep   string
	Status       OperationStatus
	LastError    string
	LeaseID      string
	LeaseUntil   time.Time
	Version      int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
	// TerminalAt is set only for committed/compensated/failed receipts. It is
	// the anchor for the terminal-only TTL index; pending recovery records do
	// not carry this field.
	TerminalAt time.Time
}

// OperationAsset records only the stable identity needed to resume an asset
// side effect. It deliberately contains no file contents or path metadata.
type OperationAsset struct {
	AssetID       string
	LocalFileID   string
	ContentSHA256 string
	Index         int
	IsAttach      bool
}

type OperationStore interface {
	Begin(context.Context, OperationReceipt) (OperationReceipt, error)
	Claim(context.Context, domain.ObjectID, string, time.Time, time.Duration) (OperationReceipt, error)
	Save(context.Context, OperationReceipt) (OperationReceipt, error)
	Get(context.Context, domain.ObjectID, string) (OperationReceipt, error)
}

type FailurePolicy string

const (
	FailureCompensate FailurePolicy = "compensate"
	FailurePending    FailurePolicy = "pending"
)

type MutationStep struct {
	Name               string
	Apply              func(context.Context) error
	Verify             func(context.Context) (bool, error)
	Compensate         func(context.Context) error
	ReplaySafe         bool
	AssignedUSN        func() int
	RestoreAssignedUSN func(int)
}

type MutationPlan struct {
	OperationID        string
	OwnerID            domain.ObjectID
	ResourceID         domain.ObjectID
	Kind               string
	InputDigest        string
	Assets             []OperationAsset
	BeforeState        []byte
	DesiredState       []byte
	Steps              []MutationStep
	AssignedUSN        func() int
	RestoreAssignedUSN func(int)
	// RestoreBeforeState lets a plan rebuild its closures from the first
	// durable before-image when two callers race before Begin returns the
	// existing receipt.  A resumable plan must never fall back to a later
	// database read as its recovery baseline.
	RestoreBeforeState  func([]byte) error
	RestoreDesiredState func([]byte) error
	CaptureResultState  func() []byte
	RestoreResultState  func([]byte) error
	FailurePolicy       FailurePolicy
}

type MutationResult struct {
	Mode         string
	Committed    bool
	PartialWrite bool
	AppliedSteps []string
	FailedStep   string
	Operation    OperationReceipt
}
