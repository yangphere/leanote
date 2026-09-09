package info

import (
	"encoding/json"
	"fmt"

	"github.com/yangphere/leanote/app/domain"
)

// controller ajax返回
type Re struct {
	Ok   bool
	Code int
	Msg  string
	Id   string
	List interface{}
	Item interface{}
}

func NewRe() Re {
	return Re{Ok: false}
}

// ValidateDynamicValues checks List and Item at the API boundary while
// preserving their existing JSON-compatible value domain and nil/empty shape.
func (r Re) ValidateDynamicValues() error {
	if err := domain.ValidateJSONValue(r.List); err != nil {
		return fmt.Errorf("Re.List: %w", err)
	}
	if err := domain.ValidateJSONValue(r.Item); err != nil {
		return fmt.Errorf("Re.Item: %w", err)
	}
	return nil
}

// MarshalJSON validates the dynamic payload fields at the serialization
// boundary while retaining the existing field names, order and nil/empty
// behavior of encoding/json's struct encoder.
func (r Re) MarshalJSON() ([]byte, error) {
	if err := r.ValidateDynamicValues(); err != nil {
		return nil, err
	}
	type reJSON Re
	return json.Marshal(reJSON(r))
}
