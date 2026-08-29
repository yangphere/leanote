package lea

import (
	"encoding/json"
	"fmt"
	"reflect"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// CodecRegistry is the default BSON registry plus explicit codecs for
// ObjectID. The explicit registration is required because the driver's
// kind-based array decoder shadows the pointer-receiver ValueUnmarshaler for
// defined [12]byte types. Pass it to the client via
// options.Client().SetRegistry and to bson.Marshal/Unmarshal helpers via the
// WithRegistry variants.
var CodecRegistry = func() *bson.Registry {
	reg := bson.NewRegistry()
	t := reflect.TypeOf(ObjectID{})
	reg.RegisterTypeEncoder(t, bson.ValueEncoderFunc(func(_ bson.EncodeContext, vw bson.ValueWriter, val reflect.Value) error {
		if !val.CanInterface() {
			return fmt.Errorf("ObjectID encoder: value not addressable")
		}
		id, ok := val.Interface().(ObjectID)
		if !ok {
			return fmt.Errorf("ObjectID encoder: expected %T, got %T", ObjectID{}, val.Interface())
		}
		return vw.WriteObjectID(bson.ObjectID(id))
	}))
	reg.RegisterTypeDecoder(t, bson.ValueDecoderFunc(func(_ bson.DecodeContext, vr bson.ValueReader, val reflect.Value) error {
		// mgo's ObjectId was a string type, so legacy documents may store ID
		// fields as BSON strings (e.g. ParentNotebookId: "" on root
		// notebooks). Mirror mgo decoding: ObjectId bytes as-is, "" as zero,
		// 24-char hex as decoded ObjectId, anything else as an error.
		switch vr.Type() {
		case bson.TypeObjectID:
			oid, err := vr.ReadObjectID()
			if err != nil {
				return err
			}
			if !val.CanSet() {
				return fmt.Errorf("ObjectID decoder: value not settable")
			}
			val.Set(reflect.ValueOf(ObjectID(oid)))
			return nil
		case bson.TypeString:
			s, err := vr.ReadString()
			if err != nil {
				return err
			}
			if !val.CanSet() {
				return fmt.Errorf("ObjectID decoder: value not settable")
			}
			if s == "" {
				val.Set(reflect.ValueOf(ObjectID{}))
				return nil
			}
			oid, err := bson.ObjectIDFromHex(s)
			if err != nil {
				return err
			}
			val.Set(reflect.ValueOf(ObjectID(oid)))
			return nil
		default:
			return fmt.Errorf("cannot decode BSON %s into an ObjectID", vr.Type())
		}
	}))
	return reg
}()

// ObjectID is the project-wide ObjectID type. It stores and transmits exactly
// like bson.ObjectID (12-byte BSON ObjectId) but keeps mgo's legacy JSON
// shape: the zero value marshals to "" instead of "000000000000000000000000",
// and both zero and 24-hex values unmarshal.
type ObjectID bson.ObjectID

// Hex returns the 24-char lowercase hex representation. The zero value
// returns "" — mgo's legacy semantics, relied on by callers that use
// Hex() == "" as an "unset" test.
func (id ObjectID) Hex() string {
	if id.IsZero() {
		return ""
	}
	return bson.ObjectID(id).Hex()
}

// IsZero reports whether id is the zero value.
func (id ObjectID) IsZero() bool {
	return bson.ObjectID(id).IsZero()
}

// String implements fmt.Stringer like bson.ObjectID.
func (id ObjectID) String() string {
	return bson.ObjectID(id).String()
}

// MarshalJSON keeps the external contract: zero → "", otherwise quoted hex.
func (id ObjectID) MarshalJSON() ([]byte, error) {
	if id.IsZero() {
		return []byte(`""`), nil
	}
	return json.Marshal(id.Hex())
}

// UnmarshalJSON accepts "" (legacy zero) and 24-char hex.
func (id *ObjectID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s == "" {
		*id = ObjectID{}
		return nil
	}
	oid, err := bson.ObjectIDFromHex(s)
	if err != nil {
		return err
	}
	*id = ObjectID(oid)
	return nil
}

// MarshalBSONValue stores the value as a standard BSON ObjectId
// (type byte 0x07 followed by the raw 12 bytes).
func (id ObjectID) MarshalBSONValue() (bson.Type, []byte, error) {
	return bson.TypeObjectID, id[:], nil
}

// UnmarshalBSONValue reads a standard BSON ObjectId.
func (id *ObjectID) UnmarshalBSONValue(t bson.Type, raw []byte) error {
	if t != bson.TypeObjectID {
		return fmt.Errorf("cannot unmarshal BSON %s into an ObjectID", t)
	}
	if len(raw) != 12 {
		return fmt.Errorf("ObjectID requires exactly 12 bytes of data, got %d", len(raw))
	}
	var arr [12]byte
	copy(arr[:], raw)
	*id = ObjectID(arr)
	return nil
}
