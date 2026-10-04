// Package domain contains framework- and persistence-neutral value types used
// by the application's domain models.
package domain

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

// ObjectID is the domain representation of a Mongo-style 12-byte identifier.
// It deliberately has no BSON or framework dependency; adapters at the
// persistence boundary own those conversions.
type ObjectID [12]byte

// ErrInvalidObjectID identifies malformed textual or JSON identifiers.
var ErrInvalidObjectID = errors.New("invalid ObjectID")

// ParseObjectID accepts the legacy empty string (the zero value) or a 24-byte
// hexadecimal identifier. Hex input is case-insensitive and Hex always emits
// lowercase output.
func ParseObjectID(value string) (ObjectID, error) {
	if value == "" {
		return ObjectID{}, nil
	}
	if len(value) != 24 {
		return ObjectID{}, fmt.Errorf("%w: expected empty or 24 hexadecimal characters", ErrInvalidObjectID)
	}
	var id ObjectID
	if _, err := hex.Decode(id[:], []byte(value)); err != nil {
		return ObjectID{}, fmt.Errorf("%w: %v", ErrInvalidObjectID, err)
	}
	return id, nil
}

// Hex returns the 24-character lowercase representation, or "" for zero.
func (id ObjectID) Hex() string {
	if id.IsZero() {
		return ""
	}
	encoded := make([]byte, hex.EncodedLen(len(id)))
	hex.Encode(encoded, id[:])
	return string(encoded)
}

// IsZero reports whether all identifier bytes are zero.
func (id ObjectID) IsZero() bool {
	return id == ObjectID{}
}

// String implements fmt.Stringer with the mongo-driver-compatible diagnostic
// form. This is intentionally distinct from Hex: the legacy lea.ObjectID
// exposed ObjectID("<24 hex chars>") (including 24 zeroes for the zero value)
// in logs and formatted errors, and the migration alias must not change that
// non-JSON representation.
func (id ObjectID) String() string {
	return fmt.Sprintf("ObjectID(%q)", hex.EncodeToString(id[:]))
}

// MarshalJSON preserves the public wire shape: zero is "", otherwise a
// lowercase 24-character hexadecimal string.
func (id ObjectID) MarshalJSON() ([]byte, error) {
	return json.Marshal(id.Hex())
}

// UnmarshalJSON accepts a JSON string or null. Empty strings and null map to
// zero; numbers, objects, arrays and malformed hex return an error.
func (id *ObjectID) UnmarshalJSON(data []byte) error {
	if id == nil {
		return fmt.Errorf("%w: nil destination", ErrInvalidObjectID)
	}
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*id = ObjectID{}
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidObjectID, err)
	}
	parsed, err := ParseObjectID(value)
	if err != nil {
		return err
	}
	*id = parsed
	return nil
}
