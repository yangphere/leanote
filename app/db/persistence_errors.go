package db

import (
	"errors"
	"strings"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

var (
	ErrMongoClientNotInitialized = errors.New("mongo client is not initialized")
	ErrPartialWrite              = errors.New("partial_write")
	ErrSideEffect                = errors.New("side_effect")
	ErrDuplicateIdentity         = errors.New("duplicate_identity")
	ErrDocumentNotFound          = errors.New("document_not_found")
	ErrTokenExpired              = errors.New("token expired")
	ErrTokenTypeMismatch         = errors.New("token type mismatch")
)

// PersistenceError keeps a stable category while preserving the wrapped
// driver cause for diagnostics and retry decisions.
type PersistenceError struct {
	Code  string
	Cause error
}

func (e *PersistenceError) Error() string {
	if e == nil {
		return ""
	}
	// Codes cross the repository boundary.  The wrapped driver error remains
	// available through Unwrap for internal diagnostics, but Error must not
	// echo index names, emails, usernames, or other server details to callers.
	return e.Code
}

func (e *PersistenceError) Unwrap() error { return e.Cause }

func (e *PersistenceError) Is(target error) bool {
	if target == nil {
		return false
	}
	if other, ok := target.(*PersistenceError); ok {
		return e.Code == other.Code
	}
	switch target {
	case ErrPartialWrite, ErrSideEffect, ErrDuplicateIdentity, ErrDocumentNotFound, ErrTokenExpired, ErrTokenTypeMismatch:
		return e.Code == target.Error()
	default:
		return false
	}
}

// MapDuplicateIdentityError turns a Mongo duplicate-key failure into a stable
// domain category without exposing the raw index message to callers.
func MapDuplicateIdentityError(err error) error {
	if err == nil {
		return nil
	}
	if !mongo.IsDuplicateKeyError(err) {
		return err
	}
	return &PersistenceError{Code: ErrDuplicateIdentity.Error(), Cause: err}
}

func IsDuplicateIdentityError(err error) bool {
	return errors.Is(err, ErrDuplicateIdentity)
}

// DuplicateIdentityField provides a stable best-effort field label for
// metrics and adapter mapping. The raw Mongo message remains wrapped for
// diagnostics but is not required by callers.
func DuplicateIdentityField(err error) string {
	return duplicateIdentityField(err)
}

func duplicateIdentityField(err error) string {
	if err == nil || !mongo.IsDuplicateKeyError(err) {
		return ""
	}
	message := strings.ToLower(err.Error())
	for _, field := range []string{"email", "username", "thirdtype", "thirduserid"} {
		if strings.Contains(message, field) {
			return field
		}
	}
	return ""
}
