package db

import (
	"encoding/json"
	"testing"

	. "github.com/yangphere/leanote/app/lea"
)

func TestMustObjectIDFromHexValid(t *testing.T) {
	const hex = "507f1f77bcf86cd799439011"
	id := MustObjectIDFromHex(hex)
	if id.Hex() != hex {
		t.Fatalf("MustObjectIDFromHex(%q).Hex() = %q, want %q", hex, id.Hex(), hex)
	}
	if zero := (ObjectID{}); zero.Hex() != "" {
		t.Fatalf("zero ObjectID.Hex() = %q, want %q (legacy mgo semantics)", zero.Hex(), "")
	}
}

func TestMustObjectIDFromHexInvalidPanics(t *testing.T) {
	for _, hex := range []string{"", "zzzz", "507f1f77bcf86cd79943901", "507f1f77bcf86cd7994390111"} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("MustObjectIDFromHex(%q) did not panic", hex)
				}
			}()
			MustObjectIDFromHex(hex)
		}()
	}
}

func TestIsValidObjectIDHex(t *testing.T) {
	cases := map[string]bool{
		"507f1f77bcf86cd799439011": true,
		"000000000000000000000000": true,
		"507F1F77BCF86CD799439011": true, // hex decoding accepts uppercase; output stays lowercase
		"":                         false,
		"507f1f77bcf86cd79943901":  false,
		"zzzzzzzzzzzzzzzzzzzzzzzz": false,
	}
	for hex, want := range cases {
		if got := IsValidObjectIDHex(hex); got != want {
			t.Errorf("IsValidObjectIDHex(%q) = %v, want %v", hex, got, want)
		}
	}
}

func TestObjectIDJSONIsLowercaseHex(t *testing.T) {
	type doc struct {
		ID ObjectID `json:"ID"`
	}
	raw, err := json.Marshal(doc{ID: MustObjectIDFromHex("507f1f77bcf86cd799439011")})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	want := `{"ID":"507f1f77bcf86cd799439011"}`
	if string(raw) != want {
		t.Fatalf("json.Marshal(ObjectID) = %s, want %s", raw, want)
	}

	zero, err := json.Marshal(doc{})
	if err != nil {
		t.Fatalf("json.Marshal zero: %v", err)
	}
	wantZero := `{"ID":""}`
	if string(zero) != wantZero {
		t.Fatalf("json.Marshal(zero ObjectID) = %s, want %s (legacy mgo shape)", zero, wantZero)
	}

	var back doc
	if err := json.Unmarshal([]byte(wantZero), &back); err != nil {
		t.Fatalf("json.Unmarshal zero: %v", err)
	}
	if !back.ID.IsZero() {
		t.Fatalf("empty JSON string did not unmarshal to zero ObjectID: %v", back.ID)
	}
}
