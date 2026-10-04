package db

import (
	"bytes"
	"encoding/json"
	"testing"

	. "github.com/yangphere/leanote/app/lea"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestSplitUpdateKindUsesDomainObjectIDCodec(t *testing.T) {
	update := bson.M{"$set": bson.M{"UserId": MustObjectIDFromHex("507f1f77bcf86cd799439011")}}
	replacement, err := splitUpdateKind(update)
	if err != nil {
		t.Fatalf("splitUpdateKind: %v", err)
	}
	if replacement {
		t.Fatal("operator update was classified as replacement")
	}

	var encoded bytes.Buffer
	encoder := bson.NewEncoder(bson.NewDocumentWriter(&encoded))
	encoder.SetRegistry(CodecRegistry)
	if err := encoder.Encode(update); err != nil {
		t.Fatalf("encode update: %v", err)
	}
	if got := bson.Raw(encoded.Bytes()).Lookup("$set", "UserId").Type; got != bson.TypeObjectID {
		t.Fatalf("encoded UserId type = %s, want object id", got)
	}
}

func TestObjectIDBSONAdapterReadsLegacyStrings(t *testing.T) {
	type document struct {
		ID ObjectID `bson:"ID"`
	}

	tests := []struct {
		name string
		raw  bson.M
		want ObjectID
	}{
		{name: "object id", raw: bson.M{"ID": bson.ObjectID{0x50, 0x7f, 0x1f, 0x77, 0xbc, 0xf8, 0x6c, 0xd7, 0x99, 0x43, 0x90, 0x11}}, want: MustObjectIDFromHex("507f1f77bcf86cd799439011")},
		{name: "empty legacy string", raw: bson.M{"ID": ""}, want: ObjectID{}},
		{name: "hex legacy string", raw: bson.M{"ID": "507F1F77BCF86CD799439011"}, want: MustObjectIDFromHex("507f1f77bcf86cd799439011")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := bson.Marshal(tt.raw)
			if err != nil {
				t.Fatalf("marshal raw BSON: %v", err)
			}
			var got document
			decoder := bson.NewDecoder(bson.NewDocumentReader(bytes.NewReader(raw)))
			decoder.SetRegistry(CodecRegistry)
			if err := decoder.Decode(&got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if got.ID != tt.want {
				t.Fatalf("decoded ID = %q, want %q", got.ID.Hex(), tt.want.Hex())
			}
		})
	}
}

func TestObjectIDBSONAdapterRejectsUnknownValues(t *testing.T) {
	type document struct {
		ID ObjectID `bson:"ID"`
	}
	for _, rawValue := range []interface{}{"bad", 1, true, bson.A{}} {
		raw, err := bson.Marshal(bson.M{"ID": rawValue})
		if err != nil {
			t.Fatalf("marshal %T: %v", rawValue, err)
		}
		var got document
		decoder := bson.NewDecoder(bson.NewDocumentReader(bytes.NewReader(raw)))
		decoder.SetRegistry(CodecRegistry)
		if err := decoder.Decode(&got); err == nil {
			t.Errorf("decode %T unexpectedly succeeded", rawValue)
		}
	}
}

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
