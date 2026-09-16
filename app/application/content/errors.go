package content

import "fmt"

// ErrorCategory is stable across adapters and HTTP mappers.
type ErrorCategory string

const (
	ErrorValidation         ErrorCategory = "validation"
	ErrorUnauthorized       ErrorCategory = "unauthorized"
	ErrorNotFound           ErrorCategory = "not_found"
	ErrorConflict           ErrorCategory = "conflict"
	ErrorTooLarge           ErrorCategory = "too_large"
	ErrorUnsupportedMedia   ErrorCategory = "unsupported_media"
	ErrorUnsafePath         ErrorCategory = "unsafe_path"
	ErrorStorageUnavailable ErrorCategory = "storage_unavailable"
	ErrorUnsupportedFS      ErrorCategory = "unsupported_filesystem"
	ErrorTimeout            ErrorCategory = "timeout"
	ErrorRendererFailed     ErrorCategory = "renderer_failed"
	ErrorPartialWrite       ErrorCategory = "partial_write"
	ErrorUnknownResult      ErrorCategory = "unknown_result"
	ErrorDependency         ErrorCategory = "dependency"
)

// Error is safe to expose at an adapter boundary.  Cause is retained for
// diagnostics but should not be rendered directly to a user.
type Error struct {
	Category ErrorCategory
	Code     string
	Cause    error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Code == "" {
		return string(e.Category)
	}
	return fmt.Sprintf("%s: %s", e.Category, e.Code)
}

func (e *Error) Unwrap() error { return e.Cause }

// NewError lets infrastructure adapters preserve the application error
// taxonomy without exposing sensitive lower-layer details in Error().
func NewError(category ErrorCategory, code string, cause error) error {
	return &Error{Category: category, Code: code, Cause: cause}
}

func contentError(category ErrorCategory, code string, cause error) error {
	return NewError(category, code, cause)
}

func validationError(code string, cause error) error {
	return contentError(ErrorValidation, code, cause)
}

func unsafePathError(code string, cause error) error {
	return contentError(ErrorUnsafePath, code, cause)
}

func conflictError(code string, cause error) error {
	return contentError(ErrorConflict, code, cause)
}

func unsupportedFSError(code string, cause error) error {
	return contentError(ErrorUnsupportedFS, code, cause)
}

func storageError(code string, cause error) error {
	return contentError(ErrorStorageUnavailable, code, cause)
}

func unknownError(code string, cause error) error {
	return contentError(ErrorUnknownResult, code, cause)
}
