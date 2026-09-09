package domain

import (
	"encoding/json"
	"fmt"
)

// ValidateJSONValue verifies that a dynamic domain value can be represented by
// encoding/json without changing the value or silently falling back. Callers
// such as API envelopes and theme metadata use this single boundary check.
func ValidateJSONValue(value interface{}) error {
	if _, err := json.Marshal(value); err != nil {
		return fmt.Errorf("domain JSON value is not encodable: %w", err)
	}
	return nil
}
