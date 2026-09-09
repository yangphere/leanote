package domain

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestObjectIDJSONContract(t *testing.T) {
	const upper = "507F1F77BCF86CD799439011"
	const lower = "507f1f77bcf86cd799439011"

	parsed, err := ParseObjectID(upper)
	if err != nil {
		t.Fatalf("ParseObjectID(%q): %v", upper, err)
	}
	if got := parsed.Hex(); got != lower {
		t.Fatalf("Hex() = %q, want %q", got, lower)
	}
	if got, err := json.Marshal(parsed); err != nil || string(got) != `"507f1f77bcf86cd799439011"` {
		t.Fatalf("MarshalJSON() = %s, %v", got, err)
	}

	var zero ObjectID
	if got, err := json.Marshal(zero); err != nil || string(got) != `""` {
		t.Fatalf("zero MarshalJSON() = %s, %v", got, err)
	}
	for _, input := range []string{`""`, `null`} {
		var got ObjectID
		if err := json.Unmarshal([]byte(input), &got); err != nil {
			t.Fatalf("UnmarshalJSON(%s): %v", input, err)
		}
		if !got.IsZero() {
			t.Fatalf("UnmarshalJSON(%s) = %q, want zero", input, got.Hex())
		}
	}
}

func TestObjectIDStringPreservesDriverDiagnosticShape(t *testing.T) {
	parsed, err := ParseObjectID("507f1f77bcf86cd799439011")
	if err != nil {
		t.Fatalf("ParseObjectID: %v", err)
	}
	if got, want := parsed.String(), `ObjectID("507f1f77bcf86cd799439011")`; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
	var zero ObjectID
	if got, want := zero.String(), `ObjectID("000000000000000000000000")`; got != want {
		t.Fatalf("zero String() = %q, want %q", got, want)
	}
}

func TestObjectIDRejectsMalformedInput(t *testing.T) {
	for _, input := range []string{"0", "not-an-id", "507f1f77bcf86cd79943901z"} {
		if _, err := ParseObjectID(input); !errors.Is(err, ErrInvalidObjectID) {
			t.Errorf("ParseObjectID(%q) error = %v, want ErrInvalidObjectID", input, err)
		}
	}
	for _, input := range []string{`1`, `{}`, `[]`, `"not-an-id"`} {
		var got ObjectID
		if err := json.Unmarshal([]byte(input), &got); !errors.Is(err, ErrInvalidObjectID) {
			t.Errorf("UnmarshalJSON(%s) error = %v, want ErrInvalidObjectID", input, err)
		}
	}
}

func TestValidateJSONValue(t *testing.T) {
	if err := ValidateJSONValue(map[string]interface{}{"items": []string{}}); err != nil {
		t.Fatalf("ValidateJSONValue valid value: %v", err)
	}
	if err := ValidateJSONValue(func() {}); err == nil {
		t.Fatal("ValidateJSONValue accepted a function")
	}
}
